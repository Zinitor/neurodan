import pytest
import asyncio
from src.core.database import init_db, async_session
from src.core.engine import MarkovEngine

@pytest.mark.asyncio
async def test_markov_train_and_generate():
    await init_db()
    async with async_session() as session:
        engine = MarkovEngine(session)
        
        # Train with a simple predictable sequence
        await engine.train("the cat sat on the mat")
        
        # Generate with seed "the"
        # "the" is followed by "cat" (1) and "mat" (0) - wait, "the" is followed by "cat" and "mat" is the end.
        # "the cat sat on the mat"
        # the -> cat
        # cat -> sat
        # sat -> on
        # on -> the
        # the -> mat
        # mat -> (none)
        
        # If we seed with "the", we should get either "cat" or "mat"
        result = await engine.generate(length=2, seed="the")
        assert result.startswith("the")
        assert any(result.endswith(w) for w in ["cat", "mat"])

@pytest.mark.asyncio
async def test_markov_random_fallback():
    await init_db()
    async with async_session() as session:
        engine = MarkovEngine(session)
        await engine.train("hello world")
        
        # Generate with a seed that has no transitions (world)
        # It should fallback to a random word
        result = await engine.generate(length=5, seed="world")
        assert len(result.split()) == 5
        assert "hello" in result or "world" in result
