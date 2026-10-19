package main

import "github.com/egoist/mygo/ui"

// palette is the colors of the window, light or dark: a sidebar, cards on
// the content, and a color of each kind of thing a card shows.
type palette struct {
	dark                bool
	sidebar, content    ui.Color
	card, cardHover     ui.Color
	well                ui.Color // the inset that holds a card's charts
	text, muted, faint  ui.Color
	border              ui.Color
	shadow              ui.Color // under a card
	lift                ui.Color // under a card raised by the pointer
	highlight           ui.Color // the light along a card's top edge
	selected, selText   ui.Color
	blue, green, red    ui.Color
	orange, teal, amber ui.Color
	purple, gray        ui.Color
	// stage counts the cards of the frame, so that they come in one after
	// another as a page opens.
	stage *stage
}

func paletteOf(t *ui.Theme) palette {
	if t.Dark {
		return palette{
			dark:    true,
			sidebar: ui.Hex("#29292c"), content: ui.Hex("#19191b"),
			card: ui.Hex("#222225"), cardHover: ui.Hex("#29292d"),
			well: ui.Hex("#19191c"),
			text: ui.Hex("#f5f5f7"), muted: ui.Hex("#a1a1a8"), faint: ui.Hex("#65656b"),
			border:    ui.RGBA(255, 255, 255, 0.06),
			shadow:    ui.RGBA(0, 0, 0, 0.32),
			lift:      ui.RGBA(0, 0, 0, 0.55),
			highlight: ui.RGBA(255, 255, 255, 0.045),
			selected:  t.Accent, selText: t.AccentText,
			blue: ui.Hex("#4c8dff"), green: ui.Hex("#4cd964"), red: ui.Hex("#ff5470"),
			orange: ui.Hex("#ffa733"), teal: ui.Hex("#40c8c0"), amber: ui.Hex("#ffd426"),
			purple: ui.Hex("#b18cff"), gray: ui.Hex("#8e8e93"),
			stage: &stage{},
		}
	}
	return palette{
		sidebar: ui.Hex("#ececef"), content: ui.Hex("#f4f4f6"),
		card: ui.Hex("#ffffff"), cardHover: ui.Hex("#fafafc"),
		well: ui.Hex("#f1f1f4"),
		text: ui.Hex("#1d1d1f"), muted: ui.Hex("#6e6e73"), faint: ui.Hex("#a7a7ad"),
		border:    ui.RGBA(0, 0, 0, 0.06),
		shadow:    ui.RGBA(20, 20, 40, 0.06),
		lift:      ui.RGBA(20, 20, 40, 0.16),
		highlight: ui.Transparent,
		selected:  t.Accent, selText: t.AccentText,
		blue: ui.Hex("#1f6fff"), green: ui.Hex("#20a83e"), red: ui.Hex("#e8344f"),
		orange: ui.Hex("#e8840c"), teal: ui.Hex("#11a39b"), amber: ui.Hex("#c99700"),
		purple: ui.Hex("#7c4dff"), gray: ui.Hex("#8e8e93"),
		stage: &stage{},
	}
}

// tint is a color faded into a card's background, for an icon's square.
func (p palette) tint(c ui.Color) ui.Color {
	if p.dark {
		return c.Alpha(0.18)
	}
	return c.Alpha(0.12)
}

// hover is the background of a control under the pointer.
func (p palette) hover() ui.Color { return p.text.Alpha(0.06) }
