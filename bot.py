
import os
import json
import random
import logging
from pathlib import Path
from zoneinfo import ZoneInfo

from dotenv import load_dotenv
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger

from telegram import Update, InputFile
from telegram.constants import ChatMemberStatus
from telegram.ext import Application, CommandHandler, MessageHandler, ContextTypes, filters

# --- Config / storage ---
BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data.json"

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
log = logging.getLogger("guard-bot")

BOT_TOKEN = os.getenv("BOT_TOKEN")
TARGET_NICK = os.getenv("TARGET_NICK", "SerjoGrass").lstrip("@").strip()

START_PHOTO_FILE_ID = os.getenv("START_PHOTO_FILE_ID", "").strip()
START_PHOTO_URL = os.getenv("START_PHOTO_URL", "").strip()
START_PHOTO_PATH = os.getenv("START_PHOTO_PATH", "").strip()

if not BOT_TOKEN:
    raise RuntimeError("Не задан BOT_TOKEN")

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
    "Охранник грустит по расписанию. @{nick}, напиши охраннику 😄",
    "Коморка охраняется, а чат — скучает. @{nick}, напиши охраннику 💬",
    "Охранник поставил чайник и ждёт твоё сообщение. @{nick}, напиши охраннику ☕",
    "У охранника всё стабильно… кроме настроения. @{nick}, напиши охраннику 😌",
    "Пора сделать доброе дело: одно сообщение охраннику. @{nick}, напиши охраннику ❤️",
    "Сводка дня: коморка цела, охранник в ожидании. @{nick}, напиши охраннику 🗞️",
    "Вахта продолжается. Подкрепи морально. @{nick}, напиши охраннику 🛡️",
]


def build_message(nick: str) -> str:
    return random.choice(TEMPLATES).format(nick=nick)


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


def start_caption():
    return (
        "Приветствую тебя в коморке 🛡️\n\n"
        f"Я каждые 2 часа с 09:00 до 21:00 (МСК) напоминаю @{TARGET_NICK} написать охраннику.\n\n"
        "Команды:\n"
        "/setchat — назначить чат\n"
        "/status — статус\n"
        "/ping — тест\n"
        "/photoid — получить file_id фото"
    )


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    caption = start_caption()

    if START_PHOTO_FILE_ID:
        await update.message.reply_photo(photo=START_PHOTO_FILE_ID, caption=caption)
    elif START_PHOTO_URL:
        await update.message.reply_photo(photo=START_PHOTO_URL, caption=caption)
    elif START_PHOTO_PATH:
        path = Path(START_PHOTO_PATH)
        if not path.is_absolute():
            path = BASE_DIR / path
        if path.exists():
            with path.open("rb") as f:
                await update.message.reply_photo(photo=InputFile(f), caption=caption)
                return
        await update.message.reply_text(caption)
    else:
        await update.message.reply_text(caption)


async def cmd_photoid(update: Update, context: ContextTypes.DEFAULT_TYPE):
    await update.message.reply_text("Отправь мне фото следующим сообщением — я верну его file_id.")


async def handle_photo(update: Update, context: ContextTypes.DEFAULT_TYPE):
    photo = update.message.photo[-1]
    await update.message.reply_text(f"FILE_ID:\n{photo.file_id}")


async def cmd_setchat(update: Update, context: ContextTypes.DEFAULT_TYPE):
    if not await is_admin(update, context):
        await update.message.reply_text("Только для админов.")
        return

    chat = update.effective_chat
    data = load_data()
    data["chat_id"] = chat.id
    save_data(data)

    await update.message.reply_text(
        f"Чат назначен. Теперь буду писать сюда.\nЦель: @{TARGET_NICK}"
    )

    # Приветствие после назначения
    await update.message.reply_text(
        f"Эй, эй, Сергей, не скучай — скоротай вечерок, заходи на чаёк ☕\n@{TARGET_NICK}"
    )


async def cmd_status(update: Update, context: ContextTypes.DEFAULT_TYPE):
    chat_id = get_target_chat_id()
    if chat_id:
        await update.message.reply_text(f"Чат: {chat_id}\nЦель: @{TARGET_NICK}")
    else:
        await update.message.reply_text("Чат ещё не назначен.")


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
    app.add_handler(CommandHandler("status", cmd_status))
    app.add_handler(CommandHandler("ping", cmd_ping))
    app.add_handler(CommandHandler("photoid", cmd_photoid))
    app.add_handler(MessageHandler(filters.PHOTO, handle_photo))

    scheduler = AsyncIOScheduler(timezone=TZ)
    trigger = CronTrigger(hour="9-21/2", minute=0, timezone=TZ)

    scheduler.add_job(send_reminder, trigger=trigger, kwargs={"app": app})
    scheduler.start()

    app.run_polling()


if __name__ == "__main__":
    main()
