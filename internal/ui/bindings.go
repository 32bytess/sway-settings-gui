package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

// comboModifiers are the modifier checkboxes in the binding editor, in the
// order they are written, with the text written for each.
var comboModifiers = []struct{ name, text string }{
	{"$mod", "$mod"},
	{"Super", "Mod4"},
	{"Ctrl", "Ctrl"},
	{"Alt", "Alt"},
	{"Shift", "Shift"},
}

var modifierNames = map[string]string{
	"$mod": "$mod", "mod4": "Super", "super": "Super", "ctrl": "Ctrl", "control": "Ctrl",
	"alt": "Alt", "mod1": "Alt", "shift": "Shift",
}

// splitCombo separates the modifiers that have checkboxes from the rest of
// a combo. Other modifiers stay in front of the key, e.g. "Mod3+x".
func splitCombo(combo string) (map[string]bool, string) {
	parts := strings.Split(combo, "+")
	mods := map[string]bool{}
	var rest []string
	for _, p := range parts[:len(parts)-1] {
		// $variables are case-sensitive; real modifier names are not.
		name, ok := modifierNames[p]
		if !ok && !strings.HasPrefix(p, "$") {
			name, ok = modifierNames[strings.ToLower(p)]
		}
		if ok {
			mods[name] = true
		} else {
			rest = append(rest, p)
		}
	}
	return mods, strings.Join(append(rest, parts[len(parts)-1]), "+")
}

func joinCombo(mods map[string]bool, key string) string {
	var parts []string
	for _, m := range comboModifiers {
		if mods[m.name] {
			parts = append(parts, m.text)
		}
	}
	return strings.Join(append(parts, key), "+")
}

// modeLabel shows "" (sway's default mode) as "default".
func modeLabel(mode string) string {
	if mode == "" {
		return "default"
	}
	return mode
}

func modeFromLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "default" {
		return ""
	}
	return label
}

var statusText = map[string]string{
	config.StatusMain:       "",
	config.StatusOverridden: "changed",
	config.StatusRemoved:    "removed",
	config.StatusAdded:      "added",
}

// showBindingEditor edits b; onSave applies it and returns an error to show.
func showBindingEditor(w fyne.Window, title string, modes []string, b config.Binding, onSave func(config.Binding) error) {
	var modeLabels []string
	for _, m := range modes {
		modeLabels = append(modeLabels, modeLabel(m))
	}
	mode := widget.NewSelectEntry(modeLabels)
	mode.SetText(modeLabel(b.Mode))

	kind := widget.NewSelect([]string{"bindsym", "bindcode"}, nil)
	if b.Kind == "" {
		b.Kind = "bindsym"
	}
	kind.SetSelected(b.Kind)

	mods, key := splitCombo(b.Combo)
	if b.Combo == "" {
		mods, key = map[string]bool{"$mod": true}, ""
	}
	modBox := container.NewHBox()
	for _, m := range comboModifiers {
		m := m
		c := widget.NewCheck(m.name, func(v bool) { mods[m.name] = v })
		c.SetChecked(mods[m.name])
		modBox.Add(c)
	}
	keyEntry := widget.NewEntry()
	keyEntry.SetPlaceHolder("e.g. Return, q, XF86AudioMute (bindcode: a number)")
	keyEntry.SetText(key)

	flags := widget.NewEntry()
	flags.SetPlaceHolder("optional, e.g. --locked --release --to-code")
	flags.SetText(strings.Join(b.Flags, " "))

	command := widget.NewEntry()
	command.SetPlaceHolder("e.g. exec foot, kill, workspace number 1")
	command.SetText(b.Command)

	form := widget.NewForm(
		widget.NewFormItem("Mode", mode),
		widget.NewFormItem("Type", kind),
		widget.NewFormItem("Modifiers", modBox),
		widget.NewFormItem("Key", keyEntry),
		widget.NewFormItem("Flags", flags),
		widget.NewFormItem("Command", command),
	)
	d := dialog.NewCustomConfirm(title, "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		nb := config.Binding{
			Mode:    modeFromLabel(mode.Text),
			Kind:    kind.Selected,
			Flags:   strings.Fields(flags.Text),
			Combo:   joinCombo(mods, strings.TrimSpace(keyEntry.Text)),
			Command: strings.TrimSpace(command.Text),
		}
		if err := onSave(nb); err != nil {
			dialog.ShowError(err, w)
		}
	}, w)
	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}

// MakeBindingsTab lists default-mode key bindings with search, add, edit,
// remove and restore.
func MakeBindingsTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	var visible []int // indices into cfg.Bindings
	selected := -1
	query := ""
	modeFilter := "All modes"

	list := widget.NewList(
		func() int { return len(visible) },
		func() fyne.CanvasObject {
			status := widget.NewLabel("overridden")
			mode := widget.NewLabel("[default]")
			combo := widget.NewLabelWithStyle("$mod+Shift+XF86AudioMute", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			cmd := widget.NewLabel("")
			cmd.Truncation = fyne.TextTruncateEllipsis
			return container.NewBorder(nil, nil, container.NewHBox(status, mode, combo), nil, cmd)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			e := cfg.Bindings[visible[id]]
			row := o.(*fyne.Container)
			cmd := row.Objects[0].(*widget.Label)
			left := row.Objects[1].(*fyne.Container)
			left.Objects[0].(*widget.Label).SetText(statusText[e.Status()])
			left.Objects[1].(*widget.Label).SetText("[" + modeLabel(e.Mode) + "]")
			combo := e.Combo
			if len(e.Flags) > 0 {
				combo = strings.Join(e.Flags, " ") + " " + combo
			}
			left.Objects[2].(*widget.Label).SetText(combo)
			cmd.SetText(e.Command)
		},
	)

	editBtn := widget.NewButton("Edit…", nil)
	removeBtn := widget.NewButton("Remove", nil)
	restoreBtn := widget.NewButton("Restore", nil)
	updateButtons := func() {
		if selected < 0 || selected >= len(cfg.Bindings) {
			editBtn.Disable()
			removeBtn.Disable()
			restoreBtn.Disable()
			return
		}
		st := cfg.Bindings[selected].Status()
		editBtn.Enable()
		if st == config.StatusRemoved {
			removeBtn.Disable()
		} else {
			removeBtn.Enable()
		}
		if st == config.StatusRemoved || st == config.StatusOverridden {
			restoreBtn.Enable()
		} else {
			restoreBtn.Disable()
		}
	}
	refresh := func() {
		visible = visible[:0]
		q := strings.ToLower(query)
		for i, e := range cfg.Bindings {
			if modeFilter != "All modes" && modeLabel(e.Mode) != modeFilter {
				continue
			}
			if q == "" || strings.Contains(strings.ToLower(e.Combo+" "+e.Command+" "+statusText[e.Status()]), q) {
				visible = append(visible, i)
			}
		}
		selected = -1
		list.UnselectAll()
		list.Refresh()
		updateButtons()
	}
	list.OnSelected = func(id widget.ListItemID) {
		selected = visible[id]
		updateButtons()
	}

	editBtn.OnTapped = func() {
		if selected < 0 {
			return
		}
		i := selected
		showBindingEditor(w, "Edit binding", cfg.Modes(), cfg.Bindings[i].Binding, func(b config.Binding) error {
			if err := cfg.UpdateBinding(i, b); err != nil {
				return err
			}
			refresh()
			return nil
		})
	}
	removeBtn.OnTapped = func() {
		if selected >= 0 {
			cfg.RemoveBinding(selected)
			refresh()
		}
	}
	restoreBtn.OnTapped = func() {
		if selected >= 0 {
			cfg.RestoreBinding(selected)
			refresh()
		}
	}
	addBtn := widget.NewButton("Add…", func() {
		newB := config.Binding{}
		if modeFilter != "All modes" {
			newB.Mode = modeFromLabel(modeFilter)
		}
		showBindingEditor(w, "Add binding", cfg.Modes(), newB, func(b config.Binding) error {
			if err := cfg.AddBinding(b); err != nil {
				return err
			}
			refresh()
			return nil
		})
	})

	search := widget.NewEntry()
	search.SetPlaceHolder(fmt.Sprintf("Search %d bindings (keys, command, status)", len(cfg.Bindings)))
	search.OnChanged = func(s string) {
		query = s
		refresh()
	}

	note := widget.NewLabel("Changes go to bindings.conf (mode bindings in mode \"…\" { } blocks there); " +
		"the main config is never edited. Typing a new mode name in the editor creates that mode.")
	note.Wrapping = fyne.TextWrapWord

	modeOptions := []string{"All modes"}
	for _, m := range cfg.Modes() {
		modeOptions = append(modeOptions, modeLabel(m))
	}
	modeSel := widget.NewSelect(modeOptions, func(s string) {
		modeFilter = s
		refresh()
	})
	modeSel.SetSelected("All modes")

	refresh()
	top := container.NewBorder(nil, nil, modeSel, addBtn, search)
	bottom := container.NewVBox(container.NewHBox(editBtn, removeBtn, restoreBtn), note)
	return container.NewBorder(top, bottom, nil, nil, list)
}
