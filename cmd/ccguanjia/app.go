package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
)

// ranges are the windows of time Overview shows, with the number of
// buckets of each's dots.
var ranges = []struct {
	d       time.Duration
	buckets int
}{
	{24 * time.Hour, 48},
	{7 * 24 * time.Hour, 56},
	{30 * 24 * time.Hour, 60},
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
	// profiles are those found, launchers the ones of ~/.zshrc.
	profiles       []*claude.Profile
	launchers      []shell.Launcher
	shellInstalled bool
	cliInstalled   bool
	loaded         bool

	// Overview.
	rangeIdx   int
	scanner    *usage.Scanner
	summary    *usage.Summary
	summaryKey string
	storage    map[string]*usage.Storage
	usageGen   int

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

	// Cleanup.
	plans      map[string]*clean.Plan
	planBusy   bool
	cleanOpts  clean.Options
	cleanFor   string // the profile cleanOpts were set up for
	cleanQ     bool
	typed      string
	cleanRun   *cleanRun
	cleanReady bool

	// Profile page.
	newOpen   bool
	newName   string
	newItems  map[string]bool
	newSkip   bool
	newErr    string
	removeQ   bool
	mcpFrom   string
	actionErr string

	// toasts waits for the next frame, which shows it.
	toasts []string
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
		env:        env,
		cfgDir:     cfgDir,
		cfg:        config.Load(cfgDir),
		exe:        exe,
		post:       func(f func()) { f() },
		now:        time.Now,
		page:       pageOverview,
		rangeIdx:   1,
		scanner:    usage.NewScanner(),
		storage:    map[string]*usage.Storage{},
		plans:      map[string]*clean.Plan{},
		chosenVers: map[string]bool{},
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

// refresh reads everything anew.
func (a *app) refresh() {
	a.loadProfiles()
	a.loadInstall()
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
			a.loadUsage()
			a.loadStorage(a.current)
			if a.page == pageCleanup {
				a.loadPlan(a.current)
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

// loadUsage reads the transcripts of the current profile for the range.
func (a *app) loadUsage() {
	p := a.cur()
	if p == nil {
		return
	}
	r := ranges[a.rangeIdx]
	key := fmt.Sprintf("%s/%d", p.Name, a.rangeIdx)
	a.usageGen++
	gen := a.usageGen
	now := a.now()
	dir := p.Dir
	a.do(func() func() {
		sum := a.scanner.Scan(dir, now.Add(-r.d), now, r.buckets)
		return func() {
			if gen == a.usageGen {
				a.summary, a.summaryKey = &sum, key
			}
		}
	})
}

func (a *app) loadStorage(name string) {
	p := a.profile(name)
	if p == nil {
		return
	}
	dir := p.Dir
	a.do(func() func() {
		st := usage.MeasureStorage(dir)
		return func() { a.storage[name] = &st }
	})
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
	if in == nil || a.instBusy != "" {
		return
	}
	cmd := install.Command(in.Kind, action, in.Path)
	if cmd == "" {
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
		return func() {
			for _, l := range lines {
				a.instLog = appendLog(a.instLog, l)
			}
			a.instBusy = ""
			if err != nil {
				a.instErr = errText(err)
			} else {
				a.instDone = cmd
			}
			a.loadInstall()
		}
	})
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

// select shows a page, of a profile for those that show one.
func (a *app) show(page, profile string) {
	if profile != "" && profile != a.current {
		a.current = profile
		a.loadUsage()
		a.loadStorage(profile)
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
	}
	a.actionErr = ""
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
