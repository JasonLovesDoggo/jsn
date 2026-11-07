// mousetrace records local macOS input events as newline-delimited JSON.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type pointer struct {
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	DX float64 `json:"delta_x"`
	DY float64 `json:"delta_y"`
}
type scroll struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
type event struct {
	Type    string   `json:"type"`
	TimeNS  uint64   `json:"time_ns"`
	Pointer *pointer `json:"pointer,omitempty"`
	Button  *int     `json:"button,omitempty"`
	Clicks  *int     `json:"click_count,omitempty"`
	Scroll  *scroll  `json:"scroll,omitempty"`
}
type recorder struct {
	buffer  *bufio.Writer
	encoder *json.Encoder
	err     error
}

func newRecorder(w io.Writer) *recorder {
	b := bufio.NewWriterSize(w, 256*1024)
	return &recorder{buffer: b, encoder: json.NewEncoder(b)}
}
func (r *recorder) write(v any) {
	if r.err == nil {
		r.err = r.encoder.Encode(v)
	}
}
func (r *recorder) flush() {
	if r.err == nil {
		r.err = r.buffer.Flush()
	}
}
func eventName(k int) string {
	switch k {
	case 1, 3, 25:
		return "mousedown"
	case 2, 4, 26:
		return "mouseup"
	case 5:
		return "mousemove"
	case 6, 7, 27:
		return "mousedrag"
	case 10:
		return "keydown"
	case 11:
		return "keyup"
	case 22:
		return "wheel"
	default:
		return "unknown"
	}
}

func createOutput(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(dir, "trace-"+time.Now().UTC().Format("20060102T150405Z")+"-*.jsonl")
	return f, err // CreateTemp creates mode 0600 and never overwrites an existing trace.
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := flag.String("dir", filepath.Join(home, ".mousetrace"), "directory for private JSONL recordings")
	duration := flag.Duration("duration", 0, "stop after this duration (0 = until Ctrl-C)")
	keyboard := flag.Bool("keyboard-timing", false, "also record keydown/up times, without key values")
	access := flag.Bool("request-access", false, "request macOS Input Monitoring permission and exit")
	flag.Parse()
	if *access {
		return requestAccess()
	}
	if *duration < 0 || flag.NArg() != 0 {
		return fmt.Errorf("usage: mousetrace [-dir path] [-duration 30s] [-keyboard-timing]")
	}
	f, err := createOutput(*dir)
	if err != nil {
		return err
	}
	r := newRecorder(f)
	r.write(struct {
		Type        string `json:"type"`
		Version     int    `json:"schema_version"`
		Started     string `json:"started_at"`
		Clock       string `json:"clock"`
		Coordinates string `json:"coordinates"`
		Keyboard    bool   `json:"keyboard_timing"`
	}{"session", 1, time.Now().UTC().Format(time.RFC3339Nano), "CGEvent timestamp: nanoseconds since system startup", "global display points; origin at main display upper-left; x right, y down", *keyboard})
	r.flush()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}
	fmt.Fprintln(os.Stderr, "Recording to", f.Name(), "(Ctrl-C to stop)")
	if r.err != nil {
		err = r.err
	} else {
		err = capture(ctx, r, *keyboard)
	}
	status := "complete"
	if err != nil {
		status = "error"
	}
	r.write(struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		Ended  string `json:"ended_at"`
	}{"end", status, time.Now().UTC().Format(time.RFC3339Nano)})
	r.flush()
	if syncErr := f.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = r.err
	}
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mousetrace:", err)
		os.Exit(1)
	}
}
