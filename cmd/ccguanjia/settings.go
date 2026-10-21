package main

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/config"
	"ccguanjia/internal/shell"
	"ccguanjia/locales"
)

// languages are the choices of Settings' language, with their tags.
var languages = []struct{ tag, name string }{
	{"", ""},
	{"en", "English"},
	{"zh-Hans", "简体中文"},
}

// settingsPage is the shell file, the ccgj command, the default profile,
// the terminal, the language, and what the app is.
func (a *app) settingsPage(c *ui.Context, p palette) {
	ui.Grid(c).ColumnTracks(ui.Fr(1), ui.Fr(1)).Gap(20).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Gap(20).MinWidth(0).Children(func() {
			card(c, p, iconTerminal, p.green, locales.CardShell(), "", nil, func() {
				on := a.shellInstalled
				if switchRow(c.Key("shell"), p, locales.ShellFile(), locales.ShellFileNote(), &on, false) {
					a.setShell(on)
				}
				mono(c, p, a.shortPath(filepath.Join(a.cfgDir, shell.FileName)))
				if a.shellInstalled {
					mono(c, p, shell.SourceLine(a.env.Home, filepath.Join(a.cfgDir, shell.FileName)))
					ui.Text(c, locales.ShellReload()).FontSize(12).TextColor(p.faint)
				}
				on = a.cliInstalled
				if switchRow(c.Key("cli"), p, locales.CommandLine(), locales.CommandLineNote(a.shortPath(binDir(a.env.Home))), &on, false) {
					a.setCLI(on)
				}
				if a.cliInstalled && !onPath(os.Getenv("PATH"), binDir(a.env.Home)) && !a.binOnShellPath() {
					ui.Text(c, locales.NotOnPath(a.shortPath(binDir(a.env.Home)))).FontSize(12).TextColor(p.orange)
				}
			})
			card(c, p, iconUsers, p.purple, locales.CardProfiles(), "", nil, func() {
				def := a.cfg.Default
				if def == "" {
					def = claude.DefaultName
				}
				row(c, p, locales.DefaultProfile(), locales.DefaultForShellsNote(), func() {
					if ui.Select(c.Key("default"), &def, a.profileNames()).Label(locales.DefaultProfile()).Width(160).Changed() {
						a.setDefault(def)
					}
				})
				term := a.cfg.TerminalApp()
				row(c, p, locales.TerminalApp(), locales.TerminalAppNote(), func() {
					if ui.Select(c.Key("terminal"), &term, config.Terminals).Label(locales.TerminalApp()).Width(160).Changed() {
						a.cfg.Terminal = term
						a.saveConfig()
					}
				})
			})
		})
		ui.Column(c).Gap(20).MinWidth(0).Children(func() {
			card(c, p, iconGear, p.gray, locales.CardGeneral(), "", nil, func() {
				names := make([]string, len(languages))
				chosen := ""
				for i, l := range languages {
					names[i] = l.name
					if i == 0 {
						names[i] = locales.LanguageSystem()
					}
				}
				chosen = names[0]
				for i, l := range languages {
					if l.tag != "" && l.tag == a.cfg.Language {
						chosen = names[i]
					}
				}
				row(c, p, locales.Language(), "", func() {
					if ui.Select(c.Key("lang"), &chosen, names).Label(locales.Language()).Width(160).Changed() {
						for i, n := range names {
							if n == chosen {
								a.cfg.Language = languages[i].tag
								a.saveConfig()
								if !a.sync {
									mygo.App.SetLocale(languages[i].tag)
								}
							}
						}
					}
				})
				row(c, p, locales.Updates(), "CCGuanjia "+appVersion(), func() {
					ui.Button(c.Key("updates"), locales.CheckForUpdates()).OnClick(func() {
						if !a.sync {
							updater.CheckForUpdates()
						}
					})
				})
				row(c, p, "GitHub", homepage, func() {
					ui.Button(c.Key("github"), locales.Open()).OnClick(func() { c.OpenURL(homepage) })
				})
			})
			card(c, p, iconCommand, p.blue, locales.CardShortcuts(), "", nil, func() {
				ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
					for _, sc := range []struct{ name, keys string }{
						{locales.ShortcutPalette(), "⌘ K"},
						{locales.NewProfile(), "⌘ N"},
						{locales.ShortcutPages(), "⌘ 1 · 2 · 3 · 4"},
						{locales.ShortcutStep(), "⌥ ⌘ ↓ ↑"},
						{locales.Refresh(), "⌘ R"},
						{locales.MenuSettings(), "⌘ ,"},
					} {
						ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(8, 0).Children(func() {
							ui.Text(c, sc.name).FontSize(13).TextColor(p.text).Grow(1).Shrink(1).MinWidth(0).SingleLine()
							ui.Box(c).Padding(3, 8).Radius(6).Background(p.well).BorderWidth(1).BorderColor(p.border).Shrink(0).Children(func() {
								ui.Text(c, sc.keys).FontSize(12).FontWeight(600).TextColor(p.muted)
							})
						})
					}
				})
			})
			card(c, p, iconShield, p.blue, locales.CardPrinciples(), "", nil, func() {
				for _, s := range []string{locales.Principle1(), locales.Principle2(), locales.Principle3(), locales.Principle4()} {
					ui.Row(c).Gap(8).Children(func() {
						ui.Icon(c, iconCheck).Size(14, 14).TextColor(p.green).Shrink(0).Padding(2, 0, 0)
						ui.Text(c, s).FontSize(13).TextColor(p.text).LineHeight(1.4).Grow(1)
					})
				}
			})
		})
	})
}

// binOnShellPath reports whether the login shell's PATH has ~/.local/bin,
// as the Install page found it.
func (a *app) binOnShellPath() bool {
	if a.inst == nil {
		return false
	}
	for _, in := range a.inst.Installs {
		if filepath.Dir(in.Path) == binDir(a.env.Home) {
			return true
		}
	}
	return false
}
