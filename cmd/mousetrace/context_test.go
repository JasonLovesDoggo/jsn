package main

import (
	"testing"
	"time"
)

func TestPollerEmitsOnlyChanges(t *testing.T) {
	values := []string{`{"app":"A"}`, `{"app":"A"}`, `{"app":"B"}`, ""}
	i := 0
	p := newPoller([]contextKind{{"focus", 0, func() []byte {
		v := values[i]
		i++
		if v == "" {
			return nil // unavailable: neither emitted nor treated as a change
		}
		return []byte(v)
	}}})

	var got []string
	clock := func() (uint64, uint64) { return 10, 20 }
	for range values {
		p.tick(time.Now(), clock, func(c contextRecord) {
			if c.Type != "focus" || c.TimeNS != 10 || c.AwakeNS != 20 {
				t.Fatalf("bad record %+v", c)
			}
			got = append(got, string(c.Data))
		})
	}
	if len(got) != 2 || got[0] != `{"app":"A"}` || got[1] != `{"app":"B"}` {
		t.Fatalf("got %v", got)
	}
}

func TestPollerRespectsInterval(t *testing.T) {
	calls := 0
	p := newPoller([]contextKind{{"tree", time.Second, func() []byte { calls++; return nil }}})
	start := time.Now()
	for _, d := range []time.Duration{0, 100 * time.Millisecond, 999 * time.Millisecond, time.Second, 1500 * time.Millisecond} {
		p.tick(start.Add(d), func() (uint64, uint64) { return 0, 0 }, func(contextRecord) {})
	}
	if calls != 2 {
		t.Fatalf("polled %d times, want 2", calls)
	}
}
