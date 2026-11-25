#!/usr/bin/env bash
# Install (or uninstall) Sway Settings: the binary, its desktop entry and icon.
#
#   bash install.sh               install for the current user (~/.local)
#   bash install.sh --system      install for all users (/usr/local, uses sudo)
#   bash install.sh --prefix DIR  install under DIR
#   bash install.sh --uninstall   remove what the matching install added
#
# The desktop entry uses the Settings and DesktopSettings categories, so it
# appears under Preferences in LXQt and in the settings section of other
# desktops' menus.
set -euo pipefail

APP_ID="com.github.32bytess.sway-settings-gui"
BIN_NAME="sway-settings"
ICON_NAME="sway-settings"

usage() {
	sed -n '2,7s/^# \{0,1\}//p' "$0"
}

prefix="${HOME}/.local"
uninstall=false
sudo=""
while [ $# -gt 0 ]; do
	case "$1" in
	--system)
		prefix="/usr/local"
		[ "$(id -u)" -eq 0 ] || sudo="sudo"
		;;
	--prefix)
		[ $# -ge 2 ] || { echo "error: --prefix needs a directory" >&2; exit 2; }
		prefix="$2"
		shift
		;;
	--uninstall) uninstall=true ;;
	-h | --help) usage; exit 0 ;;
	*) echo "error: unknown option: $1" >&2; usage >&2; exit 2 ;;
	esac
	shift
done

src="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bin_dir="${prefix}/bin"
apps_dir="${prefix}/share/applications"
icons_dir="${prefix}/share/icons/hicolor"
desktop_file="${apps_dir}/${APP_ID}.desktop"

# Refresh the menu and icon caches where the tools exist; menus that watch
# the directories (like LXQt's) pick up the change either way.
refresh_caches() {
	if command -v update-desktop-database >/dev/null; then
		$sudo update-desktop-database "$apps_dir"
	else
		echo "note: update-desktop-database not found; skipping the desktop database refresh"
	fi
	if command -v gtk-update-icon-cache >/dev/null && [ -f "${icons_dir}/index.theme" ]; then
		$sudo gtk-update-icon-cache --force --quiet "$icons_dir"
	fi
}

if $uninstall; then
	echo "Removing Sway Settings from ${prefix}"
	$sudo rm -fv "${bin_dir}/${BIN_NAME}" "$desktop_file" \
		"${icons_dir}/scalable/apps/${ICON_NAME}.svg"
	for size in 16 24 32 48 64 128 256; do
		$sudo rm -fv "${icons_dir}/${size}x${size}/apps/${ICON_NAME}.png"
	done
	refresh_caches
	echo "Uninstalled."
	exit 0
fi

# Build from source when Go is available; otherwise use an existing build.
if command -v go >/dev/null; then
	echo "Building ${BIN_NAME}…"
	(cd "$src" && go build -o "$BIN_NAME" .)
elif [ ! -x "${src}/${BIN_NAME}" ]; then
	echo "error: Go is not installed and no ${BIN_NAME} binary was found in ${src}" >&2
	exit 1
fi

echo "Installing into ${prefix}"
$sudo install -Dm755 "${src}/${BIN_NAME}" "${bin_dir}/${BIN_NAME}"
$sudo install -Dm644 "${src}/assets/sway-settings.svg" "${icons_dir}/scalable/apps/${ICON_NAME}.svg"

# PNG sizes for menus and panels that don't render SVG icons.
if command -v rsvg-convert >/dev/null; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	for size in 16 24 32 48 64 128 256; do
		rsvg-convert -w "$size" -h "$size" "${src}/assets/sway-settings.svg" -o "${tmp}/${size}.png"
		$sudo install -Dm644 "${tmp}/${size}.png" "${icons_dir}/${size}x${size}/apps/${ICON_NAME}.png"
	done
else
	echo "note: rsvg-convert not found; installing the SVG icon only"
fi

# Point Exec at the installed binary so the menu entry works even when
# ${bin_dir} isn't on the session's PATH.
tmp_desktop="$(mktemp)"
sed "s|^Exec=.*|Exec=${bin_dir}/${BIN_NAME}|" "${src}/assets/${APP_ID}.desktop" >"$tmp_desktop"
$sudo install -Dm644 "$tmp_desktop" "$desktop_file"
rm -f "$tmp_desktop"

if command -v desktop-file-validate >/dev/null; then
	desktop-file-validate "$desktop_file"
fi

refresh_caches
echo "Installed. Sway Settings is in your applications menu (LXQt: Preferences)."
