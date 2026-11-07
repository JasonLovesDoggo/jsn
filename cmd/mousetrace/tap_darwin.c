//go:build darwin && cgo

#include <ApplicationServices/ApplicationServices.h>
#include "tap.h"
#include "_cgo_export.h"

static CFMachPortRef tap;
static CFRunLoopSourceRef source;
static int failed;

static CGEventRef receive(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *info) {
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        // An interrupted stream is not an exact recording. Stop rather than hide a gap.
        failed = 1;
        return event;
    }
    CGPoint p = CGEventGetLocation(event);
    recordEvent((int)type, CGEventGetTimestamp(event), p.x, p.y,
        CGEventGetDoubleValueField(event, kCGMouseEventDeltaX),
        CGEventGetDoubleValueField(event, kCGMouseEventDeltaY),
        CGEventGetIntegerValueField(event, kCGMouseEventButtonNumber),
        CGEventGetIntegerValueField(event, kCGMouseEventClickState),
        CGEventGetDoubleValueField(event, kCGScrollWheelEventPointDeltaAxis2),
        CGEventGetDoubleValueField(event, kCGScrollWheelEventPointDeltaAxis1),
        CGEventGetDoubleValueField(event, kCGScrollWheelEventPointDeltaAxis3));
    return event;
}

int trace_start(int keyboard) {
    CGEventMask mask = CGEventMaskBit(kCGEventMouseMoved) |
        CGEventMaskBit(kCGEventLeftMouseDragged) | CGEventMaskBit(kCGEventRightMouseDragged) |
        CGEventMaskBit(kCGEventOtherMouseDragged) | CGEventMaskBit(kCGEventLeftMouseDown) |
        CGEventMaskBit(kCGEventLeftMouseUp) | CGEventMaskBit(kCGEventRightMouseDown) |
        CGEventMaskBit(kCGEventRightMouseUp) | CGEventMaskBit(kCGEventOtherMouseDown) |
        CGEventMaskBit(kCGEventOtherMouseUp) | CGEventMaskBit(kCGEventScrollWheel);
    if (keyboard) mask |= CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventKeyUp);
    failed = 0;
    tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap,
        kCGEventTapOptionListenOnly, mask, receive, NULL);
    if (!tap) return 0;
    source = CFMachPortCreateRunLoopSource(NULL, tap, 0);
    if (!source) { CFRelease(tap); tap = NULL; return 0; }
    CFRunLoopAddSource(CFRunLoopGetCurrent(), source, kCFRunLoopCommonModes);
    CGEventTapEnable(tap, true);
    return 1;
}

int trace_step(void) {
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.05, false);
    return !failed;
}

void trace_stop(void) {
    if (source) {
        CFRunLoopRemoveSource(CFRunLoopGetCurrent(), source, kCFRunLoopCommonModes);
        CFRelease(source); source = NULL;
    }
    if (tap) { CFMachPortInvalidate(tap); CFRelease(tap); tap = NULL; }
}

int trace_request_access(void) { return CGRequestListenEventAccess(); }
