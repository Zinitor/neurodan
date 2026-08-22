# Neurodan Go Rewrite — Running Instructions

## Overview

Neurodan is a Telegram bot that learns from chat messages and generates new text using a **variable-order (1–3) Markov chain**, with semantic similarity deduplication via sentence embeddings (all-MiniLM-L6-v2, ONNX Runtime).

This is the Go rewrite of the original Python microservice application.

## Features

- **Variable-order Markov chain** — every message trains order-1/2/3 n-grams. Generation re-rolls the n-gram order on every word from a per-preset distribution, so output swings between erratic bursts and coherent stretches.
- **Length presets** — `short` (1–5 words), `medium` (6–15), `long` (16–40). Each preset is just a probability distribution over n-gram orders.
- **Context-aware replies** — the bot keeps a sliding window of the last 15 messages per chat. To reply, it finds the longest suffix of the recent conversation that appears in the model (keyword bridging) and seeds generation from there, so it answers "in context".
- **Stacking reply probability** — the chance to reply grows the longer the bot stays silent: `min(REPLY_MAX, REPLY_BASE + REPLY_GROWTH × messagesSinceLastReply)`. Mentioning `@bot` always forces a reply.
- **Reactions** — some replies are bare kaomoji / emoji / Russian net-speak instead of generated text.
- **Message chaining** — a single reply event can send up to `CHAIN_MAX` messages, each seeded by the one before.
- **Russian-friendly** — the bot learns whatever it's told, handles Cyrillic fine, and the stylization pass (ALL CAPS, `...`, trailing reactions) works on any script.

## Commands

All commands are in Russian, since that's the bot's home crowd.

| Command | Description |
| --- | --- |
| `/generate [слова]` | Generate text. Optionally seed it with a starting phrase. |
| `/anec [слова]` | Continue the given text in the style of a joke (seeded generation, short). |
| `/help` | Show help (in Russian). |
| `/on [фичи]` | Enable stylization features. Without arguments, shows current settings. |
| `/off [фичи]` | Disable stylization features. |

Toggleable stylization features (`/on` / `/off`):

| Feature | Effect |
| --- | --- |
| `caps` | Uppercase a random word (ПРИВЕТ) |
| `allcaps` | Uppercase a whole phrase (ЦЕЛИКОМ КАПСОМ) |
| `punct` | Energetic trailing punctuation (…, !, !!) |
| `emotes` | Trailing reactions and kaomoji (😂, кек, лол) |
| `swearify` | Russian profanity |
| `intellectualize` | High-register "intellectual" filler (концептуально, эмпирически) |

Toggles are stored per chat in `SETTINGS_FILE` and survive bot restarts. Multiple features can be listed at once, comma- or space-separated (e.g. `/off swearify, allcaps`). Stylization applies to all generated text, including `/anec` and auto-replies.

## Prerequisites

- Go 1.22+
- Docker (optional, for compose setup) — not available in this environment, but config exists
- `.env` file with required environment variables

## Environment Variables

### Bot (`cmd/bot`)

| Variable | Default | Description |
| --- | --- | --- |
| `BOT_TOKEN` | — | Telegram bot token (required) |
| `GENERATOR_URL` | `http://localhost:8000` | Generator service base URL |
| `REPLY_BASE` | `0.10` | Reply probability for a fresh message |
| `REPLY_GROWTH` | `0.10` | Added to reply probability per silent message |
| `REPLY_MAX` | `0.60` | Caps the stacking reply probability |
| `REACTION_PROB` | `0.20` | Chance a triggered reply is a bare reaction |
| `CHAIN_MAX` | `3` | Max consecutive messages per reply event |
| `LENGTH` | `medium` | Length preset: `short`, `medium`, `long` |
| `WINDOW_SIZE` | `15` | Recent messages kept as conversation context |
| `MAX_CONTEXT_WORDS` | `300` | Caps the context payload sent to the generator |
| `SETTINGS_FILE` | `./data/chat_settings.json` | Where per-chat stylization toggles are persisted |

### Generator (`cmd/generator`)

```
QDRANT_HOST=localhost
QDRANT_PORT=6334
MODEL_PATH=models/model.onnx
PORT=8000
```

## Project Structure

```
neurodan/
├── cmd/
│   ├── bot/main.go    # Telegram bot entry point
│   └── generator/main.go  # HTTP API server entry point
├── internal/
│   ├── markov/engine.go         # variable-order (1–3) Markov chain + stylize
│   ├── embedding/embedder.go   # ONNX sentence embeddings
│   ├── storage/                # Qdrant client + transitions + embeddings
│   ├── api/                    # HTTP routes + schemas
│   └── telegram/               # Telegram client (window, triggers, chaining)
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
curl -X POST http://localhost:8000/generate -H "Content-Type: application/json" -d '{"length_cfg":"medium","seed":"привет","chat_id":123456789}'
# {"text":"generated text here"}
```

`length_cfg` is one of `short` | `medium` | `long`. To have the bot reply in the context of a conversation, pass `context` with the recent messages instead of (or in addition to) `seed`; the generator keyword-bridges from the context tail. The legacy `length` integer is still accepted and mapped onto the presets.

`styling` (optional) is an object of `{caps, allcaps, punct, emotes, swearify, intel}` booleans controlling which stylization passes run; omit it for all-defaults.

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

1. **Bot receives a message** → strips any `@mention` (never trained) and calls `POST /train` with `{text, chat_id}`
2. **Generator trains Markov chain** → stores variable-order (1–3) transitions in Qdrant (`markov_transitions_v2` collection)
3. **Generator computes embedding** → stores sentence embedding in Qdrant (`sentence_embeddings` collection)
4. **Bot decides whether to reply** → the reply probability stacks up the longer the bot is silent, or a mention forces a reply; some replies are bare reactions
5. **Bot replies in context** → sends `POST /generate` with `{length_cfg, context, chat_id}`, where `context` is the last ~15 messages; the generator keyword-bridges from the context tail and runs a weighted random walk
6. **Generator checks similarity** → if generated text is too semantically similar to existing text (cosine > 0.85), regenerates (max 5 retries)
7. **Stylization** → the result is run through a probabilistic pass (ALL CAPS word, `...`, trailing reaction) for emotional weight
8. **Chaining** → the bot may send up to `CHAIN_MAX` messages per reply event, each seeded by the previous one

## Initial Data — No Seed Needed

Unlike the Python version which had a `scripts/seed_db.py` to bulk-train from `result.json`, the Go version **trains organically** as Telegram messages arrive. Every text message sent in Telegram chats is forwarded to the generator for training.

If you want to initially populate the Markov chain, you can send training messages via the `/train` endpoint or via the Telegram bot.

> **Note:** the transitions collection was renamed to `markov_transitions_v2` when it became variable-order. Old `markov_transitions` data is not migrated; the collection repopulates organically.