package storage_test

import (
	"context"
	"testing"

	"github.com/aeremin/neurodan/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client, err := storage.NewClient("localhost", 6334)
	if err != nil {
		t.Skip("Qdrant not available, skipping")
	}
	require.NotNil(t, client)
	defer client.Close()
}

func TestEnsureCollections(t *testing.T) {
	client, err := storage.NewClient("localhost", 6334)
	require.NoError(t, err)
	defer client.Close()

	err = client.EnsureCollections(context.Background())
	if err != nil {
		t.Skip("Qdrant not available, skipping")
	}

	err = client.EnsureCollections(context.Background())
	require.NoError(t, err, "EnsureCollections should be idempotent")
}
