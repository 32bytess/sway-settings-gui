package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

func MakeAppearanceTab(cfg *config.Config) fyne.CanvasObject {
	intRow := func(label string, min, max float64, get func() int, set func(int)) fyne.CanvasObject {
		lbl := widget.NewLabel(fmt.Sprintf("%d", get()))
		s := widget.NewSlider(min, max)
		s.Step = 1
		s.SetValue(float64(get()))
		s.OnChanged = func(v float64) {
			set(int(v))
			lbl.SetText(fmt.Sprintf("%d", int(v)))
		}
		return container.NewGridWithColumns(3, widget.NewLabel(label), s, lbl)
	}

	floatRow := func(label string, min, max, step float64, get func() float64, set func(float64)) fyne.CanvasObject {
		lbl := widget.NewLabel(fmt.Sprintf("%.2f", get()))
		s := widget.NewSlider(min, max)
		s.Step = step
		s.SetValue(get())
		s.OnChanged = func(v float64) {
			set(v)
			lbl.SetText(fmt.Sprintf("%.2f", v))
		}
		return container.NewGridWithColumns(3, widget.NewLabel(label), s, lbl)
	}

	box := container.NewVBox(
		widget.NewLabelWithStyle("Gaps & Borders", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		intRow("Gaps inner (px)", 0, 50, func() int { return cfg.GapsInner }, func(v int) { cfg.GapsInner = v }),
		intRow("Gaps outer (px)", 0, 50, func() int { return cfg.GapsOuter }, func(v int) { cfg.GapsOuter = v }),
		intRow("Border width (px)", 0, 5, func() int { return cfg.BorderSize }, func(v int) { cfg.BorderSize = v }),
	)
	if !cfg.SwayFX {
		box.Add(widget.NewSeparator())
		box.Add(widget.NewLabel("Corner radius, blur and dimming need SwayFX (not detected)."))
		return box
	}

	blurCheck := widget.NewCheck("Enable blur", func(v bool) { cfg.BlurEnabled = v })
	blurCheck.SetChecked(cfg.BlurEnabled)

	box.Add(intRow("Corner radius (px)", 0, 30, func() int { return cfg.CornerRadius }, func(v int) { cfg.CornerRadius = v }))
	box.Add(widget.NewSeparator())
	box.Add(widget.NewLabelWithStyle("Blur", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	box.Add(blurCheck)
	box.Add(intRow("Blur passes", 1, 10, func() int { return cfg.BlurPasses }, func(v int) { cfg.BlurPasses = v }))
	box.Add(intRow("Blur radius", 1, 20, func() int { return cfg.BlurRadius }, func(v int) { cfg.BlurRadius = v }))
	colorEntry := func(get func() string, set func(string)) *widget.Entry {
		e := widget.NewEntry()
		e.SetPlaceHolder("#RRGGBBAA")
		e.SetText(get())
		e.Validator = func(s string) error {
			if !config.ValidHexColor(s) {
				return fmt.Errorf("use #RRGGBB or #RRGGBBAA")
			}
			return nil
		}
		e.OnChanged = set
		return e
	}
	labeled := func(label string, obj fyne.CanvasObject) fyne.CanvasObject {
		return container.NewGridWithColumns(2, widget.NewLabel(label), obj)
	}

	shadowsCheck := widget.NewCheck("Enable shadows", func(v bool) { cfg.ShadowsEnabled = v })
	shadowsCheck.SetChecked(cfg.ShadowsEnabled)
	csdCheck := widget.NewCheck("Shadows on client-side decorations", func(v bool) { cfg.ShadowsOnCSD = v })
	csdCheck.SetChecked(cfg.ShadowsOnCSD)

	box.Add(widget.NewSeparator())
	box.Add(widget.NewLabelWithStyle("Shadows", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	box.Add(shadowsCheck)
	box.Add(csdCheck)
	box.Add(intRow("Shadow blur radius", 0, 100, func() int { return cfg.ShadowBlurRadius }, func(v int) { cfg.ShadowBlurRadius = v }))
	box.Add(intRow("Shadow offset X (px)", -50, 50, func() int { return cfg.ShadowOffsetX }, func(v int) { cfg.ShadowOffsetX = v }))
	box.Add(intRow("Shadow offset Y (px)", -50, 50, func() int { return cfg.ShadowOffsetY }, func(v int) { cfg.ShadowOffsetY = v }))
	box.Add(labeled("Shadow color", colorEntry(func() string { return cfg.ShadowColor }, func(v string) { cfg.ShadowColor = v })))
	box.Add(labeled("Inactive shadow color", colorEntry(func() string { return cfg.ShadowInactiveColor }, func(v string) { cfg.ShadowInactiveColor = v })))

	box.Add(widget.NewSeparator())
	box.Add(widget.NewLabelWithStyle("Inactive Windows", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	box.Add(floatRow("Dim inactive", 0, 0.5, 0.05, func() float64 { return cfg.DimInactive }, func(v float64) { cfg.DimInactive = v }))
	return box
}
