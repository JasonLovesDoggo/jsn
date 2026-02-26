package humantype

import (
	"math/rand"
	"time"
	"unicode"

	"github.com/go-vgo/robotgo"

	_ "github.com/go-vgo/robotgo/base"
	_ "github.com/go-vgo/robotgo/key"
	_ "github.com/go-vgo/robotgo/mouse"
)

var keyTap = robotgo.KeyTap

// NearbyKeys maps characters to their neighboring keys on a QWERTY keyboard.
var NearbyKeys = map[rune][]rune{
	'q': {'w', 'a', 's'}, 'w': {'q', 'e', 's', 'd', 'a'}, 'e': {'w', 'r', 'd', 'f', 's'}, 'r': {'e', 't', 'f', 'g', 'd'},
	't': {'r', 'y', 'g', 'h', 'f'}, 'y': {'t', 'u', 'h', 'j', 'g'}, 'u': {'y', 'i', 'j', 'k', 'h'}, 'i': {'u', 'o', 'k', 'l', 'j'},
	'o': {'i', 'p', 'l', ';', 'k'}, 'p': {'o', '[', ';', ':', 'l'}, 'a': {'q', 'w', 's', 'z', 'x'}, 's': {'a', 'd', 'w', 'e', 'x', 'z', 'c'},
	'd': {'s', 'f', 'e', 'r', 'x', 'c', 'v'}, 'f': {'d', 'g', 'r', 't', 'c', 'v', 'b'}, 'g': {'f', 'h', 't', 'y', 'v', 'b', 'n'},
	'h': {'g', 'j', 'y', 'u', 'b', 'n', 'm'}, 'j': {'h', 'k', 'u', 'i', 'n', 'm', ','}, 'k': {'j', 'l', 'i', 'o', 'm', ',', '.'},
	'l': {'k', ';', 'o', 'p', ',', '.', '/'}, 'z': {'a', 's', 'x'}, 'x': {'z', 'c', 's', 'd'}, 'c': {'x', 'v', 'd', 'f'},
	'v': {'c', 'b', 'f', 'g'}, 'b': {'v', 'n', 'g', 'h'}, 'n': {'b', 'm', 'h', 'j'}, 'm': {'n', ',', 'j', 'k'},
	' ': {' ', ' ', 'c', 'v', 'b', 'n', 'm'},
}

// Config holds configuration for human-like typing behavior.
type Config struct {
	// TypoChance is the percentage chance (0-100) of making a typo per character.
	TypoChance int
	// DoubleTypoChance is the percentage chance (0-200) of typing a char, deleting it, then retyping.
	DoubleTypoChance int
	// BaseDelay is the minimum delay between keystrokes in milliseconds.
	BaseDelay int
	// DelayVariance is the random variance added to BaseDelay in milliseconds.
	DelayVariance int
	// SpaceExtraDelay is additional delay after spaces in milliseconds.
	SpaceExtraDelay int
	// NewlineExtraDelay is additional delay after newlines in milliseconds.
	NewlineExtraDelay int
	// NewlineBaseDelay is base delay after newlines in milliseconds.
	NewlineBaseDelay int
	// PauseChance is the percentage chance (0-100) of a brief pause after typing.
	PauseChance int
	// PauseBase is the minimum pause duration in milliseconds.
	PauseBase int
	// PauseVariance is the random variance added to PauseBase in milliseconds.
	PauseVariance int
	// TabCompleteChance is the percentage chance (0-100) of tab-completing a word.
	TabCompleteChance int
	// TabCompleteMinLength is the minimum word length to consider for tab-completion.
	TabCompleteMinLength int
}

// DefaultConfig returns a Config with sensible defaults for human-like typing.
func DefaultConfig() Config {
	return Config{
		TypoChance:           2,
		DoubleTypoChance:     1,
		BaseDelay:            30,
		DelayVariance:        90,
		SpaceExtraDelay:      70,
		NewlineExtraDelay:    130,
		NewlineBaseDelay:     70,
		PauseChance:          20,
		PauseBase:            100,
		PauseVariance:        200,
		TabCompleteChance:    5,
		TabCompleteMinLength: 8,
	}
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// Type simulates human typing of the given text using the provided config.
func Type(text string, cfg Config) {
	runes := []rune(text)

	speedMultiplier := 1.0

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		// Slowly vary the speed multiplier
		speedMultiplier += (rand.Float64() - 0.5) * 0.05
		if speedMultiplier < 0.7 {
			speedMultiplier = 0.7
		}
		if speedMultiplier > 1.3 {
			speedMultiplier = 1.3
		}

		currentFactor := speedMultiplier

		// Check for tab completion
		if cfg.TabCompleteChance > 0 && isWordChar(char) {
			// Find word boundaries
			wordStart := i
			for wordStart > 0 && isWordChar(runes[wordStart-1]) {
				wordStart--
			}
			wordEnd := i
			for wordEnd < len(runes) && isWordChar(runes[wordEnd]) {
				wordEnd++
			}
			wordLen := wordEnd - wordStart
			typedInWord := i - wordStart

			// If we've typed some of it and it's long enough, maybe complete it
			if wordLen >= cfg.TabCompleteMinLength && typedInWord >= 2 && rand.Intn(100) < cfg.TabCompleteChance {
				for j := i; j < wordEnd; j++ {
					keyTap(string(runes[j]))
					// Almost instant typing for completion
					time.Sleep(time.Duration(rand.Intn(10)+5) * time.Millisecond)
				}
				i = wordEnd - 1
				continue
			}
		}

		// Occasional typo - hit a nearby key then backspace
		if rand.Intn(100) < cfg.TypoChance {
			if near, ok := NearbyKeys[unicode.ToLower(char)]; ok && len(near) > 0 {
				wrongChar := near[rand.Intn(len(near))]
				keyTap(string(wrongChar))
				time.Sleep(time.Duration(float64(rand.Intn(40)+60) * currentFactor) * time.Millisecond)
				keyTap("backspace")
				time.Sleep(time.Duration(float64(rand.Intn(20)+40) * currentFactor) * time.Millisecond)
			}
		}

		key := string(char)
		switch char {
		case '\n':
			key = "enter"
		case '\t':
			key = "tab"
		case '\r':
			continue
		}
		keyTap(key)

		// Rare double-tap typo - type, delete, retype
		if rand.Intn(200) < cfg.DoubleTypoChance && char != ' ' && char != '\n' && char != '\r' {
			time.Sleep(time.Duration(float64(rand.Intn(80)+70) * currentFactor) * time.Millisecond)
			keyTap("backspace")
			time.Sleep(time.Duration(float64(rand.Intn(40)+50) * currentFactor) * time.Millisecond)
			keyTap(key)
		}

		// Calculate delay
		delay := float64(rand.Intn(cfg.DelayVariance)+cfg.BaseDelay) * currentFactor
		if char == ' ' {
			delay += float64(rand.Intn(cfg.SpaceExtraDelay)) * currentFactor
		} else if char == '\n' {
			delay += float64(rand.Intn(cfg.NewlineExtraDelay)+cfg.NewlineBaseDelay) * currentFactor
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Occasional pause at end
	if rand.Intn(100) < cfg.PauseChance {
		delay := float64(rand.Intn(cfg.PauseVariance)+cfg.PauseBase)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
}

// TypeDefault types text using the default configuration.
func TypeDefault(text string) {
	Type(text, DefaultConfig())
}
