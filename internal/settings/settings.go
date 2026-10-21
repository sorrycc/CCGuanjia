// Package settings reads and writes a profile's settings.json, the user
// settings of Claude Code in its config folder. A profile's file is often a
// link to ~/.claude's, so the package says which profiles share it. A
// change is made to a copy of the file's object, shown as lines of a diff,
// and written only when the file is still as it was read, after a backup:
// Claude Code writes the file too, from /config and /permissions.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/jsonedit"
)

// Name is the file's name in a config folder.
const Name = "settings.json"

// Resolve returns the settings.json of a config folder, where it is
// written, and whether it is a link, which makes the file it points to the
// one written.
func Resolve(dir string) (path, real string, link bool) {
	path = filepath.Join(dir, Name)
	real = path
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		link = true
		if r, err := filepath.EvalSymlinks(path); err == nil {
			real = r
		}
	}
	return path, real, link
}

// File is a profile's settings.json as it was read.
type File struct {
	Profile string
	// Path is the file in the profile's folder, Real where it is written.
	Path, Real string
	// Link reports whether Path is a link.
	Link bool
	// SharedWith are the other profiles whose settings.json is the same
	// file, which a change reaches too.
	SharedWith []string
	// Exists reports whether there is a file; a change makes one.
	Exists bool
	// Data is the file as it was read, which a save checks it still is.
	Data []byte
	// Object is the file's object, an empty one when there is no file, and
	// nil when it could not be read, as Err says.
	Object *jsonedit.Object
	Err    error
}

// Load reads the settings.json of p, and finds the profiles of all that
// share it.
func Load(p *claude.Profile, all []*claude.Profile) *File {
	f := &File{Profile: p.Name}
	f.Path, f.Real, f.Link = Resolve(p.Dir)
	for _, q := range all {
		if q.Name == p.Name {
			continue
		}
		if _, real, _ := Resolve(q.Dir); sameFile(real, f.Real) {
			f.SharedWith = append(f.SharedWith, q.Name)
		}
	}
	data, err := os.ReadFile(f.Real)
	switch {
	case errors.Is(err, os.ErrNotExist):
		f.Object = jsonedit.New()
	case err != nil:
		f.Err = err
	default:
		f.Exists, f.Data = true, data
		if len(bytes.TrimSpace(data)) == 0 {
			f.Object = jsonedit.New()
		} else if f.Object, err = jsonedit.Parse(data); err != nil {
			f.Object, f.Err = nil, err
		}
	}
	return f
}

func sameFile(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	fa, err1 := os.Stat(a)
	fb, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(fa, fb)
}

// ErrChanged is the error of saving a file that changed since it was read.
var ErrChanged = errors.New("settings.json changed since it was read")

// Save writes o in place of the file, with the file's indentation, after
// copying the file as it is now into backupDir, when that is not "". It
// returns ErrChanged when the file is not as it was read, unless force.
func (f *File) Save(o *jsonedit.Object, force bool, backupDir string) error {
	now, err := os.ReadFile(f.Real)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	exists := err == nil
	if !force && (exists != f.Exists || !bytes.Equal(now, f.Data)) {
		return ErrChanged
	}
	if exists && backupDir != "" {
		if err := backup(backupDir, f.Profile, now); err != nil {
			return fmt.Errorf("backing up settings.json: %w", err)
		}
	}
	indent := "  "
	if f.Data != nil {
		indent = jsonedit.Indent(f.Data)
	}
	out, err := o.Marshal(indent)
	if err != nil {
		return err
	}
	if bytes.HasSuffix(f.Data, []byte("\n")) {
		out = append(out, '\n')
	}
	return claude.WriteFile(f.Path, out)
}

// keepBackups is how many backups of a profile's settings.json are kept.
const keepBackups = 20

// backup writes data into dir as <profile>-<time>.json, and removes the
// profile's oldest backups past keepBackups.
func backup(dir, profile string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := profile + "-" + time.Now().Format("20060102-150405.000") + ".json"
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		return err
	}
	old, _ := filepath.Glob(filepath.Join(dir, profile+"-*.json"))
	// The names sort by time, as the time is written with its zeros.
	sort.Strings(old)
	for len(old) > keepBackups {
		os.Remove(old[0])
		old = old[1:]
	}
	return nil
}

// Detach makes the settings.json that p links to a file of p's own, a
// copy of the one it links to, so that a change to it is p's alone.
func Detach(p *claude.Profile) error {
	path, real, link := Resolve(p.Dir)
	if !link {
		return errors.New("settings.json is not a link")
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(real); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(p.Dir, ".settings.json.ccguanjia-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	// Renaming over a link replaces the link, not the file it points to.
	return os.Rename(tmp.Name(), path)
}

// Change is a line of what an edit does to settings.json.
type Change struct {
	// Path is the key, with the keys of the objects it is in before it.
	Path []string
	// Op is '+' for a value added, '-' for one removed, '~' for one
	// changed. A list of text changes as items added and removed.
	Op       byte
	Old, New string
}

// String writes the change as a line of a diff.
func (c Change) String() string {
	key := strings.Join(c.Path, ".")
	switch c.Op {
	case '+':
		return "+ " + key + ": " + c.New
	case '-':
		return "− " + key + ": " + c.Old
	}
	return "~ " + key + ": " + c.Old + " → " + c.New
}

// Changes are what makes after of before, key by key, in the order of the
// keys of before and then of those after adds.
func Changes(before, after *jsonedit.Object) []Change {
	return changes(nil, before, after)
}

func changes(path []string, before, after *jsonedit.Object) []Change {
	var out []Change
	keys := before.Keys()
	for _, k := range after.Keys() {
		if !before.Has(k) {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		p := append(slices.Clone(path), k)
		a, b := before.Raw(k), after.Raw(k)
		switch {
		case b == nil:
			out = append(out, Change{Path: p, Op: '-', Old: Short(a)})
		case a == nil:
			out = append(out, Change{Path: p, Op: '+', New: Short(b)})
		case compact(a) == compact(b):
		default:
			out = append(out, valueChanges(p, a, b)...)
		}
	}
	return out
}

// valueChanges are the changes of a value that is not the same: those of
// its keys when it is an object before and after, those of its items when
// it is a list of text, else the value.
func valueChanges(path []string, a, b json.RawMessage) []Change {
	if oa, err := jsonedit.Parse(a); err == nil {
		if ob, err := jsonedit.Parse(b); err == nil {
			return changes(path, oa, ob)
		}
	}
	var la, lb []string
	if json.Unmarshal(a, &la) == nil && json.Unmarshal(b, &lb) == nil && la != nil && lb != nil {
		var out []Change
		for _, s := range la {
			if !slices.Contains(lb, s) {
				out = append(out, Change{Path: path, Op: '-', Old: quote(s)})
			}
		}
		for _, s := range lb {
			if !slices.Contains(la, s) {
				out = append(out, Change{Path: path, Op: '+', New: quote(s)})
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []Change{{Path: path, Op: '~', Old: Short(a), New: Short(b)}}
}

func compact(raw json.RawMessage) string {
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return string(raw)
	}
	return b.String()
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// shortLen is how long a value of a diff is written at most.
const shortLen = 80

// Short writes a value on one line, cut at shortLen runes.
func Short(raw json.RawMessage) string {
	s := compact(raw)
	if r := []rune(s); len(r) > shortLen {
		return string(r[:shortLen-1]) + "…"
	}
	return s
}

// Kind is the type a key of settings.json takes.
type Kind int

const (
	String Kind = iota
	Number
	Bool
	Object
	// Strings is a list of text, StringMap an object of text, BoolMap an
	// object of true and false.
	Strings
	StringMap
	BoolMap
)

// kinds are the keys whose types Check knows, from Claude Code's settings
// documentation. The others are kept as they are, unchecked.
var kinds = map[string]Kind{
	"apiKeyHelper":               String,
	"autoUpdatesChannel":         String,
	"effortLevel":                String,
	"forceLoginMethod":           String,
	"forceLoginOrgUUID":          String,
	"language":                   String,
	"model":                      String,
	"outputStyle":                String,
	"preferredNotifChannel":      String,
	"theme":                      String,
	"cleanupPeriodDays":          Number,
	"alwaysThinkingEnabled":      Bool,
	"autoCompactEnabled":         Bool,
	"disableAllHooks":            Bool,
	"enableAllProjectMcpServers": Bool,
	"includeCoAuthoredBy":        Bool,
	"spinnerTipsEnabled":         Bool,
	"verbose":                    Bool,
	"attribution":                Object,
	"extraKnownMarketplaces":     Object,
	"hooks":                      Object,
	"permissions":                Object,
	"sandbox":                    Object,
	"statusLine":                 Object,
	"companyAnnouncements":       Strings,
	"disabledMcpjsonServers":     Strings,
	"enabledMcpjsonServers":      Strings,
	"env":                        StringMap,
	"enabledPlugins":             BoolMap,
}

// permissionKinds are those of the keys of permissions.
var permissionKinds = map[string]Kind{
	"allow":                        Strings,
	"ask":                          Strings,
	"deny":                         Strings,
	"additionalDirectories":        Strings,
	"defaultMode":                  String,
	"disableBypassPermissionsMode": String,
}

// Problem is a key whose value is not of the type Claude Code takes.
type Problem struct {
	Path string
	Want Kind
}

// Check finds the known keys of o whose values are not of their types, and
// returns the keys it does not know, which it leaves unchecked.
func Check(o *jsonedit.Object) (problems []Problem, unknown []string) {
	for _, k := range o.Keys() {
		kind, ok := kinds[k]
		if !ok {
			unknown = append(unknown, k)
			continue
		}
		raw := o.Raw(k)
		if !is(raw, kind) {
			problems = append(problems, Problem{Path: k, Want: kind})
			continue
		}
		if k == "permissions" {
			perms, _ := jsonedit.Parse(raw)
			for _, pk := range perms.Keys() {
				if kind, ok := permissionKinds[pk]; ok && !is(perms.Raw(pk), kind) {
					problems = append(problems, Problem{Path: k + "." + pk, Want: kind})
				}
			}
		}
	}
	return problems, unknown
}

func is(raw json.RawMessage, kind Kind) bool {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return false
	}
	switch kind {
	case String:
		_, ok := v.(string)
		return ok
	case Number:
		_, ok := v.(float64)
		return ok
	case Bool:
		_, ok := v.(bool)
		return ok
	case Object:
		_, ok := v.(map[string]any)
		return ok
	case Strings:
		l, ok := v.([]any)
		for _, x := range l {
			if _, s := x.(string); !s {
				return false
			}
		}
		return ok
	case StringMap, BoolMap:
		m, ok := v.(map[string]any)
		for _, x := range m {
			var good bool
			if kind == StringMap {
				_, good = x.(string)
			} else {
				_, good = x.(bool)
			}
			if !good {
				return false
			}
		}
		return ok
	}
	return true
}

// KeyDiff is a top-level key that two files do not have alike.
type KeyDiff struct {
	Key string
	// Here and There report which of the two has it; both do when its
	// values differ.
	Here, There bool
}

// CompareKeys returns the keys of here and there that differ, in the
// order of here's keys and then of there's.
func CompareKeys(here, there *jsonedit.Object) []KeyDiff {
	var out []KeyDiff
	for _, k := range here.Keys() {
		switch {
		case !there.Has(k):
			out = append(out, KeyDiff{Key: k, Here: true})
		case compact(here.Raw(k)) != compact(there.Raw(k)):
			out = append(out, KeyDiff{Key: k, Here: true, There: true})
		}
	}
	for _, k := range there.Keys() {
		if !here.Has(k) {
			out = append(out, KeyDiff{Key: k, There: true})
		}
	}
	return out
}

// CopyKeys gives to the values keys have in from, removing those from does
// not have.
func CopyKeys(from, to *jsonedit.Object, keys []string) {
	for _, k := range keys {
		if raw := from.Raw(k); raw != nil {
			to.SetRaw(k, raw)
		} else {
			to.Delete(k)
		}
	}
}

// Hook is a command a hook runs.
type Hook struct {
	Event, Matcher string
	// Type is command, prompt or another, and Command what it runs or asks.
	Type, Command string
}

// Hooks lists the hooks of o, by event in the order of the file.
func Hooks(o *jsonedit.Object) []Hook {
	events, err := jsonedit.Parse(o.Raw("hooks"))
	if o.Raw("hooks") == nil || err != nil {
		return nil
	}
	var out []Hook
	for _, ev := range events.Keys() {
		var groups []struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Prompt  string `json:"prompt"`
			} `json:"hooks"`
		}
		json.Unmarshal(events.Raw(ev), &groups)
		for _, g := range groups {
			for _, h := range g.Hooks {
				cmd := h.Command
				if cmd == "" {
					cmd = h.Prompt
				}
				out = append(out, Hook{Event: ev, Matcher: g.Matcher, Type: h.Type, Command: cmd})
			}
		}
	}
	return out
}
