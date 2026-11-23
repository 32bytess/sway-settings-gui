package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

// BrowseStartDir is where the program picker opens.
const BrowseStartDir = "/usr/bin"

// replaceProgram swaps the program (first word) of cmd for path, keeping any
// arguments. Paths containing whitespace are single-quoted for the shell.
func replaceProgram(cmd, path string) string {
	if strings.ContainsAny(path, " \t") {
		path = "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
	}
	fields := strings.Fields(cmd)
	if len(fields) <= 1 {
		return path
	}
	return path + " " + strings.Join(fields[1:], " ")
}

// withBrowse places a Browse button next to entry that picks a program,
// starting in BrowseStartDir, and substitutes it into the entry's command.
func withBrowse(entry *widget.Entry, w fyne.Window) fyne.CanvasObject {
	btn := widget.NewButton("Browse…", func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if r == nil { // cancelled
				return
			}
			path := r.URI().Path()
			if err := r.Close(); err != nil {
				dialog.ShowError(fmt.Errorf("closing %s: %w", path, err), w)
			}
			entry.SetText(replaceProgram(entry.Text, path))
		}, w)
		loc, err := storage.ListerForURI(storage.NewFileURI(BrowseStartDir))
		if err != nil {
			dialog.ShowError(fmt.Errorf("opening %s: %w", BrowseStartDir, err), w)
		} else {
			d.SetLocation(loc)
		}
		d.Resize(fyne.NewSize(600, 450))
		d.Show()
	})
	return container.NewBorder(nil, nil, nil, btn, entry)
}

func MakeVariablesTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	modSelect := widget.NewSelect([]string{"Mod4 (Super)", "Mod1 (Alt)"}, func(val string) {
		if val == "Mod1 (Alt)" {
			cfg.ModKey = "Mod1"
		} else {
			cfg.ModKey = "Mod4"
		}
	})
	if cfg.ModKey == "Mod1" {
		modSelect.SetSelected("Mod1 (Alt)")
	} else {
		modSelect.SetSelected("Mod4 (Super)")
	}

	termEntry := widget.NewEntry()
	termEntry.SetText(cfg.Terminal)
	termEntry.OnChanged = func(s string) { cfg.Terminal = s }

	menuEntry := widget.NewEntry()
	menuEntry.SetText(cfg.Menu)
	menuEntry.OnChanged = func(s string) { cfg.Menu = s }

	opacityVal := widget.NewLabel(fmt.Sprintf("%.2f", cfg.Opacity))
	opacitySlider := widget.NewSlider(0.1, 1.0)
	opacitySlider.Step = 0.05
	opacitySlider.SetValue(cfg.Opacity)
	opacitySlider.OnChanged = func(v float64) {
		cfg.Opacity = v
		opacityVal.SetText(fmt.Sprintf("%.2f", v))
	}

	return container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Modifier key", modSelect),
			widget.NewFormItem("Terminal", withBrowse(termEntry, w)),
			widget.NewFormItem("App menu command", withBrowse(menuEntry, w)),
		),
		widget.NewSeparator(),
		widget.NewLabel("Default window opacity (applies to newly opened windows)"),
		container.NewGridWithColumns(2, opacitySlider, opacityVal),
	)
}
