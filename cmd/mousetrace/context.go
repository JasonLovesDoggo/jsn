package main

import (
	"encoding/json"
	"time"
)

// A contextRecord captures what was on screen around a point in time: which app
// and element had focus, what the pointer was over, the display layout, and the
// lock state. These answer the questions a bare pointer trail cannot, such as
// whether movement happened over a visible window or typing happened in a text
// field.
type contextRecord struct {
	Type    string          `json:"type"`
	TimeNS  uint64          `json:"time_ns"`  // uptime clock, aligned with event time_ns
	AwakeNS uint64          `json:"awake_ns"` // counts through sleep, to detect gaps
	Data    json.RawMessage `json:"data"`
}

type contextOptions struct {
	browser      bool
	treeInterval time.Duration
	treeMaxNodes int
}

// contextKind is one pollable signal with its own polling interval. The
// accessibility tree is heavy, so it is polled far less often than focus.
type contextKind struct {
	name     string
	interval time.Duration
	poll     func() []byte // returns JSON, or nil when unavailable
}

// poller emits a context record whenever a signal's value changes, so an idle
// screen costs almost nothing while every real change is recorded. It assumes
// single-threaded use alongside the capture loop.
type poller struct {
	kinds   []contextKind
	last    map[string]string // name -> last JSON emitted
	nextDue map[string]time.Time
}

func newPoller(kinds []contextKind) *poller {
	return &poller{
		kinds:   kinds,
		last:    make(map[string]string),
		nextDue: make(map[string]time.Time),
	}
}

// tick polls whichever signals are due at now and writes a record for each one
// whose value changed. clock supplies the event-aligned timestamps.
func (p *poller) tick(now time.Time, clock func() (uptime, awake uint64), emit func(contextRecord)) {
	for _, k := range p.kinds {
		if due, ok := p.nextDue[k.name]; ok && now.Before(due) {
			continue
		}
		p.nextDue[k.name] = now.Add(k.interval)

		data := k.poll()
		if data == nil {
			continue
		}
		if string(data) == p.last[k.name] {
			continue
		}
		p.last[k.name] = string(data)
		uptime, awake := clock()
		emit(contextRecord{Type: k.name, TimeNS: uptime, AwakeNS: awake, Data: json.RawMessage(data)})
	}
}
