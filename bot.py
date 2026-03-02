
import os
import json
import random
import logging
import time
from pathlib import Path
from zoneinfo import ZoneInfo

from dotenv import load_dotenv
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger

from telegram import Update, InputFile
from telegram.constants import ChatMemberStatus
from telegram.ext import Application, CommandHandler, MessageHandler, ContextTypes, filters

BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data.json"

load_dotenv()

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("guard-bot")

BOT_TOKEN = os.getenv("BOT_TOKEN")
TARGET_NICK = os.getenv("TARGET_NICK", "SerjoGrass").lstrip("@").strip()

START_PHOTO_FILE_ID = os.getenv("START_PHOTO_FILE_ID", "").strip()

if not BOT_TOKEN:
    raise RuntimeError("Не задан BOT_TOKEN")

TZ = ZoneInfo("Europe/Moscow")

# ===== Напоминания =====
REMINDER_TEMPLATES = [
    "Ему так грустно — он один охраняет коморку. @{nick}, напиши охраннику 🥺",
    "Охранник поставил чайник и ждёт твоё сообщение. @{nick}, напиши охраннику ☕",
    "Коморка охраняется, а чат — скучает. @{nick}, напиши охраннику 💬",
    "Вахта продолжается. Подкрепи морально. @{nick}, напиши охраннику 🛡️",
]

# ===== Антиспам фразы =====
SPAM_WARNINGS = [
    "А-ну, не спамь, а то заберу в коморку с ночёвкой 😠",
    "Спокойнее, герой клавиатуры. Коморка не резиновая 😡",
    "Ещё одно сообщение подряд — и чай отменяется 😤",
    "Не гони волну, а то охранник включит строгий режим 😈",
    "Тише-тише, клавиши дымятся 😾",
    "Спам обнаружен. Коморка приближается 😠",
    "Полегче! Это чат, а не пулемёт 😡",
    "Успокойся, а то будешь дежурить сам 😤",
    "Коморка всё видит. Особенно спам 😈",
    "Два сообщения подряд? Смело. Но опасно 😾",
    "Не разгоняйся — тормоза в коморке платные 😠",
    "Спамить будешь дома, тут порядок 😡",
    "Ещё чуть-чуть и оформим прописку в коморке 😤",
    "Пальцы пощади. И охранника тоже 😈",
    "Спокойствие. Только спокойствие. И меньше сообщений 😾",
]

# Антиспам настройки
SPAM_WINDOW_SECONDS = 5          # сколько секунд считаем "подряд"
SPAM_COOLDOWN_SECONDS = 120      # не чаще чем раз в 2 минуты

last_messages = {}   # user_id -> timestamp последнего сообщения
last_spam_warn = 0   # когда бот последний раз ругался


def build_message(nick: str) -> str:
    return random.choice(REMINDER_TEMPLATES).format(nick=nick)


def load_data():
    if DATA_FILE.exists():
        return json.loads(DATA_FILE.read_text(encoding="utf-8"))
    return {}


def save_data(data):
    DATA_FILE.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def get_target_chat_id():
    data = load_data()
    return data.get("chat_id")


async def is_admin(update: Update, context: ContextTypes.DEFAULT_TYPE):
    chat = update.effective_chat
    user = update.effective_user
    member = await context.bot.get_chat_member(chat.id, user.id)
    return member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER)


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    caption = "Приветствую тебя в коморке 🛡️"
    if START_PHOTO_FILE_ID:
        await update.message.reply_photo(photo=START_PHOTO_FILE_ID, caption=caption)
    else:
        await update.message.reply_text(caption)


async def cmd_setchat(update: Update, context: ContextTypes.DEFAULT_TYPE):
    if not await is_admin(update, context):
        await update.message.reply_text("Только для админов.")
        return

    chat = update.effective_chat
    data = load_data()
    data["chat_id"] = chat.id
    save_data(data)

    await update.message.reply_text(
        f"Чат назначен. Теперь буду писать сюда.
Цель: @{TARGET_NICK}"
    )

    await update.message.reply_text(
        f"Эй, эй, Сергей, не скучай — скоротай вечерок, заходи на чаёк ☕
@{TARGET_NICK}"
    )


async def handle_messages(update: Update, context: ContextTypes.DEFAULT_TYPE):
    global last_spam_warn

    user = update.effective_user
    if not user or user.is_bot:
        return

    now = time.time()
    user_id = user.id

    if user_id in last_messages:
        diff = now - last_messages[user_id]

        if diff <= SPAM_WINDOW_SECONDS:
            # Проверяем cooldown
            if now - last_spam_warn >= SPAM_COOLDOWN_SECONDS:
                warning = random.choice(SPAM_WARNINGS)
                await update.message.reply_text(
                    f"@{TARGET_NICK} {warning}"
                )
                last_spam_warn = now

    last_messages[user_id] = now


async def cmd_ping(update: Update, context: ContextTypes.DEFAULT_TYPE):
    await update.message.reply_text(build_message(TARGET_NICK))


async def send_reminder(app: Application):
    chat_id = get_target_chat_id()
    if not chat_id:
        return
    await app.bot.send_message(chat_id=chat_id, text=build_message(TARGET_NICK))


def main():
    app = Application.builder().token(BOT_TOKEN).build()

    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("setchat", cmd_setchat))
    app.add_handler(CommandHandler("ping", cmd_ping))

    app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_messages))

    scheduler = AsyncIOScheduler(timezone=TZ)
    trigger = CronTrigger(hour="9-21/2", minute=0, timezone=TZ)
    scheduler.add_job(send_reminder, trigger=trigger, kwargs={"app": app})
    scheduler.start()

    app.run_polling()


if __name__ == "__main__":
    main()
