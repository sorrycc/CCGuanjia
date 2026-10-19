package clean

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ccguanjia/internal/claude"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// newHome is a home with a ~/.claude much like a real one.
func newHome(t *testing.T) *claude.Env {
	t.Helper()
	home := t.TempDir()
	d := filepath.Join(home, ".claude")
	write(t, filepath.Join(home, ".claude.json"), `{"machineID": "m1", "oauthAccount": {"accountUuid": "u", "emailAddress": "a@b.c"}, "userID": "x", "projects": {}}`)
	write(t, filepath.Join(d, "projects", "-Users-a-x", "s1.jsonl"), strings.Repeat("transcript\n", 100))
	write(t, filepath.Join(d, "projects", "-Users-a-y", "s2.jsonl"), "t")
	os.Symlink("s2.jsonl", filepath.Join(d, "projects", "-Users-a-y", "link"))
	write(t, filepath.Join(d, "file-history", "f", "v1"), "old")
	write(t, filepath.Join(d, "backups", ".claude.json.backup.1"), "{}")
	write(t, filepath.Join(d, "paste-cache", "a.png"), "png")
	write(t, filepath.Join(d, "shell-snapshots", "s.sh"), "sh")
	write(t, filepath.Join(d, "stats-cache.json"), "{}")
	write(t, filepath.Join(d, "telemetry", "1p_failed_events.1.json"), "e")
	write(t, filepath.Join(d, "telemetry", "keep.json"), "k")
	write(t, filepath.Join(d, ".claude.json"), `{"machineID": "other"}`)
	write(t, filepath.Join(d, "settings.json.old"), "{}")
	write(t, filepath.Join(d, "skills", "s", "SKILL.md"), "skill")
	write(t, filepath.Join(d, "CLAUDE.md"), "me")
	write(t, filepath.Join(d, "history.jsonl"), "h")
	write(t, filepath.Join(d, "hooks", "h.sh"), "hook")
	os.MkdirAll(filepath.Join(d, "teams"), 0o755)
	os.MkdirAll(filepath.Join(d, "downloads", "x"), 0o755)
	write(t, filepath.Join(d, "plugins", "known_marketplaces.json"), `{"official": {}}`)
	write(t, filepath.Join(d, "settings.json"), `{
    "cleanupPeriodDays": 30,
    "permissions": {
        "allow": ["Bash(ls)", "Edit(/.claude/**)", "Bash(ls)", "Read(/hooks/**)", "Edit(//abs/**)"],
        "deny": []
    },
    "enabledPlugins": {"a@official": true, "b@gone": true},
    "theme": "dark"
}
`)
	return &claude.Env{Home: home, Keychain: claude.FakeKeychain{"Claude Code-credentials": "a"}, Processes: func() []claude.Process { return nil }}
}

func rels(items []*Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Rel)
	}
	return out
}

func TestPlan(t *testing.T) {
	env := newHome(t)
	pl := NewPlan(env, env.Load(claude.DefaultName))
	want := []string{"projects", "file-history", "backups", "paste-cache", "shell-snapshots", "stats-cache.json", "telemetry/1p_failed_events.*", ".claude.json", "teams"}
	if got := rels(pl.Items); !slices.Equal(got, want) {
		t.Errorf("items %v\nwant %v", got, want)
	}
	if got := rels(pl.Variants); !slices.Equal(got, []string{"settings.json.old"}) {
		t.Errorf("variants %v", got)
	}
	if pl.Items[0].Files != 3 {
		t.Errorf("projects files %d", pl.Items[0].Files)
	}
	s := pl.Settings
	if s.CleanupDays != 30 || !slices.Equal(s.DeadAllow, []string{"Edit(/.claude/**)"}) ||
		!slices.Equal(s.DuplicateAllow, []string{"Bash(ls)"}) || !slices.Equal(s.OrphanPlugins, []string{"b@gone"}) {
		t.Errorf("settings %+v", s)
	}
	if !slices.Equal(pl.SignOut.Keys, []string{"oauthAccount", "userID"}) {
		t.Errorf("sign out %+v", pl.SignOut)
	}
}

func TestStrayConfigOfTheSameMachineStays(t *testing.T) {
	env := newHome(t)
	write(t, filepath.Join(env.MainDir(), ".claude.json"), `{"machineID": "m1"}`)
	pl := NewPlan(env, env.Load(claude.DefaultName))
	if slices.Contains(rels(pl.Items), ".claude.json") {
		t.Error("listed a .claude.json of this machine")
	}
	// In another profile, .claude.json is the profile's own config.
	os.MkdirAll(filepath.Join(env.Home, ".claude-work"), 0o700)
	write(t, filepath.Join(env.Home, ".claude-work", ".claude.json"), `{"machineID": "zzz"}`)
	if slices.Contains(rels(NewPlan(env, env.Load("work")).Items), ".claude.json") {
		t.Error("listed a profile's own .claude.json")
	}
}

func TestRun(t *testing.T) {
	env := newHome(t)
	p := env.Load(claude.DefaultName)
	pl := NewPlan(env, p)
	backups := t.TempDir()
	var phases []Phase
	res, err := Run(context.Background(), env, pl, Options{
		SetCleanupDays: true, CleanupDays: 365, DropDeadAllow: true, DropOrphanPlugins: true,
		SignOut: true, BackupDir: backups,
	}, func(pr Progress) {
		if len(phases) == 0 || phases[len(phases)-1] != pr.Phase {
			phases = append(phases, pr.Phase)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(phases, []Phase{PhaseBackup, PhaseArchive, PhaseVerify, PhaseSettings, PhaseDelete, PhaseSignOut, PhaseDone}) {
		t.Errorf("phases %v", phases)
	}
	if !strings.HasPrefix(filepath.Base(res.Root), "claude-backup-"+time.Now().Format("2006-01-02")) {
		t.Errorf("root %s", res.Root)
	}
	for _, rel := range []string{"projects", "file-history", "backups", "paste-cache", "stats-cache.json", "telemetry/1p_failed_events.1.json", ".claude.json", "teams", "settings.json.old"} {
		if _, err := os.Lstat(filepath.Join(p.Dir, rel)); err == nil {
			t.Errorf("%s still there", rel)
		}
	}
	for _, rel := range []string{"skills/s/SKILL.md", "CLAUDE.md", "history.jsonl", "hooks/h.sh", "telemetry/keep.json", "downloads/x"} {
		if _, err := os.Lstat(filepath.Join(p.Dir, rel)); err != nil {
			t.Errorf("%s removed", rel)
		}
	}
	for _, rel := range []string{"projects.tar.gz", "file-history.tar.gz", "config/settings.json", "config/dot-claude.json", "config/dot-claude-inner.json",
		"config/hooks/h.sh", "config/settings-backups/settings.json.old", "claude-json-backups/.claude.json.backup.1", "skills-manifest.txt", "MANIFEST.txt"} {
		if _, err := os.Stat(filepath.Join(res.Root, rel)); err != nil {
			t.Errorf("backup lacks %s", rel)
		}
	}
	st, err := readArchive(filepath.Join(res.Root, "projects.tar.gz"))
	if err != nil || st.Files != 2 || st.Symlinks != 1 {
		t.Errorf("projects archive %+v %v", st, err)
	}
	data, _ := os.ReadFile(filepath.Join(p.Dir, "settings.json"))
	text := string(data)
	if !strings.Contains(text, `"cleanupPeriodDays": 365`) || strings.Contains(text, "Edit(/.claude/**)") || strings.Count(text, "Bash(ls)") != 1 ||
		strings.Contains(text, "b@gone") || !strings.Contains(text, "Read(/hooks/**)") || !strings.HasPrefix(text, "{\n    \"cleanupPeriodDays\"") {
		t.Errorf("settings.json:\n%s", text)
	}
	if len(res.Edits) != 4 {
		t.Errorf("edits %v", res.Edits)
	}
	if q := env.Load(claude.DefaultName); q.SignedIn || q.Account != nil {
		t.Error("still signed in")
	}
	manifest, _ := os.ReadFile(res.Manifest)
	if !strings.Contains(string(manifest), "projects.tar.gz  (3 files, verified)") || !strings.Contains(string(manifest), "NOT backed up") {
		t.Errorf("manifest:\n%s", manifest)
	}
}

func TestRunStopsWhileClaudeRuns(t *testing.T) {
	env := newHome(t)
	env.Processes = func() []claude.Process { return []claude.Process{{PID: 1, Name: "claude"}} }
	pl := NewPlan(env, env.Load(claude.DefaultName))
	if _, err := Run(context.Background(), env, pl, Options{BackupDir: t.TempDir()}, nil); !errors.Is(err, ErrRunning) {
		t.Fatalf("err %v", err)
	}
	res, err := Run(context.Background(), env, pl, Options{BackupDir: t.TempDir(), Force: true}, nil)
	if err != nil || len(res.Removed) == 0 {
		t.Fatalf("forced: %v", err)
	}
}

// An archive that does not verify stops the run before it deletes or signs
// out.
func TestVerifyFailureDeletesNothing(t *testing.T) {
	env := newHome(t)
	p := env.Load(claude.DefaultName)
	pl := NewPlan(env, p)
	backups := t.TempDir()
	_, err := Run(context.Background(), env, pl, Options{SignOut: true, BackupDir: backups, SetCleanupDays: true, CleanupDays: 1},
		func(pr Progress) {
			if pr.Phase == PhaseVerify && pr.N == 1 {
				matches, _ := filepath.Glob(filepath.Join(backups, "*", "projects.tar.gz"))
				for _, m := range matches {
					data, _ := os.ReadFile(m)
					os.WriteFile(m, data[:len(data)/2], 0o600)
				}
			}
		})
	if !errors.Is(err, ErrVerify) {
		t.Fatalf("err %v", err)
	}
	for _, rel := range []string{"projects", "file-history", "paste-cache"} {
		if _, err := os.Stat(filepath.Join(p.Dir, rel)); err != nil {
			t.Errorf("%s deleted after a failed verify", rel)
		}
	}
	if q := env.Load(claude.DefaultName); !q.SignedIn {
		t.Error("signed out after a failed verify")
	}
	data, _ := os.ReadFile(filepath.Join(p.Dir, "settings.json"))
	if !strings.Contains(string(data), `"cleanupPeriodDays": 30`) {
		t.Error("settings edited after a failed verify")
	}
}

func TestUnselected(t *testing.T) {
	env := newHome(t)
	p := env.Load(claude.DefaultName)
	pl := NewPlan(env, p)
	for _, it := range pl.Items {
		it.Selected = it.Rel == "paste-cache"
	}
	res, err := Run(context.Background(), env, pl, Options{BackupDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(res.Root, "projects.tar.gz")); err == nil {
		t.Error("archived projects, unselected")
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "projects")); err != nil {
		t.Error("deleted projects, unselected")
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "paste-cache")); err == nil {
		t.Error("kept paste-cache")
	}
	if q := env.Load(claude.DefaultName); !q.SignedIn {
		t.Error("signed out, unasked")
	}
}

// A profile that links settings.json edits the file the link points to,
// and keeps the link.
func TestSharedSettings(t *testing.T) {
	env := newHome(t)
	work, err := env.Create("work", []string{"settings.json", "hooks"})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(work.Dir, "projects", "p", "s.jsonl"), "x")
	pl := NewPlan(env, work)
	want, _ := filepath.EvalSymlinks(filepath.Join(env.MainDir(), "settings.json"))
	if !pl.Settings.Shared || pl.Settings.Path != want {
		t.Fatalf("settings plan %+v", pl.Settings)
	}
	if _, err := Run(context.Background(), env, pl, Options{BackupDir: t.TempDir(), SetCleanupDays: true, CleanupDays: 90}, nil); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(filepath.Join(work.Dir, "settings.json")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced")
	}
	data, _ := os.ReadFile(filepath.Join(env.MainDir(), "settings.json"))
	if !strings.Contains(string(data), `"cleanupPeriodDays": 90`) {
		t.Error("shared settings not edited")
	}
	if _, err := os.Stat(filepath.Join(env.MainDir(), "hooks", "h.sh")); err != nil {
		t.Error("the shared hooks went")
	}
}

func TestDeadRule(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "hooks"), 0o755)
	for rule, dead := range map[string]bool{
		"Edit(/Users/a/.claude/**)": true,
		"Edit(/hooks/**)":           false,
		"Read(//etc/**)":            false,
		"Read(~/x/**)":              false,
		"Bash(/bin/ls)":             false,
		"Edit(/**)":                 false,
		"Edit":                      false,
	} {
		if got := deadRule(rule, dir); got != dead {
			t.Errorf("%s: dead %v", rule, got)
		}
	}
}

func TestSize(t *testing.T) {
	for n, want := range map[int64]string{500: "500 B", 1536: "1.5 KB", 868 << 20: "868 MB", 5 << 30: "5.0 GB"} {
		if got := Size(n); got != want {
			t.Errorf("%d: %s, want %s", n, got, want)
		}
	}
}
