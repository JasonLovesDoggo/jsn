package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func isLikelyVectorInput(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		switch {
		case unicode.IsSpace(r):
			continue
		case r == ',' || r == '.' || r == '-' || r == '+' || r == '[' || r == ']' || r == 'e' || r == 'E':
			continue
		case unicode.IsDigit(r):
			continue
		default:
			return false
		}
	}
	return true
}

func resolveEmbeddingModel(profile Profile) string {
	return strings.TrimSpace(profile.EmbeddingModel)
}

func resolveOpenAIKey(profile Profile) string {
	if key := strings.TrimSpace(profile.OpenAIAPIKey); key != "" {
		return key
	}
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
}

func embedQuery(ctx context.Context, profile Profile, text string) ([]float32, error) {
	model := resolveEmbeddingModel(profile)
	if model == "" {
		return nil, fmt.Errorf("embedding_model not set; add it to config or provide a vector literal")
	}
	key := resolveOpenAIKey(profile)
	if key == "" {
		return nil, fmt.Errorf("OpenAI API key missing; set openai_api_key or OPENAI_API_KEY")
	}

	client := openai.NewClient(option.WithAPIKey(key))
	resp, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
		Model: openai.EmbeddingModel(model),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Data) == 0 {
		return nil, fmt.Errorf("OpenAI embeddings: empty response")
	}
	raw := resp.Data[0].Embedding
	out := make([]float32, len(raw))
	for i, v := range raw {
		out[i] = float32(v)
	}
	return out, nil
}
