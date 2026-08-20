package storage_test

import (
	"context"
	"testing"

	"github.com/aeremin/neurodan/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestUpsertAndGetTransitions(t *testing.T) {
	client, err := storage.NewClient("localhost", 6334)
	if err != nil {
		t.Skip("Qdrant not available")
	}
	defer client.Close()

	err = client.EnsureCollections(context.Background())
	if err != nil {
		t.Skip("Qdrant not available")
	}

	const chatID int64 = 12345

	err = client.UpsertTransition(context.Background(), chatID, "hello", "world", "foo", 1)
	require.NoError(t, err)

	transitions, err := client.GetTransitions(context.Background(), chatID, "hello", "world")
	require.NoError(t, err)
	require.Len(t, transitions, 1)
	require.Equal(t, "foo", transitions[0].NextWord)
	require.Equal(t, 1, transitions[0].Weight)

	err = client.UpsertTransition(context.Background(), chatID, "hello", "world", "foo", 1)
	require.NoError(t, err)

	transitions, err = client.GetTransitions(context.Background(), chatID, "hello", "world")
	require.NoError(t, err)
	require.Len(t, transitions, 1)
	require.Equal(t, 2, transitions[0].Weight)

	transitions, err = client.GetTransitions(context.Background(), chatID, "no", "match")
	require.NoError(t, err)
	require.Empty(t, transitions)
}
