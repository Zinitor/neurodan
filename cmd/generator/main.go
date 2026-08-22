package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/aeremin/neurodan/internal/api"
	"github.com/aeremin/neurodan/internal/embedding"
	"github.com/aeremin/neurodan/internal/markov"
	"github.com/aeremin/neurodan/internal/storage"
)

type storageAdapter struct {
	client *storage.Client
}

func (a *storageAdapter) UpsertTransition(ctx context.Context, chatID int64, t markov.Transition) error {
	return a.client.UpsertTransition(ctx, chatID, storage.Transition{
		Order:    t.Order,
		Prefix:   t.Prefix,
		NextWord: t.NextWord,
		Weight:   t.Weight,
	})
}

func (a *storageAdapter) GetAllTransitions(ctx context.Context, chatID int64) ([]markov.Transition, error) {
	txs, err := a.client.GetAllTransitions(ctx, chatID)
	if err != nil {
		return nil, err
	}
	result := make([]markov.Transition, len(txs))
	for i, t := range txs {
		result[i] = markov.Transition{
			Order:    t.Order,
			Prefix:   t.Prefix,
			NextWord: t.NextWord,
			Weight:   t.Weight,
		}
	}
	return result, nil
}

func (a *storageAdapter) StoreEmbedding(ctx context.Context, chatID int64, text string, vector []float32) error {
	return a.client.StoreEmbedding(ctx, chatID, text, vector)
}

func (a *storageAdapter) FindSimilar(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
	return a.client.FindSimilar(ctx, chatID, vector, threshold)
}

func main() {
	qdrantHost := getEnv("QDRANT_HOST", "localhost")
	qdrantPort, _ := strconv.Atoi(getEnv("QDRANT_PORT", "6334"))
	modelPath := getEnv("MODEL_PATH", "models/model.onnx")
	port := getEnv("PORT", "8000")

	client, err := storage.NewClient(qdrantHost, qdrantPort)
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer client.Close()

	if err := client.EnsureCollections(context.Background()); err != nil {
		log.Fatalf("Failed to ensure collections: %v", err)
	}

	embedder, err := embedding.NewEmbedder(modelPath)
	if err != nil {
		log.Fatalf("Failed to load embedding model: %v", err)
	}
	defer embedder.Close()

	engine := markov.NewEngine()

	handler := api.NewHandler(
		&storageAdapter{client: client},
		engine,
		embedder,
	)
	router := api.NewRouter(handler)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Generator server starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
