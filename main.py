import json
import random
from pathlib import Path

from models import Model


def generate_markov_text(dataset: list[str], length: int) -> str:
    if not dataset or length <= 0:
        return ""

    # Split all inputs into a single continuous list of words
    words = " ".join(dataset).split()
    if not words:
        return ""

    # Build the transitions dictionary
    transitions = {}
    for i in range(len(words) - 1):
        current_word = words[i]
        next_word = words[i + 1]
        if current_word not in transitions:
            transitions[current_word] = []
        transitions[current_word].append(next_word)

    # Generate the chain
    current_word = random.choice(words)
    output = [current_word]

    for _ in range(length - 1):
        # Fallback to a random word if the current word has no known followers
        if current_word not in transitions or not transitions[current_word]:
            current_word = random.choice(words)
        else:
            current_word = random.choice(transitions[current_word])
        output.append(current_word)

    return " ".join(output)


def main():
    with open("result.json", "r") as file:
        content = Model.model_validate_json(file.read())
        raw_text = []

        for message in content.messages:
            if isinstance(message.text, list):
                for part in message.text:
                    if isinstance(part, str):
                        raw_text.append(part)
                    elif isinstance(part, dict) and "text" in part:
                        raw_text.append(part["text"])
            elif isinstance(message.text, str):
                raw_text.append(message.text)

        strings = generate_markov_text(raw_text, 10)
        print(strings)
main()
