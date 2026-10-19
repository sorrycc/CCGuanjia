package install

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func exe(t *testing.T, path, script string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDetect(t *testing.T) {
	home := t.TempDir()
	vdir := VersionsDir(home)
	for _, v := range []string{"2.1.9", "2.1.10", "2.1.8"} {
		exe(t, filepath.Join(vdir, v), "#!/bin/sh\n")
	}
	bin := filepath.Join(home, ".local", "bin")
	os.MkdirAll(bin, 0o755)
	os.Symlink(filepath.Join(vdir, "2.1.10"), filepath.Join(bin, "claude"))
	npm := filepath.Join(home, "npm")
	exe(t, filepath.Join(npm, "lib", "node_modules", "@anthropic-ai", "claude-code", "cli.js"), "#!/bin/sh\necho '2.0.1 (Claude Code)'\n")
	os.MkdirAll(filepath.Join(npm, "bin"), 0o755)
	os.Symlink(filepath.Join(npm, "lib", "node_modules", "@anthropic-ai", "claude-code", "cli.js"), filepath.Join(npm, "bin", "claude"))
	path := strings.Join([]string{bin, filepath.Join(npm, "bin"), bin, filepath.Join(home, "none")}, string(os.PathListSeparator))

	in := Detect(context.Background(), home, path, []string{filepath.Join(vdir, "2.1.8")})
	if len(in.Installs) != 2 {
		t.Fatalf("installs %+v", in.Installs)
	}
	if a := in.Active(); a.Kind != Native || a.Version != "2.1.10" {
		t.Errorf("active %+v", a)
	}
	if n := in.Installs[1]; n.Kind != NPM || n.Version != "2.0.1" {
		t.Errorf("npm %+v", n)
	}
	if got := in.Kinds(); !slices.Equal(got, []Kind{Native, NPM}) {
		t.Errorf("kinds %v", got)
	}
	var names []string
	for _, v := range in.Versions {
		names = append(names, v.Name)
	}
	if !slices.Equal(names, []string{"2.1.10", "2.1.9", "2.1.8"}) || !in.Versions[0].Current || !in.Versions[2].InUse {
		t.Errorf("versions %+v", in.Versions)
	}
	old := in.OldVersions()
	if len(old) != 1 || old[0].Name != "2.1.9" {
		t.Fatalf("old %+v", old)
	}
	if _, err := RemoveVersions(home, in.Versions[:1]); err == nil {
		t.Error("removed the current version")
	}
	if _, err := RemoveVersions(home, old); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(vdir, "2.1.9")); err == nil {
		t.Error("2.1.9 still there")
	}
}

func TestKind(t *testing.T) {
	for target, want := range map[string]Kind{
		"/h/.local/share/claude/versions/2.1.3":                     Native,
		"/opt/homebrew/Caskroom/claude-code/2.1.3/claude":           Homebrew,
		"/h/.bun/install/global/node_modules/@anthropic-ai/x":       Bun,
		"/usr/local/lib/node_modules/@anthropic-ai/claude-code/cli": NPM,
		"/usr/local/bin/claude":                                     Other,
	} {
		if got := kindOf("/h", target); got != want {
			t.Errorf("%s: %s, want %s", target, got, want)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("2.1.303\n"))
	}))
	defer srv.Close()
	client := srv.Client()
	client.Transport = rewrite{srv.URL, http.DefaultTransport}
	v, err := Latest(context.Background(), client)
	if err != nil || v != "2.1.303" {
		t.Errorf("latest %q %v", v, err)
	}
}

type rewrite struct {
	to string
	rt http.RoundTripper
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme, u.Host = "http", strings.TrimPrefix(r.to, "http://")
	req = req.Clone(req.Context())
	req.URL = &u
	return r.rt.RoundTrip(req)
}

func TestCommand(t *testing.T) {
	if got := Command(Native, Update, "/a b/claude"); got != "'/a b/claude' update" {
		t.Error(got)
	}
	if got := Command(Homebrew, Reinstall, ""); got != "brew reinstall --cask claude-code" {
		t.Error(got)
	}
	if Command(Other, Update, "") != "" {
		t.Error("other")
	}
}

func TestRun(t *testing.T) {
	var lines []string
	err := Run(context.Background(), "printf 'one\\n\\033[32mtwo\\033[0m\\rthree\\n'", func(l string) { lines = append(lines, l) })
	if err != nil || !slices.Equal(lines, []string{"one", "two", "three"}) {
		t.Errorf("lines %q %v", lines, err)
	}
	if err := Run(context.Background(), "exit 3", func(string) {}); err == nil {
		t.Error("no error")
	}
}

func TestCompare(t *testing.T) {
	if Compare("2.1.10", "2.1.9") != 1 || Compare("2.1", "2.1.0") != 0 || Compare("1.9.9", "2.0.0") != -1 {
		t.Error("compare")
	}
}
