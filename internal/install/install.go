// Package install finds how Claude Code is installed (the native
// installer, npm, Homebrew or Bun) and its version, the native installer's
// older versions, and updates or reinstalls it the way it was installed,
// with that way's own command.
package install

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Kind is a way of installing Claude Code.
type Kind string

const (
	Native   Kind = "native"
	NPM      Kind = "npm"
	Homebrew Kind = "homebrew"
	Bun      Kind = "bun"
	Other    Kind = "other"
)

// Install is a claude command on the PATH.
type Install struct {
	// Path is where the PATH finds it, Target the file it resolves to.
	Path   string
	Target string
	Kind   Kind
	// Version is what it says it is, "" when it would not say.
	Version string
}

// Version is a version the native installer keeps.
type Version struct {
	Name  string
	Path  string
	Bytes int64
	// Current is the one ~/.local/bin/claude points to, and InUse
	// reports whether a running process runs it.
	Current bool
	InUse   bool
}

// Info is what Detect finds.
type Info struct {
	// Installs are the claude commands of the PATH, the one that runs
	// first.
	Installs []Install
	// Versions are the native installer's versions, newest first.
	Versions []Version
	// Shadowed is how the login shell defines claude when it is not the
	// command itself: "function" or "alias", "" otherwise.
	Shadowed string
}

// Active is the install that runs, nil when there is none.
func (in *Info) Active() *Install {
	if len(in.Installs) == 0 {
		return nil
	}
	return &in.Installs[0]
}

// Kinds are the different ways Claude Code is installed, in PATH order.
func (in *Info) Kinds() []Kind {
	var kinds []Kind
	for _, i := range in.Installs {
		if !slices.Contains(kinds, i.Kind) {
			kinds = append(kinds, i.Kind)
		}
	}
	return kinds
}

// OldVersions are the native versions neither current nor running.
func (in *Info) OldVersions() []Version {
	var old []Version
	for _, v := range in.Versions {
		if !v.Current && !v.InUse {
			old = append(old, v)
		}
	}
	return old
}

// VersionsDir is where the native installer keeps its versions.
func VersionsDir(home string) string {
	return filepath.Join(home, ".local", "share", "claude", "versions")
}

// Detect looks for Claude Code along path, a PATH, with home's native
// versions, and the processes running them in inUse (paths of
// executables, from RunningExecutables).
func Detect(ctx context.Context, home, path string, inUse []string) *Info {
	in := &Info{}
	seen := map[string]bool{}
	// Targets are resolved, so the home they are compared with is too.
	realHome := home
	if h, err := filepath.EvalSymlinks(home); err == nil {
		realHome = h
	}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, "claude")
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			continue
		}
		target, err := filepath.EvalSymlinks(p)
		if err != nil {
			target = p
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		i := Install{Path: p, Target: target, Kind: kindOf(realHome, target)}
		if i.Kind == Native && strings.HasPrefix(target, VersionsDir(realHome)) {
			i.Version = filepath.Base(target)
		} else {
			i.Version = runVersion(ctx, p)
		}
		in.Installs = append(in.Installs, i)
	}
	in.Versions = nativeVersions(home, inUse)
	return in
}

func kindOf(home, target string) Kind {
	t := filepath.ToSlash(target)
	switch {
	case strings.HasPrefix(target, filepath.Join(home, ".local", "share", "claude")+string(filepath.Separator)):
		return Native
	case strings.Contains(t, "/Caskroom/claude-code/") || strings.Contains(t, "/Cellar/claude-code/"):
		return Homebrew
	case strings.Contains(t, "/.bun/"):
		return Bun
	case strings.Contains(t, "/node_modules/@anthropic-ai/claude-code/"):
		return NPM
	}
	return Other
}

var versionRE = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)

func runVersion(ctx context.Context, path string) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Env = append(os.Environ(), "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	if m := versionRE.FindStringSubmatch(string(out)); m != nil {
		return m[1]
	}
	return ""
}

func nativeVersions(home string, inUse []string) []Version {
	dir := VersionsDir(home)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	current, _ := filepath.EvalSymlinks(filepath.Join(home, ".local", "bin", "claude"))
	var vs []Version
	for _, e := range ents {
		if !versionRE.MatchString(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		fi, err := e.Info()
		if err != nil {
			continue
		}
		real, _ := filepath.EvalSymlinks(path)
		vs = append(vs, Version{
			Name:    e.Name(),
			Path:    path,
			Bytes:   fi.Size(),
			Current: real != "" && real == current,
			InUse:   slices.Contains(inUse, path) || slices.Contains(inUse, real),
		})
	}
	sort.Slice(vs, func(i, j int) bool { return Compare(vs[i].Name, vs[j].Name) > 0 })
	return vs
}

// Compare compares two versions x.y.z, as -1, 0 or 1.
func Compare(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// RemoveVersions deletes native versions of home: only files in its
// versions folder, and never the current one.
func RemoveVersions(home string, vs []Version) (int64, error) {
	dir := VersionsDir(home)
	current, _ := filepath.EvalSymlinks(filepath.Join(home, ".local", "bin", "claude"))
	var freed int64
	for _, v := range vs {
		if filepath.Dir(v.Path) != dir {
			return freed, fmt.Errorf("%s is not a native version", v.Path)
		}
		if real, _ := filepath.EvalSymlinks(v.Path); real == current {
			return freed, fmt.Errorf("%s is the current version", v.Name)
		}
		if err := os.Remove(v.Path); err != nil {
			return freed, err
		}
		freed += v.Bytes
	}
	return freed, nil
}

// RunningExecutables are the executables of the running processes, of
// lsof's txt descriptors of pids.
func RunningExecutables(pids []int) []string {
	if len(pids) == 0 {
		return nil
	}
	list := make([]string, len(pids))
	for i, p := range pids {
		list[i] = strconv.Itoa(p)
	}
	out, _ := exec.Command("/usr/sbin/lsof", "-nP", "-a", "-p", strings.Join(list, ","), "-d", "txt", "-Fn").Output()
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(line, "n"); ok && !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	return paths
}

// LoginShell is the user's shell, zsh when $SHELL is not set.
func LoginShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/zsh"
}

const marker = "__CCGUANJIA__"

// ShellProbe is what the user's interactive login shell says: its PATH,
// and how it defines claude. An app started from the Finder has a short
// PATH of its own, unlike a terminal's.
func ShellProbe(ctx context.Context) (path, shadowed string) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	script := `printf '\n` + marker + `%s\n' "$PATH"; ` +
		`if [ -n "$ZSH_VERSION" ]; then printf '` + marker + `%s\n' "$(whence -w claude 2>/dev/null)"; else printf '` + marker + `%s\n' "$(type -t claude 2>/dev/null)"; fi`
	cmd := exec.CommandContext(ctx, LoginShell(), "-ilc", script)
	cmd.Stdin = nil
	out, _ := cmd.Output()
	var found []string
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, marker); ok {
			found = append(found, rest)
		}
	}
	if len(found) > 0 {
		path = found[0]
	}
	if len(found) > 1 {
		w := strings.TrimSpace(found[1])
		w = strings.TrimPrefix(w, "claude: ")
		if w == "function" || w == "alias" {
			shadowed = w
		}
	}
	if path == "" {
		path = os.Getenv("PATH")
	}
	return path, shadowed
}

// LatestURL answers the newest version of the native installer.
const LatestURL = "https://downloads.claude.ai/claude-code-releases/latest"

// NPMLatestURL answers npm's newest version, if LatestURL does not.
const NPMLatestURL = "https://registry.npmjs.org/@anthropic-ai/claude-code/latest"

// Latest asks for the newest version of Claude Code. The requests carry
// nothing of the user's.
func Latest(ctx context.Context, client *http.Client) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	get := func(url string) (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%s: %s", url, resp.Status)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if err != nil {
			return "", err
		}
		text := strings.TrimSpace(string(body))
		if strings.HasPrefix(text, "{") {
			if m := regexp.MustCompile(`"version"\s*:\s*"(\d+\.\d+\.\d+)"`).FindStringSubmatch(text); m != nil {
				return m[1], nil
			}
		} else if m := versionRE.FindStringSubmatch(text); m != nil && len(text) <= 64 {
			return m[1], nil
		}
		return "", fmt.Errorf("%s: no version", url)
	}
	v, err := get(LatestURL)
	if err == nil {
		return v, nil
	}
	if v, err2 := get(NPMLatestURL); err2 == nil {
		return v, nil
	}
	return "", err
}

// Action is what to do to an install.
type Action int

const (
	Update Action = iota
	Reinstall
)

// Command is the shell command that does a to an install of kind, by the
// official way of that kind, "" when there is none.
func Command(kind Kind, a Action, path string) string {
	switch kind {
	case Native:
		if a == Update {
			return shellQuote(path) + " update"
		}
		return "curl -fsSL https://claude.ai/install.sh | bash"
	case NPM:
		return "npm install -g @anthropic-ai/claude-code@latest"
	case Homebrew:
		if a == Update {
			return "brew upgrade --cask claude-code"
		}
		return "brew reinstall --cask claude-code"
	case Bun:
		return "bun add -g @anthropic-ai/claude-code@latest"
	}
	return ""
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Run runs command in the user's login shell, telling each line of its
// output to line as it comes.
func Run(ctx context.Context, command string, line func(string)) error {
	if command == "" {
		return errors.New("no command")
	}
	cmd := exec.CommandContext(ctx, LoginShell(), "-lc", command)
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		sc.Split(scanLinesOrReturns)
		for sc.Scan() {
			if t := strings.TrimSpace(stripANSI(sc.Text())); t != "" {
				line(t)
			}
		}
	}()
	err := cmd.Wait()
	pw.Close()
	<-done
	return err
}

// scanLinesOrReturns splits at \n and at \r, which progress bars redraw
// their line with.
func scanLinesOrReturns(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }
