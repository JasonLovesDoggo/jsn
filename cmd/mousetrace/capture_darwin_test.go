//go:build darwin && cgo

package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNativeCallbackRecordsMouseAndRedactsKeys(t *testing.T) {
	var b bytes.Buffer
	r := newRecorder(&b)
	activeRecorder = r
	defer func() { activeRecorder = nil }()
	recordEvent(5, 123, -12.25, 40.5, 3, -2, 0, 0, 0, 0, 0)
	recordEvent(1, 124, 10, 20, 0, 0, 0, 2, 0, 0, 0)
	recordEvent(22, 125, 10, 20, 0, 0, 0, 0, 1.5, -3, 2)
	recordEvent(10, 126, 999, 999, 0, 0, 99, 99, 0, 0, 0)
	recordEvent(11, 127, 999, 999, 0, 0, 99, 99, 0, 0, 0)
	r.flush()
	d := json.NewDecoder(&b)
	var events []event
	for d.More() {
		var e event
		if err := d.Decode(&e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	if len(events) != 5 {
		t.Fatalf("got %d events", len(events))
	}
	if events[0].Pointer.X != -12.25 || events[0].Pointer.DY != -2 {
		t.Fatal(events[0])
	}
	if *events[1].Button != 0 || *events[1].Clicks != 2 {
		t.Fatal(events[1])
	}
	if events[2].Scroll.X != 1.5 || events[2].Scroll.Y != -3 || events[2].Scroll.Z != 2 {
		t.Fatal(events[2])
	}
	for _, e := range events[3:] {
		if e.Pointer != nil || e.Button != nil || e.Scroll != nil || e.Clicks != nil {
			t.Fatalf("keyboard leaked extra fields: %+v", e)
		}
	}
}
