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
	Value string
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
		return "TokString(" + t.Value + ")"
	case TokNumber:
		return "TokNumber(" + t.Value + ")"
	case TokBool:
		return "TokBool(" + t.Value + ")"
	case TokNull:
		return "TokNull"
	case TokEOF:
		return "TokEOF"
	default:
		return "Unknown"
	}
}
