package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/qdrant/go-client/qdrant"
)

// Transition is a single variable-order n-gram step persisted in Qdrant.
// Prefix has length Order.
type Transition struct {
	Order    int
	Prefix   []string
	NextWord string
	Weight   int
}

// markovCollection is the Qdrant collection holding transitions. It was
// bumped to v2 when transitions became variable-order (prefix array + order)
// instead of fixed bigrams.
const markovCollection = "markov_transitions_v2"

func transitionPointID(chatID int64, t Transition) string {
	return fmt.Sprintf("%d\x00%d\x00%s\x00%s",
		chatID, t.Order, strings.Join(t.Prefix, "\x00"), t.NextWord)
}

func transitionFromPayload(payload map[string]*qdrant.Value) Transition {
	t := Transition{
		Order:    int(payload["order"].GetIntegerValue()),
		NextWord: payload["next_word"].GetStringValue(),
		Weight:   int(payload["weight"].GetIntegerValue()),
	}
	// qdrant stores the prefix as an array of strings; a missing/invalid
	// prefix falls back to an empty slice rather than crashing.
	if list := payload["prefix"].GetListValue(); list != nil {
		for _, v := range list.GetValues() {
			t.Prefix = append(t.Prefix, v.GetStringValue())
		}
	}
	if t.Order < 1 || t.Order > 3 {
		t.Order = len(t.Prefix)
	}
	if t.Order < 1 || t.Order > 3 {
		t.Order = 1
	}
	return t
}

func (c *Client) UpsertTransition(ctx context.Context, chatID int64, t Transition) error {
	pointsClient := c.grpcClient.GetPointsClient()

	pointID := transitionPointID(chatID, t)

	existingResp, err := pointsClient.Get(ctx, &qdrant.GetPoints{
		CollectionName: markovCollection,
		Ids:            []*qdrant.PointId{qdrant.NewIDUUID(pointID)},
		WithPayload:    qdrant.NewWithPayloadEnable(true),
	})
	if err != nil {
		return fmt.Errorf("failed to get existing transition: %w", err)
	}

	totalWeight := t.Weight
	existing := existingResp.GetResult()
	if len(existing) > 0 {
		totalWeight += int(existing[0].GetPayload()["weight"].GetIntegerValue())
	}

	prefixValues := make([]*qdrant.Value, 0, len(t.Prefix))
	for _, p := range t.Prefix {
		prefixValues = append(prefixValues, qdrant.NewValueString(p))
	}

	points := []*qdrant.PointStruct{
		{
			Id:      qdrant.NewIDUUID(pointID),
			Vectors: qdrant.NewVectors(0.0),
			Payload: map[string]*qdrant.Value{
				"chat_id":   qdrant.NewValueInt(chatID),
				"order":     qdrant.NewValueInt(int64(t.Order)),
				"prefix":    qdrant.NewValueFromList(prefixValues...),
				"next_word": qdrant.NewValueString(t.NextWord),
				"weight":    qdrant.NewValueInt(int64(totalWeight)),
			},
		},
	}

	_, err = pointsClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: markovCollection,
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
		CollectionName: markovCollection,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchInt("chat_id", chatID),
			},
		},
		WithPayload: qdrant.NewWithPayloadEnable(true),
		Limit:       qdrant.PtrOf(uint32(100000)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all transitions: %w", err)
	}

	transitions := make([]Transition, 0, len(resp.GetResult()))
	for _, point := range resp.GetResult() {
		transitions = append(transitions, transitionFromPayload(point.GetPayload()))
	}
	return transitions, nil
}
