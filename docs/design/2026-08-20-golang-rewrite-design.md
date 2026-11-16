# Neurodan Go Rewrite — Design Spec

**Date:** 2026-08-20
**Status:** Draft
**Motivation:** Performance — faster text generation, lower resource usage, single static binaries

## Overview

Rewrite both Python microservices (bot + generator) in Go, replace SQLite with Qdrant for vector storage, use ONNX Runtime for sentence embeddings, and add per-chat data isolation.

## Architecture

```
Telegram API
    │
    ▼
Bot Service (Go) ─── cmd/bot/main.go
    │  HTTP calls to generator
    ▼
Generator Service (Go) ─── cmd/generator/main.go
    │
    ├──► Qdrant (markov_transitions + sentence_embeddings)
    └──► ONNX Runtime (all-MiniLM-L6-v2 inference)
```

## Project Structure

```
neurodan/
├── go.mod
├── go.sum
├── cmd/
│   ├── bot/
│   │   └── main.go
│   └── generator/
│       └── main.go
├── internal/
│   ├── markov/
│   │   └── engine.go
│   ├── embedding/
│   │   └── embedder.go
│   ├── storage/
│   │   ├── qdrant.go
│   │   ├── transitions.go
│   │   └── embeddings.go
│   ├── api/
│   │   ├── routes.go
│   │   └── schemas.go
│   └── telegram/
│       └── client.go
├── docker-compose.yml
├── Dockerfile.bot
└── Dockerfile.generator
```

## Library Choices

| Purpose | Library |
|---|---|
| Telegram bot | `github.com/go-telegram-bot-api/telegram-bot-api/v2` |
| HTTP routing | `net/http` + `github.com/go-chi/chi/v5` |
| Qdrant client | `github.com/qdrant/go-client` |
| ONNX Runtime | `github.com/yalue/onnxruntime_go` |
| Configuration | `os.Getenv` / envconfig |
| Testing | `testing` + `github.com/stretchr/testify` |

## API Contract

Identical endpoints, with `chat_id` added for per-chat isolation:

### POST /train

```json
Request:  { "text": "string", "chat_id": "int64" }
Response: { "status": "success" }
```

### POST /generate

```json
Request:  { "length": "int", "seed": "string|null", "chat_id": "int64" }
Response: { "text": "string" }
```

### GET /health

```json
Response: { "status": "ok" }
```

## Qdrant Schema

### Collection: `markov_transitions`

Stores 2nd-order Markov chain transition data.

- **Vector:** None (payload-only collection, or 1-dim dummy vector for Qdrant compatibility)
- **Payload:**
  - `chat_id` (int64) — Telegram chat ID, indexed
  - `word1` (string) — first word in transition
  - `word2` (string) — second word in transition
  - `next_word` (string) — predicted next word
  - `weight` (int) — frequency count
- **Index:** Payload index on `chat_id` for filtered queries
- **Lookup:** Filter by `chat_id` + `word1` + `word2` to get possible next words and weights

### Collection: `sentence_embeddings`

Stores sentence embeddings for semantic deduplication.

- **Vector:** 384-dim float32 (`all-MiniLM-L6-v2` output)
- **Payload:**
  - `chat_id` (int64) — Telegram chat ID, indexed
  - `text` (string) — the original text
  - `created_at` (int64) — Unix timestamp
- **Index:** Payload index on `chat_id`
- **Search:** Vector similarity search filtered by `chat_id`

## Data Flow

### Training (`POST /train`)

1. Receive `{ text, chat_id }`
2. Tokenize text into words
3. Extract all 2nd-order transitions (word_i, word_{i+1} → word_{i+2})
4. For each transition:
   - Query Qdrant for existing transition with same `chat_id` + `word1` + `word2` + `next_word`
   - If exists: increment weight
   - If new: insert with weight=1
5. Compute sentence embedding via ONNX Runtime
6. Store embedding in Qdrant with `chat_id` and text payload
7. Return `{ "status": "success" }`

### Generation (`POST /generate`)

1. Receive `{ length, seed, chat_id }`
2. Start from seed words (or random if no seed)
3. Query Qdrant for transitions matching `chat_id` + last two words
4. Weighted random walk to build candidate text
5. Compute candidate embedding via ONNX Runtime
6. Query Qdrant for similar embeddings within `chat_id` (cosine similarity > 0.85)
7. If too similar to existing text: regenerate (max 5 retries)
8. Return the unique generated text

## ONNX Embedding Pipeline

### Model Export (one-time, Python)

```bash
pip install optimum onnxruntime sentence-transformers
optimum-cli export onnx --model sentence-transformers/all-MiniLM-L6-v2 ./models/
```

Produces `model.onnx` (input: token IDs + attention mask, output: 384-dim embedding).

### Go Inference

1. Load `model.onnx` via `onnxruntime_go`
2. Implement WordPiece tokenization in Go (or use a minimal tokenizer lib)
3. Run inference: tokenize → ONNX session → pool → normalize → 384-dim vector
4. Cache the ONNX session and tokenizer at startup

## Per-Chat Isolation

- Every Qdrant operation includes a `chat_id` filter
- Different Telegram chats are completely isolated
- The bot passes the Telegram `chat.ID` with every request to the generator
- No cross-chat data leakage possible

## Error Handling

- **Qdrant unreachable:** Return 503 Service Unavailable
- **ONNX model not loaded:** Fail startup with clear error message
- **Invalid request:** Return 400 Bad Request with error detail
- **Generation timeout:** Return 504 if generation takes too long
- **Bot → Generator HTTP errors:** Log error, reply to user with generic error message

## Docker Deployment

### docker-compose.yml

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
      - QDRANT_URL=http://qdrant:6333
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

### Dockerfiles

Both services use multi-stage builds:
- **Build stage:** `golang:1.22-alpine` — compile static binary
- **Run stage:** `alpine:3.19` or `scratch` — minimal image with just the binary

## Migration from Python

1. Export existing data from SQLite (if any) — write a one-time migration script
2. Deploy Qdrant instance
3. Deploy Go services
4. Update bot `GENERATOR_URL` to point to new Go generator
5. Old Python services can be kept as fallback during transition

## Testing Strategy

- **Unit tests:** `internal/markov/engine.go` — test train/generate logic
- **Unit tests:** `internal/storage/` — test Qdrant operations (with testcontainers or mock)
- **Integration tests:** `cmd/generator/` — test full HTTP API flow
- **Unit tests:** `cmd/bot/` — test Telegram handler logic with mock HTTP client
- **Benchmarks:** Compare Go vs Python generation speed

## Scope

In scope:
- Rewrite both services in Go
- Qdrant for storage
- ONNX Runtime for embeddings
- Per-chat isolation
- Docker deployment
- Basic tests

Out of scope:
- CI/CD pipeline
- Kubernetes deployment
- Authentication/authorization
- Rate limiting
- Monitoring/metrics
- Migration of existing SQLite data
