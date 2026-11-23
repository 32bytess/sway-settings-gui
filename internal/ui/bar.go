package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

var statusCommandSuggestions = []string{
	"while date +'%Y-%m-%d %X'; do sleep 1; done",
	"i3status",
	"i3status-rs",
}

// MakeBarTab edits the swaybar in bar.conf. It is off by default so it
// doesn't add a second panel next to a desktop's own (e.g. lxqt-panel).
func MakeBarTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	enable := widget.NewCheck("Enable a sway bar", func(v bool) { cfg.Bar.Enabled = v })
	enable.SetChecked(cfg.Bar.Enabled)

	warn := "If your desktop already has a panel (like lxqt-panel), enabling this adds a second one."
	if cfg.Bar.OtherBars > 0 {
		warn += fmt.Sprintf(" %d other bar(s) are already defined in your config files.", cfg.Bar.OtherBars)
	}
	warnLabel := widget.NewLabel(warn)
	warnLabel.Wrapping = fyne.TextWrapWord

	form := widget.NewForm()
	for _, spec := range config.BarSpecs {
		switch spec.Key {
		case "font":
			form.Append(spec.Label, fontField(w, cfg.Bar.Settings, spec))
		case "status_command":
			form.Append(spec.Label, settingWidget(cfg.Bar.Settings, spec, statusCommandSuggestions))
		default:
			form.Append(spec.Label, settingWidget(cfg.Bar.Settings, spec, nil))
		}
	}

	colours := container.NewVBox(widget.NewLabelWithStyle("Colours", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	for _, spec := range config.BarColorSpecs {
		colours.Add(barColorSection(w, cfg.Bar.Colors, spec))
	}

	note := widget.NewLabel("The bar is saved to bar.conf (id " + config.BarID + "). Unticking Enable empties that file.")
	note.Wrapping = fyne.TextWrapWord

	return container.NewVScroll(container.NewVBox(
		enable, warnLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Bar", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		form,
		widget.NewSeparator(),
		colours,
		widget.NewSeparator(),
		note,
	))
}
