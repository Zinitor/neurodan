#!/usr/bin/env python3
"""Clean the raw anecdote dataset into a training-ready corpus.

Reads one-joke-per-line text, drops noise (empty lines, URLs, too-short or
non-Cyrillic entries), and dedupes while preserving order.

Usage:
    python scripts/filter_anecdotes.py [INPUT] [OUTPUT]

Input defaults to data/anecdotes/anecdotes.txt, output to
data/anecdotes/anecdotes_clean.txt.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

DEFAULT_INPUT = Path(__file__).resolve().parent.parent / "data" / "anecdotes" / "anecdotes.txt"
DEFAULT_OUTPUT = DEFAULT_INPUT.with_name("anecdotes_clean.txt")

MIN_CHARS = 15
MAX_CHARS = 500
MIN_WORDS = 3

_URL_RE = re.compile(r"https?://\S+")


def clean(text: str) -> str:
    """Return the cleaned joke, or None if the line is noise."""
    line = text.strip()
    if not line:
        return None
    # Normalize internal whitespace (tabs, multiple spaces, non-breaking spaces).
    line = " ".join(line.split())
    if not (MIN_CHARS <= len(line) <= MAX_CHARS):
        return None
    if _URL_RE.search(line):
        return None
    if len(line.split()) < MIN_WORDS:
        return None
    # Require at least a few Cyrillic letters so we don't keep pure-emoji
    # or Latin noise.
    if len(re.findall(r"[а-яА-ЯёЁ]", line)) < 5:
        return None
    return line


def main() -> int:
    src = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_INPUT
    dst = Path(sys.argv[2]) if len(sys.argv) > 2 else DEFAULT_OUTPUT

    seen: set[str] = set()
    kept = 0
    dropped = 0
    with src.open(encoding="utf-8") as f, dst.open("w", encoding="utf-8") as out:
        for line in f:
            joke = clean(line)
            if joke is None:
                dropped += 1
                continue
            if joke in seen:
                dropped += 1
                continue
            seen.add(joke)
            out.write(joke + "\n")
            kept += 1

    print(f"Read {src}")
    print(f"Wrote {kept} jokes to {dst} ({dst.stat().st_size / 1024 / 1024:.1f} MB)")
    print(f"Dropped {dropped} lines")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
