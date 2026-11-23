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

	detected, err := config.DetectOutputs()
	if err != nil {
		dialog.ShowError(fmt.Errorf("detecting outputs (showing configured outputs only): %w", err), w)
	}
	cfg.MergeDetected(detected)

	tabs := container.NewAppTabs(
		container.NewTabItem("Variables", ui.MakeVariablesTab(cfg, w)),
		container.NewTabItem("Appearance", container.NewVScroll(ui.MakeAppearanceTab(cfg, w))),
		container.NewTabItem("Outputs", ui.MakeOutputsTab(cfg, w)),
		container.NewTabItem("Input", ui.MakeInputTab(cfg, inputTypes)),
		container.NewTabItem("Behavior", ui.MakeBehaviorTab(cfg, w)),
		container.NewTabItem("Bindings", ui.MakeBindingsTab(cfg, w)),
		container.NewTabItem("Rules", ui.MakeRulesTab(cfg, w)),
		container.NewTabItem("Startup", ui.MakeStartupTab(cfg, w)),
	)

	saveBtn := widget.NewButton("Save & Reload Sway", func() {
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
		if wallpaperChanged && cfg.LXQtDesktop && cfg.Wallpaper != "" {
			if err := config.SyncLXQtWallpaper(cfg.Wallpaper, cfg.WallpaperMode); err != nil {
				dialog.ShowError(fmt.Errorf("saved and reloaded, but the LXQt desktop wallpaper was not changed: %w", err), w)
				return
			}
			msg += "\nLXQt desktop wallpaper updated."
		}
		dialog.ShowInformation("Done", msg, w)
	})

	w.SetContent(container.NewBorder(nil, container.NewPadded(saveBtn), nil, nil, tabs))
	w.ShowAndRun()
}
