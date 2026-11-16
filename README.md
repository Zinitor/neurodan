# neurodan

A Telegram bot that learns how a chat talks and answers in the same style. It began as a bot for a group chat of friends, so the point was entertainment, not a product.

Two Go services in one module:

- **bot** (`cmd/bot`) — Telegram long polling. Takes the incoming message, calls the generator over HTTP, posts the answer back.
- **generator** (`cmd/generator`) — HTTP API. Trains a 2nd-order Markov chain over stored phrases, embeds each phrase with `all-MiniLM-L6-v2` through ONNX Runtime, and rejects near-duplicates through Qdrant (cosine similarity ≥ 0.85, up to 5 generation retries).

## Why it exists

A small system to try three things end to end: splitting one bot into two services with a plain HTTP boundary, isolating data per chat, and using vector search as a deduplication filter instead of a hand-written rule.

## Stack

| Layer | Choice |
|---|---|
| Language | Go 1.22, single module, two `cmd/` binaries |
| Router | `chi` |
| Vector store | Qdrant (`markov_transitions`, `sentence_embeddings` collections) |
| Embeddings | `all-MiniLM-L6-v2`, 384-dim, exported to ONNX and served by `onnxruntime_go` |
| Telegram | `go-telegram-bot-api` |
| Tests | `testify` |

## Running

Environment: `BOT_TOKEN`, `GENERATOR_URL`, `QDRANT_HOST`, `QDRANT_PORT`, `MODEL_PATH`.

```bash
docker compose up          # bot + generator + Qdrant
go test ./...              # unit tests, no external services needed for the unit layer
```

## Layout

```
cmd/bot, cmd/generator        entry points
internal/api                  HTTP routes, handlers, schemas
internal/markov               2nd-order chain over word triples
internal/embedding            ONNX session + tokenizer wrapper
internal/storage              Qdrant client, transitions, embeddings
internal/telegram             thin client over the Bot API
docs/design                   design spec and implementation notes
models                        tokenizer assets for the embedding model
```

Every package under `internal/` carries its own `_test.go`. The API layer has the widest coverage (route and handler behaviour, ~10 KB of tests), since that is where the two services meet.

## Status and limits

Personal experiment. It has not run under production load, has no scale story, and the numbers that matter for a real deployment (throughput, latency under concurrency, Qdrant sizing) were never measured. The Python predecessors of both services are still in the repository (`api/`, `services/bot/`) for comparison; the Go services are the direction the project was heading.

## License

MIT — see [LICENSE](LICENSE).
