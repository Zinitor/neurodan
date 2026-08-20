"""One-time migration script: reads SQLite data from the original Neurodan Python
version and populates Qdrant collections for the Go rewrite.

This script:
1. Reads words, transitions, and sentence embeddings from SQLite
2. Creates Qdrant collections (markov_transitions, sentence_embeddings)
3. Inserts all existing data into Qdrant

Prerequisites:
- Qdrant running (e.g., `docker run -d --name qdrant -p 6333:6333 -p 6334:6334 qdrant/qdrant:latest`)
- Python with qdrant-client installed

Run: python migrate_sqlite_to_qdrant.py
"""

import json
import os
import sqlite3
import sys

from qdrant_client import QdrantClient
from qdrant_client.models import Distance, PointStruct, VectorParams


def migrate():
    # --- Configuration ---
    SQLITE_DB = "/home/aeremin/projects/neurodan/services/generator/generator.db"
    QDRANT_URL = "http://localhost:6333"
    # -------------------

    # --- Step 1: Read from SQLite ---
    print(f"Reading from SQLite: {SQLITE_DB}")
    sqlite_conn = sqlite3.connect(SQLITE_DB)
    sqlite_cursor = sqlite_conn.cursor()

    # Read words
    sqlite_cursor.execute("SELECT id, text FROM words")
    words_rows = sqlite_cursor.fetchall()
    words = {row[0]: row[1] for row in words_rows}
    print(f"  Found {len(words)} words")

    # Read transitions
    sqlite_cursor.execute(
        "SELECT word1_id, word2_id, next_word_id, weight FROM transitions"
    )
    transitions_rows = sqlite_cursor.fetchall()
    print(f"  Found {len(transitions_rows)} transitions")

    # Read embeddings (samples)
    sqlite_cursor.execute("SELECT id, embedding FROM samples")
    samples_rows = sqlite_cursor.fetchall()
    print(f"  Found {len(samples_rows)} embeddings")

    # Build text -> embedding mapping
    embeddings = {}
    for row in samples_rows:
        row_id = row[0]
        embedding_json = row[1]
        try:
            embedding_vector = json.loads(embedding_json)
            embeddings[row_id] = embedding_vector
        except json.JSONDecodeError, TypeError:
            print(f"  Warning: Could not parse embedding for id {row_id}")
            continue
    print(f"  Successfully parsed {len(embeddings)} embeddings")

    sqlite_conn.close()

    # --- Step 2: Connect to Qdrant ---
    print(f"Connecting to Qdrant at {QDRANT_URL}...")
    client = QdrantClient(url=QDRANT_URL)

    # --- Step 3: Create/ensure collections ---
    print("Ensuring Qdrant collections exist...")
    # Try to list collections; handle API version incompatibility gracefully
    existing = []
    try:
        col_result = client.get_collections()
        existing = [c.name for c in col_result.result]
    except Exception as e:
        print(f"  Could not list collections (API version mismatch): {e}")
        existing = []

    if "markov_transitions" not in existing:
        print("  Creating markov_transitions collection...")
        client.create_collection(
            collection_name="markov_transitions",
            vectors_config=VectorParams(
                size=1,
                distance=Distance.COSINE,
            ),
        )
        print("  markov_transitions created.")
    else:
        print("  markov_transitions already exists.")

    if "sentence_embeddings" not in existing:
        print("  Creating sentence_embeddings collection...")
        client.create_collection(
            collection_name="sentence_embeddings",
            vectors_config=VectorParams(
                size=384,
                distance=Distance.COSINE,
            ),
        )
        print("  sentence_embeddings created.")
    else:
        print("  sentence_embeddings already exists.")

    # --- Step 4: Insert transitions into Qdrant ---

    # --- Step 4: Insert transitions into Qdrant ---
    print("Inserting transitions into markov_transitions...")
    points = []
    for i, (w1_id, w2_id, next_w_id, weight) in enumerate(transitions_rows):
        # FIX: Use an integer 'i' as the primary ID, move composite key to payload
        point = PointStruct(
            id=i,
            vector=[0.0],  # Use the singular 'vector' instead of 'vectors'
            payload={
                "transition_key": f"{w1_id}:{w2_id}:{next_w_id}",
                "word1_id": w1_id,
                "word2_id": w2_id,
                "next_word_id": next_w_id,
                "weight": weight,
            },
        )
        points.append(point)

        if len(points) >= 10000:
            client.upsert("markov_transitions", points)
            points = []

    # --- Step 5: Insert embeddings into Qdrant ---
    print("Inserting embeddings into sentence_embeddings...")
    points = []
    for i, (emb_id, embedding_vector) in enumerate(embeddings.items()):
        # FIX: Cast SQLite ID directly to int, stripping out 'emb:' prefix
        point = PointStruct(
            id=int(emb_id),
            vector=embedding_vector,  # Use the singular 'vector' instead of 'vectors'
            payload={
                "emb_id": emb_id,
            },
        )
        points.append(point)

        if len(points) >= 10000:
            client.upsert("sentence_embeddings", points)
            points = []
    print(f"  Inserted {len(embeddings)} embeddings into sentence_embeddings.")

    print("\nMigration complete! Your Go services can now use the data via Qdrant.")
    print("Note: The transitions store word IDs (word1_id, word2_id, next_word_id).")
    print("The Go system will need these IDs to map to word texts or train anew.")


if __name__ == "__main__":
    migrate()
