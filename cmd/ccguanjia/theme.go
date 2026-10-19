package main

import "github.com/egoist/mygo/ui"

// palette is the colors of the window, light or dark: a sidebar, cards on
// the content, and a color of each kind of thing a card shows.
type palette struct {
	dark                bool
	sidebar, content    ui.Color
	card, cardHover     ui.Color
	well                ui.Color // the inset that holds a card's dots
	text, muted, faint  ui.Color
	border              ui.Color
	selected, selText   ui.Color
	blue, green, red    ui.Color
	orange, teal, amber ui.Color
	purple, gray        ui.Color
}

func paletteOf(t *ui.Theme) palette {
	if t.Dark {
		return palette{
			dark:    true,
			sidebar: ui.Hex("#2a2a2c"), content: ui.Hex("#1c1c1e"),
			card: ui.Hex("#232325"), cardHover: ui.Hex("#29292b"),
			well: ui.Hex("#1a1a1c"),
			text: ui.Hex("#f5f5f7"), muted: ui.Hex("#a1a1a6"), faint: ui.Hex("#636366"),
			border:   ui.RGBA(255, 255, 255, 0.07),
			selected: t.Accent, selText: t.AccentText,
			blue: ui.Hex("#4c8dff"), green: ui.Hex("#4cd964"), red: ui.Hex("#ff5470"),
			orange: ui.Hex("#ffa733"), teal: ui.Hex("#40c8c0"), amber: ui.Hex("#ffd426"),
			purple: ui.Hex("#b18cff"), gray: ui.Hex("#8e8e93"),
		}
	}
	return palette{
		sidebar: ui.Hex("#ececee"), content: ui.Hex("#f5f5f7"),
		card: ui.Hex("#ffffff"), cardHover: ui.Hex("#fbfbfd"),
		well: ui.Hex("#f2f2f5"),
		text: ui.Hex("#1d1d1f"), muted: ui.Hex("#6e6e73"), faint: ui.Hex("#aeaeb2"),
		border:   ui.RGBA(0, 0, 0, 0.08),
		selected: t.Accent, selText: t.AccentText,
		blue: ui.Hex("#1f6fff"), green: ui.Hex("#20a83e"), red: ui.Hex("#e8344f"),
		orange: ui.Hex("#e8840c"), teal: ui.Hex("#11a39b"), amber: ui.Hex("#c99700"),
		purple: ui.Hex("#7c4dff"), gray: ui.Hex("#8e8e93"),
	}
}

// tint is a color faded into a card's background, for an icon's square.
func (p palette) tint(c ui.Color) ui.Color {
	if p.dark {
		return c.Alpha(0.18)
	}
	return c.Alpha(0.13)
}
