from aiogram import Router, types, F
from aiogram.filters import Command
from src.client.generator_client import GeneratorClient
from src.config import settings

router = Router()
client = GeneratorClient()

@router.message(F.text)
async def handle_message(message: types.Message):
    # 1. Train the brain on every message
    await client.train(message.text)

@router.message(Command("generate"))
async def cmd_generate(message: types.Message):
    # Extract seed if provided: /generate seed_word
    args = message.text.split(maxsplit=1)
    seed = args[1] if len(args) > 1 else None
    
    # Generate a response (default length 10)
    response = await client.generate(length=10, seed=seed)
    
    if response:
        await message.answer(response)
    else:
        await message.answer("I'm having trouble thinking right now. Try again later!")
