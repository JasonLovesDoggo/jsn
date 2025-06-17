// Package flagr extends Go's flag package with short form support
package flagr

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"time"
)

type Flags struct {
	*flag.FlagSet
	shorts map[string]string
}

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

func (f *Flags) Usage() {
	fmt.Fprintf(f.Output(), "Usage of %s:\n", f.Name())

	flags := make(map[string]struct{ short, usage, def string })

	f.FlagSet.VisitAll(func(fl *flag.Flag) {
		if longName, isShort := f.shorts[fl.Name]; isShort {
			if info, exists := flags[longName]; exists {
				info.short = fl.Name
				flags[longName] = info
			}
		} else {
			flags[fl.Name] = struct{ short, usage, def string }{
				usage: fl.Usage,
				def:   fl.DefValue,
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
