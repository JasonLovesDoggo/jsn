# mousetrace

Records your exact mouse movements on macOS, and the on-screen context they
happened in, to a local gzip-compressed JSONL stream. Every pointer event macOS
delivers is written as is, with no sampling or smoothing. Alongside it,
`mousetrace` records which app, window and element had focus, what the pointer
was over, the browser page URL, display layout, lock state, and periodic
accessibility-tree snapshots. It's a single Go binary that links only Apple's
system frameworks, makes no network connections, and only observes input. It
never changes or swallows it.

## Usage

```sh
CGO_ENABLED=1 go build -o ~/mousetrace ./cmd/mousetrace

~/mousetrace -request-access     # one-time: ask for Input Monitoring + Accessibility
~/mousetrace                     # record until Ctrl-C
~/mousetrace -duration 30s       # record for 30 seconds
gzcat ~/.mousetrace/trace-*.jsonl.gz | jq -c 'select(.type == "page")'
```

| Flag | Default | |
| --- | --- | --- |
| `-dir` | `~/.mousetrace` | Where recordings go |
| `-duration` | `0` | Stop after this long (`0` = until Ctrl-C) |
| `-keyboard-timing` | `true` | Record keydown/keyup times. Key values are never recorded. |
| `-context` | `true` | Record focus, hover, display and lock-state changes |
| `-browser` | `true` | Record the browser page URL and title |
| `-tree-interval` | `5s` | How often to snapshot the focused window's accessibility tree (`0` = never) |
| `-tree-max-nodes` | `3000` | Maximum elements per tree snapshot |

Building needs macOS, Go 1.25+, and the Command Line Tools
(`xcode-select --install`).

### Permissions

Grant both in **System Settings → Privacy & Security**, to the terminal that
launches `mousetrace` (or the binary itself: press **+**, then **⌘⇧G**, and
enter its path). Restart the terminal afterward.

- **Input Monitoring** is required for mouse and keyboard events. Without it,
  `mousetrace` exits with instructions.
- **Accessibility** is needed for window, element, hover, page and tree
  context. Without it, `mousetrace` warns and records only the front app,
  displays and lock state.

## Output

Each run creates a new file `~/.mousetrace/trace-<UTC time>-<random>.jsonl.gz`
(`0600`, in a `0700` directory). Existing traces are never overwritten. The
stream is gzip sync-flushed every second, so `gzcat` can read a trace while it
is still being recorded, or after a crash, up to the last flush. A trace that
ended cleanly closes with an `end` record.

```jsonl
{"type":"session","schema_version":1,"started_at":"2026-11-07T22:53:05.26Z","keyboard_timing":true,"context":true,"accessibility":true,...}
{"type":"focus","time_ns":429756312717833,"awake_ns":2320821886593500,"data":{"app":"Firefox Developer Edition","bundle_id":"org.mozilla.firefoxdeveloperedition","pid":86005,"window":{"role":"AXWindow","title":"Oasis — Personal","frame":{"x":0,"y":33,"w":1512,"h":949}},"element":{"role":"AXWebArea","description":"Oasis","frame":{...}}}}
{"type":"page","time_ns":429805592175916,"awake_ns":...,"data":{"role":"AXWebArea","url":"https://example.com/…","description":"Page title","frame":{...},"pid":86005}}
{"type":"mousemove","time_ns":429820990000000,"pointer":{"x":320.5,"y":60.25,"delta_x":2,"delta_y":-1}}
{"type":"hover","time_ns":429820997279000,"awake_ns":...,"data":{"role":"AXRadioButton","subrole":"AXTabButton","title":"Libghostty 1.0","frame":{...},"pid":86005}}
{"type":"end","status":"complete","ended_at":"2026-11-07T22:53:09.30Z"}
```

### Input events

| Field | Meaning |
| --- | --- |
| `type` | `mousemove`, `mousedrag`, `mousedown`, `mouseup`, `wheel`, `keydown`, `keyup` |
| `time_ns` | The event's own CGEvent timestamp: integer nanoseconds since boot, excluding sleep. Parse losslessly, since it doesn't fit in a JS number. |
| `pointer.x`, `pointer.y` | Global position in display points, unrounded. The origin is the main display's top-left corner. Other displays can give negative values. |
| `pointer.delta_x`, `delta_y` | The event's native motion delta. It is not computed from successive positions. |
| `button`, `click_count` | Only on clicks. `button` is `0` for left, `1` for right, `2+` for other buttons. `click_count` is `2` for a double-click. |
| `scroll` | Only on `wheel` events: point deltas on the x, y and z axes |

Keyboard records hold only `type` and `time_ns`. They have no keycode or character.

### Context records

Context records are written **only when a value changes**, so an idle screen
adds almost nothing. Each has `type`, `time_ns` (same clock as input events, so
the two interleave exactly), `awake_ns` (keeps counting through sleep), and
`data`.

| `type` | Polled every | `data` |
| --- | --- | --- |
| `focus` | 100 ms | Front app name, bundle ID and pid. Also the focused window and focused element (role, title, description, frame). |
| `hover` | 100 ms | The accessibility element under the pointer, including a link's `url` |
| `page` | 250 ms | The frontmost browser's web area: `url`, title (`description`) and the page's on-screen `frame`. It works in Safari, Chrome, Firefox and Arc. |
| `displays` | 1 s | Each display's ID, whether it is the main display, its global frame, and its backing scale |
| `screen` | 1 s | Whether the screen is `locked`, and whether the session is `on_console` (the active user) |
| `tree` | `-tree-interval` | Breadth-first snapshot of the focused window's accessibility tree, with `nodes` and `truncated` |

Element frames use the same global coordinates as `pointer`. That lets you place
any mouse event relative to a window or page: `pointer.x - page.frame.x` is the
closest a desktop tool can get to a browser's `clientX`. To detect sleep, look
for a jump in `awake_ns` that `time_ns` doesn't share.

### Fidelity limits

- "Exact" means exactly what macOS delivers to the session event stream. The
  HID driver may coalesce device reports before that point. Secure Input
  (password fields) pauses delivery.
- Context is sampled at the intervals above, not pushed by the system, so a
  change shorter than one interval can be missed. The record's `time_ns` is
  when the change was observed.
- If macOS disables the event tap mid-run, recording stops with
  `"status":"error"` rather than quietly resuming across a gap.

## Relationship to Cloudflare Precursor

[Precursor](https://blog.cloudflare.com/introducing-precursor/) collects pointer
movement, keystroke timing, focus changes and page visibility in the browser.
Its evaluators cross-reference these streams. For example, they check that
pointer activity lines up with the page being visible, and that key events fire
only while a text field has focus. `mousetrace` records the same streams at the
OS level, so the same checks can run against a trace. The `focus` element role
shows text-field focus, and `focus`/`page`/`screen` show visibility. It is not
wire-compatible: Cloudflare hasn't published its payload format, and none was
reverse engineered here.

## Development

```sh
go test -race ./cmd/mousetrace
CGO_ENABLED=0 go test ./cmd/mousetrace   # non-macOS stub build
go vet ./cmd/mousetrace
```
