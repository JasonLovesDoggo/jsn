package lexer

const (
	classWhitespace = 1
	classDigit      = 2
	classSymbol     = 3
	classAlpha      = 4
)

var classTable [256]byte

func init() {
	for _, c := range []byte(" \t\n\r") {
		classTable[c] = classWhitespace
	}
	for c := byte('0'); c <= '9'; c++ {
		classTable[c] = classDigit
	}
	for _, c := range []byte("{}[],:") {
		classTable[c] = classSymbol
	}
	for c := byte('a'); c <= 'z'; c++ {
		classTable[c] = classAlpha
	}
}
