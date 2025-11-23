package lexer

import (
	"testing"

	jsoniter "github.com/json-iterator/go"
	segjson "github.com/segmentio/encoding/json"
	"github.com/tidwall/gjson"
)

func BenchmarkLex_Mine(b *testing.B) {
	input := `{"wow": 1}`
	for i := 0; i < b.N; i++ {
		_, _ = Lex(input)
	}
}

func BenchmarkLex_GJSON(b *testing.B) {
	input := `{"wow": 1}`
	for i := 0; i < b.N; i++ {
		_ = gjson.Parse(input)
	}
}

func BenchmarkLex_JsonIter(b *testing.B) {
	var v map[string]any
	input := []byte(`{"wow": 1}`)

	for i := 0; i < b.N; i++ {
		_ = jsoniter.Unmarshal(input, &v)
	}
}

func BenchmarkLex_Segmentio(b *testing.B) {
	var v map[string]any
	input := []byte(`{"wow": 1}`)

	for i := 0; i < b.N; i++ {
		_ = segjson.Unmarshal(input, &v)
	}
}
