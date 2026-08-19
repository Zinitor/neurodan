import httpx
from src.config import settings

class GeneratorClient:
    def __init__(self, base_url: str = settings.generator_url):
        self.base_url = base_url.rstrip("/")

    async def train(self, text: str) -> bool:
        async with httpx.AsyncClient() as client:
            try:
                response = await client.post(
                    f"{self.base_url}/train",
                    json={"text": text},
                    timeout=5.0
                )
                return response.status_code == 200
            except httpx.RequestError:
                return False

    async def generate(self, length: int, seed: str | None = None) -> str | None:
        async with httpx.AsyncClient() as client:
            try:
                response = await client.post(
                    f"{self.base_url}/generate",
                    json={"length": length, "seed": seed},
                    timeout=5.0
                )
                if response.status_code == 200:
                    return response.json().get("text")
                return None
            except httpx.RequestError:
                return None
