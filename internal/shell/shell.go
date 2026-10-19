// Package shell writes the zsh file that starts Claude Code with a
// profile: a function cc-<name> for each profile, the default profile of
// new shells, and `ccgj use`, which switches the shell it runs in. The
// file is the app's own, shell.zsh beside config.json, and ~/.zshrc only
// gets one line that sources it, so the lines written by hand there stay
// as they are.
package shell

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/config"
)

// ProxyVars are cleared for a profile's Claude Code: each would point a
// subscription's sign-in at another endpoint, or a key, instead.
var ProxyVars = []string{
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_SMALL_FAST_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
}

// SkipFlag is the flag a profile's options may add.
const SkipFlag = "--dangerously-skip-permissions"

// marker ends the line of ~/.zshrc that sources the file.
const marker = "# CCGuanjia"

// FileName is the name of the file in the config folder.
const FileName = "shell.zsh"

// FunctionName is the shell function of a profile.
func FunctionName(profile string) string { return "cc-" + profile }

// Render returns the file for the profiles (the names of the ~/.claude-*
// ones), their options in cfg, and cli, the words that run the ccgj
// command, "" for the one on the PATH.
func Render(profiles []string, cfg *config.Config, cli string) string {
	if cli == "" {
		cli = "command ccgj"
	}
	var b strings.Builder
	b.WriteString("# Written by CCGuanjia, which writes it again as profiles change. Edit the\n")
	b.WriteString("# profiles in the app or with ccgj, not here.\n\n")
	if d := cfg.Default; d != "" && d != claude.DefaultName && slices.Contains(profiles, d) {
		b.WriteString("# New shells use the default profile, unless CLAUDE_CONFIG_DIR is set.\n")
		fmt.Fprintf(&b, "[ -z \"${CLAUDE_CONFIG_DIR-}\" ] && export CLAUDE_CONFIG_DIR=\"$HOME/.claude-%s\"\n\n", d)
	}
	b.WriteString("# Each profile's Claude Code, on its own sign-in: the variables that would\n")
	b.WriteString("# point it at another endpoint are cleared.\n")
	unsets := make([]string, len(ProxyVars))
	for i, v := range ProxyVars {
		unsets[i] = "-u " + v
	}
	for _, name := range profiles {
		flag := ""
		if o := cfg.Profiles[name]; o != nil && o.SkipPermissions {
			flag = " " + SkipFlag
		}
		// `function name` rather than name(), which an alias of the same
		// name would expand in.
		fmt.Fprintf(&b, "function %s { CLAUDE_CONFIG_DIR=\"$HOME/.claude-%s\" command env %s claude%s \"$@\"; }\n",
			FunctionName(name), name, strings.Join(unsets, " "), flag)
	}
	b.WriteString("\n# `ccgj use <profile>` switches this shell; the rest goes to the command.\n")
	fmt.Fprintf(&b, `function ccgj {
  if [ "$1" = use ] && [ $# -ge 2 ]; then
    local dir
    dir="$(%[1]s dir "$2")" || return
    if [ "$2" = default ]; then unset CLAUDE_CONFIG_DIR; else export CLAUDE_CONFIG_DIR="$dir"; fi
  else
    %[1]s "$@"
  fi
}
function ccguanjia { ccgj "$@"; }
`, cli)
	return b.String()
}

// Quote quotes s for the shell.
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Write writes the file into the config folder, and returns its path.
func Write(dir, content string) (string, error) {
	path := filepath.Join(dir, FileName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if old, err := os.ReadFile(path); err == nil && string(old) == content {
		return path, nil
	}
	return path, os.WriteFile(path, []byte(content), 0o644)
}

// SourceLine is the line of ~/.zshrc that sources the file at path.
func SourceLine(home, path string) string {
	shown := path
	if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
		shown = "$HOME/" + rel
	}
	return fmt.Sprintf(`[ -f "%[1]s" ] && source "%[1]s" %[2]s`, shown, marker)
}

// Installed reports whether ~/.zshrc sources the file.
func Installed(home string) bool {
	data, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), marker) {
			return true
		}
	}
	return false
}

// Install adds the line that sources the file at path to the end of
// ~/.zshrc, after copying it to .zshrc.bak.<time>. It does nothing when
// the line is there.
func Install(home, path string) error {
	if Installed(home) {
		return nil
	}
	rc := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(rc)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		if err := backup(rc, data); err != nil {
			return err
		}
	}
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, '\n')
	}
	data = append(data, []byte("\n# Claude Code profiles, from CCGuanjia\n"+SourceLine(home, path)+"\n")...)
	return writeKeepingMode(rc, data)
}

// Uninstall removes the line from ~/.zshrc, after copying it.
func Uninstall(home string) error {
	rc := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(rc)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var kept []string
	for i, line := range lines {
		if strings.HasSuffix(strings.TrimSpace(line), marker) {
			continue
		}
		if line == "# Claude Code profiles, from CCGuanjia" && i+1 < len(lines) && strings.HasSuffix(strings.TrimSpace(lines[i+1]), marker) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == len(lines) {
		return nil
	}
	if err := backup(rc, data); err != nil {
		return err
	}
	return writeKeepingMode(rc, []byte(strings.Join(kept, "\n")))
}

func backup(path string, data []byte) error {
	return os.WriteFile(fmt.Sprintf("%s.bak.%d", path, time.Now().Unix()), data, 0o600)
}

func writeKeepingMode(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}

// Launcher is a function or alias of ~/.zshrc, written by hand, that
// starts Claude Code with a profile's folder.
type Launcher struct {
	Name    string
	Profile string
	// SkipPermissions reports whether it passes SkipFlag.
	SkipPermissions bool
}

var (
	configDir   = regexp.MustCompile(`CLAUDE_CONFIG_DIR=["']?(?:\$HOME|\$\{HOME\}|~)/\.claude-([A-Za-z0-9_-]+)["']?`)
	aliasName   = regexp.MustCompile(`^\s*alias\s+([^=\s]+)=`)
	funcKeyword = regexp.MustCompile(`^\s*function\s+([^\s({]+)`)
	funcParens  = regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*\(\)`)
)

// ScanRC finds the launchers of ~/.zshrc. It looks only at the lines that
// set CLAUDE_CONFIG_DIR to a ~/.claude-* folder, and keeps nothing else of
// them: the lines around them may hold keys.
func ScanRC(home string) []Launcher {
	f, err := os.Open(filepath.Join(home, ".zshrc"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Launcher
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		m := configDir.FindStringSubmatch(line)
		if m == nil || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		var name string
		for _, re := range []*regexp.Regexp{aliasName, funcKeyword, funcParens} {
			if n := re.FindStringSubmatch(line); n != nil {
				name = n[1]
				break
			}
		}
		if name == "" {
			continue
		}
		out = append(out, Launcher{Name: name, Profile: m[1], SkipPermissions: strings.Contains(line, SkipFlag)})
	}
	return out
}
