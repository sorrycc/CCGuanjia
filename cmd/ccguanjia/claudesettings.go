package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

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
	// rev counts the changes to draft, and memo is what was worked out of
	// it at a rev, so that a frame does not diff the file again.
	rev  int
	memo *csMemo
	// others are the files of the other profiles that are there and are
	// not this one, for Compare.
	others map[string]*settings.File
	// json is whether the JSON editor shows, raw its text, rawErr why it
	// does not parse and rawAt where.
	json   bool
	raw    string
	rawErr string
	rawAt  int
	// code is the JSON editor, synced the text of raw and codeVer the
	// version of its buffer that raw holds.
	code    *editor.State
	synced  string
	codeVer uint64
	// diagVer and diagErr are the version and the error the editor's
	// diagnostics show.
	diagVer uint64
	diagErr string
	review  bool
	// reviewTab is the review's list of keys (0) or the file's lines (1),
	// and diff those lines, of the draft at diffRev.
	reviewTab int
	diff      *editor.DiffState
	diffRev   int
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

// csMemo is what a frame reads of a draft: its changes, the problems of
// its types, and whether the values at paths differ from the file's.
type csMemo struct {
	rev       int
	changes   []settings.Change
	problems  []settings.Problem
	unchecked []string
	modified  map[string]bool
}

// base is the object the draft's changes are of.
func (d *csDraft) base() *jsonedit.Object {
	if d.file.Object != nil {
		return d.file.Object
	}
	return jsonedit.New()
}

// touch marks the draft changed.
func (d *csDraft) touch() { d.rev++ }

func (d *csDraft) memoNow() *csMemo {
	if d.memo != nil && d.memo.rev == d.rev {
		return d.memo
	}
	m := &csMemo{rev: d.rev, modified: map[string]bool{}}
	if d.draft != nil {
		m.changes = settings.Changes(d.base(), d.draft)
		m.problems, m.unchecked = settings.Check(d.draft)
	}
	d.memo = m
	return m
}

func (d *csDraft) changes() []settings.Change { return d.memoNow().changes }

// modified reports whether the value at path differs from the file's.
func (d *csDraft) modified(path []string) bool {
	if d.draft == nil {
		return false
	}
	m := d.memoNow()
	k := strings.Join(path, "\x00")
	v, ok := m.modified[k]
	if !ok {
		v = compactJSON(d.base().RawIn(path)) != compactJSON(d.draft.RawIn(path))
		m.modified[k] = v
	}
	return v
}

func compactJSON(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return string(raw)
	}
	return b.String()
}

// revert puts the file's value at path back in the draft.
func (d *csDraft) revert(path []string) {
	if d.draft == nil {
		return
	}
	if raw := d.base().RawIn(path); raw != nil {
		d.draft.SetRawIn(path, raw)
		d.touch()
	} else {
		d.unset(path)
	}
	d.syncFields()
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
		d.touch()
	}
}

// unset removes the value at path from the draft, and the objects it
// leaves empty that the file did not have, so that adding a value and
// removing it again changes nothing.
func (d *csDraft) unset(path []string) {
	if d.draft == nil {
		return
	}
	d.touch()
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

// output is what saving the draft writes.
func (d *csDraft) output() string {
	if d.draft == nil {
		return ""
	}
	out, err := d.file.Output(d.draft)
	if err != nil {
		return ""
	}
	return string(out)
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
		d.rawErr, d.rawAt = err.Error(), len(d.raw)
		var se *json.SyntaxError
		if errors.As(err, &se) {
			d.rawAt = int(se.Offset)
		}
		return
	}
	d.draft, d.rawErr = o, ""
	d.touch()
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

// csControlWidth is the width of the fields at the end of the form's rows,
// so that they line up.
const csControlWidth = 210

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
	// The form and the JSON come in from a little below as they swap.
	swap := ui.ElementTransition{Animation: &glide, Enter: &ui.Motion{Y: 10}}
	if d.json {
		ui.Column(c.Key("cs-json")).Gap(20).Transition(swap).Children(func() { a.csJSONCard(c, p, d) })
	} else {
		ui.Grid(c.Key("cs-form")).ColumnTracks(ui.Fr(1), ui.Fr(1)).Gap(20).AlignItems(ui.Start).Transition(swap).Children(func() {
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
	// Room under the cards for the bar of the changes.
	ui.Box(c).Height(56).Shrink(0)
	a.csReviewDialog(c, p, d)
}

// csDirty reports whether the current profile's draft has something to
// review or discard, with the number of its changes.
func (a *app) csDirty() (*csDraft, int, bool) {
	d := a.csDrafts[a.current]
	if d == nil {
		return nil, 0, false
	}
	n := len(d.changes())
	return d, n, n > 0 || d.rawErr != ""
}

// csToolbar is the choice of the form or the JSON, and whether there are
// changes.
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
		// What the file is: its own, linked, or not there yet.
		f := d.file
		switch {
		case !f.Exists:
			pill(c, p, iconFile, p.gray, locales.CsNotCreated())
		case f.Link:
			pill(c, p, iconLink, p.purple, locales.CsLinked())
		default:
			pill(c, p, iconFile, p.blue, locales.CsOwnFile())
		}
		col, icon := p.green, iconCheck
		if n > 0 || d.rawErr != "" {
			col, icon = p.orange, iconDot
		}
		ui.Row(c.Key("cs-status")).Gap(5).AlignItems(ui.Center).Padding(3, 9).Radius(999).Background(p.tint(col)).
			Transition(quick).Children(func() {
			ui.Icon(c, icon).Size(12, 12).TextColor(col).ContentTransition(ui.ContentTransition{})
			ui.Text(c, locales.CsChanges(n)).FontSize(12).FontWeight(600).TextColor(col).SingleLine().ContentTransition(numeric)
		})
	})
	if d.json && d.rawErr == "" && d.draft == nil {
		d.parseRaw()
	}
}

// csSaveBar floats at the foot of the page while there are changes, with
// Discard and Review, rising into place as the first change is made.
func (a *app) csSaveBar(c *ui.Context, p palette) {
	d, n, dirty := a.csDirty()
	if a.page != pageClaudeSettings || !dirty {
		return
	}
	invalid := d.rawErr != ""
	ui.Row(c.Key("cs-savebar")).Absolute().Left(0).Right(0).Bottom(22).Justify(ui.Center).
		Transition(ui.ElementTransition{Animation: &glide, Enter: &ui.Motion{Y: 28, Scale: 0.96}, Exit: &ui.Motion{Y: 28, Scale: 0.96}}).
		Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 8, 8, 14).Radius(14).Background(p.card).
				BorderWidth(1).BorderColor(p.border).Shadow(0, 12, 32, -8, p.lift).Shadow(0, 2, 6, 0, p.shadow).
				InsetShadow(0, 1, 0, 0, p.highlight).Children(func() {
				col := p.orange
				if invalid {
					col = p.red
				}
				ui.Box(c).Size(8, 8).Radius(4).Background(col).Shrink(0).Transition(quick)
				msg := locales.CsUnsaved(n)
				if invalid {
					msg = locales.CsInvalidShort()
				}
				ui.Text(c, msg).FontSize(13).FontWeight(500).TextColor(p.text).SingleLine().ContentTransition(numeric)
				ui.Box(c).Width(6)
				ui.Button(c.Key("cs-discard"), locales.Discard()).OnClick(func() {
					a.loadClaudeSettings(d.file.Profile, false)
				})
				ui.PrimaryButton(c.Key("cs-review"), locales.ReviewChanges()).Tooltip("⌘S").Disabled(n == 0 || invalid).
					OnClick(func() { d.review = true })
			})
		})
}

// csReview opens the review of the current profile's changes, as ⌘S does.
func (a *app) csReview() {
	if d, n, _ := a.csDirty(); d != nil && n > 0 && d.rawErr == "" {
		d.review = true
	}
}

func (a *app) csFileCard(c *ui.Context, p palette, pr *claude.Profile, d *csDraft) {
	f := d.file
	card(c, p, iconFile, p.blue, locales.CardFile(), "", nil, func() {
		ui.Column(c).Gap(4).Children(func() {
			mono(c, p, a.shortPath(f.Path)).FontSize(13).TextColor(p.text)
			switch {
			case f.Link:
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconLink).Size(12, 12).TextColor(p.faint)
					ui.Text(c, locales.CsLinksTo(a.shortPath(f.Real))).FontSize(12).TextColor(p.muted)
				})
			case !f.Exists:
				ui.Text(c, locales.CsNoFile()).FontSize(12).TextColor(p.muted)
			}
		})
		if len(f.SharedWith) > 0 {
			banner(c, p, iconUsers, p.purple, locales.CsSharedWith(strings.Join(f.SharedWith, ", ")), nil)
		}
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			if f.Link && !pr.Default {
				row(c, p, locales.CsDetachTitle(), locales.CsDetachNote(), func() {
					ui.Button(c.Key("cs-detach"), locales.Detach()).OnClick(a.detachSettings)
				})
			}
			row(c, p, locales.CsBackups(), locales.CsBackupsNote(a.shortPath(filepath.Join(a.cfgDir, "settings-backups"))), func() {
				ui.Button(c.Key("cs-reveal"), locales.ShowInFinder()).OnClick(func() {
					if !a.sync {
						mygo.Shell.ShowItemInFolder(f.Real)
					}
				})
			})
		})
	})
}

func (a *app) csGeneralCard(c *ui.Context, p palette, d *csDraft) {
	o := d.draft
	card(c, p, iconGear, p.gray, locales.CardGeneral(), "", nil, func() {
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			a.csRow(c, p, d, []string{"model"}, locales.Model(), locales.CsModelNote(), nil, func() {
				if ui.TextInput(c.Key("cs-model"), &d.model).Placeholder("opus").Label(locales.Model()).Font("monospace").Width(csControlWidth).Changed() {
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
			a.csRow(c, p, d, []string{"cleanupPeriodDays"}, "cleanupPeriodDays", locales.CleanupDaysNote()+" "+locales.CsZeroUnset(), nil, func() {
				// The number's field is of its own width: it lines up with
				// the fields above at their left.
				ui.Row(c).Width(csControlWidth).Children(func() {
					if ui.NumberInput(c.Key("cs-days"), &days, 0, 3650, 1).Label("cleanupPeriodDays").Changed() {
						if days > 0 {
							d.set([]string{"cleanupPeriodDays"}, int(days))
						} else {
							d.unset([]string{"cleanupPeriodDays"})
						}
					}
				})
			})
			a.csSwitch(c, p, d, "autoCompactEnabled", locales.AutoCompact(), locales.CsAutoCompactNote(), true)
			a.csSwitch(c, p, d, "alwaysThinkingEnabled", locales.AlwaysThinking(), locales.CsAlwaysThinkingNote(), false)
		})
	})
}

// csRow is a row of the form for the value at path: its title, with a dot
// after it while the value differs from the file's and a button putting
// the file's back, a note under it, and the control at its end. A click
// on the title calls onTitle, when set, as a switch's label does.
func (a *app) csRow(c *ui.Context, p palette, d *csDraft, path []string, title, note string, onTitle func(), end func()) {
	mod := d.modified(path)
	key := strings.Join(path, ".")
	ui.Row(c.Key("row-"+key)).Gap(12).AlignItems(ui.Center).Padding(11, 0).Children(func() {
		ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				t := ui.Text(c, title).FontSize(14).FontWeight(500).TextColor(p.text).Shrink(1).MinWidth(0)
				if onTitle != nil {
					t.Cursor(ui.CursorPointer)
					if t.Clicked() {
						onTitle()
					}
				}
				if mod {
					modifiedDot(c.Key("mod"), p)
					smallIconButton(c.Key("revert"), p, iconUndo, locales.CsRevert(key)).
						Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.6}, Exit: &ui.Motion{Scale: 0.6}}).
						OnClick(func() { d.revert(path) })
				}
			})
			if note != "" {
				ui.Text(c, note).FontSize(12).TextColor(p.muted).LineHeight(1.4)
			}
		})
		if end != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(end)
		}
	})
}

// modifiedDot marks what differs from the file, popping in as it comes.
func modifiedDot(c *ui.Context, p palette) {
	ui.Box(c).Size(7, 7).Radius(3.5).Background(p.orange).Shrink(0).Tooltip(locales.CsModified()).
		Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.2}, Exit: &ui.Motion{Scale: 0.2}})
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
	a.csRow(c, p, d, path, title, note, nil, func() {
		if ui.Select(c.Key("cs-"+key), &cur, choices).Label(title).Width(csControlWidth).Changed() {
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
	a.csToggle(c, p, d, []string{key}, title, note, def)
}

// csToggle is a row of the value at path that is true or false, def when
// unset, whose title toggles it too.
func (a *app) csToggle(c *ui.Context, p palette, d *csDraft, path []string, title, note string, def bool) {
	on := def
	d.draft.GetIn(path, &on)
	a.csRow(c, p, d, path, title, note, func() { d.set(path, !on) }, func() {
		if ui.Switch(c.Key("cs-"+strings.Join(path, ".")), &on).Label(title).Changed() {
			d.set(path, on)
		}
	})
}

func (a *app) csPermissionsCard(c *ui.Context, p palette, d *csDraft) {
	card(c, p, iconShield, p.green, locales.CardPermissions(), "", nil, func() {
		a.csSelect(c, p, d, "permissions.defaultMode", locales.DefaultMode(), locales.CsDefaultModeNote(), defaultModes)
		a.csList(c, p, d, []string{"permissions", "allow"}, locales.RuleAllow(), "Bash(npm run test:*)", p.green)
		a.csList(c, p, d, []string{"permissions", "ask"}, locales.RuleAsk(), "Bash(git push:*)", p.orange)
		a.csList(c, p, d, []string{"permissions", "deny"}, locales.RuleDeny(), "Read(./.env)", p.red)
		a.csList(c, p, d, []string{"permissions", "additionalDirectories"}, locales.AdditionalDirs(), "~/Projects/shared", p.blue)
	})
}

// listRow is how the items of the form's lists come and go: growing in
// and collapsing out, the others sliding to their places.
var listRow = ui.ElementTransition{Animation: &glide, Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}

// itemKeys are keys of a list's items that stay with each item as others
// come and go, the same text twice counting its times.
func itemKeys(items []string) []string {
	seen := map[string]int{}
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = fmt.Sprintf("%s#%d", it, seen[it])
		seen[it]++
	}
	return keys
}

// csList edits a list of text: its items, each with a button removing it,
// and a field adding one. Items the file lacks show a dot.
func (a *app) csList(c *ui.Context, p palette, d *csDraft, path []string, title, placeholder string, color ui.Color) {
	key := strings.Join(path, ".")
	var items, was []string
	d.draft.GetIn(path, &items)
	d.base().GetIn(path, &was)
	ui.Column(c.Key("cs-list-"+key)).Gap(8).Padding(12, 0, 2).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(color).Shrink(0)
			ui.Text(c, title).FontSize(14).FontWeight(500).TextColor(p.text)
			ui.Text(c, fmt.Sprint(len(items))).FontSize(11).FontWeight(600).TextColor(p.muted).FontFeatures("tnum").
				Padding(1, 7).Radius(999).Background(p.text.Alpha(0.06)).ContentTransition(numeric)
			if d.modified(path) {
				modifiedDot(c.Key("mod"), p)
			}
		})
		keys := itemKeys(items)
		ui.Column(c).Gap(4).Children(func() {
			for i, it := range items {
				fresh := !slices.Contains(was, it)
				ui.Box(c.Key(keys[i])).Transition(listRow).Children(func() {
					bg := p.well
					if fresh {
						bg = p.tint(p.green).Alpha(0.6)
					}
					r := ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(5, 4, 5, 10).Radius(8).Background(bg).Transition(quick)
					r.Children(func() {
						ui.Text(c, it).Font("monospace").FontSize(12).TextColor(p.text).Selectable().Grow(1).Shrink(1).MinWidth(0)
						smallIconButton(c.Key("rm"), p, iconX, locales.RemoveItem(it)).OnClick(func() {
							d.setList(path, slices.Delete(slices.Clone(items), i, i+1))
						})
					})
				})
			}
		})
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

// count is the number a card's title shows, none for 0.
func count(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprint(n)
}

func (a *app) csEnvCard(c *ui.Context, p palette, d *csDraft) {
	env, _ := jsonedit.Parse(d.draft.Raw("env"))
	var keys []string
	if env != nil {
		keys = env.Keys()
	}
	card(c, p, iconTerminal, p.teal, locales.CardEnv(), count(len(keys)), nil, func() {
		ui.Text(c, locales.CsEnvNote()).FontSize(12).TextColor(p.muted).LineHeight(1.4)
		if len(keys) == 0 {
			emptyNote(c, p, iconTerminal, locales.CsNoEnv())
		}
		ui.Column(c).Gap(4).Children(func() {
			for _, k := range keys {
				var val string
				if _, err := env.Get(k, &val); err != nil {
					val = settings.Short(env.Raw(k))
				}
				if secretVar(k) {
					val = "••••••"
				}
				path := []string{"env", k}
				ui.Box(c.Key("env-" + k)).Transition(listRow).Children(func() {
					bg := p.well
					if d.modified(path) {
						bg = p.tint(p.orange).Alpha(0.6)
					}
					ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(5, 4, 5, 10).Radius(8).Background(bg).Transition(quick).Children(func() {
						ui.Text(c, k).Font("monospace").FontSize(12).FontWeight(600).TextColor(p.text).Shrink(0)
						ui.Text(c, val).Font("monospace").FontSize(12).TextColor(p.muted).Selectable().Grow(1).Shrink(1).MinWidth(0).SingleLine().Tooltip(val)
						smallIconButton(c.Key("rm"), p, iconX, locales.RemoveItem(k)).OnClick(func() { d.unset(path) })
					})
				})
			}
		})
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

// emptyNote is what a card shows in place of a list with nothing in it.
func emptyNote(c *ui.Context, p palette, icon *ui.SVG, text string) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(12, 14).Radius(10).
		BorderWidth(1).BorderColor(p.text.Alpha(0.08)).Background(p.well.Alpha(0.6)).Children(func() {
		ui.Icon(c, icon).Size(15, 15).TextColor(p.faint).Shrink(0)
		ui.Text(c, text).FontSize(13).TextColor(p.muted)
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
				name, market, _ := strings.Cut(k, "@")
				a.csToggle(c, p, d, []string{"enabledPlugins", k}, name, market, false)
			}
		})
	})
}

func (a *app) csHooksCard(c *ui.Context, p palette, d *csDraft) {
	hooks := settings.Hooks(d.draft)
	var status string
	d.draft.GetIn([]string{"statusLine", "command"}, &status)
	card(c, p, iconActivity, p.purple, locales.CardHooks(), count(len(hooks)), nil, func() {
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			a.csSwitch(c, p, d, "disableAllHooks", locales.DisableAllHooks(), locales.CsDisableHooksNote(), false)
		})
		if len(hooks) == 0 && status == "" {
			emptyNote(c, p, iconActivity, locales.NoHooks())
		}
		ui.Column(c).Gap(6).Children(func() {
			for i, h := range hooks {
				ui.Column(c.Key(i)).Gap(5).Padding(9, 12).Radius(10).Background(p.well).Children(func() {
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
			if status != "" {
				ui.Column(c.Key("status")).Gap(5).Padding(9, 12).Radius(10).Background(p.well).Children(func() {
					ui.Text(c, locales.StatusLine()).FontSize(13).FontWeight(600).TextColor(p.text)
					mono(c, p, status).MaxLines(3)
				})
			}
		})
		ui.Text(c, locales.CsHooksNote()).FontSize(12).TextColor(p.faint).LineHeight(1.4)
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
			emptyNote(c, p, iconCheck, locales.CsAlike())
			return
		}
		var chosen []string
		ui.Column(c).Dividers(1, p.border).Gap(1).Children(func() {
			for _, kd := range diffs {
				note, col := locales.CsDiffers(), p.orange
				switch {
				case !kd.There:
					note, col = locales.CsOnlyHere(), p.blue
				case !kd.Here:
					note, col = locales.CsOnlyIn(d.compareWith), p.purple
				}
				ui.Row(c.Key("cmp-"+kd.Key)).Gap(10).AlignItems(ui.Center).Padding(8, 0).Children(func() {
					v := d.chosen[kd.Key]
					if ui.Checkbox(c.Key("ck"), &v, "").Label(kd.Key).Changed() {
						d.chosen[kd.Key] = v
					}
					ui.Text(c, kd.Key).Font("monospace").FontSize(13).TextColor(p.text).Grow(1).Shrink(1).MinWidth(0).SingleLine()
					pill(c, p, nil, col, note)
				})
				if d.chosen[kd.Key] {
					chosen = append(chosen, kd.Key)
				}
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.CsCopyNote(d.compareWith)).FontSize(12).TextColor(p.faint).LineHeight(1.4).Grow(1).Shrink(1).MinWidth(0)
			ui.Button(c.Key("cs-copy"), locales.CopyChosen()).Shrink(0).Disabled(len(chosen) == 0).OnClick(func() {
				settings.CopyKeys(other.Object, d.draft, chosen)
				d.touch()
				clear(d.chosen)
				d.syncFields()
			})
		})
	})
}

// codeThemes are the code themes made, one for each look of the window,
// so that the editors keep theirs from frame to frame.
var codeThemes = map[codeLook]*editor.Theme{}

type codeLook struct {
	dark   bool
	accent ui.Color
}

// codeTheme is the JSON editor's colors: the code theme of the window, on
// the well of a card.
func codeTheme(c *ui.Context, p palette) *editor.Theme {
	look := codeLook{p.dark, c.Theme().Accent}
	if t := codeThemes[look]; t != nil {
		return t
	}
	t := editor.DefaultTheme(c.Theme())
	t.Background = p.well
	t.LineNumber = p.faint
	codeThemes[look] = t
	return t
}

// csJSONCard is the whole file as JSON in a code editor, checked as it is
// typed, with the lines changed since the file was read marked in the
// gutter.
func (a *app) csJSONCard(c *ui.Context, p palette, d *csDraft) {
	if d.code == nil {
		buf := editor.NewBuffer(d.raw)
		lang := editor.LanguageFor("settings.json")
		buf.SetLanguage(lang)
		base := editor.NewBuffer(string(d.file.Data))
		base.SetLanguage(lang)
		d.code = &editor.State{Buffer: buf, DiffBase: base, Label: "settings.json", SoftWrap: true, NoScrollPastEnd: true}
		d.synced, d.codeVer = d.raw, buf.Version()
	}
	buf := d.code.Buffer
	// The text set from outside the editor, as the form's changes written
	// out, replaces the editor's, as an edit that undo takes back.
	if d.raw != d.synced {
		buf.Set(d.raw)
		d.synced, d.codeVer = d.raw, buf.Version()
	}
	d.code.Theme = codeTheme(c, p)
	card(c, p, iconFile, p.blue, a.shortPath(d.file.Path), locales.Lines(buf.LineCount()), nil, func() {
		ui.Box(c).Height(520).Radius(10).ClipX().ClipY().BorderWidth(1).BorderColor(p.border).Children(func() {
			editor.View(c, d.code).Fill()
		})
		if v := buf.Version(); v != d.codeVer {
			d.codeVer = v
			d.raw, d.synced = buf.String(), buf.String()
			d.parseRaw()
		}
		// The error of the JSON underlines where it is, set again only as
		// the text changes.
		if v := buf.Version(); v != d.diagVer || d.rawErr != d.diagErr {
			var diags []editor.Diagnostic
			if d.rawErr != "" {
				at := min(max(d.rawAt, 1), buf.Len())
				diags = []editor.Diagnostic{{Start: max(at-1, 0), End: at, Severity: editor.SeverityError, Message: d.rawErr}}
			}
			d.code.SetDiagnostics(v, diags)
			d.diagVer, d.diagErr = v, d.rawErr
		}
		if d.rawErr != "" {
			csNote(c, p, iconAlert, p.red, locales.CsInvalidJSON(d.rawErr))
			return
		}
		if !a.csProblems(c, p, d, true) {
			csNote(c, p, iconCheck, p.green, locales.CsValidJSON())
		}
	})
}

// csNote is a line of what checking the file found, with an icon.
func csNote(c *ui.Context, p palette, icon *ui.SVG, color ui.Color, text string) {
	ui.Row(c).Gap(7).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(color).Shrink(0).Margin(1, 0, 0)
		ui.Text(c, text).FontSize(12).TextColor(color).LineHeight(1.4).Grow(1).Shrink(1).MinWidth(0)
	})
}

// csProblems lists the keys whose values are not of their types, and,
// when unknown, the keys left unchecked. It reports whether there are
// problems.
func (a *app) csProblems(c *ui.Context, p palette, d *csDraft, unknown bool) bool {
	if d.draft == nil {
		return false
	}
	m := d.memoNow()
	for _, pb := range m.problems {
		csNote(c, p, iconAlert, p.red, locales.CsWant(pb.Path, kindText(pb.Want)))
	}
	if unknown && len(m.unchecked) > 0 {
		ui.Text(c, locales.CsUnchecked(strings.Join(m.unchecked, ", "))).FontSize(12).TextColor(p.faint).LineHeight(1.4)
	}
	return len(m.problems) > 0
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

// changeColor is the color of a change of op: added, removed or changed.
func changeColor(p palette, op byte) ui.Color {
	switch op {
	case '+':
		return p.green
	case '-':
		return p.red
	}
	return p.orange
}

// csReviewDialog shows the changes, key by key or as the lines of the
// file, and saves them.
func (a *app) csReviewDialog(c *ui.Context, p palette, d *csDraft) {
	if !d.review {
		return
	}
	chs := d.changes()
	ui.Modal(c, &d.review, func() {
		ui.Column(c).Width(660).Gap(14).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				iconSquare(c, p, iconFile, p.blue, 34)
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, locales.ReviewTitle()).FontSize(17).FontWeight(700).TextColor(p.text)
					mono(c, p, a.shortPath(d.file.Real)).SingleLine()
				})
				var counts [3]int
				for _, ch := range chs {
					counts[strings.IndexByte("+-~", ch.Op)]++
				}
				for i, sign := range []string{"+", "−", "~"} {
					if counts[i] > 0 {
						pill(c, p, nil, changeColor(p, "+-~"[i]), fmt.Sprint(sign, counts[i]))
					}
				}
			})
			if len(d.file.SharedWith) > 0 {
				banner(c, p, iconUsers, p.purple, locales.CsSharedWith(strings.Join(d.file.SharedWith, ", ")), nil)
			}
			ui.Row(c).Children(func() {
				segmented(c.Key("cs-review-tab"), p, &d.reviewTab, locales.CsKeys(), locales.CardFile())
			})
			if d.reviewTab == 0 {
				enter := &stage{}
				ui.Scroll(c.Key("cs-review-keys")).MaxHeight(380).Gap(3).Padding(8).Radius(12).Background(p.well).Children(func() {
					for i, ch := range chs {
						col := changeColor(p, ch.Op)
						r := ui.Row(c.Key(i)).Gap(10).AlignItems(ui.Stretch).Radius(7).Background(col.Alpha(0.08))
						enter.enter(r)
						r.Children(func() {
							ui.Box(c).Width(3).Radius(1.5).Background(col).Shrink(0)
							ui.Text(c, ch.String()).Font("monospace").FontSize(12).TextColor(col).Selectable().LineHeight(1.45).
								Padding(6, 10, 6, 0).Grow(1).Shrink(1).MinWidth(0)
						})
					}
				})
			} else {
				if d.diff == nil || d.diffRev != d.rev {
					lang := editor.LanguageFor("settings.json")
					before, after := editor.NewBuffer(string(d.file.Data)), editor.NewBuffer(d.output())
					before.SetLanguage(lang)
					after.SetLanguage(lang)
					d.diff = &editor.DiffState{Old: before, New: after, Layout: editor.DiffUnified, Wrap: true}
					d.diffRev = d.rev
				}
				d.diff.Theme = codeTheme(c, p)
				ui.Box(c.Key("cs-review-lines")).Height(380).Radius(12).ClipX().ClipY().BorderWidth(1).BorderColor(p.border).Children(func() {
					editor.DiffView(c, d.diff).Fill()
				})
			}
			bad := a.csProblems(c, p, d, false)
			if bad {
				csNote(c, p, iconAlert, p.red, locales.CsFixFirst())
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 0, 0).Children(func() {
				ui.Icon(c, iconArchive).Size(13, 13).TextColor(p.faint).Shrink(0)
				ui.Text(c, locales.CsBacksUp()).FontSize(12).TextColor(p.faint).Grow(1).Shrink(1).MinWidth(0)
				ui.Button(c.Key("cs-cancel"), locales.Cancel()).OnClick(func() { d.review = false })
				ui.PrimaryButton(c.Key("cs-save"), locales.Save()).Disabled(bad).OnClick(func() { a.saveClaudeSettings(false) })
			})
		})
	})
}

// smallIconButton is a small icon of a row, as the one removing an item.
func smallIconButton(c *ui.Context, p palette, icon *ui.SVG, label string) ui.Element {
	b := ui.Box(c).Size(22, 22).Radius(6).Center().Cursor(ui.CursorPointer).Label(label).Tooltip(label).Shrink(0).Transition(quick)
	col := p.muted
	if b.Hovered() {
		b.Background(p.hover())
		col = p.text
	}
	b.Children(func() { ui.Icon(c, icon).Size(12, 12).TextColor(col) })
	return b
}
