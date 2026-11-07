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
	"time"
)

// flushInterval bounds how much a crash can lose. Each gzip flush costs a few
// bytes and some compression ratio, so it is not done every run-loop pass.
const flushInterval = time.Second

// activeRecorder receives events from the native callback. The event tap,
// callback and context poller all run on the same locked OS thread, so no
// channel or sampling timer sits between macOS and the file.
var activeRecorder *recorder

// lastPointer is the most recent pointer position, for hover hit-testing.
var lastPointer struct {
	x, y float64
	ok   bool
}

// capture records events until ctx is done or the recorder fails. It returns
// an error only for event tap failures; write errors stay in r.err.
func capture(ctx context.Context, r *recorder, opts options) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	k := C.int(0)
	if opts.keyboard {
		k = 1
	}
	if C.trace_start(k) == 0 {
		return errors.New("cannot create macOS event tap: grant Input Monitoring to your terminal (or mousetrace) in System Settings > Privacy & Security, then restart it")
	}
	defer C.trace_stop()

	var p *poller
	if opts.context {
		p = newPoller(contextKinds(opts.contextOptions, func() (float64, float64, bool) {
			return lastPointer.x, lastPointer.y, lastPointer.ok
		}))
	}
	emit := func(c contextRecord) { r.write(c) }

	activeRecorder = r
	defer func() { activeRecorder = nil }()
	lastFlush := time.Now()
	for ctx.Err() == nil && r.err == nil {
		if C.trace_step() == 0 {
			return errors.New("macOS disabled the event tap; recording stopped because events may be missing")
		}
		now := time.Now()
		if p != nil {
			p.tick(now, clockNow, emit)
		}
		if now.Sub(lastFlush) >= flushInterval {
			r.flush()
			lastFlush = now
		}
	}
	return nil
}

//export recordEvent
func recordEvent(e *C.trace_event) {
	r := activeRecorder
	if r == nil {
		return
	}
	ev := newEvent(rawEvent{
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
	})
	if ev.Pointer != nil {
		lastPointer.x, lastPointer.y, lastPointer.ok = ev.Pointer.X, ev.Pointer.Y, true
	}
	r.write(ev)
}

func requestAccess() error {
	inputOK := C.trace_request_access() != 0
	axOK := accessibilityTrusted(true)
	switch {
	case !inputOK && !axOK:
		return errors.New("requested Input Monitoring and Accessibility; enable both in System Settings > Privacy & Security, then restart the launching app")
	case !inputOK:
		return errors.New("requested Input Monitoring; enable it in System Settings > Privacy & Security > Input Monitoring, then restart the launching app")
	case !axOK:
		return errors.New("requested Accessibility; enable it in System Settings > Privacy & Security > Accessibility, then restart the launching app")
	}
	return nil
}
