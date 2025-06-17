// Package fluent provides a fluent interface for the flagr package
package fluent

import (
	"flag"
	"time"

	"pkg.jsn.cam/jsn/pkg/flagr"
)

// Flags wraps flagr.Flags with fluent interface
type Flags struct {
	*flagr.Flags
}

// Package-level instance for compatibility with built-in flag module
// Uses flagr.CommandLine which delegates to flag.CommandLine
var CommandLine = &Flags{flagr.CommandLine}

// New creates a new fluent Flags instance
func New(name string) *Flags {
	return &Flags{flagr.New(name)}
}

// String starts building a string flag
func (f *Flags) String(name string) *StringFlag {
	return &StringFlag{flags: f.Flags, long: name}
}

// Bool starts building a bool flag
func (f *Flags) Bool(name string) *BoolFlag {
	return &BoolFlag{flags: f.Flags, long: name}
}

// Int starts building an int flag
func (f *Flags) Int(name string) *IntFlag {
	return &IntFlag{flags: f.Flags, long: name}
}

// Float64 starts building a float64 flag
func (f *Flags) Float64(name string) *Float64Flag {
	return &Float64Flag{flags: f.Flags, long: name}
}

// Duration starts building a duration flag
func (f *Flags) Duration(name string) *DurationFlag {
	return &DurationFlag{flags: f.Flags, long: name}
}

// StringFlag builder
type StringFlag struct {
	flags                          *flagr.Flags
	long, short, defaultVal, usage string
}

func (sf *StringFlag) Short(s string) *StringFlag {
	sf.short = s
	return sf
}

func (sf *StringFlag) Default(d string) *StringFlag {
	sf.defaultVal = d
	return sf
}

func (sf *StringFlag) Usage(u string) *StringFlag {
	sf.usage = u
	return sf
}

func (sf *StringFlag) Register() *string {
	return sf.flags.String(sf.long, sf.short, sf.defaultVal, sf.usage)
}

// BoolFlag builder
type BoolFlag struct {
	flags              *flagr.Flags
	long, short, usage string
	defaultVal         bool
}

func (bf *BoolFlag) Short(s string) *BoolFlag {
	bf.short = s
	return bf
}

func (bf *BoolFlag) Default(d bool) *BoolFlag {
	bf.defaultVal = d
	return bf
}

func (bf *BoolFlag) Usage(u string) *BoolFlag {
	bf.usage = u
	return bf
}

func (bf *BoolFlag) Register() *bool {
	return bf.flags.Bool(bf.long, bf.short, bf.defaultVal, bf.usage)
}

// IntFlag builder
type IntFlag struct {
	flags              *flagr.Flags
	long, short, usage string
	defaultVal         int
}

func (if_ *IntFlag) Short(s string) *IntFlag {
	if_.short = s
	return if_
}

func (if_ *IntFlag) Default(d int) *IntFlag {
	if_.defaultVal = d
	return if_
}

func (if_ *IntFlag) Usage(u string) *IntFlag {
	if_.usage = u
	return if_
}

func (if_ *IntFlag) Register() *int {
	return if_.flags.Int(if_.long, if_.short, if_.defaultVal, if_.usage)
}

// Float64Flag builder
type Float64Flag struct {
	flags              *flagr.Flags
	long, short, usage string
	defaultVal         float64
}

func (ff *Float64Flag) Short(s string) *Float64Flag {
	ff.short = s
	return ff
}

func (ff *Float64Flag) Default(d float64) *Float64Flag {
	ff.defaultVal = d
	return ff
}

func (ff *Float64Flag) Usage(u string) *Float64Flag {
	ff.usage = u
	return ff
}

func (ff *Float64Flag) Register() *float64 {
	return ff.flags.Float64(ff.long, ff.short, ff.defaultVal, ff.usage)
}

// DurationFlag builder
type DurationFlag struct {
	flags              *flagr.Flags
	long, short, usage string
	defaultVal         time.Duration
}

func (df *DurationFlag) Short(s string) *DurationFlag {
	df.short = s
	return df
}

func (df *DurationFlag) Default(d time.Duration) *DurationFlag {
	df.defaultVal = d
	return df
}

func (df *DurationFlag) Usage(u string) *DurationFlag {
	df.usage = u
	return df
}

func (df *DurationFlag) Register() *time.Duration {
	return df.flags.Duration(df.long, df.short, df.defaultVal, df.usage)
}

// Var creates a custom flag with fluent interface
func (f *Flags) Var(value flag.Value, name string) *VarFlag {
	return &VarFlag{flags: f.Flags, value: value, long: name}
}

// VarFlag builder for custom flag.Value types
type VarFlag struct {
	flags              *flagr.Flags
	value              flag.Value
	long, short, usage string
}

func (vf *VarFlag) Short(s string) *VarFlag {
	vf.short = s
	return vf
}

func (vf *VarFlag) Usage(u string) *VarFlag {
	vf.usage = u
	return vf
}

func (vf *VarFlag) Register() {
	vf.flags.Var(vf.value, vf.long, vf.short, vf.usage)
}

// Package-level functions for compatibility with built-in flag module

func String(name string) *StringFlag {
	return CommandLine.String(name)
}

func Bool(name string) *BoolFlag {
	return CommandLine.Bool(name)
}

func Int(name string) *IntFlag {
	return CommandLine.Int(name)
}

func Float64(name string) *Float64Flag {
	return CommandLine.Float64(name)
}

func Duration(name string) *DurationFlag {
	return CommandLine.Duration(name)
}

func Var(value flag.Value, name string) *VarFlag {
	return CommandLine.Var(value, name)
}

func Parse() {
	CommandLine.Parse()
}

func Usage() {
	CommandLine.Usage()
}
