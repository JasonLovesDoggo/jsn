// mousetrace records local macOS mouse input as newline-delimited JSON.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mousetrace:", err)
		os.Exit(1)
	}
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := flag.String("dir", filepath.Join(home, ".mousetrace"), "directory for private JSONL recordings")
	duration := flag.Duration("duration", 0, "stop after this duration (0 = until Ctrl-C)")
	keyboard := flag.Bool("keyboard-timing", false, "also record keydown/keyup times, without key values")
	access := flag.Bool("request-access", false, "request macOS Input Monitoring permission and exit")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: mousetrace [-dir path] [-duration 30s] [-keyboard-timing] [-request-access]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 0 || *duration < 0 {
		flag.Usage()
		os.Exit(2)
	}
	if *access {
		return requestAccess()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}
	return record(ctx, *dir, *keyboard)
}

// record writes one trace: a session header, every captured event, and an end
// record whose status reflects whether capture succeeded.
func record(ctx context.Context, dir string, keyboard bool) error {
	f, err := createOutput(dir)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Recording to", f.Name(), "(Ctrl-C to stop)")

	r := newRecorder(f)
	r.write(newSessionRecord(keyboard))
	r.flush()
	var captureErr error
	if r.err == nil {
		captureErr = capture(ctx, r, keyboard)
	}
	r.write(newEndRecord(errors.Join(captureErr, r.err)))
	r.flush()

	return errors.Join(captureErr, r.err, f.Sync(), f.Close())
}
