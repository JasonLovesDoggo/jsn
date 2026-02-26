package humantype

import (
	"testing"
)

func TestType(t *testing.T) {
	var tappedKeys []string

	// Mock keyTap
	oldKeyTap := keyTap
	defer func() { keyTap = oldKeyTap }()

	keyTap = func(key string, args ...interface{}) error {
		tappedKeys = append(tappedKeys, key)
		return nil
	}

	cfg := DefaultConfig()
	cfg.TypoChance = 0        // Disable typos for predictable testing
	cfg.DoubleTypoChance = 0  // Disable double-tap typos
	cfg.PauseChance = 0       // Disable end pause

	// Use very small delays to speed up tests
	cfg.BaseDelay = 1
	cfg.DelayVariance = 1
	cfg.NewlineBaseDelay = 1
	cfg.NewlineExtraDelay = 1
	cfg.SpaceExtraDelay = 1

	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "basic text",
			input:    "abc",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with newline",
			input:    "a\nb",
			expected: []string{"a", "enter", "b"},
		},
		{
			name:     "with tab",
			input:    "a\tb",
			expected: []string{"a", "tab", "b"},
		},
		{
			name:     "with carriage return",
			input:    "a\r\nb",
			expected: []string{"a", "enter", "b"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tappedKeys = nil
			Type(tc.input, cfg)

			if len(tappedKeys) != len(tc.expected) {
				t.Fatalf("expected %d keys, got %d: %v", len(tc.expected), len(tappedKeys), tappedKeys)
			}

			for i := range tc.expected {
				if tappedKeys[i] != tc.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tc.expected[i], tappedKeys[i])
				}
			}
		})
	}
}

func TestType_TabComplete(t *testing.T) {
	var tappedKeys []string

	// Mock keyTap
	oldKeyTap := keyTap
	defer func() { keyTap = oldKeyTap }()

	keyTap = func(key string, args ...interface{}) error {
		tappedKeys = append(tappedKeys, key)
		return nil
	}

	cfg := DefaultConfig()
	cfg.TypoChance = 0
	cfg.DoubleTypoChance = 0
	cfg.PauseChance = 0
	cfg.BaseDelay = 0
	cfg.DelayVariance = 1 // minimal variance
	cfg.TabCompleteChance = 100
	cfg.TabCompleteMinLength = 4

	input := "superlongword"
	Type(input, cfg)

	actual := ""
	for _, k := range tappedKeys {
		actual += k
	}
	if actual != input {
		t.Errorf("expected %q, got %q", input, actual)
	}
}

func TestType_WPM(t *testing.T) {
	var tappedKeys []string

	// Mock keyTap
	oldKeyTap := keyTap
	defer func() { keyTap = oldKeyTap }()

	keyTap = func(key string, args ...interface{}) error {
		tappedKeys = append(tappedKeys, key)
		return nil
	}

	cfg := DefaultConfig()
	cfg.TypoChance = 0
	cfg.DoubleTypoChance = 0
	cfg.PauseChance = 0
	cfg.WPM = 1000 // Very fast, should not hang
	cfg.TabCompleteChance = 0

	input := "speedy"
	Type(input, cfg)

	if len(tappedKeys) != len(input) {
		t.Errorf("expected %d keys, got %d", len(input), len(tappedKeys))
	}
}
