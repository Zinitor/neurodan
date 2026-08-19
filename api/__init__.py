from os import listdir, remove
from os.path import getsize
from pathlib import Path
from random import randint
from re import findall

from aiofile import AIOFile

from api import util

from .models import Model


async def improve_result(result: str):
    rnd = randint(0, 4)
    if rnd == 0:  # ну чиста для прекола иногда бот пишет постироничнее....
        return result

    elif rnd == 1:
        return result.upper()  # ну чиста для прекола в иногда бот орет кричит....

    else:
        improved_result = ""

        for i in range(len(result)):
            if i == 0:
                improved_result += result[i].upper()

            elif i > 1:
                if result[i - 1] == " " and result[i - 2] in ["?", ".", "!"]:
                    improved_result += result[i].upper()

                else:
                    improved_result += result[i]

            else:
                improved_result += result[i]

        return improved_result


class Stats:
    @staticmethod
    async def get(peer_id):
        files = [f"messages/{filename}" for filename in listdir("messages")]
        global_size = round(sum([getsize(file) for file in files]) / 1048576, 2)
        try:
            local_size = round(getsize(f"messages/{peer_id}.raw") / 1048576, 2)

        except FileNotFoundError:
            local_size = 0

        return len(files), global_size, local_size


async def censor_result(result: str):
    links = util.remove_duplicates(
        findall(r"[^ (){\}\[\]\'\";]+\.[^ (){\}\[\]\'\";]+", result)
    )

    for link in links:
        result = result.replace(link, "[ссылка удалена]")

    return result


async def escape_string(string: str):
    return string.replace(";", r"\;")


async def unescape_string(string: str):
    return string.replace(r"\;", ";")


async def parse_raw(raw: str):
    result: list[str] = []
    start = 0

    for i in range(len(raw)):
        if i != 0 and raw[i] == ";" and raw[i - 1] != "\\":
            result.append(raw[start:i])

            start = i + 1

    return [await unescape_string(message) for message in result]


class MessagesStorage:
    def __init__(self):
        self.path: str | Path = "messages.json"

    async def wipe(self):
        try:
            remove(self.path)
            return True

        except ValueError:
            return False

    async def get(self):
        raw_text: list[str] = []
        with open(self.path, "r") as file:
            content = Model.model_validate_json(file.read())
            for message in content.messages:
                # Case 1: text is a block of string
                if isinstance(message.text, str):
                    raw_text.append(message.text)

                # Case 2: text is a list of mixed parts
                elif isinstance(message.text, list):
                    for part in message.text:
                        if isinstance(part, str):
                            raw_text.append(part)
                        elif isinstance(part, dict) and "text" in part:
                            raw_text.append(part["text"])

        print(raw_text)

        # messages = []
        # for message in raw_text:
        #     parsed_result = await parse_raw(message)
        #     # Fix: Ensure we flatten the list if parse_raw returns a list
        #     if isinstance(parsed_result, list):
        #         messages.extend(parsed_result)
        #     else:
        #         messages.append(parsed_result)

        # msg_string = " ".join(messages)
        # print(msg_string)
        # messages = messages[::-1][0:150]  # в эфире рубрика ЕХперементы....
        return raw_text

    async def push(self, messages: list[str]):
        async with AIOFile(self.path, "a", encoding="utf-8") as file:
            line = ";".join([await escape_string(message) for message in messages])
            _ = await file.write(line + ";")
