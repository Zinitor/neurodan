import pytest
import asyncio
from src.core.database import init_db, async_session, Word

@pytest.mark.asyncio
async def test_db_init():
    await init_db()
    async with async_session() as session:
        word = Word(text="hello")
        session.add(word)
        await session.commit()
        
        # Verify it exists
        from sqlalchemy import select
        result = await session.execute(select(Word).filter_by(text="hello"))
        found_word = result.scalar_one()
        assert found_word.text == "hello"
