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

var startupKinds = []struct{ kind, label string }{
	{"exec", "At login"},
	{"exec_always", "At login and every reload"},
}

func startupKindLabel(kind string) string {
	for _, k := range startupKinds {
		if k.kind == kind {
			return k.label
		}
	}
	return kind
}

func showStartupEditor(w fyne.Window, title string, s config.StartupEntry, onSave func(config.StartupEntry) error) {
	var labels []string
	for _, k := range startupKinds {
		labels = append(labels, k.label)
	}
	kind := widget.NewSelect(labels, nil)
	if s.Kind == "" {
		s.Kind = "exec"
	}
	kind.SetSelected(startupKindLabel(s.Kind))

	command := widget.NewEntry()
	command.SetPlaceHolder("e.g. nm-applet --indicator")
	command.SetText(s.Command)

	form := widget.NewForm(
		widget.NewFormItem("When", kind),
		widget.NewFormItem("Command", withBrowse(command, w)),
	)
	d := dialog.NewCustomConfirm(title, "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		ns := config.StartupEntry{Kind: "exec", Command: strings.TrimSpace(command.Text)}
		for _, k := range startupKinds {
			if k.label == kind.Selected {
				ns.Kind = k.kind
			}
		}
		if err := onSave(ns); err != nil {
			dialog.ShowError(err, w)
		}
	}, w)
	d.Resize(fyne.NewSize(600, 220))
	d.Show()
}

// MakeStartupTab lists exec/exec_always entries from all config files.
// Entries in startup.conf can be edited, deleted and run immediately.
func MakeStartupTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	selected := -1
	list := widget.NewList(
		func() int { return len(cfg.Startup) },
		func() fyne.CanvasObject {
			when := widget.NewLabel("At login and every reload")
			src := widget.NewLabel("main config (read-only)")
			cmd := widget.NewLabel("")
			cmd.Truncation = fyne.TextTruncateEllipsis
			return container.NewBorder(nil, nil, when, src, cmd)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			s := cfg.Startup[id]
			row := o.(*fyne.Container)
			row.Objects[0].(*widget.Label).SetText(s.Command)
			row.Objects[1].(*widget.Label).SetText(startupKindLabel(s.Kind))
			src := sourceName(s.Source)
			if !s.Editable() {
				src += " (read-only)"
			}
			row.Objects[2].(*widget.Label).SetText(src)
		},
	)
	editBtn := widget.NewButton("Edit…", nil)
	deleteBtn := widget.NewButton("Delete", nil)
	runBtn := widget.NewButton("Run now", nil)
	updateButtons := func() {
		if selected >= 0 && selected < len(cfg.Startup) && cfg.Startup[selected].Editable() {
			editBtn.Enable()
			deleteBtn.Enable()
			runBtn.Enable()
		} else {
			editBtn.Disable()
			deleteBtn.Disable()
			runBtn.Disable()
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
		showStartupEditor(w, "Edit startup program", cfg.Startup[i], func(s config.StartupEntry) error {
			if err := cfg.UpdateStartup(i, s); err != nil {
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
		if err := cfg.DeleteStartup(selected); err != nil {
			dialog.ShowError(err, w)
			return
		}
		refresh()
	}
	runBtn.OnTapped = func() {
		if selected < 0 {
			return
		}
		cmd := cfg.Startup[selected].Command
		if err := config.RunNow(cmd); err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowInformation("Started", fmt.Sprintf("Started: %s", cmd), w)
	}
	addBtn := widget.NewButton("Add program…", func() {
		showStartupEditor(w, "Add startup program", config.StartupEntry{}, func(s config.StartupEntry) error {
			if err := cfg.AddStartup(s); err != nil {
				return err
			}
			refresh()
			list.ScrollToBottom()
			return nil
		})
	})
	note := widget.NewLabel("\"At login\" programs don't start on a reload; use Run now to start one immediately. " +
		"Programs you add are saved to startup.conf. Entries in other files are read-only: " +
		"sway can't cancel an exec from another file.")
	note.Wrapping = fyne.TextWrapWord
	refresh()
	return container.NewBorder(nil, container.NewVBox(container.NewHBox(addBtn, editBtn, deleteBtn, runBtn), note), nil, nil, list)
}
