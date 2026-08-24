# Neurodan

Neurodan is a Telegram bot that trains a variable-order Markov chain on the messages it sees in each chat and generates new text from it. It is a rewrite of the original Python microservice, split into a bot service (Telegram interaction) and a generator service (training, generation, stylization).

## Language

### Training and learning

**Train**:
The act of extracting n-grams from a message and persisting them to the model for a chat. The bot trains on every non-command message.
_Avoid_: Learn, ingest

**Training exclusion**:
Text that is never trained on. Command invocations (text starting with `/`) are excluded.
_Avoid_: Ignored text, skipped text

### Conversation

**Chat**:
A Telegram room identified by a `chat_id`. The bot scopes its model, reply decisions, and settings per chat.
_Avoid_: Conversation (for the room), group

**Conversation context**:
The recent-message sliding window (last ~15 messages) used to seed a reply so it answers coherently in the current discussion. Distinct from Chat, which is the room.
_Avoid_: Context (ambiguous with the seed), window

**Mention**:
A `@`-addressing of the bot that forces a reply and is stripped from the text before training.
_Avoid_: Tag, ping

### Replying

**Reply decision**:
Whether the bot chooses to speak at all — driven by a stacking probability that grows the longer the bot is silent, and forced by a Mention.
_Avoid_: Reply (for the decision)

**Reply event**:
A single bout of the bot speaking: a chain of one or more reply messages, each seeded by the previous one.
_Avoid_: Reply (for the event)

**Reply message**:
One message within a Reply event.
_Avoid_: Reply (for the message)

**ChanceBasedReply**:
A reply that is sent as a bare kaomoji / emoji / Russian net-speak token instead of generated text, chosen with a fixed probability when a reply fires.
_Avoid_: Reaction, emote, bare reply

### Generation

**Generation prefix**:
The explicit starting words of generated text, usually supplied by the user (e.g. `/generate <words>`).
_Avoid_: Seed, prompt

**Conversation context anchor**:
The suffix of the conversation context that appears in the model and becomes the implicit starting point when no Generation prefix is given.
_Avoid_: Bridge, keyword bridge

**Length preset**:
A named configuration (`short`, `medium`, `long`, `unbounded`) that is a probability distribution over n-gram order, controlling how long (and therefore how coherent) generated text is.
_Avoid_: Length, length config

**Similarity guard**:
The mechanism that regenerates generated text up to a few times when its embedding is too close (cosine > 0.85) to existing text, to avoid near-duplicate output.
_Avoid_: Dedup, semantic deduplication

### Stylization

**Stylization**:
The probabilistic post-processing passes applied to generated text to give it emotional weight. The umbrella for all passes.
_Avoid_: Styling (the individual passes)

**Stylization pass**:
A single toggleable post-processing effect applied to generated text. The toggles are the current set of passes.
_Avoid_: Styling feature, feature, emote

**Flair**:
The stylization pass that appends a trailing emoji, kaomoji, or Russian net-speak token (e.g. `😂`, `кек`, `лол`).
_Avoid_: Emote, emotes, reaction

### Anecdotes

**Anecdote model**:
A separate precomputed Markov model trained on a corpus of cleaned Russian jokes, distinct from the per-chat model.
_Avoid**: Corpus, anecdote corpus

**Anecdote generation**:
The generation mode that continues the user's Generation prefix from the Anecdote model, ignoring the chat's transitions entirely. Currently isolated from the chat model.
_Avoid**: Anecdote (for the mode), /anec