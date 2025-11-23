#!/usr/bin/env bash
set -euo pipefail

echo "[skit] Flushing DNS caches (may require password for legacy commands)..."

if command -v dscacheutil >/dev/null 2>&1; then
	dscacheutil -flushcache || true
fi

if command -v killall >/dev/null 2>&1; then
	killall -HUP mDNSResponder 2>/dev/null || true
	killall mDNSResponderHelper 2>/dev/null || true
fi

if command -v discoveryutil >/dev/null 2>&1; then
	discoveryutil mdnsflushcache 2>/dev/null || true
	discoveryutil udnsflushcaches 2>/dev/null || true
fi

if command -v sudo >/dev/null 2>&1; then
	sudo -n killall -HUP mDNSResponder 2>/dev/null || true
fi

echo "[skit] Current resolver snippet:"
scutil --dns | sed -n '1,30p'
