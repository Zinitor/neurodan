from fastapi import FastAPI
from src.api.routes import router, brain_state
from src.core.database import init_db, async_session

app = FastAPI(title="Neurodan Generator")

@app.on_event("startup")
async def startup():
    await init_db()
    # Warm up the brain: load embeddings into RAM
    async with async_session() as session:
        await brain_state.load_cache(session)
    print("Brain warmed up and cache loaded.")

app.include_router(router)

@app.get("/health")
async def health():
    return {"status": "ok"}
