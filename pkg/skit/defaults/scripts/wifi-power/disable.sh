#!/usr/bin/env bash
set -euo pipefail

NET="${NETWORKSETUP_BIN:-/usr/sbin/networksetup}"

require_binary() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "[skit] Required command $1 not found" >&2
		exit 1
	}
}

require_binary "$NET"

wifi_device() {
	if [[ -n "${SKIT_WIFI_DEVICE:-}" ]]; then
		echo "$SKIT_WIFI_DEVICE"
		return
	}
	"$NET" -listallhardwareports | awk '
		/Hardware Port: (Wi-Fi|AirPort)/ {getline; print $2; exit}
	'
}

DEVICE="$(wifi_device)"
if [[ -z "$DEVICE" ]]; then
	echo "[skit] Unable to locate Wi-Fi device. Set SKIT_WIFI_DEVICE (e.g., en0)." >&2
	exit 1
fi

echo "[skit] Disabling Wi-Fi on ${DEVICE}"
"$NET" -setairportpower "$DEVICE" off
"$NET" -getairportpower "$DEVICE"
echo "[skit] Wi-Fi disabled"
