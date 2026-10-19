package main

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/shell"
	"ccguanjia/locales"
)

// command is a row of the command palette: what it does, and how to find
// it.
type command struct {
	key         string
	icon        *ui.SVG
	color       func(palette) ui.Color
	title, hint string
	run         func()
}

// paletteRows is how many rows the palette shows at most.
const paletteRows = 8

func (a *app) openPalette() {
	a.paletteOpen, a.paletteQuery, a.paletteSel = true, "", 0
}

// commands are everything the palette can do: the pages, each profile's
// page and its start in the terminal, and the actions of the window.
func (a *app) commands() []command {
	cmds := []command{
		{"overview", iconOverview, func(p palette) ui.Color { return p.gray }, locales.PageOverview(), "⌘1", func() { a.show(pageOverview, "") }},
		{"install", iconPackage, func(p palette) ui.Color { return p.red }, locales.PageInstall(), "⌘2", func() { a.show(pageInstall, "") }},
		{"cleanup", iconSparkles, func(p palette) ui.Color { return p.amber }, locales.PageCleanup(), "⌘3", func() { a.show(pageCleanup, "") }},
		{"settings", iconGear, func(p palette) ui.Color { return p.gray }, locales.MenuSettings(), "⌘,", func() { a.show(pageSettings, "") }},
	}
	for _, pr := range a.profiles {
		hint := ""
		if pr.Account != nil {
			hint = pr.Account.Email
		}
		cmds = append(cmds, command{"profile-" + pr.Name, iconUser, func(p palette) ui.Color { return p.blue },
			locales.OpenProfile(pr.Name), hint, func() { a.show(pageProfile, pr.Name) }})
	}
	for _, pr := range a.profiles {
		launcher := "claude"
		if !pr.Default {
			launcher = shell.FunctionName(pr.Name)
		}
		cmds = append(cmds, command{"start-" + pr.Name, iconTerminal, func(p palette) ui.Color { return p.green },
			locales.StartProfile(pr.Name), launcher, func() {
				if a.sync {
					a.lastTerminal = a.launchCommand(pr.Name)
					return
				}
				if err := a.openTerminal(pr.Name); err != nil {
					a.toast(errText(err))
				}
			}})
	}
	cmds = append(cmds,
		command{"new", iconPlus, func(p palette) ui.Color { return p.purple }, locales.NewProfile(), "⌘N", func() {
			a.resetNew()
			a.newOpen = true
		}},
		command{"refresh", iconRefresh, func(p palette) ui.Color { return p.teal }, locales.Refresh(), "⌘R", a.refresh},
		command{"updates", iconDownload, func(p palette) ui.Color { return p.orange }, locales.CheckForUpdates(), "CCGuanjia " + appVersion(), func() {
			if !a.sync {
				updater.CheckForUpdates()
			}
		}},
	)
	return cmds
}

// matchScore says how well a command matches what was typed: 0 for not at
// all, and more for a match at the start of its title or of a word in it
// than for its letters spread through it.
func matchScore(query string, cmd command) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 1
	}
	title := strings.ToLower(cmd.title)
	hint := strings.ToLower(cmd.hint)
	switch {
	case strings.HasPrefix(title, q):
		return 100
	case strings.Contains(" "+title, " "+q):
		return 80
	case strings.Contains(title, q):
		return 60
	case strings.Contains(hint, q):
		return 40
	}
	// The letters in order, anywhere.
	at := 0
	for _, r := range q {
		i := strings.IndexRune(title[at:], r)
		if i < 0 {
			return 0
		}
		at += i + utf8.RuneLen(r)
	}
	return 20
}

// paletteMatches are the commands matching the query, best first, those
// matching as well in their order.
func (a *app) paletteMatches() []command {
	var out []command
	var scores []int
	for _, cmd := range a.commands() {
		if s := matchScore(a.paletteQuery, cmd); s > 0 {
			i := len(out)
			for i > 0 && scores[i-1] < s {
				i--
			}
			out = append(out[:i], append([]command{cmd}, out[i:]...)...)
			scores = append(scores[:i], append([]int{s}, scores[i:]...)...)
		}
	}
	return out
}

// palette is the command palette: a field, and the commands matching what
// is typed in it, the one chosen run with Return or a click.
func (a *app) palette(c *ui.Context, p palette) {
	if !a.paletteOpen {
		return
	}
	matches := a.paletteMatches()
	a.paletteSel = min(max(a.paletteSel, 0), max(len(matches)-1, 0))
	run := func(cmd command) {
		a.paletteOpen = false
		cmd.run()
	}
	ui.DialogBase(c, &a.paletteOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.28)).Justify(ui.Start).Padding(110, 0, 0)
		panel.Width(580).Radius(16).Background(p.card).BorderWidth(1).BorderColor(p.border).
			Shadow(0, 24, 60, -10, ui.RGBA(0, 0, 0, 0.4)).Clip()
		(&stage{}).enter(panel)

		if c.Shortcut(0, ui.KeyDown) || c.Shortcut(ui.Ctrl, ui.KeyN) {
			a.paletteSel = (a.paletteSel + 1) % max(len(matches), 1)
		}
		if c.Shortcut(0, ui.KeyUp) || c.Shortcut(ui.Ctrl, ui.KeyP) {
			a.paletteSel = (a.paletteSel - 1 + max(len(matches), 1)) % max(len(matches), 1)
		}
		submitted := false
		ui.Row(c).Gap(10).Padding(14, 18).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(p.border).Children(func() {
			ui.Icon(c, iconSearch).Size(18, 18).TextColor(p.muted).Shrink(0)
			in := ui.TextInput(c.Key("palette-query"), &a.paletteQuery).Placeholder(locales.SearchCommands()).
				Label(locales.SearchCommands()).AutoFocus().Grow(1).FontSize(16).
				Background(ui.Transparent).BorderWidth(0).Padding(4, 0)
			if in.Changed() {
				a.paletteSel = 0
			}
			submitted = in.Submitted()
		})
		hovered := ""
		ui.Column(c).Padding(6).Gap(2).Children(func() {
			defer func() { a.paletteHover = hovered }()
			if len(matches) == 0 {
				ui.Text(c, locales.NoMatches()).FontSize(13).TextColor(p.muted).Padding(14, 12)
				return
			}
			for i, cmd := range matches {
				if i == paletteRows {
					break
				}
				r := ui.Row(c.Key(cmd.key)).Gap(12).Padding(8, 12).Radius(10).AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Label(cmd.title).Transition(quick)
				// The pointer chooses the row it comes onto, and the keys
				// choose while it stays.
				if r.Hovered() {
					hovered = cmd.key
					if a.paletteHover != cmd.key {
						a.paletteSel = i
					}
				}
				sel := i == a.paletteSel
				if sel {
					r.Background(p.tint(p.selected))
				}
				r.OnClick(func() { run(cmd) }).Children(func() {
					iconSquare(c, p, cmd.icon, cmd.color(p), 26)
					ui.Text(c, cmd.title).FontSize(14).FontWeight(500).TextColor(p.text).SingleLine().Grow(1).Shrink(1).MinWidth(0)
					if cmd.hint != "" {
						ui.Text(c, cmd.hint).FontSize(12).TextColor(p.muted).SingleLine().Shrink(0)
					}
					if sel {
						ui.Text(c, "↩").FontSize(13).TextColor(p.muted).Shrink(0)
					}
				})
			}
		})
		ui.Row(c).Padding(9, 18).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Background(p.well.Alpha(0.5)).Children(func() {
			ui.Text(c, locales.PaletteHint()).FontSize(11).TextColor(p.faint).SingleLine()
		})
		if submitted && len(matches) > 0 {
			run(matches[a.paletteSel])
		}
	})
}
