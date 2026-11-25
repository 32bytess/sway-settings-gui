package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

var (
	forWindowSuggestions = []string{
		"floating enable", "floating disable", "fullscreen enable", "sticky enable",
		"border none", "border pixel 2", "border normal", "opacity 0.9",
		"move to scratchpad", "move position center", "resize set 800 600",
		"inhibit_idle fullscreen", "inhibit_idle focus",
		"blur enable", "blur disable", "shadows enable", "shadows disable", "corner_radius 0",
	}
	assignSuggestions = []string{
		"workspace number 1", "workspace number 2", "workspace number 3",
		"workspace number 4", "workspace number 5", "output eDP-1",
	}
)

func sourceName(path string) string {
	if path == config.MainConfigPath() {
		return "main config"
	}
	return filepath.Base(path)
}

// showWindowPicker lists open windows and calls onPick with criteria
// matching the chosen one.
func showWindowPicker(w fyne.Window, onPick func(criteria string)) {
	wins, err := config.OpenWindows()
	if err != nil {
		dialog.ShowError(fmt.Errorf("listing open windows: %w", err), w)
		return
	}
	if len(wins) == 0 {
		dialog.ShowInformation("Pick window", "No open windows with an app_id or class were found.", w)
		return
	}
	var d dialog.Dialog
	list := widget.NewList(
		func() int { return len(wins) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			win := wins[i]
			o.(*widget.Label).SetText(fmt.Sprintf("%s — %s", win.Criteria(), win.Title))
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		onPick(wins[i].Criteria())
		d.Hide()
	}
	d = dialog.NewCustom("Pick window", "Cancel", list, w)
	d.Resize(fyne.NewSize(520, 400))
	d.Show()
}

func showRuleEditor(w fyne.Window, title string, r config.WindowRule, onSave func(config.WindowRule) error) {
	if r.Kind == "" {
		r.Kind = "for_window"
	}
	command := widget.NewSelectEntry(forWindowSuggestions)
	command.SetText(r.Command)
	kind := widget.NewSelect([]string{"for_window", "assign"}, func(k string) {
		if k == "assign" {
			command.SetOptions(assignSuggestions)
			command.SetPlaceHolder("workspace number 2, or output DP-1")
		} else {
			command.SetOptions(forWindowSuggestions)
			command.SetPlaceHolder("e.g. floating enable, border none")
		}
	})
	kind.SetSelected(r.Kind)

	criteria := widget.NewEntry()
	criteria.SetPlaceHolder(`e.g. app_id="foot" title="^Settings"  (or: all, floating)`)
	criteria.SetText(r.Criteria)
	pick := widget.NewButton("Pick window…", func() {
		showWindowPicker(w, func(c string) { criteria.SetText(c) })
	})

	form := widget.NewForm(
		widget.NewFormItem("Type", kind),
		widget.NewFormItem("Windows", container.NewBorder(nil, nil, nil, pick, criteria)),
		widget.NewFormItem("Command", command),
	)
	help := widget.NewLabel("for_window runs the command on matching windows when they open. " +
		"assign moves them to a workspace or output. Criteria values are regular expressions.")
	help.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustomConfirm(title, "Save", "Cancel", container.NewVBox(form, help), func(ok bool) {
		if !ok {
			return
		}
		nr := config.WindowRule{
			Kind:     kind.Selected,
			Criteria: strings.TrimSpace(criteria.Text),
			Command:  strings.TrimSpace(command.Text),
		}
		if err := onSave(nr); err != nil {
			dialog.ShowError(err, w)
		}
	}, w)
	d.Resize(fyne.NewSize(600, 320))
	d.Show()
}

// MakeRulesTab lists window rules from all config files. Rules in
// rules.conf can be edited; others are shown read-only.
func MakeRulesTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	selected := -1
	list := widget.NewList(
		func() int { return len(cfg.Rules) },
		func() fyne.CanvasObject {
			src := widget.NewLabel("appearance.conf")
			rule := widget.NewLabel("")
			rule.Truncation = fyne.TextTruncateEllipsis
			return container.NewBorder(nil, nil, nil, src, rule)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			r := cfg.Rules[id]
			row := o.(*fyne.Container)
			row.Objects[0].(*widget.Label).SetText(fmt.Sprintf("%s [%s] %s", r.Kind, r.Criteria, r.Command))
			src := sourceName(r.Source)
			if !r.Editable() {
				src += " (read-only)"
			}
			row.Objects[1].(*widget.Label).SetText(src)
		},
	)
	editBtn := widget.NewButton("Edit…", nil)
	deleteBtn := widget.NewButton("Delete", nil)
	updateButtons := func() {
		if selected >= 0 && selected < len(cfg.Rules) && cfg.Rules[selected].Editable() {
			editBtn.Enable()
			deleteBtn.Enable()
		} else {
			editBtn.Disable()
			deleteBtn.Disable()
		}
	}
	refresh := func() {
		selected = -1
		list.UnselectAll()
		list.Refresh()
		updateButtons()
	}
	list.OnSelected = func(id widget.ListItemID) {
		selected = id
		updateButtons()
	}
	editBtn.OnTapped = func() {
		i := selected
		if i < 0 {
			return
		}
		showRuleEditor(w, "Edit rule", cfg.Rules[i].WindowRule, func(r config.WindowRule) error {
			if err := cfg.UpdateRule(i, r); err != nil {
				return err
			}
			refresh()
			return nil
		})
	}
	deleteBtn.OnTapped = func() {
		if selected < 0 {
			return
		}
		if err := cfg.DeleteRule(selected); err != nil {
			dialog.ShowError(err, w)
			return
		}
		refresh()
	}
	addBtn := widget.NewButton("Add rule…", func() {
		showRuleEditor(w, "Add rule", config.WindowRule{}, func(r config.WindowRule) error {
			if err := cfg.AddRule(r); err != nil {
				return err
			}
			refresh()
			list.ScrollToBottom()
			return nil
		})
	})
	note := widget.NewLabel("Rules you add are saved to rules.conf. Rules in other files are read-only here: " +
		"sway can't undo a for_window or assign from another file, so edit those where they are.")
	note.Wrapping = fyne.TextWrapWord
	refresh()
	return container.NewBorder(nil, container.NewVBox(container.NewHBox(addBtn, editBtn, deleteBtn), note), nil, nil, list)
}
