from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession
from src.core.database import async_session, init_db
from src.core.engine import MarkovEngine
from src.api.schemas import TrainRequest, GenerateRequest, GenerateResponse

router = APIRouter()

async def get_db():
    async with async_session() as session:
        yield session

async def get_engine(session: AsyncSession = Depends(get_db)):
    return MarkovEngine(session)

@router.post("/train")
async def train(request: TrainRequest, engine: MarkovEngine = Depends(get_engine)):
    try:
        await engine.train(request.text)
        return {"status": "success"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@router.post("/generate", response_model=GenerateResponse)
async def generate(request: GenerateRequest, engine: MarkovEngine = Depends(get_engine)):
    try:
        text = await engine.generate(length=request.length, seed=request.seed)
        if not text:
            raise HTTPException(status_code=404, detail="Could not generate text")
        return GenerateResponse(text=text)
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
