package main

import (
	"fmt"
	"time"

	"pkg.jsn.cam/jsn/pkg/flagr/fluent"
)

func main() {
	flags := fluent.New("myapp")

	// Fluent interface with full chaining
	verbose := flags.Bool("verbose").
		Short("v").
		Default(false).
		Usage("Enable verbose output").
		Register()

	output := flags.String("output").
		Short("o").
		Default("stdout").
		Usage("Output destination").
		Register()

	count := flags.Int("count").
		Short("c").
		Default(10).
		Usage("Number of items to process").
		Register()

	timeout := flags.Duration("timeout").
		Short("t").
		Default(30 * time.Second).
		Usage("Request timeout duration").
		Register()

	rate := flags.Float64("rate").
		Short("r").
		Default(1.5).
		Usage("Processing rate").
		Register()

	// Minimal setup (no short form, uses zero values)
	debug := flags.Bool("debug").
		Usage("Enable debug mode").
		Register()

	// Only short form and usage
	help := flags.Bool("help").
		Short("h").
		Usage("Show help message").
		Register()

	// Parse command line
	flags.Parse()

	// Show help if requested
	if *help {
		flags.Usage()
		return
	}

	// Use the flags
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Verbose: %v\n", *verbose)
	fmt.Printf("  Debug: %v\n", *debug)
	fmt.Printf("  Output: %s\n", *output)
	fmt.Printf("  Count: %d\n", *count)
	fmt.Printf("  Timeout: %v\n", *timeout)
	fmt.Printf("  Rate: %.2f\n", *rate)
}

/*
Example usage:

# Short forms
go run main.go -v -o file.txt -c 42 -t 1m -r 2.5

# Long forms
go run main.go --verbose --output file.txt --count 42 --timeout 1m --rate 2.5

# Mixed
go run main.go -v --output file.txt -c 42 --debug

# Help
go run main.go -h
go run main.go --help
*/
