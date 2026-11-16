# Bot Service Implementation Plan

**Goal:** Build a Telegram bot that interfaces with the Generator Service to learn from and generate text.

**Architecture:** An asynchronous bot using `aiogram` that delegates all "intelligence" to the Generator Service via a dedicated HTTP client.

**Tech Stack:** aiogram (v3.x), httpx, pydantic-settings.

**Spec:** `docs/design/2026-08-20-decoupled-architecture-design.md`

## Global Constraints
- Python version: `>=3.14`
- Bot Framework: aiogram v3.x
- Communication: Async HTTP via httpx
- Config: Environment variables via pydantic-settings

---

### Task 1: Project Scaffolding & Configuration

**Files:**
- Create: `services/bot/pyproject.toml`
- Create: `services/bot/src/config.py`
- Create: `services/bot/src/main.py`

**Interfaces:**
- Produces: `Settings` object with `BOT_TOKEN` and `GENERATOR_URL`.

- [ ] **Step 1: Create `pyproject.toml` with dependencies**
```toml
[project]
name = "neurodan-bot"
version = "0.1.0"
dependencies = [
    "aiogram",
    "httpx",
    "pydantic-settings",
]

[tool.uv]
package = false

[build-system]
requires = ["uv_build>=0.12.5,<<00.13.0"]
build-backend = "uv_build"
```

- [ ] **Step 2: Implement `config.py`**
```python
from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    bot_token: str
    generator_url: str = "http://localhost:8000"
    
    model_config = SettingsConfigDict(env_file=".env")

settings = Settings()
```

- [ ] **Step 3: Create minimal `main.py`**
```python
import asyncio
from aiogram import Bot, Dispatcher
from src.config import settings

async def main():
    bot = Bot(token=settings.bot_token)
    dp = Dispatcher()
    
    print("Bot started...")
    await dp.start_polling(bot)

if __name__ == "__main__":
    asyncio.run(main())
```

- [ ] **Step 4: Commit**
```bash
git add services/bot/
git commit -m "feat(bot): scaffold project and config"
```

### Task 2: Generator API Client

**Files:**
- Create: `services/bot/src/client/generator_client.py`
- Create: `services/bot/tests/test_client.py`

**Interfaces:**
- Produces: `GeneratorClient.train(text: str)`, `GeneratorClient.generate(length: int, seed: str = None)`

- [ ] **Step 1: Implement `GeneratorClient`**
Use `httpx.AsyncClient` to wrap the Generator Service endpoints (`/train`, `/generate`).

- [ ] **Step 2: Write tests in `test_client.py`**
Use `pytest-asyncio` and `respx` (or similar) to mock the Generator API responses.

- [ ] **Step 3: Run tests and verify PASS**

- [ ] **Step 4: Commit**
```bash
git add services/bot/src/client/ services/bot/tests/test_client.py
git commit -m "feat(bot): implement generator api client"
```

### Task 3: Message Handlers

**Files:**
- Create: `services/bot/src/handlers/messages.py`
- Modify: `services/bot/src/main.py`

**Interfaces:**
- Consumes: `GeneratorClient`
- Produces: Telegram event handlers for text messages.

- [ ] **Step 1: Implement message handler in `messages.py`**
Logic:
1. On every text message:
    - Call `client.train(message.text)`.
2. If message starts with `/generate` (or similar trigger):
    - Call `client.generate(...)`.
    - Send the resulting text back to the user.

- [ ] **Step 2: Register handlers in `main.py`**
```python
from src.handlers.messages import router as msg_router
dp.include_router(msg_router)
```

- [ ] **Step 3: Write tests for handlers in `services/bot/tests/test_handlers.py`**
Mock the `GeneratorClient` and verify that the bot calls the correct methods and sends the correct responses.

- [ ] **Step 4: Run tests and verify PASS**

- [ ] **Step 5: Commit**
```bash
git add services/bot/src/handlers/ services/bot/src/main.py services/bot/tests/test_handlers.py
git commit -m "feat(bot): implement message handlers"
```

### Task 4: Integration & Final Polish

**Files:**
- Create: `docker-compose.yml`

- [ ] **Step 1: Create `docker-compose.yml`**
Define two services: `generator` and `bot`, ensuring the bot uses the `generator` service name as its `GENERATOR_URL`.

- [ ] **Step 2: Final verification**
Run `docker-compose up`, send a message to the bot, and verify it is learned and can be generated.

- [ ] **Step 3: Commit**
```bash
git add docker-compose.yml
git commit -m "chore: add docker-compose for full system orchestration"
```
