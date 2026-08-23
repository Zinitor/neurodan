package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aeremin/neurodan/internal/api"
	"github.com/aeremin/neurodan/internal/markov"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	upsertTransitionFn  func(ctx context.Context, chatID int64, t markov.Transition) error
	getAllTransitionsFn func(ctx context.Context, chatID int64) ([]markov.Transition, error)
	storeEmbeddingFn    func(ctx context.Context, chatID int64, text string, vector []float32) error
	findSimilarFn       func(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error)
}

func (m *mockStorage) UpsertTransition(ctx context.Context, chatID int64, t markov.Transition) error {
	if m.upsertTransitionFn != nil {
		return m.upsertTransitionFn(ctx, chatID, t)
	}
	return nil
}

func (m *mockStorage) GetAllTransitions(ctx context.Context, chatID int64) ([]markov.Transition, error) {
	if m.getAllTransitionsFn != nil {
		return m.getAllTransitionsFn(ctx, chatID)
	}
	return nil, nil
}

func (m *mockStorage) StoreEmbedding(ctx context.Context, chatID int64, text string, vector []float32) error {
	if m.storeEmbeddingFn != nil {
		return m.storeEmbeddingFn(ctx, chatID, text, vector)
	}
	return nil
}

func (m *mockStorage) FindSimilar(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
	if m.findSimilarFn != nil {
		return m.findSimilarFn(ctx, chatID, vector, threshold)
	}
	return nil, nil
}

type mockEngine struct {
	trainFn    func(text string) []markov.Transition
	generateFn func(transitions []markov.Transition, opts markov.GenerateOptions) string
	stylizeFn  func(text string) string

	generateIndexFn func(idx *markov.Index, opts markov.GenerateOptions) string

	stylizeMu sync.Mutex
	stylings  []markov.Styling
}

func (m *mockEngine) Train(text string) []markov.Transition {
	if m.trainFn != nil {
		return m.trainFn(text)
	}
	return nil
}

func (m *mockEngine) Generate(transitions []markov.Transition, opts markov.GenerateOptions) string {
	if m.generateFn != nil {
		return m.generateFn(transitions, opts)
	}
	return "generated text"
}

func (m *mockEngine) GenerateIndex(idx *markov.Index, opts markov.GenerateOptions) string {
	if m.generateIndexFn != nil {
		return m.generateIndexFn(idx, opts)
	}
	if m.generateFn != nil {
		return m.generateFn(nil, opts)
	}
	return "generated anecdote"
}

func (m *mockEngine) Stylize(text string, s markov.Styling) string {
	m.stylizeMu.Lock()
	m.stylings = append(m.stylings, s)
	m.stylizeMu.Unlock()
	if m.stylizeFn != nil {
		return m.stylizeFn(text)
	}
	return text
}

type mockEmbedder struct {
	embedFn func(text string) ([]float32, error)
}

func (m *mockEmbedder) Embed(text string) ([]float32, error) {
	if m.embedFn != nil {
		return m.embedFn(text)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func TestTrainRequestJSON(t *testing.T) {
	req := api.TrainRequest{Text: "hello world", ChatID: 12345}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	require.Contains(t, string(data), "hello world")
	require.Contains(t, string(data), "12345")
}

func TestGenerateRequestJSON(t *testing.T) {
	req := api.GenerateRequest{Length: 10, Seed: "hello", ChatID: 12345}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	require.Contains(t, string(data), "hello")
	require.Contains(t, string(data), "12345")
}

func TestGenerateRequestOmitEmptySeed(t *testing.T) {
	req := api.GenerateRequest{Length: 10, ChatID: 12345}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	require.NotContains(t, string(data), "seed")
}

func TestGenerateResponseJSON(t *testing.T) {
	resp := api.GenerateResponse{Text: "generated text"}
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(data), "generated text")
}

func TestStatusResponseJSON(t *testing.T) {
	resp := api.StatusResponse{Status: "ok"}
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(data), "ok")
}

func TestErrorResponseJSON(t *testing.T) {
	resp := api.ErrorResponse{Error: "something went wrong"}
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(data), "something went wrong")
}

func TestHealthHandler(t *testing.T) {
	handler := api.NewHandler(&mockStorage{}, &mockEngine{}, &mockEmbedder{})
	router := api.NewRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp api.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Status)
}

func TestTrainHandler(t *testing.T) {
	var upserted []markov.Transition
	storage := &mockStorage{
		upsertTransitionFn: func(ctx context.Context, chatID int64, tr markov.Transition) error {
			require.Equal(t, int64(42), chatID)
			upserted = append(upserted, tr)
			return nil
		},
	}
	engine := &mockEngine{
		trainFn: func(text string) []markov.Transition {
			return []markov.Transition{
				{Order: 2, Prefix: []string{"hello", "beautiful"}, NextWord: "world", Weight: 1},
				{Order: 2, Prefix: []string{"beautiful", "world"}, NextWord: "today", Weight: 1},
			}
		},
	}
	embedder := &mockEmbedder{}

	handler := api.NewHandler(storage, engine, embedder)
	router := api.NewRouter(handler)

	body, _ := json.Marshal(api.TrainRequest{Text: "hello beautiful world today", ChatID: 42})
	req := httptest.NewRequest(http.MethodPost, "/train", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, upserted, 2)
	require.Equal(t, "hello", upserted[0].Prefix[0])
	require.Equal(t, "today", upserted[1].NextWord)

	var resp api.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
}

func TestTrainHandlerEmptyText(t *testing.T) {
	handler := api.NewHandler(&mockStorage{}, &mockEngine{}, &mockEmbedder{})
	router := api.NewRouter(handler)

	body, _ := json.Marshal(api.TrainRequest{Text: "", ChatID: 42})
	req := httptest.NewRequest(http.MethodPost, "/train", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, "text is required", resp.Error)
}

func TestTrainHandlerInvalidJSON(t *testing.T) {
	handler := api.NewHandler(&mockStorage{}, &mockEngine{}, &mockEmbedder{})
	router := api.NewRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/train", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGenerateHandler(t *testing.T) {
	var receivedTransitions []markov.Transition
	storage := &mockStorage{
		getAllTransitionsFn: func(ctx context.Context, chatID int64) ([]markov.Transition, error) {
			require.Equal(t, int64(99), chatID)
			return []markov.Transition{
				{Order: 2, Prefix: []string{"a", "b"}, NextWord: "c", Weight: 1},
			}, nil
		},
		findSimilarFn: func(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
			return nil, nil
		},
	}
	engine := &mockEngine{
		generateFn: func(transitions []markov.Transition, opts markov.GenerateOptions) string {
			receivedTransitions = transitions
			require.Equal(t, markov.LengthLong, opts.Length)
			require.Equal(t, "seed", opts.Seed)
			return "a b c"
		},
	}
	embedder := &mockEmbedder{}

	handler := api.NewHandler(storage, engine, embedder)
	router := api.NewRouter(handler)

	body, _ := json.Marshal(api.GenerateRequest{LengthCfg: "long", Seed: "seed", ChatID: 99})
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, receivedTransitions, 1)
	require.Equal(t, "a", receivedTransitions[0].Prefix[0])

	var resp api.GenerateResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, "a b c", resp.Text)
}

func TestGenerateHandlerForwardsContext(t *testing.T) {
	engine := &mockEngine{
		generateFn: func(transitions []markov.Transition, opts markov.GenerateOptions) string {
			require.Equal(t, "last fifteen messages", opts.Context)
			require.Empty(t, opts.Seed)
			return "result"
		},
	}
	storage := &mockStorage{
		findSimilarFn: func(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
			return nil, nil
		},
	}

	handler := api.NewHandler(storage, engine, &mockEmbedder{})
	router := api.NewRouter(handler)

	body, _ := json.Marshal(api.GenerateRequest{Context: "last fifteen messages", ChatID: 1})
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestGenerateHandlerDefaultLength(t *testing.T) {
	var gotLength markov.LengthPreset
	engine := &mockEngine{
		generateFn: func(transitions []markov.Transition, opts markov.GenerateOptions) string {
			gotLength = opts.Length
			return "result"
		},
	}
	storage := &mockStorage{
		findSimilarFn: func(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
			return nil, nil
		},
	}

	handler := api.NewHandler(storage, engine, &mockEmbedder{})
	router := api.NewRouter(handler)

	body, _ := json.Marshal(api.GenerateRequest{ChatID: 1})
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, markov.LengthMedium, gotLength)
}

func TestGenerateHandlerInvalidJSON(t *testing.T) {
	handler := api.NewHandler(&mockStorage{}, &mockEngine{}, &mockEmbedder{})
	router := api.NewRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGenerateHandlerStorageError(t *testing.T) {
	storage := &mockStorage{
		getAllTransitionsFn: func(ctx context.Context, chatID int64) ([]markov.Transition, error) {
			return nil, errors.New("storage error")
		},
	}
	handler := api.NewHandler(storage, &mockEngine{}, &mockEmbedder{})
	router := api.NewRouter(handler)

	body, _ := json.Marshal(api.GenerateRequest{Length: 5, ChatID: 1})
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGenerateHandlerRetriesOnSimilar(t *testing.T) {
	attempts := 0
	storage := &mockStorage{
		getAllTransitionsFn: func(ctx context.Context, chatID int64) ([]markov.Transition, error) {
			return []markov.Transition{{Order: 2, Prefix: []string{"a", "b"}, NextWord: "c", Weight: 1}}, nil
		},
		findSimilarFn: func(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
			attempts++
			if attempts < 3 {
				return []string{"similar text"}, nil
			}
			return nil, nil
		},
	}
	engine := &mockEngine{
		generateFn: func(transitions []markov.Transition, opts markov.GenerateOptions) string {
			return "generated"
		},
	}

	handler := api.NewHandler(storage, engine, &mockEmbedder{})
	router := api.NewRouter(handler)

	body, _ := json.Marshal(api.GenerateRequest{Length: 5, ChatID: 1})
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 3, attempts)
}
