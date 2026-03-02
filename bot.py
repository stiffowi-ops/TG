import os
import json
import random
import logging
import time
import asyncio
from pathlib import Path
from zoneinfo import ZoneInfo
from datetime import datetime

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
DATA_FILE = BASE_DIR / "data.json"  # НЕ коммитим

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

# В 21:00 по МСК
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

# Антиспам фразы (15)
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

# Антиспам настройки
SPAM_WINDOW_SECONDS = 5
SPAM_COOLDOWN_SECONDS = 120

# Runtime state
last_messages: dict[int, float] = {}
last_spam_warn_ts: float = 0.0

# Личка: ждём фото после /photoid или форвард после /chatid
awaiting_photoid: set[int] = set()
awaiting_chatid: set[int] = set()


# ----------------------------
# Storage helpers
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


async def fake_check_denied(update: Update) -> None:
    """
    Фейковая "проверка" с редактированием сообщения, потом отказ.
    """
    msg = await update.message.reply_text("Выполняется проверка...")
    await asyncio.sleep(0.7)
    await msg.edit_text("Пип... Пип...")
    await asyncio.sleep(0.7)
    await msg.edit_text("Вычисляем....")
    await asyncio.sleep(0.8)
    await msg.edit_text("Петушок вычислен: ты вне чата, тебе тут нечего делать 🐓")


async def require_group_member(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    """
    Разрешаем личные команды только участникам группы, заданной через /setchat.
    """
    # Вызываем только в личке
    chat_id = get_target_chat_id()
    if chat_id is None:
        # Если чат ещё не настроен — отказываем (и объясняем)
        await update.message.reply_text("Коморка ещё не настроена. Пусть админ выполнит /setchat в группе 🙂")
        return False

    user = update.effective_user
    if not user:
        return False

    try:
        member = await context.bot.get_chat_member(chat_id=chat_id, user_id=user.id)
        if member.status in (ChatMemberStatus.LEFT, ChatMemberStatus.KICKED):
            await fake_check_denied(update)
            return False
        return True
    except Exception:
        # Обычно: бот не в группе, нет доступа или Telegram не даёт проверить
        await update.message.reply_text(
            "Не могу проверить твоё участие в коморке 😕\n"
            "Убедись, что бот добавлен в нужную группу и в ней выполнен /setchat."
        )
        return False


# ----------------------------
# Commands
# ----------------------------
async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    caption = "Приветствую тебя в коморке 🛡️"

    # Ограничение: если пишут в личку — только участникам коморки
    if is_private(update):
        if not await require_group_member(update, context):
            return

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
        f"Чат назначен. Теперь буду писать сюда.\n"
        f"Цель: @{TARGET_NICK}"
    )
    await update.message.reply_text(
        f"Эй, эй, Сергей, не скучай — скоротай вечерок, заходи на чаёк ☕\n"
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


async def cmd_chatid(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_private(update):
        await update.message.reply_text("Эта команда работает только в личке со мной 🙂")
        return
    if not await require_group_member(update, context):
        return

    user = update.effective_user
    awaiting_chatid.add(user.id)
    await update.message.reply_text(
        "Ок! Перешли мне любое сообщение из нужной группы/чата — я попробую вернуть chat_id этого чата."
    )


# ----------------------------
# Private handlers: photoid / chatid
# ----------------------------
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
    await update.message.reply_text(f"FILE_ID:\n{photo.file_id}")


async def handle_forwarded_for_chatid(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    if not is_private(update):
        return

    user = update.effective_user
    if not user or user.id not in awaiting_chatid:
        return

    chat_id = None

    # Старое поле пересылки
    fchat = getattr(update.message, "forward_from_chat", None)
    if fchat and getattr(fchat, "id", None) is not None:
        chat_id = fchat.id

    # Новое поле пересылки
    if chat_id is None:
        origin = getattr(update.message, "forward_origin", None)
        if origin is not None:
            possible_chat = getattr(origin, "chat", None)
            if possible_chat is not None and getattr(possible_chat, "id", None) is not None:
                chat_id = possible_chat.id

    awaiting_chatid.discard(user.id)

    if chat_id is None:
        await update.message.reply_text(
            "Не смог понять chat_id из пересылки 😕\n"
            "Telegram иногда скрывает источник пересылки.\n"
            "Самый надёжный способ — добавить меня в группу и сделать /setchat."
        )
        return

    await update.message.reply_text(f"CHAT_ID:\n{chat_id}")


# ----------------------------
# Anti-spam (ONLY for @TARGET_NICK) in the configured group chat
# ----------------------------
async def handle_text(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
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

    # ругаем только если спамит TARGET_NICK
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
# Scheduled reminders
# ----------------------------
async def send_reminder(app: Application) -> None:
    chat_id = get_target_chat_id()
    if chat_id is None:
        return

    hour_msk = datetime.now(TZ).hour
    templates = NIGHT_TEMPLATES if hour_msk == 21 else REMINDER_TEMPLATES
    text = build_from(templates, TARGET_NICK)

    try:
        await app.bot.send_message(chat_id=chat_id, text=text)
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

    # Личные команды (доступ только участникам коморки)
    app.add_handler(CommandHandler("photoid", cmd_photoid))
    app.add_handler(CommandHandler("chatid", cmd_chatid))

    # Эти хэндлеры реально отвечают только в личке и только после команды
    app.add_handler(MessageHandler(filters.PHOTO, handle_photo))
    app.add_handler(MessageHandler(filters.FORWARDED, handle_forwarded_for_chatid))

    # Антиспам: только текст (не команды) и только если пишет TARGET_NICK
    app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_text))

    # Каждые 2 часа с 09:00 до 21:00 (включительно): 9,11,13,15,17,19,21 (МСК)
    scheduler = AsyncIOScheduler(timezone=TZ)
    trigger = CronTrigger(hour="9-21/2", minute=0, timezone=TZ)
    scheduler.add_job(send_reminder, trigger=trigger, kwargs={"app": app}, replace_existing=True)
    scheduler.start()

    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
