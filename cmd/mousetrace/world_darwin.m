//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#include <time.h>
#include "world.h"

uint64_t world_uptime_ns(void) { return clock_gettime_nsec_np(CLOCK_UPTIME_RAW); }
uint64_t world_awake_ns(void) { return clock_gettime_nsec_np(CLOCK_MONOTONIC_RAW); }

void world_init(void) {
    // Setting the timeout on the system-wide element sets the default for every
    // element, so a hung app cannot stall the context poller for long.
    AXUIElementRef sys = AXUIElementCreateSystemWide();
    AXUIElementSetMessagingTimeout(sys, 0.25);
    CFRelease(sys);
}

int world_accessibility_trusted(int prompt) {
    NSDictionary *opts = @{(__bridge id)kAXTrustedCheckOptionPrompt: prompt ? @YES : @NO};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts) ? 1 : 0;
}

// jsonString serializes obj to a malloc'd UTF-8 string the caller frees.
static char *jsonString(id obj) {
    if (!obj) return NULL;
    NSData *data = [NSJSONSerialization dataWithJSONObject:obj options:0 error:nil];
    if (!data) return NULL;
    char *out = malloc(data.length + 1);
    if (!out) return NULL;
    memcpy(out, data.bytes, data.length);
    out[data.length] = '\0';
    return out;
}

static id axCopy(AXUIElementRef el, CFStringRef attr) {
    CFTypeRef value = NULL;
    if (AXUIElementCopyAttributeValue(el, attr, &value) != kAXErrorSuccess) return nil;
    return (__bridge_transfer id)value;
}

static NSString *axString(AXUIElementRef el, CFStringRef attr) {
    id v = axCopy(el, attr);
    return [v isKindOfClass:NSString.class] && [v length] ? v : nil;
}

static NSDictionary *axFrame(AXUIElementRef el) {
    id pos = axCopy(el, kAXPositionAttribute);
    id size = axCopy(el, kAXSizeAttribute);
    if (!pos || !size) return nil;
    CGPoint p = CGPointZero;
    CGSize s = CGSizeZero;
    AXValueGetValue((__bridge AXValueRef)pos, kAXValueTypeCGPoint, &p);
    AXValueGetValue((__bridge AXValueRef)size, kAXValueTypeCGSize, &s);
    return @{@"x": @(p.x), @"y": @(p.y), @"w": @(s.width), @"h": @(s.height)};
}

// describe returns one accessibility element's identifying attributes.
static NSMutableDictionary *describe(AXUIElementRef el) {
    NSMutableDictionary *out = [NSMutableDictionary dictionary];
    NSString *v;
    if ((v = axString(el, kAXRoleAttribute))) out[@"role"] = v;
    if ((v = axString(el, kAXSubroleAttribute))) out[@"subrole"] = v;
    if ((v = axString(el, kAXTitleAttribute))) out[@"title"] = v;
    if ((v = axString(el, kAXDescriptionAttribute))) out[@"description"] = v;
    if ((v = axString(el, kAXIdentifierAttribute))) out[@"identifier"] = v;
    id url = axCopy(el, kAXURLAttribute); // links and web areas
    if ([url isKindOfClass:NSURL.class]) out[@"url"] = [url absoluteString];
    NSDictionary *frame = axFrame(el);
    if (frame) out[@"frame"] = frame;
    return out;
}

// frontmostPID uses Accessibility when trusted. Otherwise it falls back to the
// owner of the frontmost normal window, since NSWorkspace's frontmost app does
// not update in a process without a running main run loop.
static pid_t frontmostPID(void) {
    if (AXIsProcessTrusted()) {
        AXUIElementRef sys = AXUIElementCreateSystemWide();
        AXUIElementRef app = (__bridge_retained AXUIElementRef)axCopy(sys, kAXFocusedApplicationAttribute);
        CFRelease(sys);
        if (app) {
            pid_t pid = 0;
            AXUIElementGetPid(app, &pid);
            CFRelease(app);
            if (pid) return pid;
        }
    }
    NSArray *windows = (__bridge_transfer NSArray *)CGWindowListCopyWindowInfo(
        kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    for (NSDictionary *w in windows) {
        if ([w[(id)kCGWindowLayer] intValue] == 0) return [w[(id)kCGWindowOwnerPID] intValue];
    }
    return 0;
}

char *world_focus(void) {
    pid_t pid = frontmostPID();
    if (!pid) return NULL;

    NSMutableDictionary *out = [NSMutableDictionary dictionary];
    out[@"pid"] = @(pid);
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    if (app.localizedName) out[@"app"] = app.localizedName;
    if (app.bundleIdentifier) out[@"bundle_id"] = app.bundleIdentifier;

    if (AXIsProcessTrusted()) {
        AXUIElementRef ax = AXUIElementCreateApplication(pid);
        AXUIElementRef window = (__bridge_retained AXUIElementRef)axCopy(ax, kAXFocusedWindowAttribute);
        if (window) {
            NSMutableDictionary *w = describe(window);
            if (w.count) out[@"window"] = w;
            CFRelease(window);
        }
        AXUIElementRef el = (__bridge_retained AXUIElementRef)axCopy(ax, kAXFocusedUIElementAttribute);
        if (el) {
            NSMutableDictionary *e = describe(el);
            if (e.count) out[@"element"] = e;
            CFRelease(el);
        }
        CFRelease(ax);
    }
    return jsonString(out);
}

char *world_hover(double x, double y) {
    if (!AXIsProcessTrusted()) return NULL;
    AXUIElementRef sys = AXUIElementCreateSystemWide();
    AXUIElementRef el = NULL;
    AXError err = AXUIElementCopyElementAtPosition(sys, x, y, &el);
    CFRelease(sys);
    if (err != kAXErrorSuccess || !el) return NULL;

    NSMutableDictionary *out = describe(el);
    pid_t pid = 0;
    if (AXUIElementGetPid(el, &pid) == kAXErrorSuccess) out[@"pid"] = @(pid);
    CFRelease(el);
    return jsonString(out);
}

char *world_displays(void) {
    CGDirectDisplayID ids[32];
    uint32_t count = 0;
    if (CGGetActiveDisplayList(32, ids, &count) != kCGErrorSuccess || count == 0) return NULL;

    NSMutableArray *list = [NSMutableArray array];
    for (uint32_t i = 0; i < count; i++) {
        CGRect b = CGDisplayBounds(ids[i]);
        double scale = 1;
        CGDisplayModeRef mode = CGDisplayCopyDisplayMode(ids[i]);
        if (mode) {
            if (CGDisplayModeGetWidth(mode)) scale = (double)CGDisplayModeGetPixelWidth(mode) / CGDisplayModeGetWidth(mode);
            CGDisplayModeRelease(mode);
        }
        [list addObject:@{
            @"id": @(ids[i]),
            @"main": CGDisplayIsMain(ids[i]) ? @YES : @NO,
            @"frame": @{@"x": @(b.origin.x), @"y": @(b.origin.y), @"w": @(b.size.width), @"h": @(b.size.height)},
            @"scale": @(scale),
        }];
    }
    return jsonString(@{@"displays": list});
}

char *world_screen(void) {
    NSDictionary *d = (__bridge_transfer NSDictionary *)CGSessionCopyCurrentDictionary();
    if (!d) return NULL;
    return jsonString(@{
        @"locked": [d[@"CGSSessionScreenIsLocked"] boolValue] ? @YES : @NO,
        @"on_console": [d[(id)kCGSessionOnConsoleKey] boolValue] ? @YES : @NO,
    });
}

char *world_page(int max_nodes) {
    if (!AXIsProcessTrusted()) return NULL;
    pid_t pid = frontmostPID();
    if (!pid) return NULL;

    AXUIElementRef ax = AXUIElementCreateApplication(pid);
    id focused = axCopy(ax, kAXFocusedUIElementAttribute);
    id window = axCopy(ax, kAXFocusedWindowAttribute);
    CFRelease(ax);

    // Browsers (Safari, Chrome, Firefox, Arc...) expose the page URL on their
    // AXWebArea. Check the focused element first, then search the window.
    id found = nil;
    if (focused && [axString((__bridge AXUIElementRef)focused, kAXRoleAttribute) isEqualToString:@"AXWebArea"]) {
        found = focused;
    }
    NSMutableArray *queue = window ? [NSMutableArray arrayWithObject:window] : [NSMutableArray array];
    for (NSUInteger head = 0; !found && head < queue.count && head < (NSUInteger)max_nodes; head++) {
        AXUIElementRef el = (__bridge AXUIElementRef)queue[head];
        if ([axString(el, kAXRoleAttribute) isEqualToString:@"AXWebArea"]) {
            found = queue[head];
            break;
        }
        NSArray *kids = axCopy(el, kAXChildrenAttribute);
        if ([kids isKindOfClass:NSArray.class]) [queue addObjectsFromArray:kids];
    }
    if (!found) return NULL;

    NSMutableDictionary *out = describe((__bridge AXUIElementRef)found);
    out[@"pid"] = @(pid);
    return jsonString(out);
}

char *world_tree(int max_nodes, double budget_seconds) {
    if (!AXIsProcessTrusted()) return NULL;
    pid_t pid = frontmostPID();
    if (!pid) return NULL;

    AXUIElementRef ax = AXUIElementCreateApplication(pid);
    id root = axCopy(ax, kAXFocusedWindowAttribute);
    CFRelease(ax);
    if (!root) return NULL;

    // Breadth-first, so a truncated snapshot keeps the outer layout rather than
    // one deep branch. Each queue entry pairs an element with its output node.
    uint64_t deadline = world_uptime_ns() + (uint64_t)(budget_seconds * 1e9);
    NSMutableDictionary *rootNode = describe((__bridge AXUIElementRef)root);
    NSMutableArray *queue = [NSMutableArray arrayWithObject:@[root, rootNode]];
    NSUInteger head = 0;
    int visited = 0;
    BOOL truncated = NO;

    while (head < queue.count) {
        if (visited >= max_nodes || world_uptime_ns() > deadline) {
            truncated = YES;
            break;
        }
        NSArray *entry = queue[head++];
        visited++;
        NSArray *kids = axCopy((__bridge AXUIElementRef)entry[0], kAXChildrenAttribute);
        if (![kids isKindOfClass:NSArray.class] || kids.count == 0) continue;

        NSMutableArray *childNodes = [NSMutableArray arrayWithCapacity:kids.count];
        for (id kid in kids) {
            NSMutableDictionary *node = describe((__bridge AXUIElementRef)kid);
            [childNodes addObject:node];
            [queue addObject:@[kid, node]];
        }
        entry[1][@"children"] = childNodes;
    }

    return jsonString(@{@"pid": @(pid), @"nodes": @(visited), @"truncated": @(truncated), @"root": rootNode});
}
