package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

var inputTitles = map[string]string{
	config.Keyboard: "Keyboard",
	config.Touchpad: "Touchpad",
	config.Pointer:  "Mouse / pointer",
}

// settingWidget builds the control for one setting. Choices use a Select with
// "(not set)" until something is picked, so untouched settings aren't saved;
// free-form values use an Entry validated against the spec (and an empty
// entry removes a line the app wrote earlier).
func settingWidget(values map[string]string, spec config.SettingSpec, suggestions []string) fyne.CanvasObject {
	if spec.Options != nil {
		sel := widget.NewSelect(spec.Options, nil)
		sel.PlaceHolder = "(not set)"
		if v := values[spec.Key]; v != "" {
			sel.SetSelected(v)
		}
		sel.OnChanged = func(s string) { values[spec.Key] = s }
		return sel
	}

	validator := func(s string) error {
		if s != "" && !spec.Valid(s) {
			return fmt.Errorf("invalid value")
		}
		return nil
	}
	if len(suggestions) > 0 {
		e := widget.NewSelectEntry(suggestions)
		e.SetPlaceHolder(spec.Hint)
		e.SetText(values[spec.Key])
		e.Validator = validator
		e.OnChanged = func(s string) { values[spec.Key] = s }
		return e
	}
	e := widget.NewEntry()
	e.SetPlaceHolder(spec.Hint)
	e.SetText(values[spec.Key])
	e.Validator = validator
	e.OnChanged = func(s string) { values[spec.Key] = s }
	return e
}

// MakeInputTab shows keyboard, touchpad and pointer settings. present lists
// the device types sway reports; nil means detection failed, so all are shown.
func MakeInputTab(cfg *config.Config, present map[string]bool) fyne.CanvasObject {
	box := container.NewVBox()
	// Layout suggestions are optional: without them the field is a plain
	// entry, but say why the list is missing.
	layouts, err := config.XkbLayouts()
	if err != nil {
		box.Add(widget.NewLabel(fmt.Sprintf("Layout suggestions unavailable: %v", err)))
	}
	for _, ident := range config.InputTypes {
		if present != nil && !present[ident] {
			continue
		}
		if len(box.Objects) > 0 {
			box.Add(widget.NewSeparator())
		}
		box.Add(widget.NewLabelWithStyle(inputTitles[ident], fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		form := widget.NewForm()
		for _, spec := range config.InputSpecs[ident] {
			var suggestions []string
			if spec.Key == "xkb_layout" {
				suggestions = layouts
			}
			form.Append(spec.Label, settingWidget(cfg.Input[ident], spec, suggestions))
		}
		box.Add(form)
	}
	box.Add(widget.NewSeparator())
	note := widget.NewLabel("Applies to all devices of each type. Only settings you change (or already in input.conf) are saved; " +
		"input.conf overrides the main config.")
	note.Wrapping = fyne.TextWrapWord
	box.Add(note)
	return container.NewVScroll(box)
}
