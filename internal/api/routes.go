package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/aeremin/neurodan/internal/markov"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Storage interface {
	UpsertTransition(ctx context.Context, chatID int64, t markov.Transition) error
	GetAllTransitions(ctx context.Context, chatID int64) ([]markov.Transition, error)
	StoreEmbedding(ctx context.Context, chatID int64, text string, vector []float32) error
	FindSimilar(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error)
}

type Engine interface {
	Train(text string) []markov.Transition
	Generate(transitions []markov.Transition, opts markov.GenerateOptions) string
	GenerateIndex(idx *markov.Index, opts markov.GenerateOptions) string
	Stylize(text string, s markov.Styling) string
}

type Embedder interface {
	Embed(text string) ([]float32, error)
}

type Handler struct {
	storage  Storage
	engine   Engine
	embedder Embedder

	// anecdotes is the precomputed joke model backing /anec. When nil, /anec
	// returns 503.
	anecdotes *markov.Index
}

func NewHandler(storage Storage, engine Engine, embedder Embedder) *Handler {
	return &Handler{
		storage:  storage,
		engine:   engine,
		embedder: embedder,
	}
}

// SetAnecdotes enables the /anec endpoint with a precomputed joke model.
func (h *Handler) SetAnecdotes(idx *markov.Index) {
	h.anecdotes = idx
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

	if req.ChatID == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "chat_id is required"})
		return
	}

	for _, t := range h.engine.Train(req.Text) {
		if err := h.storage.UpsertTransition(r.Context(), req.ChatID, t); err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to persist transitions"})
			return
		}
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

	if req.ChatID == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "chat_id is required"})
		return
	}

	if req.Anecdote {
		h.anecdoteHandler(w, req)
		return
	}

	length := resolveLength(req)

	transitions, err := h.storage.GetAllTransitions(r.Context(), req.ChatID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to fetch transitions"})
		return
	}

	var result string
	for attempt := 0; attempt < 5; attempt++ {
		result = h.engine.Generate(transitions, markov.GenerateOptions{
			Seed:    req.Seed,
			Context: req.Context,
			Length:  length,
		})
		if result == "" {
			break
		}

		vec, err := h.embedder.Embed(result)
		if err != nil {
			break
		}

		similar, err := h.storage.FindSimilar(r.Context(), req.ChatID, vec, 0.85)
		if err != nil || len(similar) == 0 {
			break
		}
	}

	styling := markov.DefaultStyling()
	if req.Styling != nil {
		styling = *req.Styling
	}
	writeJSON(w, http.StatusOK, GenerateResponse{Text: h.engine.Stylize(result, styling)})
}

// anecdoteHandler serves a /anec request: it bridges the user's seed into the
// precomputed joke model and generates an unbounded continuation. It never
// touches chat transitions or embeddings. Styling comes straight from the
// request (which the bot uses to leave /anec un-stylized unless the user
// opted in via the "s" flag).
func (h *Handler) anecdoteHandler(w http.ResponseWriter, req GenerateRequest) {
	if h.anecdotes == nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "anecdote corpus not configured"})
		return
	}

	// Preserve the request styling exactly: an omitted Styling means no
	// stylization for anecdotes, never DefaultStyling.
	var styling markov.Styling
	if req.Styling != nil {
		styling = *req.Styling
	}

	result := h.engine.GenerateIndex(h.anecdotes, markov.GenerateOptions{
		Seed:   req.Seed,
		Length: markov.LengthUnbounded,
	})
	writeJSON(w, http.StatusOK, GenerateResponse{Text: h.engine.Stylize(result, styling)})
}

// resolveLength maps a request's length config to a length preset, preserving
// backwards compatibility with the legacy integer length field.
func resolveLength(req GenerateRequest) markov.LengthPreset {
	if req.LengthCfg != "" {
		if p, ok := markov.ParseLengthPreset(req.LengthCfg); ok {
			return p
		}
		log.Printf("unknown length_cfg %q, falling back to medium", req.LengthCfg)
	}
	switch {
	case req.Length > 15:
		return markov.LengthLong
	case req.Length > 0:
		return markov.LengthShort
	default:
		return markov.LengthMedium
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
