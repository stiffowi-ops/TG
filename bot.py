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

# ----------------------------
# Files / config
# ----------------------------
BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data.json"  # do not commit

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
log = logging.getLogger("guard-bot")

BOT_TOKEN = os.getenv("BOT_TOKEN")
TARGET_NICK = os.getenv("TARGET_NICK", "SerjoGrass").lstrip("@").strip()

# Photo for /start (optional)
START_PHOTO_FILE_ID = os.getenv("START_PHOTO_FILE_ID", "").strip()
START_PHOTO_URL = os.getenv("START_PHOTO_URL", "").strip()
START_PHOTO_PATH = os.getenv("START_PHOTO_PATH", "").strip()

if not BOT_TOKEN:
    raise RuntimeError("BOT_TOKEN is not set")

TZ = ZoneInfo("Europe/Moscow")

# ----------------------------
# Templates
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

NIGHT_TEMPLATES = [
    "Коморка закрывается — сон твой начинается. Спокойной ночи, @{nick} 🌙",
    "Охранник гасит свет и ставит чайник на паузу. Спокойной ночи, @{nick} 😴",
    "Смена окончена: коморка засыпает, и ты тоже. Спокойной ночи, @{nick} 🛌",
    "Заслон опущен, дверь на замке. Спокойной ночи, @{nick} 🔒🌙",
    "Коморка шепчет: «пора отдыхать». Спокойной ночи, @{nick} ✨",
    "Охранник кивает: «до завтра». Спокойной ночи, @{nick} 🌛",
    "Тишина в коморке — лучший плед. Спокойной ночи, @{nick} 🧣😴",
    "Чай допит, фонарь погас. Спокойной ночи, @{nick} ☕💤",
    "Коморка уходит в ночной режим. Спокойной ночи, @{nick} 🌌",
    "Пусть снится коморка без спама и с уютом. Спокойной ночи, @{nick} 💤",
]

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
            log.exception("Failed to read data.json, using empty config")
    return {}


def save_data(data: dict) -> None:
    DATA_FILE.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def get_target_chat_id() -> int | None:
    data = load_data()
    chat_id = data.get("chat_id")
    return int(chat_id) if chat_id is not None else None


def build_from(templates: list[str], nick: str) -> str:
    nick = nick.lstrip("@").strip()
    return random.choice(templates).format(nick=nick)


def is_private(update: Update) -> bool:
    chat = update.effective_chat
    return bool(chat and chat.type == "private")


def user_is_target(update: Update) -> bool:
    user = update.effective_user
    if not user or user.is_bot:
        return False
    username = (user.username or "").lstrip("@")
    return username.lower() == TARGET_NICK.lower()


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


async def fake_check_sequence(update: Update, final_text: str) -> None:
    msg = await update.message.reply_text("Выполняется проверка...")
    await asyncio.sleep(0.7)
    await msg.edit_text("Пип... Пип...")
    await asyncio.sleep(0.7)
    await msg.edit_text("Вычисляем....")
    await asyncio.sleep(0.8)
    await msg.edit_text(final_text)


async def require_group_member(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    # Access to private commands is only for members of the group set via /setchat.
    # If chat is not set yet, we still show the "check" animation (your logic).
    chat_id = get_target_chat_id()
    if chat_id is None:
        await fake_check_sequence(
            update,
            "Петушок вычислен: коморка ещё не привязана. Пусть админ выполнит /setchat в группе 🐓",
        )
        return False

    user = update.effective_user
    if not user:
        return False

    try:
        member = await context.bot.get_chat_member(chat_id=chat_id, user_id=user.id)
        if member.status in (ChatMemberStatus.LEFT, ChatMemberStatus.KICKED):
            await fake_check_sequence(update, "Петушок вычислен: ты вне чата, тебе тут нечего делать 🐓")
            return False
        return True
    except Exception:
        await update.message.reply_text(
            "Не могу проверить участие в коморке 😕
"
            "Убедись, что бот добавлен в нужную группу и в ней выполнен /setchat."
        )
        return False


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    # In private: members only (with fake check). In group: just greeting.
    if is_private(update):
        if not await require_group_member(update, context):
            return

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
                    await update.message.reply_photo(photo=InputFile(f, filename=path.name), caption=caption)
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

    await update.message.reply_text(
        f"Чат назначен ✅
"
        f"CHAT_ID: {chat.id}
"
        f"Цель: @{TARGET_NICK}"
    )

    await update.message.reply_text(
        f"Эй, эй, Сергей, не скучай — скоротай вечерок, заходи на чаёк ☕
"
        f"@{TARGET_NICK}"
    )


async def cmd_ping(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    await update.message.reply_text(build_from(REMINDER_TEMPLATES, TARGET_NICK))


async def cmd_photoid(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_private(update):
        await update.message.reply_text("Эта команда работает только в личке со мной 🙂")
        return

    if not await require_group_member(update, context):
        return

    user = update.effective_user
    awaiting_photoid.add(user.id)
    await update.message.reply_text("Ок! Отправь мне фото следующим сообщением — я верну его file_id.")


async def handle_photo(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message or not update.message.photo:
        return
    if not is_private(update):
        return

    user = update.effective_user
    if not user or user.id not in awaiting_photoid:
        return

    awaiting_photoid.discard(user.id)
    photo = update.message.photo[-1]
    await update.message.reply_text(f"FILE_ID:
{photo.file_id}")


async def handle_text(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    # Anti-spam: only in the configured group and only if TARGET_NICK is spamming
    global last_spam_warn_ts

    if not update.message:
        return

    chat = update.effective_chat
    user = update.effective_user
    if not chat or not user or user.is_bot:
        return

    target_chat_id = get_target_chat_id()
    if target_chat_id is None or chat.id != target_chat_id:
        return

    if not user_is_target(update):
        return

    now = time.time()
    user_id = user.id

    prev_ts = last_messages.get(user_id)
    if prev_ts is not None:
        if now - prev_ts <= SPAM_WINDOW_SECONDS:
            if now - last_spam_warn_ts >= SPAM_COOLDOWN_SECONDS:
                await update.message.reply_text(f"@{TARGET_NICK} {random.choice(SPAM_WARNINGS)}")
                last_spam_warn_ts = now

    last_messages[user_id] = now


# ----------------------------
# Scheduled reminders (split jobs to guarantee 21:00 behavior)
# ----------------------------
async def send_day_reminder(app: Application) -> None:
    chat_id = get_target_chat_id()
    if chat_id is None:
        return
    text = build_from(REMINDER_TEMPLATES, TARGET_NICK)
    try:
        await app.bot.send_message(chat_id=chat_id, text=text)
    except Exception:
        log.exception("Failed to send day reminder")


async def send_night_reminder(app: Application) -> None:
    chat_id = get_target_chat_id()
    if chat_id is None:
        return
    text = build_from(NIGHT_TEMPLATES, TARGET_NICK)
    try:
        await app.bot.send_message(chat_id=chat_id, text=text)
    except Exception:
        log.exception("Failed to send night reminder")


def main() -> None:
    app = Application.builder().token(BOT_TOKEN).build()

    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("setchat", cmd_setchat))
    app.add_handler(CommandHandler("ping", cmd_ping))
    app.add_handler(CommandHandler("photoid", cmd_photoid))

    # file_id only in private and only after /photoid
    app.add_handler(MessageHandler(filters.PHOTO, handle_photo))

    # Anti-spam: only text (not commands) and only if TARGET_NICK
    app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_text))

    scheduler = AsyncIOScheduler(timezone=TZ)

    # Day: 09:00, 11:00, 13:00, 15:00, 17:00, 19:00
    day_trigger = CronTrigger(hour="9-19/2", minute=0, timezone=TZ)
    scheduler.add_job(send_day_reminder, trigger=day_trigger, kwargs={"app": app}, id="day", replace_existing=True)

    # Night: exactly 21:00
    night_trigger = CronTrigger(hour=21, minute=0, timezone=TZ)
    scheduler.add_job(send_night_reminder, trigger=night_trigger, kwargs={"app": app}, id="night", replace_existing=True)

    scheduler.start()
    log.info("Scheduler started: day 09-19/2 and night at 21:00 (MSK)")

    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
