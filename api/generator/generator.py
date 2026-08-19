from collections import defaultdict
from random import choice

# Специальные токены для обозначения начала и конца фразы
_START = "___start___"
_END = "___end___"


def generate(samples: list, tries_count: int, size: int):
    if not samples:
        return None

    # Таблица переходов: ключ — это пара слов (w1, w2), значение — список возможных следующих слов
    frame_map = defaultdict(list)
    # Список пар слов, с которых реально начинались сообщения в чате
    start_pairs = []

    # Шаг 1: Обучение на логах чата
    for sample in samples:
        # Убираем лишние пробелы и пустые строки
        words = [w.strip() for w in sample.split(" ") if w.strip()]
        if len(words) < 1:
            continue

        # Формируем цепочку с фейковыми стартовыми токенами для 2-го порядка
        chain = [_START, _START] + words + [_END]

        # Запоминаем, с какой пары слов начинается реальный текст
        if len(words) >= 1:
            start_pairs.append((_START, words[0]))

        # Строим карту переходов по тройкам слов
        for i in range(len(chain) - 2):
            w1, w2, w3 = chain[i], chain[i + 1], chain[i + 2]
            frame_map[(w1, w2)].append(w3)

    if not start_pairs:
        return None

    # Шаг 2: Генерация бреда
    for _ in range(tries_count):
        # Случайно выбираем живое начало фразы из чата
        current_pair = choice(start_pairs)

        # Если сообщение состояло всего из одного слова, и сразу конец
        if frame_map[current_pair] and frame_map[current_pair][0] == _END:
            result = [current_pair[1]]
        else:
            result = [current_pair[1]]

            # Генерируем цепочку, пока не наткнемся на _END
            while True:
                possible_next = frame_map[current_pair]
                if not possible_next:
                    break

                next_word = choice(possible_next)
                if next_word == _END:
                    break

                result.append(next_word)
                # Сдвигаем окно: теперь пара — это старое второе слово и новое сгенерированное
                current_pair = (current_pair[1], next_word)

        str_result = " ".join(result)

        # Шаг 3: Проверка на оригинальность и длину фразы
        if str_result not in samples:
            # Упрощенная проверка размера (количество слов)
            word_count = len(result)

            if size == 0:  # Любой размер
                if word_count <= 100:
                    return str_result
            elif size == 1:  # Короткие (рофлы, выкрики)
                if 1 <= word_count <= 3:
                    return str_result
            elif size == 2:  # Средние (базовые пасты)
                if 4 <= word_count <= 8:
                    return str_result
            elif size == 3:  # Длинные (шизопосты)
                if 9 <= word_count <= 100:
                    return str_result
            else:
                raise ValueError("Size must be 0, 1, 2 or 3")

    return None
