# mousetrace

Record macOS mouse input locally, with no sampling timer, smoothing, network
traffic, external runtime, or third-party dependencies. One Go binary links only
Apple's system frameworks. It observes events without modifying or swallowing them.

```sh
CGO_ENABLED=1 go build -o "$HOME/mousetrace" ./cmd/mousetrace
"$HOME/mousetrace" -request-access    # ask macOS for Input Monitoring
"$HOME/mousetrace"                    # Ctrl-C to finish
"$HOME/mousetrace" -duration 30s
"$HOME/mousetrace" -keyboard-timing   # optional keydown/up timing; never key values
```

Build on macOS with Go 1.25+ and Apple's Command Line Tools (`xcode-select --install`).
The executable needs no Go installation afterward. Build separately on Intel and
Apple Silicon for the corresponding architecture.

Grant the launching terminal (or executable, depending on how macOS attributes
it) **System Settings → Privacy & Security → Input Monitoring**, and restart it
if requested. If the tap cannot open, the program exits nonzero with instructions.
No background service is installed; recording lasts only while you run it.

## Files and fidelity

Default output: `~/.mousetrace/trace-<UTC date>-<unique suffix>.jsonl`.
Override the directory with `-dir /path`. New directories are mode 0700, files
0600; existing files are never overwritten. Existing directory permissions are
not changed. Keep recordings private: pointer activity can reveal habits.

Every line is JSON. A `session` header gives schema version, wall-clock start,
coordinate system, clock definition, and whether keyboard timing is enabled.
Input records contain:

- `type`: `mousemove`, `mousedrag`, `mousedown`, `mouseup`, `wheel`, and optionally
  `keydown` / `keyup`.
- `time_ns`: original CGEvent timestamp, integer nanoseconds since system startup.
  Preserve as an integer (JavaScript consumers should use a lossless JSON parser).
- `pointer`: unrounded global `x`, `y` in display points, with the main display's
  upper-left as origin (x right, y down). Other displays can have negative values.
  `delta_x` and `delta_y` preserve the native event's motion delta fields; these
  are not calculated by subtracting positions and are meaningful for motion events.
- Clicks include `button` (0 left, 1 right, 2+ other) and `click_count`.
- Wheel events include `scroll` point deltas on x/y/z axes.
- Keyboard records contain **only type and time**, no keycode, text, modifiers,
  cursor position, clipboard, window title, or application identity.

An `end` record marks completion or capture failure. No end record means the
process did not finish cleanly. Data is flushed after each run-loop iteration
(roughly 50 ms under normal load), synced on clean shutdown, and retained on
failure. SIGINT and SIGTERM finish the file; SIGKILL/power loss can lose buffered
data. Disk errors return nonzero. A disabled event tap stops with an error rather
than silently resuming across a gap.

“Exact” means the original delivered CGEvent timestamps and coordinates, including
sub-point fractions. macOS/device drivers may coalesce or omit physical device
reports before delivery. This is not raw USB/HID motion, and no software recorder
can guarantee that every physical movement reaches the session event stream.
System sleep, secure input, and permissions may also limit delivery.

## Relationship to Cloudflare Precursor

[Cloudflare's introduction](https://blog.cloudflare.com/introducing-precursor/)
(13 July 2026) describes pointer movement, keyboard timing/rhythm, focus changes,
and page visibility. It says keys themselves are not recorded. The injected bundle
is dynamically assembled and obfuscated; the article publishes no field schema,
encoding, sampling rate, or customer site suitable for verifying its wire format.

This tool captures the native pointer trajectory and timing needed to study the
same *kind* of movement signals (speed changes, corrections, arcs and pauses),
plus optional keyboard timing. It does **not** claim byte-for-byte Precursor
compatibility or reproduce Cloudflare's evaluators. Native desktop events cannot
supply DOM target focus, document visibility, browser viewport coordinates,
trusted-event flags, or site session metadata. Those require instrumentation in
the browser; desktop coordinates must not be treated as browser clientX/clientY.
No live Precursor payload was obtained or reverse engineered for this tool.

## Validation

```sh
go test -race ./cmd/mousetrace
go vet ./cmd/mousetrace
CGO_ENABLED=1 go build -o "$HOME/mousetrace" ./cmd/mousetrace
```

Tests cover native callback conversion, coordinate/timestamp precision, click and
scroll fields, keyboard field exclusion, private unique output, and disk failure.
For a live smoke test, grant Input Monitoring, run for 10 seconds, move and drag
across displays, click and scroll, and inspect the JSONL. Verify events have
increasing timestamps and a final `end` record with `status: complete`.

Initial validation on macOS arm64: race-enabled package tests, `go vet`, normal
build and the CGO-disabled fallback tests passed. The built Mach-O executable
links system libraries only. Live startup and `-request-access` were exercised,
but macOS denied the event tap in the test environment; physical input capture
has not yet been verified after granting Input Monitoring.
