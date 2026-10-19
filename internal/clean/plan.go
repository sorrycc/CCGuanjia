// Package clean backs up and cleans a profile's folder, in the order of
// clean-claude.ts, each step only after the one before succeeds:
//
//  1. copy the configuration out of the folder,
//  2. archive projects/ and file-history/ to tar.gz,
//  3. verify both archives: the gzip stream whole, and as many files in
//     each as were archived, of the same total size,
//  4. only then edit settings.json, and delete,
//  5. last of all, if asked, sign out.
//
// A failure before the deletes stops the run with nothing deleted, and
// signing out comes last so that no earlier failure leaves the profile
// signed out.
package clean

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/jsonedit"
)

// Kind is what a run does with an item.
type Kind int

const (
	// Archive: archived to tar.gz, verified, then deleted.
	Archive Kind = iota
	// Delete: deleted, after a copy for backups/.
	Delete
	// DeleteEmpty: a folder deleted only while it is empty.
	DeleteEmpty
)

// Item is a file or folder of the profile a run may remove.
type Item struct {
	Kind Kind
	// Rel is its path in the profile's folder; telemetry's is
	// "telemetry/1p_failed_events.*", for its files.
	Rel   string
	Paths []string
	Bytes int64
	Files int
	// Selected says whether the run removes it.
	Selected bool
}

// ArchiveTargets are archived before they are deleted.
var ArchiveTargets = []string{"projects", "file-history"}

// PlainDeletes are deleted outright: caches, scratch folders and stale
// state that Claude Code makes again when it needs them.
var PlainDeletes = []string{
	"backups",
	"jobs",
	"paste-cache",
	"shell-snapshots",
	"tasks",
	"session-env",
	".DS_Store",
	"daemon.log",
	"daemon-auth-cooldown",
	"daemon-auth-status.json",
	"stats-cache.json",
	".last-update-result.json",
}

// EmptyDeletes are deleted only when empty.
var EmptyDeletes = []string{"teams", "downloads"}

// TelemetryPrefix names the telemetry files that failed to send.
const TelemetryPrefix = "1p_failed_events."

// StrayConfig is a .claude.json inside ~/.claude, which Claude Code reads
// only when CLAUDE_CONFIG_DIR names ~/.claude; it is listed when its
// machine ID is not the one of ~/.claude.json.
const StrayConfig = ".claude.json"

// CompressionRatio is how large an archive of transcripts comes out,
// measured on real ones: they carry images and tool output that do not
// compress much.
const CompressionRatio = 0.45

// SpaceSafety is the headroom asked of the free space for the archives.
const SpaceSafety = 1.5

// Plan is what a run of a profile would do, from a look at its folder.
type Plan struct {
	Profile *claude.Profile
	Items   []*Item
	// Variants are the settings.json.* copies, deleted after a copy.
	Variants []*Item
	Settings SettingsPlan
	SignOut  claude.SignOutPlan
	// Running are the processes of Claude Code, which a run waits for
	// unless forced.
	Running []claude.Process
}

// SettingsPlan is what the edits of settings.json would change.
type SettingsPlan struct {
	// Path is the file written, where settings.json links to.
	Path string
	// Shared reports whether settings.json is a link, so that an edit
	// changes every profile sharing it.
	Shared bool
	// CleanupDays is cleanupPeriodDays, 0 when unset.
	CleanupDays int
	// DeadAllow are the entries of permissions.allow that match nothing,
	// DuplicateAllow those listed twice, and OrphanPlugins the
	// enabledPlugins whose marketplace is not known.
	DeadAllow      []string
	DuplicateAllow []string
	OrphanPlugins  []string
	// Err says why settings.json could not be read, "" when it was.
	Err string
}

// Options are the choices of a run.
type Options struct {
	// SetCleanupDays sets cleanupPeriodDays to CleanupDays.
	SetCleanupDays bool
	CleanupDays    int
	// DropDeadAllow drops the dead and duplicate allow entries, and
	// DropOrphanPlugins the orphan plugins.
	DropDeadAllow     bool
	DropOrphanPlugins bool
	// SignOut signs the profile out, last.
	SignOut bool
	// Force runs while Claude Code is running.
	Force bool
	// BackupDir is where the backup folder is made.
	BackupDir string
}

// NewPlan looks at the profile's folder.
func NewPlan(env *claude.Env, p *claude.Profile) *Plan {
	pl := &Plan{Profile: p, Running: env.Running()}
	for _, rel := range ArchiveTargets {
		if it := statItem(p.Dir, rel, Archive); it != nil {
			pl.Items = append(pl.Items, it)
		}
	}
	for _, rel := range PlainDeletes {
		if it := statItem(p.Dir, rel, Delete); it != nil {
			pl.Items = append(pl.Items, it)
		}
	}
	if it := telemetryItem(p.Dir); it != nil {
		pl.Items = append(pl.Items, it)
	}
	if p.Default {
		if it := strayItem(env, p); it != nil {
			pl.Items = append(pl.Items, it)
		}
	}
	for _, rel := range EmptyDeletes {
		path := filepath.Join(p.Dir, rel)
		if fi, err := os.Lstat(path); err == nil && fi.IsDir() {
			if ents, err := os.ReadDir(path); err == nil && len(ents) == 0 {
				pl.Items = append(pl.Items, &Item{Kind: DeleteEmpty, Rel: rel, Paths: []string{path}, Selected: true})
			}
		}
	}
	if ents, err := os.ReadDir(p.Dir); err == nil {
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "settings.json.") && e.Type().IsRegular() {
				if it := statItem(p.Dir, e.Name(), Delete); it != nil {
					pl.Variants = append(pl.Variants, it)
				}
			}
		}
	}
	pl.Settings = planSettings(p)
	pl.SignOut = env.PlanSignOut(p)
	return pl
}

// statItem is the item of rel, nil when it is not there or is a link: a
// run never removes through a link.
func statItem(dir, rel string, kind Kind) *Item {
	path := filepath.Join(dir, rel)
	fi, err := os.Lstat(path)
	if err != nil || fi.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	st := Walk(path)
	return &Item{Kind: kind, Rel: rel, Paths: []string{path}, Bytes: st.Bytes, Files: st.Files + st.Symlinks, Selected: true}
}

func telemetryItem(dir string) *Item {
	tel := filepath.Join(dir, "telemetry")
	ents, err := os.ReadDir(tel)
	if err != nil {
		return nil
	}
	it := &Item{Kind: Delete, Rel: "telemetry/" + TelemetryPrefix + "*", Selected: true}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), TelemetryPrefix) && e.Type().IsRegular() {
			path := filepath.Join(tel, e.Name())
			if fi, err := e.Info(); err == nil {
				it.Bytes += fi.Size()
			}
			it.Paths = append(it.Paths, path)
			it.Files++
		}
	}
	if it.Files == 0 {
		return nil
	}
	return it
}

func strayItem(env *claude.Env, p *claude.Profile) *Item {
	stray := filepath.Join(p.Dir, StrayConfig)
	inner := machineID(stray)
	if inner == "" && !regular(stray) {
		return nil
	}
	if inner != "" && inner == machineID(p.ConfigFile) {
		return nil
	}
	return statItem(p.Dir, StrayConfig, Delete)
}

func machineID(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var v struct {
		MachineID string `json:"machineID"`
	}
	json.Unmarshal(data, &v)
	return v.MachineID
}

func regular(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode().IsRegular()
}

// Stats are a tree's files, folders, links and bytes.
type Stats struct {
	Files, Dirs, Symlinks int
	Bytes                 int64
}

// Walk tallies the tree at root without following links. A folder it
// cannot read is skipped.
func Walk(root string) Stats {
	var s Stats
	fi, err := os.Lstat(root)
	if err != nil {
		return s
	}
	if !fi.IsDir() {
		if fi.Mode()&os.ModeSymlink != 0 {
			s.Symlinks = 1
		} else {
			s.Files, s.Bytes = 1, fi.Size()
		}
		return s
	}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			s.Symlinks++
		case d.IsDir():
			s.Dirs++
		case d.Type().IsRegular():
			s.Files++
			if fi, err := d.Info(); err == nil {
				s.Bytes += fi.Size()
			}
		}
		return nil
	})
	return s
}

// Reclaim is what the selected items and the variants take.
func (pl *Plan) Reclaim() int64 {
	var n int64
	for _, it := range pl.Items {
		if it.Selected {
			n += it.Bytes
		}
	}
	for _, v := range pl.Variants {
		n += v.Bytes
	}
	return n
}

// ArchiveBytes is what the selected archives take before compression.
func (pl *Plan) ArchiveBytes() int64 {
	var n int64
	for _, it := range pl.Items {
		if it.Selected && it.Kind == Archive {
			n += it.Bytes
		}
	}
	return n
}

// EstimatedBackup is the size the archives should come to.
func (pl *Plan) EstimatedBackup() int64 { return int64(float64(pl.ArchiveBytes()) * CompressionRatio) }

// SpaceNeeded is the free space a run asks for: the archives with
// headroom, and 512 MB at least.
func (pl *Plan) SpaceNeeded() int64 {
	return max(512<<20, int64(float64(pl.EstimatedBackup())*SpaceSafety))
}

// FreeSpace is the free space of the disk that holds dir, or of its
// nearest folder that exists.
func FreeSpace(dir string) int64 {
	for d := dir; ; d = filepath.Dir(d) {
		var st syscall.Statfs_t
		if err := syscall.Statfs(d, &st); err == nil {
			return int64(st.Bavail) * int64(st.Bsize)
		}
		if d == filepath.Dir(d) {
			return 0
		}
	}
}

func planSettings(p *claude.Profile) SettingsPlan {
	link := filepath.Join(p.Dir, "settings.json")
	sp := SettingsPlan{Path: link}
	if fi, err := os.Lstat(link); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		sp.Shared = true
		if real, err := filepath.EvalSymlinks(link); err == nil {
			sp.Path = real
		}
	}
	data, err := os.ReadFile(sp.Path)
	if os.IsNotExist(err) {
		sp.Path = ""
		return sp
	}
	if err != nil {
		sp.Err = err.Error()
		return sp
	}
	o, err := jsonedit.Parse(data)
	if err != nil {
		sp.Err = err.Error()
		return sp
	}
	var days json.Number
	if ok, _ := o.Get("cleanupPeriodDays", &days); ok {
		if n, err := days.Int64(); err == nil {
			sp.CleanupDays = int(n)
		}
	}
	var perms struct {
		Allow []json.RawMessage `json:"allow"`
	}
	o.Get("permissions", &perms)
	seen := map[string]bool{}
	for _, raw := range perms.Allow {
		var rule string
		if json.Unmarshal(raw, &rule) != nil {
			continue
		}
		switch {
		case seen[rule]:
			sp.DuplicateAllow = append(sp.DuplicateAllow, rule)
		case deadRule(rule, filepath.Dir(sp.Path)):
			sp.DeadAllow = append(sp.DeadAllow, rule)
		}
		seen[rule] = true
	}
	var plugins map[string]json.RawMessage
	o.Get("enabledPlugins", &plugins)
	if known := knownMarketplaces(p); known != nil {
		for key := range plugins {
			_, market, ok := strings.Cut(key, "@")
			if ok && !known[market] {
				sp.OrphanPlugins = append(sp.OrphanPlugins, key)
			}
		}
		sort.Strings(sp.OrphanPlugins)
	}
	return sp
}

// fileTools are the tools whose rules name paths.
var fileTools = []string{"Read", "Edit", "Write", "MultiEdit", "NotebookEdit", "Glob", "Grep"}

// deadRule reports whether rule, of a settings file in dir, names a path
// that is not there. A rule's path that starts with one slash is relative
// to the settings file's folder, as `Edit(/.claude/**)` is, so a rule
// written as an absolute path with one slash names a folder inside it,
// which is almost never there.
func deadRule(rule, dir string) bool {
	tool, rest, ok := strings.Cut(rule, "(")
	if !ok || !strings.HasSuffix(rest, ")") || !slices.Contains(fileTools, tool) {
		return false
	}
	pattern := strings.TrimSuffix(rest, ")")
	if !strings.HasPrefix(pattern, "/") || strings.HasPrefix(pattern, "//") {
		return false
	}
	prefix := pattern
	if i := strings.IndexAny(prefix, "*?[{"); i >= 0 {
		prefix = prefix[:i]
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return false
	}
	_, err := os.Lstat(filepath.Join(dir, prefix))
	return os.IsNotExist(err)
}

func knownMarketplaces(p *claude.Profile) map[string]bool {
	data, err := os.ReadFile(filepath.Join(p.Dir, "plugins", "known_marketplaces.json"))
	if err != nil {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	known := map[string]bool{}
	for k := range m {
		known[k] = true
	}
	return known
}
