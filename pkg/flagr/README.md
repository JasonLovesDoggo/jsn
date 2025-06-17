# flagr

A minimal Go package that extends the standard `flag` package to support both long and short form flags.

## Features

- Drop-in extension of Go's standard `flag` package
- Support for both long (`--verbose`) and short (`-v`) forms
- All standard flag types: `string`, `bool`, `int`, `float64`, `duration`
- Custom `flag.Value` support
- Enhanced usage output showing both forms
- Minimal code footprint

## Installation

```bash
go get pkg.jsn.cam/jsn/pkg/flagr
```

## Usage

```go
package main

import (
    "fmt"
    "pkg.jsn.cam/jsn/pkg/flagr"
)

func main() {
    flags := flagr.New("myapp")
    
    // Fluent interface - chain flag definitions
    verbose := flags.Bool("verbose", "v", false, "Enable verbose output")
    output  := flags.String("output", "o", "stdout", "Output file")
    debug   := flags.Bool("debug", "", false, "Debug mode")
    
    // Simple Parse() - automatically uses os.Args[1:]
    flags.Parse()
    
    if *verbose {
        fmt.Println("Verbose mode enabled")
        fmt.Printf("Output: %s\n", *output)
    }
}
```

For more examples:
- [Simple interface example](example/main.go) - Complete example with all flag types
- [Fluent interface example](fluent/example/main.go) - Method chaining approach

## Command Line Examples

```bash
# Short forms
./myapp -v -o file.txt

# Long forms  
./myapp --verbose --output file.txt

# Mixed
./myapp -v --output file.txt --debug
```

## API

### Core Methods
- `New(name string) *Flags` - Create new flag set
- `String(long, short, default, usage string) *string`
- `Bool(long, short string, default bool, usage string) *bool`
- `Int(long, short string, default int, usage string) *int`
- `Float64(long, short string, default float64, usage string) *float64`
- `Duration(long, short string, default time.Duration, usage string) *time.Duration`
- `Var(value flag.Value, long, short, usage string)` - Custom types
- `Parse()` - Parse command-line flags from os.Args[1:]

### Inherited from flag.FlagSet
- `Parse(arguments []string) error` - Parse from custom arguments
- `Parsed() bool`
- `SetOutput(output io.Writer)`
- `Usage()`
