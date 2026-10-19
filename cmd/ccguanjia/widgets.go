package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// iconSquare is an icon in a rounded square of its color's tint.
func iconSquare(c *ui.Context, p palette, icon *ui.SVG, color ui.Color, size float32) {
	ui.Box(c).Size(size, size).Radius(size * 0.28).Background(p.tint(color)).Center().Shrink(0).Children(func() {
		ui.Icon(c, icon).Size(size*0.58, size*0.58).TextColor(color)
	})
}

// card is a panel of the content. With onOpen, its title row shows an
// arrow, and a click on it opens what the card is about.
func card(c *ui.Context, p palette, icon *ui.SVG, color ui.Color, title, extra string, onOpen func(), body func()) ui.Element {
	box := ui.Column(c).Radius(18).Background(p.card).Padding(18, 20, 20).Gap(14).MinWidth(0)
	box.Children(func() {
		head := ui.Row(c).Gap(12).AlignItems(ui.Center)
		if onOpen != nil {
			head.Cursor(ui.CursorPointer).OnClick(onOpen)
		}
		head.Children(func() {
			iconSquare(c, p, icon, color, 30)
			ui.Text(c, title).FontSize(17).FontWeight(600).TextColor(color).SingleLine().Grow(1)
			if extra != "" {
				ui.Text(c, extra).FontSize(13).TextColor(p.muted).SingleLine()
			}
			if onOpen != nil {
				ui.Icon(c, iconChevron).Size(16, 16).TextColor(p.faint)
			}
		})
		body()
	})
	return box
}

// bigValue is a card's large number, with its unit after it.
func bigValue(c *ui.Context, p palette, label, value, unit string) {
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(p.muted).SingleLine().Padding(0, 150, 0, 0)
		ui.Row(c).Gap(4).AlignItems(ui.End).Children(func() {
			size := float32(40)
			if len([]rune(value)) > 5 {
				size = 32
			}
			ui.Text(c, value).FontSize(size).FontWeight(700).TextColor(p.text).FontFeatures("tnum").SingleLine().Shrink(1)
			if unit != "" {
				ui.Text(c, unit).FontSize(18).FontWeight(500).TextColor(p.muted).Padding(0, 0, 6).SingleLine()
			}
		})
	})
}

// stat is a label and a value under a card's large number.
type stat struct{ label, value string }

// stats are three stats side by side.
func stats(c *ui.Context, p palette, ss ...stat) {
	ui.Grid(c).Columns(3).GapX(12).Children(func() {
		for _, s := range ss {
			ui.Column(c).Gap(3).MinWidth(0).Children(func() {
				ui.Text(c, s.label).FontSize(13).TextColor(p.muted).SingleLine()
				ui.Text(c, s.value).FontSize(15).FontWeight(600).TextColor(p.text).FontFeatures("tnum").SingleLine()
			})
		}
	})
}

// pill is a small status in a color's tint.
func pill(c *ui.Context, p palette, icon *ui.SVG, color ui.Color, text string) {
	pillIn(ui.Row(c), c, p, icon, color, text)
}

// cornerPill is a pill at the top right of its parent, beside the label
// of a large number.
func cornerPill(c *ui.Context, p palette, icon *ui.SVG, color ui.Color, text string) {
	pillIn(ui.Row(c).Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(-3), c, p, icon, color, text)
}

func pillIn(e ui.Element, c *ui.Context, p palette, icon *ui.SVG, color ui.Color, text string) {
	e.Gap(5).Padding(3, 9).Radius(999).Background(p.tint(color)).AlignItems(ui.Center).Shrink(0).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(13, 13).TextColor(color)
		}
		ui.Text(c, text).FontSize(12).FontWeight(600).TextColor(color).SingleLine()
	})
}

// dotRows is the height of a strip of dots, in dots.
const dotRows = 6

// dots is a strip of dots: a column of them for each value, lit from the
// bottom as far as the value goes toward the largest, the others faint.
func dots(c *ui.Context, p palette, color ui.Color, values []float64) {
	peak := 0.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	ui.Box(c).Height(66).Radius(12).Background(color.Alpha(0.10)).Draw(func(pt *ui.Painter, r ui.Rect) {
		n := len(values)
		if n == 0 {
			return
		}
		const pad = 12
		w, h := r.W-2*pad, r.H-2*pad
		dx := w / float32(n)
		dy := h / float32(dotRows-1)
		radius := min(dx, dy) * 0.28
		for i, v := range values {
			lit := 0
			if peak > 0 && v > 0 {
				lit = max(1, int(math.Round(v/peak*dotRows)))
			}
			x := r.X + pad + dx*(float32(i)+0.5)
			for row := 0; row < dotRows; row++ {
				y := r.Y + pad + h - dy*float32(row)
				col := color.Alpha(0.22)
				if row < lit {
					col = color
				}
				pt.Fill(ui.Rect{X: x - radius, Y: y - radius, W: 2 * radius, H: 2 * radius}, col, radius)
			}
		}
	})
}

// shareDots is a strip of dots colored by the shares of a whole, as a bar
// of parts is: the columns go to the parts in turn.
func shareDots(c *ui.Context, p palette, colors []ui.Color, shares []float64, columns int) {
	total := 0.0
	for _, s := range shares {
		total += s
	}
	owner := make([]int, columns)
	at, acc := 0, 0.0
	for i := range owner {
		mid := (float64(i) + 0.5) / float64(columns) * total
		for at < len(shares)-1 && acc+shares[at] < mid {
			acc += shares[at]
			at++
		}
		owner[i] = at
	}
	ui.Box(c).Height(66).Radius(12).Background(p.well).Draw(func(pt *ui.Painter, r ui.Rect) {
		if total == 0 {
			return
		}
		const pad = 12
		w, h := r.W-2*pad, r.H-2*pad
		dx := w / float32(columns)
		dy := h / float32(dotRows-1)
		radius := min(dx, dy) * 0.28
		for i, o := range owner {
			x := r.X + pad + dx*(float32(i)+0.5)
			for row := 0; row < dotRows; row++ {
				y := r.Y + pad + h - dy*float32(row)
				pt.Fill(ui.Rect{X: x - radius, Y: y - radius, W: 2 * radius, H: 2 * radius}, colors[o%len(colors)], radius)
			}
		}
	})
}

// donut is a ring of shares, with a text in its middle.
func donut(c *ui.Context, p palette, colors []ui.Color, shares []float64, value, label string) {
	total := 0.0
	for _, s := range shares {
		total += s
	}
	ui.Box(c).Size(112, 112).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		radius := min(r.W, r.H)/2 - 9
		const width = 14
		ring := func(from, to float64, col ui.Color) {
			var path ui.Path
			steps := max(2, int((to-from)*120))
			for i := 0; i <= steps; i++ {
				a := (from + (to-from)*float64(i)/float64(steps)) * 2 * math.Pi
				x := cx + radius*float32(math.Sin(a))
				y := cy - radius*float32(math.Cos(a))
				if i == 0 {
					path.MoveTo(x, y)
				} else {
					path.LineTo(x, y)
				}
			}
			pt.StrokePath(&path, width, col)
		}
		ring(0, 1, p.well)
		if total > 0 {
			at := 0.0
			for i, s := range shares {
				part := s / total
				if part > 0.004 {
					gap := math.Min(0.006, part/4)
					ring(at+gap, at+part-gap, colors[i%len(colors)])
				}
				at += part
			}
		}
		big := ui.Span{Text: value, Size: 17, Weight: 700, Color: p.text}
		bw, bh := pt.MeasureText(0, big)
		small := ui.Span{Text: label, Size: 12, Color: p.muted}
		sw, sh := pt.MeasureText(0, small)
		y := cy - (bh+sh)/2
		pt.RichText(cx-bw/2, y, 0, big)
		pt.RichText(cx-sw/2, y+bh, 0, small)
	})
}

// legend is the rows naming the parts of a donut.
func legend(c *ui.Context, p palette, colors []ui.Color, names, values []string) {
	ui.Column(c).Gap(9).Grow(1).MinWidth(0).Children(func() {
		for i, name := range names {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(9, 9).Radius(5).Background(colors[i%len(colors)]).Shrink(0)
				ui.Text(c, name).FontSize(13).TextColor(p.text).SingleLine().Grow(1)
				ui.Text(c, values[i]).FontSize(13).TextColor(p.muted).FontFeatures("tnum").SingleLine()
			})
		}
	})
}

// tile is a part of a treemap.
type tile struct {
	name, value string
	weight      float64
}

// treemap lays tiles out in a box, each as large as its weight, by
// splitting them in two of about the same weight along the longer side.
func treemap(c *ui.Context, p palette, tiles []tile, height float32) {
	ui.Box(c).Height(height).Draw(func(pt *ui.Painter, r ui.Rect) {
		var place func(ts []tile, r ui.Rect)
		place = func(ts []tile, r ui.Rect) {
			if len(ts) == 0 || r.W < 1 || r.H < 1 {
				return
			}
			if len(ts) == 1 {
				drawTile(pt, p, ts[0], r)
				return
			}
			total := 0.0
			for _, t := range ts {
				total += t.weight
			}
			split, acc := 1, ts[0].weight
			for split < len(ts)-1 && acc+ts[split].weight/2 < total/2 {
				acc += ts[split].weight
				split++
			}
			f := float32(acc / total)
			const gap = 6
			if r.W >= r.H {
				w := (r.W - gap) * f
				place(ts[:split], ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H})
				place(ts[split:], ui.Rect{X: r.X + w + gap, Y: r.Y, W: r.W - w - gap, H: r.H})
			} else {
				h := (r.H - gap) * f
				place(ts[:split], ui.Rect{X: r.X, Y: r.Y, W: r.W, H: h})
				place(ts[split:], ui.Rect{X: r.X, Y: r.Y + h + gap, W: r.W, H: r.H - h - gap})
			}
		}
		place(tiles, r)
	})
}

func drawTile(pt *ui.Painter, p palette, t tile, r ui.Rect) {
	pt.Fill(r, p.well, 10)
	if r.W < 64 || r.H < 40 {
		return
	}
	pt.Clip(r, 10, func() {
		name := ui.Span{Text: t.name, Size: 13, Weight: 600, Color: p.text}
		value := ui.Span{Text: t.value, Size: 12, Color: p.muted}
		_, nh := pt.MeasureText(r.W-20, name)
		_, vh := pt.MeasureText(r.W-20, value)
		y := r.Y + r.H - 10 - nh - vh
		if r.H < 60 {
			y = r.Y + 8
		}
		pt.RichText(r.X+10, y, r.W-20, name)
		pt.RichText(r.X+10, y+nh, r.W-20, value)
	})
}

// bar is a row of a list of bars: a name, a value, and a bar under them.
func bar(c *ui.Context, p palette, color ui.Color, name, value string, frac float64) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, name).FontSize(14).FontWeight(500).TextColor(p.text).SingleLine().Grow(1)
			ui.Text(c, value).FontSize(13).TextColor(p.muted).FontFeatures("tnum").SingleLine()
		})
		ui.Box(c).Height(6).Radius(3).Background(p.well).Draw(func(pt *ui.Painter, r ui.Rect) {
			w := max(r.H, r.W*float32(math.Min(1, math.Max(0, frac))))
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, color, r.H/2)
		})
	})
}

// segmented is a pill of segments, as the screenshot's range control.
func segmented(c *ui.Context, p palette, selected *int, labels ...string) bool {
	seg := ui.SegmentedBase(c, selected, len(labels))
	changed := false
	seg.Track.Padding(3).Radius(10).Background(p.card).Gap(2).Children(func() {
		for i, l := range labels {
			s := seg.Segment(i).Padding(5, 13).Radius(8).Label(l)
			if i == *selected {
				s.Background(p.cardHover.Mix(p.text, 0.12))
			}
			s.Children(func() {
				col := p.muted
				if i == *selected {
					col = p.text
				}
				ui.Text(c, l).FontSize(13).FontWeight(600).TextColor(col)
			})
		}
	})
	if seg.Track.Changed() {
		changed = true
	}
	return changed
}

// iconButton is a round button of an icon, as the screenshot's pause.
func iconButton(c *ui.Context, p palette, icon *ui.SVG, label string) ui.Element {
	b := ui.Box(c).Size(34, 34).Radius(10).Background(p.card).Center().Cursor(ui.CursorPointer).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(p.cardHover.Mix(p.text, 0.08))
	}
	b.Children(func() { ui.Icon(c, icon).Size(16, 16).TextColor(p.muted) })
	return b
}

// row is a line of a card's list: a title, a note under it and what goes
// at its end.
func row(c *ui.Context, p palette, title, note string, end func()) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(9, 0).Children(func() {
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(14).FontWeight(500).TextColor(p.text)
			if note != "" {
				ui.Text(c, note).FontSize(12).TextColor(p.muted).LineHeight(1.35)
			}
		})
		if end != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(end)
		}
	})
}

// switchRow is a row of a setting with a switch at its end, whose title
// toggles it too, as a check box's label does. It reports a change.
func switchRow(c *ui.Context, p palette, title, note string, on *bool, disabled bool) bool {
	changed := false
	ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(9, 0).Children(func() {
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			t := ui.Text(c, title).FontSize(14).FontWeight(500).TextColor(p.text)
			if !disabled {
				t.Cursor(ui.CursorPointer)
				if t.Clicked() {
					*on = !*on
					changed = true
				}
			}
			if note != "" {
				ui.Text(c, note).FontSize(12).TextColor(p.muted).LineHeight(1.35)
			}
		})
		if ui.Switch(c.Key("switch"), on).Label(title).Disabled(disabled).Changed() {
			changed = true
		}
	})
	return changed
}

// banner is a note across the content, in a color's tint.
func banner(c *ui.Context, p palette, icon *ui.SVG, color ui.Color, text string, end func()) {
	ui.Row(c).Gap(10).Padding(12, 16).Radius(14).Background(p.tint(color)).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(18, 18).TextColor(color).Shrink(0)
		ui.Text(c, text).FontSize(13).TextColor(p.text).LineHeight(1.35).Grow(1)
		if end != nil {
			ui.Row(c).Gap(8).Shrink(0).Children(end)
		}
	})
}

// mono is text in the code font, as paths and commands.
func mono(c *ui.Context, p palette, text string) ui.Element {
	return ui.Text(c, text).Font("monospace").FontSize(12).TextColor(p.muted).Selectable()
}

// tokens writes a number of tokens short: 950, 12.3K, 4.5M, 1.2B.
func tokens(n int64) string {
	f := float64(n)
	switch {
	case n < 1000:
		return fmt.Sprint(n)
	case f < 1e6:
		return trimZero(f/1e3) + "K"
	case f < 1e9:
		return trimZero(f/1e6) + "M"
	}
	return trimZero(f/1e9) + "B"
}

func trimZero(f float64) string {
	if f >= 100 {
		return fmt.Sprintf("%.0f", f)
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", f), ".0")
}

// splitNumber splits "12.3M" or "868 MB" into the number and its unit.
func splitNumber(s string) (string, string) {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i], s[i+1:]
	}
	i := len(s)
	for i > 0 && (s[i-1] < '0' || s[i-1] > '9') {
		i--
	}
	return s[:i], s[i:]
}
