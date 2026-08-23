#!/usr/bin/env python3
"""Download the Russian jokes dataset used to seed the /anec command.

Fetches IgorVolochay/russian_jokes (MIT license) from Hugging Face Hub and
saves it as one-joke-per-line text.

Usage:
    python scripts/download_anecdotes.py [OUTPUT] [URL]

Output defaults to data/anecdotes/anecdotes.txt.
"""

from __future__ import annotations

import hashlib
import sys
import time
import urllib.request
from pathlib import Path

DEFAULT_URL = (
    "https://huggingface.co/datasets/IgorVolochay/russian_jokes/"
    "resolve/main/dataset.txt?download=true"
)
DEFAULT_OUTPUT = Path(__file__).resolve().parent.parent / "data" / "anecdotes" / "anecdotes.txt"

_HEADERS = {"User-Agent": "neurodan-anecdote-seed/0.1"}
_MAX_ATTEMPTS = 3

# SHA-256 of the expected dataset (as downloaded from the default URL).
EXPECTED_SHA256 = "734a77560ebaf508c8374944449b112d43191a502e77a89faac4760bb37ac894"


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def download(url: str, dest: Path) -> None:
    dest.parent.mkdir(parents=True, exist_ok=True)
    print(f"Downloading {url}")
    last_err: Exception | None = None
    for attempt in range(1, _MAX_ATTEMPTS + 1):
        try:
            req = urllib.request.Request(url, headers=_HEADERS)
            with urllib.request.urlopen(req, timeout=300) as src, dest.open("wb") as out:
                while chunk := src.read(1024 * 1024):
                    out.write(chunk)
            break
        except Exception as e:  # noqa: BLE001 - retry any transient network error
            last_err = e
            print(f"  attempt {attempt}/{_MAX_ATTEMPTS} failed: {e}", file=sys.stderr)
            if attempt < _MAX_ATTEMPTS:
                time.sleep(2)
    else:
        raise SystemExit(f"failed to download after {_MAX_ATTEMPTS} attempts: {last_err}")

    print(f"Saved to {dest}")

    if EXPECTED_SHA256:
        actual = sha256_file(dest)
        if actual != EXPECTED_SHA256:
            print(
                f"WARNING: SHA-256 mismatch\n  expected {EXPECTED_SHA256}\n  got      {actual}",
                file=sys.stderr,
            )
        else:
            print("Checksum OK")


def main() -> int:
    output = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_OUTPUT
    url = sys.argv[2] if len(sys.argv) > 2 else DEFAULT_URL
    download(url, output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
