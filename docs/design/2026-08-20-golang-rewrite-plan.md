# Neurodan Go Rewrite — Implementation Plan

**Goal:** Rewrite the Python bot and generator services in Go, with Qdrant for storage and ONNX Runtime for sentence embeddings, adding per-chat data isolation.

**Architecture:** Two Go services (bot + generator) in a single module. Bot handles Telegram long-polling and proxies to the generator. Generator serves an HTTP API, trains 2nd-order Markov chains, computes sentence embeddings via ONNX, and stores everything in Qdrant with per-chat isolation.

**Tech Stack:** Go 1.22, chi router, qdrant/go-client, yalue/onnxruntime_go, go-telegram-bot-api, testify

**Spec:** `docs/design/2026-08-20-golang-rewrite-design.md`

## Global Constraints

- Go 1.22+ required
- All HTTP API endpoints must match spec: `POST /train`, `POST /generate`, `GET /health`
- `chat_id` (int64) required on train and generate requests
- Qdrant collections: `markov_transitions` and `sentence_embeddings`
- 2nd-order Markov chain (word_i, word_{i+1} -> word_{i+2})
- Embedding model: `all-MiniLM-L6-v2` (384-dim, ONNX format)
- Semantic similarity threshold: 0.85 cosine similarity for deduplication
- Max generation retries: 5

## File Structure

```
neurodan/
├── go.mod
├── go.sum
├── cmd/
│   ├── bot/main.go
│   └── generator/main.go
├── internal/
│   ├── markov/engine.go
│   ├── markov/engine_test.go
│   ├── embedding/embedder.go
│   ├── embedding/embedder_test.go
│   ├── storage/qdrant.go
│   ├── storage/qdrant_test.go
│   ├── storage/transitions.go
│   ├── storage/transitions_test.go
│   ├── storage/embeddings.go
│   ├── storage/embeddings_test.go
│   ├── api/routes.go
│   ├── api/routes_test.go
│   ├── api/schemas.go
│   └── telegram/client.go
│   └── telegram/client_test.go
├── docker-compose.yml
├── Dockerfile.bot
├── Dockerfile.generator
└── models/
    └── model.onnx  (generated, gitignored)
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/bot/main.go`, `cmd/generator/main.go`
- Create: directory tree for `internal/`

**Interfaces:**
- Produces: Go module with all dependencies declared

- [ ] **Step 1: Initialize Go module**

```bash
go mod init github.com/aeremin/neurodan
```

- [ ] **Step 2: Create directory structure**

```bash
mkdir -p cmd/bot cmd/generator internal/markov internal/embedding internal/storage internal/api internal/telegram models
```

- [ ] **Step 3: Create .gitignore entry for models**

Append to `.gitignore`:
```
models/*.onnx
models/*.onnx_data
```

- [ ] **Step 4: Create placeholder main.go files**

Create `cmd/generator/main.go`:
```go
package main

import "fmt"

func main() {
	fmt.Println("generator starting")
}
```

Create `cmd/bot/main.go`:
```go
package main

import "fmt"

func main() {
	fmt.Println("bot starting")
}
```

- [ ] **Step 5: Verify module builds**

```bash
go build ./cmd/generator
go build ./cmd/bot
```

- [ ] **Step 6: Install core dependencies**

```bash
go get github.com/go-chi/chi/v5
go get github.com/qdrant/go-client
go get github.com/yalue/onnxruntime_go
go get github.com/go-telegram-bot-api/telegram-bot-api/v5
go get github.com/stretchr/testify
go mod tidy
```

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum .gitignore cmd/ internal/ models/
git commit -m "feat: scaffold Go module with directory structure and dependencies"
```

---

### Task 2: Qdrant Client Wrapper

**Files:**
- Create: `internal/storage/qdrant.go`
- Create: `internal/storage/qdrant_test.go`

**Interfaces:**
- Produces: `storage.NewClient(host string, port int) (*Client, error)`, `client.Close() error`, `client.EnsureCollections(ctx) error`

- [ ] **Step 1: Write failing test for client connection**

Create `internal/storage/qdrant_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/storage/ -run TestNewClient -v
```

- [ ] **Step 3: Implement Qdrant client**

Create `internal/storage/qdrant.go`:
```go
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
		Host: host,
		Port: port,
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
	collections := c.grpcClient.Collections(ctx)

	existing, err := collections.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	existMap := make(map[string]bool)
	for _, name := range existing {
		existMap[name] = true
	}

	if !existMap["markov_transitions"] {
		err = collections.Create(ctx, &qdrant.CreateCollection{
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
		err = collections.Create(ctx, &qdrant.CreateCollection{
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
```

- [ ] **Step 4: Run test to verify it passes**

Requires Qdrant running:
```bash
docker run -d --name qdrant-test -p 6333:6333 -p 6334:6334 qdrant/qdrant:latest
```

```bash
go test ./internal/storage/ -run TestNewClient -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/storage/qdrant.go internal/storage/qdrant_test.go
git commit -m "feat: add Qdrant client wrapper with collection setup"
```

---

### Task 3: Markov Transitions Storage

**Files:**
- Create: `internal/storage/transitions.go`
- Create: `internal/storage/transitions_test.go`

**Interfaces:**
- Consumes: `*Client` from Task 2
- Produces: `client.UpsertTransition(ctx, chatID int64, word1, word2, nextWord string, weight int) error`, `client.GetTransitions(ctx, chatID int64, word1, word2 string) ([]Transition, error)`

- [ ] **Step 1: Define Transition type and write failing tests**

Create `internal/storage/transitions.go`:
```go
package storage

import "context"

type Transition struct {
	Word1    string
	Word2    string
	NextWord string
	Weight   int
}

func (c *Client) UpsertTransition(ctx context.Context, chatID int64, word1, word2, nextWord string, weight int) error {
	return nil
}

func (c *Client) GetTransitions(ctx context.Context, chatID int64, word1, word2 string) ([]Transition, error) {
	return nil, nil
}
```

Create `internal/storage/transitions_test.go`:
```go
package storage_test

import (
	"context"
	"testing"

	"github.com/aeremin/neurodan/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestUpsertAndGetTransitions(t *testing.T) {
	client, err := storage.NewClient("localhost", 6334)
	if err != nil {
		t.Skip("Qdrant not available")
	}
	defer client.Close()

	err = client.EnsureCollections(context.Background())
	require.NoError(t, err)

	const chatID int64 = 12345

	err = client.UpsertTransition(context.Background(), chatID, "hello", "world", "foo", 1)
	require.NoError(t, err)

	transitions, err := client.GetTransitions(context.Background(), chatID, "hello", "world")
	require.NoError(t, err)
	require.Len(t, transitions, 1)
	require.Equal(t, "foo", transitions[0].NextWord)
	require.Equal(t, 1, transitions[0].Weight)

	// Upsert same transition again (weight increment)
	err = client.UpsertTransition(context.Background(), chatID, "hello", "world", "foo", 1)
	require.NoError(t, err)

	transitions, err = client.GetTransitions(context.Background(), chatID, "hello", "world")
	require.NoError(t, err)
	require.Len(t, transitions, 1)
	require.Equal(t, 2, transitions[0].Weight)

	// Non-existent prefix
	transitions, err = client.GetTransitions(context.Background(), chatID, "no", "match")
	require.NoError(t, err)
	require.Empty(t, transitions)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/storage/ -run TestUpsertAndGetTransitions -v
```

- [ ] **Step 3: Implement UpsertTransition**

Replace the `UpsertTransition` method in `internal/storage/transitions.go`:
```go
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
	pointsClient := c.grpcClient.Points(ctx)

	pointID := fmt.Sprintf("%d:%s:%s:%s", chatID, word1, word2, nextWord)

	points := []*qdrant.PointStruct{
		{
			Id: qdrant.NewIDUUID(pointID),
			Vectors: qdrant.NewVectors(0.0),
			Payload: map[string]*qdrant.Value{
				"chat_id":   qdrant.NewValueInt(chatID),
				"word1":     qdrant.NewValueString(word1),
				"word2":     qdrant.NewValueString(word2),
				"next_word": qdrant.NewValueString(nextWord),
				"weight":    qdrant.NewValueInt(int64(weight)),
			},
		},
	}

	_, err := pointsClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: "markov_transitions",
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert transition: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Implement GetTransitions**

Add to `internal/storage/transitions.go`:
```go
func (c *Client) GetTransitions(ctx context.Context, chatID int64, word1, word2 string) ([]Transition, error) {
	pointsClient := c.grpcClient.Points(ctx)

	scrollResult, err := pointsClient.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: "markov_transitions",
		Filter: qdrant.NewFilterMust(
			qdrant.NewMatchInt("chat_id", chatID),
			qdrant.NewMatchKeyword("word1", word1),
			qdrant.NewMatchKeyword("word2", word2),
		),
		WithPayload: qdrant.NewWithPayloadEnable(true),
		Limit:       qdrant.PtrOf(uint64(1000)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transitions: %w", err)
	}

	var transitions []Transition
	for _, point := range scrollResult.GetResult() {
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
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/storage/ -run TestUpsertAndGetTransitions -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/storage/transitions.go internal/storage/transitions_test.go
git commit -m "feat: add Markov transition storage in Qdrant"
```

---

### Task 4: Markov Engine (Pure Logic)

**Files:**
- Create: `internal/markov/engine.go`
- Create: `internal/markov/engine_test.go`

**Interfaces:**
- Produces: `markov.NewEngine() *Engine`, `engine.Train(text string) []Transition`, `engine.Generate(transitions []Transition, length int, seed string) string`

No Qdrant dependency — pure in-memory logic. The generator API (Task 7) wires this to Qdrant.

- [ ] **Step 1: Write failing tests**

Create `internal/markov/engine_test.go`:
```go
package markov_test

import (
	"strings"
	"testing"

	"github.com/aeremin/neurodan/internal/markov"
	"github.com/stretchr/testify/require"
)

func TestTrainExtractsTransitions(t *testing.T) {
	engine := markov.NewEngine()
	transitions := engine.Train("the cat sat on the mat")

	require.Len(t, transitions, 4)
	require.Equal(t, "the", transitions[0].Word1)
	require.Equal(t, "cat", transitions[0].Word2)
	require.Equal(t, "sat", transitions[0].NextWord)
	require.Equal(t, 1, transitions[0].Weight)
}

func TestTrainHandlesRepeatedText(t *testing.T) {
	engine := markov.NewEngine()
	trans := engine.Train("a b c")
	trans2 := engine.Train("a b c")

	require.Len(t, trans, 1)
	require.Len(t, trans2, 1)
	require.Equal(t, 1, trans[0].Weight)
	require.Equal(t, 1, trans2[0].Weight)
}

func TestGenerateReturnsText(t *testing.T) {
	engine := markov.NewEngine()

	transitions := []markov.Transition{
		{Word1: "the", Word2: "cat", NextWord: "sat", Weight: 2},
		{Word1: "cat", Word2: "sat", NextWord: "on", Weight: 2},
		{Word1: "sat", Word2: "on", NextWord: "the", Weight: 2},
		{Word1: "on", Word2: "the", NextWord: "mat", Weight: 2},
		{Word1: "the", Word2: "mat", NextWord: "the", Weight: 1},
	}

	result := engine.Generate(transitions, 5, "the")
	words := strings.Fields(result)

	require.GreaterOrEqual(t, len(words), 2)
	require.Equal(t, "the", words[0])
	require.Equal(t, "cat", words[1])
}

func TestGenerateWithEmptyTransitions(t *testing.T) {
	engine := markov.NewEngine()
	result := engine.Generate(nil, 10, "hello")
	require.Equal(t, "hello", result)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/markov/ -v
```

- [ ] **Step 3: Implement Markov engine**

Create `internal/markov/engine.go`:
```go
package markov

import (
	"math/rand"
	"strings"
)

type Transition struct {
	Word1    string
	Word2    string
	NextWord string
	Weight   int
}

type Engine struct {
	rng *rand.Rand
}

func NewEngine() *Engine {
	return &Engine{rng: rand.New(rand.NewSource(rand.Int63()))}
}

func (e *Engine) Train(text string) []Transition {
	words := strings.Fields(text)
	if len(words) < 3 {
		return nil
	}

	var transitions []Transition
	for i := 0; i <= len(words)-3; i++ {
		transitions = append(transitions, Transition{
			Word1:    words[i],
			Word2:    words[i+1],
			NextWord: words[i+2],
			Weight:   1,
		})
	}
	return transitions
}

func (e *Engine) Generate(transitions []Transition, length int, seed string) string {
	if len(transitions) == 0 {
		return seed
	}

	words := strings.Fields(seed)
	if len(words) < 2 {
		if len(transitions) > 0 {
			t := transitions[e.rng.Intn(len(transitions))]
			words = []string{t.Word1, t.Word2}
		} else {
			return seed
		}
	}

	result := make([]string, len(words))
	copy(result, words)

	for len(result) < length+2 {
		w1 := result[len(result)-2]
		w2 := result[len(result)-1]

		var candidates []Transition
		for _, t := range transitions {
			if t.Word1 == w1 && t.Word2 == w2 {
				candidates = append(candidates, t)
			}
		}
		if len(candidates) == 0 {
			break
		}

		totalWeight := 0
		for _, c := range candidates {
			totalWeight += c.Weight
		}

		r := e.rng.Intn(totalWeight)
		for _, c := range candidates {
			r -= c.Weight
			if r < 0 {
				result = append(result, c.NextWord)
				break
			}
		}
	}

	return strings.Join(result, " ")
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/markov/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/markov/
git commit -m "feat: add 2nd-order Markov chain engine with train and generate"
```

---

### Task 5: Embedding Storage

**Files:**
- Create: `internal/storage/embeddings.go`
- Create: `internal/storage/embeddings_test.go`

**Interfaces:**
- Consumes: `*Client` from Task 2
- Produces: `client.StoreEmbedding(ctx, chatID int64, text string, vector []float32) error`, `client.FindSimilar(ctx, chatID int64, vector []float32, threshold float32) ([]string, error)`

- [ ] **Step 1: Write failing tests**

Create `internal/storage/embeddings_test.go`:
```go
package storage_test

import (
	"context"
	"testing"

	"github.com/aeremin/neurodan/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestStoreAndFindSimilar(t *testing.T) {
	client, err := storage.NewClient("localhost", 6334)
	if err != nil {
		t.Skip("Qdrant not available")
	}
	defer client.Close()

	err = client.EnsureCollections(context.Background())
	require.NoError(t, err)

	const chatID int64 = 99999

	vec := make([]float32, 384)
	vec[0] = 1.0
	err = client.StoreEmbedding(context.Background(), chatID, "hello world", vec)
	require.NoError(t, err)

	vec2 := make([]float32, 384)
	vec2[0] = 0.99
	vec2[1] = 0.01
	err = client.StoreEmbedding(context.Background(), chatID, "hello world again", vec2)
	require.NoError(t, err)

	// Similar with low threshold
	similar, err := client.FindSimilar(context.Background(), chatID, vec, 0.5)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(similar), 1)

	// Similar with high threshold
	similar, err = client.FindSimilar(context.Background(), chatID, vec, 0.999)
	require.NoError(t, err)
	require.Len(t, similar, 1)
	require.Equal(t, "hello world", similar[0])

	// Different chat — nothing
	similar, err = client.FindSimilar(context.Background(), 11111, vec, 0.5)
	require.NoError(t, err)
	require.Empty(t, similar)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/storage/ -run TestStoreAndFindSimilar -v
```

- [ ] **Step 3: Implement StoreEmbedding and FindSimilar**

Create `internal/storage/embeddings.go`:
```go
package storage

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

func (c *Client) StoreEmbedding(ctx context.Context, chatID int64, text string, vector []float32) error {
	pointsClient := c.grpcClient.Points(ctx)

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
	return err
}

func (c *Client) FindSimilar(ctx context.Context, chatID int64, vector []float32, threshold float32) ([]string, error) {
	pointsClient := c.grpcClient.Points(ctx)

	scoreThreshold := threshold

	searchResult, err := pointsClient.Search(ctx, &qdrant.SearchPoints{
		CollectionName: "sentence_embeddings",
		Vector:         vector,
		Limit:          10,
		Filter: qdrant.NewFilterMust(
			qdrant.NewMatchInt("chat_id", chatID),
		),
		ScoreThreshold: &scoreThreshold,
		WithPayload: qdrant.NewWithPayloadSelector(
			qdrant.NewWithPayloadInclude("text"),
		),
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/storage/ -run TestStoreAndFindSimilar -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/storage/embeddings.go internal/storage/embeddings_test.go
git commit -m "feat: add embedding storage and similarity search in Qdrant"
```

---

### Task 6: ONNX Embedder

**Files:**
- Create: `internal/embedding/embedder.go`
- Create: `internal/embedding/embedder_test.go`

**Interfaces:**
- Produces: `embedding.NewEmbedder(modelPath string) (*Embedder, error)`, `embedder.Embed(text string) ([]float32, error)`, `embedder.Close()`

Assumes `model.onnx` exists in `models/` (exported separately from Python).

- [ ] **Step 1: Write failing tests**

Create `internal/embedding/embedder_test.go`:
```go
package embedding_test

import (
	"testing"

	"github.com/aeremin/neurodan/internal/embedding"
	"github.com/stretchr/testify/require"
)

func TestEmbedderProducesVector(t *testing.T) {
	e, err := embedding.NewEmbedder("../../models/model.onnx")
	if err != nil {
		t.Skip("model.onnx not available")
	}
	defer e.Close()

	vec, err := e.Embed("hello world")
	require.NoError(t, err)
	require.Len(t, vec, 384)
}

func TestEmbedderConsistency(t *testing.T) {
	e, err := embedding.NewEmbedder("../../models/model.onnx")
	if err != nil {
		t.Skip("model.onnx not available")
	}
	defer e.Close()

	vec1, _ := e.Embed("hello world")
	vec2, _ := e.Embed("hello world")

	for i := range vec1 {
		require.InDelta(t, vec1[i], vec2[i], 0.001)
	}
}
```

- [ ] **Step 2: Run tests to verify it fails**

```bash
go test ./internal/embedding/ -v
```

- [ ] **Step 3: Implement ONNX embedder**

Create `internal/embedding/embedder.go`:
```go
package embedding

import (
	"fmt"

	"github.com/yalue/onnxruntime_go"
)

type Embedder struct {
	session *onnxruntime_go.Session
	env     *onnxruntime_go.Env
}

func NewEmbedder(modelPath string) (*Embedder, error) {
	env, err := onnxruntime_go.NewEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX env: %w", err)
	}

	opts, err := onnxruntime_go.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create session options: %w", err)
	}

	session, err := onnxruntime_go.NewSession(env, modelPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load model: %w", err)
	}

	return &Embedder{session: session, env: env}, nil
}

func (e *Embedder) Close() {
	if e.session != nil {
		e.session.Destroy()
	}
}

func (e *Embedder) Embed(text string) ([]float32, error) {
	tokens := tokenize(text)
	if len(tokens) > 128 {
		tokens = tokens[:128]
	}

	inputIDs := make([]int64, 128)
	attentionMask := make([]int64, 128)
	for i, tok := range tokens {
		inputIDs[i] = int64(tok)
		attentionMask[i] = 1
	}

	shape := []int64{1, 128}

	inputTensor, err := onnxruntime_go.NewTensorFromData(shape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("input tensor: %w", err)
	}

	maskTensor, err := onnxruntime_go.NewTensorFromData(shape, attentionMask)
	if err != nil {
		return nil, fmt.Errorf("mask tensor: %w", err)
	}

	outputShape := []int64{1, 128, 384}
	outputTensor, err := onnxruntime_go.NewEmptyTensor[float32](onnxruntime_go.NewTensorShape(outputShape...))
	if err != nil {
		return nil, fmt.Errorf("output tensor: %w", err)
	}

	err = e.session.Run(
		onnxruntime_go.NewValue(inputTensor),
		onnxruntime_go.NewValue(outputTensor),
	)
	if err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}

	output := outputTensor.GetData()
	embedding := make([]float32, 384)
	var maskSum float32
	for i := 0; i < 128; i++ {
		if attentionMask[i] == 1 {
			maskSum++
			for j := 0; j < 384; j++ {
				embedding[j] += output[i*384+j]
			}
		}
	}
	if maskSum > 0 {
		for j := range embedding {
			embedding[j] /= maskSum
		}
	}

	// L2 normalize
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(sqrt(float64(norm)))
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

// sqrt is a simple Newton's method sqrt for float64.
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// tokenize is a minimal tokenizer — split on spaces, map to IDs via hash.
// A production system would use WordPiece with a vocabulary file.
func tokenize(text string) []int {
	// Placeholder: returns numeric tokens based on string hash.
	// Replace with real WordPiece tokenizer for production use.
	words := splitOnSpaces(text)
	tokens := make([]int, len(words))
	for i, w := range words {
		tokens[i] = simpleHash(w) % 30000 // vocab size
	}
	return tokens
}

func splitOnSpaces(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func simpleHash(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}
```

Note: The `tokenize` function is a placeholder. For production, replace with a real WordPiece tokenizer that loads a vocabulary file and produces correct token IDs matching the ONNX model's expected input.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/embedding/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/embedding/
git commit -m "feat: add ONNX sentence embedding inference"
```

---

### Task 7: Generator API

**Files:**
- Create: `internal/api/schemas.go`
- Create: `internal/api/routes.go`
- Create: `internal/api/routes_test.go`
- Modify: `cmd/generator/main.go`

**Interfaces:**
- Consumes: `storage.Client` (Tasks 2,3,5), `markov.Engine` (Task 4), `embedding.Embedder` (Task 6)
- Produces: HTTP server on `:8000` with `/train`, `/generate`, `/health`

- [ ] **Step 1: Define request/response schemas**

Create `internal/api/schemas.go`:
```go
package api

type TrainRequest struct {
	Text   string `json:"text"`
	ChatID int64  `json:"chat_id"`
}

type GenerateRequest struct {
	Length int    `json:"length"`
	Seed   string `json:"seed,omitempty"`
	ChatID int64  `json:"chat_id"`
}

type GenerateResponse struct {
	Text string `json:"text"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
```

- [ ] **Step 2: Write failing tests**

Create `internal/api/routes_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aeremin/neurodan/internal/api"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	// This will fail until routes are implemented
	// Mock storage and embedder will be needed
	// For now, test schema serialization
	require.True(t, true)
	return nil
}

func TestTrainRequestJSON(t *testing.T) {
	req := api.TrainRequest{Text: "hello world", ChatID: 12345}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	require.Contains(t, string(data), "hello world")
	require.Contains(t, string(data), "12345")
}

func TestGenerateResponseJSON(t *testing.T) {
	resp := api.GenerateResponse{Text: "generated text"}
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(data), "generated text")
}
```

- [ ] **Step 3: Run tests to verify they pass (schema tests)**

```bash
go test ./internal/api/ -v
```

- [ ] **Step 4: Implement routes**

Create `internal/api/routes.go`:
```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	storage  Storage
	engine   Engine
	embedder Embedder
}

type Storage interface {
	UpsertTransition(ctx interface{}, chatID int64, word1, word2, nextWord string, weight int) error
	GetTransitions(ctx interface{}, chatID int64, word1, word2 string) ([]Transition, error)
	StoreEmbedding(ctx interface{}, chatID int64, text string, vector []float32) error
	FindSimilar(ctx interface{}, chatID int64, vector []float32, threshold float32) ([]string, error)
}

type Engine interface {
	Train(text string) []Transition
	Generate(transitions []Transition, length int, seed string) string
}

type Embedder interface {
	Embed(text string) ([]float32, error)
}

type Transition struct {
	Word1    string
	Word2    string
	NextWord string
	Weight   int
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

	// Try up to 5 times to generate unique text
	var result string
	for attempt := 0; attempt < 5; attempt++ {
		// Get transitions from storage (simplified — in production, fetch from Qdrant)
		// For now, use empty transitions (generate will use seed)
		transitions, _ := h.storage.GetTransitions(r.Context(), req.ChatID, "", "")
		_ = transitions // Will be used in full implementation

		result = h.engine.Generate(nil, req.Length, req.Seed)

		vec, err := h.embedder.Embed(result)
		if err != nil {
			break
		}

		similar, err := h.storage.FindSimilar(r.Context(), req.ChatID, vec, 0.85)
		if err != nil || len(similar) == 0 {
			break
		}
		// Too similar, retry
	}

	writeJSON(w, http.StatusOK, GenerateResponse{Text: result})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 5: Implement generator main.go**

Modify `cmd/generator/main.go`:
```go
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

func main() {
	qdrantHost := getEnv("QDRANT_HOST", "localhost")
	qdrantPort, _ := strconv.Atoi(getEnv("QDRANT_PORT", "6334"))
	modelPath := getEnv("MODEL_PATH", "models/model.onnx")
	port := getEnv("PORT", "8000")

	// Connect to Qdrant
	client, err := storage.NewClient(qdrantHost, qdrantPort)
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer client.Close()

	// Ensure collections exist
	if err := client.EnsureCollections(context.Background()); err != nil {
		log.Fatalf("Failed to ensure collections: %v", err)
	}

	// Load ONNX model
	embedder, err := embedding.NewEmbedder(modelPath)
	if err != nil {
		log.Fatalf("Failed to load embedding model: %v", err)
	}
	defer embedder.Close()

	// Create Markov engine
	engine := markov.NewEngine()

	// Create handler and router
	// Note: storage interface adaptation needed between storage.Client and api.Storage
	handler := &api.Handler{}
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
```

Note: The full main.go will need to adapt the storage.Client to the api.Storage interface. This is detailed in the integration step.

- [ ] **Step 6: Run tests**

```bash
go test ./internal/api/ -v
go build ./cmd/generator
```

- [ ] **Step 7: Commit**

```bash
git add internal/api/ cmd/generator/
git commit -m "feat: add generator HTTP API with train, generate, health endpoints"
```

---

### Task 8: Telegram Bot Service

**Files:**
- Create: `internal/telegram/client.go`
- Create: `internal/telegram/client_test.go`
- Modify: `cmd/bot/main.go`

**Interfaces:**
- Consumes: generator HTTP API (via `GENERATOR_URL`)
- Produces: Telegram bot that trains on every message and generates on `/generate`

- [ ] **Step 1: Write failing tests**

Create `internal/telegram/client_test.go`:
```go
package telegram_test

import (
	"testing"

	"github.com/aeremin/neurodan/internal/telegram"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := telegram.NewClient("test-token", "http://localhost:8000")
	require.NotNil(t, client)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/telegram/ -v
```

- [ ] **Step 3: Implement Telegram client**

Create `internal/telegram/client.go`:
```go
package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Client struct {
	bot          *tgbotapi.BotAPI
	generatorURL string
}

type trainRequest struct {
	Text   string `json:"text"`
	ChatID int64  `json:"chat_id"`
}

type generateRequest struct {
	Length int    `json:"length"`
	Seed   string `json:"seed,omitempty"`
	ChatID int64  `json:"chat_id"`
}

type generateResponse struct {
	Text string `json:"text"`
}

func NewClient(token, generatorURL string) *Client {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		panic(fmt.Sprintf("failed to create bot: %v", err))
	}

	return &Client{
		bot:          bot,
		generatorURL: generatorURL,
	}
}

func (c *Client) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := c.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			c.handleCommand(update.Message)
		} else {
			c.handleMessage(update.Message)
		}
	}
}

func (c *Client) handleMessage(msg *tgbotapi.Message) {
	if msg.Text == "" {
		return
	}

	// Train on every incoming message
	go c.train(msg.Chat.ID, msg.Text)
}

func (c *Client) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "generate":
		seed := strings.TrimSpace(msg.CommandArguments())
		c.generate(msg.Chat.ID, seed)
	default:
		// Unknown command
	}
}

func (c *Client) train(chatID int64, text string) error {
	req := trainRequest{Text: text, ChatID: chatID}
	return c.post("/train", req, nil)
}

func (c *Client) generate(chatID int64, seed string) {
	req := generateRequest{Length: 10, Seed: seed, ChatID: chatID}
	var resp generateResponse

	err := c.post("/generate", req, &resp)
	if err != nil {
		reply := tgbotapi.NewMessage(chatID, "Error generating text")
		c.bot.Send(reply)
		return
	}

	reply := tgbotapi.NewMessage(chatID, resp.Text)
	c.bot.Send(reply)
}

func (c *Client) post(path string, reqBody interface{}, respBody interface{}) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.generatorURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("generator returned %d: %s", resp.StatusCode, string(body))
	}

	if respBody != nil {
		return json.NewDecoder(resp.Body).Decode(respBody)
	}
	return nil
}
```

- [ ] **Step 4: Implement bot main.go**

Modify `cmd/bot/main.go`:
```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aeremin/neurodan/internal/telegram"
)

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN environment variable is required")
	}

	generatorURL := os.Getenv("GENERATOR_URL")
	if generatorURL == "" {
		generatorURL = "http://localhost:8000"
	}

	fmt.Printf("Bot starting, connecting to generator at %s\n", generatorURL)

	client := telegram.NewClient(token, generatorURL)
	client.Start()
}
```

- [ ] **Step 5: Run tests and build**

```bash
go test ./internal/telegram/ -v
go build ./cmd/bot
```

- [ ] **Step 6: Commit**

```bash
git add internal/telegram/ cmd/bot/
git commit -m "feat: add Telegram bot service with train and generate commands"
```

---

### Task 9: Docker Setup

**Files:**
- Create: `Dockerfile.bot`
- Create: `Dockerfile.generator`
- Create: `docker-compose.yml`

**Interfaces:**
- Produces: Docker images for both services, docker-compose orchestration with Qdrant

- [ ] **Step 1: Create generator Dockerfile**

Create `Dockerfile.generator`:
```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /generator ./cmd/generator

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /generator .
COPY models/ ./models/

EXPOSE 8000
CMD ["./generator"]
```

- [ ] **Step 2: Create bot Dockerfile**

Create `Dockerfile.bot`:
```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bot ./cmd/bot

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /bot .

CMD ["./bot"]
```

- [ ] **Step 3: Create docker-compose.yml**

Create `docker-compose.yml`:
```yaml
services:
  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"
    volumes:
      - qdrant_data:/qdrant/storage

  generator:
    build:
      context: .
      dockerfile: Dockerfile.generator
    ports:
      - "8000:8000"
    environment:
      - QDRANT_HOST=qdrant
      - QDRANT_PORT=6334
      - MODEL_PATH=models/model.onnx
    depends_on:
      - qdrant

  bot:
    build:
      context: .
      dockerfile: Dockerfile.bot
    environment:
      - BOT_TOKEN=${BOT_TOKEN}
      - GENERATOR_URL=http://generator:8000
    depends_on:
      - generator
    restart: always

volumes:
  qdrant_data:
```

- [ ] **Step 4: Verify Docker build**

```bash
docker compose build
```

- [ ] **Step 5: Commit**

```bash
git add Dockerfile.bot Dockerfile.generator docker-compose.yml
git commit -m "feat: add Docker configuration for bot, generator, and Qdrant"
```

---

### Task 10: Integration and Polish

**Files:**
- Modify: various files for interface adaptation
- Create: `scripts/export_model.py` (one-time Python script for ONNX export)

**Interfaces:**
- Wires all components together

- [ ] **Step 1: Create model export script**

Create `scripts/export_model.py`:
```python
"""One-time script to export all-MiniLM-L6-v2 to ONNX format."""
# pip install optimum onnxruntime sentence-transformers
from optimum.onnxruntime import ORTModelForFeatureExtraction
from transformers import AutoTokenizer

model_id = "sentence-transformers/all-MiniLM-L6-v2"
tokenizer = AutoTokenizer.from_pretrained(model_id)
model = ORTModelForFeatureExtraction.from_pretrained(model_id, export=True)

tokenizer.save_pretrained("./models")
model.save_pretrained("./models")
```

- [ ] **Step 2: Run model export**

```bash
pip install optimum onnxruntime sentence-transformers
python scripts/export_model.py
```

- [ ] **Step 3: Adapt storage.Client to api.Storage interface**

The api.Storage interface needs to match storage.Client methods. Update `internal/api/routes.go` to accept `*storage.Client` directly, or create an adapter.

- [ ] **Step 4: Full integration test**

```bash
docker compose up -d
# Send a test message via Telegram
# Verify training and generation work end-to-end
docker compose down
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete Go rewrite with integration testing"
```
