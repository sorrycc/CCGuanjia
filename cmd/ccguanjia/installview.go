package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/clean"
	"ccguanjia/internal/install"
	"ccguanjia/locales"
)

// installPage is how Claude Code is installed: the command that runs,
// updating, reinstalling it or installing a chosen version its own way,
// the other installs, and the native installer's older versions.
func (a *app) installPage(c *ui.Context, p palette) {
	if a.inst == nil {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c)
			ui.Text(c, locales.Detecting()).TextColor(p.muted)
		})
		return
	}
	in := a.inst.Active()
	if a.shadowed != "" {
		banner(c, p, iconAlert, p.orange, locales.Shadowed(a.shadowed), nil)
	}
	if len(a.inst.Kinds()) > 1 {
		banner(c, p, iconAlert, p.orange, locales.SeveralInstalls(len(a.inst.Installs)), nil)
	}
	ui.Grid(c).ColumnTracks(a.tracks(c, ui.Fr(1), ui.Fr(1))...).Gap(20).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Gap(20).MinWidth(0).Children(func() {
			card(c, p, iconPackage, p.red, "Claude Code", "", nil, func() {
				ui.Box(c).Children(func() {
					v := "—"
					if in != nil && in.Version != "" {
						v = in.Version
					}
					bigValue(c, p, locales.Installed(), v, "")
					switch {
					case in == nil:
						cornerPill(c, p, iconAlert, p.red, locales.NotInstalled())
					case a.latest != "" && in.Version != "" && install.Compare(in.Version, a.latest) < 0:
						cornerPill(c, p, iconDownload, p.orange, locales.UpdateTo(a.latest))
					case a.latest != "":
						cornerPill(c, p, iconCheck, p.green, locales.UpToDate())
					}
				})
				if in == nil {
					ui.Text(c, locales.InstallHint()).FontSize(13).TextColor(p.muted).LineHeight(1.4)
					mono(c, p, "curl -fsSL https://claude.ai/install.sh | bash")
					return
				}
				latest := a.latest
				if latest == "" {
					latest = "—"
				}
				stats(c, p, stat{locales.Method(), kindName(string(in.Kind))}, stat{locales.Latest(), latest}, stat{locales.Installs(), fmt.Sprint(len(a.inst.Installs))})
				mono(c, p, a.shortPath(in.Path)+" → "+a.shortPath(in.Target))
				busy := a.instBusy != ""
				ui.Row(c).Gap(8).Padding(4, 0, 0).Children(func() {
					if install.Command(in.Kind, install.Update, in.Path) == "" {
						ui.Text(c, locales.NoUpdateCommand()).FontSize(13).TextColor(p.muted)
						return
					}
					ui.PrimaryButton(c.Key("update"), locales.Update()).Disabled(busy).OnClick(func() { a.runInstall(install.Update) })
					ui.Button(c.Key("reinstall"), locales.ReinstallButton()).Disabled(busy).OnClick(func() { a.reinstallQ = true })
					if canChooseVersion(in) {
						ui.Button(c.Key("install-version"), locales.InstallVersionButton()).Disabled(busy).OnClick(func() { a.openVersionDialog("") })
					}
					if a.updateAvailable() {
						ui.Button(c.Key("whats-new"), locales.WhatsNew()).Tooltip(changelogURL).OnClick(func() { c.OpenURL(changelogURL) })
					}
					if busy {
						ui.Spinner(c)
					}
				})
				if in.Kind == install.Homebrew {
					ui.Text(c, locales.NoVersionCommand()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
				}
				ui.Text(c, locales.OfficialCommands()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
			})
			if a.instBusy != "" || len(a.instLog) > 0 || a.instErr != "" {
				card(c, p, iconTerminal, p.gray, locales.Output(), "", nil, func() {
					mono(c, p, "$ "+firstNonEmpty(a.instBusy, a.instDone))
					ui.Scroll(c.Key("log")).MaxHeight(240).Padding(10, 12).Radius(10).Background(p.well).Children(func() {
						for i, l := range a.instLog {
							ui.Text(c.Key(i), l).Font("monospace").FontSize(12).TextColor(p.text).Selectable()
						}
					})
					switch {
					case a.instErr != "":
						ui.Text(c, a.instErr).FontSize(13).TextColor(p.red)
					case a.instDone != "" && a.instBusy == "":
						ui.Text(c, locales.CommandDone()).FontSize(13).TextColor(p.green)
					}
				})
			}
		})
		ui.Column(c).Gap(20).MinWidth(0).Children(func() {
			card(c, p, iconFolder, p.blue, locales.OnPath(), locales.InstallsCount(len(a.inst.Installs)), nil, func() {
				if len(a.inst.Installs) == 0 {
					ui.Text(c, locales.NoneFound()).FontSize(13).TextColor(p.muted)
				}
				ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
					for i, it := range a.inst.Installs {
						note := kindName(string(it.Kind))
						if it.Version != "" {
							note += " · " + it.Version
						}
						title := a.shortPath(it.Path)
						row(c.Key(i), p, title, note, func() {
							if i == 0 {
								pill(c, p, nil, p.green, locales.Runs())
							}
						})
					}
				})
			})
			a.versionsCard(c, p)
		})
	})
	if in != nil {
		switch ui.AlertDialog(c, &a.reinstallQ, locales.ReinstallQuestion(), locales.ReinstallDetail(install.Command(in.Kind, install.Reinstall, in.Path)), locales.Reinstall(), locales.Cancel()) {
		case 0:
			a.runInstall(install.Reinstall)
		}
		a.versionDialog(c, p, in)
	}
}

// canChooseVersion reports whether the way in was installed can install a
// version of one's choice.
func canChooseVersion(in *install.Install) bool {
	return install.VersionCommand(in.Kind, in.Path, "0.0.0") != ""
}

// versionDialog installs a version of one's choice: typed, or picked from
// npm's newest, with the command it runs, and whether to keep it from
// auto-update.
func (a *app) versionDialog(c *ui.Context, p palette, in *install.Install) {
	ui.Modal(c, &a.verOpen, func() {
		ui.Column(c).Width(460).Gap(14).Children(func() {
			ui.Text(c, locales.InstallVersion()).FontSize(17).FontWeight(700)
			ui.Text(c, locales.InstallVersionNote()).FontSize(13).TextColor(p.muted).LineHeight(1.4)
			placeholder := a.latest
			if placeholder == "" {
				placeholder = "2.1.288"
			}
			submitted := ui.TextInput(c.Key("ver-want"), &a.verWant).Placeholder(placeholder).Label(locales.Version()).AutoFocus().Submitted()
			want := strings.TrimSpace(a.verWant)
			valid := install.ValidVersion(want)
			if want != "" && !valid {
				ui.Text(c, locales.InvalidVersion()).FontSize(12).TextColor(p.red)
			}
			ui.Text(c, locales.RecentVersions()).FontSize(13).FontWeight(600).Padding(4, 0, 0)
			switch {
			case a.verListing:
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c)
					ui.Text(c, locales.FetchingVersions()).FontSize(13).TextColor(p.muted)
				})
			case a.verListErr:
				ui.Text(c, locales.VersionsUnavailable()).FontSize(13).TextColor(p.muted).LineHeight(1.4)
			default:
				a.versionList(c, p, in, want)
			}
			ui.Column(c).Gap(4).Children(func() {
				ui.Checkbox(c.Key("ver-keep"), &a.verKeep, locales.KeepVersion())
				ui.Text(c, locales.KeepVersionNote(a.shortPath(filepath.Join(a.env.MainDir(), "settings.json")))).FontSize(12).TextColor(p.faint).LineHeight(1.35)
			})
			if valid {
				mono(c, p, "$ "+install.VersionCommand(in.Kind, in.Path, want))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Padding(6, 0, 0).Children(func() {
				ui.Button(c.Key("ver-cancel"), locales.Cancel()).OnClick(func() { a.verOpen = false })
				ok := valid && a.instBusy == ""
				label := locales.InstallVersionButton()
				if valid {
					label = locales.InstallThis(want)
				}
				if ui.PrimaryButton(c.Key("ver-install"), label).Disabled(!ok).Clicked() || submitted && ok {
					a.verOpen = false
					a.installVersion(want, a.verKeep)
				}
			})
		})
	})
}

// versionList is npm's newest versions, the one chosen lit, each marked
// when it is the one installed, the newest, or a native version kept.
func (a *app) versionList(c *ui.Context, p palette, in *install.Install, want string) {
	kept := map[string]bool{}
	if in.Kind == install.Native {
		for _, v := range a.inst.Versions {
			kept[v.Name] = true
		}
	}
	ui.Scroll(c.Key("ver-list")).MaxHeight(200).Padding(4).Radius(10).Background(p.well).Children(func() {
		for _, v := range a.verList {
			r := ui.Row(c.Key(v)).Gap(8).Padding(6, 10).Radius(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			if v == want {
				r.Background(p.tint(p.blue))
			}
			r.OnClick(func() { a.chooseVersion(v) }).Children(func() {
				ui.Text(c, v).Font("monospace").FontSize(13).TextColor(p.text).Grow(1)
				switch {
				case v == in.Version:
					pill(c, p, nil, p.green, locales.Current())
				case kept[v]:
					pill(c, p, nil, p.gray, locales.Downloaded())
				}
				if v == a.latest {
					pill(c, p, nil, p.blue, locales.Latest())
				}
			})
		}
	})
}

// versionStateWidth is the width of the end of a native version's row.
const versionStateWidth = 104

func (a *app) versionsCard(c *ui.Context, p palette) {
	vs := a.inst.Versions
	// The native versions can be switched to when native is what runs.
	in := a.inst.Active()
	canUse := in != nil && in.Kind == install.Native && a.instBusy == ""
	card(c, p, iconArchive, p.amber, locales.NativeVersions(), locales.VersionsCount(len(vs)), nil, func() {
		if len(vs) == 0 {
			ui.Text(c, locales.NoNativeVersions()).FontSize(13).TextColor(p.muted)
			return
		}
		var chosen int
		var chosenBytes int64
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			for _, v := range vs {
				note := clean.Size(v.Bytes)
				row(c.Key(v.Name), p, v.Name, note, func() {
					if canUse && !v.Current {
						ui.Button(c.Key("use"), locales.Use()).OnClick(func() { a.openVersionDialog(v.Name) })
					}
					// What a version is, or the box removing it, in a column
					// of its own width, so that the Use buttons line up.
					ui.Row(c).Width(versionStateWidth).Justify(ui.End).AlignItems(ui.Center).Shrink(0).Children(func() {
						switch {
						case v.Current:
							pill(c, p, nil, p.green, locales.Current())
						case v.InUse:
							pill(c, p, nil, p.blue, locales.InUse())
						default:
							on := a.chosenVers[v.Name]
							if ui.Checkbox(c.Key("ck"), &on, locales.Remove()).Changed() {
								a.chosenVers[v.Name] = on
							}
						}
					})
				})
				if !v.Current && !v.InUse && a.chosenVers[v.Name] {
					chosen++
					chosenBytes += v.Bytes
				}
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Button(c.Key("remove-versions"), locales.RemoveVersions(chosen, clean.Size(chosenBytes))+"…").Disabled(chosen == 0).OnClick(func() { a.removeVerQ = true })
		})
		ui.Text(c, locales.VersionsNote()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
		if ui.AlertDialog(c, &a.removeVerQ, locales.RemoveVersionsQuestion(chosen), locales.RemoveVersionsDetail(), locales.Remove(), locales.Cancel()) == 0 {
			a.removeVersions()
		}
	})
}

// changelogURL is Claude Code's list of changes, for an update.
const changelogURL = "https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md"

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
