package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"pkg.jsn.cam/jsn/internal"
	"pkg.jsn.cam/jsn/pkg/humantype"
)

var (
	delay     = flag.Duration("delay", 3*time.Second, "delay before starting to type")
	typoRate  = flag.Int("typo-rate", 2, "percentage chance of typo per character (0-100)")
	baseDelay = flag.Int("base-delay", 30, "minimum delay between keystrokes in ms")
	variance  = flag.Int("variance", 90, "random variance added to base delay in ms")
)

func main() {
	internal.HandleStartup()

	cfg := humantype.DefaultConfig()
	cfg.TypoChance = *typoRate
	cfg.BaseDelay = *baseDelay
	cfg.DelayVariance = *variance

	var input string

	if flag.NArg() > 0 {
		input = flag.Arg(0)
	} else {
		if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Fprintln(os.Stderr, "Enter text to type, then press Cmd+D (Ctrl+D on Linux) when done:")
		}
		data, err := io.ReadAll(bufio.NewReader(os.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
			os.Exit(1)
		}
		input = string(data)
	}

	if input == "" {
		fmt.Fprintln(os.Stderr, "no input provided")
		os.Exit(1)
	}

	fmt.Printf("Will type %d characters in %v. Focus your target window...\n", len(input), *delay)
	time.Sleep(*delay)

	humantype.Type(input, cfg)
}
