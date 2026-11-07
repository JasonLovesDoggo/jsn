package main

import "testing"

func TestNewEventMouseMove(t *testing.T) {
	e := newEvent(rawEvent{Kind: cgMouseMoved, TimeNS: 123, X: -12.25, Y: 40.5, DeltaX: 3, DeltaY: -2})
	if e.Type != "mousemove" || e.TimeNS != 123 {
		t.Fatalf("got %+v", e)
	}
	if *e.Pointer != (pointer{X: -12.25, Y: 40.5, DX: 3, DY: -2}) {
		t.Fatalf("pointer = %+v", *e.Pointer)
	}
	if e.Button != nil || e.Clicks != nil || e.Scroll != nil {
		t.Fatalf("move has click or scroll fields: %+v", e)
	}
}

func TestNewEventClick(t *testing.T) {
	e := newEvent(rawEvent{Kind: cgRightMouseDown, X: 10, Y: 20, Button: 1, Clicks: 2})
	if e.Type != "mousedown" || *e.Button != 1 || *e.Clicks != 2 {
		t.Fatalf("got %+v", e)
	}
}

func TestNewEventScroll(t *testing.T) {
	e := newEvent(rawEvent{Kind: cgScrollWheel, ScrollX: 1.5, ScrollY: -3, ScrollZ: 2})
	if e.Type != "wheel" || *e.Scroll != (scroll{X: 1.5, Y: -3, Z: 2}) {
		t.Fatalf("got %+v", e)
	}
}

func TestNewEventKeyboardKeepsOnlyTypeAndTime(t *testing.T) {
	for _, kind := range []int{cgKeyDown, cgKeyUp} {
		e := newEvent(rawEvent{Kind: kind, TimeNS: 7, X: 999, Y: 999, Button: 99, Clicks: 99})
		if e != (event{Type: eventName(kind), TimeNS: 7}) {
			t.Fatalf("keyboard event leaked fields: %+v", e)
		}
	}
}
