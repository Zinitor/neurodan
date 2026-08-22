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

	tr := storage.Transition{
		Order:    2,
		Prefix:   []string{"hello", "world"},
		NextWord: "foo",
		Weight:   1,
	}

	err = client.UpsertTransition(context.Background(), chatID, tr)
	require.NoError(t, err)

	transitions, err := client.GetAllTransitions(context.Background(), chatID)
	require.NoError(t, err)
	require.Len(t, transitions, 1)
	require.Equal(t, "foo", transitions[0].NextWord)
	require.Equal(t, []string{"hello", "world"}, transitions[0].Prefix)
	require.Equal(t, 1, transitions[0].Weight)

	// Re-upserting the same transition aggregates weight instead of duplicating.
	err = client.UpsertTransition(context.Background(), chatID, tr)
	require.NoError(t, err)

	transitions, err = client.GetAllTransitions(context.Background(), chatID)
	require.NoError(t, err)
	require.Len(t, transitions, 1)
	require.Equal(t, 2, transitions[0].Weight)

	// Different chat is isolated.
	other, err := client.GetAllTransitions(context.Background(), 999)
	require.NoError(t, err)
	require.Empty(t, other)
}
