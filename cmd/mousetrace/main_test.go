package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestOutputIsPrivateAndUnique(t *testing.T) {
	dir := t.TempDir() + "/traces"
	a, err := createOutput(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := createOutput(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if a.Name() == b.Name() {
		t.Fatal("reused trace filename")
	}
	for _, p := range []string{dir, a.Name(), b.Name()} {
		s, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if s.Mode().Perm()&0077 != 0 {
			t.Fatalf("public permissions on %s: %v", p, s.Mode())
		}
	}
}
func TestWriteFailurePersists(t *testing.T) {
	r := newRecorder(failingWriter{})
	r.write(event{Type: "mousemove"})
	r.flush()
	if r.err == nil {
		t.Fatal("lost write error")
	}
	first := r.err
	r.write(event{})
	r.flush()
	if r.err != first {
		t.Fatal("replaced original write error")
	}
}
func TestExactCoordinateAndTimestampRoundTrip(t *testing.T) {
	var b bytes.Buffer
	r := newRecorder(&b)
	want := event{Type: "mousemove", TimeNS: 12345678901234567, Pointer: &pointer{X: -100.125, Y: 42.875, DX: -3, DY: 2}}
	r.write(want)
	r.flush()
	var got event
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TimeNS != want.TimeNS || *got.Pointer != *want.Pointer {
		t.Fatalf("precision lost: %+v", got)
	}
}
