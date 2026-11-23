package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
)

// minutesEntry edits a minutes value where 0 means never.
func minutesEntry(v *int) *widget.Entry {
	e := widget.NewEntry()
	e.SetPlaceHolder("0 = never")
	e.SetText(strconv.Itoa(*v))
	e.Validator = func(s string) error {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 0 {
			return fmt.Errorf("enter minutes (0 = never)")
		}
		return nil
	}
	e.OnChanged = func(s string) {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			n = -1 // invalid input is rejected when saving
		}
		*v = n
	}
	return e
}

// MakeIdleTab configures swayidle (lock, screen off, suspend) and swaylock.
func MakeIdleTab(cfg *config.Config, w fyne.Window) fyne.CanvasObject {
	idle := &cfg.Idle
	enable := widget.NewCheck("Enable idle actions (swayidle)", func(v bool) { idle.Enabled = v })
	enable.SetChecked(idle.Enabled)

	box := container.NewVBox(enable)
	lxqtOn, err := config.LXQtIdleWatcherEnabled()
	if err != nil {
		box.Add(widget.NewLabel(fmt.Sprintf("Couldn't check LXQt power management: %v", err)))
	} else if lxqtOn {
		warn := widget.NewLabel("LXQt power management is also set to act when idle. Running both can " +
			"blank or lock twice; turn off its idle settings (LXQt Power Management) if you enable this.")
		warn.Wrapping = fyne.TextWrapWord
		box.Add(warn)
	}

	check := func(label string, v *bool) *widget.Check {
		c := widget.NewCheck(label, func(b bool) { *v = b })
		c.SetChecked(*v)
		return c
	}

	timeouts := widget.NewForm(
		widget.NewFormItem("Lock after (minutes)", minutesEntry(&idle.LockMinutes)),
		widget.NewFormItem("Screens off after (minutes)", minutesEntry(&idle.ScreenOffMinutes)),
		widget.NewFormItem("Suspend after (minutes)", minutesEntry(&idle.SuspendMinutes)),
	)

	color := newColorButton(w, "Lock screen colour", false,
		func() string { return "#" + idle.LockColor },
		func(hex string) { idle.LockColor = strings.TrimPrefix(hex, "#") })

	image := widget.NewEntry()
	image.SetPlaceHolder("optional image")
	image.SetText(idle.LockImage)
	image.OnChanged = func(s string) { idle.LockImage = s }
	browse := widget.NewButton("Browse…", func() {
		pickFile(w, wallpaperStartDir(idle.LockImage), config.WallpaperImageExts(), image.SetText)
	})

	lockForm := widget.NewForm(
		widget.NewFormItem("Colour", color.object()),
		widget.NewFormItem("Image", container.NewBorder(nil, nil, nil, browse, image)),
	)
	lockNow := widget.NewButton("Lock now", func() {
		if err := config.LockNow(*idle); err != nil {
			dialog.ShowError(fmt.Errorf("locking: %w", err), w)
		}
	})

	note := widget.NewLabel("Settings are kept in ~/.config/sway/sway-settings/idle.sh, started by idle.conf " +
		"on every reload. Screens turn back on when you move the mouse or press a key. " +
		"Unticking Enable stops this app's swayidle (not one you run yourself).")
	note.Wrapping = fyne.TextWrapWord

	box.Add(widget.NewSeparator())
	box.Add(widget.NewLabelWithStyle("When idle", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	box.Add(timeouts)
	box.Add(check("Lock before the system sleeps", &idle.LockBeforeSleep))
	box.Add(widget.NewSeparator())
	box.Add(widget.NewLabelWithStyle("Lock screen", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	box.Add(lockForm)
	box.Add(check("Show failed attempts", &idle.ShowFailedAttempts))
	box.Add(check("Ignore empty password", &idle.IgnoreEmptyPassword))
	box.Add(lockNow)
	box.Add(widget.NewSeparator())
	box.Add(note)
	return container.NewVScroll(box)
}
