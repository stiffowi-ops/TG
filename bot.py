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
from telegram.ext import Application, CommandHandler, MessageHandler, ContextTypes, filters

BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data.json"

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
log = logging.getLogger("guard-bot")

BOT_TOKEN = os.getenv("BOT_TOKEN", "").strip()
TARGET_NICK = os.getenv("TARGET_NICK", "SerjoGrass").lstrip("@").strip()

START_PHOTO_FILE_ID = os.getenv("START_PHOTO_FILE_ID", "").strip()
START_PHOTO_URL = os.getenv("START_PHOTO_URL", "").strip()
START_PHOTO_PATH = os.getenv("START_PHOTO_PATH", "").strip()

if not BOT_TOKEN:
    raise RuntimeError("Не задан BOT_TOKEN")

TZ = ZoneInfo("Europe/Moscow")

REMINDER_TEMPLATES = [
    "Ему так грустно — он один охраняет коморку. @{nick}, напиши охраннику 🥺",
    "Охранник поставил чайник и ждёт твоё сообщение. @{nick}, напиши охраннику ☕",
    "Коморка под надежной охраной… но без твоего сообщения скучно. @{nick}, напиши охраннику 🙂",
    "Сигнал тревоги: охраннику не хватает твоего «привет». @{nick}, напиши охраннику 🚨",
    "Вахта продолжается. Подкрепи морально. @{nick}, напиши охраннику 🛡️",
]

NIGHT_TEMPLATES = [
    "Коморка закрывается — сон твой начинается. Спокойной ночи, @{nick} 🌙",
    "Охранник гасит свет и ставит чайник на паузу. Спокойной ночи, @{nick} 😴",
    "Заслон опущен, дверь на замке. Спокойной ночи, @{nick} 🔒🌙",
    "Коморка уходит в ночной режим. Спокойной ночи, @{nick} 🌌",
]

SPAM_WARNINGS = [
    "А-ну, не спамь, а то заберу в коморку с ночёвкой 😠",
    "Спокойнее, герой клавиатуры. Коморка не резиновая 😡",
    "Ещё одно сообщение подряд — и чай отменяется 😤",
    "Полегче! Это чат, а не пулемёт 😡",
    "Ещё чуть-чуть — и оформим прописку в коморке 😤",
]

SPAM_WINDOW_SECONDS = 5
SPAM_COOLDOWN_SECONDS = 120

last_messages: dict[int, float] = {}
last_spam_warn_ts: float = 0.0

awaiting_photoid: set[int] = set()


def load_data() -> dict:
    if DATA_FILE.exists():
        try:
            return json.loads(DATA_FILE.read_text(encoding="utf-8"))
        except Exception:
            log.exception("Failed reading data.json")
            return {}
    return {}


def save_data(data: dict) -> None:
    DATA_FILE.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def get_target_chat_id() -> int | None:
    data = load_data()
    chat_id = data.get("chat_id")
    if chat_id is None:
        return None
    try:
        return int(chat_id)
    except Exception:
        return None


def set_target_chat_id(chat_id: int) -> None:
    data = load_data()
    data["chat_id"] = int(chat_id)
    save_data(data)


def build_from(templates: list[str], nick: str) -> str:
    nick = nick.lstrip("@").strip()
    return random.choice(templates).format(nick=nick)


def is_private(update: Update) -> bool:
    chat = update.effective_chat
    return bool(chat and chat.type == "private")


def is_target_user(update: Update) -> bool:
    user = update.effective_user
    if not user or user.is_bot:
        return False
    username = (user.username or "").lstrip("@").strip().lower()
    return username == TARGET_NICK.lower()


async def fake_check_sequence(update: Update, final_text: str) -> None:
    msg = await update.effective_message.reply_text("Выполняется проверка...")
    await asyncio.sleep(0.7)
    await msg.edit_text("Пип... Пип...")
    await asyncio.sleep(0.7)
    await msg.edit_text("Вычисляем....")
    await asyncio.sleep(0.8)
    await msg.edit_text(final_text)


async def require_group_member(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    chat_id = get_target_chat_id()
    if chat_id is None:
        await fake_check_sequence(
            update,
            "Петушок вычислен, коморка ещё не привязана. Пусть админ выполнит /setchat в группе 🐓",
        )
        return False

    user = update.effective_user
    if not user:
        return False

    try:
        member = await context.bot.get_chat_member(chat_id=chat_id, user_id=user.id)
        if member.status in (ChatMemberStatus.LEFT, ChatMemberStatus.KICKED):
            await fake_check_sequence(update, "Петушок вычислен, ты вне чата, тебе тут нечего делать 🐓")
            return False
        return True
    except Exception:
        await fake_check_sequence(
            update,
            "Пип... Пип... Ошибка доступа. Добавь бота в коморку и сделай /setchat 🐓",
        )
        return False


async def is_admin(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    chat = update.effective_chat
    user = update.effective_user
    if not chat or not user:
        return False
    try:
        member = await context.bot.get_chat_member(chat_id=chat.id, user_id=user.id)
        return member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER)
    except Exception:
        return False


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if is_private(update):
        ok = await require_group_member(update, context)
        if not ok:
            return

    caption = "Приветствую тебя в коморке 🛡️"
    msg = update.effective_message

    try:
        if START_PHOTO_FILE_ID:
            await msg.reply_photo(photo=START_PHOTO_FILE_ID, caption=caption)
            return
        if START_PHOTO_URL:
            await msg.reply_photo(photo=START_PHOTO_URL, caption=caption)
            return
        if START_PHOTO_PATH:
            path = Path(START_PHOTO_PATH)
            if not path.is_absolute():
                path = BASE_DIR / path
            if path.exists():
                with path.open("rb") as f:
                    await msg.reply_photo(photo=InputFile(f, filename=path.name), caption=caption)
                    return
        await msg.reply_text(caption)
    except Exception:
        log.exception("Failed /start")
        await msg.reply_text(caption)


async def cmd_setchat(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not await is_admin(update, context):
        await update.effective_message.reply_text("Только для админов.")
        return

    chat = update.effective_chat
    set_target_chat_id(chat.id)

    await update.effective_message.reply_text(
        f"Чат назначен ✅\\nCHAT_ID: {chat.id}\\nЦель: @{TARGET_NICK}"
    )
    await update.effective_message.reply_text(
        f"Эй, эй, Сергей, не скучай — скоротай вечерок, заходи на чаёк ☕\\n@{TARGET_NICK}"
    )


async def cmd_photoid(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_private(update):
        await update.effective_message.reply_text("Эта команда работает только в личке со мной 🙂")
        return

    ok = await require_group_member(update, context)
    if not ok:
        return

    user = update.effective_user
    awaiting_photoid.add(user.id)
    await update.effective_message.reply_text("Ок! Отправь мне фото следующим сообщением — я верну его file_id.")


async def handle_photo(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_private(update):
        return
    msg = update.effective_message
    if not msg or not msg.photo:
        return
    user = update.effective_user
    if not user or user.id not in awaiting_photoid:
        return

    awaiting_photoid.discard(user.id)
    photo = msg.photo[-1]
    await msg.reply_text(f"FILE_ID:\\n{photo.file_id}")


async def handle_text(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    global last_spam_warn_ts

    chat_id = get_target_chat_id()
    chat = update.effective_chat
    msg = update.effective_message
    user = update.effective_user
    if not chat_id or not chat or not msg or not user or user.is_bot:
        return
    if chat.id != chat_id:
        return
    if not is_target_user(update):
        return

    now = time.time()
    prev = last_messages.get(user.id)

    if prev is not None and (now - prev) <= SPAM_WINDOW_SECONDS:
        if (now - last_spam_warn_ts) >= SPAM_COOLDOWN_SECONDS:
            await msg.reply_text(f"@{TARGET_NICK} {random.choice(SPAM_WARNINGS)}")
            last_spam_warn_ts = now

    last_messages[user.id] = now


async def send_day_reminder(app: Application) -> None:
    chat_id = get_target_chat_id()
    if not chat_id:
        return
    text = build_from(REMINDER_TEMPLATES, TARGET_NICK)
    await app.bot.send_message(chat_id=chat_id, text=text)


async def send_night_reminder(app: Application) -> None:
    chat_id = get_target_chat_id()
    if not chat_id:
        return
    text = build_from(NIGHT_TEMPLATES, TARGET_NICK)
    await app.bot.send_message(chat_id=chat_id, text=text)


def main() -> None:
    app = Application.builder().token(BOT_TOKEN).build()

    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("setchat", cmd_setchat))
    app.add_handler(CommandHandler("photoid", cmd_photoid))

    app.add_handler(MessageHandler(filters.PHOTO, handle_photo))
    app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_text))

    scheduler = AsyncIOScheduler(timezone=TZ)

    scheduler.add_job(
        send_day_reminder,
        trigger=CronTrigger(hour="9-19/2", minute=0, timezone=TZ),
        kwargs={"app": app},
        id="day",
        replace_existing=True,
    )

    scheduler.add_job(
        send_night_reminder,
        trigger=CronTrigger(hour=21, minute=0, timezone=TZ),
        kwargs={"app": app},
        id="night",
        replace_existing=True,
    )

    scheduler.start()
    log.info("Scheduler started (MSK): day 09-19/2, night 21:00")

    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
