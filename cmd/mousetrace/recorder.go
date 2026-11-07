package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"
)

const schemaVersion = 1

// sessionRecord is the first line of every trace.
type sessionRecord struct {
	Type           string `json:"type"`
	SchemaVersion  int    `json:"schema_version"`
	StartedAt      string `json:"started_at"`
	Clock          string `json:"clock"`
	Coordinates    string `json:"coordinates"`
	KeyboardTiming bool   `json:"keyboard_timing"`
}

// endRecord is the last line of a trace that shut down cleanly.
type endRecord struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	EndedAt string `json:"ended_at"`
}

func newSessionRecord(keyboard bool) sessionRecord {
	return sessionRecord{
		Type:           "session",
		SchemaVersion:  schemaVersion,
		StartedAt:      timestamp(),
		Clock:          "CGEvent timestamp: nanoseconds since system startup",
		Coordinates:    "global display points; origin at main display upper-left; x right, y down",
		KeyboardTiming: keyboard,
	}
}

func newEndRecord(err error) endRecord {
	status := "complete"
	if err != nil {
		status = "error"
	}
	return endRecord{Type: "end", Status: status, EndedAt: timestamp()}
}

func timestamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// recorder buffers JSONL records. The first write or flush error is kept and
// all later calls become no-ops, so callers can check err once.
type recorder struct {
	buf *bufio.Writer
	enc *json.Encoder
	err error
}

func newRecorder(w io.Writer) *recorder {
	buf := bufio.NewWriterSize(w, 256*1024)
	return &recorder{buf: buf, enc: json.NewEncoder(buf)}
}

func (r *recorder) write(v any) {
	if r.err == nil {
		r.err = r.enc.Encode(v)
	}
}

func (r *recorder) flush() {
	if r.err == nil {
		r.err = r.buf.Flush()
	}
}

// createOutput makes a new trace file in dir. CreateTemp creates it with mode
// 0600 and never overwrites an existing trace.
func createOutput(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.CreateTemp(dir, "trace-"+time.Now().UTC().Format("20060102T150405Z")+"-*.jsonl")
}
