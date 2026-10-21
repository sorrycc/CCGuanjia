package claude

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// newEnv is a home folder with ~/.claude holding the shared items, and
// ~/.claude.json signed in to a Max account.
func newEnv(t *testing.T) *Env {
	t.Helper()
	home := t.TempDir()
	main := filepath.Join(home, ".claude")
	for _, d := range []string{"skills", "plugins", "hooks", "commands"} {
		must(t, os.MkdirAll(filepath.Join(main, d), 0o755))
	}
	write(t, filepath.Join(main, "CLAUDE.md"), "# me")
	write(t, filepath.Join(main, "settings.json"), "{}")
	write(t, filepath.Join(home, ".claude.json"), `{"numStartups": 3, "oauthAccount": {"accountUuid": "u1", "emailAddress": "ada@example.com", "displayName": "Ada", "organizationName": "Ada's Org", "organizationType": "claude_max", "organizationRateLimitTier": "default_claude_max_20x"}, "userID": "x", "mcpServers": {"fs": {"command": "fs"}, "git": {"command": "git"}}}`)
	return &Env{Home: home, Keychain: FakeKeychain{"Claude Code-credentials": "ada"}}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	must(t, os.WriteFile(path, []byte(text), 0o600))
}

func TestKeychainService(t *testing.T) {
	if got := KeychainService("/Users/x/.claude", true); got != "Claude Code-credentials" {
		t.Errorf("default: %s", got)
	}
	// The suffix Claude Code 2.1 gives /Users/ada/.claude-work, as `printf %s /Users/ada/.claude-work | shasum -a 256` begins.
	if got := KeychainService("/Users/ada/.claude-work", false); got != "Claude Code-credentials-6a5b8464" {
		t.Errorf("profile: %s", got)
	}
}

func TestDiscoverAndAccount(t *testing.T) {
	e := newEnv(t)
	must(t, os.MkdirAll(filepath.Join(e.Home, ".claude-work"), 0o700))
	write(t, filepath.Join(e.Home, ".claude-notadir"), "")
	ps, err := e.Discover()
	must(t, err)
	var names []string
	for _, p := range ps {
		names = append(names, p.Name)
	}
	if !slices.Equal(names, []string{"default", "work"}) {
		t.Fatalf("profiles %v", names)
	}
	d := ps[0]
	if !d.Default || !d.SignedIn || d.Account == nil || d.Account.Plan != "Max 20x" || d.Account.Email != "ada@example.com" {
		t.Errorf("default profile %+v %+v", d, d.Account)
	}
	if w := ps[1]; w.SignedIn || w.Account != nil || w.ConfigFile != filepath.Join(w.Dir, ".claude.json") {
		t.Errorf("work profile %+v", w)
	}
}

func TestCreateLinkUnlink(t *testing.T) {
	e := newEnv(t)
	p, err := e.Create("work", SharedItems)
	must(t, err)
	// statusline-command.js and agents are not in ~/.claude, so are not linked.
	for _, l := range p.Links {
		want := Linked
		if !l.Source {
			want = Unlinked
		}
		if l.State != want {
			t.Errorf("%s: state %v, want %v", l.Item, l.State, want)
		}
	}
	if _, err := e.Create("work", nil); err == nil {
		t.Error("created twice")
	}
	if _, err := e.Create("Bad Name", nil); err == nil {
		t.Error("bad name")
	}
	must(t, e.Unlink(p, "skills"))
	if fi, err := os.Lstat(filepath.Join(p.Dir, "skills")); err == nil {
		t.Errorf("skills still there: %v", fi.Mode())
	}
	// A file of the profile's own is never replaced nor removed.
	write(t, filepath.Join(p.Dir, "skills"), "mine")
	if err := e.Link(p, "skills"); err == nil || !strings.Contains(err.Error(), "its own") {
		t.Errorf("linked over its own: %v", err)
	}
	if err := e.Unlink(p, "skills"); err == nil {
		t.Error("unlinked its own")
	}
	if got := e.Load("work").LinkOf("skills").State; got != Own {
		t.Errorf("skills state %v", got)
	}
	// A link elsewhere is told apart, and relinked.
	must(t, os.Remove(filepath.Join(p.Dir, "hooks")))
	must(t, os.Symlink(t.TempDir(), filepath.Join(p.Dir, "hooks")))
	if got := e.Load("work").LinkOf("hooks").State; got != LinkedElsewhere {
		t.Errorf("hooks state %v", got)
	}
	must(t, e.Link(p, "hooks"))
	if got := e.Load("work").LinkOf("hooks").State; got != Linked {
		t.Errorf("relinked hooks state %v", got)
	}
}

func TestCopyMCP(t *testing.T) {
	e := newEnv(t)
	p, err := e.Create("work", nil)
	must(t, err)
	write(t, p.ConfigFile, "{\n  \"theme\": \"dark\",\n  \"mcpServers\": {\n    \"git\": {\"command\": \"mine\"}\n  }\n}")
	added, err := CopyMCP(e.Load(DefaultName), p)
	must(t, err)
	if !slices.Equal(added, []string{"fs"}) {
		t.Errorf("added %v", added)
	}
	data, _ := os.ReadFile(p.ConfigFile)
	text := string(data)
	if !strings.Contains(text, `"command": "mine"`) || !strings.Contains(text, `"fs"`) || !strings.HasPrefix(text, "{\n  \"theme\"") {
		t.Errorf("config:\n%s", text)
	}
	if got := e.Load("work").MCPServers(); !slices.Equal(got, []string{"fs", "git"}) {
		t.Errorf("servers %v", got)
	}
	// To a profile without .claude.json yet.
	q, _ := e.Create("fresh", nil)
	added, err = CopyMCP(e.Load(DefaultName), q)
	must(t, err)
	if len(added) != 2 {
		t.Errorf("fresh: added %v", added)
	}
}

func TestDisableAutoUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	write(t, path, "{\n    \"model\": \"opus\",\n    \"env\": {\"FOO\": \"bar\"}\n}")
	must(t, DisableAutoUpdate(dir))
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.HasPrefix(text, "{\n    \"model\"") || !strings.Contains(text, `"FOO": "bar"`) || !strings.Contains(text, `"DISABLE_AUTOUPDATER": "1"`) {
		t.Errorf("settings:\n%s", text)
	}
	must(t, DisableAutoUpdate(dir))
	if again, _ := os.ReadFile(path); string(again) != text {
		t.Errorf("rewritten:\n%s", again)
	}
	// Without settings.json yet.
	fresh := t.TempDir()
	must(t, DisableAutoUpdate(fresh))
	if data, _ := os.ReadFile(filepath.Join(fresh, "settings.json")); !strings.Contains(string(data), `"DISABLE_AUTOUPDATER": "1"`) {
		t.Errorf("fresh:\n%s", data)
	}
}

func TestSignOut(t *testing.T) {
	e := newEnv(t)
	p := e.Load(DefaultName)
	s := e.PlanSignOut(p)
	if s.KeychainService != "Claude Code-credentials" || s.KeychainAccount != "ada" || !slices.Equal(s.Keys, []string{"oauthAccount", "userID"}) {
		t.Fatalf("plan %+v", s)
	}
	removed, err := e.SignOut(p, s)
	must(t, err)
	if len(removed) != 3 {
		t.Errorf("removed %v", removed)
	}
	q := e.Load(DefaultName)
	if q.SignedIn || q.Account != nil {
		t.Errorf("still signed in: %+v", q)
	}
	data, _ := os.ReadFile(p.ConfigFile)
	if !strings.Contains(string(data), `"numStartups": 3`) || !strings.Contains(string(data), "mcpServers") {
		t.Errorf("config lost keys: %s", data)
	}
	if !e.PlanSignOut(q).Empty() {
		t.Error("plan not empty after sign out")
	}
}

func TestRemove(t *testing.T) {
	e := newEnv(t)
	p, _ := e.Create("work", []string{"skills"})
	e.Keychain.(FakeKeychain)[p.KeychainService] = "ada"
	must(t, e.Remove(p))
	if p.Exists() {
		t.Error("folder still there")
	}
	if _, ok := e.Keychain.(FakeKeychain)[p.KeychainService]; ok {
		t.Error("Keychain item still there")
	}
	// The shared item it linked to stays.
	if _, err := os.Stat(filepath.Join(e.MainDir(), "skills")); err != nil {
		t.Error("removed the shared skills")
	}
	if err := e.Remove(e.Load(DefaultName)); err == nil {
		t.Error("removed ~/.claude")
	}
}

func TestParsePS(t *testing.T) {
	got := parsePS("  1 /sbin/launchd\n 42 claude\n 43 /Applications/Claude.app/Contents/MacOS/Claude\n 44 /Users/a/.local/share/claude/versions/2.1.3\n 45 claude-helper\n")
	var pids []int
	for _, p := range got {
		pids = append(pids, p.PID)
	}
	if !slices.Equal(pids, []int{42, 43, 44}) {
		t.Errorf("pids %v", pids)
	}
}
