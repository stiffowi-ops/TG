
import os
import json
import random
import logging
import time
import asyncio
from pathlib import Path
from zoneinfo import ZoneInfo

from dotenv import load_dotenv
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger

from telegram import Update, InputFile
from telegram.constants import ChatMemberStatus
from telegram.ext import (
    Application,
    CommandHandler,
    MessageHandler,
    ContextTypes,
    filters,
)

BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data.json"

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
log = logging.getLogger("guard-bot")

BOT_TOKEN = os.getenv("BOT_TOKEN")
TARGET_NICK = os.getenv("TARGET_NICK", "SerjoGrass").lstrip("@").strip()

START_PHOTO_FILE_ID = os.getenv("START_PHOTO_FILE_ID", "").strip()
START_PHOTO_URL = os.getenv("START_PHOTO_URL", "").strip()
START_PHOTO_PATH = os.getenv("START_PHOTO_PATH", "").strip()

if not BOT_TOKEN:
    raise RuntimeError("BOT_TOKEN is not set")

TZ = ZoneInfo("Europe/Moscow")

REMINDER_TEMPLATES = [
    "Ему так грустно — он один охраняет коморку. @{nick}, напиши охраннику 🥺",
    "Охранник смотрит в даль и вздыхает. @{nick}, напиши охраннику 😄",
    "Коморка под надежной охраной… но без твоего сообщения скучно. @{nick}, напиши охраннику 🙂",
    "Сигнал тревоги: охраннику не хватает твоего «привет». @{nick}, напиши охраннику 🚨",
]

NIGHT_TEMPLATES = [
    "Коморка закрывается — сон твой начинается. Спокойной ночи, @{nick} 🌙",
    "Охранник гасит свет. Спокойной ночи, @{nick} 😴",
]

SPAM_WARNINGS = [
    "А-ну, не спамь, а то заберу в коморку с ночёвкой 😠",
    "Полегче! Это чат, а не пулемёт 😡",
]

SPAM_WINDOW_SECONDS = 5
SPAM_COOLDOWN_SECONDS = 120

last_messages = {}
last_spam_warn_ts = 0
awaiting_photoid = set()


def load_data():
    if DATA_FILE.exists():
        try:
            return json.loads(DATA_FILE.read_text(encoding="utf-8"))
        except Exception:
            return {}
    return {}


def save_data(data):
    DATA_FILE.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def get_target_chat_id():
    data = load_data()
    return data.get("chat_id")


def build_from(templates, nick):
    return random.choice(templates).format(nick=nick)


def is_private(update):
    return update.effective_chat and update.effective_chat.type == "private"


async def fake_check_sequence(update, final_text):
    msg = await update.message.reply_text("Выполняется проверка...")
    await asyncio.sleep(0.7)
    await msg.edit_text("Пип... Пип...")
    await asyncio.sleep(0.7)
    await msg.edit_text("Вычисляем....")
    await asyncio.sleep(0.8)
    await msg.edit_text(final_text)


async def require_group_member(update, context):
    chat_id = get_target_chat_id()
    if not chat_id:
        await fake_check_sequence(update, "Петушок вычислен: коморка ещё не привязана 🐓")
        return False

    try:
        member = await context.bot.get_chat_member(chat_id, update.effective_user.id)
        if member.status in (ChatMemberStatus.LEFT, ChatMemberStatus.KICKED):
            await fake_check_sequence(update, "Петушок вычислен: ты вне чата 🐓")
            return False
        return True
    except Exception:
        await update.message.reply_text(
            "Не могу проверить участие в коморке 😕\n"
            "Убедись, что бот добавлен в группу и выполнен /setchat."
        )
        return False


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    if is_private(update):
        if not await require_group_member(update, context):
            return

    caption = "Приветствую тебя в коморке 🛡️"

    if START_PHOTO_FILE_ID:
        await update.message.reply_photo(photo=START_PHOTO_FILE_ID, caption=caption)
    else:
        await update.message.reply_text(caption)


async def cmd_setchat(update: Update, context: ContextTypes.DEFAULT_TYPE):
    member = await context.bot.get_chat_member(update.effective_chat.id, update.effective_user.id)
    if member.status not in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER):
        await update.message.reply_text("Только для админов.")
        return

    data = load_data()
    data["chat_id"] = update.effective_chat.id
    save_data(data)

    await update.message.reply_text(
        f"Чат назначен ✅\nCHAT_ID: {update.effective_chat.id}\nЦель: @{TARGET_NICK}"
    )


async def handle_text(update: Update, context: ContextTypes.DEFAULT_TYPE):
    global last_spam_warn_ts

    chat_id = get_target_chat_id()
    if not chat_id or update.effective_chat.id != chat_id:
        return

    user = update.effective_user
    if not user or user.username != TARGET_NICK:
        return

    now = time.time()
    prev = last_messages.get(user.id)

    if prev and now - prev <= SPAM_WINDOW_SECONDS:
        if now - last_spam_warn_ts >= SPAM_COOLDOWN_SECONDS:
            await update.message.reply_text(f"@{TARGET_NICK} {random.choice(SPAM_WARNINGS)}")
            last_spam_warn_ts = now

    last_messages[user.id] = now


async def send_day_reminder(app):
    chat_id = get_target_chat_id()
    if not chat_id:
        return
    await app.bot.send_message(chat_id, build_from(REMINDER_TEMPLATES, TARGET_NICK))


async def send_night_reminder(app):
    chat_id = get_target_chat_id()
    if not chat_id:
        return
    await app.bot.send_message(chat_id, build_from(NIGHT_TEMPLATES, TARGET_NICK))


def main():
    app = Application.builder().token(BOT_TOKEN).build()

    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("setchat", cmd_setchat))
    app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_text))

    scheduler = AsyncIOScheduler(timezone=TZ)

    scheduler.add_job(send_day_reminder, CronTrigger(hour="9-19/2", minute=0, timezone=TZ), kwargs={"app": app})
    scheduler.add_job(send_night_reminder, CronTrigger(hour=21, minute=0, timezone=TZ), kwargs={"app": app})

    scheduler.start()
    app.run_polling()


if __name__ == "__main__":
    main()
