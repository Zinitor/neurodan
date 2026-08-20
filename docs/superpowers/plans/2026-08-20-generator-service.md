# Generator Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a stateful Markov chain text generation service with a REST API.

**Architecture:** A FastAPI application using an asynchronous SQLite database via SQLAlchemy. The core logic is encapsulated in a `MarkovEngine` class that manages word transitions.

**Tech Stack:** FastAPI, Uvicorn, SQLAlchemy, aiosqlite, Pydantic.

**Spec:** `docs/superpowers/specs/2026-08-20-decoupled-architecture-design.md`

## Global Constraints
- Python version: `>=3.14`
- Database: SQLite (async)
- API: REST/JSON
- Framework: FastAPI

---

### Task 1: Project Scaffolding & Dependencies

**Files:**
- Create: `services/generator/pyproject.toml`
- Create: `services/generator/src/main.py`

**Interfaces:**
- Produces: Basic FastAPI app structure and dependency list.

- [ ] **Step 1: Create `pyproject.toml` with dependencies**
```toml
[project]
name = "neurodan-generator"
version = "0.1.0"
dependencies = [
    "fastapi",
    "uvicorn",
    "sqlalchemy",
    "aiosqlite",
    "pydantic",
]

[build-system]
requires = ["uv_build>=0.12.5,<0.13.0"]
build-backend = "uv_build"
```

- [ ] **Step 2: Create minimal `main.py`**
```python
from fastapi import FastAPI

app = FastAPI(title="Neurodan Generator")

@app.get("/health")
async def health():
    return {"status": "ok"}
```

- [ ] **Step 3: Verify health endpoint**
Run: `uv run --package services/generator uvicorn src.main:app --port 8000`
Test: `curl http://localhost:8000/health`
Expected: `{"status": "ok"}`

- [ ] **Step 4: Commit**
```bash
git add services/generator/
git commit -m "feat(generator): scaffold project and health check"
```

### Task 2: Database Schema & Session Management

**Files:**
- Create: `services/generator/src/core/database.py`

**Interfaces:**
- Produces: `AsyncSession` provider and SQLAlchemy models (`Word`, `Transition`).

- [ ] **Step 1: Define SQLAlchemy models**
```python
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker, AsyncSession
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column
from sqlalchemy import ForeignKey, Integer, String

class Base(DeclarativeBase):
    pass

class Word(Base):
    __tablename__ = "words"
    id: Mapped[int] = mapped_column(primary_key=True)
    text: Mapped[str] = mapped_column(String, unique=True, index=True)

class Transition(Base):
    __tablename__ = "transitions"
    id: Mapped[int] = mapped_column(primary_key=True)
    from_word_id: Mapped[int] = mapped_column(ForeignKey("words.id"))
    to_word_id: Mapped[int] = mapped_column(ForeignKey("words.id"))
    weight: Mapped[int] = mapped_column(Integer, default=1)

engine = create_async_engine("sqlite+aiosqlite:///./generator.db")
async_session = async_sessionmaker(engine, expire_on_commit=False)

async def init_db():
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
```

- [ ] **Step 2: Create test to verify DB initialization**
Create `services/generator/tests/test_db.py`:
```python
import pytest
from src.core.database import init_db, async_session, Word

@pytest.mark.asyncio
async def test_db_init():
    await init_db()
    async with async_session() as session:
        word = Word(text="hello")
        session.add(word)
        await session.commit()
        # verify it exists
```

- [ ] **Step 3: Run test and verify**
Run: `pytest services/generator/tests/test_db.py`
Expected: PASS

- [ ] **Step 4: Commit**
```bash
git add services/generator/src/core/database.py services/generator/tests/test_db.py
git commit -m "feat(generator): implement async sqlite schema"
```

### Task 3: Markov Engine Logic

**Files:**
- Create: `services/generator/src/core/engine.py`
- Create: `services/generator/tests/test_engine.py`

**Interfaces:**
- Consumes: `AsyncSession` from `database.py`
- Produces: `MarkovEngine.train(text: str)`, `MarkovEngine.generate(length: int, seed: str = None)`

- [ ] **Step 1: Implement `MarkovEngine.train`**
Logic:
1. Tokenize text.
2. For each pair of words (A, B):
    - Ensure A and B exist in `words` table.
    - Upsert `Transition(from=A, to=B)` incrementing `weight`.

- [ ] **Step 2: Implement `MarkovEngine.generate`**
Logic:
1. Pick starting word (seed or random).
2. While length < target:
    - Query `transitions` for all `to_word_id` where `from_word_id` is current.
    - Pick next word weighted by `weight`.
    - If no transitions, pick random word.

- [ ] **Step 3: Write failing tests in `test_engine.py`**
- [ ] **Step 4: Run tests to verify failure**
- [ ] **Step 5: Implement logic to make tests pass**
- [ ] **Step 6: Run tests and verify PASS**

- [ ] **Step 7: Commit**
```bash
git add services/generator/src/core/engine.py services/generator/tests/test_engine.py
git commit -m "feat(generator): implement markov engine logic"
```

### Task 4: API Endpoints Implementation

**Files:**
- Create: `services/generator/src/api/schemas.py`
- Create: `services/generator/src/api/routes.py`
- Modify: `services/generator/src/main.py`

**Interfaces:**
- Consumes: `MarkovEngine` from `core/engine.py`
- Produces: `POST /train`, `POST /generate`

- [ ] **Step 1: Define Pydantic schemas in `schemas.py`**
```python
from pydantic import BaseModel

class TrainRequest(BaseModel):
    text: str

class GenerateRequest(BaseModel):
    length: int
    seed: str | None = None

class GenerateResponse(BaseModel):
    text: str
```

- [ ] **Step 2: Implement routes in `routes.py`**
Use `Depends` to inject `AsyncSession` and `MarkovEngine`.

- [ ] **Step 3: Wire routes into `main.py`**
Add `app.include_router(router)` and `app.on_event("startup")(init_db)`.

- [ ] **Step 4: Write integration tests in `services/generator/tests/test_api.py`**
Use `httpx.AsyncClient` to test `/train` and `/generate`.

- [ ] **Step 5: Run tests and verify PASS**

- [ ] **Step 6: Commit**
```bash
git add services/generator/src/api/ services/generator/src/main.py services/generator/tests/test_api.py
git commit -m "feat(generator): implement api endpoints"
```
