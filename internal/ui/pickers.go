package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

// hexToColor parses #RRGGBB or #RRGGBBAA.
func hexToColor(s string) (color.NRGBA, error) {
	if !config.ValidHexColor(s) {
		return color.NRGBA{}, fmt.Errorf("%q is not #RRGGBB or #RRGGBBAA", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.NRGBA{}, err
	}
	if len(s) == 7 {
		return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}, nil
	}
	return color.NRGBA{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}, nil
}

// colorToHex formats c as #RRGGBB, or #RRGGBBAA when alpha is required or the
// colour is not fully opaque.
func colorToHex(c color.Color, alpha bool) string {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	if alpha || n.A != 0xff {
		return fmt.Sprintf("#%02X%02X%02X%02X", n.R, n.G, n.B, n.A)
	}
	return fmt.Sprintf("#%02X%02X%02X", n.R, n.G, n.B)
}

// clientDefaults are sway's built-in colours for each client.* class:
// border, background, text, indicator, child_border.
var clientDefaults = map[string][]string{
	"client.focused":          {"#4c7899", "#285577", "#ffffff", "#2e9ef4", "#285577"},
	"client.focused_inactive": {"#333333", "#5f676a", "#ffffff", "#484e50", "#5f676a"},
	"client.unfocused":        {"#333333", "#222222", "#888888", "#292d2e", "#222222"},
	"client.urgent":           {"#2f343a", "#900000", "#ffffff", "#900000", "#900000"},
	"client.placeholder":      {"#000000", "#0c0c0c", "#ffffff", "#000000", "#0c0c0c"},
}

var clientColorNames = []string{"Border", "Background", "Text", "Indicator", "Child border"}

// clientColorValues returns all five colours of a client.* value, filling
// missing ones the way sway does: the class default for unset colours, and
// the background for an omitted child_border.
func clientColorValues(value, class string) []string {
	vals := append([]string(nil), clientDefaults[class]...)
	given := strings.Fields(value)
	copy(vals, given)
	if len(given) >= 2 && len(given) < 5 {
		vals[4] = vals[1]
	}
	return vals
}

// setClientColor changes colour idx of a client.* value and returns the full
// five-colour value.
func setClientColor(value, class string, idx int, hex string) string {
	vals := clientColorValues(value, class)
	vals[idx] = hex
	return strings.Join(vals, " ")
}

// fontStyleWords are the pango style options recognised at the end of a
// font description (before the size).
var fontStyleWords = map[string]bool{
	"normal": true, "regular": true, "book": true, "italic": true, "oblique": true,
	"thin": true, "ultra-light": true, "light": true, "semi-light": true, "medium": true,
	"semi-bold": true, "semibold": true, "bold": true, "ultra-bold": true, "heavy": true,
	"ultra-heavy": true, "condensed": true, "semi-condensed": true, "expanded": true,
	"small-caps": true,
}

// parseFontDesc splits "pango:Family Style Size" into its parts.
func parseFontDesc(desc string) (family, style, size string) {
	toks := strings.Fields(strings.TrimPrefix(desc, "pango:"))
	if n := len(toks); n > 1 {
		if _, err := strconv.ParseFloat(toks[n-1], 64); err == nil {
			size, toks = toks[n-1], toks[:n-1]
		}
	} else if n == 1 {
		if _, err := strconv.ParseFloat(toks[0], 64); err == nil {
			return "", "", toks[0]
		}
	}
	i := len(toks)
	for i > 1 && fontStyleWords[strings.ToLower(toks[i-1])] {
		i--
	}
	return strings.Join(toks[:i], " "), strings.Join(toks[i:], " "), size
}

func formatFontDesc(family, style, size string) string {
	parts := []string{"pango:" + family}
	if style != "" {
		parts = append(parts, style)
	}
	if size != "" {
		parts = append(parts, size)
	}
	return strings.Join(parts, " ")
}

// colorButton shows a swatch and the hex value; clicking opens a colour
// picker. get returns the current hex value, set receives the new one.
type colorButton struct {
	swatch *canvas.Rectangle
	button *widget.Button
	get    func() string
}

func newColorButton(w fyne.Window, title string, alpha bool, get func() string, set func(string)) *colorButton {
	cb := &colorButton{get: get}
	cb.swatch = canvas.NewRectangle(color.Transparent)
	cb.swatch.SetMinSize(fyne.NewSize(28, 28))
	cb.swatch.StrokeWidth = 1
	cb.swatch.StrokeColor = color.Gray{Y: 0x80}
	cb.button = widget.NewButton("", func() {
		picker := dialog.NewColorPicker(title, "", func(c color.Color) {
			set(colorToHex(c, alpha))
			cb.refresh()
		}, w)
		picker.Advanced = true
		if c, err := hexToColor(get()); err == nil {
			picker.SetColor(c)
		}
		picker.Show()
	})
	cb.refresh()
	return cb
}

func (cb *colorButton) refresh() {
	hex := cb.get()
	c, err := hexToColor(hex)
	if err != nil {
		cb.button.SetText("invalid: " + hex)
		cb.swatch.FillColor = color.Transparent
	} else {
		cb.button.SetText(hex)
		cb.swatch.FillColor = c
	}
	cb.swatch.Refresh()
}

func (cb *colorButton) object() fyne.CanvasObject {
	return container.NewBorder(nil, nil, cb.swatch, nil, cb.button)
}

// clientColorsSection edits one client.* class with a picker per colour.
// An unset class shows sway's defaults; changing any colour writes all five.
func clientColorsSection(w fyne.Window, values map[string]string, spec config.SettingSpec) fyne.CanvasObject {
	status := widget.NewLabel("")
	var buttons []*colorButton
	refreshAll := func() {
		if values[spec.Key] == "" {
			status.SetText("(sway default)")
		} else {
			status.SetText("")
		}
		for _, b := range buttons {
			b.refresh()
		}
	}
	grid := container.NewGridWithColumns(3)
	for i, name := range clientColorNames {
		i := i
		b := newColorButton(w, spec.Label+": "+name, false,
			func() string { return clientColorValues(values[spec.Key], spec.Key)[i] },
			func(hex string) {
				values[spec.Key] = setClientColor(values[spec.Key], spec.Key, i, hex)
				refreshAll()
			})
		buttons = append(buttons, b)
		grid.Add(container.NewVBox(widget.NewLabel(name), b.object()))
	}
	reset := widget.NewButton("Reset", func() {
		values[spec.Key] = ""
		refreshAll()
	})
	refreshAll()
	header := container.NewHBox(
		widget.NewLabelWithStyle(spec.Label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		status, reset,
	)
	return container.NewVBox(header, grid)
}

// fontField is an editable font description with a Browse button that
// opens a font browser.
func fontField(w fyne.Window, values map[string]string, spec config.SettingSpec) fyne.CanvasObject {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(spec.Hint)
	entry.SetText(values[spec.Key])
	entry.Validator = func(s string) error {
		if s != "" && !spec.Valid(s) {
			return fmt.Errorf("invalid value")
		}
		return nil
	}
	entry.OnChanged = func(s string) { values[spec.Key] = s }
	browse := widget.NewButton("Browse…", func() {
		families, err := config.FontFamilies()
		if err != nil {
			dialog.ShowError(fmt.Errorf("listing fonts: %w", err), w)
			return
		}
		showFontBrowser(w, families, entry.Text, entry.SetText)
	})
	return container.NewBorder(nil, nil, nil, browse, entry)
}

var fontStyles = []string{"Regular", "Bold", "Italic", "Bold Italic", "Light", "Medium", "Semi-Bold"}

func showFontBrowser(w fyne.Window, families []string, current string, onPick func(string)) {
	family, style, size := parseFontDesc(current)
	if style == "" {
		style = "Regular"
	}
	if size == "" {
		size = "10"
	}

	filtered := families
	list := widget.NewList(
		func() int { return len(filtered) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(filtered[i]) },
	)
	result := widget.NewLabel("")
	styleSel := widget.NewSelect(fontStyles, nil)
	sizeSel := widget.NewSelectEntry([]string{"8", "9", "10", "11", "12", "13", "14", "16", "18", "20"})
	update := func() {
		st := styleSel.Selected
		if st == "Regular" {
			st = ""
		}
		if family == "" {
			result.SetText("Pick a font family")
			return
		}
		result.SetText(formatFontDesc(family, st, sizeSel.Text))
	}
	selectFamily := func() {
		list.UnselectAll()
		for i, f := range filtered {
			if f == family {
				list.Select(i)
				list.ScrollTo(i)
				return
			}
		}
	}
	list.OnSelected = func(i widget.ListItemID) {
		family = filtered[i]
		update()
	}

	search := widget.NewEntry()
	search.SetPlaceHolder("Search fonts")
	search.OnChanged = func(q string) {
		q = strings.ToLower(q)
		filtered = nil
		for _, f := range families {
			if strings.Contains(strings.ToLower(f), q) {
				filtered = append(filtered, f)
			}
		}
		list.Refresh()
		selectFamily()
	}

	styleSel.SetSelected(style)
	styleSel.OnChanged = func(string) { update() }
	sizeSel.SetText(size)
	sizeSel.Validator = func(s string) error {
		if f, err := strconv.ParseFloat(s, 64); err != nil || f <= 0 {
			return fmt.Errorf("size must be a positive number")
		}
		return nil
	}
	sizeSel.OnChanged = func(string) { update() }

	bottom := container.NewVBox(
		widget.NewForm(widget.NewFormItem("Style", styleSel), widget.NewFormItem("Size", sizeSel)),
		result,
	)
	content := container.NewBorder(search, bottom, nil, nil, list)
	d := dialog.NewCustomConfirm("Choose font", "Use font", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		if family == "" {
			dialog.ShowError(fmt.Errorf("no font family selected"), w)
			return
		}
		if err := sizeSel.Validate(); err != nil {
			dialog.ShowError(err, w)
			return
		}
		onPick(result.Text)
	}, w)
	d.Resize(fyne.NewSize(520, 560))
	d.Show()
	selectFamily()
	update()
}
