from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession
from src.api.schemas import GenerateRequest, GenerateResponse, TrainRequest
from src.core.database import async_session, init_db
from src.core.engine import BrainState, MarkovEngine

router = APIRouter()
# Global state for the brain
brain_state = BrainState()


async def get_db():
    async with async_session() as session:
        yield session


async def get_engine(session: AsyncSession = Depends(get_db)):
    return MarkovEngine(session, brain_state)


@router.post("/train")
async def train(request: TrainRequest, engine: MarkovEngine = Depends(get_engine)):
    try:
        await engine.train(request.text)
        return {"status": "success"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/generate", response_model=GenerateResponse)
async def generate(
    request: GenerateRequest, engine: MarkovEngine = Depends(get_engine)
):
    # try:
    text = None
    while text == None:
        text = await engine.generate(length=request.length, seed=request.seed, size=0)
    return GenerateResponse(text=text)
    # except HTTPException:
    #     raise
    # except Exception as e:
    #     raise HTTPException(status_code=500, detail=str(e))
