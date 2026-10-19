package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The motion of the window: short, and settling without a bounce, so that
// it shows where things go without keeping anyone waiting.
var (
	// quick is how hovers and selections change color.
	quick = ui.ElementTransition{Colors: true, Duration: 140 * time.Millisecond}
	// glide is how a selection moves to another row or segment.
	glide = ui.Animation{Spring: ui.SpringOptions{Duration: 380 * time.Millisecond, Bounce: 0.12}}
	// numeric rolls the digits of a number that changes.
	numeric = ui.ContentTransition{Numeric: true}
)

// stage counts the cards built in a frame, so that the cards of a page
// come in one after another.
type stage struct{ n int }

// enter fades an element in and raises it into place the first time it is
// built, after those built before it in the frame.
func (s *stage) enter(e ui.Element) {
	i := 0
	if s != nil {
		i = s.n
		s.n++
	}
	tl := e.Timeline("enter", ui.TimelineOptions{})
	t := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 420 * time.Millisecond, Ease: easeOutCubic},
		ui.AtTime(time.Duration(min(i, 10))*35*time.Millisecond))
	if t < 1 {
		e.Opacity(t).Translate(0, (1-t)*10)
	}
}

// grow is a value from 0 to 1 as an element's chart draws itself the first
// time it shows.
func grow(e ui.Element) float32 {
	tl := e.Timeline("grow", ui.TimelineOptions{})
	return tl.Tween(ui.Tween{From: 0, To: 1, Duration: 750 * time.Millisecond, Ease: easeOutCubic}, ui.AtTime(120*time.Millisecond))
}

func easeOutCubic(t float32) float32 {
	u := 1 - t
	return 1 - u*u*u
}

// stagger is how far along part i of n is when the whole is at t: the
// parts start one after another and each takes most of the way.
func stagger(t float32, i, n int) float32 {
	if n <= 1 {
		return t
	}
	const spread = 0.45
	start := spread * float32(i) / float32(n-1)
	return float32(math.Max(0, math.Min(1, float64((t-start)/(1-spread)))))
}
