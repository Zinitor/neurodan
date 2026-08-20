# Neurodan Go Rewrite — Running Instructions

## Overview

Neurodan is a Telegram bot that learns from chat messages and generates new text using a 2nd-order Markov chain, with semantic similarity deduplication via sentence embeddings (all-MiniLM-L6-v2, ONNX Runtime).

This is the Go rewrite of the original Python microservice application.

## Prerequisites

- Go 1.22+
- Docker (optional, for compose setup) — not available in this environment, but config exists
- `.env` file with required environment variables

## Environment Variables

Create a `.env` file in the project root with:

```
BOT_TOKEN=your_telegram_bot_token
GENERATOR_URL=http://localhost:8000
QDRANT_HOST=localhost
QDRANT_PORT=6334
MODEL_PATH=models/model.onnx
```

## Project Structure

```
neurodan/
├── cmd/
│   ├── bot/main.go    # Telegram bot entry point
│   └── generator/main.go  # HTTP API server entry point
├── internal/
│   ├── markov/engine.go         # 2nd-order Markov chain
│   ├── embedding/embedder.go   # ONNX sentence embeddings
│   ├── storage/                # Qdrant client + transitions + embeddings
│   ├── api/                    # HTTP routes + schemas
│   └── telegram/               # Telegram client
├── models/
│   └── model.onnx              # Exported ONNX model (already generated)
├── scripts/
│   └── export_model.py         # One-time model export script
├── go.mod
├── go.sum
├── Dockerfile.bot
├── Dockerfile.generator
└── docker-compose.yml
```

## Running Without Docker

### 1. Start Qdrant

Qdrant is the vector database used for storing Markov transitions and sentence embeddings.

```bash
# Start Qdrant via Docker
docker run -d --name qdrant -p 6333:6333 -p 6334:6334 qdrant/qdrant:latest

# Or start locally if already running
```

### 2. Build and Run the Generator Service

```bash
go build -o generator ./cmd/generator
./generator
```

Or using `go run`:

```bash
go run ./cmd/generator
```

The generator will start on `http://localhost:8000` by default (or override with `PORT` env var).

### 3. Build and Run the Bot Service

```bash
go build -o bot ./cmd/bot
./bot
```

Or:

```bash
go run ./cmd/bot
```

The bot will start long-polling Telegram. It needs `BOT_TOKEN` and `GENERATOR_URL` env vars.

### 4. Test the Endpoints

**Health check:**
```bash
curl http://localhost:8000/health
# {"status":"ok"}
```

**Train (send a message to the bot on Telegram, or use curl):**
```bash
curl -X POST http://localhost:8000/train -H "Content-Type: application/json" -d '{"text":"hello world test","chat_id":123456789}'
# {"status":"success"}
```

**Generate:**
```bash
curl -X POST http://localhost:8000/generate -H "Content-Type: application/json" -d '{"length":10,"seed":"hello","chat_id":123456789}'
# {"text":"generated text here"}
```

## Running With Docker Compose

If Docker is available:

```bash
docker compose up -d
```

This starts:
- **qdrant** — vector database on ports 6333/6334
- **generator** — HTTP API on port 8000
- **bot** — Telegram bot (depends on generator)

Environment variables are loaded from `.env` and the compose file.

## Model — Already Exported

The ONNX model (`models/model.onnx`) has already been exported using `sentence-transformers[onnx]` and `optimum`. No re-export is needed.

The model was generated with:
```bash
uv pip install "sentence-transformers[onnx]"
/home/aeremin/projects/neurodan/.venv/bin/python scripts/export_model.py
```

## Development

### Running Tests

```bash
go test ./...
```

All tests should pass (37+ tests across all packages).

### Running Lint/Typecheck

```bash
go vet ./...
go build ./...
```

## How the System Works

1. **Bot receives a message** → calls `POST /train` on the generator with `{text, chat_id}`
2. **Generator trains Markov chain** → stores 2nd-order transitions in Qdrant (`markov_transitions` collection)
3. **Generator computes embedding** → stores sentence embedding in Qdrant (`sentence_embeddings` collection)
4. **Bot `/generate` command** → calls `POST /generate` with `{length, seed, chat_id}`
5. **Generator generates text** → weighted random walk through Markov chain
6. **Generator checks similarity** → if generated text is too semantically similar to existing text (cosine > 0.85), regenerates (max 5 retries)
7. **Response sent** → unique text back to Telegram user

## Initial Data — No Seed Needed

Unlike the Python version which had a `scripts/seed_db.py` to bulk-train from `result.json`, the Go version **trains organically** as Telegram messages arrive. Every text message sent in Telegram chats is forwarded to the generator for training.

If you want to initially populate the Markov chain, you can send training messages via the `/train` endpoint or via the Telegram bot.