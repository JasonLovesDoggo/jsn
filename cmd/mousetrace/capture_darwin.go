//go:build darwin && cgo

package main

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include "tap.h"
*/
import "C"

import (
	"context"
	"fmt"
	"runtime"
)

// The event tap and callback run on the same locked OS thread. No channel or
// sampling timer discards input; an output failure terminates the recording.
var activeRecorder *recorder

func capture(ctx context.Context, r *recorder, keyboard bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	k := 0
	if keyboard {
		k = 1
	}
	if C.trace_start(C.int(k)) == 0 {
		return fmt.Errorf("cannot create macOS event tap: grant Input Monitoring to your terminal (or mousetrace) in System Settings > Privacy & Security, then restart it")
	}
	defer C.trace_stop()
	activeRecorder = r
	defer func() { activeRecorder = nil }()
	for ctx.Err() == nil && r.err == nil {
		if C.trace_step() == 0 {
			return fmt.Errorf("macOS disabled the event tap; recording stopped because events may be missing")
		}
		r.flush()
	}
	return r.err
}

//export recordEvent
func recordEvent(kind C.int, timestamp C.uint64_t, x, y, dx, dy C.double, button, clicks C.longlong, sx, sy, sz C.double) {
	r := activeRecorder
	if r == nil || r.err != nil {
		return
	}
	e := event{Type: eventName(int(kind)), TimeNS: uint64(timestamp)}
	if int(kind) != 10 && int(kind) != 11 {
		e.Pointer = &pointer{X: float64(x), Y: float64(y), DX: float64(dx), DY: float64(dy)}
		switch int(kind) {
		case 1, 2, 3, 4, 25, 26:
			b, c := int(button), int(clicks)
			e.Button, e.Clicks = &b, &c
		case 22:
			e.Scroll = &scroll{X: float64(sx), Y: float64(sy), Z: float64(sz)}
		}
	}
	r.write(e)
}

func requestAccess() error {
	if C.trace_request_access() == 0 {
		return fmt.Errorf("Input Monitoring requested; enable access in System Settings > Privacy & Security > Input Monitoring and restart the launching app if needed")
	}
	return nil
}
