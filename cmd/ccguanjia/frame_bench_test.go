package main

import (
	"testing"
	"time"
)

// BenchmarkFrame builds and lays out a frame of each page of the fixture,
// as the window does while something on it moves.
//
//	go test -run XXX -bench Frame ./cmd/ccguanjia
func BenchmarkFrame(b *testing.B) {
	for _, page := range []struct{ name, page, profile string }{
		{"overview-all", pageOverview, ""},
		{"overview-work", pageOverview, "work"},
		{"profile", pageProfile, "work"},
		{"install", pageInstall, ""},
		{"cleanup", pageCleanup, ""},
		{"claude-settings", pageClaudeSettings, "work"},
	} {
		b.Run(page.name, func(b *testing.B) {
			a := fixture(b)
			if page.page == pageOverview && page.profile == "" {
				a.showAll()
			} else {
				a.show(page.page, page.profile)
			}
			tt := newTester(a.view, 1240, 860)
			tt.Advance(2 * time.Second)
			b.ResetTimer()
			for range b.N {
				tt.Frame()
			}
		})
	}
}
