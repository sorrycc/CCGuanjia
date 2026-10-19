package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ccguanjia/internal/config"
)

func TestRender(t *testing.T) {
	cfg := config.Load(t.TempDir())
	cfg.Default = "work"
	cfg.Options("personal").SkipPermissions = true
	got := Render([]string{"personal", "work"}, cfg, "")
	for _, want := range []string{
		`export CLAUDE_CONFIG_DIR="$HOME/.claude-work"`,
		`function cc-personal { CLAUDE_CONFIG_DIR="$HOME/.claude-personal" command env -u ANTHROPIC_API_KEY`,
		`claude --dangerously-skip-permissions "$@"; }`,
		`function cc-work { CLAUDE_CONFIG_DIR="$HOME/.claude-work" command env`,
		`dir="$(command ccgj dir "$2")"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, `-work" command env -u ANTHROPIC_API_KEY -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_BASE_URL -u ANTHROPIC_MODEL -u ANTHROPIC_SMALL_FAST_MODEL -u ANTHROPIC_DEFAULT_HAIKU_MODEL -u ANTHROPIC_DEFAULT_SONNET_MODEL -u ANTHROPIC_DEFAULT_OPUS_MODEL claude --dangerously`) {
		t.Error("work skips permissions")
	}
	// No default, no export.
	cfg.Default = "default"
	if strings.Contains(Render([]string{"work"}, cfg, ""), `[ -z "${CLAUDE_CONFIG_DIR-}" ]`) {
		t.Error("exported for the default profile")
	}
	if got := Render(nil, cfg, Quote("/A b/it's")+" cli"); !strings.Contains(got, `'/A b/it'\''s' cli dir "$2"`) {
		t.Errorf("quoting: %s", got)
	}
}

// The file runs in zsh: cc-<name> starts claude with the profile's folder
// and without the proxy variables, and ccgj use switches the shell.
func TestRunsInZsh(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("no zsh")
	}
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho \"dir=$CLAUDE_CONFIG_DIR key=${ANTHROPIC_API_KEY-unset} args=$*\"\n"), 0o755)
	os.WriteFile(filepath.Join(bin, "ccgj"), []byte("#!/bin/sh\n[ \"$1\" = dir ] && echo \"$HOME/.claude-$2\"\n"), 0o755)
	cfg := config.Load(home)
	cfg.Options("work").SkipPermissions = true
	path, err := Write(filepath.Join(home, "cfg"), Render([]string{"work"}, cfg, ""))
	if err != nil {
		t.Fatal(err)
	}
	script := `alias cc-work='echo alias'; source "` + path + `"; cc-work -p hi; ccgj use work; echo "now=$CLAUDE_CONFIG_DIR"; ccgj use default; echo "after=${CLAUDE_CONFIG_DIR-unset}"`
	cmd := exec.Command(zsh, "-fc", "unalias cc-work 2>/dev/null; "+script)
	cmd.Env = []string{"HOME=" + home, "PATH=" + bin + ":/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	want := "dir=" + home + "/.claude-work key=unset args=--dangerously-skip-permissions -p hi\nnow=" + home + "/.claude-work\nafter=unset\n"
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

func TestInstallUninstall(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".zshrc")
	os.WriteFile(rc, []byte("export A=1"), 0o600)
	path := filepath.Join(home, ".config", "ccguanjia", FileName)
	if err := Install(home, path); err != nil {
		t.Fatal(err)
	}
	Install(home, path)
	data, _ := os.ReadFile(rc)
	if strings.Count(string(data), marker) != 1 || !strings.HasPrefix(string(data), "export A=1\n") ||
		!strings.Contains(string(data), `source "$HOME/.config/ccguanjia/shell.zsh"`) || !Installed(home) {
		t.Fatalf("rc:\n%s", data)
	}
	if fi, _ := os.Stat(rc); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode())
	}
	if err := Uninstall(home); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(rc)
	if Installed(home) || !strings.HasPrefix(string(data), "export A=1") || strings.Contains(string(data), "CCGuanjia") {
		t.Errorf("after uninstall:\n%q", data)
	}
	backups, _ := filepath.Glob(rc + ".bak.*")
	if len(backups) == 0 {
		t.Error("no backup")
	}
}

func TestScanRC(t *testing.T) {
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".zshrc"), []byte(`alias ccx="ANTHROPIC_API_KEY=sk-secret claude"
function cc2 { CLAUDE_CONFIG_DIR="$HOME/.claude-personal" env -u X claude --dangerously-skip-permissions "$@"; }
cc3() { CLAUDE_CONFIG_DIR=~/.claude-client claude; }
alias cc-work='CLAUDE_CONFIG_DIR="${HOME}/.claude-work" claude'
# function old { CLAUDE_CONFIG_DIR="$HOME/.claude-old" claude; }
`), 0o600)
	got := ScanRC(home)
	want := []Launcher{{"cc2", "personal", true}, {"cc3", "client", false}, {"cc-work", "work", false}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
