import pytest
import asyncio
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker
from src.core.database import Base, init_db_custom, Word, Transition
from src.core.engine import MarkovEngine

@pytest.mark.asyncio
async def test_markov_train_and_generate():
    # Use in-memory DB for isolation and speed
    url = "sqlite+aiosqlite:///:memory:"
    engine = await init_db_custom(url)
    async_session = async_sessionmaker(engine, expire_on_commit=False)
    
    async with async_session() as session:
        engine_logic = MarkovEngine(session)
        await engine_logic.train("the cat sat on the mat")
        
        result = await engine_logic.generate(length=2, seed="the")
        assert result.startswith("the")
        assert any(result.endswith(w) for w in ["cat", "mat"])

@pytest.mark.asyncio
async def test_markov_random_fallback():
    url = "sqlite+aiosqlite:///:memory:"
    engine = await init_db_custom(url)
    async_session = async_sessionmaker(engine, expire_on_commit=False)
    
    async with async_session() as session:
        engine_logic = MarkovEngine(session)
        await engine_logic.train("hello world")
        
        # Generate with a seed that has no transitions (world)
        # It should fallback to a random word
        result = await engine_logic.generate(length=2, seed="world")
        assert len(result.split()) == 2
        assert "hello" in result or "world" in result
