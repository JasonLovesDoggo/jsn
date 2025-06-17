package fluent

import (
	"testing"
	"time"
)

func TestFluentStringFlag(t *testing.T) {
	flags := New("test")

	output := flags.String("output").
		Short("o").
		Default("stdout").
		Usage("Output destination").
		Register()

	// Test default
	if *output != "stdout" {
		t.Errorf("Expected default 'stdout', got '%s'", *output)
	}

	// Test parsing
	flags.Parse([]string{"-o", "file.txt"})
	if *output != "file.txt" {
		t.Errorf("Expected 'file.txt', got '%s'", *output)
	}
}

func TestFluentBoolFlag(t *testing.T) {
	flags := New("test")

	verbose := flags.Bool("verbose").
		Short("v").
		Default(false).
		Usage("Enable verbose output").
		Register()

	flags.Parse([]string{"--verbose"})
	if !*verbose {
		t.Error("Expected verbose to be true")
	}
}

func TestFluentIntFlag(t *testing.T) {
	flags := New("test")

	count := flags.Int("count").
		Short("c").
		Default(10).
		Usage("Number of items").
		Register()

	flags.Parse([]string{"-c", "42"})
	if *count != 42 {
		t.Errorf("Expected 42, got %d", *count)
	}
}

func TestFluentFloat64Flag(t *testing.T) {
	flags := New("test")

	rate := flags.Float64("rate").
		Short("r").
		Default(1.0).
		Usage("Processing rate").
		Register()

	flags.Parse([]string{"--rate", "3.14"})
	if *rate != 3.14 {
		t.Errorf("Expected 3.14, got %f", *rate)
	}
}

func TestFluentDurationFlag(t *testing.T) {
	flags := New("test")

	timeout := flags.Duration("timeout").
		Short("t").
		Default(30 * time.Second).
		Usage("Request timeout").
		Register()

	flags.Parse([]string{"-t", "1m"})
	if *timeout != time.Minute {
		t.Errorf("Expected 1m, got %v", *timeout)
	}
}

func TestFluentChaining(t *testing.T) {
	flags := New("test")

	// Test that all methods return the builder for chaining
	flag := flags.String("test").Short("t").Default("value").Usage("test flag")
	if flag == nil {
		t.Error("Chaining should return builder")
	}

	ptr := flag.Register()
	if ptr == nil {
		t.Error("Register should return pointer")
	}
}

func TestFluentOptionalMethods(t *testing.T) {
	flags := New("test")

	// Test flag without short form
	output1 := flags.String("output1").
		Default("default").
		Usage("Output 1").
		Register()

	// Test flag without default (uses type zero value)
	output2 := flags.String("output2").
		Short("o").
		Usage("Output 2").
		Register()

	// Test flag with minimal setup
	verbose := flags.Bool("verbose").Register()

	flags.Parse([]string{"--output1", "test1", "-o", "test2", "--verbose"})

	if *output1 != "test1" {
		t.Errorf("Expected 'test1', got '%s'", *output1)
	}
	if *output2 != "test2" {
		t.Errorf("Expected 'test2', got '%s'", *output2)
	}
	if !*verbose {
		t.Error("Expected verbose to be true")
	}
}
