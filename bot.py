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
from telegram.ext import (
    Application,
    CommandHandler,
    MessageHandler,
    ContextTypes,
    filters,
)

# ----------------------------
# Files / config
# ----------------------------
BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data.json"  # НЕ коммитим (в .gitignore)

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
log = logging.getLogger("guard-bot")

BOT_TOKEN = os.getenv("BOT_TOKEN")
TARGET_NICK = os.getenv("TARGET_NICK", "SerjoGrass").lstrip("@").strip()

# Фото для /start (опционально — достаточно одного из вариантов)
START_PHOTO_FILE_ID = os.getenv("START_PHOTO_FILE_ID", "").strip()
START_PHOTO_URL = os.getenv("START_PHOTO_URL", "").strip()
START_PHOTO_PATH = os.getenv("START_PHOTO_PATH", "").strip()

if not BOT_TOKEN:
    raise RuntimeError("Не задан BOT_TOKEN")

TZ = ZoneInfo("Europe/Moscow")

# ----------------------------
# Reminder templates
# ----------------------------
REMINDER_TEMPLATES = [
    "Ему так грустно — он один охраняет коморку. @{nick}, напиши охраннику 🥺",
    "Охранник смотрит в даль и вздыхает. @{nick}, напиши охраннику 😄",
    "Коморка под надежной охраной… но без твоего сообщения скучно. @{nick}, напиши охраннику 🙂",
    "Сигнал тревоги: охраннику не хватает твоего «привет». @{nick}, напиши охраннику 🚨",
    "Охранник уже придумал тебе прозвище «Пропавший в чате». @{nick}, напиши охраннику 😅",
    "Дежурство идёт, часы тикают… @{nick}, напиши охраннику ⏳",
    "Вахта держится на одном человеке и одном сообщении от тебя. @{nick}, напиши охраннику 😌",
    "Охранник включил режим ожидания. @{nick}, напиши охраннику 🤖",
    "Коморка под присмотром, но настроение — нет. @{nick}, напиши охраннику 🫶",
    "Охранник поставил чайник и ждёт твоё сообщение. @{nick}, напиши охраннику ☕",
    "У охранника всё стабильно… кроме настроения. @{nick}, напиши охраннику 😌",
    "Пора сделать доброе дело: одно сообщение охраннику. @{nick}, напиши охраннику ❤️",
    "Сводка дня: коморка цела, охранник в ожидании. @{nick}, напиши охраннику 🗞️",
    "Вахта продолжается. Подкрепи морально. @{nick}, напиши охраннику 🛡️",
]

# ----------------------------
# Anti-spam warnings (15)
# ----------------------------
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
    "Ещё чуть-чуть — и оформим прописку в коморке 😤",
    "Пальцы пощади. И охранника тоже 😈",
    "Спокойствие. Только спокойствие. И меньше сообщений 😾",
]

# Anti-spam settings:
SPAM_WINDOW_SECONDS = 5       # если 2+ сообщений за 5 секунд — считаем спамом
SPAM_COOLDOWN_SECONDS = 120   # бот ругается не чаще чем раз в 2 минуты

# Runtime state (in-memory)
last_messages: dict[int, float] = {}  # user_id -> last msg time
last_spam_warn_ts: float = 0.0        # last time bot warned


# ----------------------------
# Helpers: data storage
# ----------------------------
def load_data() -> dict:
    if DATA_FILE.exists():
        try:
            return json.loads(DATA_FILE.read_text(encoding="utf-8"))
        except Exception:
            log.exception("Failed to read data.json, using empty config")
    return {}


def save_data(data: dict) -> None:
    DATA_FILE.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def get_target_chat_id() -> int | None:
    data = load_data()
    chat_id = data.get("chat_id")
    return int(chat_id) if chat_id is not None else None


def build_reminder(nick: str) -> str:
    nick = nick.lstrip("@").strip()
    return random.choice(REMINDER_TEMPLATES).format(nick=nick)


# ----------------------------
# Permissions
# ----------------------------
async def is_admin(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    chat = update.effective_chat
    user = update.effective_user
    if not chat or not user:
        return False
    try:
        member = await context.bot.get_chat_member(chat.id, user.id)
        return member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER)
    except Exception:
        log.exception("Failed to check admin status")
        return False


# ----------------------------
# Commands
# ----------------------------
async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    # Пользователь просил: только приветствие + фото (если настроено)
    caption = "Приветствую тебя в коморке 🛡️"

    try:
        if START_PHOTO_FILE_ID:
            await update.message.reply_photo(photo=START_PHOTO_FILE_ID, caption=caption)
            return

        if START_PHOTO_URL:
            await update.message.reply_photo(photo=START_PHOTO_URL, caption=caption)
            return

        if START_PHOTO_PATH:
            path = Path(START_PHOTO_PATH)
            if not path.is_absolute():
                path = BASE_DIR / path
            if path.exists():
                with path.open("rb") as f:
                    await update.message.reply_photo(
                        photo=InputFile(f, filename=path.name),
                        caption=caption,
                    )
                return

        await update.message.reply_text(caption)
    except Exception:
        log.exception("Failed to send /start")
        await update.message.reply_text(caption)


async def cmd_setchat(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not await is_admin(update, context):
        await update.message.reply_text("Только для админов.")
        return

    chat = update.effective_chat
    data = load_data()
    data["chat_id"] = chat.id
    save_data(data)

    # ВАЖНО: никаких переносов внутри строк без \n (иначе SyntaxError)
    await update.message.reply_text(
        f"Чат назначен. Теперь буду писать сюда.\n"
        f"Цель: @{TARGET_NICK}"
    )

    await update.message.reply_text(
        f"Эй, эй, Сергей, не скучай — скоротай вечерок, заходи на чаёк ☕\n"
        f"@{TARGET_NICK}"
    )


async def cmd_ping(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    await update.message.reply_text(build_reminder(TARGET_NICK))


async def cmd_photoid(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    await update.message.reply_text("Отправь фото следующим сообщением — я верну file_id.")


# ----------------------------
# Photo handler (for /photoid)
# ----------------------------
async def handle_photo(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message or not update.message.photo:
        return
    photo = update.message.photo[-1]
    await update.message.reply_text(f"FILE_ID:\n{photo.file_id}")


# ----------------------------
# Anti-spam handler
# ----------------------------
async def handle_text(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """
    Если пользователь спамит (2+ сообщений подряд за SPAM_WINDOW_SECONDS),
    бот ругается, но не чаще чем раз в 2 минуты.
    Ругаемся только в том чате, который задан через /setchat.
    """
    global last_spam_warn_ts

    if not update.message:
        return

    chat = update.effective_chat
    user = update.effective_user
    if not chat or not user or user.is_bot:
        return

    target_chat_id = get_target_chat_id()
    if target_chat_id is None or chat.id != target_chat_id:
        return  # антиспам только в "целевом" чате

    now = time.time()
    user_id = user.id

    prev_ts = last_messages.get(user_id)
    if prev_ts is not None:
        diff = now - prev_ts
        if diff <= SPAM_WINDOW_SECONDS:
            if now - last_spam_warn_ts >= SPAM_COOLDOWN_SECONDS:
                warning = random.choice(SPAM_WARNINGS)
                await update.message.reply_text(f"@{TARGET_NICK} {warning}")
                last_spam_warn_ts = now

    last_messages[user_id] = now


# ----------------------------
# Scheduled reminders
# ----------------------------
async def send_reminder(app: Application) -> None:
    chat_id = get_target_chat_id()
    if chat_id is None:
        return
    try:
        await app.bot.send_message(chat_id=chat_id, text=build_reminder(TARGET_NICK))
    except Exception:
        log.exception("Failed to send scheduled reminder")


# ----------------------------
# Main
# ----------------------------
def main() -> None:
    app = Application.builder().token(BOT_TOKEN).build()

    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("setchat", cmd_setchat))
    app.add_handler(CommandHandler("ping", cmd_ping))
    app.add_handler(CommandHandler("photoid", cmd_photoid))

    # Любое фото -> вернём file_id (удобно для настройки START_PHOTO_FILE_ID)
    app.add_handler(MessageHandler(filters.PHOTO, handle_photo))

    # Антиспам на обычные текстовые сообщения (не команды)
    app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_text))

    # Schedule: every 2 hours from 09:00 to 21:00 MSK (inclusive): 9,11,13,15,17,19,21
    scheduler = AsyncIOScheduler(timezone=TZ)
    trigger = CronTrigger(hour="9-21/2", minute=0, timezone=TZ)
    scheduler.add_job(send_reminder, trigger=trigger, kwargs={"app": app}, replace_existing=True)
    scheduler.start()

    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
