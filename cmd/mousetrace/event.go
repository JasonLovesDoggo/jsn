package main

// CGEventType values from <CoreGraphics/CGEventTypes.h>. They are mirrored
// here so event conversion can be built and tested without cgo.
const (
	cgLeftMouseDown     = 1
	cgLeftMouseUp       = 2
	cgRightMouseDown    = 3
	cgRightMouseUp      = 4
	cgMouseMoved        = 5
	cgLeftMouseDragged  = 6
	cgRightMouseDragged = 7
	cgKeyDown           = 10
	cgKeyUp             = 11
	cgScrollWheel       = 22
	cgOtherMouseDown    = 25
	cgOtherMouseUp      = 26
	cgOtherMouseDragged = 27
)

// rawEvent is the subset of CGEvent fields read by the native tap.
type rawEvent struct {
	Kind           int
	TimeNS         uint64
	X, Y           float64
	DeltaX, DeltaY float64
	Button, Clicks int
	ScrollX        float64
	ScrollY        float64
	ScrollZ        float64
}

type pointer struct {
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	DX float64 `json:"delta_x"`
	DY float64 `json:"delta_y"`
}

type scroll struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// event is one input record in the JSONL trace.
type event struct {
	Type    string   `json:"type"`
	TimeNS  uint64   `json:"time_ns"`
	Pointer *pointer `json:"pointer,omitempty"`
	Button  *int     `json:"button,omitempty"`
	Clicks  *int     `json:"click_count,omitempty"`
	Scroll  *scroll  `json:"scroll,omitempty"`
}

// newEvent converts a native event into a trace record. Keyboard events keep
// only their type and timestamp; every other field is dropped.
func newEvent(r rawEvent) event {
	e := event{Type: eventName(r.Kind), TimeNS: r.TimeNS}
	if r.Kind == cgKeyDown || r.Kind == cgKeyUp {
		return e
	}
	e.Pointer = &pointer{X: r.X, Y: r.Y, DX: r.DeltaX, DY: r.DeltaY}
	switch r.Kind {
	case cgLeftMouseDown, cgLeftMouseUp, cgRightMouseDown, cgRightMouseUp, cgOtherMouseDown, cgOtherMouseUp:
		e.Button, e.Clicks = &r.Button, &r.Clicks
	case cgScrollWheel:
		e.Scroll = &scroll{X: r.ScrollX, Y: r.ScrollY, Z: r.ScrollZ}
	}
	return e
}

func eventName(kind int) string {
	switch kind {
	case cgLeftMouseDown, cgRightMouseDown, cgOtherMouseDown:
		return "mousedown"
	case cgLeftMouseUp, cgRightMouseUp, cgOtherMouseUp:
		return "mouseup"
	case cgMouseMoved:
		return "mousemove"
	case cgLeftMouseDragged, cgRightMouseDragged, cgOtherMouseDragged:
		return "mousedrag"
	case cgKeyDown:
		return "keydown"
	case cgKeyUp:
		return "keyup"
	case cgScrollWheel:
		return "wheel"
	default:
		return "unknown"
	}
}
