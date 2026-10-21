package main

import (
	"cmp"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/clean"
	"ccguanjia/internal/install"
	"ccguanjia/internal/shell"
	"ccguanjia/internal/usage"
	"ccguanjia/locales"
)

// overview is the dashboard of every profile, or of the current one: the
// accounts, tokens and sessions, what Claude Code is, what the folders
// take, and the profiles.
func (a *app) overview(c *ui.Context, p palette) {
	pr := a.cur()
	if pr == nil {
		ui.Spinner(c)
		return
	}
	sum := a.summary
	if sum != nil && a.summaryKey != a.usageKey() {
		sum = nil
	}
	st := a.overviewStorage()
	signed := 0
	for _, q := range a.profiles {
		if q.SignedIn {
			signed++
		}
	}

	// Three columns, or two in a narrow window, where three would cut the
	// titles of the cards short.
	cols := 3
	if a.contentWidth(c) < 900 {
		cols = 2
	}
	ui.Grid(c).Columns(cols).Gap(20).Children(func() {
		// Accounts.
		if a.overviewAll {
			card(c, p, iconUser, p.blue, locales.CardAccounts(), locales.SignedInOf(signed, len(a.profiles)), nil, func() {
				a.accountList(c, p)
			})
		} else {
			a.overviewAccount(c, p, pr)
		}

		// Tokens.
		card(c, p, iconActivity, p.green, locales.CardTokens(), "", nil, func() {
			if sum == nil {
				loadingValue(c, p, a.rangeLabel())
				return
			}
			ui.Box(c).Children(func() {
				n, unit := splitNumber(tokens(sum.Tokens.Total()))
				bigValue(c, p, a.rangeLabel(), n, unit)
				deltaPill(c, p, float64(sum.Tokens.Total()), float64(sum.Prev.Tokens.Total()), a.prevLabel())
			})
			stats(c, p,
				stat{locales.Input(), tokens(sum.Tokens.Input)},
				stat{locales.Output(), tokens(sum.Tokens.Output)},
				stat{locales.Cached(), tokens(sum.Tokens.CacheRead + sum.Tokens.CacheWrite)})
			columns(c, p, p.green, floats(sum.Buckets), func(i int) string {
				return tokens(sum.Buckets[i]) + " · " + a.bucketTime(sum, i)
			})
		})

		// Install.
		card(c, p, iconPackage, p.red, locales.CardInstall(), "", func() { a.show(pageInstall, "") }, func() {
			in := (*install.Install)(nil)
			if a.inst != nil {
				in = a.inst.Active()
			}
			ui.Box(c).Children(func() {
				v := "…"
				if a.inst != nil {
					v = "—"
					if in != nil && in.Version != "" {
						v = in.Version
					}
				}
				bigValue(c, p, locales.Installed(), v, "")
				switch {
				case a.inst == nil:
				case in == nil:
					cornerPill(c, p, iconAlert, p.red, locales.NotInstalled())
				case a.latest != "" && in.Version != "" && install.Compare(in.Version, a.latest) < 0:
					cornerPill(c, p, iconDownload, p.orange, locales.UpdateAvailable())
				case a.latest != "":
					cornerPill(c, p, iconCheck, p.green, locales.UpToDate())
				}
			})
			method, latest, count := "—", "—", "—"
			if a.inst != nil {
				if in != nil {
					method = kindName(string(in.Kind))
				}
				count = fmt.Sprint(len(a.inst.Installs))
			}
			if a.latest != "" {
				latest = a.latest
			}
			stats(c, p, stat{locales.Method(), method}, stat{locales.Latest(), latest}, stat{locales.Installs(), count})
			if a.inst != nil {
				var old int64
				for _, v := range a.inst.OldVersions() {
					old += v.Bytes
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 0, 0).Children(func() {
					ui.Icon(c, iconArchive).Size(14, 14).TextColor(p.muted)
					ui.Text(c, locales.VersionsKept(len(a.inst.Versions), clean.Size(old))).FontSize(13).TextColor(p.muted).LineHeight(1.35).Grow(1).Shrink(1).MinWidth(0)
				})
			}
		})

		// Storage.
		where := a.shortPath(pr.Dir)
		if a.overviewAll {
			where = locales.AllProfiles()
		}
		card(c, p, iconDisk, p.orange, locales.CardStorage(), "", func() { a.show(pageCleanup, "") }, func() {
			if st == nil {
				loadingValue(c, p, where)
				return
			}
			n, unit := splitNumber(clean.Size(st.Total))
			bigValue(c, p, where, n, unit)
			reclaim := "—"
			if r, ok := a.reclaimable(pr); ok {
				reclaim = clean.Size(r)
			}
			stats(c, p, stat{"projects", clean.Size(entryBytes(st.ByType, "projects"))},
				stat{"file-history", clean.Size(entryBytes(st.ByType, "file-history"))},
				stat{locales.Reclaimable(), reclaim})
			names, shares, _ := topEntries(st.ByType, 5)
			ui.Column(c).Gap(12).Padding(4, 0, 0).Children(func() {
				shareBar(c, p, a.typeColors(p), shares)
				ui.Row(c).Gap(14).Wrap().Children(func() {
					for i, name := range names {
						if i == 3 {
							break
						}
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Size(8, 8).Radius(4).Background(a.typeColors(p)[i%6])
							ui.Text(c, name).FontSize(12).TextColor(p.muted).SingleLine()
						})
					}
				})
			})
		})

		// Sessions.
		card(c, p, iconMessages, p.teal, locales.CardSessions(), "", nil, func() {
			if sum == nil {
				loadingValue(c, p, a.rangeLabel())
				return
			}
			ui.Box(c).Children(func() {
				bigValue(c, p, a.rangeLabel(), fmt.Sprint(sum.Sessions), "")
				deltaPill(c, p, float64(sum.Sessions), float64(sum.Prev.Sessions), a.prevLabel())
			})
			stats(c, p,
				stat{locales.Replies(), fmt.Sprint(sum.Replies)},
				stat{locales.Projects(), fmt.Sprint(len(sum.Projects))},
				stat{locales.Models(), fmt.Sprint(len(sum.Models))})
			columns(c, p, p.teal, floats(sum.Counts), func(i int) string {
				return locales.RepliesCount(int(sum.Counts[i])) + " · " + a.bucketTime(sum, i)
			})
		})

		// Profiles, and the tokens of each over the range.
		card(c, p, iconUsers, p.purple, locales.CardProfiles(), locales.SignedInOf(signed, len(a.profiles)), nil, func() {
			ui.Box(c).Children(func() {
				bigValue(c, p, locales.OnThisMac(), fmt.Sprint(len(a.profiles)), "")
				if a.shellInstalled {
					cornerPill(c, p, iconTerminal, p.green, locales.ShellOn())
				} else {
					cornerPill(c, p, iconTerminal, p.gray, locales.ShellOff())
				}
			})
			a.profileUsage(c, p)
		})

		// Storage by type.
		card(c, p, iconPie, p.orange, locales.StorageByType(), "", nil, func() {
			if st == nil {
				loadingList(c, p, 4)
				return
			}
			names, shares, values := topEntries(st.ByType, 5)
			ui.Row(c).Gap(16).AlignItems(ui.Center).Padding(6, 0, 0).Children(func() {
				n, unit := splitNumber(clean.Size(st.Total))
				donut(c, p, a.typeColors(p), shares, n+" "+unit, locales.InAll())
				legend(c, p, a.typeColors(p), names, values)
			})
		})

		// Storage by project.
		extra := ""
		if st != nil {
			extra = locales.ProjectsCount(len(st.ByProject))
		}
		card(c, p, iconGrid, p.teal, locales.StorageByProject(), extra, nil, func() {
			if st == nil {
				loadingList(c, p, 4)
				return
			}
			var tiles []tile
			for i, e := range st.ByProject {
				if i == 7 {
					break
				}
				name := e.Name
				if e.Path != "" {
					name = filepath.Base(e.Path)
				}
				tiles = append(tiles, tile{name: name, value: clean.Size(e.Bytes), weight: float64(max(e.Bytes, 1))})
			}
			if len(tiles) == 0 {
				ui.Text(c, locales.NoProjects()).FontSize(13).TextColor(p.muted)
				return
			}
			treemap(c, p, []ui.Color{p.teal, p.blue, p.purple, p.green, p.orange, p.amber, p.gray}, tiles, 180)
		})

		// Tokens by model.
		extra = ""
		if sum != nil {
			extra = locales.TokensInAll(tokens(sum.Tokens.Total()))
		}
		card(c, p, iconCpu, p.amber, locales.TokensByModel(), extra, nil, func() {
			if sum == nil {
				loadingList(c, p, 4)
				return
			}
			if len(sum.Models) == 0 {
				ui.Text(c, locales.NoReplies()).FontSize(13).TextColor(p.muted)
				return
			}
			ui.Column(c).Gap(14).Children(func() {
				top := float64(sum.Models[0].Tokens)
				for i, m := range sum.Models {
					if i == 4 {
						break
					}
					bar(c.Key(m.Name), p, p.amber, m.Name, tokens(m.Tokens), float64(m.Tokens)/max(top, 1))
				}
			})
		})

		// Activity by hour.
		extra = ""
		if sum != nil {
			if d, h, ok := busiest(sum); ok {
				extra = locales.Busiest(weekday(d) + fmt.Sprintf(" %02d:00", h))
			}
		}
		card(c, p, iconCalendar, p.blue, locales.CardActivity(), extra, nil, func() {
			if sum == nil {
				loadingList(c, p, 3)
				return
			}
			heatmap(c, p, p.blue, sum)
		}).ColumnSpan(min(2, cols))

		// Tokens by project.
		card(c, p, iconFolder, p.purple, locales.TokensByProject(), "", nil, func() {
			if sum == nil {
				loadingList(c, p, 4)
				return
			}
			if len(sum.Projects) == 0 {
				ui.Text(c, locales.NoReplies()).FontSize(13).TextColor(p.muted)
				return
			}
			ui.Column(c).Gap(14).Children(func() {
				top := float64(sum.Projects[0].Tokens)
				for i, pj := range sum.Projects {
					if i == 5 {
						break
					}
					name := filepath.Base(pj.Name)
					if pj.Name == "" {
						name = "—"
					}
					bar(c.Key(pj.Name), p, p.purple, name, tokens(pj.Tokens), float64(pj.Tokens)/max(top, 1))
				}
			})
		})

		// Recent sessions.
		card(c, p, iconHistory, p.green, locales.CardRecent(), "", nil, func() {
			if sum == nil {
				loadingList(c, p, 4)
				return
			}
			a.recentSessions(c, p, sum)
		}).ColumnSpan(cols)
	})
}

// overviewAccount is the account of a profile: its plan, sign-in, name,
// organization and launcher, and how much it shares with ~/.claude.
func (a *app) overviewAccount(c *ui.Context, p palette, pr *claude.Profile) {
	card(c, p, iconUser, p.blue, locales.CardAccount(), "", func() { a.show(pageProfile, pr.Name) }, func() {
		ui.Box(c).Children(func() {
			plan, label := "—", locales.Plan()
			if pr.Account != nil && pr.Account.Plan != "" {
				plan = pr.Account.Plan
			}
			bigValue(c, p, label, plan, "")
			if pr.SignedIn {
				cornerPill(c, p, iconCheck, p.green, locales.SignedIn())
			} else {
				cornerPill(c, p, iconAlert, p.red, locales.SignedOut())
			}
		})
		org, name := "—", "—"
		if acc := pr.Account; acc != nil {
			if acc.Org != "" {
				org = acc.Org
			}
			if acc.Name != "" {
				name = acc.Name
			}
		}
		launcher := "claude"
		if !pr.Default {
			launcher = shell.FunctionName(pr.Name)
		}
		stats(c, p, stat{locales.Name(), name}, stat{locales.Organization(), org}, stat{locales.Launcher(), launcher})
		linked, total := a.linkCount(pr.Name)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 0, 0).Children(func() {
			ui.Icon(c, iconLink).Size(14, 14).TextColor(p.muted)
			if pr.Default {
				ui.Text(c, locales.MainFolderNote()).FontSize(13).TextColor(p.muted).SingleLine()
			} else {
				ui.Text(c, locales.SharedCount(linked, total)).FontSize(13).TextColor(p.muted).SingleLine()
			}
		})
	})
}

// accountList is a row for each profile's account: its email, plan and
// sign-in. A click shows the profile.
func (a *app) accountList(c *ui.Context, p palette) {
	const most = 4
	ui.Column(c).Gap(4).Children(func() {
		for i, pr := range a.profiles {
			if i == most {
				break
			}
			r := ui.Row(c.Key("account-"+pr.Name)).Gap(10).AlignItems(ui.Center).Padding(5, 6).Margin(0, -6).Radius(8).
				Cursor(ui.CursorPointer).Label(pr.Name).Transition(quick)
			if r.Hovered() {
				r.Background(p.hover())
			}
			r.OnClick(func() { a.show(pageProfile, pr.Name) }).Children(func() {
				avatar(c, p, pr.Name, a.profileColor(p, pr.Name), 26, a.live(pr.Name), p.card)
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					email, plan := "—", "—"
					if acc := pr.Account; acc != nil {
						if acc.Email != "" {
							email = acc.Email
						}
						if acc.Plan != "" {
							plan = acc.Plan
						}
					}
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, pr.Name).FontSize(13).FontWeight(600).TextColor(p.text).SingleLine().Grow(1).Shrink(1).MinWidth(0)
						ui.Text(c, plan).FontSize(12).TextColor(p.muted).SingleLine().Shrink(0)
					})
					ui.Text(c, email).FontSize(12).TextColor(p.muted).SingleLine()
				})
				if pr.SignedIn {
					ui.Icon(c, iconCheck).Size(14, 14).TextColor(p.green).Tooltip(locales.SignedIn())
				} else {
					ui.Icon(c, iconAlert).Size(14, 14).TextColor(p.red).Tooltip(locales.SignedOut())
				}
			})
		}
		if n := len(a.profiles) - most; n > 0 {
			ui.Text(c, locales.MoreProfiles(n)).FontSize(12).TextColor(p.faint).Padding(2, 0, 0)
		}
	})
}

// loadingValue stands in for a large number and its stats while they are
// read.
func loadingValue(c *ui.Context, p palette, label string) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(p.muted).SingleLine()
		skeleton(c, p, 120, 38)
	})
	ui.Grid(c).Columns(3).GapX(12).Children(func() {
		for range 3 {
			ui.Column(c).Gap(6).Children(func() {
				skeleton(c, p, 48, 10)
				skeleton(c, p, 64, 14)
			})
		}
	})
	skeleton(c, p, 0, chartHeight)
}

// loadingList stands in for n rows while they are read.
func loadingList(c *ui.Context, p palette, n int) {
	ui.Column(c).Gap(12).Children(func() {
		for i := range n {
			skeleton(c, p, float32(220-i*30), 12)
		}
	})
}

// bucketTime says when a bucket of the summary starts: its hour for the
// last 24 hours, its day and hour for a week, its date for a month.
func (a *app) bucketTime(sum *usage.Summary, i int) string {
	span := sum.Until.Sub(sum.Since) / time.Duration(max(len(sum.Buckets), 1))
	at := sum.Since.Add(span * time.Duration(i)).Local()
	switch a.rangeIdx {
	case 0:
		return at.Format("15:04")
	case 1:
		return weekday(at.Weekday()) + at.Format(" 15:04")
	}
	return at.Format("01-02")
}

// weekday is the short name of a day of the week.
func weekday(d time.Weekday) string {
	names := strings.Fields(locales.Weekdays())
	if int(d) < len(names) {
		return names[d]
	}
	return d.String()[:3]
}

// busiest is the hour of the week with the most replies.
func busiest(sum *usage.Summary) (time.Weekday, int, bool) {
	var best int64
	day, hour := time.Sunday, 0
	for d, hours := range sum.Hours {
		for h, n := range hours {
			if n > best {
				best, day, hour = n, time.Weekday(d), h
			}
		}
	}
	return day, hour, best > 0
}

// heatmap is the replies of each hour of the week, Monday first, as cells
// of a color as deep as the hour was busy. The cell under the pointer says
// how many.
func heatmap(c *ui.Context, p palette, color ui.Color, sum *usage.Summary) {
	var peak int64
	for _, hours := range sum.Hours {
		for _, n := range hours {
			peak = max(peak, n)
		}
	}
	const labelW, gap = float32(40), float32(3)
	const rows = 7
	// The cells take the height the card has beside its neighbor.
	box := ui.Box(c).MinHeight(7*16 + 6*gap + 20).Grow(1)
	g := easeOutCubic(grow(box))
	px, py, over := box.PointerPosition()
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		cw := (r.W - labelW - 23*gap) / 24
		ch := (r.H - 20 - 6*gap) / rows
		hotD, hotH := -1, -1
		for row := range rows {
			d := (row + 1) % 7 // Monday first
			y := r.Y + float32(row)*(ch+gap)
			if row%2 == 0 {
				pt.RichText(r.X, y+(ch-13)/2, labelW, ui.Span{Text: weekday(time.Weekday(d)), Size: 11, Color: p.muted})
			}
			for h := range 24 {
				x := r.X + labelW + float32(h)*(cw+gap)
				cell := ui.Rect{X: x, Y: y, W: cw, H: ch}
				n := sum.Hours[d][h]
				// An hour without replies is a faint cell of the card, not a
				// hole in it.
				empty := p.text.Alpha(0.05)
				col := empty
				if n > 0 && peak > 0 {
					level := 0.22 + 0.78*float32(math.Sqrt(float64(n)/float64(peak)))
					col = color.Alpha(level * stagger(g, h, 24))
					if g < 1 && level*stagger(g, h, 24) < 0.05 {
						col = empty
					}
				}
				pt.Fill(cell, col, 4)
				if over && r.X+px >= x && r.X+px < x+cw+gap && r.Y+py >= y && r.Y+py < y+ch+gap {
					hotD, hotH = d, h
					pt.Stroke(cell, p.text.Alpha(0.5), 4, 1)
				}
			}
		}
		for _, h := range []int{0, 6, 12, 18, 23} {
			x := r.X + labelW + float32(h)*(cw+gap)
			pt.RichText(x, r.Y+r.H-14, 0, ui.Span{Text: fmt.Sprintf("%02d", h), Size: 10, Color: p.faint})
		}
		if hotD >= 0 {
			x := r.X + labelW + float32(hotH)*(cw+gap) + cw/2
			row := (hotD + 6) % 7
			y := r.Y + float32(row)*(ch+gap) - 26
			if row == 0 {
				y = r.Y + ch + gap
			}
			label := weekday(time.Weekday(hotD)) + fmt.Sprintf(" %02d:00 · ", hotH) + locales.RepliesCount(int(sum.Hours[hotD][hotH]))
			tooltipAt(pt, p, label, x, y, r)
		}
	})
}

// recentSessions lists the sessions active last, each with its folder,
// profile when Overview shows them all, model, size and when, and a button
// resuming it in the terminal.
func (a *app) recentSessions(c *ui.Context, p palette, sum *usage.Summary) {
	if len(sum.Recent) == 0 {
		ui.Text(c, locales.NoSessions()).FontSize(13).TextColor(p.muted)
		return
	}
	now := a.now()
	shown := sum.Recent
	if !a.allSessions && len(shown) > recentShown {
		shown = shown[:recentShown]
	}
	ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
		for _, s := range shown {
			r := ui.Row(c.Key(s.Profile+"/"+s.ID)).Gap(14).AlignItems(ui.Center).Padding(9, 8).Radius(10).
				Transition(ui.ElementTransition{Colors: true, Position: true, Duration: 220 * time.Millisecond,
					Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}})
			hovered := r.Hovered()
			if hovered {
				r.Background(p.hover())
			}
			r.Children(func() {
				iconSquare(c, p, iconFolder, sessionColor(p, s.Project), 30)
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					name := filepath.Base(s.Project)
					if s.Project == "" {
						name = "—"
					}
					ui.Text(c, name).FontSize(14).FontWeight(600).TextColor(p.text).SingleLine()
					ui.Text(c, a.shortPath(s.Project)).FontSize(12).TextColor(p.muted).SingleLine()
				})
				if a.overviewAll {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Width(100).Shrink(0).Children(func() {
						avatar(c, p, s.Profile, a.profileColor(p, s.Profile), 18, false, p.card)
						ui.Text(c, s.Profile).FontSize(12).TextColor(p.muted).SingleLine().Shrink(1).MinWidth(0)
					})
				}
				ui.Text(c, strings.TrimPrefix(s.Model, "claude-")).FontSize(12).TextColor(p.muted).SingleLine().Width(110).Shrink(0)
				ui.Text(c, locales.RepliesCount(s.Replies)).FontSize(12).TextColor(p.muted).FontFeatures("tnum").SingleLine().Width(90).Shrink(0)
				ui.Text(c, tokens(s.Tokens)).FontSize(13).FontWeight(600).TextColor(p.text).FontFeatures("tnum").SingleLine().Width(60).Shrink(0)
				if now.Sub(s.Last) < liveWithin {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Width(90).Shrink(0).Tooltip(locales.LiveNote()).Children(func() {
						liveDot(ui.Box(c), c, p, 7, ui.Transparent)
						ui.Text(c, locales.Live()).FontSize(12).FontWeight(600).TextColor(p.green).SingleLine()
					})
				} else {
					ui.Text(c, ago(now, s.Last)).FontSize(12).TextColor(p.muted).SingleLine().Width(90).Shrink(0)
				}
				// Show in Finder and Copy ID recede until the row is under the
				// pointer, so that the list reads as its sessions; Resume
				// stays.
				ui.Row(c).Gap(6).Shrink(0).AlignItems(ui.Center).Children(func() {
					quiet := float32(0.35)
					if hovered || a.justCopied(c, "copy-"+s.ID) {
						quiet = 1
					}
					q := ui.Row(c.Key("quiet")).Gap(6).AlignItems(ui.Center)
					q.Opacity(q.AnimateTo("o", quiet, ui.Animation{Duration: 160 * time.Millisecond}))
					q.Children(func() {
						reveal := iconButton(c.Key("reveal"), p, iconReveal, locales.ShowInFinder())
						reveal.OnClick(func() {
							if fi, err := os.Stat(s.Project); err != nil || !fi.IsDir() {
								a.toast(locales.FolderGone())
							} else if !a.sync {
								mygo.Shell.ShowItemInFolder(s.Project)
							}
						})
						a.copyIconButton(c, p, "copy-"+s.ID, locales.CopyID(), s.ID)
					})
					ui.Button(c.Key("resume"), locales.Resume()).Tooltip(locales.ResumeNote()).OnClick(func() {
						if err := a.resumeSession(s.Profile, s); err != nil {
							a.toast(errText(err))
						}
					})
				})
			})
		}
	})
	if n := len(sum.Recent); n > recentShown {
		more := ui.Row(c.Key("more")).Gap(6).AlignItems(ui.Center).Justify(ui.Center).Padding(8).Radius(10).
			Cursor(ui.CursorPointer).Transition(quick)
		label := locales.ShowAllSessions(n)
		if a.allSessions {
			label = locales.ShowFewer()
		}
		more.Label(label)
		if more.Hovered() {
			more.Background(p.hover())
		}
		more.OnClick(func() { a.allSessions = !a.allSessions }).Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(p.muted)
			turn := float32(0)
			if a.allSessions {
				turn = 180
			}
			ui.Icon(c, iconDown).Size(14, 14).TextColor(p.muted).AnimatePose(ui.Pose{Rotate: turn}, ui.Snappy)
		})
	}
}

// recentShown is how many sessions Recent Sessions shows before Show All.
const recentShown = 6

// profileUsage is a row for each profile, the busiest first, with its
// tokens over the range as a bar; a click shows its overview.
func (a *app) profileUsage(c *ui.Context, p palette) {
	if a.byProfile == nil || a.byProfileIdx != a.rangeIdx {
		loadingList(c, p, 3)
		return
	}
	names := a.profileNames()
	slices.SortStableFunc(names, func(x, y string) int {
		return cmp.Compare(a.byProfile[y].Tokens.Total(), a.byProfile[x].Tokens.Total())
	})
	var top int64
	for _, n := range names {
		top = max(top, a.byProfile[n].Tokens.Total())
	}
	const most = 3
	ui.Column(c).Gap(4).Children(func() {
		for i, name := range names {
			if i == most {
				break
			}
			total := a.byProfile[name].Tokens.Total()
			r := ui.Row(c.Key("usage-"+name)).Gap(10).AlignItems(ui.Center).Padding(5, 6).Margin(0, -6).Radius(8).
				Cursor(ui.CursorPointer).Label(name).Transition(quick)
			if r.Hovered() {
				r.Background(p.hover())
			}
			if name == a.current && !a.overviewAll {
				r.Background(p.tint(p.selected).Alpha(0.5))
			}
			r.OnClick(func() { a.show(pageOverview, name) }).Children(func() {
				avatar(c, p, name, a.profileColor(p, name), 22, a.live(name), p.card)
				ui.Column(c).Gap(5).Grow(1).MinWidth(0).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, name).FontSize(13).FontWeight(500).TextColor(p.text).SingleLine().Grow(1).Shrink(1).MinWidth(0)
						ui.Text(c, tokens(total)).FontSize(12).TextColor(p.muted).FontFeatures("tnum").SingleLine().Shrink(0).ContentTransition(numeric)
					})
					frac := float32(0)
					if top > 0 {
						frac = float32(total) / float32(top)
					}
					color := a.profileColor(p, name)
					track := ui.Box(c).Height(4).Radius(2).Background(p.well)
					f := track.AnimateTo("f", frac, ui.Smooth) * easeOutCubic(grow(track))
					track.Draw(func(pt *ui.Painter, r ui.Rect) {
						if f > 0 {
							pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(r.H, r.W*f), H: r.H}, color, r.H/2)
						}
					})
				})
			})
		}
		if n := len(names) - most; n > 0 {
			ui.Text(c, locales.MoreProfiles(n)).FontSize(12).TextColor(p.faint).Padding(2, 0, 0)
		}
	})
}

// reclaimable is what Cleanup would free of the folders Overview shows,
// known once each has been looked at.
func (a *app) reclaimable(pr *claude.Profile) (int64, bool) {
	if !a.overviewAll {
		if pl := a.plans[pr.Name]; pl != nil {
			return pl.Reclaim(), true
		}
		return 0, false
	}
	var n int64
	for _, q := range a.profiles {
		pl := a.plans[q.Name]
		if pl == nil {
			return 0, false
		}
		n += pl.Reclaim()
	}
	return n, true
}

// prevLabel says what the change of a number is measured against.
func (a *app) prevLabel() string {
	switch a.rangeIdx {
	case 0:
		return locales.ThanPrevDay()
	case 2:
		return locales.ThanPrevMonth()
	}
	return locales.ThanPrevWeek()
}

// sessionColor is the color of a project's folder, the same each time.
func sessionColor(p palette, project string) ui.Color {
	h := uint32(2166136261)
	for i := 0; i < len(project); i++ {
		h = (h ^ uint32(project[i])) * 16777619
	}
	return []ui.Color{p.green, p.blue, p.purple, p.teal, p.orange, p.amber}[h%6]
}

// ago says how long ago a time was, as a list of recent things does.
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return locales.JustNow()
	case d < time.Hour:
		return locales.MinutesAgo(int(d / time.Minute))
	case d < 48*time.Hour:
		return locales.HoursAgo(int(d / time.Hour))
	}
	return locales.DaysAgo(int(d / (24 * time.Hour)))
}

func (a *app) rangeLabel() string {
	switch a.rangeIdx {
	case 0:
		return locales.Last24h()
	case 2:
		return locales.Last30d()
	}
	return locales.Last7d()
}

func (a *app) typeColors(p palette) []ui.Color {
	return []ui.Color{p.orange, p.blue, p.green, p.purple, p.teal, p.gray}
}

func (a *app) linkCount(name string) (linked, total int) {
	pr := a.profile(name)
	if pr == nil {
		return 0, 0
	}
	for _, l := range pr.Links {
		if l.Source {
			total++
			if l.State == claude.Linked {
				linked++
			}
		}
	}
	return linked, total
}

func floats(vs []int64) []float64 {
	out := make([]float64, len(vs))
	for i, v := range vs {
		out[i] = float64(v)
	}
	return out
}

func entryBytes(es []usage.Entry, name string) int64 {
	for _, e := range es {
		if e.Name == name {
			return e.Bytes
		}
	}
	return 0
}

// topEntries are the n largest entries, and the others as one, as the
// names, shares and sizes of a donut's parts.
func topEntries(es []usage.Entry, n int) (names []string, shares []float64, values []string) {
	var other int64
	for i, e := range es {
		if i < n {
			names = append(names, strings.TrimPrefix(e.Name, "."))
			shares = append(shares, float64(e.Bytes))
			values = append(values, clean.Size(e.Bytes))
		} else {
			other += e.Bytes
		}
	}
	if other > 0 {
		names = append(names, locales.Other())
		shares = append(shares, float64(other))
		values = append(values, clean.Size(other))
	}
	return names, shares, values
}
