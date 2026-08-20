package storage

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

type Transition struct {
	Word1    string
	Word2    string
	NextWord string
	Weight   int
}

func (c *Client) UpsertTransition(ctx context.Context, chatID int64, word1, word2, nextWord string, weight int) error {
	pointsClient := c.grpcClient.GetPointsClient()

	pointID := fmt.Sprintf("%d:%s:%s:%s", chatID, word1, word2, nextWord)

	existingResp, err := pointsClient.Get(ctx, &qdrant.GetPoints{
		CollectionName: "markov_transitions",
		Ids:            []*qdrant.PointId{qdrant.NewIDUUID(pointID)},
		WithPayload:    qdrant.NewWithPayloadEnable(true),
	})
	if err != nil {
		return fmt.Errorf("failed to get existing transition: %w", err)
	}

	totalWeight := weight
	existing := existingResp.GetResult()
	if len(existing) > 0 {
		payload := existing[0].GetPayload()
		totalWeight += int(payload["weight"].GetIntegerValue())
	}

	points := []*qdrant.PointStruct{
		{
			Id:      qdrant.NewIDUUID(pointID),
			Vectors: qdrant.NewVectors(0.0),
			Payload: map[string]*qdrant.Value{
				"chat_id":   qdrant.NewValueInt(chatID),
				"word1":     qdrant.NewValueString(word1),
				"word2":     qdrant.NewValueString(word2),
				"next_word": qdrant.NewValueString(nextWord),
				"weight":    qdrant.NewValueInt(int64(totalWeight)),
			},
		},
	}

	_, err = pointsClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: "markov_transitions",
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert transition: %w", err)
	}
	return nil
}

func (c *Client) GetAllTransitions(ctx context.Context, chatID int64) ([]Transition, error) {
	pointsClient := c.grpcClient.GetPointsClient()

	resp, err := pointsClient.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: "markov_transitions",
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchInt("chat_id", chatID),
			},
		},
		WithPayload: qdrant.NewWithPayloadEnable(true),
		Limit:       qdrant.PtrOf(uint32(10000)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all transitions: %w", err)
	}

	var transitions []Transition
	for _, point := range resp.GetResult() {
		payload := point.GetPayload()
		transitions = append(transitions, Transition{
			Word1:    payload["word1"].GetStringValue(),
			Word2:    payload["word2"].GetStringValue(),
			NextWord: payload["next_word"].GetStringValue(),
			Weight:   int(payload["weight"].GetIntegerValue()),
		})
	}
	return transitions, nil
}

func (c *Client) GetTransitions(ctx context.Context, chatID int64, word1, word2 string) ([]Transition, error) {
	pointsClient := c.grpcClient.GetPointsClient()

	resp, err := pointsClient.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: "markov_transitions",
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchInt("chat_id", chatID),
				qdrant.NewMatchKeyword("word1", word1),
				qdrant.NewMatchKeyword("word2", word2),
			},
		},
		WithPayload: qdrant.NewWithPayloadEnable(true),
		Limit:       qdrant.PtrOf(uint32(1000)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transitions: %w", err)
	}

	var transitions []Transition
	for _, point := range resp.GetResult() {
		payload := point.GetPayload()
		transitions = append(transitions, Transition{
			Word1:    payload["word1"].GetStringValue(),
			Word2:    payload["word2"].GetStringValue(),
			NextWord: payload["next_word"].GetStringValue(),
			Weight:   int(payload["weight"].GetIntegerValue()),
		})
	}
	return transitions, nil
}
