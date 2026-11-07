// mousetrace records local macOS mouse input, and the on-screen context it
// happened in, as gzip-compressed newline-delimited JSON.
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
	"time"
)

type options struct {
	keyboard bool
	context  bool
	contextOptions
}

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
	var opts options
	dir := flag.String("dir", filepath.Join(home, ".mousetrace"), "directory for private recordings")
	duration := flag.Duration("duration", 0, "stop after this duration (0 = until Ctrl-C)")
	access := flag.Bool("request-access", false, "request Input Monitoring and Accessibility permission and exit")
	flag.BoolVar(&opts.keyboard, "keyboard-timing", true, "record keydown/keyup times, without key values")
	flag.BoolVar(&opts.context, "context", true, "record focused app, window, element, hover target, displays and lock state")
	flag.BoolVar(&opts.browser, "browser", true, "record the frontmost browser page's URL and title")
	flag.DurationVar(&opts.treeInterval, "tree-interval", 5*time.Second, "how often to snapshot the focused window's accessibility tree (0 = never)")
	flag.IntVar(&opts.treeMaxNodes, "tree-max-nodes", 3000, "maximum elements per accessibility tree snapshot")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: mousetrace [flags]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 0 || *duration < 0 || opts.treeInterval < 0 || opts.treeMaxNodes < 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *access {
		return requestAccess()
	}
	if opts.context && !accessibilityTrusted(false) {
		fmt.Fprintln(os.Stderr, "mousetrace: Accessibility not granted; recording front app, displays and lock state only (run with -request-access)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}
	return record(ctx, *dir, opts)
}

// record writes one trace: a session header, every captured event and context
// change, and an end record whose status reflects whether capture succeeded.
func record(ctx context.Context, dir string, opts options) error {
	f, err := createOutput(dir)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Recording to", f.Name(), "(Ctrl-C to stop)")

	r := newRecorder(f)
	r.write(newSessionRecord(opts))
	r.flush()
	var captureErr error
	if r.err == nil {
		captureErr = capture(ctx, r, opts)
	}
	r.write(newEndRecord(errors.Join(captureErr, r.err)))
	r.close()

	return errors.Join(captureErr, r.err, f.Sync(), f.Close())
}
