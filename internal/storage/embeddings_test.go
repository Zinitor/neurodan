package storage_test

import (
	"context"
	"testing"

	"github.com/aeremin/neurodan/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestStoreAndFindSimilar(t *testing.T) {
	client, err := storage.NewClient("localhost", 6334)
	if err != nil {
		t.Skip("Qdrant not available")
	}
	defer client.Close()

	err = client.EnsureCollections(context.Background())
	if err != nil {
		t.Skip("Qdrant not available:", err)
	}

	const chatID int64 = 99999

	vec := make([]float32, 384)
	vec[0] = 1.0
	err = client.StoreEmbedding(context.Background(), chatID, "hello world", vec)
	require.NoError(t, err)

	vec2 := make([]float32, 384)
	vec2[0] = 0.99
	vec2[1] = 0.01
	err = client.StoreEmbedding(context.Background(), chatID, "hello world again", vec2)
	require.NoError(t, err)

	// Similar with low threshold
	similar, err := client.FindSimilar(context.Background(), chatID, vec, 0.5)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(similar), 1)

	// Similar with high threshold
	similar, err = client.FindSimilar(context.Background(), chatID, vec, 0.999)
	require.NoError(t, err)
	require.Len(t, similar, 1)
	require.Equal(t, "hello world", similar[0])

	// Different chat — nothing
	similar, err = client.FindSimilar(context.Background(), 11111, vec, 0.5)
	require.NoError(t, err)
	require.Empty(t, similar)
}
