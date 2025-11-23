package lexer

func isSymbol(ch byte) bool {
	switch ch {
	case '{', '}', '[', ']', ',', ':':
		return true
	}
	return false
}

func lexSymbol(ch byte) Token {
	switch ch {
	case '{':
		return Token{Type: TokLBrace}
	case '}':
		return Token{Type: TokRBrace}
	case '[':
		return Token{Type: TokLBracket}
	case ']':
		return Token{Type: TokRBracket}
	case ',':
		return Token{Type: TokComma}
	case ':':
		return Token{Type: TokColon}
	}
	panic("unreachable")
}
