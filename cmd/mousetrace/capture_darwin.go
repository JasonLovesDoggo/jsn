//go:build darwin && cgo

package main

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include "tap.h"
*/
import "C"

import (
	"context"
	"errors"
	"runtime"
)

// activeRecorder receives events from the native callback. The event tap and
// callback run on the same locked OS thread, so no channel or sampling timer
// sits between macOS and the file.
var activeRecorder *recorder

// capture records events until ctx is done or the recorder fails. It returns
// an error only for event tap failures; write errors stay in r.err.
func capture(ctx context.Context, r *recorder, keyboard bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	k := C.int(0)
	if keyboard {
		k = 1
	}
	if C.trace_start(k) == 0 {
		return errors.New("cannot create macOS event tap: grant Input Monitoring to your terminal (or mousetrace) in System Settings > Privacy & Security, then restart it")
	}
	defer C.trace_stop()

	activeRecorder = r
	defer func() { activeRecorder = nil }()
	for ctx.Err() == nil && r.err == nil {
		if C.trace_step() == 0 {
			return errors.New("macOS disabled the event tap; recording stopped because events may be missing")
		}
		r.flush()
	}
	return nil
}

//export recordEvent
func recordEvent(e *C.trace_event) {
	r := activeRecorder
	if r == nil {
		return
	}
	r.write(newEvent(rawEvent{
		Kind:    int(e._type),
		TimeNS:  uint64(e.timestamp),
		X:       float64(e.x),
		Y:       float64(e.y),
		DeltaX:  float64(e.delta_x),
		DeltaY:  float64(e.delta_y),
		Button:  int(e.button),
		Clicks:  int(e.clicks),
		ScrollX: float64(e.scroll_x),
		ScrollY: float64(e.scroll_y),
		ScrollZ: float64(e.scroll_z),
	}))
}

func requestAccess() error {
	if C.trace_request_access() == 0 {
		return errors.New("Input Monitoring requested; enable it in System Settings > Privacy & Security > Input Monitoring and restart the launching app if needed")
	}
	return nil
}
