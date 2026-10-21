package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"ccguanjia/internal/claude"
	"ccguanjia/internal/jsonedit"
	"ccguanjia/internal/settings"
	"ccguanjia/locales"
)

// csDraft is a profile's settings.json on the Claude Settings page: the
// file as it was read, and the copy of its object that the form and the
// JSON editor change, which Save writes.
type csDraft struct {
	file *settings.File
	// draft is nil while the file, or the JSON typed, does not parse.
	draft *jsonedit.Object
	// others are the files of the other profiles that are there and are
	// not this one, for Compare.
	others map[string]*settings.File
	// json is whether the JSON editor shows, raw its text and rawErr why
	// it does not parse.
	json   bool
	raw    string
	rawErr string
	review bool
	// conflict is set when a save found the file changed since it was read.
	conflict bool
	// The fields of the form: the model, the new item of each list, and
	// the new variable.
	model          string
	add            map[string]string
	envKey, envVal string
	compareWith    string
	chosen         map[string]bool
}

// base is the object the draft's changes are of.
func (d *csDraft) base() *jsonedit.Object {
	if d.file.Object != nil {
		return d.file.Object
	}
	return jsonedit.New()
}

func (d *csDraft) changes() []settings.Change {
	if d.draft == nil {
		return nil
	}
	return settings.Changes(d.base(), d.draft)
}

// syncFields fills the fields of the form from the draft.
func (d *csDraft) syncFields() {
	d.model = ""
	if d.draft != nil {
		d.draft.Get("model", &d.model)
	}
}

// set sets the value at path in the draft.
func (d *csDraft) set(path []string, v any) {
	if d.draft != nil {
		d.draft.SetIn(path, v)
	}
}

// unset removes the value at path from the draft, and the objects it
// leaves empty that the file did not have, so that adding a value and
// removing it again changes nothing.
func (d *csDraft) unset(path []string) {
	if d.draft == nil {
		return
	}
	d.draft.DeleteIn(path)
	for i := len(path) - 1; i > 0; i-- {
		parent := path[:i]
		if d.base().RawIn(parent) != nil {
			return
		}
		if sub, err := jsonedit.Parse(d.draft.RawIn(parent)); err != nil || len(sub.Keys()) > 0 {
			return
		}
		d.draft.DeleteIn(parent)
	}
}

// setList sets a list, removing it when it is empty and the file had none.
func (d *csDraft) setList(path []string, items []string) {
	if len(items) == 0 && d.base().RawIn(path) == nil {
		d.unset(path)
		return
	}
	if items == nil {
		items = []string{}
	}
	d.set(path, items)
}

// toJSON shows the JSON editor with the draft written as Save writes it.
func (d *csDraft) toJSON() {
	d.json = true
	if d.draft == nil {
		return
	}
	indent := "  "
	if d.file.Data != nil {
		indent = jsonedit.Indent(d.file.Data)
	}
	if out, err := d.draft.Marshal(indent); err == nil {
		d.raw, d.rawErr = string(out), ""
	}
}

// parseRaw takes the JSON typed as the draft, when it parses.
func (d *csDraft) parseRaw() {
	o, err := jsonedit.Parse([]byte(d.raw))
	if err != nil {
		d.rawErr = err.Error()
		return
	}
	d.draft, d.rawErr = o, ""
	d.syncFields()
}

// loadClaudeSettings reads the profile's settings.json for the Claude
// Settings page. A draft with changes is kept when keep, so that the
// window coming to the front loses nothing typed; saving it then finds
// whether the file changed.
func (a *app) loadClaudeSettings(name string, keep bool) {
	p := a.profile(name)
	if p == nil {
		return
	}
	if d := a.csDrafts[name]; keep && d != nil && (len(d.changes()) > 0 || d.rawErr != "") {
		return
	}
	f := settings.Load(p, a.profiles)
	d := &csDraft{file: f, others: map[string]*settings.File{}, add: map[string]string{}, chosen: map[string]bool{}}
	if old := a.csDrafts[name]; old != nil {
		d.json, d.compareWith = old.json, old.compareWith
	}
	if f.Object != nil {
		d.draft = f.Object.Clone()
	}
	d.syncFields()
	if d.json || f.Object == nil && f.Exists {
		d.json, d.raw = true, string(f.Data)
		if d.draft != nil {
			d.toJSON()
		} else {
			d.parseRaw()
		}
	}
	reals := map[string]bool{f.Real: true}
	for _, q := range a.profiles {
		if q.Name == name {
			continue
		}
		of := settings.Load(q, a.profiles)
		if reals[of.Real] || !of.Exists || of.Object == nil {
			continue
		}
		reals[of.Real] = true
		d.others[q.Name] = of
	}
	a.csDrafts[name] = d
}

// saveClaudeSettings writes the current profile's draft, over a change
// made since it was read only when force.
func (a *app) saveClaudeSettings(force bool) {
	d := a.csDrafts[a.current]
	if d == nil || d.draft == nil {
		return
	}
	err := d.file.Save(d.draft, force, filepath.Join(a.cfgDir, "settings-backups"))
	d.review = false
	switch {
	case errors.Is(err, settings.ErrChanged):
		d.conflict = true
	case err != nil:
		a.actionErr = errText(err)
	default:
		a.toast(locales.CsSaved())
		for _, n := range append([]string{d.file.Profile}, d.file.SharedWith...) {
			delete(a.plans, n)
			if n != d.file.Profile {
				delete(a.csDrafts, n)
			}
		}
		a.loadClaudeSettings(a.current, false)
	}
}

// detachSettings makes the current profile's settings.json its own.
func (a *app) detachSettings() {
	p := a.cur()
	if p == nil {
		return
	}
	if err := settings.Detach(p); err != nil {
		a.actionErr = errText(err)
		return
	}
	a.toast(locales.CsDetached())
	clear(a.csDrafts)
	a.loadProfiles()
	a.loadClaudeSettings(p.Name, false)
}

// effortLevels and defaultModes are the choices of their selects; "—"
// leaves the key unset.
var (
	effortLevels = []string{"—", "low", "medium", "high", "xhigh", "max"}
	defaultModes = []string{"—", "default", "acceptEdits", "plan", "auto", "dontAsk", "bypassPermissions"}
)

// claudeSettingsPage edits the current profile's settings.json: a form of
// the keys most changed, or the whole file as JSON, and a review of the
// changes before they are saved.
func (a *app) claudeSettingsPage(c *ui.Context, p palette) {
	pr := a.cur()
	if pr == nil {
		return
	}
	d := a.csDrafts[pr.Name]
	if d == nil {
		a.loadClaudeSettings(pr.Name, true)
		if d = a.csDrafts[pr.Name]; d == nil {
			return
		}
	}
	if a.actionErr != "" {
		banner(c, p, iconAlert, p.red, a.actionErr, func() {
			ui.Button(c, locales.Dismiss()).OnClick(func() { a.actionErr = "" })
		})
	}
	if d.conflict {
		banner(c, p, iconAlert, p.orange, locales.CsChanged(), func() {
			ui.Button(c.Key("cs-reload"), locales.Reload()).OnClick(func() { a.loadClaudeSettings(pr.Name, false) })
			ui.Button(c.Key("cs-overwrite"), locales.Overwrite()).TextColor(p.red).OnClick(func() {
				d.conflict = false
				a.saveClaudeSettings(true)
			})
		})
	}
	f := d.file
	if f.Err != nil && !f.Exists {
		banner(c, p, iconAlert, p.red, locales.SettingsUnreadable(errText(f.Err)), nil)
		return
	}
	a.csToolbar(c, p, d)
	if d.json {
		a.csJSONCard(c, p, d)
	} else {
		ui.Grid(c).ColumnTracks(ui.Fr(1), ui.Fr(1)).Gap(20).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Gap(20).MinWidth(0).Children(func() {
				a.csFileCard(c, p, pr, d)
				a.csGeneralCard(c, p, d)
				a.csPermissionsCard(c, p, d)
			})
			ui.Column(c).Gap(20).MinWidth(0).Children(func() {
				a.csEnvCard(c, p, d)
				a.csPluginsCard(c, p, d)
				a.csHooksCard(c, p, d)
				a.csCompareCard(c, p, d)
			})
		})
	}
	a.csReviewDialog(c, p, d)
}

// csToolbar is the choice of the form or the JSON, and what the changes
// can do.
func (a *app) csToolbar(c *ui.Context, p palette, d *csDraft) {
	n := len(d.changes())
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		i := 0
		if d.json {
			i = 1
		}
		if segmented(c.Key("cs-mode"), p, &i, locales.CsForm(), "JSON") {
			if i == 1 {
				d.toJSON()
			} else if d.rawErr == "" && d.draft != nil {
				d.json = false
				d.syncFields()
			}
		}
		ui.Box(c).Grow(1)
		ui.Text(c, locales.CsChanges(n)).FontSize(13).TextColor(p.muted).ContentTransition(numeric)
		ui.Button(c.Key("cs-discard"), locales.Discard()).Disabled(n == 0 && d.rawErr == "").OnClick(func() {
			a.loadClaudeSettings(d.file.Profile, false)
		})
		ui.PrimaryButton(c.Key("cs-review"), locales.ReviewChanges()).Disabled(n == 0 || d.rawErr != "").OnClick(func() { d.review = true })
	})
	if d.json && d.rawErr == "" && d.draft == nil {
		d.parseRaw()
	}
}

func (a *app) csFileCard(c *ui.Context, p palette, pr *claude.Profile, d *csDraft) {
	f := d.file
	card(c, p, iconFile, p.blue, locales.CardFile(), "", nil, func() {
		mono(c, p, a.shortPath(f.Path))
		switch {
		case f.Link:
			ui.Text(c, locales.CsLinksTo(a.shortPath(f.Real))).FontSize(13).TextColor(p.muted)
		case !f.Exists:
			ui.Text(c, locales.CsNoFile()).FontSize(13).TextColor(p.muted)
		}
		if len(f.SharedWith) > 0 {
			banner(c, p, iconLink, p.purple, locales.CsSharedWith(strings.Join(f.SharedWith, ", ")), nil)
		}
		if f.Link && !pr.Default {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.CsDetachNote()).FontSize(13).TextColor(p.muted).LineHeight(1.35).Grow(1).Shrink(1).MinWidth(0)
				ui.Button(c.Key("cs-detach"), locales.Detach()).Shrink(0).OnClick(a.detachSettings)
			})
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.CsBackupsNote(a.shortPath(filepath.Join(a.cfgDir, "settings-backups")))).FontSize(12).TextColor(p.faint).LineHeight(1.35).Grow(1).Shrink(1).MinWidth(0)
			ui.Button(c.Key("cs-reveal"), locales.ShowInFinder()).Shrink(0).OnClick(func() {
				if !a.sync {
					mygo.Shell.ShowItemInFolder(f.Real)
				}
			})
		})
	})
}

func (a *app) csGeneralCard(c *ui.Context, p palette, d *csDraft) {
	o := d.draft
	card(c, p, iconGear, p.gray, locales.CardGeneral(), "", nil, func() {
		row(c, p, locales.Model(), locales.CsModelNote(), func() {
			if ui.TextInput(c.Key("cs-model"), &d.model).Placeholder("opus").Label(locales.Model()).Font("monospace").Width(190).Changed() {
				if m := strings.TrimSpace(d.model); m != "" {
					d.set([]string{"model"}, m)
				} else {
					d.unset([]string{"model"})
				}
			}
		})
		a.csSelect(c, p, d, "effortLevel", locales.Effort(), locales.CsEffortNote(), effortLevels)
		days := 0.0
		o.Get("cleanupPeriodDays", &days)
		row(c, p, "cleanupPeriodDays", locales.CleanupDaysNote()+" "+locales.CsZeroUnset(), func() {
			if ui.NumberInput(c.Key("cs-days"), &days, 0, 3650, 1).Label("cleanupPeriodDays").Width(150).Changed() {
				if days > 0 {
					d.set([]string{"cleanupPeriodDays"}, int(days))
				} else {
					d.unset([]string{"cleanupPeriodDays"})
				}
			}
		})
		a.csSwitch(c, p, d, "autoCompactEnabled", locales.AutoCompact(), locales.CsAutoCompactNote(), true)
		a.csSwitch(c, p, d, "alwaysThinkingEnabled", locales.AlwaysThinking(), locales.CsAlwaysThinkingNote(), false)
	})
}

// csSelect is a row choosing the value of a key of text among choices,
// with the file's own value among them when it is another.
func (a *app) csSelect(c *ui.Context, p palette, d *csDraft, key, title, note string, choices []string) {
	path := strings.Split(key, ".")
	cur := choices[0]
	d.draft.GetIn(path, &cur)
	if !slices.Contains(choices, cur) {
		choices = append(slices.Clone(choices), cur)
	}
	row(c, p, title, note, func() {
		if ui.Select(c.Key("cs-"+key), &cur, choices).Label(title).Width(190).Changed() {
			if cur == choices[0] {
				d.unset(path)
			} else {
				d.set(path, cur)
			}
		}
	})
}

// csSwitch is a row of a key that is true or false, which is def when
// unset.
func (a *app) csSwitch(c *ui.Context, p palette, d *csDraft, key, title, note string, def bool) {
	on := def
	d.draft.Get(key, &on)
	if switchRow(c.Key("cs-"+key), p, title, note, &on, false) {
		d.set([]string{key}, on)
	}
}

func (a *app) csPermissionsCard(c *ui.Context, p palette, d *csDraft) {
	card(c, p, iconShield, p.green, locales.CardPermissions(), "", nil, func() {
		a.csSelect(c, p, d, "permissions.defaultMode", locales.DefaultMode(), locales.CsDefaultModeNote(), defaultModes)
		a.csList(c, p, d, []string{"permissions", "allow"}, locales.RuleAllow(), "Bash(npm run test:*)")
		a.csList(c, p, d, []string{"permissions", "ask"}, locales.RuleAsk(), "Bash(git push:*)")
		a.csList(c, p, d, []string{"permissions", "deny"}, locales.RuleDeny(), "Read(./.env)")
		a.csList(c, p, d, []string{"permissions", "additionalDirectories"}, locales.AdditionalDirs(), "~/Projects/shared")
	})
}

// csList edits a list of text: its items, each with a button removing it,
// and a field adding one.
func (a *app) csList(c *ui.Context, p palette, d *csDraft, path []string, title, placeholder string) {
	key := strings.Join(path, ".")
	var items []string
	d.draft.GetIn(path, &items)
	ui.Column(c.Key("cs-list-"+key)).Gap(6).Padding(8, 0, 4).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(14).FontWeight(500).TextColor(p.text)
			ui.Text(c, fmt.Sprint(len(items))).FontSize(12).TextColor(p.muted).FontFeatures("tnum")
		})
		if len(items) > 0 {
			ui.Column(c).Gap(4).Children(func() {
				for i, it := range items {
					ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Center).Padding(5, 4, 5, 10).Radius(8).Background(p.well).Children(func() {
						ui.Text(c, it).Font("monospace").FontSize(12).TextColor(p.text).Selectable().Grow(1).Shrink(1).MinWidth(0)
						smallIconButton(c.Key("rm"), p, iconX, locales.RemoveItem(it)).OnClick(func() {
							d.setList(path, slices.Delete(slices.Clone(items), i, i+1))
						})
					})
				}
			})
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			v := d.add[key]
			submitted := ui.TextInput(c.Key("new"), &v).Placeholder(placeholder).Label(locales.AddTo(title)).Font("monospace").Grow(1).MinWidth(0).Submitted()
			d.add[key] = v
			add := ui.Button(c.Key("add"), locales.Add()).Disabled(strings.TrimSpace(v) == "")
			if add.Clicked() || submitted {
				if s := strings.TrimSpace(v); s != "" && !slices.Contains(items, s) {
					d.setList(path, append(slices.Clone(items), s))
				}
				d.add[key] = ""
			}
		})
	})
}

// secretWords mark the variables whose values the form hides.
var secretWords = []string{"KEY", "TOKEN", "SECRET", "PASSWORD"}

func secretVar(name string) bool {
	up := strings.ToUpper(name)
	for _, w := range secretWords {
		if strings.Contains(up, w) {
			return true
		}
	}
	return false
}

func (a *app) csEnvCard(c *ui.Context, p palette, d *csDraft) {
	env, _ := jsonedit.Parse(d.draft.Raw("env"))
	var keys []string
	if env != nil {
		keys = env.Keys()
	}
	card(c, p, iconTerminal, p.teal, locales.CardEnv(), fmt.Sprint(len(keys)), nil, func() {
		ui.Text(c, locales.CsEnvNote()).FontSize(12).TextColor(p.muted).LineHeight(1.35)
		if len(keys) > 0 {
			ui.Column(c).Gap(4).Children(func() {
				for _, k := range keys {
					var val string
					if _, err := env.Get(k, &val); err != nil {
						val = settings.Short(env.Raw(k))
					}
					if secretVar(k) {
						val = "••••••"
					}
					ui.Row(c.Key("env-"+k)).Gap(8).AlignItems(ui.Center).Padding(5, 4, 5, 10).Radius(8).Background(p.well).Children(func() {
						ui.Text(c, k).Font("monospace").FontSize(12).FontWeight(600).TextColor(p.text).Shrink(0)
						ui.Text(c, val).Font("monospace").FontSize(12).TextColor(p.muted).Selectable().Grow(1).Shrink(1).MinWidth(0)
						smallIconButton(c.Key("rm"), p, iconX, locales.RemoveItem(k)).OnClick(func() { d.unset([]string{"env", k}) })
					})
				}
			})
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("env-key"), &d.envKey).Placeholder("NAME").Label(locales.VarName()).Font("monospace").Width(160)
			submitted := ui.TextInput(c.Key("env-val"), &d.envVal).Placeholder(locales.VarValue()).Label(locales.VarValue()).Font("monospace").Grow(1).MinWidth(0).Submitted()
			name := strings.TrimSpace(d.envKey)
			add := ui.Button(c.Key("env-add"), locales.Add()).Disabled(name == "")
			if add.Clicked() || submitted && name != "" {
				d.set([]string{"env", name}, d.envVal)
				d.envKey, d.envVal = "", ""
			}
		})
	})
}

func (a *app) csPluginsCard(c *ui.Context, p palette, d *csDraft) {
	plugins, _ := jsonedit.Parse(d.draft.Raw("enabledPlugins"))
	if plugins == nil || len(plugins.Keys()) == 0 {
		return
	}
	keys := plugins.Keys()
	on := 0
	for _, k := range keys {
		var v bool
		if plugins.Get(k, &v); v {
			on++
		}
	}
	card(c, p, iconPackage, p.orange, locales.CardPlugins(), locales.CsPluginsOn(on, len(keys)), nil, func() {
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			for _, k := range keys {
				var v bool
				plugins.Get(k, &v)
				name, market, _ := strings.Cut(k, "@")
				if switchRow(c.Key("plugin-"+k), p, name, market, &v, false) {
					d.set([]string{"enabledPlugins", k}, v)
				}
			}
		})
	})
}

func (a *app) csHooksCard(c *ui.Context, p palette, d *csDraft) {
	hooks := settings.Hooks(d.draft)
	var status string
	d.draft.GetIn([]string{"statusLine", "command"}, &status)
	card(c, p, iconActivity, p.purple, locales.CardHooks(), fmt.Sprint(len(hooks)), nil, func() {
		a.csSwitch(c, p, d, "disableAllHooks", locales.DisableAllHooks(), locales.CsDisableHooksNote(), false)
		if len(hooks) == 0 {
			ui.Text(c, locales.NoHooks()).FontSize(13).TextColor(p.muted)
		}
		ui.Column(c).Gap(8).Children(func() {
			for i, h := range hooks {
				ui.Column(c.Key(i)).Gap(3).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, h.Event).FontSize(13).FontWeight(600).TextColor(p.text)
						if h.Matcher != "" {
							pill(c, p, nil, p.purple, h.Matcher)
						}
						if h.Type != "" && h.Type != "command" {
							pill(c, p, nil, p.gray, h.Type)
						}
					})
					mono(c, p, h.Command).MaxLines(3)
				})
			}
		})
		if status != "" {
			ui.Column(c).Gap(3).Padding(6, 0, 0).Children(func() {
				ui.Text(c, locales.StatusLine()).FontSize(13).FontWeight(600).TextColor(p.text)
				mono(c, p, status).MaxLines(3)
			})
		}
		ui.Text(c, locales.CsHooksNote()).FontSize(12).TextColor(p.faint).LineHeight(1.35)
	})
}

// csCompareCard shows the keys whose values differ from another profile's
// file, and copies those chosen into the draft.
func (a *app) csCompareCard(c *ui.Context, p palette, d *csDraft) {
	if len(d.others) == 0 {
		return
	}
	var names []string
	for _, q := range a.profiles {
		if d.others[q.Name] != nil {
			names = append(names, q.Name)
		}
	}
	if !slices.Contains(names, d.compareWith) {
		d.compareWith = names[0]
	}
	other := d.others[d.compareWith]
	diffs := settings.CompareKeys(d.draft, other.Object)
	card(c, p, iconCopy, p.teal, locales.CardCompare(), locales.CsKeysDiffer(len(diffs)), nil, func() {
		row(c, p, locales.CompareWith(), a.shortPath(other.Real), func() {
			ui.Select(c.Key("cs-compare"), &d.compareWith, names).Label(locales.CompareWith()).Width(160)
		})
		if len(diffs) == 0 {
			ui.Text(c, locales.CsAlike()).FontSize(13).TextColor(p.muted)
			return
		}
		var chosen []string
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			for _, kd := range diffs {
				note := locales.CsDiffers()
				switch {
				case !kd.There:
					note = locales.CsOnlyHere()
				case !kd.Here:
					note = locales.CsOnlyIn(d.compareWith)
				}
				ui.Row(c.Key("cmp-"+kd.Key)).Gap(10).AlignItems(ui.Center).Padding(7, 0).Children(func() {
					v := d.chosen[kd.Key]
					if ui.Checkbox(c.Key("ck"), &v, "").Label(kd.Key).Changed() {
						d.chosen[kd.Key] = v
					}
					ui.Text(c, kd.Key).Font("monospace").FontSize(13).TextColor(p.text).Grow(1).Shrink(1).MinWidth(0).SingleLine()
					ui.Text(c, note).FontSize(12).TextColor(p.muted).Shrink(0)
				})
				if d.chosen[kd.Key] {
					chosen = append(chosen, kd.Key)
				}
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.CsCopyNote(d.compareWith)).FontSize(12).TextColor(p.faint).LineHeight(1.35).Grow(1).Shrink(1).MinWidth(0)
			ui.Button(c.Key("cs-copy"), locales.CopyChosen()).Shrink(0).Disabled(len(chosen) == 0).OnClick(func() {
				settings.CopyKeys(other.Object, d.draft, chosen)
				clear(d.chosen)
				d.syncFields()
			})
		})
	})
}

// csJSONCard is the whole file as JSON, checked as it is typed.
func (a *app) csJSONCard(c *ui.Context, p palette, d *csDraft) {
	card(c, p, iconFile, p.blue, a.shortPath(d.file.Path), "", nil, func() {
		if ui.TextArea(c.Key("cs-raw"), &d.raw).Label("settings.json").Font("monospace").FontSize(12).Height(480).Changed() {
			d.parseRaw()
		}
		if d.rawErr != "" {
			ui.Text(c, locales.CsInvalidJSON(d.rawErr)).FontSize(12).TextColor(p.red).LineHeight(1.35)
			return
		}
		a.csProblems(c, p, d, true)
	})
}

// csProblems lists the keys whose values are not of their types, and,
// when unknown, the keys left unchecked. It reports whether there are
// problems.
func (a *app) csProblems(c *ui.Context, p palette, d *csDraft, unknown bool) bool {
	if d.draft == nil {
		return false
	}
	problems, unchecked := settings.Check(d.draft)
	for _, pb := range problems {
		ui.Text(c, locales.CsWant(pb.Path, kindText(pb.Want))).FontSize(12).TextColor(p.red).LineHeight(1.35)
	}
	if unknown && len(unchecked) > 0 {
		ui.Text(c, locales.CsUnchecked(strings.Join(unchecked, ", "))).FontSize(12).TextColor(p.faint).LineHeight(1.35)
	}
	return len(problems) > 0
}

func kindText(k settings.Kind) string {
	switch k {
	case settings.Number:
		return locales.KindNumber()
	case settings.Bool:
		return locales.KindBool()
	case settings.Object:
		return locales.KindObject()
	case settings.Strings:
		return locales.KindStrings()
	case settings.StringMap:
		return locales.KindStringMap()
	case settings.BoolMap:
		return locales.KindBoolMap()
	}
	return locales.KindText()
}

// csReviewDialog shows the changes as lines of a diff, and saves them.
func (a *app) csReviewDialog(c *ui.Context, p palette, d *csDraft) {
	if !d.review {
		return
	}
	ui.Modal(c, &d.review, func() {
		ui.Column(c).Width(620).Gap(14).Children(func() {
			ui.Text(c, locales.ReviewTitle()).FontSize(17).FontWeight(700)
			mono(c, p, a.shortPath(d.file.Real))
			if len(d.file.SharedWith) > 0 {
				banner(c, p, iconLink, p.purple, locales.CsSharedWith(strings.Join(d.file.SharedWith, ", ")), nil)
			}
			ui.Scroll(c).MaxHeight(360).Gap(4).Padding(10, 12).Radius(10).Background(p.well).Children(func() {
				for i, ch := range d.changes() {
					col := p.orange
					switch ch.Op {
					case '+':
						col = p.green
					case '-':
						col = p.red
					}
					ui.Text(c.Key(i), ch.String()).Font("monospace").FontSize(12).TextColor(col).Selectable().LineHeight(1.4)
				}
			})
			bad := a.csProblems(c, p, d, false)
			if bad {
				ui.Text(c, locales.CsFixFirst()).FontSize(12).TextColor(p.red)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Padding(6, 0, 0).Children(func() {
				ui.Button(c.Key("cs-cancel"), locales.Cancel()).OnClick(func() { d.review = false })
				ui.PrimaryButton(c.Key("cs-save"), locales.Save()).Disabled(bad).OnClick(func() { a.saveClaudeSettings(false) })
			})
		})
	})
}

// smallIconButton is a small icon of a row, as the one removing an item.
func smallIconButton(c *ui.Context, p palette, icon *ui.SVG, label string) ui.Element {
	b := ui.Box(c).Size(22, 22).Radius(6).Center().Cursor(ui.CursorPointer).Label(label).Tooltip(label).Shrink(0).Transition(quick)
	if b.Hovered() {
		b.Background(p.hover())
	}
	b.Children(func() { ui.Icon(c, icon).Size(12, 12).TextColor(p.muted) })
	return b
}
