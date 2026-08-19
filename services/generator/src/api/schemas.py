from pydantic import BaseModel

class TrainRequest(BaseModel):
    text: str

class GenerateRequest(BaseModel):
    length: int
    seed: str | None = None

class GenerateResponse(BaseModel):
    text: str
