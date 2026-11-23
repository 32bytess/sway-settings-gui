package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

var (
	transformOptions    = []string{"normal", "90", "180", "270", "flipped", "flipped-90", "flipped-180", "flipped-270"}
	adaptiveSyncOptions = []string{"on", "off"}
	scaleOptions        = []string{"1", "1.25", "1.5", "1.75", "2"}
	imageExts           = []string{".png", ".jpg", ".jpeg", ".webp", ".bmp", ".gif", ".svg"}
)

// wallpaperStartDir is the folder the wallpaper picker opens in: the folder
// of the current wallpaper, else the system backgrounds folder, else home.
func wallpaperStartDir(current string) string {
	var candidates []string
	if current != "" {
		candidates = append(candidates, filepath.Dir(current))
	}
	candidates = append(candidates, "/usr/share/backgrounds")
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, home)
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "/"
}

func makeWallpaperSection(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder("/path/to/image.png (or #RRGGBB for solid_color)")
	pathEntry.SetText(cfg.Wallpaper)
	pathEntry.OnChanged = func(s string) { cfg.Wallpaper = s }

	browse := widget.NewButton("Browse…", func() {
		pickFile(w, wallpaperStartDir(cfg.Wallpaper), imageExts, func(path string) {
			pathEntry.SetText(path)
		})
	})

	modeSel := widget.NewSelect(config.WallpaperModes, nil)
	modeSel.PlaceHolder = "(mode)"
	modeSel.SetSelected(cfg.WallpaperMode)
	modeSel.OnChanged = func(s string) { cfg.WallpaperMode = s }
	if cfg.WallpaperMode == "" {
		cfg.WallpaperMode = "fill"
		modeSel.SetSelected("fill")
	}

	return container.NewVBox(
		widget.NewLabelWithStyle("Wallpaper (all outputs)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Image", container.NewBorder(nil, nil, nil, browse, pathEntry)),
			widget.NewFormItem("Mode", modeSel),
		),
	)
}

// optionalSelect is a Select bound to a field where "" means "leave as is".
// The live value is shown as a placeholder until the user picks something,
// so untouched settings are never written.
func optionalSelect(options []string, field *string, current string) *widget.Select {
	sel := widget.NewSelect(options, nil)
	if current != "" {
		sel.PlaceHolder = "current: " + current
	} else {
		sel.PlaceHolder = "(not set)"
	}
	if *field != "" {
		sel.SetSelected(*field)
	}
	sel.OnChanged = func(s string) { *field = s }
	return sel
}

func makeOutputCard(out *config.Output) fyne.CanvasObject {
	title := out.Name
	d := out.Detected
	if d != nil {
		title = fmt.Sprintf("%s — %s %s", out.Name, d.Make, d.Model)
	} else {
		title += " (not connected)"
	}

	var resolution fyne.CanvasObject
	if d != nil && len(d.Modes) > 0 {
		modes := uniqueStrings(d.Modes)
		if out.Resolution != "" && !contains(modes, out.Resolution) {
			modes = append([]string{out.Resolution}, modes...)
		}
		resolution = optionalSelect(modes, &out.Resolution, d.CurrentMode)
	} else {
		e := widget.NewEntry()
		e.SetPlaceHolder("2560x1600@60Hz")
		e.SetText(out.Resolution)
		e.OnChanged = func(s string) { out.Resolution = s }
		resolution = e
	}

	position := widget.NewEntry()
	position.SetPlaceHolder("X,Y (not set)")
	position.SetText(out.Position)
	position.OnChanged = func(s string) { out.Position = s }

	scale := widget.NewSelectEntry(scaleOptions)
	scale.SetPlaceHolder("(not set)")
	if d != nil {
		scale.SetPlaceHolder(fmt.Sprintf("current: %g", d.Scale))
	}
	scale.SetText(out.Scale)
	scale.Validator = func(s string) error {
		if s != "" && !config.ValidScale(s) {
			return fmt.Errorf("must be a positive number")
		}
		return nil
	}
	scale.OnChanged = func(s string) { out.Scale = s }

	var curTransform, curSync string
	if d != nil {
		curTransform, curSync = d.Transform, d.AdaptiveSync
	}

	return container.NewVBox(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Resolution", resolution),
			widget.NewFormItem("Position", position),
			widget.NewFormItem("Scale", scale),
			widget.NewFormItem("Transform", optionalSelect(transformOptions, &out.Transform, curTransform)),
			widget.NewFormItem("Adaptive sync", optionalSelect(adaptiveSyncOptions, &out.AdaptiveSync, curSync)),
		),
	)
}

func MakeOutputsTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	box := container.NewVBox(makeWallpaperSection(cfg, w))
	if len(cfg.Outputs) == 0 {
		box.Add(widget.NewSeparator())
		box.Add(widget.NewLabel("No outputs detected or configured."))
	}
	for i := range cfg.Outputs {
		box.Add(widget.NewSeparator())
		box.Add(makeOutputCard(&cfg.Outputs[i]))
	}
	box.Add(widget.NewSeparator())
	box.Add(widget.NewLabel("Only settings you change (or that are already in outputs.conf) are saved."))
	return container.NewVScroll(box)
}

func uniqueStrings(in []string) []string {
	var out []string
	for _, s := range in {
		if !contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
