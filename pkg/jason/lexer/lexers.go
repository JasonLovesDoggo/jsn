package lexer

import (
	"fmt"
)

func isAlpha(ch byte) bool {
	return ch >= 'a' && ch <= 'z'
}

func lexKeyword(input string, i int) (Token, int, error) {
	start := i
	for i < len(input) && isAlpha(input[i]) {
		i++
	}

	word := input[start:i]

	switch word {
	case "true", "false":
		return Token{Type: TokBool, Value: word}, i, nil
	case "null":
		return Token{Type: TokNull}, i, nil
	}

	return Token{}, 0, fmt.Errorf("unexpected identifier: %s", word)
}

func lexNumber(input string, i int) (Token, int) {
	start := i

	if input[i] == '-' {
		i++
	}

	for i < len(input) && isDigit(input[i]) {
		i++
	}

	if i < len(input) && input[i] == '.' {
		i++
		for i < len(input) && isDigit(input[i]) {
			i++
		}
	}

	if i < len(input) && (input[i] == 'e' || input[i] == 'E') {
		i++
		if i < len(input) && (input[i] == '+' || input[i] == '-') {
			i++
		}
		for i < len(input) && isDigit(input[i]) {
			i++
		}
	}

	return Token{Type: TokNumber, Value: input[start:i]}, i
}

func lexString(input string, i int) (Token, int, error) {
	i++ // skip opening "
	start := i
	var chars []byte

	for i < len(input) {
		// closing quote not escaped
		if input[i] == '"' && (i == start || input[i-1] != '\\') {
			value := string(chars)
			return Token{Type: TokString, Value: value}, i + 1, nil
		}

		chars = append(chars, input[i])
		i++
	}

	return Token{}, 0, fmt.Errorf("unterminated string literal")
}
