import pytest
import asyncio
from httpx import ASGITransport, AsyncClient
from src.main import app
from src.core.database import init_db

@pytest.mark.asyncio
async def test_api_train_and_generate():
    # Initialize DB
    await init_db()
    
    # Use ASGITransport for testing FastAPI apps without running a server
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        # 1. Test Train
        train_resp = await ac.post("/train", json={"text": "the quick brown fox jumps over the lazy dog"})
        assert train_resp.status_code == 200
        assert train_resp.json() == {"status": "success"}
        
        # 2. Test Generate
        gen_resp = await ac.post("/generate", json={"length": 3, "seed": "the"})
        assert gen_resp.status_code == 200
        data = gen_resp.json()
        assert "text" in data
        assert len(data["text"].split()) == 3
        assert data["text"].startswith("the")

@pytest.mark.asyncio
async def test_api_health():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        resp = await ac.get("/health")
        assert resp.status_code == 200
        assert resp.json() == {"status": "ok"}
