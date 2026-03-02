# TG Guard Reminder Bot (Python)

Бот для группового чата Telegram: каждые 2 часа с **09:00 до 21:00 по МСК** отправляет шутливое напоминание и отмечает пользователя `@TARGET_NICK` (по умолчанию `@SerjoGrass`).

## Как работает
1. Ты добавляешь бота в нужный **групповой чат**.
2. Админ чата пишет команду **/setchat** — бот запоминает этот чат (сохраняет `chat_id` в `data.json`).
3. Дальше бот сам постит напоминания по расписанию.

Команды:
- `/setchat` — (только админы) назначить текущий чат для напоминаний
- `/status` — показать текущие настройки (назначен ли чат)
- `/ping` — разово отправить напоминание в текущий чат

## Подготовка
### 1) Создай бота у @BotFather
Скопируй токен.

### 2) Переменные окружения
На сервере создай файл `.env` рядом с `bot.py`:

```env
BOT_TOKEN=...
TARGET_NICK=SerjoGrass
```

> `TARGET_NICK` можно не задавать — по умолчанию `SerjoGrass`.

## Запуск локально
```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env   # и отредактируй
python bot.py
```

## Деплой на сервер (простой вариант)
1. Установи Python 3.10+.
2. Склонируй репозиторий:
   ```bash
   git clone <твоя_ссылка_на_git>
   cd tg-guard-reminder-bot
   ```
3. Создай venv и установи зависимости:
   ```bash
   python -m venv .venv
   source .venv/bin/activate
   pip install -r requirements.txt
   ```
4. Создай `.env` (НЕ коммить его) и запусти:
   ```bash
   python bot.py
   ```

## Заливка на GitHub
1. Создай новый репозиторий на GitHub (например `tg-guard-reminder-bot`).
2. В папке проекта:
   ```bash
   git init
   git add .
   git commit -m "Initial commit"
   git branch -M main
   git remote add origin <URL_репозитория>
   git push -u origin main
   ```
3. На сервере положи `.env` рядом с `bot.py`.

## Настройка чата
- Добавь бота в нужную группу.
- Дай ему право **писать сообщения**.
- Выполни `/setchat` **админом** в этом чате.
- Проверь `/ping` или `/status`.

## Где хранятся настройки
- `data.json` создаётся рядом с `bot.py` автоматически (в `.gitignore`).
