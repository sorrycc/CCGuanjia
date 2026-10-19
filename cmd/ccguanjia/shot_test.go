package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
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
		for _, page := range []struct{ name, page, profile string }{
			{"overview", pageOverview, ""},
			{"profile", pageProfile, "work"},
			{"install", pageInstall, ""},
			{"cleanup", pageCleanup, ""},
			{"settings", pageSettings, ""},
		} {
			a := fixture(t)
			a.show(page.page, page.profile)
			tt := ui.NewTester(a.view, 1240, 860)
			tt.SetScale(2)
			tt.SetDark(dark)
			tt.Frame()
			tt.Frame()
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
