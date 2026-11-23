package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

// MakeBehaviorTab shows look and behaviour commands written to behavior.conf.
func MakeBehaviorTab(cfg *config.Config) fyne.CanvasObject {
	section := func(title string, specs []config.SettingSpec) fyne.CanvasObject {
		form := widget.NewForm()
		for _, spec := range specs {
			form.Append(spec.Label, settingWidget(cfg.Behavior, spec, nil))
		}
		return container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			form,
		)
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
