from fastapi import FastAPI

app = FastAPI(title="Neurodan Generator")

@app.get("/health")
async def health():
    return {"status": "ok"}
