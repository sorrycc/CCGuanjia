package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
)

// maxCrashes is how many crash logs the crashes folder keeps.
const maxCrashes = 10

// keepCrashes has the runtime write the panic or fatal error that ends
// the app, with every goroutine's stack, to crash.log in dir as well as
// to stderr, which nothing keeps when the app starts from the Finder or
// the Dock. A crash.log a crash left is moved first into the crashes
// folder, named by its time, so that each launch starts an empty one.
func keepCrashes(dir string) {
	if dir == "" {
		return
	}
	path := filepath.Join(dir, "crash.log")
	if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
		crashes := filepath.Join(dir, "crashes")
		if os.MkdirAll(crashes, 0o755) == nil {
			name := fi.ModTime().Format("2006-01-02-150405") + ".log"
			os.Rename(path, filepath.Join(crashes, name))
			pruneCrashes(crashes)
		}
	}
	os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	debug.SetCrashOutput(f, debug.CrashOptions{})
	f.Close() // SetCrashOutput keeps its own copy
}

// pruneCrashes removes all but the newest maxCrashes logs of dir.
func pruneCrashes(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	for len(names) > maxCrashes {
		os.Remove(filepath.Join(dir, names[0]))
		names = names[1:]
	}
}
