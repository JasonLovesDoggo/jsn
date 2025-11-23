package lexer

import "fmt"

type handler func(input string, i, n int, tokens []Token) (int, []Token, error)

// Jump table: byte → handler func
var dispatch [256]handler

func init() {
	// default
	for i := 0; i < 256; i++ {
		dispatch[i] = handleUnexpected
	}

	// whitespace
	for _, c := range []byte(" \t\n\r") {
		dispatch[c] = handleWhitespace
	}

	// quote
	dispatch['"'] = handleString

	// symbols
	symbols := map[byte]TokenType{
		'{': TokLBrace,
		'}': TokRBrace,
		'[': TokLBracket,
		']': TokRBracket,
		',': TokComma,
		':': TokColon,
	}
	for c, t := range symbols {
		symbolTable[c] = t
		dispatch[c] = handleSymbol
	}

	// digits
	for c := byte('0'); c <= '9'; c++ {
		dispatch[c] = handleNumber
	}

	// alpha
	for c := byte('a'); c <= 'z'; c++ {
		dispatch[c] = handleAlpha
	}
}

// ============================================================================
// Dispatch handlers
// ============================================================================

func handleWhitespace(input string, i, n int, tokens []Token) (int, []Token, error) {
	i++
	for i < n {
		ch := input[i]
		if ch != ' ' && ch != '\n' && ch != '\t' && ch != '\r' {
			break
		}
		i++
	}
	return i, tokens, nil
}

func handleSymbol(input string, i, n int, tokens []Token) (int, []Token, error) {
	tokens = append(tokens, Token{Type: symbolTable[input[i]]})
	return i + 1, tokens, nil
}
func handleString(input string, i, n int, tokens []Token) (int, []Token, error) {
	next, err := scanString(input, i, n)
	if err != nil {
		return 0, nil, err
	}

	tokens = append(tokens, Token{
		Type:  TokString,
		Start: i + 1,
		End:   next - 1,
	})
	return next, tokens, nil
}

func handleNumber(input string, i, n int, tokens []Token) (int, []Token, error) {
	tok, next := scanNumberOffsets(input, i, n)
	tokens = append(tokens, tok)
	return next, tokens, nil
}

func handleAlpha(input string, i, n int, tokens []Token) (int, []Token, error) {
	tok, next, err := scanKeywordOffsets(input, i, n)
	if err != nil {
		return 0, nil, err
	}
	tokens = append(tokens, tok)
	return next, tokens, nil
}

func handleUnexpected(input string, i, n int, tokens []Token) (int, []Token, error) {
	return 0, nil, fmt.Errorf("unexpected character: %c", input[i])
}

// ============================================================================
// Main entry point using jump table
// ============================================================================

func Lex(input string) ([]Token, error) {
	n := len(input)
	if n == 0 {
		return []Token{{Type: TokEOF}}, nil
	}

	_ = input[n-1] // bounds-check elimination

	// guess size of tokens
	capHint := len(input)/4 + 4 // works extremely well for JSON
	tokens := make([]Token, 0, capHint)

	i := 0

	for i < n {
		h := dispatch[input[i]]
		next, newTokens, err := h(input, i, n, tokens)
		if err != nil {
			return nil, err
		}
		i = next
		tokens = newTokens
	}

	tokens = append(tokens, Token{Type: TokEOF})
	return tokens, nil
}
