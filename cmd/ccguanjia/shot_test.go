package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestShots draws every page of the fixture, dark and light, into the
// folder CCGJ_SHOTS names, for the README and for looking at the layout.
//
//	CCGJ_SHOTS=build/shots go test -run TestShots ./cmd/ccguanjia
func TestShots(t *testing.T) {
	dir := os.Getenv("CCGJ_SHOTS")
	if dir == "" {
		t.Skip("CCGJ_SHOTS is not set")
	}
	os.MkdirAll(dir, 0o755)
	for _, dark := range []bool{true, false} {
		for _, page := range []struct {
			name, page, profile string
			height              int
		}{
			{"overview", pageOverview, "", 860},
			{"overview-full", pageOverview, "", 1720},
			{"profile", pageProfile, "work", 860},
			{"install", pageInstall, "", 860},
			{"cleanup", pageCleanup, "", 860},
			{"settings", pageSettings, "", 860},
		} {
			a := fixture(t)
			a.show(page.page, page.profile)
			tt := newTester(a.view, 1240, page.height)
			tt.SetScale(2)
			tt.SetDark(dark)
			tt.Frame()
			// Colors and content fade across even with less motion.
			tt.Advance(2 * time.Second)
			name := page.name + "-light.png"
			if dark {
				name = page.name + "-dark.png"
			}
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
