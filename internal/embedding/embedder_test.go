package embedding_test

import (
	"testing"

	"github.com/aeremin/neurodan/internal/embedding"
	"github.com/stretchr/testify/require"
)

func TestEmbedderProducesVector(t *testing.T) {
	e, err := embedding.NewEmbedder("../../models/model.onnx")
	if err != nil {
		t.Skip("model.onnx not available")
	}
	defer e.Close()

	vec, err := e.Embed("hello world")
	require.NoError(t, err)
	require.Len(t, vec, 384)
}

func TestEmbedderConsistency(t *testing.T) {
	e, err := embedding.NewEmbedder("../../models/model.onnx")
	if err != nil {
		t.Skip("model.onnx not available")
	}
	defer e.Close()

	vec1, _ := e.Embed("hello world")
	vec2, _ := e.Embed("hello world")

	for i := range vec1 {
		require.InDelta(t, vec1[i], vec2[i], 0.001)
	}
}
