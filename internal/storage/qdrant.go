package storage

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

type Client struct {
	grpcClient *qdrant.Client
}

func NewClient(host string, port int) (*Client, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                 host,
		Port:                 port,
		PoolSize:             1,
		SkipCompatibilityCheck: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to qdrant: %w", err)
	}
	return &Client{grpcClient: client}, nil
}

func (c *Client) Close() error {
	return c.grpcClient.Close()
}

func (c *Client) EnsureCollections(ctx context.Context) error {
	existing, err := c.grpcClient.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	existMap := make(map[string]bool, len(existing))
	for _, name := range existing {
		existMap[name] = true
	}

	if !existMap["markov_transitions"] {
		err = c.grpcClient.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: "markov_transitions",
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     1,
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			return fmt.Errorf("failed to create markov_transitions: %w", err)
		}
	}

	if !existMap["sentence_embeddings"] {
		err = c.grpcClient.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: "sentence_embeddings",
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     384,
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			return fmt.Errorf("failed to create sentence_embeddings: %w", err)
		}
	}

	return nil
}
