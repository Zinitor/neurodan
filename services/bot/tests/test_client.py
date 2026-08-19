import pytest
import respx
from httpx import Response
from src.client.generator_client import GeneratorClient

@pytest.mark.asyncio
async def test_client_train_success():
    client = GeneratorClient(base_url="http://test")
    async with respx.mock:
        respx.post("http://test/train").mock(return_value=Response(200, json={"status": "success"}))
        
        result = await client.train("hello world")
        assert result is True

@pytest.mark.asyncio
async def test_client_train_failure():
    client = GeneratorClient(base_url="http://test")
    async with respx.mock:
        respx.post("http://test/train").mock(return_value=Response(500))
        
        result = await client.train("hello world")
        assert result is False

@pytest.mark.asyncio
async def test_client_generate_success():
    client = GeneratorClient(base_url="http://test")
    async with respx.mock:
        respx.post("http://test/generate").mock(return_value=Response(200, json={"text": "generated text"}))
        
        result = await client.generate(length=3)
        assert result == "generated text"

@pytest.mark.asyncio
async def test_client_generate_failure():
    client = GeneratorClient(base_url="http://test")
    async with respx.mock:
        respx.post("http://test/generate").mock(return_value=Response(404))
        
        result = await client.generate(length=3)
        assert result is None
