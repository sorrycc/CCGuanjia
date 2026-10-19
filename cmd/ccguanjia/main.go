// CCGuanjia (管家, a steward) looks after Claude Code on a Mac: the
// profiles of your own accounts, each a CLAUDE_CONFIG_DIR folder with its
// own sign-in, how Claude Code is installed, and the cleanup of its
// folders. It works on the local files only. Run with `cli` first, it is
// the ccgj command instead.
package main

import (
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/config"
	"ccguanjia/locales"
)

// appID is the bundle identifier, as mygo.json has it.
const appID = "dev.sorrycc.ccguanjia"

// Facts about the app, for Settings.
const (
	homepage = "https://github.com/sorrycc/CCGuanjia"
	issues   = "https://github.com/sorrycc/CCGuanjia/issues"
)

func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return "dev"
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	env := &claude.Env{Home: home, Keychain: claude.SystemKeychain()}
	cfgDir := config.Dir(home)
	if len(os.Args) > 1 && os.Args[1] == "cli" {
		os.Exit(runCLI(env, cfgDir, os.Args[2:], os.Stdout, os.Stderr))
	}

	mygo.App.SetName("CCGuanjia")
	// CCGUANJIA_DATA_DIR keeps the window's state elsewhere, as
	// scripts/dev.sh does.
	if dir := os.Getenv("CCGUANJIA_DATA_DIR"); dir != "" {
		mygo.App.SetPath(mygo.PathUserData, dir)
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
		keepCrashes(dir)
	}
	// Release builds check for updates once a day and offer them in
	// MyGo's update window; development builds never check.
	mygo.Use(native.Plugin)

	exe, _ := os.Executable()
	a := newApp(env, cfgDir, exe)
	var win *mygo.Window
	mygo.App.OnSecondInstance(func([]string, string) {
		if win != nil {
			win.Show()
			win.Focus()
		}
	})
	mygo.App.WhenReady(func() {
		a.setMenu()
		mygo.App.OnLocaleChanged(func(string) { a.setMenu() })
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "CCGuanjia",
			Width:         1240,
			Height:        860,
			MinWidth:      980,
			MinHeight:     640,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHidden,
			Vibrancy:      mygo.VibrancySidebar,
			Content:       ui.View(a.view),
		})
		a.post = win.Update
		// What changes outside the app, a sign-in or a profile made in a
		// terminal, shows as the window comes back to the front.
		win.OnFocus(func() { a.loadProfiles() })
		a.refresh()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// setMenu is the menu bar: the app menu, with Settings and the updates,
// and the Edit and Window menus, so copy and paste work in the fields.
func (a *app) setMenu() {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Label: "CCGuanjia", Submenu: []*mygo.MenuItem{
			{Label: locales.MenuAbout(), Click: func(*mygo.MenuItem, *mygo.Window) { a.post(func() { a.show(pageSettings, "") }) }},
			updater.MenuItem(),
			mygo.Separator(),
			{Label: locales.MenuSettings(), Accelerator: "Cmd+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.post(func() { a.show(pageSettings, "") }) }},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: locales.MenuView(), Submenu: []*mygo.MenuItem{
			{Label: locales.PageOverview(), Accelerator: "Cmd+1", Click: func(*mygo.MenuItem, *mygo.Window) { a.post(func() { a.show(pageOverview, "") }) }},
			{Label: locales.PageInstall(), Accelerator: "Cmd+2", Click: func(*mygo.MenuItem, *mygo.Window) { a.post(func() { a.show(pageInstall, "") }) }},
			{Label: locales.PageCleanup(), Accelerator: "Cmd+3", Click: func(*mygo.MenuItem, *mygo.Window) { a.post(func() { a.show(pageCleanup, "") }) }},
			mygo.Separator(),
			{Label: locales.CommandPalette(), Accelerator: "Cmd+K", Click: func(*mygo.MenuItem, *mygo.Window) { a.post(a.openPalette) }},
			{Label: locales.NewProfile(), Accelerator: "Cmd+N", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.post(func() {
					a.resetNew()
					a.newOpen = true
				})
			}},
			mygo.Separator(),
			{Label: locales.Refresh(), Accelerator: "Cmd+R", Click: func(*mygo.MenuItem, *mygo.Window) { a.post(a.refresh) }},
		}},
		{Role: mygo.RoleWindowMenu},
	}))
}
