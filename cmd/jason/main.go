package main

import (
	"os"

	"pkg.jsn.cam/jsn/pkg/jason/lexer"
)

func main() {
	tokens, err := lexer.Lex(os.Args[1])
	if err != nil {
		panic(err)
	}

	for _, tok := range tokens {
		println(tok.String())
	}

}
