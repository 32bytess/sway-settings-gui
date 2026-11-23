package ui

import (
	"fmt"
	"sort"
	"strings"

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
func MakeInputTab(cfg *config.Config, present map[string]bool, devices []config.InputDevice, devErr error) fyne.CanvasObject {
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
	box.Add(deviceSection(cfg, devices, devErr, layouts))
	box.Add(widget.NewSeparator())
	note := widget.NewLabel("Type sections apply to all devices of that type; device settings override them for one device. Only settings you change (or already in input.conf) are saved; " +
		"input.conf overrides the main config.")
	note.Wrapping = fyne.TextWrapWord
	box.Add(note)
	return container.NewVScroll(box)
}

// deviceSection edits settings for one specific device, chosen from the
// connected devices plus any configured ones that aren't connected.
func deviceSection(cfg *config.Config, devices []config.InputDevice, devErr error, layouts []string) fyne.CanvasObject {
	box := container.NewVBox(widget.NewLabelWithStyle("Individual devices", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if devErr != nil {
		box.Add(widget.NewLabel(fmt.Sprintf("Couldn't list devices: %v", devErr)))
	}

	type choice struct {
		id    string
		specs []config.SettingSpec
	}
	choices := map[string]choice{}
	var labels []string
	add := func(label string, c choice) {
		choices[label] = c
		labels = append(labels, label)
	}
	connected := map[string]bool{}
	for _, d := range devices {
		connected[d.Identifier] = true
		var kinds []string
		for _, t := range d.Types {
			kinds = append(kinds, inputTitles[t])
		}
		add(fmt.Sprintf("%s (%s)", d.Name, strings.Join(kinds, ", ")), choice{d.Identifier, config.DeviceSpecs(d.Types)})
	}
	for _, id := range sortedDeviceIDs(cfg) {
		if !connected[id] {
			add(id+" (not connected)", choice{id, config.DeviceSpecs(config.InputTypes)})
		}
	}
	if len(labels) == 0 {
		box.Add(widget.NewLabel("No devices found."))
		return box
	}

	formBox := container.NewVBox()
	var current choice
	var show func()
	show = func() {
		values := cfg.InputDevice(current.id)
		form := widget.NewForm()
		form.Append("Identifier", widget.NewLabel(current.id))
		for _, spec := range current.specs {
			var suggestions []string
			if spec.Key == "xkb_layout" {
				suggestions = layouts
			}
			form.Append(spec.Label, settingWidget(values, spec, suggestions))
		}
		clear := widget.NewButton("Clear this device's settings", func() {
			for k := range values {
				values[k] = ""
			}
			show()
		})
		formBox.Objects = []fyne.CanvasObject{form, clear}
		formBox.Refresh()
	}
	sel := widget.NewSelect(labels, func(label string) {
		current = choices[label]
		show()
	})
	sel.PlaceHolder = "Choose a device"
	box.Add(sel)
	box.Add(formBox)
	return box
}

func sortedDeviceIDs(cfg *config.Config) []string {
	var ids []string
	for id, values := range cfg.Input {
		if strings.HasPrefix(id, "type:") {
			continue
		}
		for _, v := range values {
			if v != "" {
				ids = append(ids, id)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}
