package lexer

type TokenType int

const (
	TokLBrace TokenType = iota
	TokRBrace
	TokLBracket
	TokRBracket
	TokComma
	TokColon
	TokString
	TokNumber
	TokBool
	TokNull
	TokEOF
)

type Token struct {
	Type  TokenType
	Start int
	End   int
}

func (t Token) String() string {
	switch t.Type {
	case TokLBrace:
		return "TokLBrace"
	case TokRBrace:
		return "TokRBrace"
	case TokLBracket:
		return "TokLBracket"
	case TokRBracket:
		return "TokRBracket"
	case TokComma:
		return "TokComma"
	case TokColon:
		return "TokColon"
	case TokString:
		return "TokString[" + string(rune(t.Start)) + "," + string(rune(t.End)) + "]"
	case TokNumber:
		return "TokNumber[" + string(rune(t.Start)) + "," + string(rune(t.End)) + "]"
	case TokBool:
		return "TokBool" + "[" + string(rune(t.Start)) + "," + string(rune(t.End)) + "]"
	case TokNull:
		return "TokNull"
	case TokEOF:
		return "TokEOF"
	default:
		return "Unknown"
	}
}

var symbolTable [256]TokenType
