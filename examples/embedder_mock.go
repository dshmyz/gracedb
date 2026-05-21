package main

import (
	"context"
)

// mockEmbedder generates deterministic embeddings from text for testing/demo purposes.
type mockEmbedder struct {
	dim int
}

func newMockEmbedder(dim int) *mockEmbedder {
	return &mockEmbedder{dim: dim}
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, m.dim)
	for i := 0; i < m.dim && i < len(text); i++ {
		vec[i] = float32(text[i]) / 255.0
	}
	return vec, nil
}

func (m *mockEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := m.Embed(context.Background(), t)
		if err != nil {
			return nil, err
		}
		vecs[i] = v
	}
	return vecs, nil
}

func (m *mockEmbedder) Dimension() int {
	return m.dim
}
