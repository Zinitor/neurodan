# Restructure Project to Standard Python Layout

Date: 2026-08-20

## Goal

Reorganize the project into a standard Python `src/` layout with two
independent modules — `generator` and `telegram` — that stay separated for
future ease of development, while sharing common code. The two modules are
started together from a single entrypoint, and the Telegram bot calls into
the generator.

## Target Structure

```
src/
  app.py                    # entrypoint: starts FastAPI server + Telegram bot concurrently
  models.py                 # shared pydantic models
  util.py                   # convert_size, remove_duplicates
  generator/
    __init__.py             # re-exports public API
    generator.py            # markov generate()
    storage.py              # MessagesStorage, Stats
    text.py                 # improve_result, censor_result, escape/unescape, parse_raw
    server.py               # FastAPI endpoints (/generate, /count, /wipe, /push, /stats)
  telegram/
    __init__.py
    bot.py                  # TelegramBot — calls generator.generate()
    group_handler.py        # GroupChatHandler
    main.py                 # bot wiring
tests/
  test_generator.py
```

## File Migration

| Current | Destination | Action |
|---|---|---|
| `api/__main__.py` (FastAPI app) | `src/generator/server.py` | endpoints moved here |
| `api/generator/generator.py` | `src/generator/generator.py` | move up one layer |
| `api/generator/generator.pyx` | — | removed |
| `api/__init__.py` (storage/stats/text) | `src/generator/storage.py` + `src/generator/text.py` | split |
| `api/models.py` | `src/models.py` | dedupe |
| `api/util.py` | `src/util.py` | move |
| `neurodan/telegram/*` | `src/telegram/*` | move |
| `models.py` (root) | — | removed (duplicate) |
| `main.py` (root) | — | removed (duplicate logic) |
| `neurodan/` | — | removed after migration |
| `api/` | — | removed after migration |

## Entrypoint (`src/app.py`)

- Loads `TELEGRAM_BOT_TOKEN` from environment; fails fast with a clear
  message if missing.
- Starts the Telegram bot and the FastAPI server (uvicorn) in the same
  process.
- Runs both concurrently (e.g. `asyncio.gather`) with clean shutdown on
  interrupt.
- Imports the FastAPI `app` object from `generator.server`.

## Telegram → Generator Integration

`TelegramBot.handle_message` calls `generator.generate(...)` on received
messages instead of echoing them back.

## `pyproject.toml` Changes

- `[tool.uv] package = true`, `[tool.uv] package-dir = "src"`.
- `[project.scripts] neurodan = "app:main"`.
- Add missing dependencies already imported by the code: `fastapi`,
  `uvicorn`, `pydantic`, `python-telegram-bot`, `python-dotenv`.

## Error Handling & Testing

- `app.py` fails fast with a clear message if `TELEGRAM_BOT_TOKEN` is
  missing.
- `tests/test_generator.py` covers `generate()`: empty input → `None`,
  valid input → non-empty string, size filtering.
