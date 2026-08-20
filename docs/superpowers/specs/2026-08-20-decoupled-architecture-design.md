# Design Spec: Neurodan Decoupled Architecture

## Date: 2026-08-20
## Status: Approved

## 1. Overview
Neurodan is an evolving system that reads messages from a Telegram chat, learns from them to expand its vocabulary, and generates responses using a Markov chain. 

To support scalability and maintainability, the project is moving from a monolithic script to a decoupled service architecture.

## 2. Architecture
The system is split into two independent services communicating over HTTP/REST.

### 2.1 Generator Service (The Brain)
A stateful service responsible for the "knowledge" and text generation.
- **Responsibilities**:
    - Maintain a database of word transitions.
    - Provide an API to "train" the model with new text.
    - Provide an API to generate text based on the learned model.
- **Stack**:
    - Framework: `FastAPI` + `Uvicorn`
    - Database: `SQLite` via `SQLAlchemy` (async)
    - Logic: `MarkovEngine` class for chain management.

### 2.2 Bot Service (The Interface)
A stateless service that handles the user interaction.
- **Responsibilities**:
    - Listen for Telegram messages.
    - Forward messages to the Generator for training.
    - Request generated responses from the Generator.
    - Send responses back to the user.
- **Stack**:
    - Framework: `aiogram` (v3.x)
    - HTTP Client: `httpx` (async)
    - Config: `pydantic-settings`

## 3. API Specification

### 3.1 Training Endpoint
- **Endpoint**: `POST /train`
- **Payload**: `{"text": "string"}`
- **Success**: `200 OK`
- **Behavior**: Splits text into tokens and updates the transition weights in the database.

### 3.2 Generation Endpoint
- **Endpoint**: `POST /generate`
- **Payload**: `{"length": int, "seed": "string" (optional)}`
- **Success**: `200 OK` $\rightarrow$ `{"text": "generated string"}`
- **Behavior**: Generates a sequence of words based on the Markov chain.

### 3.3 Health Check
- **Endpoint**: `GET /health`
- **Success**: `200 OK` $\rightarrow$ `{"status": "ok"}`

## 4. Data Model
The database will use a relational structure to store the Markov chain:
- `words` table: Stores unique words to avoid redundancy.
- `transitions` table: Stores pairs of `(from_word_id, to_word_id)` and a `weight` (count of occurrences).

## 5. Project Structure
```text
neurodan/
├── services/
│   ├── generator/
│   │   ├── src/
│   │   │   ├── api/            # Routes and Schemas
│   │   │   ├── core/           # MarkovEngine and DB logic
│   │   │   └── main.py         # Entry point
│   │   └── pyproject.toml
│   └── bot/
│       ├── src/
│       │   ├── handlers/       # Telegram event handlers
│       │   ├── client/         # Generator API client
│       │   └── main.py         # Entry point
│       └── pyproject.toml
└── docker-compose.yml
```

## 6. Success Criteria
- [ ] Generator service can be started independently and accepts `/train` and `/generate` calls.
- [ ] Bot service can be started and successfully forwards messages to the Generator.
- [ ] The system dynamically expands its dictionary as new messages arrive.
- [ ] The system handles Generator downtime without crashing the Bot.
