from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker, AsyncSession
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column
from sqlalchemy import ForeignKey, Integer, String

class Base(DeclarativeBase):
    pass

class Word(Base):
    __tablename__ = "words"
    id: Mapped[int] = mapped_column(primary_key=True)
    text: Mapped[str] = mapped_column(String, unique=True, index=True)

class Sample(Base):
    __tablename__ = "samples"
    id: Mapped[int] = mapped_column(primary_key=True)
    embedding: Mapped[list[float]] = mapped_column(String) # Stored as JSON string

class Transition(Base):
    __tablename__ = "transitions"
    id: Mapped[int] = mapped_column(primary_key=True)
    word1_id: Mapped[int] = mapped_column(ForeignKey("words.id"))
    word2_id: Mapped[int] = mapped_column(ForeignKey("words.id"))
    next_word_id: Mapped[int] = mapped_column(ForeignKey("words.id"))
    weight: Mapped[int] = mapped_column(Integer, default=1)

engine = create_async_engine("sqlite+aiosqlite:///./generator.db")
async_session = async_sessionmaker(engine, expire_on_commit=False)

async def init_db():
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

async def init_db_custom(url: str):
    custom_engine = create_async_engine(url)
    async with custom_engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    return custom_engine
