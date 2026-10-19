package main

import (
	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/clean"
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
			ui.Scroll(c.Key(a.page+"/"+a.current)).Grow(1).Padding(4, 24, 28).Gap(20).Children(func() {
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
				}
			})
		})
	})
	a.newProfileDialog(c, p)
}

// sidebar is the pages, the profiles, and the version of Claude Code.
func (a *app) sidebar(c *ui.Context, p palette) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 52)).DragWindow()
	ui.Scroll(c).Grow(1).Padding(0, 12).Gap(2).Children(func() {
		a.navItem(c, p, "nav-overview", iconOverview, p.gray, locales.PageOverview(), "", a.page == pageOverview, func() { a.show(pageOverview, "") })

		a.navSection(c, p, locales.SectionProfiles())
		for _, pr := range a.profiles {
			value := "—"
			if pr.Account != nil && pr.Account.Plan != "" {
				value = pr.Account.Plan
			} else if !pr.SignedIn {
				value = locales.SignedOutShort()
			}
			color := p.blue
			if pr.Default {
				color = p.purple
			}
			label := pr.Name
			if a.isDefault(pr.Name) {
				label += " •"
			}
			a.navItem(c, p, "nav-profile-"+pr.Name, iconUser, color, label, value,
				a.page == pageProfile && a.current == pr.Name, func() { a.show(pageProfile, pr.Name) })
		}
		a.navItem(c, p, "nav-new", iconPlus, p.gray, locales.NewProfile(), "", false, func() {
			a.resetNew()
			a.newOpen = true
		})

		a.navSection(c, p, locales.SectionMaintenance())
		version := ""
		if a.inst != nil && a.inst.Active() != nil {
			version = a.inst.Active().Version
		}
		a.navItem(c, p, "nav-install", iconPackage, p.red, locales.PageInstall(), version, a.page == pageInstall, func() { a.show(pageInstall, "") })
		reclaim := ""
		if pl := a.plans[a.current]; pl != nil {
			reclaim = clean.Size(pl.Reclaim())
		}
		a.navItem(c, p, "nav-cleanup", iconSparkles, p.amber, locales.PageCleanup(), reclaim, a.page == pageCleanup, func() { a.show(pageCleanup, "") })
	})
	ui.Row(c).Padding(12, 16, 16, 24).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, "Claude Code").FontSize(13).FontWeight(600).TextColor(p.text).SingleLine()
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

func (a *app) navSection(c *ui.Context, p palette, title string) {
	ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(p.faint).Padding(16, 10, 6)
}

// navItem is a row of the sidebar: an icon in a square of its color, a
// label and a value at its end.
func (a *app) navItem(c *ui.Context, p palette, key string, icon *ui.SVG, color ui.Color, label, value string, selected bool, onClick func()) {
	r := ui.Row(c.Key(key)).Height(34).Padding(0, 10).Gap(10).Radius(8).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label(label)
	text, muted := p.text, p.muted
	switch {
	case selected:
		r.Background(p.selected)
		text, muted = p.selText, p.selText.Alpha(0.8)
	case r.Hovered():
		r.Background(p.text.Alpha(0.06))
	}
	r.OnClick(onClick).Children(func() {
		sq := ui.Box(c).Size(22, 22).Radius(6).Background(color).Center().Shrink(0)
		if selected {
			sq.Background(p.selText.Alpha(0.25))
		}
		sq.Children(func() { ui.Icon(c, icon).Size(13, 13).TextColor(ui.Hex("#ffffff")) })
		ui.Text(c, label).FontSize(14).TextColor(text).SingleLine().Grow(1)
		if value != "" {
			ui.Text(c, value).FontSize(12).TextColor(muted).FontFeatures("tnum").SingleLine()
		}
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
		if cur != nil {
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
	}
	ui.Row(c).Height(max(bar.Height, 52)+20).Padding(0, 24, 0, 28).Gap(12).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(20).FontWeight(700).TextColor(p.text).SingleLine()
			if sub != "" {
				ui.Text(c, sub).FontSize(12).TextColor(p.muted).SingleLine()
			}
		})
		if (a.page == pageOverview || a.page == pageCleanup) && len(a.profiles) > 1 {
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
		iconButton(c.Key("refresh"), p, iconRefresh, locales.Refresh()).OnClick(a.refresh)
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
