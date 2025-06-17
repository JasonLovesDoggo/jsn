package flagr

import (
	"strconv"
	"time"
)

type stringValue struct{ ptr *string }

func (s *stringValue) Set(val string) error { *s.ptr = val; return nil }
func (s *stringValue) String() string       { return *s.ptr }

type boolValue struct{ ptr *bool }

func (b *boolValue) Set(val string) error {
	v, err := strconv.ParseBool(val)
	*b.ptr = v
	return err
}
func (b *boolValue) String() string   { return strconv.FormatBool(*b.ptr) }
func (b *boolValue) IsBoolFlag() bool { return true }

type intValue struct{ ptr *int }

func (i *intValue) Set(val string) error {
	v, err := strconv.Atoi(val)
	*i.ptr = v
	return err
}
func (i *intValue) String() string { return strconv.Itoa(*i.ptr) }

type float64Value struct{ ptr *float64 }

func (f *float64Value) Set(val string) error {
	v, err := strconv.ParseFloat(val, 64)
	*f.ptr = v
	return err
}
func (f *float64Value) String() string { return strconv.FormatFloat(*f.ptr, 'g', -1, 64) }

type durationValue struct{ ptr *time.Duration }

func (d *durationValue) Set(val string) error {
	v, err := time.ParseDuration(val)
	*d.ptr = v
	return err
}
func (d *durationValue) String() string { return d.ptr.String() }
