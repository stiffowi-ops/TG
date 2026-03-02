import os
import json
import random
import logging
from pathlib import Path
from zoneinfo import ZoneInfo

from dotenv import load_dotenv
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger

from telegram import Update
from telegram.constants import ChatMemberStatus
from telegram.ext import Application, CommandHandler, ContextTypes

# --- Config / storage ---
BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data.json"  # хранится на сервере, в репозиторий не коммитим

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
log = logging.getLogger("guard-bot")

BOT_TOKEN = os.getenv("BOT_TOKEN")
TARGET_NICK = os.getenv("TARGET_NICK", "SerjoGrass").lstrip("@").strip()

if not BOT_TOKEN:
    raise RuntimeError("Не задан BOT_TOKEN (в переменных окружения или .env)")

TZ = ZoneInfo("Europe/Moscow")

TEMPLATES = [
    "Ему так грустно — он один охраняет коморку. @{nick}, напиши охраннику 🥺",
    "Охранник смотрит в даль и вздыхает. @{nick}, напиши охраннику 😄",
    "Коморка под надежной охраной… но без твоего сообщения скучно. @{nick}, напиши охраннику 🙂",
    "Сигнал тревоги: охраннику не хватает твоего «привет». @{nick}, напиши охраннику 🚨",
    "Охранник уже придумал тебе прозвище «Пропавший в чате». @{nick}, напиши охраннику 😅",
    "Дежурство идёт, часы тикают… @{nick}, напиши охраннику ⏳",
    "Вахта держится на одном человеке и одном сообщении от тебя. @{nick}, напиши охраннику 😌",
    "Охранник включил режим ожидания. @{nick}, напиши охраннику 🤖",
    "Коморка под присмотром, но настроение — нет. @{nick}, напиши охраннику 🫶",
    "Охранник уже третий раз перечитал инструкцию. Спаси его. @{nick}, напиши охраннику 📜",
    "Срочно: у охранника кончились мемы. @{nick}, напиши охраннику 😭",
    "Если бы сообщения были витаминами — охранник был бы в дефиците. @{nick}, напиши охраннику 💊",
    "Тишина в коморке настолько громкая… @{nick}, напиши охраннику 🔊",
    "Охранник грустит по расписанию (как и мы). @{nick}, напиши охраннику 😄",
    "Внимание! Обнаружен дефицит внимания к охраннику. @{nick}, напиши охраннику 🕵️",
    "Коморка охраняется, а чат — скучает. @{nick}, напиши охраннику 💬",
    "Охранник поставил чайник и ждёт твоё сообщение. @{nick}, напиши охраннику ☕",
    "Охранник только что посмотрел на телефон. Пусто. @{nick}, напиши охраннику 📱",
    "У охранника всё стабильно… кроме настроения. @{nick}, напиши охраннику 😌",
    "Пора сделать доброе дело: одно сообщение охраннику. @{nick}, напиши охраннику ❤️",
    "Охранник шепчет: «ну где же он…» @{nick}, напиши охраннику 🙈",
    "Сводка дня: коморка цела, охранник в ожидании. @{nick}, напиши охраннику 🗞️",
    "Охранник уже начал разговаривать с табуреткой. @{nick}, напиши охраннику 🪑",
    "Не дай охраннику почувствовать себя одиноко. @{nick}, напиши охраннику 🥲",
    "Время планового «пинга». @{nick}, напиши охраннику ✅",
    "Охранник держится молодцом, но сообщение от тебя сделает день. @{nick}, напиши охраннику 🌟",
    "Коморка: охраняется. Охранник: скучает. @{nick}, напиши охраннику 🧷",
    "Вахта продолжается. Подкрепи морально. @{nick}, напиши охраннику 🛡️",
    "Коморка не убежит, а охранник — может заскучать. @{nick}, напиши охраннику 😄",
]


def build_message(nick: str) -> str:
    nick = nick.lstrip("@").strip()
    return random.choice(TEMPLATES).format(nick=nick)


def load_data() -> dict:
    if DATA_FILE.exists():
        try:
            return json.loads(DATA_FILE.read_text(encoding="utf-8"))
        except Exception:
            log.exception("Failed to read data.json, using defaults")
    return {}


def save_data(data: dict) -> None:
    DATA_FILE.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def get_target_chat_id() -> int | None:
    data = load_data()
    chat_id = data.get("chat_id")
    return int(chat_id) if chat_id is not None else None


async def is_admin(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    '''Разрешаем /setchat только админам/создателю чата.'''
    chat = update.effective_chat
    user = update.effective_user
    if not chat or not user:
        return False
    try:
        member = await context.bot.get_chat_member(chat_id=chat.id, user_id=user.id)
        return member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER)
    except Exception:
        log.exception("Failed to check admin status")
        return False


async def cmd_setchat(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    '''
    /setchat — запомнить текущий чат как целевой для рассылки.
    Работает только для админов.
    '''
    if not await is_admin(update, context):
        await update.message.reply_text("Эта команда доступна только админам чата 🙂")
        return

    chat = update.effective_chat
    data = load_data()
    data["chat_id"] = chat.id
    save_data(data)

    await update.message.reply_text(
        f"Ок! Теперь я буду напоминать в этом чате (chat_id: {chat.id}).\n"
        f"Цель: @{TARGET_NICK}\n"
        f"Расписание: каждые 2 часа с 09:00 до 21:00 (МСК)."
    )


async def cmd_status(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    chat_id = get_target_chat_id()
    if chat_id is None:
        await update.message.reply_text(
            "Чат для напоминаний ещё не задан.\n"
            "Зайди в нужный групповой чат и выполни /setchat (админом)."
        )
    else:
        await update.message.reply_text(
            f"Текущий чат для напоминаний: {chat_id}\n"
            f"Цель: @{TARGET_NICK}\n"
            "Расписание: каждые 2 часа с 09:00 до 21:00 (МСК)."
        )


async def cmd_ping(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    '''Разовое сообщение в текущий чат, чтобы проверить формат.'''
    await update.message.reply_text(build_message(TARGET_NICK))


async def send_reminder(app: Application) -> None:
    chat_id = get_target_chat_id()
    if chat_id is None:
        log.info("Chat not set yet; skipping reminder")
        return

    msg = build_message(TARGET_NICK)
    try:
        await app.bot.send_message(chat_id=chat_id, text=msg)
        log.info("Sent reminder to %s: %s", chat_id, msg)
    except Exception:
        log.exception("Failed to send reminder")


def main() -> None:
    app = Application.builder().token(BOT_TOKEN).build()

    app.add_handler(CommandHandler("setchat", cmd_setchat))
    app.add_handler(CommandHandler("status", cmd_status))
    app.add_handler(CommandHandler("ping", cmd_ping))

    scheduler = AsyncIOScheduler(timezone=TZ)

    # Каждые 2 часа с 09:00 до 21:00 (включительно): 9,11,13,15,17,19,21
    trigger = CronTrigger(hour="9-21/2", minute=0, timezone=TZ)

    scheduler.add_job(
        send_reminder,
        trigger=trigger,
        kwargs={"app": app},
        id="guard_reminder",
        replace_existing=True,
        misfire_grace_time=60 * 10,
        coalesce=True,
        max_instances=1,
    )
    scheduler.start()
    log.info("Scheduler started: every 2 hours 09:00-21:00 MSK")

    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
