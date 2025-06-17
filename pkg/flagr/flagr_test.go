package flagr

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestStringFlag(t *testing.T) {
	flags := New("test")
	output := flags.String("output", "o", "default.txt", "Output file")

	// Test long form
	flags.Parse([]string{"--output", "long.txt"})
	if *output != "long.txt" {
		t.Errorf("Expected 'long.txt', got '%s'", *output)
	}

	// Test short form
	flags = New("test")
	output = flags.String("output", "o", "default.txt", "Output file")
	flags.Parse([]string{"-o", "short.txt"})
	if *output != "short.txt" {
		t.Errorf("Expected 'short.txt', got '%s'", *output)
	}
}

func TestBoolFlag(t *testing.T) {
	flags := New("test")
	verbose := flags.Bool("verbose", "v", false, "Verbose output")

	flags.Parse([]string{"-v"})
	if !*verbose {
		t.Error("Expected verbose to be true")
	}
}

func TestIntFlag(t *testing.T) {
	flags := New("test")
	count := flags.Int("count", "c", 10, "Count value")

	flags.Parse([]string{"--count", "42"})
	if *count != 42 {
		t.Errorf("Expected 42, got %d", *count)
	}
}

func TestFloat64Flag(t *testing.T) {
	flags := New("test")
	rate := flags.Float64("rate", "r", 1.5, "Rate value")

	flags.Parse([]string{"-r", "3.14"})
	if *rate != 3.14 {
		t.Errorf("Expected 3.14, got %f", *rate)
	}
}

func TestDurationFlag(t *testing.T) {
	flags := New("test")
	timeout := flags.Duration("timeout", "t", time.Second, "Timeout duration")

	flags.Parse([]string{"--timeout", "30s"})
	if *timeout != 30*time.Second {
		t.Errorf("Expected 30s, got %v", *timeout)
	}
}

func TestNoShortForm(t *testing.T) {
	flags := New("test")
	debug := flags.Bool("debug", "", false, "Debug mode")

	flags.Parse([]string{"--debug"})
	if !*debug {
		t.Error("Expected debug to be true")
	}
}

func TestUsage(t *testing.T) {
	flags := New("test")
	var buf bytes.Buffer
	flags.SetOutput(&buf)

	flags.String("output", "o", "stdout", "Output file")
	flags.Bool("verbose", "v", false, "Verbose output")
	flags.Bool("debug", "", false, "Debug mode")

	flags.Usage()

	output := buf.String()
	if !strings.Contains(output, "-o, --output") {
		t.Error("Usage should show both short and long forms")
	}
	if !strings.Contains(output, "--debug") && !strings.Contains(output, "-d, --debug") {
		t.Error("Usage should show long form for flags without short form")
	}
}

func TestBothFormsSetSameValue(t *testing.T) {
	flags := New("test")
	output := flags.String("output", "o", "default", "Output file")

	// Set via long form
	flags.Parse([]string{"--output", "test1"})
	val1 := *output

	// Reset and set via short form
	flags = New("test")
	output = flags.String("output", "o", "default", "Output file")
	flags.Parse([]string{"-o", "test2"})
	val2 := *output

	if val1 == val2 {
		t.Error("Different parses should yield different values")
	}
	if val1 != "test1" || val2 != "test2" {
		t.Errorf("Expected test1 and test2, got %s and %s", val1, val2)
	}
}
