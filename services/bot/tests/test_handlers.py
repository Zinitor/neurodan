import pytest
from unittest.mock import AsyncMock, MagicMock
from aiogram import types
from src.handlers.messages import handle_message, cmd_generate

@pytest.mark.asyncio
async def test_handle_message_trains_client(monkeypatch):
    # Mock the GeneratorClient
    mock_client = AsyncMock()
    monkeypatch.setattr("src.handlers.messages.client", mock_client)
    
    # Mock a Telegram message
    message = MagicMock(spec=types.Message)
    message.text = "hello world"
    
    await handle_message(message)
    
    # Verify client.train was called with the message text
    mock_client.train.assert_called_once_with("hello world")

@pytest.mark.asyncio
async def test_cmd_generate_success(monkeypatch):
    # Mock the GeneratorClient
    mock_client = AsyncMock()
    mock_client.generate.return_value = "generated response"
    monkeypatch.setattr("src.handlers.messages.client", mock_client)
    
    # Mock a Telegram message
    message = MagicMock(spec=types.Message)
    message.text = "/generate seed"
    message.answer = AsyncMock()
    
    await cmd_generate(message)
    
    # Verify client.generate was called with correct args
    mock_client.generate.assert_called_once_with(length=10, seed="seed")
    # Verify bot answered with the response
    message.answer.assert_called_once_with("generated response")

@pytest.mark.asyncio
async def test_cmd_generate_failure(monkeypatch):
    # Mock the GeneratorClient to return None
    mock_client = AsyncMock()
    mock_client.generate.return_value = None
    monkeypatch.setattr("src.handlers.messages.client", mock_client)
    
    # Mock a Telegram message
    message = MagicMock(spec=types.Message)
    message.text = "/generate"
    message.answer = AsyncMock()
    
    await cmd_generate(message)
    
    # Verify bot answered with error message
    message.answer.assert_called_once_with("I'm having trouble thinking right now. Try again later!")
