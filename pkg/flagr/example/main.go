package main

import (
	"fmt"
	"os"
	"time"

	"pkg.jsn.cam/jsn/pkg/flagr"
)

func main() {
	flags := flagr.New("myapp")

	// Register flags with both long and short forms
	verbose := flags.Bool("verbose", "v", false, "Enable verbose output")
	output := flags.String("output", "o", "stdout", "Output destination")
	count := flags.Int("count", "c", 10, "Number of items")
	timeout := flags.Duration("timeout", "t", 30*time.Second, "Request timeout")

	// Register flag with only long form
	debug := flags.Bool("debug", "", false, "Enable debug mode")

	// Parse command line arguments
	flags.Parse(os.Args)

	// Use the flags
	if *verbose {
		fmt.Println("Verbose mode enabled")
	}

	if *debug {
		fmt.Println("Debug mode enabled")
	}

	fmt.Printf("Output: %s\n", *output)
	fmt.Printf("Count: %d\n", *count)
	fmt.Printf("Timeout: %v\n", *timeout)
}

// Example usage:
// go run main.go -v -o file.txt --count 42 -t 1m --debug
// go run main.go --verbose --output file.txt -c 42 --timeout 1m
