from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from src.core.database import Word, Transition, Sample
import random
import json
import numpy as np
from sentence_transformers import SentenceTransformer

_START = "___start___"
_END = "___end___"

class BrainState:
    """Global state to hold the ML model and the vector cache across requests."""
    def __init__(self):
        self.model = SentenceTransformer('all-MiniLM-L6-v2')
        self.vector_cache = None # numpy array of shape (N, 384)

    async def load_cache(self, session: AsyncSession):
        result = await session.execute(select(Sample.embedding))
        embeddings = result.scalars().all()
        
        if not embeddings:
            self.vector_cache = np.empty((0, 384), dtype=np.float32)
        else:
            self.vector_cache = np.array([json.loads(e) for e in embeddings], dtype=np.float32)

class MarkovEngine:
    def __init__(self, session: AsyncSession, state: BrainState):
        self.session = session
        self.state = state

    async def train(self, text: str, commit: bool = True):
        words = [w.strip() for w in text.split(" ") if w.strip()]
        if not words:
            return

        # Store the embedding of the original sample for originality checks
        embedding = self.state.model.encode(text).tolist()
        self.session.add(Sample(embedding=json.dumps(embedding)))
        
        # Update cache immediately
        new_vec = np.array(embedding, dtype=np.float32).reshape(1, -1)
        if self.state.vector_cache is None or self.state.vector_cache.size == 0:
            self.state.vector_cache = new_vec
        else:
            self.state.vector_cache = np.vstack([self.state.vector_cache, new_vec])

        # 2nd order Markov: chain = [_START, _START] + words + [_END]
        chain = [_START, _START] + words + [_END]

        for i in range(len(chain) - 2):
            w1_text, w2_text, w3_text = chain[i], chain[i + 1], chain[i + 2]

            w1_id = await self._get_or_create_word(w1_text)
            w2_id = await self._get_or_create_word(w2_text)
            w3_id = await self._get_or_create_word(w3_text)

            # Upsert transition (w1, w2) -> w3
            result = await self.session.execute(
                select(Transition).filter_by(word1_id=w1_id, word2_id=w2_id, next_word_id=w3_id)
            )
            transition = result.scalar_one_or_none()

            if transition:
                transition.weight += 1
            else:
                self.session.add(Transition(word1_id=w1_id, word2_id=w2_id, next_word_id=w3_id, weight=1))
        
        if commit:
            await self.session.commit()

    async def generate(self, length: int, seed: str | None = None, size: int = 0) -> str | None:
        tries_count = 10
        
        for _ in range(tries_count):
            start_word_id = await self._get_or_create_word(_START)
            
            result = await self.session.execute(
                select(Transition.word2_id, Transition.next_word_id)
                .filter_by(word1_id=start_word_id)
            )
            start_pairs = result.all()
            
            if not start_pairs:
                return None

            pair = random.choice(start_pairs)
            current_w1 = start_word_id
            current_w2 = pair[0]
            
            first_word_text = await self._get_word_text(current_w2)
            if first_word_text == _START:
                continue

            result_words = [first_word_text]

            while True:
                res = await self.session.execute(
                    select(Transition.next_word_id, Transition.weight)
                    .filter_by(word1_id=current_w1, word2_id=current_w2)
                )
                transitions = res.all()

                if not transitions:
                    break

                total_weight = sum(t.weight for t in transitions)
                r = random.uniform(0, total_weight)
                upto = 0
                next_word_id = None
                for tid, weight in transitions:
                    if upto + weight >= r:
                        next_word_id = tid
                        break
                    upto += weight
                
                next_word_text = await self._get_word_text(next_word_id)
                if next_word_text == _END:
                    break
                
                result_words.append(next_word_text)
                current_w1 = current_w2
                current_w2 = next_word_id

            str_result = " ".join(result_words)
            word_count = len(result_words)

            if await self._is_too_similar(str_result):
                continue

            if size == 0:
                if word_count <= 100: return str_result
            elif size == 1:
                if 1 <= word_count <= 3: return str_result
            elif size == 2:
                if 4 <= word_count <= 8: return str_result
            elif size == 3:
                if 9 <= word_count <= 100: return str_result
            else:
                raise ValueError("Size must be 0, 1, 2 or 3")

        return None

    async def _is_too_similar(self, text: str, threshold: float = 0.95) -> bool:
        if self.state.vector_cache is None or self.state.vector_cache.size == 0:
            return False
        
        current_vector = self.state.model.encode([text])[0].astype(np.float32)
        similarities = np.dot(self.state.vector_cache, current_vector)
        
        return np.any(similarities > threshold)

    async def _get_or_create_word(self, text: str) -> int:
        result = await self.session.execute(select(Word).filter_by(text=text))
        word = result.scalar_one_or_none()
        if word:
            return word.id
        
        word = Word(text=text)
        self.session.add(word)
        await self.session.flush()
        return word.id

    async def _get_word_text(self, word_id: int) -> str:
        result = await self.session.execute(select(Word.text).filter_by(id=word_id))
        return result.scalar_one()
