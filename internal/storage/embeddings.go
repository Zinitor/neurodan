package storage

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

func (c *Client) StoreEmbedding(ctx context.Context, chatID int64, text string, vector []float32) error {
	pointsClient := c.grpcClient.GetPointsClient()

	pointID := fmt.Sprintf("%d:%s", chatID, text)

	points := []*qdrant.PointStruct{
		{
			Id:      qdrant.NewIDUUID(pointID),
			Vectors: qdrant.NewVectors(vector...),
			Payload: map[string]*qdrant.Value{
				"chat_id": qdrant.NewValueInt(chatID),
				"text":    qdrant.NewValueString(text),
			},
		},
	}

	_, err := pointsClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: "sentence_embeddings",
		Points:         points,
	})
	return fmt.Errorf("failed to upsert embedding: %w", err)
}

func (c *Client) FindSimilar(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
	pointsClient := c.grpcClient.GetPointsClient()

	scoreThreshold := threshold

	searchResult, err := pointsClient.Search(ctx, &qdrant.SearchPoints{
		CollectionName: "sentence_embeddings",
		Vector:         vector,
		Limit:          10,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchInt("chat_id", chatID),
			},
		},
		ScoreThreshold: &scoreThreshold,
		WithPayload:    qdrant.NewWithPayloadInclude("text"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}

	var texts []string
	for _, hit := range searchResult.GetResult() {
		texts = append(texts, hit.GetPayload()["text"].GetStringValue())
	}
	return texts, nil
}
