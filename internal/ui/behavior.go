package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

// MakeBehaviorTab shows look and behaviour commands written to behavior.conf.
// The font gets a font browser and client.* colours get a picker per colour.
func MakeBehaviorTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	section := func(title string, specs []config.SettingSpec) fyne.CanvasObject {
		box := container.NewVBox(widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		form := widget.NewForm()
		var colours []fyne.CanvasObject
		for _, spec := range specs {
			switch {
			case spec.Key == "font":
				form.Append(spec.Label, fontField(w, cfg.Behavior, spec))
			case strings.HasPrefix(spec.Key, "client."):
				colours = append(colours, clientColorsSection(w, cfg.Behavior, spec))
			default:
				form.Append(spec.Label, settingWidget(cfg.Behavior, spec, nil))
			}
		}
		box.Add(form)
		if len(colours) > 0 {
			box.Add(widget.NewSeparator())
			box.Add(widget.NewLabelWithStyle("Window colours", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			for _, c := range colours {
				box.Add(c)
			}
		}
		return box
	}
	note := widget.NewLabel("Only settings you change (or already in behavior.conf) are saved; " +
		"behavior.conf overrides the main config. Clear a field to remove its line.")
	note.Wrapping = fyne.TextWrapWord
	return container.NewVScroll(container.NewVBox(
		section("Look", config.LookSpecs),
		widget.NewSeparator(),
		section("Behaviour", config.BehaviorSpecs),
		widget.NewSeparator(),
		note,
	))
}
