package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
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
			{"install-version", pageInstall, "", 860},
			{"cleanup", pageCleanup, "", 860},
			{"settings", pageSettings, "", 860},
			{"claude-settings", pageClaudeSettings, "work", 860},
			{"claude-settings-edited", pageClaudeSettings, "work", 860},
			{"claude-settings-json", pageClaudeSettings, "work", 860},
			{"claude-settings-review", pageClaudeSettings, "work", 860},
			{"claude-settings-review-file", pageClaudeSettings, "work", 860},
			{"palette", pageOverview, "", 860},
		} {
			a := fixture(t)
			a.show(page.page, page.profile)
			if page.name == "palette" {
				a.openPalette()
				a.paletteSel = 2
			}
			if strings.HasPrefix(page.name, "claude-settings-") {
				d := a.csDrafts["work"]
				d.set([]string{"model"}, "opus")
				d.syncFields()
				d.set([]string{"permissions", "allow"}, []string{"Bash(ls)", "Read"})
				d.set([]string{"env", "DISABLE_TELEMETRY"}, "1")
				switch page.name {
				case "claude-settings-json":
					d.toJSON()
				case "claude-settings-review":
					d.review = true
				case "claude-settings-review-file":
					d.review, d.reviewTab = true, 1
				}
			}
			if page.name == "install-version" {
				a.available = func() ([]string, error) { return []string{"2.1.303", "2.1.302", "2.1.301", "2.1.289", "2.1.288"}, nil }
				a.openVersionDialog("2.1.289")
			}
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
