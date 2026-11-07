package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestOutputIsPrivateAndUnique(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "traces")
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
		if s.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s is readable by others: %v", p, s.Mode())
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

func TestCoordinateAndTimestampRoundTrip(t *testing.T) {
	var b bytes.Buffer
	r := newRecorder(&b)
	want := event{Type: "mousemove", TimeNS: 12345678901234567, Pointer: &pointer{X: -100.125, Y: 42.875, DX: -3, DY: 2}}
	r.write(want)
	r.close()

	var got event
	if err := json.Unmarshal(gunzip(t, b.Bytes()), &got); err != nil {
		t.Fatal(err)
	}
	if got.TimeNS != want.TimeNS || *got.Pointer != *want.Pointer {
		t.Fatalf("precision lost: %+v", got)
	}
}

func TestFlushedPrefixIsReadableBeforeClose(t *testing.T) {
	var b bytes.Buffer
	r := newRecorder(&b)
	r.write(event{Type: "mousemove", TimeNS: 1})
	r.flush()
	r.write(event{Type: "mousemove", TimeNS: 2}) // buffered, never flushed

	// Simulates a crash after the first flush: no gzip trailer.
	zr, err := gzip.NewReader(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var got event
	if err := json.NewDecoder(zr).Decode(&got); err != nil || got.TimeNS != 1 {
		t.Fatalf("flushed record unreadable: %+v, %v", got, err)
	}
}

func gunzip(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
