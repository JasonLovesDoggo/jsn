//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework ApplicationServices
#include <stdlib.h>
#include "world.h"
*/
import "C"

import (
	"time"
	"unsafe"
)

func clockNow() (uptime, awake uint64) {
	return uint64(C.world_uptime_ns()), uint64(C.world_awake_ns())
}

func accessibilityTrusted(prompt bool) bool {
	p := C.int(0)
	if prompt {
		p = 1
	}
	return C.world_accessibility_trusted(p) != 0
}

// takeJSON converts and frees a C JSON string.
func takeJSON(s *C.char) []byte {
	if s == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(s))
	return []byte(C.GoString(s))
}

// contextKinds lists the signals to poll. pointer reports the last recorded
// pointer position, for hit-testing what is under it.
func contextKinds(opts contextOptions, pointer func() (x, y float64, ok bool)) []contextKind {
	C.world_init()
	kinds := []contextKind{
		{"focus", 100 * time.Millisecond, func() []byte { return takeJSON(C.world_focus()) }},
		{"hover", 100 * time.Millisecond, func() []byte {
			x, y, ok := pointer()
			if !ok {
				return nil
			}
			return takeJSON(C.world_hover(C.double(x), C.double(y)))
		}},
		{"displays", time.Second, func() []byte { return takeJSON(C.world_displays()) }},
		{"screen", time.Second, func() []byte { return takeJSON(C.world_screen()) }},
	}
	if opts.browser {
		kinds = append(kinds, contextKind{"page", 250 * time.Millisecond, func() []byte {
			return takeJSON(C.world_page(400))
		}})
	}
	if opts.treeInterval > 0 {
		kinds = append(kinds, contextKind{"tree", opts.treeInterval, func() []byte {
			return takeJSON(C.world_tree(C.int(opts.treeMaxNodes), 0.5))
		}})
	}
	return kinds
}
