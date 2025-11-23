package lexer

import "fmt"

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isAlpha(ch byte) bool {
	return ch >= 'a' && ch <= 'z'
}

// faster, split loop, avoids checking j-1 every iteration
func scanString(input string, i, n int) (int, error) {
	// start AFTER opening quote
	j := i + 1

	for j < n {
		c := input[j]

		if c == '"' {
			// fast exit if no escape before quote
			if input[j-1] != '\\' {
				return j + 1, nil
			}

			// if escaped, fall through to slow path
			break
		}

		// fast forward until potential special char
		if c != '\\' {
			j++
			continue
		}

		// escape found — slow path
		break
	}

	// slower full scan for escapes + unicode
	for j < n {
		if input[j] == '"' && input[j-1] != '\\' {
			return j + 1, nil
		}
		j++
	}

	return 0, fmt.Errorf("unterminated string")
}
