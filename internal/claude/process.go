package claude

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Process is a running process of Claude Code.
type Process struct {
	PID  int
	Name string
	// Path is its executable, as ps tells it.
	Path string
}

// Running lists the processes named claude (the command line) or Claude
// (the desktop app), as `pgrep -x` finds them.
func Running() []Process {
	out, err := exec.Command("/bin/ps", "-axo", "pid=,comm=").Output()
	if err != nil {
		return nil
	}
	return parsePS(string(out))
}

func parsePS(out string) []Process {
	var procs []Process
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		pidText, path, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(pidText)
		if err != nil {
			continue
		}
		path = strings.TrimSpace(path)
		name := filepath.Base(path)
		// The native installer's binaries are named by their version, in
		// ~/.local/share/claude/versions.
		if name == "claude" || name == "Claude" || strings.Contains(path, "/claude/versions/") {
			procs = append(procs, Process{PID: pid, Name: name, Path: path})
		}
	}
	return procs
}
