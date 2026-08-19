from fastapi import FastAPI
from src.api.routes import router
from src.core.database import init_db

app = FastAPI(title="Neurodan Generator")

@app.on_event("startup")
async def startup():
    await init_db()

app.include_router(router)

@app.get("/health")
async def health():
    return {"status": "ok"}
