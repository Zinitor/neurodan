import asyncio
import logging
from aiogram import Bot, Dispatcher
from src.config import settings
from src.handlers.messages import router as msg_router

async def main():
    # Configure logging
    logging.basicConfig(level=logging.INFO)
    
    bot = Bot(token=settings.bot_token)
    dp = Dispatcher()
    
    # Register handlers
    dp.include_router(msg_router)
    
    print(f"Bot started with token: {settings.bot_token[:5]}...")
    try:
        await dp.start_polling(bot)
    finally:
        await bot.session.close()

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
