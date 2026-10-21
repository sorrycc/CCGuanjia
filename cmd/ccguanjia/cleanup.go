package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/clean"
	"ccguanjia/locales"
)

// cleanupPage backs up and cleans the current profile's folder, in the
// order of clean-claude.ts: back up, archive, verify, edit settings.json,
// delete, and sign out last, if asked.
func (a *app) cleanupPage(c *ui.Context, p palette) {
	pr := a.cur()
	if pr == nil {
		return
	}
	if run := a.cleanRun; run != nil && run.profile == pr.Name {
		a.cleanProgress(c, p, run)
		if !run.done {
			return
		}
	}
	pl := a.plans[pr.Name]
	if pl == nil || a.cleanFor != pr.Name {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c)
			ui.Text(c, locales.Scanning()).TextColor(p.muted)
		})
		return
	}
	o := &a.cleanOpts
	if n := len(pl.Running); n > 0 {
		banner(c, p, iconAlert, p.orange, locales.ClaudeRunning(n), func() {
			ui.Checkbox(c.Key("force"), &o.Force, locales.RunAnyway())
		})
	}
	ui.Grid(c).ColumnTracks(a.tracks(c, ui.Fr(1.15), ui.Fr(1))...).Gap(20).AlignItems(ui.Start).Children(func() {
		// What goes, where it is backed up and the button doing it read
		// down the left; the edits of settings.json and signing out, the
		// right.
		ui.Column(c).Gap(20).MinWidth(0).Children(func() {
			a.itemsCard(c, p, pl)
			a.backupCard(c, p, pl)
		})
		ui.Column(c).Gap(20).MinWidth(0).Children(func() {
			a.settingsEditsCard(c, p, pl)
			a.signOutCard(c, p, pl)
		})
	})
}

func (a *app) itemsCard(c *ui.Context, p palette, pl *clean.Plan) {
	card(c, p, iconArchive, p.amber, locales.CardRemoves(), clean.Size(pl.Reclaim()), nil, func() {
		if len(pl.Items) == 0 && len(pl.Variants) == 0 {
			ui.Text(c, locales.NothingToClean()).FontSize(13).TextColor(p.muted)
			return
		}
		a.reclaimSummary(c, p, pl)
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			for _, it := range pl.Items {
				note := itemNote(it.Rel)
				if it.Files > 0 {
					note += " · " + locales.FilesCount(it.Files)
				}
				ui.Row(c.Key("item-"+it.Rel)).Gap(12).AlignItems(ui.Center).Padding(9, 0).Children(func() {
					ui.Checkbox(c.Key("ck"), &it.Selected, "").Label(it.Rel)
					ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, it.Rel).Font("monospace").FontSize(13).TextColor(p.text).SingleLine()
							if it.Kind == clean.Archive {
								pill(c, p, nil, p.blue, locales.ArchivedFirst())
							}
						})
						ui.Text(c, note).FontSize(12).TextColor(p.muted).LineHeight(1.35)
					})
					ui.Text(c, clean.Size(it.Bytes)).FontSize(13).TextColor(p.muted).FontFeatures("tnum").Shrink(0)
				})
			}
		})
		if n := len(pl.Variants); n > 0 {
			var b int64
			var names []string
			for _, v := range pl.Variants {
				b += v.Bytes
				names = append(names, v.Rel)
			}
			ui.Text(c, locales.VariantsNote(strings.Join(names, ", "), clean.Size(b))).FontSize(12).TextColor(p.muted).LineHeight(1.35)
		}
		ui.Text(c, locales.KeptNote()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
	})
}

// reclaimSummary is the folder as it is and as the cleanup chosen leaves
// it, with a bar of the part it removes, which follows the check boxes.
func (a *app) reclaimSummary(c *ui.Context, p palette, pl *clean.Plan) {
	st := a.storage[pl.Profile.Name]
	if st == nil || st.Total <= 0 {
		return
	}
	reclaim := min(pl.Reclaim(), st.Total)
	ui.Column(c).Gap(10).Padding(0, 0, 6).Children(func() {
		ui.Row(c).Gap(14).AlignItems(ui.End).Children(func() {
			side := func(label, value string, color ui.Color) {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, label).FontSize(12).TextColor(p.muted).SingleLine()
					ui.Text(c, value).FontSize(22).FontWeight(700).TextColor(color).FontFeatures("tnum").SingleLine().ContentTransition(numeric)
				})
			}
			side(locales.FolderNow(), clean.Size(st.Total), p.text)
			ui.Text(c, "→").FontSize(18).TextColor(p.faint).Padding(0, 0, 3)
			side(locales.AfterCleanup(), clean.Size(st.Total-reclaim), p.green)
			ui.Box(c).Grow(1)
			pillIn(ui.Row(c).Margin(0, 0, 4), c, p, iconSparkles, p.amber, "−"+clean.Size(reclaim))
		})
		track := ui.Box(c).Height(10).Radius(5).Background(p.well)
		f := track.AnimateTo("f", float32(float64(reclaim)/float64(st.Total)), ui.Smooth) * easeOutCubic(grow(track))
		track.Draw(func(pt *ui.Painter, r ui.Rect) {
			kept := r.W * (1 - f)
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(kept, r.H), H: r.H}, p.green.Alpha(0.75), r.H/2)
			if f > 0 {
				x := r.X + kept + 2
				pt.FillGradient(ui.Rect{X: x, Y: r.Y, W: max(r.X+r.W-x, r.H), H: r.H}, ui.LinearGradient{From: p.amber.Alpha(0.7), To: p.amber, Angle: 90}, r.H/2)
			}
		})
	})
}

// itemNote says what an item of a cleanup is.
func itemNote(rel string) string {
	switch rel {
	case "projects":
		return locales.NoteProjects()
	case "file-history":
		return locales.NoteFileHistory()
	case "backups":
		return locales.NoteBackups()
	case "jobs", "tasks":
		return locales.NoteJobs()
	case "paste-cache":
		return locales.NotePasteCache()
	case "shell-snapshots":
		return locales.NoteShellSnapshots()
	case "session-env":
		return locales.NoteSessionEnv()
	case ".DS_Store":
		return locales.NoteDSStore()
	case "daemon.log", "daemon-auth-cooldown", "daemon-auth-status.json":
		return locales.NoteDaemon()
	case "stats-cache.json":
		return locales.NoteStatsCache()
	case ".last-update-result.json":
		return locales.NoteUpdateResult()
	case clean.StrayConfig:
		return locales.NoteStray()
	case "teams", "downloads":
		return locales.NoteEmpty()
	}
	if strings.HasPrefix(rel, "telemetry/") {
		return locales.NoteTelemetry()
	}
	return ""
}

func (a *app) settingsEditsCard(c *ui.Context, p palette, pl *clean.Plan) {
	s := pl.Settings
	o := &a.cleanOpts
	card(c, p, iconFile, p.blue, "settings.json", "", nil, func() {
		if s.Path == "" {
			ui.Text(c, locales.NoSettings()).FontSize(13).TextColor(p.muted)
			return
		}
		if s.Err != "" {
			ui.Text(c, locales.SettingsUnreadable(s.Err)).FontSize(13).TextColor(p.red)
			return
		}
		if s.Shared {
			banner(c, p, iconLink, p.purple, locales.SettingsShared(a.shortPath(s.Path)), nil)
		}
		from := locales.Unset()
		if s.CleanupDays > 0 {
			from = fmt.Sprint(s.CleanupDays)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c.Key("days"), &o.SetCleanupDays, locales.CsCleanupDays()).Grow(1).Shrink(1).MinWidth(0)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
				days := float64(o.CleanupDays)
				if ui.NumberInput(c.Key("days-n"), &days, 1, 3650, 1).Label(locales.CsCleanupDays()).Width(150).Disabled(!o.SetCleanupDays).Changed() {
					o.CleanupDays = int(days)
				}
			})
		})
		diff(c, p, fmt.Sprintf("cleanupPeriodDays: %s → %d", from, o.CleanupDays), o.SetCleanupDays && s.CleanupDays != o.CleanupDays)
		ui.Text(c, locales.CleanupDaysNote()).FontSize(12).TextColor(p.faint).LineHeight(1.35)

		dead := append(append([]string{}, s.DeadAllow...), s.DuplicateAllow...)
		ui.Checkbox(c.Key("dead"), &o.DropDeadAllow, locales.DropDeadAllow(len(dead))).Disabled(len(dead) == 0)
		for _, r := range s.DeadAllow {
			diff(c, p, "permissions.allow: − "+r, o.DropDeadAllow)
		}
		for _, r := range s.DuplicateAllow {
			diff(c, p, "permissions.allow: − "+r+"  ("+locales.Duplicate()+")", o.DropDeadAllow)
		}
		ui.Checkbox(c.Key("orphans"), &o.DropOrphanPlugins, locales.DropOrphans(len(s.OrphanPlugins))).Disabled(len(s.OrphanPlugins) == 0)
		for _, k := range s.OrphanPlugins {
			diff(c, p, "enabledPlugins: − "+k, o.DropOrphanPlugins)
		}
	})
}

// diff is a line of a change to settings.json, faint while not chosen.
func diff(c *ui.Context, p palette, text string, on bool) {
	col := p.faint
	if on {
		col = p.orange
	}
	ui.Text(c, text).Font("monospace").FontSize(12).TextColor(col).Padding(0, 0, 0, 28)
}

func (a *app) signOutCard(c *ui.Context, p palette, pl *clean.Plan) {
	o := &a.cleanOpts
	s := pl.SignOut
	card(c, p, iconLogOut, p.red, locales.CardSignOut(), "", nil, func() {
		if s.Empty() {
			ui.Text(c, locales.NothingToSignOut()).FontSize(13).TextColor(p.muted)
			o.SignOut = false
			return
		}
		ui.Checkbox(c.Key("signout"), &o.SignOut, locales.AlsoSignOut())
		var what []string
		if s.KeychainService != "" {
			what = append(what, "Keychain: "+s.KeychainService)
		}
		if s.CredentialsFile != "" {
			what = append(what, ".credentials.json")
		}
		for _, k := range s.Keys {
			what = append(what, ".claude.json → "+k)
		}
		for _, w := range what {
			diff(c, p, "− "+w, o.SignOut)
		}
		ui.Text(c, locales.SignOutNote()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
		if o.SignOut {
			ui.TextInput(c.Key("typed"), &a.typed).Placeholder(pl.Profile.Name).Label(locales.TypeToConfirm(pl.Profile.Name))
			ui.Text(c, locales.TypeToConfirm(pl.Profile.Name)).FontSize(12).TextColor(p.muted)
		}
	})
}

func (a *app) backupCard(c *ui.Context, p palette, pl *clean.Plan) {
	o := &a.cleanOpts
	card(c, p, iconShield, p.green, locales.CardBackup(), "", nil, func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Padding(8, 12).Radius(10).Background(p.well).Children(func() {
				ui.Text(c, a.shortPath(o.BackupDir)).Font("monospace").FontSize(12).TextColor(p.text).SingleLine()
			})
			ui.Button(c.Key("choose-backup"), locales.Choose()).OnClick(a.chooseBackupDir)
		})
		free := a.freeSpace(o.BackupDir)
		need := pl.SpaceNeeded()
		stats(c, p,
			stat{locales.ToArchive(), clean.Size(pl.ArchiveBytes())},
			stat{locales.EstimatedBackup(), "~" + clean.Size(pl.EstimatedBackup())},
			stat{locales.FreeSpace(), clean.Size(free)})
		if free < need {
			ui.Text(c, locales.NotEnoughSpace(clean.Size(need))).FontSize(12).TextColor(p.red)
		}
		ui.Text(c, locales.BackupOrder()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
		blocked := free < need || len(pl.Running) > 0 && !o.Force || o.SignOut && a.typed != pl.Profile.Name ||
			pl.Reclaim() == 0 && !o.SignOut && !anyEdit(pl, o)
		ui.Row(c).Justify(ui.End).Padding(4, 0, 0).Children(func() {
			ui.PrimaryButton(c.Key("run-clean"), locales.BackUpAndCleanButton()).Disabled(blocked).OnClick(func() { a.cleanQ = true })
		})
	})
	what := clean.Size(pl.Reclaim())
	if o.SignOut {
		what = locales.AndSignIn(what)
	}
	switch ui.AlertDialog(c, &a.cleanQ, locales.CleanQuestion(pl.Profile.Name), locales.CleanDetail(what, a.shortPath(o.BackupDir)), locales.BackUpAndClean(), locales.Cancel()) {
	case 0:
		a.startClean()
	}
}

func anyEdit(pl *clean.Plan, o *clean.Options) bool {
	s := pl.Settings
	return o.SetCleanupDays && s.Path != "" && s.CleanupDays != o.CleanupDays ||
		o.DropDeadAllow && len(s.DeadAllow)+len(s.DuplicateAllow) > 0 ||
		o.DropOrphanPlugins && len(s.OrphanPlugins) > 0
}

func (a *app) chooseBackupDir() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: locales.ChooseBackupFolder(), Directory: true, CreateDirectories: true})
		if err != nil || len(paths) == 0 {
			return
		}
		a.post(func() {
			a.cleanOpts.BackupDir = paths[0]
			a.cfg.BackupDir = paths[0]
			a.saveConfig()
		})
	}()
}

// phases are the steps a cleanup shows, with their names.
func phases() []struct {
	phase clean.Phase
	name  string
} {
	return []struct {
		phase clean.Phase
		name  string
	}{
		{clean.PhaseBackup, locales.PhaseBackup()},
		{clean.PhaseArchive, locales.PhaseArchive()},
		{clean.PhaseVerify, locales.PhaseVerify()},
		{clean.PhaseSettings, locales.PhaseSettings()},
		{clean.PhaseDelete, locales.PhaseDelete()},
		{clean.PhaseSignOut, locales.PhaseSignOut()},
	}
}

// cleanProgress is a cleanup going on, or what it did.
func (a *app) cleanProgress(c *ui.Context, p palette, run *cleanRun) {
	color, title := p.blue, locales.Cleaning()
	switch {
	case run.done && run.err == nil:
		color, title = p.green, locales.CleanDone()
	case run.done:
		color, title = p.red, locales.CleanFailed()
	}
	card(c, p, iconSparkles, color, title, "", nil, func() {
		ui.Row(c).Gap(18).Wrap().Children(func() {
			for _, ph := range phases() {
				if ph.phase == clean.PhaseSignOut && !a.cleanOpts.SignOut {
					continue
				}
				state := 0 // waiting
				switch {
				case run.done && run.err == nil, run.progress.Phase > ph.phase:
					state = 2
				case run.progress.Phase == ph.phase:
					state = 1
					if run.done {
						state = 3
					}
				}
				ui.Row(c.Key(ph.name)).Gap(6).AlignItems(ui.Center).Children(func() {
					switch state {
					case 2:
						ui.Icon(c, iconCheck).Size(15, 15).TextColor(p.green)
					case 1:
						ui.Spinner(c).Size(14, 14)
					case 3:
						ui.Icon(c, iconAlert).Size(15, 15).TextColor(p.red)
					default:
						ui.Box(c).Size(8, 8).Radius(4).Background(p.faint)
					}
					label := ph.name
					if state == 1 && run.progress.Total > 0 {
						label += fmt.Sprintf(" %d/%d", run.progress.N, run.progress.Total)
					}
					col := p.muted
					if state > 0 {
						col = p.text
					}
					ui.Text(c, label).FontSize(13).TextColor(col)
				})
			}
		})
		progressBar(c, p, color, cleanFraction(run, a.cleanOpts.SignOut))
		if !run.done {
			if run.progress.Item != "" {
				mono(c, p, run.progress.Item)
			}
			if run.progress.Phase < clean.PhaseSettings {
				ui.Row(c).Children(func() {
					ui.Button(c.Key("stop"), locales.Stop()).OnClick(run.cancel)
				})
			}
			return
		}
		if run.err != nil {
			ui.Text(c, cleanError(run.err)).FontSize(13).TextColor(p.red).Selectable().LineHeight(1.4)
		}
		if res := run.result; res != nil && run.err == nil {
			stats(c, p, stat{locales.Removed(), clean.Size(res.Freed)}, stat{locales.BackupSize(), clean.Size(res.BackupBytes)}, stat{locales.FolderNow(), clean.Size(res.After)})
			for _, e := range res.Edits {
				diff(c, p, e, true)
			}
			for _, s := range res.SignedOut {
				diff(c, p, "− "+s, true)
			}
			if len(res.SignedOut) > 0 {
				ui.Text(c, locales.SignInAgain()).FontSize(12).TextColor(p.orange)
			}
		}
		ui.Row(c).Gap(8).Children(func() {
			if res := run.result; res != nil {
				ui.Button(c.Key("reveal"), locales.ShowBackup()).OnClick(func() { mygo.Shell.ShowItemInFolder(firstNonEmpty(res.Manifest, res.Root)) })
			}
			ui.Button(c.Key("done"), locales.Done()).OnClick(func() { a.cleanRun = nil; a.loadPlan(a.current) })
		})
	})
}

// cleanFraction is how far along a cleanup is, from 0 to 1: the phases
// done, and the part of the one going on.
func cleanFraction(run *cleanRun, signOut bool) float64 {
	shown := 0
	at := 0.0
	for _, ph := range phases() {
		if ph.phase == clean.PhaseSignOut && !signOut {
			continue
		}
		switch {
		case run.progress.Phase > ph.phase:
			at++
		case run.progress.Phase == ph.phase && run.progress.Total > 0:
			at += float64(run.progress.N) / float64(run.progress.Total)
		}
		shown++
	}
	if run.done && run.err == nil || shown == 0 {
		return 1
	}
	return at / float64(shown)
}

// progressBar is a bar going smoothly to frac, from 0 to 1.
func progressBar(c *ui.Context, p palette, color ui.Color, frac float64) {
	track := ui.Box(c).Height(6).Radius(3).Background(p.well)
	f := track.AnimateTo("f", float32(frac), ui.Smooth)
	track.Draw(func(pt *ui.Painter, r ui.Rect) {
		if f > 0 {
			pt.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: max(r.H, r.W*f), H: r.H}, ui.LinearGradient{From: color.Alpha(0.7), To: color, Angle: 90}, r.H/2)
		}
	})
}

// cleanError says why a cleanup stopped, and that nothing was deleted
// when it stopped before the deletes.
func cleanError(err error) string {
	switch {
	case errors.Is(err, clean.ErrRunning):
		return locales.ErrRunning()
	case errors.Is(err, clean.ErrNoSpace):
		return locales.ErrNoSpace()
	case errors.Is(err, clean.ErrVerify):
		return locales.ErrVerify(errText(err))
	}
	return errText(err)
}
