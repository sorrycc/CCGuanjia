package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/install"
	"ccguanjia/internal/usage"
	"ccguanjia/locales"
)

func write(t testing.TB, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// newTester runs a view as the desktop asking for less motion, so that
// what the clock of the tester, which stands still, would leave half way
// in is where it goes at once.
func newTester(view func(*ui.Context), w, h int) *ui.Tester {
	tt := ui.NewTester(view, w, h)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Advance(time.Second)
	return tt
}

var fixtureNow = time.Date(2026, 10, 19, 16, 0, 0, 0, time.Local)

// fixture is a home of made-up profiles: ~/.claude signed in to a Max
// account, a work profile linking the shared items, and a profile signed
// out, with a week of transcripts.
func fixture(t testing.TB) *app {
	t.Helper()
	home := t.TempDir()
	main := filepath.Join(home, ".claude")
	write(t, filepath.Join(home, ".claude.json"), `{"machineID": "m", "userID": "u", "oauthAccount": {"accountUuid": "1", "emailAddress": "ada@example.com", "displayName": "Ada", "organizationName": "Ada's Studio", "organizationType": "claude_max", "organizationRateLimitTier": "default_claude_max_20x"}, "mcpServers": {"github": {"command": "gh"}, "files": {"command": "fs"}}}`)
	for _, f := range []string{"skills/review/SKILL.md", "hooks/notify.sh", "commands/ship.md", "plugins/known_marketplaces.json"} {
		write(t, filepath.Join(main, f), "{}")
	}
	write(t, filepath.Join(main, "CLAUDE.md"), "# Ada")
	write(t, filepath.Join(main, "settings.json"), `{"cleanupPeriodDays": 30, "permissions": {"allow": ["Bash(ls)", "Edit(/Users/ada/.claude/**)", "Bash(ls)"]}, "enabledPlugins": {"old@gone": true}}`)
	write(t, filepath.Join(main, "paste-cache", "a.png"), strings.Repeat("x", 300_000))
	write(t, filepath.Join(main, "file-history", "s", "f@v1"), strings.Repeat("x", 900_000))
	env := &claude.Env{Home: home, Keychain: claude.FakeKeychain{}, Processes: func() []claude.Process { return nil }}
	env.Keychain.(claude.FakeKeychain)["Claude Code-credentials"] = "ada"
	work, err := env.Create("work", claude.SharedItems)
	if err != nil {
		t.Fatal(err)
	}
	env.Keychain.(claude.FakeKeychain)[work.KeychainService] = "ada"
	write(t, work.ConfigFile, `{"oauthAccount": {"accountUuid": "2", "emailAddress": "ada@studio.example", "displayName": "Ada", "organizationName": "Studio", "organizationType": "claude_team"}}`)
	env.Create("side", nil)

	projects := []string{"/Users/ada/Projects/atlas", "/Users/ada/Projects/notes", "/Users/ada/Projects/web", "/Users/ada/Projects/ios", "/Users/ada/dotfiles"}
	models := []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"}
	for d, dir := range []string{main, work.Dir} {
		for i, proj := range projects {
			var b strings.Builder
			for h := 0; h < 7*24; h += 1 + (h*7+i)%5 {
				at := fixtureNow.Add(-time.Duration(h)*time.Hour - time.Duration(i*7)*time.Minute)
				n := 1 + (h*13+i*5)%9
				for k := 0; k < n; k++ {
					fmt.Fprintf(&b, `{"type":"assistant","timestamp":%q,"requestId":"r%d-%d-%d-%d","sessionId":"s%d-%d","cwd":%q,"message":{"id":"m%d-%d-%d-%d","model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d}}}`+"\n",
						at.UTC().Format(time.RFC3339), d, i, h, k, i, h/6, proj, d, i, h, k, models[(i+k)%3], 40+k*7, 300+h%400, 1200, 18000+h*30)
				}
			}
			name := strings.ReplaceAll(proj, "/", "-")
			write(t, filepath.Join(dir, "projects", name, "session.jsonl"), b.String()+strings.Repeat(" ", 40_000*(5-i)))
		}
	}
	for _, f := range []string{
		filepath.Join(main, "projects"),
		filepath.Join(work.Dir, "projects"),
	} {
		filepath.Walk(f, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				os.Chtimes(path, fixtureNow, fixtureNow)
			}
			return nil
		})
	}

	a := newApp(env, filepath.Join(home, ".config", "ccguanjia"), "/Applications/CCGuanjia.app/Contents/MacOS/CCGuanjia")
	a.sync = true
	a.now = func() time.Time { return fixtureNow }
	vdir := install.VersionsDir(home)
	a.detect = func() (*install.Info, string, string) {
		return &install.Info{
			Installs: []install.Install{{Path: filepath.Join(home, ".local", "bin", "claude"), Target: filepath.Join(vdir, "2.1.302"), Kind: install.Native, Version: "2.1.302"}},
			Versions: []install.Version{
				{Name: "2.1.302", Path: filepath.Join(vdir, "2.1.302"), Bytes: 268 << 20, Current: true},
				{Name: "2.1.289", Path: filepath.Join(vdir, "2.1.289"), Bytes: 266 << 20, InUse: true},
				{Name: "2.1.288", Path: filepath.Join(vdir, "2.1.288"), Bytes: 266 << 20},
			},
		}, "", "2.1.303"
	}
	a.refresh()
	return a
}

func TestOverview(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 940)
	for _, want := range []string{locales.PageOverview(), "Max 20x", locales.CardTokens(), "2.1.302", locales.UpdateAvailable(), locales.StorageByType(), "claude-opus-5-5", "work", "side"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if a.summary == nil || a.summary.Replies == 0 {
		t.Fatalf("summary %+v", a.summary)
	}
	week := a.summary.Tokens.Total()
	if err := tt.Click(locales.Range24h()); err != nil {
		t.Fatal(err)
	}
	if a.rangeIdx != 0 || a.summary.Tokens.Total() >= week {
		t.Errorf("24h: range %d, tokens %d of the week's %d", a.rangeIdx, a.summary.Tokens.Total(), week)
	}
}

func TestProfilePage(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 940)
	if err := tt.Click("work"); err != nil {
		t.Fatal(err)
	}
	if a.page != pageProfile || a.current != "work" {
		t.Fatalf("page %s %s", a.page, a.current)
	}
	for _, want := range []string{"ada@studio.example", "Team", "cc-work", "skills", locales.LinkLinked()} {
		if !tt.HasText(want) {
			t.Errorf("no %q", want)
		}
	}
	// Unlinking skills removes the link, and the switch turns it back.
	if err := tt.Click("skills"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.profile("work").LinkOf("skills").State != claude.Unlinked {
		t.Fatal("skills still linked")
	}
	tt.Click("skills")
	tt.Frame()
	if a.profile("work").LinkOf("skills").State != claude.Linked {
		t.Fatal("skills not linked again")
	}
	// Skipping permissions goes to the config and the shell file.
	if err := tt.Click(locales.SkipPermissions()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	data, _ := os.ReadFile(filepath.Join(a.cfgDir, "shell.zsh"))
	if !strings.Contains(string(data), `.claude-work" command env`) || !strings.Contains(string(data), "--dangerously-skip-permissions") {
		t.Errorf("shell.zsh:\n%s", data)
	}
	// MCP servers come over from ~/.claude.
	if err := tt.Click(locales.CopyServers()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.profile("work").MCPServers(); len(got) != 2 {
		t.Errorf("servers %v", got)
	}
}

func TestNewAndRemoveProfile(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 940)
	if err := tt.Click(locales.NewProfile()); err != nil {
		t.Fatal(err)
	}
	tt.Type("Bad Name")
	tt.Click(locales.Create())
	if a.newErr == "" || !a.newOpen {
		t.Fatal("made a profile of a bad name")
	}
	a.newName = "client"
	tt.Frame()
	tt.Click(locales.Create())
	tt.Frame()
	pr := a.profile("client")
	if pr == nil || a.newOpen || a.current != "client" {
		t.Fatalf("not created: %v %v", pr, a.newOpen)
	}
	if pr.LinkOf("skills").State != claude.Linked {
		t.Error("skills not linked")
	}
	if err := tt.Click(locales.RemoveProfileButton()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEnter) // the default button is Cancel
	tt.Frame()
	if !pr.Exists() {
		t.Fatal("removed on Cancel")
	}
	tt.Click(locales.RemoveProfileButton())
	tt.Frame()
	if err := tt.Click(locales.RemoveProfile()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if pr.Exists() || a.profile("client") != nil {
		t.Fatal("not removed")
	}
}

func TestCleanup(t *testing.T) {
	a := fixture(t)
	a.cfg.BackupDir = t.TempDir()
	tt := newTester(a.view, 1320, 1400)
	tt.Click(locales.PageCleanup())
	tt.Frame()
	if a.plans[claude.DefaultName] == nil {
		t.Fatal("no plan")
	}
	for _, want := range []string{"projects", "file-history", "paste-cache", "Edit(/Users/ada/.claude/**)", "enabledPlugins: − old@gone"} {
		if !tt.HasText(want) {
			t.Errorf("no %q", want)
		}
	}
	if err := tt.Click(locales.BackUpAndCleanButton()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.cleanQ {
		t.Fatal("no question")
	}
	if err := tt.Click(locales.BackUpAndClean()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	run := a.cleanRun
	if run == nil || !run.done || run.err != nil {
		t.Fatalf("run %+v", run)
	}
	if !tt.HasText(locales.CleanDone()) {
		t.Errorf("no result in %q", tt.Texts())
	}
	if _, err := os.Stat(filepath.Join(a.env.MainDir(), "projects")); err == nil {
		t.Error("projects still there")
	}
	if _, err := os.Stat(filepath.Join(run.result.Root, "projects.tar.gz")); err != nil {
		t.Error("no archive")
	}
	if !a.profile(claude.DefaultName).SignedIn {
		t.Error("signed out, unasked")
	}
}

func TestSignOutNeedsTheName(t *testing.T) {
	a := fixture(t)
	a.cfg.BackupDir = t.TempDir()
	a.show(pageCleanup, "work")
	tt := newTester(a.view, 1320, 1400)
	if err := tt.Click(locales.AlsoSignOut()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Click(locales.BackUpAndCleanButton())
	tt.Frame()
	if a.cleanQ {
		t.Fatal("asked without the name typed")
	}
	a.typed = "work"
	tt.Frame()
	tt.Click(locales.BackUpAndCleanButton())
	tt.Frame()
	tt.Click(locales.BackUpAndClean())
	tt.Frame()
	if a.cleanRun == nil || a.cleanRun.err != nil {
		t.Fatalf("run %+v", a.cleanRun)
	}
	if a.profile("work").SignedIn {
		t.Error("still signed in")
	}
}

func TestInstallPage(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 940)
	tt.Click(locales.PageInstall())
	tt.Frame()
	for _, want := range []string{locales.UpdateTo("2.1.303"), "2.1.288", locales.Current(), locales.InUse(), locales.RemoveVersions(1, "266 MB")} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
}

func TestSettingsPage(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 940)
	a.show(pageSettings, "")
	tt.Frame()
	if err := tt.Click(locales.ShellFile()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.shellInstalled {
		t.Fatal("shell file not on")
	}
	data, _ := os.ReadFile(filepath.Join(a.env.Home, ".zshrc"))
	if !strings.Contains(string(data), "source") {
		t.Errorf(".zshrc: %s", data)
	}
	if err := tt.Click(locales.CommandLine()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.cliInstalled {
		t.Fatal("command not installed")
	}
	script, _ := os.ReadFile(filepath.Join(a.env.Home, ".local", "bin", "ccgj"))
	if !strings.Contains(string(script), "'/Applications/CCGuanjia.app/Contents/MacOS/CCGuanjia' cli \"$@\"") {
		t.Errorf("ccgj: %s", script)
	}
}

func TestCLI(t *testing.T) {
	a := fixture(t)
	run := func(args ...string) (string, int) {
		var out, errOut bytes.Buffer
		code := runCLI(a.env, a.cfgDir, args, &out, &errOut)
		return out.String() + errOut.String(), code
	}
	out, code := run("list")
	if code != 0 || !strings.Contains(out, "ada@studio.example") || !strings.Contains(out, "default *") {
		t.Errorf("list %d:\n%s", code, out)
	}
	if out, code := run("dir", "work"); code != 0 || strings.TrimSpace(out) != filepath.Join(a.env.Home, ".claude-work") {
		t.Errorf("dir: %d %s", code, out)
	}
	if _, code := run("dir", "nope"); code == 0 {
		t.Error("dir of no profile")
	}
	if out, _ := run("use", "work"); !strings.HasPrefix(out, "export CLAUDE_CONFIG_DIR='"+a.env.Home+"/.claude-work'\n") {
		t.Errorf("use: %s", out)
	}
	if _, code := run("default", "work"); code != 0 {
		t.Error("default")
	}
	if out, _ := run("default"); strings.TrimSpace(out) != "work" {
		t.Errorf("default is %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(a.cfgDir, "shell.zsh"))
	if !strings.Contains(string(data), `export CLAUDE_CONFIG_DIR="$HOME/.claude-work"`) {
		t.Errorf("shell.zsh:\n%s", data)
	}
	if _, code := run("new", "lab", "--bare"); code != 0 || !a.env.Load("lab").Exists() {
		t.Error("new")
	}
	if out, code := run("frob"); code == 0 || !strings.Contains(out, "unknown command") {
		t.Error("unknown")
	}
	env := profileEnv([]string{"PATH=/bin", "ANTHROPIC_API_KEY=k", "CLAUDE_CONFIG_DIR=/x", "HOME=/h"}, a.env.Load("work"))
	if strings.Join(env, " ") != "PATH=/bin HOME=/h CLAUDE_CONFIG_DIR="+a.env.Home+"/.claude-work" {
		t.Errorf("env %v", env)
	}
}

func TestChinese(t *testing.T) {
	a := fixture(t)
	view := func(c *ui.Context) { a.view(c) }
	tt := newTester(view, 1320, 940)
	_ = tt
	if got := locales.In("zh-Hans").PageOverview(); got != "概览" {
		t.Errorf("zh: %s", got)
	}
	if got := locales.In("zh-Hans").SharedCount(6, 8); got != "已链接 6/8 个共享项" {
		t.Errorf("zh: %s", got)
	}
}

func TestTokens(t *testing.T) {
	for n, want := range map[int64]string{950: "950", 12_345: "12.3K", 4_000_000: "4M", 693_601_258: "694M", 1_260_000_000: "1.3B"} {
		if got := tokens(n); got != want {
			t.Errorf("%d: %s, want %s", n, got, want)
		}
	}
	if n, u := splitNumber("868 MB"); n != "868" || u != "MB" {
		t.Error(n, u)
	}
	if n, u := splitNumber("12.3M"); n != "12.3" || u != "M" {
		t.Error(n, u)
	}
}

func TestOverviewActivity(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 2200)
	for _, want := range []string{locales.CardActivity(), locales.TokensByProject(), locales.CardRecent(), "atlas", locales.Resume()} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if len(a.summary.Recent) == 0 {
		t.Fatal("no recent sessions")
	}
	s := a.summary.Recent[0]
	if err := a.resumeSession("work", s); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(a.lastTerminal, " cli run work --resume '"+s.ID+"'") {
		t.Errorf("resume: %s", a.lastTerminal)
	}
}

func TestPalette(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 940)
	tt.Key(ui.Cmd, ui.KeyK)
	if !a.paletteOpen {
		t.Fatal("⌘K did not open the palette")
	}
	tt.Type("work")
	tt.Frame()
	if m := a.paletteMatches(); len(m) == 0 || m[0].key != "profile-work" {
		t.Fatalf("matches for work: %+v", m)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.paletteOpen || a.page != pageProfile || a.current != "work" {
		t.Errorf("palette %v, page %s %s", a.paletteOpen, a.page, a.current)
	}
	// The arrows choose, and Return runs what they chose.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("instal")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.page != pageInstall {
		t.Errorf("page %s", a.page)
	}
	if got := matchScore("ovw", command{title: "Overview"}); got == 0 {
		t.Error("letters in order do not match")
	}
	if got := matchScore("xyz", command{title: "Overview"}); got != 0 {
		t.Error("matched letters not there")
	}
}

func TestShortcuts(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 940)
	tt.Key(ui.Cmd|ui.Alt, ui.KeyDown)
	tt.Frame()
	if a.current != "side" {
		t.Errorf("next profile: %s", a.current)
	}
	tt.Key(ui.Cmd, ui.KeyN)
	tt.Frame()
	if !a.newOpen {
		t.Error("⌘N did not open the new profile dialog")
	}
}

func TestChange(t *testing.T) {
	for _, c := range []struct {
		cur, prev float64
		text      string
		up, ok    bool
	}{
		{120, 100, "+20%", true, true},
		{50, 100, "−50%", false, true},
		{100, 100, "±0%", true, true},
		{3000, 100, "30×", true, true},
		{5, 0, "", false, false},
	} {
		text, up, ok := change(c.cur, c.prev)
		if text != c.text || up != c.up || ok != c.ok {
			t.Errorf("change(%v, %v) = %q %v %v", c.cur, c.prev, text, up, ok)
		}
	}
}

// The fixture's transcripts were written just now, so the profiles with
// some are working, and the latest session is live.
func TestOverviewLive(t *testing.T) {
	a := fixture(t)
	tt := newTester(a.view, 1320, 2200)
	if !a.live("default") || !a.live("work") || a.live("side") {
		t.Errorf("live: %v", a.active)
	}
	if !tt.HasText(locales.Live()) {
		t.Errorf("no live session in %q", tt.Texts())
	}
	if a.byProfile["work"].Tokens.Total() == 0 || a.byProfile["side"].Tokens.Total() != 0 || a.byProfileIdx != a.rangeIdx {
		t.Errorf("by profile %+v", a.byProfile)
	}
	n := len(a.summary.Recent)
	if n <= recentShown {
		t.Fatalf("%d recent sessions", n)
	}
	if err := tt.Click(locales.ShowAllSessions(n)); err != nil {
		t.Fatal(err)
	}
	if !a.allSessions || !tt.HasText(locales.ShowFewer()) {
		t.Error("Show All did not show them all")
	}
	// A day compares with the day before, which the fixture has.
	if err := tt.Click(locales.Range24h()); err != nil {
		t.Fatal(err)
	}
	if a.summary.Prev.Replies == 0 {
		t.Errorf("no previous day: %+v", a.summary.Prev)
	}
	if text, _, _ := change(float64(a.summary.Tokens.Total()), float64(a.summary.Prev.Tokens.Total())); !tt.HasText(text) {
		t.Errorf("no %q in %q", text, tt.Texts())
	}
}

func TestPaletteResume(t *testing.T) {
	a := fixture(t)
	newTester(a.view, 1320, 940)
	a.openPalette()
	a.paletteQuery = locales.ResumeIn("atlas")
	m := a.paletteMatches()
	if len(m) == 0 || !strings.HasPrefix(m[0].key, "resume-") {
		t.Fatalf("matches %+v", m)
	}
	m[0].run()
	if !strings.Contains(a.lastTerminal, " cli run default --resume ") {
		t.Errorf("resume: %s", a.lastTerminal)
	}
}

// The window coming to the front does not walk a folder measured a moment
// ago; Refresh does.
func TestStorageFresh(t *testing.T) {
	a := fixture(t)
	real := a.storage["default"]
	if real == nil || real.Total == 0 {
		t.Fatalf("storage %+v", real)
	}
	a.storage["default"] = &usage.Storage{Total: 1}
	a.loadProfiles()
	if a.storage["default"].Total != 1 {
		t.Error("measured again at once")
	}
	a.refresh()
	if a.storage["default"].Total != real.Total {
		t.Errorf("refresh: %d, want %d", a.storage["default"].Total, real.Total)
	}
}
