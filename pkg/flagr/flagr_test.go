package flagr

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStringFlag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test long form
	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	output := String("output", "o", "default.txt", "Output file")
	os.Args = []string{"test", "--output", "long.txt"}
	Parse()
	if *output != "long.txt" {
		t.Errorf("Expected 'long.txt', got '%s'", *output)
	}

	// Test short form
	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	output = String("output", "o", "default.txt", "Output file")
	os.Args = []string{"test", "-o", "short.txt"}
	Parse()
	if *output != "short.txt" {
		t.Errorf("Expected 'short.txt', got '%s'", *output)
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	output2 := flags.String("output", "o", "default.txt", "Output file")
	flags.FlagSet.Parse([]string{"--output", "new.txt"})
	if *output2 != "new.txt" {
		t.Errorf("Expected 'new.txt', got '%s'", *output2)
	}
}

func TestBoolFlag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	verbose := Bool("verbose", "v", false, "Verbose output")
	os.Args = []string{"test", "-v"}
	Parse()
	if !*verbose {
		t.Error("Expected verbose to be true")
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	verbose2 := flags.Bool("verbose", "v", false, "Verbose output")
	flags.FlagSet.Parse([]string{"-v"})
	if !*verbose2 {
		t.Error("Expected verbose to be true")
	}
}

func TestIntFlag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	count := Int("count", "c", 10, "Count value")
	os.Args = []string{"test", "--count", "42"}
	Parse()
	if *count != 42 {
		t.Errorf("Expected 42, got %d", *count)
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	count2 := flags.Int("count", "c", 10, "Count value")
	flags.FlagSet.Parse([]string{"--count", "42"})
	if *count2 != 42 {
		t.Errorf("Expected 42, got %d", *count2)
	}
}

func TestFloat64Flag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	rate := Float64("rate", "r", 1.5, "Rate value")
	os.Args = []string{"test", "-r", "3.14"}
	Parse()
	if *rate != 3.14 {
		t.Errorf("Expected 3.14, got %f", *rate)
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	rate2 := flags.Float64("rate", "r", 1.5, "Rate value")
	flags.FlagSet.Parse([]string{"-r", "3.14"})
	if *rate2 != 3.14 {
		t.Errorf("Expected 3.14, got %f", *rate2)
	}
}

func TestDurationFlag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	timeout := Duration("timeout", "t", time.Second, "Timeout duration")
	os.Args = []string{"test", "--timeout", "30s"}
	Parse()
	if *timeout != 30*time.Second {
		t.Errorf("Expected 30s, got %v", *timeout)
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	timeout2 := flags.Duration("timeout", "t", time.Second, "Timeout duration")
	flags.FlagSet.Parse([]string{"--timeout", "30s"})
	if *timeout2 != 30*time.Second {
		t.Errorf("Expected 30s, got %v", *timeout2)
	}
}

func TestNoShortForm(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	debug := Bool("debug", "", false, "Debug mode")
	os.Args = []string{"test", "--debug"}
	Parse()
	if !*debug {
		t.Error("Expected debug to be true")
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	debug2 := flags.Bool("debug", "", false, "Debug mode")
	flags.FlagSet.Parse([]string{"--debug"})
	if !*debug2 {
		t.Error("Expected debug to be true")
	}
}

func TestUsage(t *testing.T) {
	// Test with package-level functions
	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	var buf bytes.Buffer
	SetOutput(&buf)

	String("output", "o", "stdout", "Output file")
	Bool("verbose", "v", false, "Verbose output")
	Bool("debug", "", false, "Debug mode")

	Usage()

	output := buf.String()
	if !strings.Contains(output, "-o, --output") {
		t.Error("Usage should show both short and long forms")
	}
	if !strings.Contains(output, "--debug") && !strings.Contains(output, "-d, --debug") {
		t.Error("Usage should show long form for flags without short form")
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	var buf2 bytes.Buffer
	flags.SetOutput(&buf2)

	flags.String("output", "o", "stdout", "Output file")
	flags.Bool("verbose", "v", false, "Verbose output")
	flags.Bool("debug", "", false, "Debug mode")

	flags.Usage()

	output2 := buf2.String()
	if !strings.Contains(output2, "-o, --output") {
		t.Error("Usage should show both short and long forms")
	}
	if !strings.Contains(output2, "--debug") && !strings.Contains(output2, "-d, --debug") {
		t.Error("Usage should show long form for flags without short form")
	}
}

func TestBothFormsSetSameValue(t *testing.T) {
	// Test with New method (backward compatibility)
	flags := New("test")
	output := flags.String("output", "o", "default", "Output file")

	// Set via long form
	flags.FlagSet.Parse([]string{"--output", "test1"})
	val1 := *output

	// Reset and set via short form
	flags = New("test")
	output = flags.String("output", "o", "default", "Output file")
	flags.FlagSet.Parse([]string{"-o", "test2"})
	val2 := *output

	if val1 == val2 {
		t.Error("Different parses should yield different values")
	}
	if val1 != "test1" || val2 != "test2" {
		t.Errorf("Expected test1 and test2, got %s and %s", val1, val2)
	}
}
