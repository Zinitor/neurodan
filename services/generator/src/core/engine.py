from sqlalchemy import select, update, insert
from sqlalchemy.ext.asyncio import AsyncSession
from src.core.database import Word, Transition

class MarkovEngine:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def train(self, text: str):
        words = text.split()
        if len(words) < 2:
            return

        for i in range(len(words) - 1):
            w1_text = words[i]
            w2_text = words[i + 1]

            # Ensure words exist and get their IDs
            w1_id = await self._get_or_create_word(w1_text)
            w2_id = await self._get_or_create_word(w2_text)

            # Update or create transition
            result = await self.session.execute(
                select(Transition).filter_by(from_word_id=w1_id, to_word_id=w2_id)
            )
            transition = result.scalar_one_or_none()

            if transition:
                transition.weight += 1
            else:
                self.session.add(Transition(from_word_id=w1_id, to_word_id=w2_id, weight=1))
        
        await self.session.commit()

    async def generate(self, length: int, seed: str | None = None) -> str:
        if length <= 0:
            return ""

        # Pick starting word
        current_word_id = await self._get_starting_word_id(seed)
        if current_word_id is None:
            return ""

        output_ids = [current_word_id]

        for _ in range(length - 1):
            # Find all transitions from the current word
            result = await self.session.execute(
                select(Transition.to_word_id, Transition.weight)
                .filter_by(from_word_id=current_word_id)
            )
            transitions = result.all()

            if not transitions:
                # Fallback: pick a random word from the database
                current_word_id = await self._get_random_word_id()
            else:
                # Weighted random selection
                total_weight = sum(t.weight for t in transitions)
                import random
                r = random.uniform(0, total_weight)
                upto = 0
                for tid, weight in transitions:
                    if upto + weight >= r:
                        current_word_id = tid
                        break
                    upto += weight
            
            output_ids.append(current_word_id)

        # Convert IDs back to text
        result = await self.session.execute(
            select(Word.text).where(Word.id.in_(output_ids))
        )
        # Note: .in_ doesn't guarantee order, so we fetch and map
        all_words = await self.session.execute(select(Word))
        word_map = {w.id: w.text for w in all_words.scalars()}
        
        return " ".join(word_map.get(wid, "") for wid in output_ids)

    async def _get_or_create_word(self, text: str) -> int:
        result = await self.session.execute(select(Word).filter_by(text=text))
        word = result.scalar_one_or_none()
        if word:
            return word.id
        
        word = Word(text=text)
        self.session.add(word)
        await self.session.flush()
        return word.id

    async def _get_starting_word_id(self, seed: str | None) -> int | None:
        if seed:
            result = await self.session.execute(select(Word).filter_by(text=seed))
            word = result.scalar_one_or_none()
            if word:
                return word.id
        
        # Fallback: random word
        return await self._get_random_word_id()

    async def _get_random_word_id(self) -> int | None:
        import random
        result = await self.session.execute(select(Word))
        words = result.scalars().all()
        if not words:
            return None
        return random.choice(words).id
