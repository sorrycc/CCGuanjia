package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/shell"
	"ccguanjia/locales"
)

// profilePage is a profile: its account, how to start it, the items it
// shares with ~/.claude, its MCP servers, and removing it.
func (a *app) profilePage(c *ui.Context, p palette) {
	pr := a.cur()
	if pr == nil {
		return
	}
	if a.actionErr != "" {
		banner(c, p, iconAlert, p.red, a.actionErr, func() {
			ui.Button(c, locales.Dismiss()).OnClick(func() { a.actionErr = "" })
		})
	}
	ui.Grid(c).ColumnTracks(ui.Fr(1), ui.Fr(1)).Gap(20).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Gap(20).MinWidth(0).Children(func() {
			a.accountCard(c, p, pr)
			a.launchCard(c, p, pr)
		})
		ui.Column(c).Gap(20).MinWidth(0).Children(func() {
			a.sharedCard(c, p, pr)
			a.mcpCard(c, p, pr)
			if !pr.Default {
				a.removeCard(c, p, pr)
			}
		})
	})
}

func (a *app) accountCard(c *ui.Context, p palette, pr *claude.Profile) {
	card(c, p, iconUser, p.blue, locales.CardAccount(), "", nil, func() {
		acc := pr.Account
		ui.Box(c).Children(func() {
			plan := "—"
			if acc != nil && acc.Plan != "" {
				plan = acc.Plan
			}
			bigValue(c, p, locales.Plan(), plan, "")
			if pr.SignedIn {
				cornerPill(c, p, iconCheck, p.green, locales.SignedIn())
			} else {
				cornerPill(c, p, iconAlert, p.red, locales.SignedOut())
			}
		})
		ui.Column(c).Children(func() {
			field := func(label, value string) {
				ui.Row(c).Gap(12).Padding(7, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.border).Children(func() {
					ui.Text(c, label).FontSize(13).TextColor(p.muted).Width(110).Shrink(0)
					ui.Text(c, value).FontSize(13).TextColor(p.text).Selectable().Grow(1).MinWidth(0)
				})
			}
			email, name, org := "—", "—", "—"
			if acc != nil {
				email, name, org = orDash(acc.Email), orDash(acc.Name), orDash(acc.Org)
			}
			field(locales.Email(), email)
			field(locales.Name(), name)
			field(locales.Organization(), org)
			field(locales.Folder(), a.shortPath(pr.Dir))
			field(locales.ConfigFile(), a.shortPath(pr.ConfigFile))
			field(locales.KeychainItem(), pr.KeychainService)
		})
		ui.Text(c, locales.LocalOnlyNote()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
	})
}

func (a *app) launchCard(c *ui.Context, p palette, pr *claude.Profile) {
	card(c, p, iconTerminal, p.green, locales.CardLaunch(), "", nil, func() {
		cmd := "claude"
		if !pr.Default {
			cmd = shell.FunctionName(pr.Name)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Padding(9, 12).Radius(10).Background(p.well).Children(func() {
				ui.Text(c, cmd).Font("monospace").FontSize(14).TextColor(p.text).Selectable().SingleLine()
			})
			ui.Button(c.Key("copy-cmd"), locales.Copy()).OnClick(func() {
				c.WriteClipboard(cmd)
				a.toast(locales.Copied())
			})
			ui.PrimaryButton(c.Key("open-terminal"), locales.OpenInTerminal()).OnClick(func() {
				if err := a.openTerminal(pr.Name); err != nil {
					a.actionErr = errText(err)
				}
			})
		})
		if !pr.Default && !a.shellInstalled {
			ui.Text(c, locales.ShellNeeded(cmd)).FontSize(12).TextColor(p.muted).LineHeight(1.35)
		}
		if !pr.SignedIn {
			ui.Text(c, locales.SignInHint()).FontSize(12).TextColor(p.orange).LineHeight(1.35)
		}
		on := a.isDefault(pr.Name)
		if switchRow(c.Key("default"), p, locales.DefaultForShells(), locales.DefaultForShellsNote(), &on, false) {
			if on {
				a.setDefault(pr.Name)
			} else {
				a.setDefault(claude.DefaultName)
			}
		}
		if !pr.Default {
			o := a.cfg.Options(pr.Name)
			skip := o.SkipPermissions
			if switchRow(c.Key("skip"), p, locales.SkipPermissions(), locales.SkipPermissionsNote(), &skip, false) {
				o.SkipPermissions = skip
				a.saveConfig()
			}
		}
		if ls := a.launchersOf(pr.Name); len(ls) > 0 {
			ui.Text(c, locales.AlsoStartedBy(strings.Join(ls, ", "))).FontSize(12).TextColor(p.muted).LineHeight(1.35)
		}
	})
}

func (a *app) sharedCard(c *ui.Context, p palette, pr *claude.Profile) {
	linked, total := a.linkCount(pr.Name)
	extra := ""
	if !pr.Default {
		extra = locales.SharedCount(linked, total)
	}
	card(c, p, iconLink, p.purple, locales.CardShared(), extra, nil, func() {
		if pr.Default {
			ui.Text(c, locales.MainFolderShared()).FontSize(13).TextColor(p.muted).LineHeight(1.4)
			return
		}
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			for _, l := range pr.Links {
				note := ""
				on := l.State == claude.Linked
				disabled := false
				switch {
				case l.State == claude.Own:
					note, disabled = locales.LinkOwn(), true
				case l.State == claude.LinkedElsewhere:
					note = locales.LinkElsewhere(l.Target)
				case !l.Source:
					note, disabled = locales.LinkNoSource(), true
				case on:
					note = locales.LinkLinked()
				default:
					note = locales.LinkUnlinked()
				}
				if switchRow(c.Key("link-"+l.Item), p, l.Item, note, &on, disabled) {
					a.setLink(pr, l.Item, on)
				}
			}
		})
	})
}

func (a *app) mcpCard(c *ui.Context, p palette, pr *claude.Profile) {
	servers := pr.MCPServers()
	card(c, p, iconServer, p.teal, locales.CardMCP(), locales.ServersCount(len(servers)), nil, func() {
		if len(servers) == 0 {
			ui.Text(c, locales.NoServers()).FontSize(13).TextColor(p.muted)
		} else {
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, s := range servers {
					ui.Box(c.Key("srv-"+s)).Padding(4, 10).Radius(999).Background(p.well).Children(func() {
						ui.Text(c, s).FontSize(12).TextColor(p.text)
					})
				}
			})
		}
		var others []string
		for _, q := range a.profiles {
			if q.Name != pr.Name && len(q.MCPServers()) > 0 {
				others = append(others, q.Name)
			}
		}
		if len(others) == 0 {
			return
		}
		if !slices.Contains(others, a.mcpFrom) {
			a.mcpFrom = others[0]
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.CopyServersFrom()).FontSize(13).TextColor(p.muted)
			ui.Select(c.Key("mcp-from"), &a.mcpFrom, others).Label(locales.CopyServersFrom()).Width(160)
			ui.Button(c.Key("mcp-copy"), locales.CopyServers()).OnClick(func() { a.copyMCP(pr) })
		})
		ui.Text(c, locales.CopyServersNote()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
	})
}

func (a *app) removeCard(c *ui.Context, p palette, pr *claude.Profile) {
	card(c, p, iconTrash, p.red, locales.CardRemove(), "", nil, func() {
		ui.Text(c, locales.RemoveNote(a.shortPath(pr.Dir))).FontSize(13).TextColor(p.muted).LineHeight(1.4)
		ui.Row(c).Children(func() {
			ui.Button(c.Key("remove"), locales.RemoveProfileButton()).TextColor(p.red).OnClick(func() { a.removeQ = true })
		})
	})
	switch ui.AlertDialog(c, &a.removeQ, locales.RemoveQuestion(pr.Name), locales.RemoveDetail(a.shortPath(pr.Dir)), locales.RemoveProfile(), locales.Cancel()) {
	case 0:
		a.removeProfile(pr)
	}
}

// newProfileDialog makes a profile: its name, the items it links to
// ~/.claude, and whether its launcher skips permissions.
func (a *app) newProfileDialog(c *ui.Context, p palette) {
	ui.Modal(c, &a.newOpen, func() {
		ui.Column(c).Width(440).Gap(14).Children(func() {
			ui.Text(c, locales.NewProfile()).FontSize(17).FontWeight(700)
			ui.Text(c, locales.NewProfileNote()).FontSize(13).TextColor(p.muted).LineHeight(1.4)
			submitted := ui.TextInput(c.Key("new-name"), &a.newName).Placeholder("work").Label(locales.ProfileName()).AutoFocus().Submitted()
			name := strings.TrimSpace(a.newName)
			if name != "" {
				mono(c, p, "~/.claude-"+name+"   ·   "+shell.FunctionName(name))
			}
			if a.newErr != "" {
				ui.Text(c, a.newErr).FontSize(12).TextColor(p.red)
			}
			ui.Text(c, locales.LinkToMain()).FontSize(13).FontWeight(600).Padding(4, 0, 0)
			ui.Grid(c).Columns(2).GapY(6).GapX(12).Children(func() {
				for _, item := range claude.SharedItems {
					v := a.newItems[item]
					if ui.Checkbox(c.Key("new-"+item), &v, item).Changed() {
						a.newItems[item] = v
					}
				}
			})
			ui.Checkbox(c.Key("new-skip"), &a.newSkip, locales.SkipPermissions())
			ui.Row(c).Gap(8).Justify(ui.End).Padding(6, 0, 0).Children(func() {
				ui.Button(c.Key("new-cancel"), locales.Cancel()).OnClick(func() { a.newOpen = false })
				create := ui.PrimaryButton(c.Key("new-create"), locales.Create()).Disabled(name == "")
				if create.Clicked() || submitted && name != "" {
					if a.createProfile() {
						a.newOpen = false
					}
				}
			})
		})
	})
}
