package clean

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/jsonedit"
)

// Phase is a step of a run.
type Phase int

const (
	PhaseBackup Phase = iota
	PhaseArchive
	PhaseVerify
	PhaseSettings
	PhaseDelete
	PhaseSignOut
	PhaseDone
)

// Progress is what a run tells as it goes: its phase, and in it the item
// it is at, the n-th of total.
type Progress struct {
	Phase    Phase
	Item     string
	N, Total int
}

// Result is what a run did.
type Result struct {
	// Root is the backup folder.
	Root string
	// Removed are the items deleted, Freed their bytes.
	Removed []string
	Freed   int64
	// Edits are the changes to settings.json.
	Edits []string
	// SignedOut are what signing out removed.
	SignedOut []string
	// BackupBytes is the size of the backup folder, and After of the
	// profile's folder after the run.
	BackupBytes int64
	After       int64
	// Manifest is the path of MANIFEST.txt.
	Manifest string
}

// Errors a run stops with before it changes anything.
var (
	ErrRunning    = errors.New("Claude Code is running")
	ErrNoSpace    = errors.New("not enough free space for the archives")
	ErrVerify     = errors.New("an archive did not verify; nothing was deleted")
	ErrNoBackupTo = errors.New("no folder to back up to")
)

// BackupRoot is a new folder in dir for the backup of the profile.
func BackupRoot(dir string, p *claude.Profile, now time.Time) string {
	name := "claude-backup-"
	if !p.Default {
		name += p.Name + "-"
	}
	base := filepath.Join(dir, name+now.Format("2006-01-02"))
	if _, err := os.Lstat(base); os.IsNotExist(err) {
		return base
	}
	return base + "-" + now.Format("150405")
}

// Run backs up and cleans the profile as pl and opts say, telling its
// progress to report, which may be nil. ctx stops it, until it deletes.
func Run(ctx context.Context, env *claude.Env, pl *Plan, opts Options, report func(Progress)) (*Result, error) {
	if report == nil {
		report = func(Progress) {}
	}
	if len(env.Running()) > 0 && !opts.Force {
		return nil, ErrRunning
	}
	if opts.BackupDir == "" {
		return nil, ErrNoBackupTo
	}
	if FreeSpace(opts.BackupDir) < pl.SpaceNeeded() {
		return nil, ErrNoSpace
	}
	p := pl.Profile
	res := &Result{Root: BackupRoot(opts.BackupDir, p, time.Now())}
	if err := os.MkdirAll(res.Root, 0o700); err != nil {
		return nil, err
	}

	// 1. The configuration.
	report(Progress{Phase: PhaseBackup})
	copied, err := backupConfig(pl, res.Root)
	if err != nil {
		return res, fmt.Errorf("backing up the configuration: %w", err)
	}

	// 2. The archives, each tallied just before, so that what verifies is
	// what was there as it was archived.
	type archive struct {
		item *Item
		file string
		want Stats
	}
	var archives []archive
	var targets []*Item
	for _, it := range pl.Items {
		if it.Selected && it.Kind == Archive {
			targets = append(targets, it)
		}
	}
	for i, it := range targets {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		report(Progress{Phase: PhaseArchive, Item: it.Rel, N: i + 1, Total: len(targets)})
		src := filepath.Join(p.Dir, it.Rel)
		want := Walk(src)
		file := filepath.Join(res.Root, it.Rel+".tar.gz")
		if err := writeArchive(file, p.Dir, it.Rel); err != nil {
			return res, fmt.Errorf("archiving %s: %w", it.Rel, err)
		}
		archives = append(archives, archive{it, file, want})
	}

	// 3. Verify every archive, and stop on any that does not.
	var verified []string
	for i, a := range archives {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		report(Progress{Phase: PhaseVerify, Item: a.item.Rel, N: i + 1, Total: len(archives)})
		got, err := readArchive(a.file)
		if err != nil {
			return res, fmt.Errorf("%w: %s: %v", ErrVerify, filepath.Base(a.file), err)
		}
		if got.Files+got.Symlinks != a.want.Files+a.want.Symlinks || got.Bytes != a.want.Bytes {
			return res, fmt.Errorf("%w: %s holds %d files of %d bytes, the folder %d of %d",
				ErrVerify, filepath.Base(a.file), got.Files+got.Symlinks, got.Bytes, a.want.Files+a.want.Symlinks, a.want.Bytes)
		}
		verified = append(verified, fmt.Sprintf("%s  (%d files, verified)", filepath.Base(a.file), got.Files+got.Symlinks))
	}
	if err := ctx.Err(); err != nil {
		return res, err
	}

	// 4. settings.json, then the deletes. There is no stopping from here.
	report(Progress{Phase: PhaseSettings})
	edits, err := editSettings(pl.Settings, opts)
	if err != nil {
		return res, fmt.Errorf("editing settings.json: %w", err)
	}
	res.Edits = edits

	var deletes []*Item
	for _, it := range pl.Items {
		if it.Selected {
			deletes = append(deletes, it)
		}
	}
	deletes = append(deletes, pl.Variants...)
	for i, it := range deletes {
		report(Progress{Phase: PhaseDelete, Item: it.Rel, N: i + 1, Total: len(deletes)})
		for _, path := range it.Paths {
			if !inside(p.Dir, path) {
				return res, fmt.Errorf("%s is outside %s", path, p.Dir)
			}
			var err error
			if it.Kind == DeleteEmpty {
				err = os.Remove(path)
			} else {
				err = os.RemoveAll(path)
			}
			if err != nil {
				return res, err
			}
		}
		res.Freed += it.Bytes
		res.Removed = append(res.Removed, fmt.Sprintf("%s (%s)", it.Rel, Size(it.Bytes)))
	}

	// 5. Signing out, last.
	if opts.SignOut && !pl.SignOut.Empty() {
		report(Progress{Phase: PhaseSignOut})
		res.SignedOut, err = env.SignOut(p, pl.SignOut)
		if err != nil {
			return res, fmt.Errorf("signing out: %w", err)
		}
	}

	res.After = Walk(p.Dir).Bytes
	res.BackupBytes = Walk(res.Root).Bytes
	res.Manifest = filepath.Join(res.Root, "MANIFEST.txt")
	os.WriteFile(res.Manifest, []byte(manifest(pl, opts, res, copied, verified)), 0o600)
	report(Progress{Phase: PhaseDone})
	return res, nil
}

func inside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

// backupConfig copies the configuration out of the profile's folder, and
// returns what it copied, relative to root.
func backupConfig(pl *Plan, root string) ([]string, error) {
	p := pl.Profile
	cfg := filepath.Join(root, "config")
	items := [][2]string{
		{filepath.Join(p.Dir, "settings.json"), filepath.Join(cfg, "settings.json")},
		{p.ConfigFile, filepath.Join(cfg, "dot-claude.json")},
		{filepath.Join(p.Dir, "CLAUDE.md"), filepath.Join(cfg, "CLAUDE.md")},
		{filepath.Join(p.Dir, "statusline-command.js"), filepath.Join(cfg, "statusline-command.js")},
		{filepath.Join(p.Dir, "hooks"), filepath.Join(cfg, "hooks")},
		{filepath.Join(p.Dir, "plugins", "installed_plugins.json"), filepath.Join(cfg, "plugins", "installed_plugins.json")},
		{filepath.Join(p.Dir, "plugins", "known_marketplaces.json"), filepath.Join(cfg, "plugins", "known_marketplaces.json")},
		{filepath.Join(p.Dir, "backups"), filepath.Join(root, "claude-json-backups")},
	}
	if p.Default {
		items = append(items, [2]string{filepath.Join(p.Dir, StrayConfig), filepath.Join(cfg, "dot-claude-inner.json")})
	}
	for _, v := range pl.Variants {
		items = append(items, [2]string{v.Paths[0], filepath.Join(cfg, "settings-backups", v.Rel)})
	}
	var copied []string
	for _, it := range items {
		ok, err := copyTree(it[0], it[1])
		if err != nil {
			return copied, err
		}
		if ok {
			rel, _ := filepath.Rel(root, it[1])
			copied = append(copied, rel)
		}
	}
	// The skills stay where they are: a list of them is enough.
	if ents, err := os.ReadDir(filepath.Join(p.Dir, "skills")); err == nil {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		slices.Sort(names)
		text := fmt.Sprintf("skills kept in place at %s\ncount: %d\n\n%s\n", filepath.Join(p.Dir, "skills"), len(names), strings.Join(names, "\n"))
		if err := os.WriteFile(filepath.Join(root, "skills-manifest.txt"), []byte(text), 0o600); err != nil {
			return copied, err
		}
		copied = append(copied, "skills-manifest.txt")
	}
	return copied, nil
}

// copyTree copies src to dst, through a link at src itself but copying
// the links inside it as links, keeping the times. It reports whether
// there was anything to copy.
func copyTree(src, dst string) (bool, error) {
	fi, err := os.Stat(src)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !fi.IsDir() {
		return true, copyFile(src, dst, fi)
	}
	root := src
	if real, err := filepath.EvalSymlinks(src); err == nil {
		root = real
	}
	return true, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		target := filepath.Join(dst, rel)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.IsDir():
			return os.MkdirAll(target, 0o700)
		case d.Type().IsRegular():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return copyFile(path, target, info)
		}
		return nil
	})
}

func copyFile(src, dst string, fi os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}

// writeArchive writes dir/rel to file as tar -czf file -C dir rel does:
// its entries are named from rel, and links are kept as links.
func writeArchive(file, dir, rel string) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	walkErr := filepath.WalkDir(filepath.Join(dir, rel), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		link := ""
		if fi.Mode()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		} else if !fi.Mode().IsRegular() && !fi.IsDir() {
			return nil // sockets and the like
		}
		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		name, _ := filepath.Rel(dir, path)
		hdr.Name = filepath.ToSlash(name)
		if fi.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		n, err := io.Copy(tw, in)
		if err == nil && n != fi.Size() {
			err = fmt.Errorf("%s changed as it was archived", name)
		}
		return err
	})
	errs := []error{walkErr, tw.Close(), gz.Close(), f.Sync(), f.Close()}
	return errors.Join(errs...)
}

// readArchive reads an archive to its end, which checks the gzip stream's
// checksum, and tallies its entries.
func readArchive(file string) (Stats, error) {
	var s Stats
	f, err := os.Open(file)
	if err != nil {
		return s, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return s, err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return s, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			s.Dirs++
		case tar.TypeSymlink:
			s.Symlinks++
		case tar.TypeReg:
			n, err := io.Copy(io.Discard, tr)
			if err != nil {
				return s, err
			}
			s.Files++
			s.Bytes += n
		}
	}
	// The gzip trailer is checked as the reader reaches it.
	if _, err := io.Copy(io.Discard, gz); err != nil {
		return s, err
	}
	return s, gz.Close()
}

// editSettings makes the edits opts asks of settings.json, and returns
// them as lines of a diff.
func editSettings(sp SettingsPlan, opts Options) ([]string, error) {
	if sp.Path == "" || sp.Err != "" {
		return nil, nil
	}
	data, err := os.ReadFile(sp.Path)
	if err != nil {
		return nil, err
	}
	o, err := jsonedit.Parse(data)
	if err != nil {
		return nil, err
	}
	var changes []string
	if opts.SetCleanupDays && opts.CleanupDays > 0 && sp.CleanupDays != opts.CleanupDays {
		from := "unset"
		if sp.CleanupDays != 0 {
			from = fmt.Sprint(sp.CleanupDays)
		}
		o.Set("cleanupPeriodDays", opts.CleanupDays)
		changes = append(changes, fmt.Sprintf("cleanupPeriodDays: %s → %d", from, opts.CleanupDays))
	}
	if opts.DropDeadAllow && len(sp.DeadAllow)+len(sp.DuplicateAllow) > 0 {
		perms, err := jsonedit.Parse(o.Raw("permissions"))
		if err != nil {
			return nil, err
		}
		var allow []json.RawMessage
		perms.Get("allow", &allow)
		seen := map[string]bool{}
		kept := allow[:0]
		for _, raw := range allow {
			var rule string
			if json.Unmarshal(raw, &rule) == nil {
				if slices.Contains(sp.DeadAllow, rule) {
					changes = append(changes, fmt.Sprintf("permissions.allow: dropped %q", rule))
					continue
				}
				if seen[rule] {
					changes = append(changes, fmt.Sprintf("permissions.allow: removed duplicate %q", rule))
					continue
				}
				seen[rule] = true
			}
			kept = append(kept, raw)
		}
		perms.Set("allow", kept)
		raw, err := perms.Marshal("")
		if err != nil {
			return nil, err
		}
		o.SetRaw("permissions", raw)
	}
	if opts.DropOrphanPlugins && len(sp.OrphanPlugins) > 0 {
		plugins, err := jsonedit.Parse(o.Raw("enabledPlugins"))
		if err != nil {
			return nil, err
		}
		for _, k := range sp.OrphanPlugins {
			if plugins.Delete(k) {
				changes = append(changes, fmt.Sprintf("enabledPlugins: removed %q (marketplace no longer known)", k))
			}
		}
		raw, err := plugins.Marshal("")
		if err != nil {
			return nil, err
		}
		o.SetRaw("enabledPlugins", raw)
	}
	if len(changes) == 0 {
		return nil, nil
	}
	out, err := o.Marshal(jsonedit.Indent(data))
	if err != nil {
		return nil, err
	}
	return changes, claude.WriteFile(sp.Path, append(out, '\n'))
}

func manifest(pl *Plan, opts Options, res *Result, copied, verified []string) string {
	p := pl.Profile
	lines := []string{
		"CCGuanjia cleanup of " + p.Dir + " — " + time.Now().Format(time.RFC3339),
		"",
		"BACKUP ROOT: " + res.Root,
		"",
		fmt.Sprintf("REMOVED (%s raw):", Size(res.Freed)),
	}
	for _, r := range res.Removed {
		lines = append(lines, "  - "+r)
	}
	lines = append(lines, "", "CONFIG COPIED:")
	for _, c := range copied {
		lines = append(lines, "  - "+c)
	}
	lines = append(lines, "", "ARCHIVES:")
	for _, v := range verified {
		lines = append(lines, "  - "+v)
	}
	lines = append(lines, "", "RESTORE:")
	for _, it := range pl.Items {
		if it.Selected && it.Kind == Archive {
			lines = append(lines, fmt.Sprintf("  tar -xzf %q -C %q %s/<one folder>", filepath.Join(res.Root, it.Rel+".tar.gz"), p.Dir, it.Rel))
		}
	}
	lines = append(lines, "  config files: copy back from "+filepath.Join(res.Root, "config"), "", "SETTINGS EDITS APPLIED:")
	if len(res.Edits) == 0 {
		lines = append(lines, "  - (none)")
	}
	for _, e := range res.Edits {
		lines = append(lines, "  - "+e)
	}
	if opts.SignOut {
		lines = append(lines, "", "SIGNED OUT:")
		if len(res.SignedOut) == 0 {
			lines = append(lines, "  - (nothing to sign out of)")
		}
		for _, s := range res.SignedOut {
			lines = append(lines, "  - "+s)
		}
		lines = append(lines, "  the sign-in was NOT backed up: sign in again to restore it")
	}
	kept := []string{}
	for _, item := range []string{"skills", "plugins", "hooks", "CLAUDE.md", "history.jsonl"} {
		if st := Walk(filepath.Join(p.Dir, item)); st.Files+st.Symlinks+st.Dirs > 0 {
			kept = append(kept, fmt.Sprintf("%s (%s)", item, Size(st.Bytes)))
		}
	}
	lines = append(lines, "", "KEPT IN PLACE: "+strings.Join(kept, ", "), "",
		fmt.Sprintf("%s after: %s", p.Dir, Size(res.After)))
	return strings.Join(lines, "\n") + "\n"
}

// Size writes a number of bytes as clean-claude.ts does: 1.5 MB, 868 MB.
func Size(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n) / 1024
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if v < 10 {
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
	return fmt.Sprintf("%.0f %s", v, units[i])
}
