#!/usr/bin/env bash
set -euo pipefail

echo "[skit] Host: $(hostname)"
echo "[skit] Date: $(date)"

list_ifaces() {
	/usr/sbin/networksetup -listallhardwareports | awk '
		/Hardware Port:/ {port=$0}
		/Ethernet Address:/ {mac=$3}
		/^Device: / {dev=$2; sub("Hardware Port: ", "", port); print port "|" dev "|" mac}
	'
}

if command -v /usr/sbin/networksetup >/dev/null 2>&1; then
	echo "[skit] Interfaces:"
	while IFS="|" read -r port dev mac; do
		[[ -z "$dev" ]] && continue
		ipv4=$(ipconfig getifaddr "$dev" 2>/dev/null || echo "-")
		ipv6=$(ipconfig getifaddr "$dev" inet6 2>/dev/null || echo "-")
		printf "  %-20s %-6s IPv4:%-15s IPv6:%s\n" "$port" "$dev" "$ipv4" "$ipv6"
	done < <(list_ifaces)
else
	if command -v ifconfig >/dev/null 2>&1; then
		ifconfig
	else
		echo "[skit] networksetup or ifconfig not available."
	fi
fi
