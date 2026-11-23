package main

import (
	"fmt"
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"sway-settings/internal/config"
	"sway-settings/internal/ui"
)

func main() {
	a := app.NewWithID("com.github.32bytess.sway-settings-gui")
	w := a.NewWindow("Sway Settings")
	w.Resize(fyne.NewSize(620, 520))
	showSettings(a, w, 0)
	w.ShowAndRun()
}

// loadConfig reads all config files and the live sway state. Problems are
// shown to the user; the app continues with what it could load.
func loadConfig(w fyne.Window) (*config.Config, map[string]bool, []config.LayerSurface, error) {
	cfg, err := config.Load()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to load config: %w", err), w)
		cfg = config.Default()
	}

	swayFX, err := config.DetectSwayFX()
	if err != nil {
		dialog.ShowError(fmt.Errorf("detecting SwayFX (SwayFX settings disabled): %w", err), w)
	}
	cfg.SwayFX = swayFX

	lxqtDesktop, err := config.LXQtDesktopRunning()
	if err != nil {
		dialog.ShowError(fmt.Errorf("checking for the LXQt desktop: %w", err), w)
	}
	cfg.LXQtDesktop = lxqtDesktop

	inputTypes, err := config.DetectInputTypes()
	if err != nil {
		dialog.ShowError(fmt.Errorf("detecting input devices (showing all sections): %w", err), w)
	}

	layerSurfaces, layerErr := config.DetectLayerSurfaces()

	detected, err := config.DetectOutputs()
	if err != nil {
		dialog.ShowError(fmt.Errorf("detecting outputs (showing configured outputs only): %w", err), w)
	}
	cfg.MergeDetected(detected)

	return cfg, inputTypes, layerSurfaces, layerErr
}

// showSettings loads the config and (re)builds the window's content,
// opening on tab index selected.
func showSettings(a fyne.App, w fyne.Window, selected int) {
	cfg, inputTypes, layerSurfaces, layerErr := loadConfig(w)

	tabs := container.NewAppTabs(
		container.NewTabItem("Variables", ui.MakeVariablesTab(cfg, w)),
		container.NewTabItem("Appearance", container.NewVScroll(ui.MakeAppearanceTab(cfg, w))),
		container.NewTabItem("Outputs", ui.MakeOutputsTab(cfg, w)),
		container.NewTabItem("Input", ui.MakeInputTab(cfg, inputTypes)),
		container.NewTabItem("Behavior", ui.MakeBehaviorTab(cfg, w)),
		container.NewTabItem("Bindings", ui.MakeBindingsTab(cfg, w)),
		container.NewTabItem("Rules", ui.MakeRulesTab(cfg, w)),
		container.NewTabItem("Startup", ui.MakeStartupTab(cfg, w)),
		container.NewTabItem("Bar", ui.MakeBarTab(cfg, w)),
		container.NewTabItem("Idle", ui.MakeIdleTab(cfg, w)),
		container.NewTabItem("Layers", ui.MakeLayersTab(cfg, w, layerSurfaces, layerErr)),
	)
	if selected >= 0 && selected < len(tabs.Items) {
		tabs.SelectIndex(selected)
	}

	revertBtn := widget.NewButton("Revert", func() {
		dialog.ShowConfirm("Revert changes?",
			"Discard all unsaved changes and reload the settings from your config files?",
			func(ok bool) {
				if ok {
					showSettings(a, w, tabs.SelectedIndex())
				}
			}, w)
	})

	saveBtn := widget.NewButton("Save & Reload Sway", func() { save(cfg, w) })
	saveBtn.Importance = widget.HighImportance

	quitBtn := widget.NewButton("Quit", func() { a.Quit() })

	buttons := container.NewGridWithColumns(3, revertBtn, saveBtn, quitBtn)
	w.SetContent(container.NewBorder(nil, container.NewPadded(buttons), nil, nil, tabs))
}

// save writes the config, checks it with sway, reloads sway and applies
// the runtime side effects (idle, LXQt wallpaper).
func save(cfg *config.Config, w fyne.Window) {
	wallpaperChanged := cfg.WallpaperChanged()
	if err := cfg.Save(); err != nil {
		dialog.ShowError(err, w)
		return
	}
	if err := config.CheckConfig(); err != nil {
		dialog.ShowError(fmt.Errorf("saved, but Sway was NOT reloaded because the config has errors: %w", err), w)
		return
	}
	if err := exec.Command("swaymsg", "reload").Run(); err != nil {
		dialog.ShowError(fmt.Errorf("swaymsg reload: %w", err), w)
		return
	}
	msg := "Configuration saved and Sway reloaded."
	if !cfg.Idle.Enabled {
		if err := config.StopIdle(); err != nil {
			dialog.ShowError(fmt.Errorf("saved and reloaded, but stopping swayidle failed: %w", err), w)
			return
		}
	}
	if wallpaperChanged && cfg.LXQtDesktop && cfg.Wallpaper != "" {
		if err := config.SyncLXQtWallpaper(cfg.Wallpaper, cfg.WallpaperMode); err != nil {
			dialog.ShowError(fmt.Errorf("saved and reloaded, but the LXQt desktop wallpaper was not changed: %w", err), w)
			return
		}
		msg += "\nLXQt desktop wallpaper updated."
	}
	dialog.ShowInformation("Done", msg, w)
}
