package lexer

import "fmt"

func scanNumberOffsets(input string, i, n int) (Token, int) {
	start := i

	if input[i] == '-' {
		i++
	}

	for i < n && input[i] >= '0' && input[i] <= '9' {
		i++
	}

	if i < n && input[i] == '.' {
		i++
		for i < n && input[i] >= '0' && input[i] <= '9' {
			i++
		}
	}

	if i < n && (input[i] == 'e' || input[i] == 'E') {
		i++
		if i < n && (input[i] == '+' || input[i] == '-') {
			i++
		}
		for i < n && input[i] >= '0' && input[i] <= '9' {
			i++
		}
	}

	return Token{
		Type:  TokNumber,
		Start: start,
		End:   i,
	}, i
}

func scanKeywordOffsets(input string, i, n int) (Token, int, error) {
	if i+4 <= n && input[i:i+4] == "true" {
		return Token{Type: TokBool, Start: i, End: i + 4}, i + 4, nil
	}
	if i+5 <= n && input[i:i+5] == "false" {
		return Token{Type: TokBool, Start: i, End: i + 5}, i + 5, nil
	}
	if i+4 <= n && input[i:i+4] == "null" {
		return Token{Type: TokNull, Start: i, End: i + 4}, i + 4, nil
	}
	return Token{}, 0, fmt.Errorf("unexpected identifier")
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
