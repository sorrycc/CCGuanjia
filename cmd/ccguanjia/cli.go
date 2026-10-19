package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/config"
	"ccguanjia/internal/shell"
)

// cliNames are the names the command is installed as in ~/.local/bin.
var cliNames = []string{"ccgj", "ccguanjia"}

// cliMark is in the scripts the app installs, to tell them from others.
const cliMark = "# Written by CCGuanjia"

const cliUsage = `ccgj — Claude Code profiles, from CCGuanjia

Usage:
  ccgj list                     list the profiles
  ccgj run <profile> [args…]    run Claude Code with a profile
  ccgj use <profile>            switch this shell to a profile (with the shell file)
  ccgj default [<profile>]      show or set the profile of new shells
  ccgj dir <profile>            print a profile's folder
  ccgj new <profile> [--bare]   create a profile, linking the shared items unless --bare
  ccgj open                     open CCGuanjia
  ccgj version                  print the version

"default" is ~/.claude. Every other profile is a ~/.claude-<name> folder,
which Claude Code uses through CLAUDE_CONFIG_DIR.
`

// runCLI runs the command line, and returns the exit code.
func runCLI(env *claude.Env, cfgDir string, args []string, stdout, stderr io.Writer) int {
	cfg := config.Load(cfgDir)
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "ccgj: "+format+"\n", a...)
		return 1
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, cliUsage)
		return 0
	}
	known := func(name string) (*claude.Profile, error) {
		p := env.Load(name)
		if name != claude.DefaultName {
			if err := claude.ValidName(name); err != nil {
				return nil, err
			}
			if !p.Exists() {
				return nil, fmt.Errorf("no profile %q (ccgj list shows them)", name)
			}
		}
		return p, nil
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "list", "ls":
		profiles, _ := env.Discover()
		w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PROFILE\tPLAN\tACCOUNT\tSIGNED IN\tFOLDER")
		def := cfg.Default
		if def == "" {
			def = claude.DefaultName
		}
		for _, p := range profiles {
			name := p.Name
			if name == def {
				name += " *"
			}
			plan, email := "-", "-"
			if p.Account != nil {
				plan, email = orDash(p.Account.Plan), orDash(p.Account.Email)
			}
			signed := "no"
			if p.SignedIn {
				signed = "yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, plan, email, signed, p.Dir)
		}
		w.Flush()
		fmt.Fprintln(stdout, "\n* the profile of new shells")
		return 0
	case "dir":
		if len(rest) != 1 {
			return fail("usage: ccgj dir <profile>")
		}
		p, err := known(rest[0])
		if err != nil {
			return fail("%v", err)
		}
		fmt.Fprintln(stdout, p.Dir)
		return 0
	case "use":
		if len(rest) != 1 {
			return fail("usage: ccgj use <profile>")
		}
		p, err := known(rest[0])
		if err != nil {
			return fail("%v", err)
		}
		// Without the shell file's function, a command cannot change
		// the shell it runs in: it prints what would.
		if p.Default {
			fmt.Fprintln(stdout, "unset CLAUDE_CONFIG_DIR")
		} else {
			fmt.Fprintf(stdout, "export CLAUDE_CONFIG_DIR=%s\n", shell.Quote(p.Dir))
		}
		fmt.Fprintln(stderr, "ccgj: to switch this shell, run: eval \"$(ccgj use "+p.Name+")\", or add the shell file in CCGuanjia's settings")
		return 0
	case "default":
		if len(rest) == 0 {
			d := cfg.Default
			if d == "" {
				d = claude.DefaultName
			}
			fmt.Fprintln(stdout, d)
			return 0
		}
		p, err := known(rest[0])
		if err != nil {
			return fail("%v", err)
		}
		cfg.Default = p.Name
		if p.Default {
			cfg.Default = ""
		}
		if err := cfg.Save(); err != nil {
			return fail("%v", err)
		}
		writeShellFile(env, cfg, cfgDir)
		fmt.Fprintf(stdout, "new shells use %s\n", p.Name)
		return 0
	case "run":
		if len(rest) == 0 {
			return fail("usage: ccgj run <profile> [args…]")
		}
		p, err := known(rest[0])
		if err != nil {
			return fail("%v", err)
		}
		bin, err := exec.LookPath("claude")
		if err != nil {
			return fail("claude is not on the PATH")
		}
		argv := []string{"claude"}
		if o := cfg.Profiles[p.Name]; o != nil && o.SkipPermissions && !p.Default {
			argv = append(argv, shell.SkipFlag)
		}
		argv = append(argv, rest[1:]...)
		if err := syscall.Exec(bin, argv, profileEnv(os.Environ(), p)); err != nil {
			return fail("%v", err)
		}
		return 0
	case "new":
		if len(rest) == 0 {
			return fail("usage: ccgj new <profile> [--bare]")
		}
		items := claude.SharedItems
		if slices.Contains(rest[1:], "--bare") {
			items = nil
		}
		p, err := env.Create(rest[0], items)
		if err != nil {
			return fail("%v", err)
		}
		cfg.Options(p.Name)
		cfg.Save()
		writeShellFile(env, cfg, cfgDir)
		fmt.Fprintf(stdout, "created %s\nsign in once with: ccgj run %s   (or %s, with the shell file)\n", p.Dir, p.Name, shell.FunctionName(p.Name))
		return 0
	case "open":
		if err := exec.Command("/usr/bin/open", "-b", appID).Run(); err != nil {
			return fail("%v", err)
		}
		return 0
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "ccgj "+appVersion())
		return 0
	default:
		return fail("unknown command %q\n\n%s", cmd, cliUsage)
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// profileEnv is env for Claude Code with the profile p: its folder, and
// none of the variables that would point it elsewhere.
func profileEnv(env []string, p *claude.Profile) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if k == "CLAUDE_CONFIG_DIR" || slices.Contains(shell.ProxyVars, k) {
			continue
		}
		out = append(out, kv)
	}
	if !p.Default {
		out = append(out, "CLAUDE_CONFIG_DIR="+p.Dir)
	}
	return out
}

// writeShellFile writes shell.zsh from the command line, as the app does.
func writeShellFile(env *claude.Env, cfg *config.Config, cfgDir string) {
	profiles, _ := env.Discover()
	var names []string
	for _, p := range profiles {
		if !p.Default {
			names = append(names, p.Name)
		}
	}
	cli := ""
	if !cliInstalled(env.Home) {
		if exe, err := os.Executable(); err == nil {
			cli = shell.Quote(exe) + " cli"
		}
	}
	shell.Write(cfgDir, shell.Render(names, cfg, cli))
}

// binDir is where the command is installed.
func binDir(home string) string { return filepath.Join(home, ".local", "bin") }

// cliInstalled reports whether the app's scripts are in ~/.local/bin.
func cliInstalled(home string) bool {
	for _, name := range cliNames {
		data, err := os.ReadFile(filepath.Join(binDir(home), name))
		if err != nil || !strings.Contains(string(data), cliMark) {
			return false
		}
	}
	return true
}

// installCLI writes ccgj and ccguanjia into ~/.local/bin: scripts that
// run the app's executable, wherever it was when they were written. A
// file of another's there is left alone.
func installCLI(home, exe string) error {
	if exe == "" {
		return errors.New("no executable")
	}
	dir := binDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	script := fmt.Sprintf("#!/bin/sh\n%s; runs Claude Code with a profile. `ccgj help` says how.\nexec %s cli \"$@\"\n", cliMark, shell.Quote(exe))
	for _, name := range cliNames {
		path := filepath.Join(dir, name)
		if data, err := os.ReadFile(path); err == nil && !strings.Contains(string(data), cliMark) {
			return fmt.Errorf("%s is another program's", path)
		}
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// uninstallCLI removes the app's scripts from ~/.local/bin.
func uninstallCLI(home string) error {
	for _, name := range cliNames {
		path := filepath.Join(binDir(home), name)
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), cliMark) {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// onPath reports whether dir is on the PATH of path.
func onPath(path, dir string) bool {
	return slices.Contains(filepath.SplitList(path), dir)
}
