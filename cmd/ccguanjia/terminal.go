package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"ccguanjia/internal/shell"
	"ccguanjia/internal/usage"
)

// launchCommand is the shell command that runs Claude Code with the
// profile name: the app's executable as ccgj, so that it works whether
// the command and the shell file are installed or not.
func (a *app) launchCommand(name string) string {
	if a.exe == "" {
		return "ccgj run " + name
	}
	return shell.Quote(a.exe) + " cli run " + name
}

// openTerminal opens the terminal of the settings running Claude Code with
// the profile name.
func (a *app) openTerminal(name string) error {
	return openTerminal(a.cfg.TerminalApp(), a.launchCommand(name))
}

// resumeSession opens the terminal in the folder of a session, resuming
// it with the profile name.
func (a *app) resumeSession(name string, s usage.Session) error {
	cmd := a.launchCommand(name) + " --resume " + shell.Quote(s.ID)
	if fi, err := os.Stat(s.Project); err == nil && fi.IsDir() {
		cmd = "cd " + shell.Quote(s.Project) + " && " + cmd
	}
	if a.sync {
		a.lastTerminal = cmd
		return nil
	}
	return openTerminal(a.cfg.TerminalApp(), cmd)
}

func openTerminal(app, command string) error {
	var cmd *exec.Cmd
	switch app {
	case "iTerm":
		cmd = exec.Command("/usr/bin/osascript",
			"-e", `tell application "iTerm"`,
			"-e", "activate",
			"-e", "set w to (create window with default profile)",
			"-e", fmt.Sprintf("tell current session of w to write text %s", appleString(command)),
			"-e", "end tell")
	case "Ghostty":
		// A login shell runs the command, then stays, as a terminal's tab
		// does.
		cmd = exec.Command("/usr/bin/open", "-na", "Ghostty", "--args", "-e", "/bin/zsh", "-ilc", command+"; exec /bin/zsh -il")
	default:
		cmd = exec.Command("/usr/bin/osascript",
			"-e", `tell application "Terminal"`,
			"-e", "activate",
			"-e", fmt.Sprintf("do script %s", appleString(command)),
			"-e", "end tell")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", app, strings.TrimSpace(string(out)))
	}
	return nil
}

// appleString quotes s for AppleScript.
func appleString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
