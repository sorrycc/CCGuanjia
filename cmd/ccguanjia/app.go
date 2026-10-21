package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/clean"
	"ccguanjia/internal/config"
	"ccguanjia/internal/install"
	"ccguanjia/internal/shell"
	"ccguanjia/internal/usage"
	"ccguanjia/locales"
)

// The pages of the window.
const (
	pageOverview = "overview"
	pageProfile  = "profile"
	pageInstall  = "install"
	pageCleanup  = "cleanup"
	pageSettings = "settings"
	// pageClaudeSettings is a profile's settings.json, apart from the
	// app's own Settings.
	pageClaudeSettings = "claude-settings"
)

// ranges are the windows of time Overview shows, with the number of
// buckets of each's dots.
var ranges = []struct {
	d       time.Duration
	buckets int
}{
	{24 * time.Hour, 24},
	{7 * 24 * time.Hour, 28},
	{30 * 24 * time.Hour, 30},
}

// app is the state of the window: the profiles and what was read of
// them, the page shown and the work going on.
type app struct {
	env *claude.Env
	cfg *config.Config
	// cfgDir is the folder of config.json and shell.zsh.
	cfgDir string
	// exe is the app's executable, which the shell file and the ccgj
	// command run.
	exe string
	// post runs a function on the main thread and draws a frame; sync
	// runs background work at once instead, for tests.
	post func(func())
	sync bool
	now  func() time.Time

	page    string
	current string // the profile the pages show
	// overviewAll is whether Overview sums every profile, rather than
	// showing the current one.
	overviewAll bool
	// profiles are those found, launchers the ones of ~/.zshrc.
	profiles       []*claude.Profile
	launchers      []shell.Launcher
	shellInstalled bool
	cliInstalled   bool
	loaded         bool

	// Overview. usageWant is the key of the summary being read, and
	// byProfile the totals of every profile over the range of byProfileIdx.
	rangeIdx   int
	scanner    *usage.Scanner
	summary    *usage.Summary
	summaryKey string
	usageWant  string
	storage    map[string]*usage.Storage
	// merged is the storage of every profile summed, as Overview shows
	// it, kept until one of mergedOf, those it sums, is measured again.
	merged       *usage.Storage
	mergedOf     []*usage.Storage
	usageGen     int
	usageDone    int
	byProfile    map[string]usage.Period
	byProfileIdx int
	allSessions  bool // Recent Sessions shows them all
	// storageAt is when each profile's folder was measured, so that the
	// window coming to the front does not walk it again each time;
	// measuring holds those being measured.
	storageAt map[string]time.Time
	measuring map[string]bool
	// active is when each profile last wrote a transcript.
	active map[string]time.Time

	// Install. detect finds the installs, the login shell's claude and the
	// newest version, as installDetect does but for tests.
	detect      func() (*install.Info, string, string)
	inst        *install.Info
	shadowed    string
	latest      string
	instLoading bool
	instBusy    string // the command running
	instLog     []string
	instErr     string
	instDone    string
	reinstallQ  bool
	removeVerQ  bool
	chosenVers  map[string]bool
	// The Install Version dialog: the version typed or chosen, whether to
	// turn auto-update off after, and npm's versions, which available
	// lists (as install.Available does but for tests).
	verOpen    bool
	verWant    string
	verKeep    bool
	verList    []string
	verListErr bool
	verListing bool
	available  func() ([]string, error)

	// Cleanup.
	plans      map[string]*clean.Plan
	planBusy   bool
	cleanOpts  clean.Options
	cleanFor   string // the profile cleanOpts were set up for
	cleanQ     bool
	typed      string
	cleanRun   *cleanRun
	cleanReady bool
	// free is the free space of the backup folder, as last measured, and
	// when.
	free     int64
	freeDir  string
	freeTime time.Time

	// Claude Settings: the draft of each profile's settings.json.
	csDrafts map[string]*csDraft

	// Profile page.
	newOpen   bool
	newName   string
	newItems  map[string]bool
	newSkip   bool
	newErr    string
	removeQ   bool
	mcpFrom   string
	actionErr string

	// The command palette: what is typed in it, and the row chosen.
	paletteOpen  bool
	paletteQuery string
	paletteSel   int
	paletteHover string // the row under the pointer
	// clipboard is what a command of the palette copies, which the palette
	// writes as it closes.
	clipboard string

	// scrolls is where the content of each page is scrolled, so that the
	// header shows a hairline once the content goes under it.
	scrolls map[string]*ui.ScrollState

	// toasts waits for the next frame, which shows it.
	toasts []string
	// copiedKey is the button that copied last, at copiedAt, which shows
	// a check for a moment.
	copiedKey string
	copiedAt  time.Time
	// lastTerminal is the command a test would have opened a terminal
	// with.
	lastTerminal string
}

// cleanRun is a cleanup going on, or done.
type cleanRun struct {
	profile  string
	progress clean.Progress
	result   *clean.Result
	err      error
	done     bool
	cancel   context.CancelFunc
}

func newApp(env *claude.Env, cfgDir, exe string) *app {
	a := &app{
		env:         env,
		cfgDir:      cfgDir,
		cfg:         config.Load(cfgDir),
		exe:         exe,
		post:        func(f func()) { f() },
		now:         time.Now,
		page:        pageOverview,
		overviewAll: true,
		rangeIdx:    1,
		scanner:     usage.NewScanner(),
		storage:     map[string]*usage.Storage{},
		storageAt:   map[string]time.Time{},
		measuring:   map[string]bool{},
		active:      map[string]time.Time{},
		plans:       map[string]*clean.Plan{},
		chosenVers:  map[string]bool{},
		csDrafts:    map[string]*csDraft{},
		scrolls:     map[string]*ui.ScrollState{},
	}
	a.resetNew()
	return a
}

// do runs work off the main thread, and what it returns on it.
func (a *app) do(work func() func()) {
	if a.sync {
		work()()
		return
	}
	go func() {
		apply := work()
		a.post(apply)
	}()
}

func (a *app) toast(s string) { a.toasts = append(a.toasts, s) }

// busy reports whether something is still being read.
func (a *app) busy() bool {
	return !a.loaded || a.instLoading || a.planBusy || a.usageDone != a.usageGen && a.summaryKey != a.usageWant
}

// liveWithin is how recently a profile wrote a transcript for it to show
// as working now.
const liveWithin = 5 * time.Minute

// live reports whether a profile wrote a transcript in the last minutes.
func (a *app) live(name string) bool {
	t, ok := a.active[name]
	return ok && !t.IsZero() && a.now().Sub(t) < liveWithin
}

// profile returns the profile name, nil when there is none.
func (a *app) profile(name string) *claude.Profile {
	for _, p := range a.profiles {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// cur is the profile the pages show.
func (a *app) cur() *claude.Profile {
	if p := a.profile(a.current); p != nil {
		return p
	}
	if len(a.profiles) > 0 {
		return a.profiles[0]
	}
	return nil
}

func (a *app) profileNames() []string {
	names := make([]string, len(a.profiles))
	for i, p := range a.profiles {
		names[i] = p.Name
	}
	return names
}

// isDefault reports whether name is the default profile of new shells.
func (a *app) isDefault(name string) bool {
	d := a.cfg.Default
	if d == "" {
		d = claude.DefaultName
	}
	return d == name
}

// refresh reads everything anew, measuring the folders again too.
func (a *app) refresh() {
	clear(a.storageAt)
	a.loadProfiles()
	a.loadInstall()
}

// pulse reads again what changes as Claude Code works: when each profile
// last wrote, and the usage shown. The window does it every minute while
// it is in front; it reads only the transcripts that changed.
func (a *app) pulse() {
	if !a.loaded {
		return
	}
	a.loadActivity()
	a.loadUsage()
}

// loadActivity finds when each profile last wrote a transcript.
func (a *app) loadActivity() {
	dirs := map[string]string{}
	for _, p := range a.profiles {
		dirs[p.Name] = p.Dir
	}
	a.do(func() func() {
		active := make(map[string]time.Time, len(dirs))
		for name, dir := range dirs {
			active[name] = usage.LastActive(dir)
		}
		return func() { a.active = active }
	})
}

// loadProfiles finds the profiles and the launchers of ~/.zshrc. A
// profile seen for the first time takes the skip-permissions flag of the
// launcher ~/.zshrc has for it, so an adopted one starts as it did.
func (a *app) loadProfiles() {
	a.do(func() func() {
		profiles, _ := a.env.Discover()
		launchers := shell.ScanRC(a.env.Home)
		installed := shell.Installed(a.env.Home)
		cli := cliInstalled(a.env.Home)
		return func() {
			a.profiles, a.launchers, a.shellInstalled, a.cliInstalled = profiles, launchers, installed, cli
			a.loaded = true
			changed := false
			for _, p := range profiles {
				if p.Default || a.cfg.Known(p.Name) {
					continue
				}
				o := a.cfg.Options(p.Name)
				for _, l := range launchers {
					if l.Profile == p.Name && l.SkipPermissions {
						o.SkipPermissions = true
					}
				}
				changed = true
			}
			if changed {
				a.cfg.Save()
			}
			a.writeShell()
			if a.profile(a.current) == nil {
				a.current = claude.DefaultName
				if d := a.cfg.Default; d != "" && a.profile(d) != nil {
					a.current = d
				}
			}
			a.loadActivity()
			a.loadUsage()
			a.loadOverviewStorage()
			switch a.page {
			case pageCleanup:
				a.loadPlan(a.current)
			case pageClaudeSettings:
				a.loadClaudeSettings(a.current, true)
			}
		}
	})
}

// writeShell writes shell.zsh for the profiles as they are.
func (a *app) writeShell() {
	var names []string
	for _, p := range a.profiles {
		if !p.Default {
			names = append(names, p.Name)
		}
	}
	shell.Write(a.cfgDir, shell.Render(names, a.cfg, a.cliCommand()))
}

// cliCommand is how the shell file runs ccgj: the command on the PATH
// once installed, else the app's executable.
func (a *app) cliCommand() string {
	if a.cliInstalled || a.exe == "" {
		return ""
	}
	return shell.Quote(a.exe) + " cli"
}

func (a *app) saveConfig() {
	if err := a.cfg.Save(); err != nil {
		a.toast(errText(err))
	}
	a.writeShell()
}

// loadUsage reads the transcripts of every profile for the range: the
// summary Overview shows, of them all or of the current one, and the
// totals of each.
func (a *app) loadUsage() {
	p := a.cur()
	if p == nil {
		return
	}
	r := ranges[a.rangeIdx]
	idx := a.rangeIdx
	key := a.usageKey()
	a.usageGen++
	a.usageWant = key
	gen := a.usageGen
	now := a.now()
	all, name := a.overviewAll, p.Name
	dirs := map[string]string{}
	for _, q := range a.profiles {
		dirs[q.Name] = q.Dir
	}
	a.do(func() func() {
		sums := make(map[string]usage.Summary, len(dirs))
		totals := make(map[string]usage.Period, len(dirs))
		for n, dir := range dirs {
			sum := a.scanner.Scan(dir, now.Add(-r.d), now, r.buckets)
			sums[n] = sum
			totals[n] = usage.Period{Tokens: sum.Tokens, Replies: sum.Replies, Sessions: sum.Sessions}
		}
		var sum usage.Summary
		if all {
			sum = usage.Merge(sums)
		} else {
			sum = usage.Merge(map[string]usage.Summary{name: sums[name]})
		}
		return func() {
			if gen == a.usageGen {
				a.summary, a.summaryKey = &sum, key
				a.usageDone = gen
				a.byProfile, a.byProfileIdx = totals, idx
			}
		}
	})
}

// usageKey names the summary Overview wants: of every profile or of the
// current one, over the range.
func (a *app) usageKey() string {
	name := "*"
	if !a.overviewAll {
		if p := a.cur(); p != nil {
			name = p.Name
		}
	}
	return fmt.Sprintf("%s/%d", name, a.rangeIdx)
}

// storageFresh is how long a folder's measure is shown before the window
// coming to the front measures it again; Refresh always does.
const storageFresh = 2 * time.Minute

func (a *app) loadStorage(name string) {
	p := a.profile(name)
	if p == nil || a.measuring[name] {
		return
	}
	if at, ok := a.storageAt[name]; ok && a.storage[name] != nil && a.now().Sub(at) < storageFresh && !a.now().Before(at) {
		return
	}
	a.measuring[name] = true
	dir := p.Dir
	a.do(func() func() {
		st := usage.MeasureStorage(dir)
		return func() {
			a.storage[name] = &st
			a.storageAt[name] = a.now()
			delete(a.measuring, name)
		}
	})
}

// loadOverviewStorage measures the folders Overview shows: every one, or
// the current profile's.
func (a *app) loadOverviewStorage() {
	if !a.overviewAll {
		a.loadStorage(a.current)
		return
	}
	for _, p := range a.profiles {
		a.loadStorage(p.Name)
	}
}

// overviewStorage is what Overview shows of the folders, nil while one is
// still being measured.
func (a *app) overviewStorage() *usage.Storage {
	if !a.overviewAll {
		if p := a.cur(); p != nil {
			return a.storage[p.Name]
		}
		return nil
	}
	of := make([]*usage.Storage, 0, len(a.profiles))
	for _, p := range a.profiles {
		st := a.storage[p.Name]
		if st == nil {
			return nil
		}
		of = append(of, st)
	}
	if a.merged == nil || !slices.Equal(of, a.mergedOf) {
		sts := make([]usage.Storage, len(of))
		for i, st := range of {
			sts[i] = *st
		}
		st := usage.MergeStorage(sts)
		a.merged, a.mergedOf = &st, of
	}
	return a.merged
}

// loadInstall finds the installs in the PATH of the login shell, and
// asks for the newest version.
func (a *app) loadInstall() {
	if a.instLoading {
		return
	}
	a.instLoading = true
	detect := a.detect
	if detect == nil {
		detect = a.installDetect
	}
	a.do(func() func() {
		info, shadowed, latest := detect()
		return func() {
			a.inst, a.shadowed, a.latest, a.instLoading = info, shadowed, latest, false
			for name := range a.chosenVers {
				a.chosenVers[name] = false
			}
			for _, v := range info.OldVersions() {
				if _, ok := a.chosenVers[v.Name]; !ok {
					a.chosenVers[v.Name] = true
				}
			}
		}
	})
}

// installDetect finds the installs in the PATH of the login shell, and
// asks for the newest version.
func (a *app) installDetect() (*install.Info, string, string) {
	ctx := context.Background()
	path, shadowed := install.ShellProbe(ctx)
	var pids []int
	for _, p := range a.env.Running() {
		pids = append(pids, p.PID)
	}
	info := install.Detect(ctx, a.env.Home, path, install.RunningExecutables(pids))
	latest, _ := install.Latest(ctx, nil)
	return info, shadowed, latest
}

// runInstall runs an update or a reinstall, showing its output.
func (a *app) runInstall(action install.Action) {
	in := a.inst.Active()
	if in == nil {
		return
	}
	a.runCommand(install.Command(in.Kind, action, in.Path), nil)
}

// installVersion installs version in place of the install that runs,
// and then, when keep, turns auto-update off in ~/.claude/settings.json.
func (a *app) installVersion(version string, keep bool) {
	in := a.inst.Active()
	if in == nil {
		return
	}
	var after func() error
	if keep {
		after = func() error { return claude.DisableAutoUpdate(a.env.MainDir()) }
	}
	a.runCommand(install.VersionCommand(in.Kind, in.Path, version), after)
}

// runCommand runs an install command, showing its output, and after it
// when it succeeds.
func (a *app) runCommand(cmd string, after func() error) {
	if cmd == "" || a.instBusy != "" {
		return
	}
	a.instBusy, a.instLog, a.instErr, a.instDone = cmd, nil, "", ""
	a.do(func() func() {
		var lines []string
		err := install.Run(context.Background(), cmd, func(l string) {
			if a.sync {
				lines = append(lines, l)
				return
			}
			a.post(func() { a.instLog = appendLog(a.instLog, l) })
		})
		var afterErr error
		if err == nil && after != nil {
			afterErr = after()
		}
		return func() {
			for _, l := range lines {
				a.instLog = appendLog(a.instLog, l)
			}
			a.instBusy = ""
			switch {
			case err != nil:
				a.instErr = errText(err)
			case afterErr != nil:
				a.instDone = cmd
				a.instErr = errText(afterErr)
			default:
				a.instDone = cmd
				if after != nil {
					a.toast(locales.AutoUpdateOff(a.shortPath(filepath.Join(a.env.MainDir(), "settings.json"))))
				}
			}
			a.loadInstall()
		}
	})
}

// openVersionDialog opens Install Version with version filled in, and
// fetches npm's versions the first time.
func (a *app) openVersionDialog(version string) {
	a.verOpen = true
	a.chooseVersion(version)
	if a.verList != nil || a.verListing {
		return
	}
	a.verListing, a.verListErr = true, false
	available := a.available
	if available == nil {
		available = func() ([]string, error) { return install.Available(context.Background(), nil, 100) }
	}
	a.do(func() func() {
		vs, err := available()
		return func() {
			a.verListing = false
			if err != nil {
				a.verListErr = true
				return
			}
			a.verList = vs
		}
	})
}

// chooseVersion fills in version, keeping it from auto-update when it is
// older than the newest.
func (a *app) chooseVersion(version string) {
	a.verWant = version
	a.verKeep = version != "" && a.latest != "" && install.Compare(version, a.latest) < 0
}

func appendLog(log []string, line string) []string {
	log = append(log, line)
	if len(log) > 300 {
		log = log[len(log)-300:]
	}
	return log
}

// removeVersions removes the native versions chosen.
func (a *app) removeVersions() {
	var vs []install.Version
	for _, v := range a.inst.OldVersions() {
		if a.chosenVers[v.Name] {
			vs = append(vs, v)
		}
	}
	freed, err := install.RemoveVersions(a.env.Home, vs)
	if err != nil {
		a.toast(errText(err))
	} else {
		a.toast(locales.VersionsRemoved(len(vs), clean.Size(freed)))
	}
	a.loadInstall()
}

// loadPlan looks at the profile's folder for Cleanup.
func (a *app) loadPlan(name string) {
	p := a.profile(name)
	if p == nil || a.planBusy {
		return
	}
	a.planBusy = true
	a.do(func() func() {
		pl := clean.NewPlan(a.env, p)
		return func() {
			a.plans[name] = pl
			a.planBusy = false
			if a.cleanFor != name {
				a.cleanFor = name
				a.cleanOpts = clean.Options{
					SetCleanupDays:    true,
					CleanupDays:       365,
					DropDeadAllow:     true,
					DropOrphanPlugins: true,
					BackupDir:         a.backupDir(),
				}
				a.typed = ""
			}
		}
	})
}

// freeSpace is the free space of dir, measured again when dir changes and
// at most every few seconds, rather than in every frame.
func (a *app) freeSpace(dir string) int64 {
	if now := a.now(); dir != a.freeDir || now.Sub(a.freeTime) > 5*time.Second || now.Before(a.freeTime) {
		a.free, a.freeDir, a.freeTime = clean.FreeSpace(dir), dir, now
	}
	return a.free
}

func (a *app) backupDir() string {
	if a.cfg.BackupDir != "" {
		return a.cfg.BackupDir
	}
	return filepath.Join(a.env.Home, "Downloads")
}

// startClean runs the cleanup of the current profile.
func (a *app) startClean() {
	pl := a.plans[a.current]
	if pl == nil || a.cleanRun != nil && !a.cleanRun.done {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &cleanRun{profile: pl.Profile.Name, cancel: cancel}
	a.cleanRun = run
	opts := a.cleanOpts
	a.do(func() func() {
		res, err := clean.Run(ctx, a.env, pl, opts, func(pr clean.Progress) {
			if a.sync {
				run.progress = pr
				return
			}
			a.post(func() { run.progress = pr })
		})
		return func() {
			run.result, run.err, run.done = res, err, true
			delete(a.plans, run.profile)
			a.loadProfiles()
			a.loadStorage(run.profile)
		}
	})
}

// show shows a page, of a profile for those that show one. Overview of a
// profile shows that one alone.
func (a *app) show(page, profile string) {
	reload := false
	if profile != "" && profile != a.current {
		a.current = profile
		reload = true
	}
	if page == pageOverview && profile != "" && a.overviewAll {
		a.overviewAll = false
		reload = true
	}
	if reload {
		a.loadUsage()
		a.loadOverviewStorage()
		a.loadStorage(a.current)
	}
	a.page = page
	switch page {
	case pageCleanup:
		if a.plans[a.current] == nil {
			a.loadPlan(a.current)
		}
	case pageInstall:
		if a.inst == nil {
			a.loadInstall()
		}
	case pageClaudeSettings:
		a.loadClaudeSettings(a.current, true)
	}
	a.actionErr = ""
}

// showAll shows Overview of every profile.
func (a *app) showAll() {
	a.page = pageOverview
	if !a.overviewAll {
		a.overviewAll = true
		a.loadUsage()
		a.loadOverviewStorage()
	}
}

func (a *app) setRange(i int) {
	a.rangeIdx = i
	a.loadUsage()
}

func (a *app) resetNew() {
	a.newName, a.newErr, a.newSkip = "", "", false
	a.newItems = map[string]bool{}
	for _, item := range claude.SharedItems {
		a.newItems[item] = true
	}
}

// createProfile makes the profile of the new-profile dialog.
func (a *app) createProfile() bool {
	name := strings.TrimSpace(a.newName)
	if err := claude.ValidName(name); err != nil {
		a.newErr = locales.BadName()
		return false
	}
	var items []string
	for _, item := range claude.SharedItems {
		if a.newItems[item] {
			items = append(items, item)
		}
	}
	if _, err := a.env.Create(name, items); err != nil {
		a.newErr = errText(err)
		return false
	}
	a.cfg.Options(name).SkipPermissions = a.newSkip
	a.saveConfig()
	a.current, a.page = name, pageProfile
	a.loadProfiles()
	a.toast(locales.Created(shell.FunctionName(name)))
	return true
}

func (a *app) setLink(p *claude.Profile, item string, on bool) {
	var err error
	if on {
		err = a.env.Link(p, item)
	} else {
		err = a.env.Unlink(p, item)
	}
	if err != nil {
		a.actionErr = errText(err)
	}
	a.loadProfiles()
}

func (a *app) copyMCP(to *claude.Profile) {
	from := a.profile(a.mcpFrom)
	if from == nil {
		return
	}
	added, err := claude.CopyMCP(from, to)
	switch {
	case err != nil:
		a.actionErr = errText(err)
	case len(added) == 0:
		a.toast(locales.NoNewServers())
	default:
		a.toast(locales.ServersCopied(len(added)))
	}
	a.loadProfiles()
}

func (a *app) removeProfile(p *claude.Profile) {
	if err := a.env.Remove(p); err != nil {
		a.actionErr = errText(err)
		return
	}
	delete(a.cfg.Profiles, p.Name)
	if a.cfg.Default == p.Name {
		a.cfg.Default = ""
	}
	a.saveConfig()
	a.current, a.page = claude.DefaultName, pageOverview
	a.loadProfiles()
}

func (a *app) setDefault(name string) {
	if name == claude.DefaultName {
		name = ""
	}
	a.cfg.Default = name
	a.saveConfig()
}

func (a *app) setShell(on bool) {
	var err error
	if on {
		a.writeShell()
		err = shell.Install(a.env.Home, filepath.Join(a.cfgDir, shell.FileName))
	} else {
		err = shell.Uninstall(a.env.Home)
	}
	if err != nil {
		a.toast(errText(err))
	}
	a.shellInstalled = shell.Installed(a.env.Home)
}

func (a *app) setCLI(on bool) {
	var err error
	if on {
		err = installCLI(a.env.Home, a.exe)
	} else {
		err = uninstallCLI(a.env.Home)
	}
	if err != nil {
		a.toast(errText(err))
	}
	a.cliInstalled = cliInstalled(a.env.Home)
	a.writeShell()
}

// launchersOf are the hand-written launchers of ~/.zshrc for a profile.
func (a *app) launchersOf(name string) []string {
	var out []string
	for _, l := range a.launchers {
		if l.Profile == name && !slices.Contains(out, l.Name) {
			out = append(out, l.Name)
		}
	}
	return out
}

// errText is an error as the window says it: its first line.
func errText(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// shortPath writes a path under the home folder with ~.
func (a *app) shortPath(path string) string {
	if rel, err := filepath.Rel(a.env.Home, path); err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return "~"
		}
		return "~" + string(os.PathSeparator) + rel
	}
	return path
}
