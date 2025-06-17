package fluent

import (
	"os"
	"testing"
	"time"

	"pkg.jsn.cam/jsn/pkg/flagr"
)

func TestFluentStringFlag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{flagr.New("test")}

	output := String("output").
		Short("o").
		Default("stdout").
		Usage("Output destination").
		Register()

	// Test default
	if *output != "stdout" {
		t.Errorf("Expected default 'stdout', got '%s'", *output)
	}

	// Test parsing
	os.Args = []string{"test", "-o", "file.txt"}
	Parse()
	if *output != "file.txt" {
		t.Errorf("Expected 'file.txt', got '%s'", *output)
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	output2 := flags.String("output").
		Short("o").
		Default("stdout").
		Usage("Output destination").
		Register()

	flags.FlagSet.Parse([]string{"-o", "file2.txt"})
	if *output2 != "file2.txt" {
		t.Errorf("Expected 'file2.txt', got '%s'", *output2)
	}
}

func TestFluentBoolFlag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{flagr.New("test")}

	verbose := Bool("verbose").
		Short("v").
		Default(false).
		Usage("Enable verbose output").
		Register()

	os.Args = []string{"test", "--verbose"}
	Parse()
	if !*verbose {
		t.Error("Expected verbose to be true")
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	verbose2 := flags.Bool("verbose").
		Short("v").
		Default(false).
		Usage("Enable verbose output").
		Register()

	flags.FlagSet.Parse([]string{"--verbose"})
	if !*verbose2 {
		t.Error("Expected verbose to be true")
	}
}

func TestFluentIntFlag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{flagr.New("test")}

	count := Int("count").
		Short("c").
		Default(10).
		Usage("Number of items").
		Register()

	os.Args = []string{"test", "-c", "42"}
	Parse()
	if *count != 42 {
		t.Errorf("Expected 42, got %d", *count)
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	count2 := flags.Int("count").
		Short("c").
		Default(10).
		Usage("Number of items").
		Register()

	flags.FlagSet.Parse([]string{"-c", "42"})
	if *count2 != 42 {
		t.Errorf("Expected 42, got %d", *count2)
	}
}

func TestFluentFloat64Flag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{flagr.New("test")}

	rate := Float64("rate").
		Short("r").
		Default(1.0).
		Usage("Processing rate").
		Register()

	os.Args = []string{"test", "--rate", "3.14"}
	Parse()
	if *rate != 3.14 {
		t.Errorf("Expected 3.14, got %f", *rate)
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	rate2 := flags.Float64("rate").
		Short("r").
		Default(1.0).
		Usage("Processing rate").
		Register()

	flags.FlagSet.Parse([]string{"--rate", "3.14"})
	if *rate2 != 3.14 {
		t.Errorf("Expected 3.14, got %f", *rate2)
	}
}

func TestFluentDurationFlag(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{flagr.New("test")}

	timeout := Duration("timeout").
		Short("t").
		Default(30 * time.Second).
		Usage("Request timeout").
		Register()

	os.Args = []string{"test", "-t", "1m"}
	Parse()
	if *timeout != time.Minute {
		t.Errorf("Expected 1m, got %v", *timeout)
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	timeout2 := flags.Duration("timeout").
		Short("t").
		Default(30 * time.Second).
		Usage("Request timeout").
		Register()

	flags.FlagSet.Parse([]string{"-t", "1m"})
	if *timeout2 != time.Minute {
		t.Errorf("Expected 1m, got %v", *timeout2)
	}
}

func TestFluentChaining(t *testing.T) {
	// Test with package-level functions
	CommandLine = &Flags{flagr.New("test")}

	// Test that all methods return the builder for chaining
	flag := String("test").Short("t").Default("value").Usage("test flag")
	if flag == nil {
		t.Error("Chaining should return builder")
	}

	ptr := flag.Register()
	if ptr == nil {
		t.Error("Register should return pointer")
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	flag2 := flags.String("test2").Short("t").Default("value").Usage("test flag")
	if flag2 == nil {
		t.Error("Chaining should return builder")
	}

	ptr2 := flag2.Register()
	if ptr2 == nil {
		t.Error("Register should return pointer")
	}
}

func TestFluentOptionalMethods(t *testing.T) {
	// Test with package-level functions
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	CommandLine = &Flags{flagr.New("test")}

	// Test flag without short form
	output1 := String("output1").
		Default("default").
		Usage("Output 1").
		Register()

	// Test flag without default (uses type zero value)
	output2 := String("output2").
		Short("o").
		Usage("Output 2").
		Register()

	// Test flag with minimal setup
	verbose := Bool("verbose").Register()

	os.Args = []string{"test", "--output1", "test1", "-o", "test2", "--verbose"}
	Parse()

	if *output1 != "test1" {
		t.Errorf("Expected 'test1', got '%s'", *output1)
	}
	if *output2 != "test2" {
		t.Errorf("Expected 'test2', got '%s'", *output2)
	}
	if !*verbose {
		t.Error("Expected verbose to be true")
	}

	// Test with New method (backward compatibility)
	flags := New("test")
	output3 := flags.String("output3").
		Default("default").
		Usage("Output 3").
		Register()

	flags.FlagSet.Parse([]string{"--output3", "test3"})
	if *output3 != "test3" {
		t.Errorf("Expected 'test3', got '%s'", *output3)
	}
}
