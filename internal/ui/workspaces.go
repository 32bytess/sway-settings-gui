package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

// MakeWorkspacesTab edits names, output pinning and gaps of numbered
// workspaces. Names are applied by rewriting the bindings that switch to
// or move to each workspace.
func MakeWorkspacesTab(cfg *config.Config) fyne.CanvasObject {
	var outputs []string
	for _, o := range cfg.Outputs {
		if o.Detected != nil {
			outputs = append(outputs, o.Name)
		}
	}
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	grid := container.NewGridWithColumns(5,
		widget.NewLabelWithStyle("Workspace", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Inner gaps", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Outer gaps", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)
	gapsValidator := func(s string) error {
		if s == "" {
			return nil
		}
		if n, err := strconv.Atoi(s); err != nil || n < 0 {
			return fmt.Errorf("pixels")
		}
		return nil
	}
	for i := range cfg.Workspaces {
		w := &cfg.Workspaces[i]
		n := w.Number

		name := widget.NewEntry()
		name.SetPlaceHolder("e.g. web")
		name.SetText(w.Label)
		name.Validator = func(s string) error {
			for _, r := range s {
				if r == ' ' || r == '"' || r == '\'' || r == '\\' || r < 0x20 {
					return fmt.Errorf("no spaces or quotes")
				}
			}
			return nil
		}
		name.OnChanged = func(s string) {
			if name.Validate() != nil {
				return // shown on the field; applied once valid
			}
			if err := cfg.SetWorkspaceLabel(n, s); err != nil {
				status.SetText(fmt.Sprintf("Workspace %d: %v", n, err))
				return
			}
			status.SetText("")
		}

		output := widget.NewSelectEntry(outputs)
		output.SetPlaceHolder("any")
		output.SetText(w.Outputs)
		output.OnChanged = func(s string) { w.Outputs = s }

		inner := widget.NewEntry()
		inner.SetPlaceHolder("default")
		inner.SetText(w.GapsInner)
		inner.Validator = gapsValidator
		inner.OnChanged = func(s string) { w.GapsInner = s }

		outer := widget.NewEntry()
		outer.SetPlaceHolder("default")
		outer.SetText(w.GapsOuter)
		outer.Validator = gapsValidator
		outer.OnChanged = func(s string) { w.GapsOuter = s }

		grid.Add(widget.NewLabel(strconv.Itoa(n)))
		grid.Add(name)
		grid.Add(output)
		grid.Add(inner)
		grid.Add(outer)
	}

	note := widget.NewLabel("A name changes the bindings that switch to or move windows to that workspace " +
		"(e.g. $mod+1 becomes \"workspace number 1:web\"); see the Bindings tab. Output pinning and gaps " +
		"are saved to workspaces.conf. Clear a name to go back to the plain number.")
	note.Wrapping = fyne.TextWrapWord
	return container.NewVScroll(container.NewVBox(grid, status, widget.NewSeparator(), note))
}
