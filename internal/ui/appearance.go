package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

func MakeAppearanceTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
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
		borderStyleRow("Window border style", &cfg.BorderStyle),
		borderStyleRow("Floating border style", &cfg.FloatingBorderStyle),
	)
	if !cfg.SwayFX {
		box.Add(widget.NewSeparator())
		box.Add(widget.NewLabel("Corner radius, blur, shadows, dimming and title bar\nseparator settings need SwayFX (not detected)."))
		return box
	}

	blurCheck := widget.NewCheck("Enable blur", func(v bool) { cfg.BlurEnabled = v })
	blurCheck.SetChecked(cfg.BlurEnabled)

	checkbox := func(label string, get func() bool, set func(bool)) *widget.Check {
		c := widget.NewCheck(label, set)
		c.SetChecked(get())
		return c
	}

	box.Add(intRow("Corner radius (px)", 0, 30, func() int { return cfg.CornerRadius }, func(v int) { cfg.CornerRadius = v }))
	box.Add(checkbox("Round corners only when there are gaps", func() bool { return cfg.SmartCornerRadius }, func(v bool) { cfg.SmartCornerRadius = v }))
	box.Add(checkbox("Separator between title bar and window", func() bool { return cfg.TitlebarSeparator }, func(v bool) { cfg.TitlebarSeparator = v }))
	box.Add(widget.NewSeparator())
	box.Add(widget.NewLabelWithStyle("Blur", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	box.Add(blurCheck)
	box.Add(intRow("Blur passes", 1, 10, func() int { return cfg.BlurPasses }, func(v int) { cfg.BlurPasses = v }))
	box.Add(intRow("Blur radius", 1, 20, func() int { return cfg.BlurRadius }, func(v int) { cfg.BlurRadius = v }))
	box.Add(checkbox("X-ray (blur only the wallpaper)", func() bool { return cfg.BlurXray }, func(v bool) { cfg.BlurXray = v }))
	box.Add(floatRow("Blur noise", 0, 1, 0.01, func() float64 { return cfg.BlurNoise }, func(v float64) { cfg.BlurNoise = v }))
	box.Add(floatRow("Blur brightness", 0, 2, 0.05, func() float64 { return cfg.BlurBrightness }, func(v float64) { cfg.BlurBrightness = v }))
	box.Add(floatRow("Blur contrast", 0, 2, 0.05, func() float64 { return cfg.BlurContrast }, func(v float64) { cfg.BlurContrast = v }))
	box.Add(floatRow("Blur saturation", 0, 2, 0.05, func() float64 { return cfg.BlurSaturation }, func(v float64) { cfg.BlurSaturation = v }))
	// SwayFX colours take an alpha channel, so the picker always writes
	// #RRGGBBAA.
	colorEntry := func(title string, get func() string, set func(string)) fyne.CanvasObject {
		return newColorButton(w, title, true, get, set).object()
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
	box.Add(labeled("Shadow color", colorEntry("Shadow color", func() string { return cfg.ShadowColor }, func(v string) { cfg.ShadowColor = v })))
	box.Add(labeled("Inactive shadow color", colorEntry("Inactive shadow color", func() string { return cfg.ShadowInactiveColor }, func(v string) { cfg.ShadowInactiveColor = v })))

	box.Add(widget.NewSeparator())
	box.Add(widget.NewLabelWithStyle("Inactive Windows", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	box.Add(floatRow("Dim inactive", 0, 1, 0.05, func() float64 { return cfg.DimInactive }, func(v float64) { cfg.DimInactive = v }))
	box.Add(labeled("Dim color", colorEntry("Dim color", func() string { return cfg.DimUnfocusedColor }, func(v string) { cfg.DimUnfocusedColor = v })))
	box.Add(labeled("Dim color (urgent)", colorEntry("Dim color (urgent)", func() string { return cfg.DimUrgentColor }, func(v string) { cfg.DimUrgentColor = v })))
	return box
}

// borderStyleRow picks normal (title bar), pixel or none. Unset keeps the
// style already in the config.
func borderStyleRow(label string, style *string) fyne.CanvasObject {
	sel := widget.NewSelect([]string{"normal", "pixel", "none"}, nil)
	sel.PlaceHolder = "(keep current)"
	if *style != "" {
		sel.SetSelected(*style)
	}
	sel.OnChanged = func(s string) { *style = s }
	return container.NewGridWithColumns(2, widget.NewLabel(label), sel)
}
