package main

import (
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/clean"
	"ccguanjia/internal/install"
	"ccguanjia/internal/settings"
	"ccguanjia/locales"
)

const sidebarWidth = 260

// view is the window: the sidebar of pages and profiles at the left, and
// the page under its header.
func (a *app) view(c *ui.Context) {
	t := c.Theme()
	p := paletteOf(t)
	for _, s := range a.toasts {
		c.Toast(s)
	}
	a.toasts = nil
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		side := ui.Column(c).Width(sidebarWidth).Shrink(0).BorderWidth(0, 1, 0, 0).BorderColor(p.border)
		if !c.Vibrancy() {
			side.Background(p.sidebar)
		}
		side.Children(func() { a.sidebar(c, p) })
		ui.Column(c).Grow(1).MinWidth(0).Background(p.content).Children(func() {
			a.header(c, p)
			scope := a.current
			if a.page == pageOverview && a.overviewAll {
				scope = "*"
			}
			ui.Scroll(c.Key(a.page+"/"+scope)).Grow(1).Padding(4, 24, 28).Gap(20).Children(func() {
				switch a.page {
				case pageOverview:
					a.overview(c, p)
				case pageProfile:
					a.profilePage(c, p)
				case pageInstall:
					a.installPage(c, p)
				case pageCleanup:
					a.cleanupPage(c, p)
				case pageSettings:
					a.settingsPage(c, p)
				case pageClaudeSettings:
					a.claudeSettingsPage(c, p)
				}
			})
		})
	})
	a.newProfileDialog(c, p)
	a.palette(c, p)
	a.shortcuts(c)
}

// shortcuts are the keys of the window. The View menu has ⌘K and ⌘N as
// well, and takes them first in the app.
func (a *app) shortcuts(c *ui.Context) {
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyK):
		a.openPalette()
	case c.Shortcut(ui.Cmd, ui.KeyN):
		a.resetNew()
		a.newOpen = true
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyDown):
		a.stepProfile(1)
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyUp):
		a.stepProfile(-1)
	}
}

// stepProfile shows the profile after the current one, or before it.
// Overview steps through all of them first.
func (a *app) stepProfile(d int) {
	if len(a.profiles) == 0 {
		return
	}
	i := 0
	for j, pr := range a.profiles {
		if pr.Name == a.current {
			i = j
		}
	}
	if a.page == pageOverview {
		// All is -1, before the first profile.
		if a.overviewAll {
			i = -1
		}
		n := len(a.profiles) + 1
		i = (i+1+d+n)%n - 1
		if i < 0 {
			a.showAll()
		} else {
			a.show(pageOverview, a.profiles[i].Name)
		}
		return
	}
	i = (i + d + len(a.profiles)) % len(a.profiles)
	page := a.page
	if page != pageOverview && page != pageCleanup && page != pageClaudeSettings {
		page = pageProfile
	}
	a.show(page, a.profiles[i].Name)
}

// sidebar is the pages, the profiles, and the version of Claude Code.
func (a *app) sidebar(c *ui.Context, p palette) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 52)).DragWindow()
	ui.Scroll(c).Grow(1).Padding(0, 12).Gap(2).Children(func() {
		a.navItem(c, p, "nav-overview", navIcon(c, iconOverview, p.gray), locales.PageOverview(), "", a.page == pageOverview, func() { a.show(pageOverview, "") })

		a.navSection(c, p, locales.SectionProfiles())
		for _, pr := range a.profiles {
			value := "—"
			if pr.Account != nil && pr.Account.Plan != "" {
				value = pr.Account.Plan
			} else if !pr.SignedIn {
				value = locales.SignedOutShort()
			}
			selected := a.page == pageProfile && a.current == pr.Name
			live := a.live(pr.Name)
			ring := p.sidebar
			if selected {
				ring = p.selected
			}
			lead := func(bool) { avatar(c, p, pr.Name, a.profileColor(p, pr.Name), 22, live, ring) }
			label := pr.Name
			if a.isDefault(pr.Name) {
				label += " ★"
			}
			a.navItem(c, p, "nav-profile-"+pr.Name, lead, label, value, selected, func() { a.show(pageProfile, pr.Name) })
		}
		a.navItem(c, p, "nav-new", navIcon(c, iconPlus, p.gray), locales.NewProfile(), "", false, func() {
			a.resetNew()
			a.newOpen = true
		})

		a.navSection(c, p, locales.SectionMaintenance())
		version := ""
		if a.inst != nil && a.inst.Active() != nil {
			version = a.inst.Active().Version
		}
		if a.updateAvailable() {
			version = "↑ " + a.latest
		}
		a.navItem(c, p, "nav-install", navIcon(c, iconPackage, p.red), locales.PageInstall(), version, a.page == pageInstall, func() { a.show(pageInstall, "") })
		reclaim := ""
		if pl := a.plans[a.current]; pl != nil {
			reclaim = clean.Size(pl.Reclaim())
		}
		a.navItem(c, p, "nav-cleanup", navIcon(c, iconSparkles, p.amber), locales.PageCleanup(), reclaim, a.page == pageCleanup, func() { a.show(pageCleanup, "") })
		changes := ""
		if d := a.csDrafts[a.current]; d != nil {
			if n := len(d.changes()); n > 0 {
				changes = locales.CsChanges(n)
			}
		}
		a.navItem(c, p, "nav-claude-settings", navIcon(c, iconFile, p.blue), locales.PageClaudeSettings(), changes, a.page == pageClaudeSettings, func() { a.show(pageClaudeSettings, "") })
	})
	ui.Row(c).Padding(12, 16, 16, 24).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Children(func() {
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Claude Code").FontSize(13).FontWeight(600).TextColor(p.text).SingleLine()
				if a.updateAvailable() {
					u := ui.Row(c.Key("update-badge")).Cursor(ui.CursorPointer).Label(locales.UpdateTo(a.latest)).Tooltip(locales.UpdateTo(a.latest)).
						Transition(ui.ElementTransition{Enter: &ui.Motion{Scale: 0.6}})
					u.OnClick(func() { a.show(pageInstall, "") })
					pillIn(u, c, p, nil, p.orange, locales.UpdateShort())
				}
			})
			sub := locales.Detecting()
			if a.inst != nil {
				if in := a.inst.Active(); in != nil {
					sub = in.Version + " · " + kindName(string(in.Kind))
				} else {
					sub = locales.NotInstalled()
				}
			}
			ui.Text(c, sub).FontSize(12).TextColor(p.muted).SingleLine()
		})
		g := iconButton(c.Key("settings"), p, iconGear, locales.MenuSettings())
		if a.page == pageSettings {
			g.Background(p.tint(p.selected))
		}
		g.OnClick(func() { a.show(pageSettings, "") })
	})
}

// updateAvailable reports whether a newer Claude Code than the one that
// runs is out.
func (a *app) updateAvailable() bool {
	if a.inst == nil || a.latest == "" {
		return false
	}
	in := a.inst.Active()
	return in != nil && in.Version != "" && install.Compare(in.Version, a.latest) < 0
}

func (a *app) navSection(c *ui.Context, p palette, title string) {
	ui.Text(c, title).FontSize(11).FontWeight(600).LetterSpacing(0.4).TextColor(p.faint).Padding(16, 10, 6)
}

// navIcon is the lead of a sidebar row: an icon in a square of its color,
// fading into the highlight when the row is chosen.
func navIcon(c *ui.Context, icon *ui.SVG, color ui.Color) func(selected bool) {
	return func(selected bool) {
		sq := ui.Box(c).Size(22, 22).Radius(6).Background(color).Center().Shrink(0).Transition(quick)
		if selected {
			sq.Background(ui.Hex("#ffffff").Alpha(0.25))
		}
		sq.Children(func() { ui.Icon(c, icon).Size(13, 13).TextColor(ui.Hex("#ffffff")) })
	}
}

// navMatch is the identity of the sidebar's highlight, which glides from
// the row chosen before to the one chosen now.
type navMatch struct{}

// navItem is a row of the sidebar: what leads it, a label and a value at
// its end.
func (a *app) navItem(c *ui.Context, p palette, key string, lead func(selected bool), label, value string, selected bool, onClick func()) {
	r := ui.Row(c.Key(key)).Height(34).Radius(8).Cursor(ui.CursorPointer).Label(label).Transition(quick)
	text, muted := p.text, p.muted
	switch {
	case selected:
		text, muted = p.selText, p.selText.Alpha(0.8)
	case r.Hovered():
		r.Background(p.hover())
	}
	pose := ui.Pose{}
	if r.Pressed() {
		pose.Scale = 0.98
	}
	r.AnimatePose(pose, ui.Snappy)
	r.OnClick(onClick).Children(func() {
		if selected {
			ui.Box(c.Key("sel")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Radius(8).
				LinearGradient(ui.LinearGradient{From: p.selected.Mix(ui.Hex("#ffffff"), 0.1), To: p.selected, Angle: 180}).
				Shadow(0, 1, 3, 0, p.selected.Alpha(0.35)).
				Transition(ui.ElementTransition{Animation: &glide, Match: navMatch{}})
		}
		ui.Row(c.Key("content")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Padding(0, 10).Gap(10).AlignItems(ui.Center).Children(func() {
			lead(selected)
			t := ui.Text(c, label).FontSize(14).SingleLine().Grow(1).Shrink(1).MinWidth(0)
			t.TextColor(t.AnimateColor("col", text, ui.Animation{Duration: 160 * time.Millisecond}))
			if value != "" {
				v := ui.Text(c, value).FontSize(12).FontFeatures("tnum").SingleLine().Shrink(0)
				v.TextColor(v.AnimateColor("col", muted, ui.Animation{Duration: 160 * time.Millisecond}))
			}
		})
	})
}

// header is the page's title, and its controls at the right.
func (a *app) header(c *ui.Context, p palette) {
	bar := c.TitleBar()
	title, sub := "", ""
	cur := a.cur()
	switch a.page {
	case pageOverview:
		title = locales.PageOverview()
		if a.overviewAll {
			signed := 0
			for _, q := range a.profiles {
				if q.SignedIn {
					signed++
				}
			}
			sub = locales.ProfilesCount(len(a.profiles)) + " · " + locales.SignedInOf(signed, len(a.profiles))
		} else if cur != nil {
			sub = a.profileTitle(cur)
		}
	case pageProfile:
		if cur != nil {
			title, sub = cur.Name, a.shortPath(cur.Dir)
		}
	case pageInstall:
		title, sub = locales.PageInstall(), locales.InstallSubtitle()
	case pageCleanup:
		title = locales.PageCleanup()
		if cur != nil {
			sub = a.shortPath(cur.Dir)
		}
	case pageSettings:
		title, sub = locales.MenuSettings(), "CCGuanjia "+appVersion()
	case pageClaudeSettings:
		title = locales.PageClaudeSettings()
		if cur != nil {
			sub = a.shortPath(filepath.Join(cur.Dir, settings.Name))
		}
	}
	ui.Row(c).Height(max(bar.Height, 52)+20).Padding(0, 24, 0, 28).Gap(12).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(21).FontWeight(700).LetterSpacing(-0.3).TextColor(p.text).SingleLine().ContentTransition(ui.ContentTransition{})
			if sub != "" {
				ui.Text(c, sub).FontSize(12).TextColor(p.muted).SingleLine().ContentTransition(ui.ContentTransition{})
			}
		})
		switch {
		case a.page == pageOverview && len(a.profiles) > 1:
			// All profiles comes first; no profile can take its name, as
			// those are lowercase.
			names := append([]string{locales.AllProfiles()}, a.profileNames()...)
			name := a.current
			if a.overviewAll {
				name = names[0]
			}
			if ui.Select(c.Key("profile-select"), &name, names).Label(locales.Profile()).Width(170).Changed() {
				if name == names[0] {
					a.showAll()
				} else {
					a.show(a.page, name)
				}
			}
		case (a.page == pageCleanup || a.page == pageClaudeSettings) && len(a.profiles) > 1:
			name := a.current
			if ui.Select(c.Key("profile-select"), &name, a.profileNames()).Label(locales.Profile()).Width(170).Changed() {
				a.show(a.page, name)
			}
		}
		if a.page == pageOverview {
			i := a.rangeIdx
			if segmented(c.Key("range"), p, &i, locales.Range24h(), locales.Range7d(), locales.Range30d()) {
				a.setRange(i)
			}
		}
		iconButton(c.Key("palette"), p, iconCommand, locales.CommandPalette()+"  ⌘K").OnClick(a.openPalette)
		spinButton(c.Key("refresh"), p, iconRefresh, locales.Refresh(), a.busy()).OnClick(a.refresh)
	})
}

// profileTitle names a profile with its account.
func (a *app) profileTitle(pr *claude.Profile) string {
	if pr.Account != nil && pr.Account.Email != "" {
		return pr.Name + " · " + pr.Account.Email
	}
	return pr.Name
}

func kindName(kind string) string {
	switch kind {
	case "native":
		return locales.KindNative()
	case "npm":
		return "npm"
	case "homebrew":
		return "Homebrew"
	case "bun":
		return "Bun"
	}
	return locales.KindOther()
}
