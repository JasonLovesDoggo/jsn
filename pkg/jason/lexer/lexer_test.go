package lexer

import (
	"testing"

	jsoniter "github.com/json-iterator/go"
	segjson "github.com/segmentio/encoding/json"
	"github.com/tidwall/gjson"
)

var benchInputs = []struct {
	name string
	data string
}{
	{
		name: "tiny",
		data: `{"wow":1}`,
	},
	{
		name: "small",
		data: `{"user":{"id":1,"name":"Jason","active":true},"scores":[1,2,3],"ok":false}`,
	},
	{
		name: "medium",
		data: `{"user":{"id":123,"name":"Jason","meta":{"age":17,"city":"Toronto"}},` +
			`"tags":["go","json","lexer"],"posts":[{"id":1,"title":"hello"},{"id":2,"title":"world"}]}`,
	},
	{
		name: "large",
		data: `{"feed":[{"id":1,"text":"hello world","likes":10},{"id":2,"text":"test123","likes":22},` +
			`{"id":3,"text":"more data","likes":17},{"id":4,"text":"more content","likes":55}]}`,
	},
}

// ============================================================================
// Mine
// ============================================================================
func BenchmarkLex_Mine(b *testing.B) {
	for _, tc := range benchInputs {
		b.Run(tc.name, func(b *testing.B) {
			input := tc.data
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			for i := 0; i < b.N; i++ {
				_, _ = Lex(input)
			}
		})
	}
}

// ============================================================================
// GJSON (fast scan)
// ============================================================================
func BenchmarkLex_GJSON(b *testing.B) {
	for _, tc := range benchInputs {
		b.Run(tc.name, func(b *testing.B) {
			input := tc.data
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			for i := 0; i < b.N; i++ {
				_ = gjson.Parse(input)
			}
		})
	}
}

// ============================================================================
// jsoniter (full parse to map)
// ============================================================================
func BenchmarkLex_JsonIter(b *testing.B) {
	for _, tc := range benchInputs {
		b.Run(tc.name, func(b *testing.B) {
			input := []byte(tc.data)
			var v map[string]any
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			for i := 0; i < b.N; i++ {
				_ = jsoniter.Unmarshal(input, &v)
			}
		})
	}
}

// ============================================================================
// segmentio (fast full parse)
// ============================================================================
func BenchmarkLex_Segmentio(b *testing.B) {
	for _, tc := range benchInputs {
		b.Run(tc.name, func(b *testing.B) {
			input := []byte(tc.data)
			var v map[string]any
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			for i := 0; i < b.N; i++ {
				_ = segjson.Unmarshal(input, &v)
			}
		})
	}
}
