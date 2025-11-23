package lexer

import "fmt"

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func Lex(input string) ([]Token, error) {
	var tokens []Token
	i := 0

	for i < len(input) {
		ch := input[i]

		switch {
		case isWhitespace(ch):
			i++

		case isSymbol(ch):
			tokens = append(tokens, lexSymbol(ch))
			i++

		case ch == '"':
			tok, next, err := lexString(input, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next

		case ch == '-' || isDigit(ch):
			tok, next := lexNumber(input, i)
			tokens = append(tokens, tok)
			i = next

		case isAlpha(ch):
			tok, next, err := lexKeyword(input, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next

		default:
			return nil, fmt.Errorf("unexpected character: %c", ch)
		}
	}

	tokens = append(tokens, Token{Type: TokEOF})
	return tokens, nil
}
