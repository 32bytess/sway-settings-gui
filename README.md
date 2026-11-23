# Sway Settings

A lightweight, Go-based GUI configuration tool built specifically for my sway config in [dotfiles](https://github.com/albertoodev/.dotfiles). It manages [Sway](https://swaywm.org/) and [SwayFX](https://github.com/WillPower3309/swayfx) settings using the [Fyne](https://fyne.io/) toolkit.

## Features

The settings are grouped into tabs:

- **General**: modifier key, terminal and app menu (with a program browser), default window opacity.
- **Outputs**: detected displays with resolution (real modes and refresh rates), position, scale, transform, adaptive sync and advanced options (HDR, bit depth, render time, subpixel, scale filter, tearing, enable/disable). Also sets the wallpaper, and applies it to the LXQt desktop too when that's running.
- **Input**: keyboard, touchpad and pointer settings, plus settings for individual devices.
- **Appearance**: gaps, borders and border style. On SwayFX it also covers corner radius, blur, shadows, dimming and title bar separators, with colour pickers.
- **Layers** (SwayFX): blur, shadows and corner radius for panels, launchers and notifications.
- **Behavior**: font (font browser), title bars, window colours (a colour picker for each), focus and workspace behaviour.
- **Workspaces**: names, output pinning and gaps for each workspace.
- **Bindings**: search, add, edit, remove and restore key bindings, including those in modes such as resize.
- **Rules**: `for_window` / `assign` rules, with a picker for open windows.
- **Startup**: programs to run at login or on every reload, with **Run now**.
- **Bar**: a swaybar editor (off by default).
- **Idle**: swayidle and swaylock: lock, screens off, suspend and the lock screen's look.

**Save & Reload Sway** checks the config with `sway -C` before reloading, and doesn't reload if Sway reports errors. **Revert** discards unsaved changes.

## Prerequisites

- **Sway** or **SwayFX**.
- **Go** 1.22+ (to build from source).
- **Fyne dependencies**: see the [Fyne setup guide](https://developer.fyne.io/started/) for your platform (usually development headers for Wayland/X11 and GL).

## Install

```sh
bash install.sh               # for the current user (~/.local)
bash install.sh --system      # for all users (/usr/local, uses sudo)
bash install.sh --uninstall   # remove it again (add --system if installed that way)
```

The script builds the app and installs the binary, a desktop entry and an icon. The entry uses the `Settings` and `DesktopSettings` categories, so it appears under **Preferences** in LXQt and in the settings section of other desktops' menus.

## Configuration Structure

Your main config is never rewritten wholesale. The main config is `~/.config/sway/config`, or `~/.config/lxqt/wayland/lxqt-sway.config` in an LXQt session. The app only updates the `set $mod/$term/$menu/$opacity` lines there; everything else goes in files under `~/.config/sway/config.d/`, which the main config includes:

| File | Contents |
|---|---|
| `appearance.conf` | gaps, borders, SwayFX effects |
| `outputs.conf` | displays and wallpaper |
| `input.conf` | keyboard, touchpad, pointer and device settings |
| `behavior.conf` | font, title bars, window colours, focus behaviour |
| `workspaces.conf` | workspace outputs and gaps |
| `bindings.conf` | binding overrides (`bindsym --no-warn`), removals (`unbindsym`) and additions |
| `rules.conf` | window rules added in the app |
| `startup.conf` | startup programs added in the app |
| `bar.conf` | the swaybar, when enabled |
| `idle.conf` | starts `~/.config/sway/sway-settings/idle.sh` when idle actions are enabled |
| `layers.conf` | SwayFX layer effects |

Settings are only written when you change them, so unchanged values stay in your main config.
