//go:build darwin && cgo

#include <ApplicationServices/ApplicationServices.h>
#include "tap.h"
#include "_cgo_export.h"

static CFMachPortRef tap;
static CFRunLoopSourceRef source;
static int disabled;

static CGEventRef receive(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *info) {
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        // An interrupted stream is not an exact recording. Stop rather than hide a gap.
        disabled = 1;
        return event;
    }
    CGPoint p = CGEventGetLocation(event);
    trace_event e = {
        .type = (int)type,
        .timestamp = CGEventGetTimestamp(event),
        .x = p.x,
        .y = p.y,
        .delta_x = CGEventGetDoubleValueField(event, kCGMouseEventDeltaX),
        .delta_y = CGEventGetDoubleValueField(event, kCGMouseEventDeltaY),
        .button = CGEventGetIntegerValueField(event, kCGMouseEventButtonNumber),
        .clicks = CGEventGetIntegerValueField(event, kCGMouseEventClickState),
        .scroll_x = CGEventGetDoubleValueField(event, kCGScrollWheelEventPointDeltaAxis2),
        .scroll_y = CGEventGetDoubleValueField(event, kCGScrollWheelEventPointDeltaAxis1),
        .scroll_z = CGEventGetDoubleValueField(event, kCGScrollWheelEventPointDeltaAxis3),
    };
    recordEvent(&e);
    return event;
}

int trace_start(int keyboard) {
    CGEventMask mask =
        CGEventMaskBit(kCGEventMouseMoved) |
        CGEventMaskBit(kCGEventLeftMouseDragged) |
        CGEventMaskBit(kCGEventRightMouseDragged) |
        CGEventMaskBit(kCGEventOtherMouseDragged) |
        CGEventMaskBit(kCGEventLeftMouseDown) |
        CGEventMaskBit(kCGEventLeftMouseUp) |
        CGEventMaskBit(kCGEventRightMouseDown) |
        CGEventMaskBit(kCGEventRightMouseUp) |
        CGEventMaskBit(kCGEventOtherMouseDown) |
        CGEventMaskBit(kCGEventOtherMouseUp) |
        CGEventMaskBit(kCGEventScrollWheel);
    if (keyboard) mask |= CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventKeyUp);

    disabled = 0;
    tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap,
        kCGEventTapOptionListenOnly, mask, receive, NULL);
    if (!tap) return 0;
    source = CFMachPortCreateRunLoopSource(NULL, tap, 0);
    if (!source) {
        CFRelease(tap);
        tap = NULL;
        return 0;
    }
    CFRunLoopAddSource(CFRunLoopGetCurrent(), source, kCFRunLoopCommonModes);
    CGEventTapEnable(tap, true);
    return 1;
}

// trace_step runs the current thread's run loop briefly, delivering any
// pending events. It returns 0 once macOS has disabled the tap.
int trace_step(void) {
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.05, false);
    return !disabled;
}

void trace_stop(void) {
    if (source) {
        CFRunLoopRemoveSource(CFRunLoopGetCurrent(), source, kCFRunLoopCommonModes);
        CFRelease(source);
        source = NULL;
    }
    if (tap) {
        CFMachPortInvalidate(tap);
        CFRelease(tap);
        tap = NULL;
    }
}

int trace_request_access(void) { return CGRequestListenEventAccess(); }
