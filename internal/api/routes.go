package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Transition struct {
	Word1    string
	Word2    string
	NextWord string
	Weight   int
}

type Storage interface {
	UpsertTransition(ctx context.Context, chatID int64, word1, word2, nextWord string, weight int) error
	GetAllTransitions(ctx context.Context, chatID int64) ([]Transition, error)
	StoreEmbedding(ctx context.Context, chatID int64, text string, vector []float32) error
	FindSimilar(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error)
}

type Engine interface {
	Train(text string) []Transition
	Generate(transitions []Transition, length int, seed string) string
}

type Embedder interface {
	Embed(text string) ([]float32, error)
}

type Handler struct {
	storage  Storage
	engine   Engine
	embedder Embedder
}

func NewHandler(storage Storage, engine Engine, embedder Embedder) *Handler {
	return &Handler{
		storage:  storage,
		engine:   engine,
		embedder: embedder,
	}
}

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.healthHandler)
	r.Post("/train", h.trainHandler)
	r.Post("/generate", h.generateHandler)

	return r
}

func (h *Handler) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

func (h *Handler) trainHandler(w http.ResponseWriter, r *http.Request) {
	var req TrainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "text is required"})
		return
	}

	transitions := h.engine.Train(req.Text)
	for _, t := range transitions {
		_ = h.storage.UpsertTransition(r.Context(), req.ChatID, t.Word1, t.Word2, t.NextWord, t.Weight)
	}

	vec, err := h.embedder.Embed(req.Text)
	if err == nil {
		_ = h.storage.StoreEmbedding(r.Context(), req.ChatID, req.Text, vec)
	}

	writeJSON(w, http.StatusOK, StatusResponse{Status: "success"})
}

func (h *Handler) generateHandler(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Length <= 0 {
		req.Length = 10
	}

	var result string
	for attempt := 0; attempt < 5; attempt++ {
		transitions, err := h.storage.GetAllTransitions(r.Context(), req.ChatID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to fetch transitions"})
			return
		}

		result = h.engine.Generate(transitions, req.Length, req.Seed)

		vec, err := h.embedder.Embed(result)
		if err != nil {
			break
		}

		similar, err := h.storage.FindSimilar(r.Context(), req.ChatID, vec, 0.85)
		if err != nil || len(similar) == 0 {
			break
		}
	}

	writeJSON(w, http.StatusOK, GenerateResponse{Text: result})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
