package settings

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/jsonedit"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// home has ~/.claude with a settings.json, work linking it, and solo with
// a file of its own.
func home(t *testing.T) (*claude.Env, []*claude.Profile) {
	t.Helper()
	env := &claude.Env{Home: t.TempDir()}
	write(t, filepath.Join(env.MainDir(), Name), "{\n    \"model\": \"opus\",\n    \"permissions\": {\"allow\": [\"Bash(ls)\"]}\n}\n")
	if _, err := env.Create("work", []string{Name}); err != nil {
		t.Fatal(err)
	}
	env.Create("solo", nil)
	write(t, filepath.Join(env.DirFor("solo"), Name), `{"model": "haiku", "env": {"A": "1"}}`)
	all, _ := env.Discover()
	return env, all
}

func find(all []*claude.Profile, name string) *claude.Profile {
	for _, p := range all {
		if p.Name == name {
			return p
		}
	}
	return nil
}

func TestLoadShared(t *testing.T) {
	_, all := home(t)
	work := Load(find(all, "work"), all)
	if !work.Link || work.Err != nil || !slices.Equal(work.SharedWith, []string{"default"}) {
		t.Fatalf("work: %+v", work)
	}
	main := Load(find(all, "default"), all)
	if main.Link || !slices.Equal(main.SharedWith, []string{"work"}) {
		t.Errorf("default: link %v, shared with %v", main.Link, main.SharedWith)
	}
	solo := Load(find(all, "solo"), all)
	if solo.Link || len(solo.SharedWith) != 0 {
		t.Errorf("solo: %+v", solo)
	}
}

func TestSaveWritesThroughTheLink(t *testing.T) {
	env, all := home(t)
	f := Load(find(all, "work"), all)
	o := f.Object.Clone()
	o.Set("model", "sonnet")
	o.SetIn([]string{"permissions", "allow"}, []string{"Bash(ls)", "Read"})
	ch := Changes(f.Object, o)
	var lines []string
	for _, c := range ch {
		lines = append(lines, c.String())
	}
	if want := []string{`~ model: "opus" → "sonnet"`, `+ permissions.allow: "Read"`}; !slices.Equal(lines, want) {
		t.Fatalf("changes %q", lines)
	}
	backups := t.TempDir()
	if err := f.Save(o, false, backups); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(env.MainDir(), Name))
	if !strings.Contains(string(data), "\n    \"model\": \"sonnet\"") || !strings.HasSuffix(string(data), "}\n") {
		t.Errorf("written:\n%s", data)
	}
	if fi, _ := os.Lstat(f.Path); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced")
	}
	if b, _ := filepath.Glob(filepath.Join(backups, "work-*.json")); len(b) != 1 {
		t.Errorf("backups %v", b)
	}
}

func TestSaveRefusesAChangedFile(t *testing.T) {
	_, all := home(t)
	f := Load(find(all, "solo"), all)
	o := f.Object.Clone()
	o.Set("model", "opus")
	write(t, f.Real, `{"model": "haiku", "theme": "dark"}`)
	if err := f.Save(o, false, ""); !errors.Is(err, ErrChanged) {
		t.Fatalf("saved over a change: %v", err)
	}
	if err := f.Save(o, true, ""); err != nil {
		t.Fatal(err)
	}
	if again := Load(find(all, "solo"), all); again.Object.Has("theme") {
		t.Error("forced save kept the other change")
	}
}

func TestSaveMakesAFile(t *testing.T) {
	env := &claude.Env{Home: t.TempDir()}
	p, _ := env.Create("new", nil)
	f := Load(p, []*claude.Profile{p})
	if f.Exists || f.Err != nil || f.Object == nil {
		t.Fatalf("%+v", f)
	}
	o := f.Object.Clone()
	o.Set("cleanupPeriodDays", 90)
	if err := f.Save(o, false, ""); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(f.Path); string(data) != "{\n  \"cleanupPeriodDays\": 90\n}" {
		t.Errorf("%q", data)
	}
}

func TestUnreadable(t *testing.T) {
	_, all := home(t)
	solo := find(all, "solo")
	write(t, filepath.Join(solo.Dir, Name), `{"model": `)
	if f := Load(solo, all); f.Err == nil || f.Object != nil {
		t.Errorf("%+v", f)
	}
}

func TestDetach(t *testing.T) {
	_, all := home(t)
	work := find(all, "work")
	if err := Detach(work); err != nil {
		t.Fatal(err)
	}
	f := Load(work, all)
	if f.Link || len(f.SharedWith) != 0 || !f.Object.Has("model") {
		t.Fatalf("%+v", f)
	}
	o := f.Object.Clone()
	o.Set("model", "haiku")
	if err := f.Save(o, false, ""); err != nil {
		t.Fatal(err)
	}
	if main := Load(find(all, "default"), all); string(main.Object.Raw("model")) != `"opus"` {
		t.Errorf("~/.claude changed: %s", main.Object.Raw("model"))
	}
	if Detach(work) == nil {
		t.Error("detached a file")
	}
}

func TestCheck(t *testing.T) {
	o, _ := jsonedit.Parse([]byte(`{"model": 1, "cleanupPeriodDays": 30, "env": {"A": 1}, "permissions": {"allow": "Bash", "defaultMode": "plan"}, "tui": "x", "enabledPlugins": {"a@b": true}}`))
	problems, unknown := Check(o)
	want := []Problem{{"model", String}, {"env", StringMap}, {"permissions.allow", Strings}}
	if !slices.Equal(problems, want) {
		t.Errorf("problems %v", problems)
	}
	if !slices.Equal(unknown, []string{"tui"}) {
		t.Errorf("unknown %v", unknown)
	}
}

func TestCompareAndCopy(t *testing.T) {
	here, _ := jsonedit.Parse([]byte(`{"model": "haiku", "env": {"A": "1"}, "theme": "dark"}`))
	there, _ := jsonedit.Parse([]byte(`{"theme": "dark", "model": "opus", "hooks": {}}`))
	got := CompareKeys(here, there)
	want := []KeyDiff{{"model", true, true}, {"env", true, false}, {"hooks", false, true}}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	CopyKeys(there, here, []string{"model", "env", "hooks"})
	if len(CompareKeys(here, there)) != 0 {
		t.Errorf("after copying: %v", CompareKeys(here, there))
	}
}

func TestHooks(t *testing.T) {
	o, _ := jsonedit.Parse([]byte(`{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}], "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "prompt", "prompt": "check it"}]}]}}`))
	got := Hooks(o)
	want := []Hook{{"Stop", "", "command", "say done"}, {"PreToolUse", "Bash", "prompt", "check it"}}
	if !slices.Equal(got, want) {
		t.Errorf("%v", got)
	}
}
