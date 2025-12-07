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
}

// DefaultConfig returns a Config with sensible defaults for human-like typing.
func DefaultConfig() Config {
	return Config{
		TypoChance:        2,
		DoubleTypoChance:  1,
		BaseDelay:         30,
		DelayVariance:     90,
		SpaceExtraDelay:   70,
		NewlineExtraDelay: 130,
		NewlineBaseDelay:  70,
		PauseChance:       20,
		PauseBase:         100,
		PauseVariance:     200,
	}
}

// Type simulates human typing of the given text using the provided config.
func Type(text string, cfg Config) {
	for _, char := range text {
		// Occasional typo - hit a nearby key then backspace
		if rand.Intn(100) < cfg.TypoChance {
			if near, ok := NearbyKeys[unicode.ToLower(char)]; ok && len(near) > 0 {
				wrongChar := near[rand.Intn(len(near))]
				robotgo.KeyTap(string(wrongChar))
				time.Sleep(time.Duration(rand.Intn(40)+60) * time.Millisecond)
				robotgo.KeyTap("backspace")
				time.Sleep(time.Duration(rand.Intn(20)+40) * time.Millisecond)
			}
		}

		robotgo.KeyTap(string(char))

		// Rare double-tap typo - type, delete, retype
		if rand.Intn(200) < cfg.DoubleTypoChance && char != ' ' && char != '\n' {
			time.Sleep(time.Duration(rand.Intn(80)+70) * time.Millisecond)
			robotgo.KeyTap("backspace")
			time.Sleep(time.Duration(rand.Intn(40)+50) * time.Millisecond)
			robotgo.KeyTap(string(char))
		}

		// Calculate delay
		delay := rand.Intn(cfg.DelayVariance) + cfg.BaseDelay
		if char == ' ' {
			delay += rand.Intn(cfg.SpaceExtraDelay)
		} else if char == '\n' {
			delay += rand.Intn(cfg.NewlineExtraDelay) + cfg.NewlineBaseDelay
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Occasional pause at end
	if rand.Intn(100) < cfg.PauseChance {
		time.Sleep(time.Duration(rand.Intn(cfg.PauseVariance)+cfg.PauseBase) * time.Millisecond)
	}
}

// TypeDefault types text using the default configuration.
func TypeDefault(text string) {
	Type(text, DefaultConfig())
}
