package flagr

import (
	"flag"
	"os"
	"testing"
)

func TestFlagrAndFlagInterop(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Reset command line for clean test
	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	
	// Reset flag.CommandLine to use our test instance
	flag.CommandLine = CommandLine.FlagSet

	// Define flags using both flagr and flag
	flagrOutput := String("output", "o", "default.txt", "Output file (flagr)")
	flagVerbose := flag.Bool("verbose", false, "Verbose mode (flag)")
	flagrCount := Int("count", "c", 10, "Count (flagr)")

	// Test that both flagr.Parse and flag.Parse work with the same underlying FlagSet
	os.Args = []string{"test", "--output", "test.txt", "--verbose", "--count", "42"}
	
	// Test flagr.Parse() - should work
	err := CommandLine.FlagSet.Parse(os.Args[1:])
	if err != nil {
		t.Fatalf("flagr.Parse() failed: %v", err)
	}

	// Verify values
	if *flagrOutput != "test.txt" {
		t.Errorf("Expected flagr output 'test.txt', got '%s'", *flagrOutput)
	}
	if !*flagVerbose {
		t.Error("Expected flag verbose to be true")
	}
	if *flagrCount != 42 {
		t.Errorf("Expected flagr count 42, got %d", *flagrCount)
	}

	// Test short forms work
	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError), 
		shorts:  make(map[string]string),
	}
	flag.CommandLine = CommandLine.FlagSet

	flagrOutput2 := String("output", "o", "default.txt", "Output file (flagr)")
	flagVerbose2 := flag.Bool("verbose", false, "Verbose mode (flag)")

	os.Args = []string{"test", "-o", "short.txt", "--verbose"}
	err = CommandLine.FlagSet.Parse(os.Args[1:])
	if err != nil {
		t.Fatalf("Short form parsing failed: %v", err)
	}

	if *flagrOutput2 != "short.txt" {
		t.Errorf("Expected short form output 'short.txt', got '%s'", *flagrOutput2)
	}
	if !*flagVerbose2 {
		t.Error("Expected flag verbose to be true")
	}
}

func TestBothParseMethodsWork(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create fresh instances 
	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	flag.CommandLine = CommandLine.FlagSet

	// Define flags
	flagrOutput := String("output", "o", "default.txt", "Output file")
	flagCount := flag.Int("count", 5, "Count value")

	os.Args = []string{"test", "--output", "example.txt", "--count", "15"}

	// Test that calling flagr.Parse() works
	Parse()

	if *flagrOutput != "example.txt" {
		t.Errorf("flagr.Parse(): Expected output 'example.txt', got '%s'", *flagrOutput)
	}
	if *flagCount != 15 {
		t.Errorf("flagr.Parse(): Expected count 15, got %d", *flagCount)
	}

	// Reset for second test
	CommandLine = &Flags{
		FlagSet: flag.NewFlagSet("test", flag.ContinueOnError),
		shorts:  make(map[string]string),
	}
	flag.CommandLine = CommandLine.FlagSet

	flagrOutput2 := String("output", "o", "default.txt", "Output file")
	flagCount2 := flag.Int("count", 5, "Count value")

	os.Args = []string{"test", "-o", "flag.txt", "--count", "25"}

	// Test that calling flag.Parse() also works
	flag.Parse()

	if *flagrOutput2 != "flag.txt" {
		t.Errorf("flag.Parse(): Expected output 'flag.txt', got '%s'", *flagrOutput2)
	}
	if *flagCount2 != 25 {
		t.Errorf("flag.Parse(): Expected count 25, got %d", *flagCount2)
	}
}