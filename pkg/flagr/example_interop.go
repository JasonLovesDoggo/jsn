// +build ignore

package main

import (
	"flag"
	"fmt"

	"pkg.jsn.cam/jsn/pkg/flagr"
)

func main() {
	// You can mix flagr and flag in the same program!
	
	// Define some flags using flagr (with short forms)
	output := flagr.String("output", "o", "stdout", "Output destination")
	verbose := flagr.Bool("verbose", "v", false, "Enable verbose output")
	count := flagr.Int("count", "c", 1, "Number of iterations")
	
	// Define some flags using the built-in flag package
	debug := flag.Bool("debug", false, "Enable debug mode")
	config := flag.String("config", "config.json", "Configuration file")
	
	// You can parse using either flagr.Parse() or flag.Parse() - they both work!
	flag.Parse() // Using flag.Parse() instead of flagr.Parse()
	// flagr.Parse() // This would also work
	
	fmt.Printf("Output: %s\n", *output)
	fmt.Printf("Verbose: %t\n", *verbose)
	fmt.Printf("Count: %d\n", *count)
	fmt.Printf("Debug: %t\n", *debug)
	fmt.Printf("Config: %s\n", *config)
	
	// Examples of usage:
	// go run example_interop.go --output file.txt --verbose --count 5 --debug --config app.json
	// go run example_interop.go -o file.txt -v -c 5 --debug --config app.json
}