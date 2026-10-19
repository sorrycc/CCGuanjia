package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// iconSquare is an icon in a rounded square of its color's tint.
func iconSquare(c *ui.Context, p palette, icon *ui.SVG, color ui.Color, size float32) {
	ui.Box(c).Size(size, size).Radius(size * 0.3).Background(p.tint(color)).Center().Shrink(0).Children(func() {
		ui.Icon(c, icon).Size(size*0.56, size*0.56).TextColor(color)
	})
}

// panel is the box of a card: its background, a hairline border and a
// soft shadow, coming in as the page opens.
func panel(c *ui.Context, p palette) ui.Element {
	box := ui.Column(c).Radius(16).Background(p.card).BorderWidth(1).BorderColor(p.border).
		Shadow(0, 1, 2, 0, p.shadow).Shadow(0, 8, 24, -12, p.shadow).
		InsetShadow(0, 1, 0, 0, p.highlight).MinWidth(0)
	p.stage.enter(box)
	return box
}

// card is a panel of the content. With onOpen, its title row shows an
// arrow, and a click on it opens what the card is about.
func card(c *ui.Context, p palette, icon *ui.SVG, color ui.Color, title, extra string, onOpen func(), body func()) ui.Element {
	box := panel(c, p).Padding(18, 20, 20).Gap(14)
	box.Children(func() {
		head := ui.Row(c).Gap(11).AlignItems(ui.Center)
		hovered := false
		if onOpen != nil {
			head.Cursor(ui.CursorPointer).OnClick(onOpen).Label(title)
			hovered = head.Hovered()
		}
		head.Children(func() {
			iconSquare(c, p, icon, color, 28)
			ui.Text(c, title).FontSize(15).FontWeight(600).TextColor(p.text).SingleLine().Grow(1).Shrink(1).MinWidth(0)
			if extra != "" {
				ui.Text(c, extra).FontSize(12).TextColor(p.muted).FontFeatures("tnum").SingleLine().Shrink(0)
			}
			if onOpen != nil {
				col := p.faint
				pose := ui.Pose{}
				if hovered {
					col, pose = p.muted, ui.Pose{X: 3}
				}
				ui.Icon(c, iconChevron).Size(16, 16).TextColor(col).AnimatePose(pose, ui.Snappy)
			}
		})
		body()
	})
	if onOpen != nil {
		pose := ui.Pose{}
		if box.Hovered() {
			box.Background(p.cardHover).Shadow(0, 14, 30, -14, p.lift)
			pose.Y = -2
		}
		box.Transition(quick).AnimatePose(pose, ui.Snappy)
	}
	return box
}

// bigValue is a card's large number, with its unit after it.
func bigValue(c *ui.Context, p palette, label, value, unit string) {
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(p.muted).SingleLine().Padding(0, 150, 0, 0)
		ui.Row(c).Gap(4).AlignItems(ui.End).Children(func() {
			size := float32(38)
			if len([]rune(value)) > 5 {
				size = 31
			}
			ui.Text(c, value).FontSize(size).FontWeight(700).TextColor(p.text).FontFeatures("tnum").
				LetterSpacing(-0.5).SingleLine().Shrink(1).ContentTransition(numeric)
			if unit != "" {
				ui.Text(c, unit).FontSize(17).FontWeight(500).TextColor(p.muted).Padding(0, 0, 6).SingleLine()
			}
		})
	})
}

// skeleton is a block standing in for what is still being read, glowing
// softly.
func skeleton(c *ui.Context, p palette, w, h float32) {
	b := ui.Box(c).Height(h).Radius(min(h/2, 8)).Background(p.text.Alpha(0.07))
	if w > 0 {
		b.Width(w)
	}
	b.Opacity(0.55 + 0.45*b.Loop("pulse", 1400*time.Millisecond, ui.Bounce(ui.EaseInOut)))
}

// stat is a label and a value under a card's large number.
type stat struct{ label, value string }

// stats are three stats side by side.
func stats(c *ui.Context, p palette, ss ...stat) {
	ui.Grid(c).Columns(3).GapX(12).Children(func() {
		for _, s := range ss {
			ui.Column(c).Gap(3).MinWidth(0).Children(func() {
				ui.Text(c, s.label).FontSize(12).TextColor(p.muted).SingleLine()
				ui.Text(c, s.value).FontSize(15).FontWeight(600).TextColor(p.text).FontFeatures("tnum").SingleLine().
					Tooltip(s.value).ContentTransition(numeric)
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
			ui.Icon(c, icon).Size(12, 12).TextColor(color)
		}
		ui.Text(c, text).FontSize(12).FontWeight(600).TextColor(color).SingleLine()
	})
}

// chartHeight is the height of a card's strip of columns.
const chartHeight = 72

// columns is a strip of rounded columns, one for each value, as tall as
// the value is toward the largest. They grow in the first time they show,
// and to their new heights as the values change; the one under the
// pointer lights up and tip says what it is.
func columns(c *ui.Context, p palette, color ui.Color, values []float64, tip func(i int) string) {
	peak := 0.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	box := ui.Box(c).Height(chartHeight).Radius(12).Background(color.Alpha(0.08))
	n := len(values)
	if n == 0 {
		return
	}
	g := grow(box)
	heights := make([]float32, n)
	for i, v := range values {
		f := float32(0)
		if peak > 0 {
			f = float32(v / peak)
		}
		heights[i] = box.AnimateTo(i, f, ui.Smooth) * easeOutCubic(stagger(g, i, n))
	}
	const pad = 10
	px, _, over := box.PointerPosition()
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		w, h := r.W-2*pad, r.H-2*pad
		dx := w / float32(n)
		bw := max(2, min(dx*0.58, 9))
		hover := -1
		if over && tip != nil {
			hover = min(max(int((px-pad)/dx), 0), n-1)
		}
		for i, f := range heights {
			x := r.X + pad + dx*(float32(i)+0.5) - bw/2
			bh := max(bw, h*f)
			col := color
			switch {
			case hover >= 0 && i != hover:
				col = color.Alpha(0.45)
			case f <= 0:
				col = color.Alpha(0.2)
				bh = bw
			}
			bar := ui.Rect{X: x, Y: r.Y + pad + h - bh, W: bw, H: bh}
			if f > 0 && (hover < 0 || i == hover) {
				pt.FillGradient(bar, ui.LinearGradient{From: color, To: color.Alpha(0.62), Angle: 180}, bw/2)
			} else {
				pt.Fill(bar, col, bw/2)
			}
		}
		if hover >= 0 {
			tooltipAt(pt, p, tip(hover), r.X+pad+dx*(float32(hover)+0.5), r.Y+6, r)
		}
	})
}

// tooltipAt paints a small label centered on x, held within the box r.
func tooltipAt(pt *ui.Painter, p palette, text string, x, y float32, r ui.Rect) {
	s := ui.Span{Text: text, Size: 11, Weight: 600, Color: p.text}
	tw, th := pt.MeasureText(0, s)
	w, h := tw+14, th+6
	left := min(max(x-w/2, r.X+4), r.X+r.W-w-4)
	box := ui.Rect{X: left, Y: y, W: w, H: h}
	pt.Shadow(box, 7, 0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.18))
	pt.Fill(box, p.card, 7)
	pt.Stroke(box, p.border, 7, 1)
	pt.RichText(left+7, y+3, 0, s)
}

// shareBar is a capsule split into the shares of a whole, each part in
// its color, drawing itself from the left the first time it shows.
func shareBar(c *ui.Context, p palette, colors []ui.Color, shares []float64) {
	total := 0.0
	for _, s := range shares {
		total += s
	}
	box := ui.Box(c).Height(12)
	g := easeOutCubic(grow(box))
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(r, p.well, r.H/2)
		if total == 0 {
			return
		}
		pt.Clip(ui.Rect{X: r.X, Y: r.Y, W: r.W * g, H: r.H}, r.H/2, func() {
			x := r.X
			for i, s := range shares {
				w := r.W * float32(s/total)
				if w < 0.5 {
					continue
				}
				part := ui.Rect{X: x, Y: r.Y, W: max(w-2, 1), H: r.H}
				pt.Fill(part, colors[i%len(colors)], 2)
				x += w
			}
		})
	})
}

// donut is a ring of shares, with a text in its middle, sweeping round the
// first time it shows.
func donut(c *ui.Context, p palette, colors []ui.Color, shares []float64, value, label string) {
	total := 0.0
	for _, s := range shares {
		total += s
	}
	box := ui.Box(c).Size(120, 120).Shrink(0)
	g := float64(easeOutCubic(grow(box)))
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		radius := min(r.W, r.H)/2 - 9
		const width = 13
		ring := func(from, to float64, col ui.Color) {
			if to <= from {
				return
			}
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
					gap := math.Min(0.007, part/4)
					ring((at+gap)*g, math.Min(at+part-gap, g), colors[i%len(colors)])
				}
				at += part
			}
		}
		big := ui.Span{Text: value, Size: 17, Weight: 700, Color: p.text}
		bw, bh := pt.MeasureText(0, big)
		small := ui.Span{Text: label, Size: 11, Color: p.muted}
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
				ui.Box(c).Size(8, 8).Radius(4).Background(colors[i%len(colors)]).Shrink(0)
				ui.Text(c, name).FontSize(13).TextColor(p.text).SingleLine().Grow(1).Shrink(1).MinWidth(0).Tooltip(name)
				ui.Text(c, values[i]).FontSize(12).TextColor(p.muted).FontFeatures("tnum").SingleLine().Shrink(0)
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
// The tiles take the colors in turn, and the one under the pointer lights
// up.
func treemap(c *ui.Context, p palette, colors []ui.Color, tiles []tile, height float32) {
	box := ui.Box(c).Height(height)
	g := grow(box)
	px, py, over := box.PointerPosition()
	box.Draw(func(pt *ui.Painter, root ui.Rect) {
		at := 0
		var place func(ts []tile, r ui.Rect)
		place = func(ts []tile, r ui.Rect) {
			if len(ts) == 0 || r.W < 1 || r.H < 1 {
				return
			}
			if len(ts) == 1 {
				i := at
				at++
				x, y := root.X+px, root.Y+py
				hot := over && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
				drawTile(pt, p, colors[i%len(colors)], ts[0], r, easeOutCubic(stagger(g, i, len(tiles))), hot)
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
		place(tiles, root)
	})
}

func drawTile(pt *ui.Painter, p palette, color ui.Color, t tile, r ui.Rect, shown float32, hot bool) {
	if shown <= 0 {
		return
	}
	a := float32(0.16)
	if p.dark {
		a = 0.2
	}
	if hot {
		a += 0.1
	}
	pt.Fill(r, color.Alpha(a*shown), 10)
	if r.W < 64 || r.H < 40 {
		return
	}
	pt.Clip(r, 10, func() {
		name := ui.Span{Text: t.name, Size: 13, Weight: 600, Color: p.text.Alpha(shown)}
		value := ui.Span{Text: t.value, Size: 12, Color: p.muted.Alpha(shown)}
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

// bar is a row of a list of bars: a name, a value, and a bar under them,
// which grows in the first time it shows and to its new length after.
func bar(c *ui.Context, p palette, color ui.Color, name, value string, frac float64) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, name).FontSize(13).FontWeight(500).TextColor(p.text).SingleLine().Grow(1).Shrink(1).MinWidth(0).Tooltip(name)
			ui.Text(c, value).FontSize(12).TextColor(p.muted).FontFeatures("tnum").SingleLine().Shrink(0).ContentTransition(numeric)
		})
		track := ui.Box(c).Height(6).Radius(3).Background(p.well)
		f := track.AnimateTo("f", float32(math.Min(1, math.Max(0, frac))), ui.Smooth) * easeOutCubic(grow(track))
		track.Draw(func(pt *ui.Painter, r ui.Rect) {
			w := max(r.H, r.W*f)
			pt.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, ui.LinearGradient{From: color.Alpha(0.7), To: color, Angle: 90}, r.H/2)
		})
	})
}

// segmented is a pill of segments of the same width, whose thumb glides to
// the one chosen.
func segmented(c *ui.Context, p palette, selected *int, labels ...string) bool {
	seg := ui.SegmentedBase(c, selected, len(labels))
	const segW, gap, pad = float32(54), float32(2), float32(3)
	idx := seg.Track.AnimateTo("thumb", float32(*selected), glide)
	thumb := p.cardHover.Mix(p.text, 0.12)
	if !p.dark {
		thumb = p.card
	}
	well := p.card
	if !p.dark {
		well = p.text.Alpha(0.06)
	}
	seg.Track.Padding(pad).Radius(10).Background(well).Gap(gap).Draw(func(pt *ui.Painter, r ui.Rect) {
		x := r.X + pad + idx*(segW+gap)
		box := ui.Rect{X: x, Y: r.Y + pad, W: segW, H: r.H - 2*pad}
		if !p.dark {
			pt.Shadow(box, 8, 0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
		}
		pt.Fill(box, thumb, 8)
	}).Children(func() {
		for i, l := range labels {
			s := seg.Segment(i).Width(segW).Padding(5, 0).Radius(8).Center().Label(l)
			col := p.muted
			if i == *selected {
				col = p.text
			} else if s.Hovered() {
				col = p.text.Alpha(0.8)
			}
			s.Children(func() {
				t := ui.Text(c, l).FontSize(12).FontWeight(600).SingleLine()
				t.TextColor(t.AnimateColor("col", col, ui.Animation{Duration: 160 * time.Millisecond}))
			})
		}
	})
	return seg.Track.Changed()
}

// iconButton is a square button of an icon.
func iconButton(c *ui.Context, p palette, icon *ui.SVG, label string) ui.Element {
	return spinButton(c, p, icon, label, false)
}

// spinButton is an icon button whose icon turns while spin is set, as
// refreshing does while there is still something to read.
func spinButton(c *ui.Context, p palette, icon *ui.SVG, label string, spin bool) ui.Element {
	b := ui.Box(c).Size(34, 34).Radius(10).Background(p.card).BorderWidth(1).BorderColor(p.border).
		Center().Cursor(ui.CursorPointer).Label(label).Tooltip(label).Transition(quick)
	pose := ui.Pose{}
	if b.Hovered() {
		b.Background(p.cardHover.Mix(p.text, 0.06))
	}
	if b.Pressed() {
		pose.Scale = 0.92
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() {
		i := ui.Icon(c, icon).Size(16, 16).TextColor(p.muted)
		if spin {
			i.Rotate(i.Loop("spin", 900*time.Millisecond, ui.Linear) * 360)
		}
	})
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
	ui.Row(c).Gap(10).Padding(12, 16).Radius(14).Background(p.tint(color)).BorderWidth(1).BorderColor(color.Alpha(0.18)).
		AlignItems(ui.Center).Transition(ui.ElementTransition{Enter: &ui.Motion{Y: -6}, Exit: &ui.Motion{Y: -6}}).Children(func() {
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

// profileColor is the color of a profile's avatar: the profiles take the
// colors in turn, in the order the sidebar lists them, so that the first
// six differ.
func (a *app) profileColor(p palette, name string) ui.Color {
	colors := []ui.Color{p.blue, p.purple, p.teal, p.orange, p.green, p.red}
	for i, pr := range a.profiles {
		if pr.Name == name {
			return colors[i%len(colors)]
		}
	}
	return p.gray
}

// initial is the first letter of a name, as an avatar shows it.
func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}

// avatar is a profile's initial on a square of its color, with a dot
// breathing at its corner while the profile is working.
func avatar(c *ui.Context, p palette, name string, color ui.Color, size float32, live bool, ring ui.Color) {
	ui.Box(c).Size(size, size).Radius(size * 0.3).Shrink(0).Center().
		LinearGradient(ui.LinearGradient{From: color.Mix(ui.Hex("#ffffff"), 0.18), To: color, Angle: 180}).
		Children(func() {
			ui.Text(c, initial(name)).FontSize(size * 0.5).FontWeight(700).TextColor(ui.Hex("#ffffff")).SingleLine()
			if live {
				liveDot(ui.Box(c).Absolute().Right(-5).Bottom(-5), c, p, 8, ring)
			}
		})
}

// liveDot is a small green dot for what is working now, with a halo
// breathing out of it a few times as it appears. It then stands still, so
// that it draws no frames while it stays; Reduce Motion stills it at once.
func liveDot(e ui.Element, c *ui.Context, p palette, size float32, ring ui.Color) {
	const breaths = 3
	tl := e.Timeline("live", ui.TimelineOptions{})
	v := tl.Tween(ui.Tween{From: 0, To: breaths, Duration: breaths * 1800 * time.Millisecond, Ease: ui.Linear}, ui.AtTime(0))
	t := float32(1)
	if v < breaths {
		t = easeOutCubic(v - float32(math.Floor(float64(v))))
	}
	e.Size(size+4, size+4).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		halo := size/2 + 5*t
		pt.Fill(ui.Rect{X: cx - halo, Y: cy - halo, W: 2 * halo, H: 2 * halo}, p.green.Alpha(0.35*(1-t)), halo)
		if ring.A > 0 {
			o := size/2 + 2
			pt.Fill(ui.Rect{X: cx - o, Y: cy - o, W: 2 * o, H: 2 * o}, ring, o)
		}
		d := size / 2
		pt.Fill(ui.Rect{X: cx - d, Y: cy - d, W: size, H: size}, p.green, d)
	})
}

// change says how much cur is above or below prev, as "+12%", and whether
// it went up. ok is false when there is nothing to compare with.
func change(cur, prev float64) (text string, up, ok bool) {
	if prev <= 0 {
		return "", false, false
	}
	f := (cur - prev) / prev * 100
	switch {
	case f >= 999:
		return fmt.Sprintf("%.0f×", cur/prev), true, true
	case math.Abs(f) < 0.5:
		return "±0%", true, true
	case f > 0:
		return fmt.Sprintf("+%.0f%%", f), true, true
	}
	return fmt.Sprintf("−%.0f%%", -f), false, true
}

// deltaPill is a corner pill of how a number changed from the window
// before, with what it is compared with under the pointer.
func deltaPill(c *ui.Context, p palette, cur, prev float64, than string) {
	text, up, ok := change(cur, prev)
	if !ok {
		return
	}
	icon, color := iconTrendUp, p.green
	if !up {
		icon, color = iconTrendDown, p.orange
	}
	e := ui.Row(c).Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(-3).Tooltip(than)
	pillIn(e, c, p, icon, color, text)
}
