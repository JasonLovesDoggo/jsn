package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
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
	Context        bool   `json:"context"`
	Accessibility  bool   `json:"accessibility"`
}

// endRecord is the last line of a trace that shut down cleanly.
type endRecord struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	EndedAt string `json:"ended_at"`
}

func newSessionRecord(opts options) sessionRecord {
	return sessionRecord{
		Type:           "session",
		SchemaVersion:  schemaVersion,
		StartedAt:      timestamp(),
		Clock:          "time_ns: CGEvent clock, nanoseconds since boot excluding sleep; awake_ns: includes sleep",
		Coordinates:    "global display points; origin at main display upper-left; x right, y down",
		KeyboardTiming: opts.keyboard,
		Context:        opts.context,
		Accessibility:  accessibilityTrusted(false),
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

// recorder streams gzip-compressed JSONL records. The first write or flush
// error is kept and all later calls become no-ops, so callers can check err
// once.
type recorder struct {
	gz  *gzip.Writer
	buf *bufio.Writer
	enc *json.Encoder
	err error
}

func newRecorder(w io.Writer) *recorder {
	gz := gzip.NewWriter(w)
	buf := bufio.NewWriterSize(gz, 256*1024)
	return &recorder{gz: gz, buf: buf, enc: json.NewEncoder(buf)}
}

func (r *recorder) write(v any) {
	if r.err == nil {
		r.err = r.enc.Encode(v)
	}
}

// flush pushes buffered records through gzip with a sync flush, so everything
// written so far can be decompressed even if the process dies afterwards.
func (r *recorder) flush() {
	if r.err == nil {
		r.err = errors.Join(r.buf.Flush(), r.gz.Flush())
	}
}

// close flushes and writes the gzip trailer. It does not close the underlying
// writer.
func (r *recorder) close() {
	if r.err == nil {
		r.err = errors.Join(r.buf.Flush(), r.gz.Close())
	}
}

// createOutput makes a new trace file in dir. CreateTemp creates it with mode
// 0600 and never overwrites an existing trace.
func createOutput(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.CreateTemp(dir, "trace-"+time.Now().UTC().Format("20060102T150405Z")+"-*.jsonl.gz")
}
