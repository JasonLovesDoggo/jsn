# mousetrace

Records your exact mouse movements on macOS to a local JSONL file. Every pointer
event macOS delivers is written as is. There is no sampling, smoothing, network
access or background service. It's a single Go binary that links only Apple's
system frameworks, and it only observes input. It never changes or swallows it.

## Usage

```sh
CGO_ENABLED=1 go build -o ~/mousetrace ./cmd/mousetrace

~/mousetrace -request-access     # one-time: ask macOS for Input Monitoring
~/mousetrace                     # record until Ctrl-C
~/mousetrace -duration 30s       # record for 30 seconds
~/mousetrace -keyboard-timing    # also record key down/up times (never which key)
~/mousetrace -dir /path/to/dir   # write somewhere other than ~/.mousetrace
```

Building needs macOS, Go 1.25+, and the Command Line Tools
(`xcode-select --install`). Build on each architecture you plan to run on.

### Permissions

macOS only delivers input events to apps that have **System Settings → Privacy &
Security → Input Monitoring** turned on. Depending on how macOS attributes the
process, you grant it to the terminal that launches `mousetrace` or to the binary
itself. In the binary's case, press **+** and then **⌘⇧G** and enter its path.
Restart the terminal afterward. Without permission, `mousetrace` exits nonzero
and prints instructions.

## Output

Each run creates a new file `~/.mousetrace/trace-<UTC time>-<random>.jsonl`.
New directories get mode `0700` and files get `0600`. Existing traces are never
overwritten.

```jsonl
{"type":"session","schema_version":1,"started_at":"2026-11-07T22:43:38.89Z","clock":"CGEvent timestamp: nanoseconds since system startup","coordinates":"global display points; origin at main display upper-left; x right, y down","keyboard_timing":false}
{"type":"mousemove","time_ns":429190374325833,"pointer":{"x":549.94,"y":611.30,"delta_x":2,"delta_y":-1}}
{"type":"mousedown","time_ns":429190382654875,"pointer":{"x":549.94,"y":611.30,"delta_x":0,"delta_y":0},"button":0,"click_count":1}
{"type":"wheel","time_ns":429190395174625,"pointer":{"x":549.94,"y":611.30,"delta_x":0,"delta_y":0},"scroll":{"x":0,"y":3,"z":0}}
{"type":"end","status":"complete","ended_at":"2026-11-07T22:43:39.93Z"}
```

| Field | Meaning |
| --- | --- |
| `type` | `mousemove`, `mousedrag`, `mousedown`, `mouseup`, `wheel`; with `-keyboard-timing`, also `keydown` and `keyup` |
| `time_ns` | The event's own CGEvent timestamp: integer nanoseconds since boot. Parse losslessly, since it doesn't fit in a JS number. |
| `pointer.x`, `pointer.y` | Global position in display points, unrounded. The origin is the main display's top-left corner. Other displays can give negative values. |
| `pointer.delta_x`, `delta_y` | The event's native motion delta. It is not computed from successive positions. |
| `button` | Only on clicks: `0` is left, `1` right, `2+` other buttons |
| `click_count` | Only on clicks: `2` for a double-click, and so on |
| `scroll` | Only on `wheel` events: point deltas on the x, y and z axes |

Keyboard records hold **only `type` and `time_ns`**. They have no keycode,
character, modifiers, position, or app.

The file is flushed about every 50 ms and synced on exit. SIGINT and SIGTERM
both write the `end` record. If that record is missing, the process was killed.
If macOS disables the event tap mid-run, recording stops with
`"status":"error"` rather than quietly resuming across a gap.

### Fidelity limits

"Exact" means exactly what macOS delivers to the session event stream. The HID
driver and WindowServer may already have coalesced device reports before that
point, so this is not raw USB/HID data. Sleep and Secure Input (password fields)
also pause delivery.

## Relationship to Cloudflare Precursor

[Precursor](https://blog.cloudflare.com/introducing-precursor/) collects pointer
movement, keystroke timing (not key values), focus changes and page visibility
in the browser. Cloudflare hasn't published its payload format, and none was
reverse engineered here. `mousetrace` records the same kind of pointer and
key-timing signal at the OS level, but it is not wire-compatible. Desktop
coordinates are not browser `clientX`/`clientY`. DOM focus, visibility and
trusted-event flags are only available from inside a page.

## Development

```sh
go test -race ./cmd/mousetrace
CGO_ENABLED=0 go test ./cmd/mousetrace   # non-macOS stub build
go vet ./cmd/mousetrace
```

| File | Role |
| --- | --- |
| `main.go` | Flags, signal handling, and writing one trace |
| `recorder.go` | Buffered JSONL writer, session and end records, output file creation |
| `event.go` | Converts native CGEvent fields into trace records. Pure Go, so tests run anywhere. |
| `capture_darwin.go`, `tap_darwin.c`, `tap.h` | cgo bridge to a listen-only `CGEventTap` on a locked OS thread |
| `capture_other.go` | Stub that returns an error on non-macOS or `CGO_ENABLED=0` builds |
