package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aeremin/neurodan/internal/anecdote"
	"github.com/aeremin/neurodan/internal/api"
	"github.com/aeremin/neurodan/internal/markov"
	"github.com/stretchr/testify/require"
)

func newAnecdoteIndex(t *testing.T, jokes ...string) *markov.Index {
	t.Helper()
	return anecdote.BuildIndex(jokes)
}

func anecdoteGenerateRequest() *http.Request {
	body, _ := json.Marshal(api.GenerateRequest{Anecdote: true, GenPrefix: "привет мир", ChatID: 1})
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// trackChatStorage fails the test if the chat-backed storage methods are
// called, asserting the anecdote path never touches chat data.
type trackChatStorage struct {
	*mockStorage
	mu          sync.Mutex
	chatTouched bool
}

func (s *trackChatStorage) GetAllTransitions(ctx context.Context, chatID int64) ([]markov.Transition, error) {
	s.mu.Lock()
	s.chatTouched = true
	s.mu.Unlock()
	return nil, nil
}

func (s *trackChatStorage) FindSimilar(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
	s.mu.Lock()
	s.chatTouched = true
	s.mu.Unlock()
	return nil, nil
}

func (s *trackChatStorage) touched() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chatTouched
}

func anecdoteEngine() *mockEngine {
	return &mockEngine{
		trainFn: func(text string) []markov.Transition {
			return []markov.Transition{{Order: 2, Prefix: []string{"a", "b"}, NextWord: "c", Weight: 1}}
		},
		generateFn: func(transitions []markov.Transition, opts markov.GenerateOptions) string {
			return "generated anecdote"
		},
	}
}

func TestGenerateAnecdoteOK(t *testing.T) {
	storage := &trackChatStorage{mockStorage: &mockStorage{}}
	engine := anecdoteEngine()

	handler := api.NewHandler(storage, engine, &mockEmbedder{})
	handler.SetAnecdotes(newAnecdoteIndex(t, "первый анекдот", "второй анекдот"))
	router := api.NewRouter(handler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, anecdoteGenerateRequest())

	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, storage.touched(), "anecdote path must not touch chat storage")

	var resp api.GenerateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "generated anecdote", resp.Text)
}

func TestGenerateAnecdoteUsesPrecomputedIndex(t *testing.T) {
	jokes := []string{"первый анекдот", "второй анекдот", "третий анекдот"}
	idx := newAnecdoteIndex(t, jokes...)

	var gotIdx *markov.Index
	var gotOpts markov.GenerateOptions
	engine := &mockEngine{
		generateIndexFn: func(ix *markov.Index, opts markov.GenerateOptions) string {
			gotIdx = ix
			gotOpts = opts
			return "result"
		},
	}

	handler := api.NewHandler(&mockStorage{}, engine, &mockEmbedder{})
	handler.SetAnecdotes(idx)
	router := api.NewRouter(handler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, anecdoteGenerateRequest())

	require.Equal(t, http.StatusOK, w.Code)
	require.Same(t, idx, gotIdx, "handler must pass the precomputed index through")
	require.Equal(t, "привет мир", gotOpts.GenPrefix)
	require.Equal(t, markov.LengthUnbounded, gotOpts.Length)
}

func TestGenerateAnecdoteNotConfigured(t *testing.T) {
	handler := api.NewHandler(&mockStorage{}, &mockEngine{}, &mockEmbedder{})
	router := api.NewRouter(handler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, anecdoteGenerateRequest())

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestGenerateAnecdoteDefaultsToNoStyling(t *testing.T) {
	engine := anecdoteEngine()

	handler := api.NewHandler(&mockStorage{}, engine, &mockEmbedder{})
	handler.SetAnecdotes(newAnecdoteIndex(t, "один анекдот", "два анекдота"))
	router := api.NewRouter(handler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, anecdoteGenerateRequest())

	require.Equal(t, http.StatusOK, w.Code)
	// The anecdote path must run through Stylize with the zero Styling (all
	// passes off), never DefaultStyling.
	engine.stylizeMu.Lock()
	defer engine.stylizeMu.Unlock()
	require.Len(t, engine.stylings, 1)
	require.Equal(t, markov.Styling{}, engine.stylings[0])
}

func TestAnecdoteRequestJSON(t *testing.T) {
	body, err := json.Marshal(api.GenerateRequest{Anecdote: true, ChatID: 3})
	require.NoError(t, err)
	require.Contains(t, string(body), `"anecdote":true`)

	var req api.GenerateRequest
	require.NoError(t, json.Unmarshal([]byte(`{"anecdote":true,"chat_id":3}`), &req))
	require.True(t, req.Anecdote)
}
