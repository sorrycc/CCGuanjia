package main

import (
	"math"
	"strconv"
	"strings"
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

// dim fades a dialog's backdrop in to a shade of a, the first time it is
// built. The light appearance dims less, so that the window behind stays
// clear rather than going gray.
func dim(backdrop ui.Element, p palette, a float32) {
	if !p.dark {
		a *= 0.6
	}
	tl := backdrop.Timeline("dim", ui.TimelineOptions{})
	t := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 200 * time.Millisecond, Ease: easeOutCubic})
	backdrop.Background(ui.RGBA(0, 0, 0, a*t))
}

// rise brings a dialog's panel in the first time it is built: it fades in
// and grows to its size from a little below, as a sheet does on macOS.
func rise(panel ui.Element) {
	tl := panel.Timeline("rise", ui.TimelineOptions{})
	t := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 260 * time.Millisecond, Ease: easeOutQuint})
	if t < 1 {
		s := 0.96 + 0.04*t
		panel.Opacity(min(1, t*1.6)).Scale(s, s).Translate(0, (1-t)*8)
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

func easeInOutSine(t float32) float32 {
	return float32(0.5 - 0.5*math.Cos(float64(t)*math.Pi))
}

func easeOutQuint(t float32) float32 {
	u := 1 - t
	return 1 - u*u*u*u*u
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

// countUp is a number as it counts up from zero the first time e, which
// shows it, is built: "67.8" passes 12.3 and 45.1 on the way, with as many
// decimals. Anything else, as a version or a dash, shows as it is.
// counting reports whether it is still on the way.
func countUp(e ui.Element, value string) (shown string, counting bool) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil || f <= 0 {
		return value, false
	}
	tl := e.Timeline("count", ui.TimelineOptions{})
	t := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 700 * time.Millisecond, Ease: easeOutQuint}, ui.AtTime(80*time.Millisecond))
	if t >= 1 {
		return value, false
	}
	decimals := 0
	if i := strings.IndexByte(value, '.'); i >= 0 {
		decimals = len(value) - i - 1
	}
	return strconv.FormatFloat(f*float64(t), 'f', decimals, 64), true
}
