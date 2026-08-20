import asyncio
import json
from pathlib import Path
from src.core.database import async_session, init_db
from src.core.engine import MarkovEngine
from src.core.models import Model

async def seed_data():
    # Path to the result.json file in the root
    # Since we run from services/generator, we go up two levels
    json_path = Path("result.json")
    if not json_path.exists():
        print(f"Error: {json_path} not found")
        return

    # Load and parse the JSON
    with open(json_path, "r") as f:
        data = Model.model_validate_json(f.read())

    # Initialize DB
    await init_db()

    async with async_session() as session:
        engine = MarkovEngine(session)
        print(f"Seeding {len(data.messages)} messages directly into DB...")

        for i, msg in enumerate(data.messages):
            # Extract text from message
            raw_text = []
            if isinstance(msg.text, list):
                for part in msg.text:
                    if isinstance(part, str):
                        raw_text.append(part)
                    elif hasattr(part, 'text'):
                        raw_text.append(part.text)
            elif isinstance(msg.text, str):
                raw_text.append(msg.text)

            full_text = " ".join(raw_text)
            if full_text:
                # Use commit=False to batch all updates into one transaction
                await engine.train(full_text, commit=False)

            if (i + 1) % 100 == 0:
                print(f"Processed {i + 1}/{len(data.messages)} messages...")

        # Final commit for the entire batch
        await session.commit()

    print("Seeding complete!")

if __name__ == "__main__":
    asyncio.run(seed_data())
