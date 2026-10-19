package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/clean"
	"ccguanjia/internal/install"
	"ccguanjia/internal/shell"
	"ccguanjia/internal/usage"
	"ccguanjia/locales"
)

// overview is the dashboard of the current profile: its account, tokens
// and sessions, what Claude Code is, what the folder takes, and the
// profiles.
func (a *app) overview(c *ui.Context, p palette) {
	pr := a.cur()
	if pr == nil {
		ui.Spinner(c)
		return
	}
	sum := a.summary
	if sum != nil && a.summaryKey != fmt.Sprintf("%s/%d", pr.Name, a.rangeIdx) {
		sum = nil
	}
	st := a.storage[pr.Name]

	ui.Grid(c).Columns(3).Gap(20).Children(func() {
		// Account.
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

		// Tokens.
		card(c, p, iconActivity, p.green, locales.CardTokens(), "", nil, func() {
			if sum == nil {
				bigValue(c, p, a.rangeLabel(), "…", "")
				stats(c, p, stat{locales.Input(), "—"}, stat{locales.Output(), "—"}, stat{locales.Cached(), "—"})
				dots(c, p, p.green, nil)
				return
			}
			n, unit := splitNumber(tokens(sum.Tokens.Total()))
			bigValue(c, p, a.rangeLabel(), n, unit)
			stats(c, p,
				stat{locales.Input(), tokens(sum.Tokens.Input)},
				stat{locales.Output(), tokens(sum.Tokens.Output)},
				stat{locales.Cached(), tokens(sum.Tokens.CacheRead + sum.Tokens.CacheWrite)})
			dots(c, p, p.green, floats(sum.Buckets))
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
					ui.Text(c, locales.VersionsKept(len(a.inst.Versions), clean.Size(old))).FontSize(13).TextColor(p.muted).SingleLine()
				})
			}
		})

		// Storage.
		card(c, p, iconDisk, p.orange, locales.CardStorage(), "", func() { a.show(pageCleanup, "") }, func() {
			if st == nil {
				bigValue(c, p, a.shortPath(pr.Dir), "…", "")
				stats(c, p, stat{"projects", "—"}, stat{"file-history", "—"}, stat{locales.Reclaimable(), "—"})
				dots(c, p, p.orange, nil)
				return
			}
			n, unit := splitNumber(clean.Size(st.Total))
			bigValue(c, p, a.shortPath(pr.Dir), n, unit)
			reclaim := "—"
			if pl := a.plans[pr.Name]; pl != nil {
				reclaim = clean.Size(pl.Reclaim())
			}
			stats(c, p, stat{"projects", clean.Size(entryBytes(st.ByType, "projects"))},
				stat{"file-history", clean.Size(entryBytes(st.ByType, "file-history"))},
				stat{locales.Reclaimable(), reclaim})
			_, shares, _ := topEntries(st.ByType, 5)
			shareDots(c, p, a.typeColors(p), shares, 60)
		})

		// Sessions.
		card(c, p, iconMessages, p.teal, locales.CardSessions(), "", nil, func() {
			if sum == nil {
				bigValue(c, p, a.rangeLabel(), "…", "")
				stats(c, p, stat{locales.Replies(), "—"}, stat{locales.Projects(), "—"}, stat{locales.Models(), "—"})
				dots(c, p, p.teal, nil)
				return
			}
			bigValue(c, p, a.rangeLabel(), fmt.Sprint(sum.Sessions), "")
			stats(c, p,
				stat{locales.Replies(), fmt.Sprint(sum.Replies)},
				stat{locales.Projects(), fmt.Sprint(len(sum.Projects))},
				stat{locales.Models(), fmt.Sprint(len(sum.Models))})
			dots(c, p, p.teal, floats(sum.Counts))
		})

		// Profiles.
		card(c, p, iconUsers, p.purple, locales.CardProfiles(), "", nil, func() {
			signed := 0
			for _, q := range a.profiles {
				if q.SignedIn {
					signed++
				}
			}
			ui.Box(c).Children(func() {
				bigValue(c, p, locales.OnThisMac(), fmt.Sprint(len(a.profiles)), "")
				if a.shellInstalled {
					cornerPill(c, p, iconTerminal, p.green, locales.ShellOn())
				} else {
					cornerPill(c, p, iconTerminal, p.gray, locales.ShellOff())
				}
			})
			def := a.cfg.Default
			if def == "" {
				def = "default"
			}
			cli := locales.Off()
			if a.cliInstalled {
				cli = "ccgj"
			}
			stats(c, p, stat{locales.SignedIn(), fmt.Sprintf("%d / %d", signed, len(a.profiles))}, stat{locales.DefaultProfile(), def}, stat{locales.Command(), cli})
			ui.Row(c).Gap(8).Padding(4, 0, 0).Children(func() {
				ui.Button(c.Key("new-from-card"), locales.NewProfile()).OnClick(func() {
					a.resetNew()
					a.newOpen = true
				})
			})
		})

		// Storage by type.
		card(c, p, iconPie, p.orange, locales.StorageByType(), "", nil, func() {
			if st == nil {
				ui.Spinner(c)
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
				ui.Spinner(c)
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
			treemap(c, p, tiles, 172)
		})

		// Tokens by model.
		extra = ""
		if sum != nil {
			extra = locales.TokensInAll(tokens(sum.Tokens.Total()))
		}
		card(c, p, iconCpu, p.amber, locales.TokensByModel(), extra, nil, func() {
			if sum == nil {
				ui.Spinner(c)
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
					bar(c, p, p.amber, m.Name, tokens(m.Tokens), float64(m.Tokens)/max(top, 1))
				}
			})
		})
	})
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
