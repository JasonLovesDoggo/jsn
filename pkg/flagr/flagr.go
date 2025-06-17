// Package flagr extends Go's flag package with short form support
package flagr

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

type Flags struct {
	*flag.FlagSet
	shorts map[string]string
}

// Package-level instance for compatibility with built-in flag module
var CommandLine = &Flags{
	FlagSet: flag.NewFlagSet("", flag.ExitOnError),
	shorts:  make(map[string]string),
}

// New creates a new Flags instance with support for both long and short form flags.
// The name parameter is used in usage output and error messages.
func New(name string) *Flags {
	return &Flags{
		FlagSet: flag.NewFlagSet(name, flag.ExitOnError),
		shorts:  make(map[string]string),
	}
}

func (f *Flags) String(long, short, defaultVal, usage string) *string {
	ptr := f.FlagSet.String(long, defaultVal, usage)
	if short != "" {
		f.FlagSet.Var(&stringValue{ptr}, short, usage)
		f.shorts[short] = long
	}
	return ptr
}

func (f *Flags) Bool(long, short string, defaultVal bool, usage string) *bool {
	ptr := f.FlagSet.Bool(long, defaultVal, usage)
	if short != "" {
		f.FlagSet.Var(&boolValue{ptr}, short, usage)
		f.shorts[short] = long
	}
	return ptr
}

func (f *Flags) Int(long, short string, defaultVal int, usage string) *int {
	ptr := f.FlagSet.Int(long, defaultVal, usage)
	if short != "" {
		f.FlagSet.Var(&intValue{ptr}, short, usage)
		f.shorts[short] = long
	}
	return ptr
}

func (f *Flags) Float64(long, short string, defaultVal float64, usage string) *float64 {
	ptr := f.FlagSet.Float64(long, defaultVal, usage)
	if short != "" {
		f.FlagSet.Var(&float64Value{ptr}, short, usage)
		f.shorts[short] = long
	}
	return ptr
}

func (f *Flags) Duration(long, short string, defaultVal time.Duration, usage string) *time.Duration {
	ptr := f.FlagSet.Duration(long, defaultVal, usage)
	if short != "" {
		f.FlagSet.Var(&durationValue{ptr}, short, usage)
		f.shorts[short] = long
	}
	return ptr
}

// Var defines a flag with the specified name, value, and usage string.
// The argument p points to a flag.Value variable in which to store the value of the flag.
// Supports both long form (--name) and optional short form (-x) variants.
func (f *Flags) Var(value flag.Value, long, short, usage string) {
	f.FlagSet.Var(value, long, usage)
	if short != "" {
		f.FlagSet.Var(value, short, usage)
		f.shorts[short] = long
	}
}

func (f *Flags) SetOutput(output io.Writer) {
	f.FlagSet.SetOutput(output)
}

// Parse parses flag definitions from os.Args[1:]. Must be called after all flags
// are defined and before flags are accessed by the program.
func (f *Flags) Parse() {
	f.FlagSet.Parse(os.Args[1:])
}

// Usage prints to standard error a usage message documenting all defined command-line flags.
// The format shows both short and long forms when available, with defaults and descriptions.
func (f *Flags) Usage() {
	fmt.Fprintf(f.Output(), "Usage of %s:\n", f.Name())

	flags := make(map[string]struct{ short, usage, def string })

	// First pass: collect all long flags
	f.FlagSet.VisitAll(func(fl *flag.Flag) {
		if _, isShort := f.shorts[fl.Name]; !isShort {
			flags[fl.Name] = struct{ short, usage, def string }{
				usage: fl.Usage,
				def:   fl.DefValue,
			}
		}
	})

	// Second pass: add short forms to existing long flags
	f.FlagSet.VisitAll(func(fl *flag.Flag) {
		if longName, isShort := f.shorts[fl.Name]; isShort {
			if info, exists := flags[longName]; exists {
				info.short = fl.Name
				flags[longName] = info
			}
		}
	})

	var names []string
	for name := range flags {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		info := flags[name]
		nameStr := "--" + name
		if info.short != "" {
			nameStr = "-" + info.short + ", " + nameStr
		}

		fmt.Fprintf(f.Output(), "  %-20s %s", nameStr, info.usage)
		if info.def != "" && info.def != "false" {
			fmt.Fprintf(f.Output(), " (default %s)", info.def)
		}
		fmt.Fprintln(f.Output())
	}
}

// Package-level functions for compatibility with built-in flag module

func String(long, short, defaultVal, usage string) *string {
	return CommandLine.String(long, short, defaultVal, usage)
}

func Bool(long, short string, defaultVal bool, usage string) *bool {
	return CommandLine.Bool(long, short, defaultVal, usage)
}

func Int(long, short string, defaultVal int, usage string) *int {
	return CommandLine.Int(long, short, defaultVal, usage)
}

func Float64(long, short string, defaultVal float64, usage string) *float64 {
	return CommandLine.Float64(long, short, defaultVal, usage)
}

func Duration(long, short string, defaultVal time.Duration, usage string) *time.Duration {
	return CommandLine.Duration(long, short, defaultVal, usage)
}

func Var(value flag.Value, long, short, usage string) {
	CommandLine.Var(value, long, short, usage)
}

func Parse() {
	CommandLine.Parse()
}

func Usage() {
	CommandLine.Usage()
}

func SetOutput(output io.Writer) {
	CommandLine.SetOutput(output)
}
