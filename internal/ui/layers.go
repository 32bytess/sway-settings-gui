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

// layerSection edits one namespace's effects. Clear unsets them all, which
// drops the namespace from layers.conf.
func layerSection(l *config.LayerEffect, title string) fyne.CanvasObject {
	form := container.NewVBox()
	build := func() {
		f := widget.NewForm()
		for _, spec := range config.LayerEffectSpecs {
			f.Append(spec.Label, settingWidget(l.Values, spec, nil))
		}
		form.Objects = []fyne.CanvasObject{f}
		form.Refresh()
	}
	clear := widget.NewButton("Clear", func() {
		for k := range l.Values {
			l.Values[k] = ""
		}
		build()
	})
	build()
	header := container.NewHBox(widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), clear)
	return container.NewVBox(header, form)
}

// MakeLayersTab edits SwayFX layer_effects for panels, launchers and
// notifications. surfaces are the layer surfaces sway reports now.
func MakeLayersTab(cfg *config.Config, w fyne.Window, surfaces []config.LayerSurface, detectErr error) fyne.CanvasObject {
	if !cfg.SwayFX {
		return container.NewCenter(widget.NewLabel("Layer effects need SwayFX (not detected)."))
	}
	box := container.NewVBox()
	if detectErr != nil {
		box.Add(widget.NewLabel(fmt.Sprintf("Couldn't list layer surfaces: %v", detectErr)))
	}

	shown := map[string]bool{}
	var unsupported []string
	for _, s := range surfaces {
		if shown[s.Namespace] {
			continue
		}
		if !s.SupportsEffects() {
			unsupported = append(unsupported, fmt.Sprintf("%s (%s layer)", s.Namespace, s.Layer))
			continue
		}
		shown[s.Namespace] = true
		box.Add(layerSection(cfg.Layer(s.Namespace), fmt.Sprintf("%s — %s layer", s.Namespace, s.Layer)))
		box.Add(widget.NewSeparator())
	}
	for i := range cfg.Layers {
		l := &cfg.Layers[i]
		if !shown[l.Namespace] {
			shown[l.Namespace] = true
			box.Add(layerSection(l, l.Namespace+" — not running now"))
			box.Add(widget.NewSeparator())
		}
	}

	added := container.NewVBox()
	nsEntry := widget.NewEntry()
	nsEntry.SetPlaceHolder("namespace, e.g. rofi, notifications, waybar")
	addBtn := widget.NewButton("Add", func() {
		ns := strings.TrimSpace(nsEntry.Text)
		if ns == "" {
			return
		}
		if shown[ns] {
			dialog.ShowInformation("Layer effects", ns+" is already listed.", w)
			return
		}
		shown[ns] = true
		added.Add(layerSection(cfg.Layer(ns), ns+" — not running now"))
		added.Add(widget.NewSeparator())
		nsEntry.SetText("")
	})
	box.Add(added)
	box.Add(widget.NewLabelWithStyle("Add another app", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	box.Add(container.NewBorder(nil, nil, nil, addBtn, nsEntry))

	text := "Effects apply to layer-shell surfaces such as panels, launchers and notifications, saved to layers.conf. " +
		"A namespace with nothing set is not written."
	if len(unsupported) > 0 {
		text += " Background/bottom layers can't use effects: " + strings.Join(unsupported, ", ") + "."
	}
	note := widget.NewLabel(text)
	note.Wrapping = fyne.TextWrapWord
	box.Add(widget.NewSeparator())
	box.Add(note)
	return container.NewVScroll(box)
}
