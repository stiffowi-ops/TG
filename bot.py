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

from telegram import Update, InputFile, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.constants import ChatMemberStatus, ParseMode
from telegram.ext import (
    Application,
    CommandHandler,
    MessageHandler,
    ContextTypes,
    filters,
    CallbackQueryHandler,
    ConversationHandler,
)

BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data.json"

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
log = logging.getLogger("guard-bot")

BOT_TOKEN = os.getenv("BOT_TOKEN", "").strip()
if not BOT_TOKEN:
    raise RuntimeError("Не задан BOT_TOKEN")

SERJO_NICK = os.getenv("SERJO_NICK", os.getenv("TARGET_NICK", "SerjoGrass")).lstrip("@").strip()
CHERNOV_NICK = os.getenv("CHERNOV_NICK", "chernovhush").lstrip("@").strip()
BUTTON_ADMIN = os.getenv("BUTTON_ADMIN", "Stiff_OWi").lstrip("@").strip()

START_PHOTO_FILE_ID = os.getenv("START_PHOTO_FILE_ID", "").strip()
START_PHOTO_URL = os.getenv("START_PHOTO_URL", "").strip()
START_PHOTO_PATH = os.getenv("START_PHOTO_PATH", "").strip()

TZ = ZoneInfo("Europe/Moscow")

SERJO_REMINDERS = [
    "Ему так грустно — он один охраняет коморку. @{nick}, напиши охраннику 🥺",
    "Охранник поставил чайник и ждёт твоё сообщение. @{nick}, напиши охраннику ☕",
    "Коморка под надежной охраной… но без твоего сообщения скучно. @{nick}, напиши охраннику 🙂",
    "Сигнал тревоги: охраннику не хватает твоего «привет». @{nick}, напиши охраннику 🚨",
    "Вахта продолжается. Подкрепи морально. @{nick}, напиши охраннику 🛡️",
    "Охранник на посту, а ты где? @{nick}, напиши охраннику 😄",
    "Коморка ждёт твоего знака. @{nick}, напиши охраннику ✍️",
    "Страж коморки грустит в тишине… @{nick}, напиши охраннику 🌫️",
    "Если бы сообщения грели — охранник бы не мёрз. @{nick}, напиши охраннику 🧣",
    "Охранник сверяет журнал: «сообщения нет». @{nick}, напиши охраннику 📒",
    "Коморка скучает по твоим буквам. @{nick}, напиши охраннику 📨",
    "Охранник уже третий раз смотрит на дверь… @{nick}, напиши охраннику 🚪",
    "Пора разморозить чат одним сообщением. @{nick}, напиши охраннику ❄️➡️🔥",
    "Страж просит: просто одно короткое «йо». @{nick}, напиши охраннику 😅",
    "Коморка в режиме ожидания. @{nick}, напиши охраннику ⏳",
    "У охранника всё стабильно… кроме настроения. @{nick}, напиши охраннику 😌",
    "Поддержи коморочный дух! @{nick}, напиши охраннику 🫶",
    "Охранник говорит шёпотом: «ну напиши…». @{nick}, напиши охраннику 🤫",
    "Дежурство идёт, часы тикают… @{nick}, напиши охраннику ⏰",
    "Коморка не любит одиночество. @{nick}, напиши охраннику 🧱",
    "Охранник придумал новый пароль: «пиши мне». @{nick}, напиши охраннику 🔐",
    "Тревожная сводка: мораль охранника падает. @{nick}, напиши охраннику 📉",
    "Охранник обещал не ворчать (почти). @{nick}, напиши охраннику 🙂",
    "Коморка держится на твоём «привет». @{nick}, напиши охраннику 👋",
    "На посту тихо… слишком тихо. @{nick}, напиши охраннику 👀",
]

SERJO_NIGHT = [
    "Коморка закрывается — сон твой начинается. Спокойной ночи, @{nick} 🌙",
    "Охранник гасит свет и ставит чайник на паузу. Спокойной ночи, @{nick} 😴",
    "Смена окончена: коморка засыпает, и ты тоже. Спокойной ночи, @{nick} 🛌",
    "Заслон опущен, дверь на замке. Спокойной ночи, @{nick} 🔒🌙",
    "Коморка шепчет: «пора отдыхать». Спокойной ночи, @{nick} ✨",
    "Охранник кивает: «до завтра». Спокойной ночи, @{nick} 🌛",
    "Чай допит, фонарь погас. Спокойной ночи, @{nick} ☕💤",
    "Коморка уходит в ночной режим. Спокойной ночи, @{nick} 🌌",
    "Пусть снится коморка без спама и с уютом. Спокойной ночи, @{nick} 💤",
    "Тишина в коморке — лучший плед. Спокойной ночи, @{nick} 🧣😴",
]

CHERNOV_REMINDERS = [
    "Отличный день для пробежки — бросай хлеб с маслом и погнали, @{nick} 🏃‍♂️🥪",
    "Пора шевелиться! Хлеб с маслом подождёт — @{nick}, на пробежку! 🏃‍♂️",
    "Коморка рекомендует кардио: @{nick}, кроссы на ноги и вперёд 💨",
    "Если не сейчас, то когда? @{nick}, пробежка сама себя не пробежит 😄",
    "Лёгкий старт: 10 минут туда, 10 обратно. @{nick}, погнали! 🏃‍♂️",
    "Хлеб с маслом — после! @{nick}, сначала километрик-другой 🥪➡️🏃‍♂️",
    "Время проветрить голову: @{nick}, на улицу и бегом 🌬️",
    "Коморка ставит челлендж: @{nick}, пробежка до ближайшей мысли и обратно 🏃‍♂️💡",
    "Разогревай мотор! @{nick}, вперёд за эндорфинами 😈🏃‍♂️",
    "Твой кроссовок грустит без тебя. @{nick}, пробежка! 👟🥺",
    "Секундомер уже включён в воображении. @{nick}, выходи бегать ⏱️🏃‍♂️",
    "Пока хлеб с маслом не убежал — убеги ты. @{nick}, погнали! 🥪🏃‍♂️",
    "Коморка объявляет: час здоровья. @{nick}, на пробежку 🏃‍♂️🫀",
    "Сделай вид, что ты спортсмен. @{nick}, пробежка по протоколу 😎🏃‍♂️",
    "Погода не важна — настроение важнее. @{nick}, пробежка! 🌦️🏃‍♂️",
]

SERJO_SPAM_WARNINGS = [
    "А-ну, не спамь, а то заберу в коморку с ночёвкой 😠",
    "Спокойнее, герой клавиатуры. Коморка не резиновая 😡",
    "Ещё одно сообщение подряд — и чай отменяется 😤",
    "Не гони волну, а то охранник включит строгий режим 😈",
    "Тише-тише, клавиши дымятся 😾",
    "Полегче! Это чат, а не пулемёт 😡",
    "Ещё чуть-чуть — и оформим прописку в коморке 😤",
]

CHERNOV_SPAM_WARNINGS = [
    "А, ну руки убери от клавиатуры, а то накажу, как твоего дружка 😠",
    "Хватит долбить по кнопкам, @{nick}. Спрячь клавиатуру, пока цела 😡",
    "Стоп-спам, @{nick}. Ещё раз — и получишь по коморочному протоколу 😈",
]

SPAM_WINDOW_SECONDS = 5
SPAM_COOLDOWN_SECONDS = 120

last_messages: dict[int, float] = {}
last_spam_warn_ts: dict[int, float] = {}

awaiting_photoid: set[int] = set()

BC_MENU, BC_TITLE, BC_TEXT, BC_FILE, BC_CONFIRM = range(5)


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


def get_username(update: Update) -> str:
    u = update.effective_user
    if not u or not u.username:
        return ""
    return u.username.lstrip("@").strip()


def is_button_admin(update: Update) -> bool:
    return get_username(update).lower() == BUTTON_ADMIN.lower()


def escape_html(s: str) -> str:
    return (s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;"))


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
        await fake_check_sequence(update, "Петушок вычислен, коморка ещё не привязана. Пусть админ выполнит /setchat в группе 🐓")
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
        await fake_check_sequence(update, "Пип... Пип... Ошибка доступа. Добавь бота в коморку и сделай /setchat 🐓")
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
        if not await require_group_member(update, context):
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

    await update.effective_message.reply_text(f"Чат назначен ✅\nCHAT_ID: {chat.id}\nЦели: @{SERJO_NICK}, @{CHERNOV_NICK}")
    await update.effective_message.reply_text(f"Эй, эй, Сергей, не скучай — скоротай вечерок, заходи на чаёк ☕\n@{SERJO_NICK}")


async def cmd_photoid(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_private(update):
        await update.effective_message.reply_text("Эта команда работает только в личке со мной 🙂")
        return

    if not await require_group_member(update, context):
        return

    awaiting_photoid.add(update.effective_user.id)
    await update.effective_message.reply_text("Ок! Отправь мне фото следующим сообщением — я верну его file_id.")


# /Button conversation (private + only BUTTON_ADMIN, no group membership required)
async def cmd_button(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    if not is_private(update):
        await update.effective_message.reply_text("Эта команда работает только в личке 🙂")
        return ConversationHandler.END
    if not is_button_admin(update):
        await update.effective_message.reply_text("Тебе сюда нельзя 🙂")
        return ConversationHandler.END

    keyboard = [[InlineKeyboardButton("📢 Рассылка", callback_data="bc_broadcast")]]
    await update.effective_message.reply_text("Панель управления:", reply_markup=InlineKeyboardMarkup(keyboard))
    return BC_MENU


async def bc_menu_click(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    q = update.callback_query
    await q.answer()
    if q.data != "bc_broadcast":
        return BC_MENU
    context.user_data["bc"] = {"title": "", "text": "", "file": None}
    await q.edit_message_text("Введи заголовок рассылки (он будет жирным):")
    return BC_TITLE


async def bc_title(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    title = (update.effective_message.text or "").strip()
    if not title:
        await update.effective_message.reply_text("Заголовок пустой. Введи заголовок ещё раз:")
        return BC_TITLE
    context.user_data["bc"]["title"] = title
    await update.effective_message.reply_text("Теперь введи текст рассылки:")
    return BC_TEXT


async def bc_text(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    text = (update.effective_message.text or "").strip()
    if not text:
        await update.effective_message.reply_text("Текст пустой. Введи текст ещё раз:")
        return BC_TEXT
    context.user_data["bc"]["text"] = text

    keyboard = [
        [InlineKeyboardButton("📎 Прикрепить файл", callback_data="bc_attach")],
        [InlineKeyboardButton("✅ Отправить", callback_data="bc_send"), InlineKeyboardButton("❌ Отмена", callback_data="bc_cancel")],
    ]
    bc = context.user_data["bc"]
    preview = f"<b>{escape_html(bc['title'])}</b>\n{escape_html(bc['text'])}"
    await update.effective_message.reply_text("Черновик готов.\n\nПредпросмотр:\n" + preview, parse_mode=ParseMode.HTML, reply_markup=InlineKeyboardMarkup(keyboard))
    return BC_CONFIRM


async def bc_confirm_click(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    q = update.callback_query
    await q.answer()

    if q.data == "bc_cancel":
        context.user_data.pop("bc", None)
        await q.edit_message_text("Ок, отменил.")
        return ConversationHandler.END

    if q.data == "bc_attach":
        await q.edit_message_text("Ок! Отправь файл (документ/фото/видео). Или напиши /skip чтобы без файла.")
        return BC_FILE

    if q.data == "bc_send":
        chat_id = get_target_chat_id()
        if not chat_id:
            await q.edit_message_text("Чат не назначен. Сначала /setchat в группе.")
            return ConversationHandler.END

        bc = context.user_data.get("bc", {})
        payload = f"<b>{escape_html(bc.get('title',''))}</b>\n{escape_html(bc.get('text',''))}"

        try:
            file_info = bc.get("file")
            if not file_info:
                await context.bot.send_message(chat_id=chat_id, text=payload, parse_mode=ParseMode.HTML)
            else:
                kind = file_info.get("kind")
                file_id = file_info.get("file_id")
                if kind == "photo":
                    await context.bot.send_photo(chat_id=chat_id, photo=file_id, caption=payload, parse_mode=ParseMode.HTML)
                elif kind == "video":
                    await context.bot.send_video(chat_id=chat_id, video=file_id, caption=payload, parse_mode=ParseMode.HTML)
                else:
                    await context.bot.send_document(chat_id=chat_id, document=file_id, caption=payload, parse_mode=ParseMode.HTML)
            await q.edit_message_text("✅ Отправлено в группу.")
        except Exception:
            log.exception("Broadcast send failed")
            await q.edit_message_text("❌ Не смог отправить. Проверь права бота в группе.")
        finally:
            context.user_data.pop("bc", None)

        return ConversationHandler.END

    return BC_CONFIRM


async def bc_file(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    msg = update.effective_message
    bc = context.user_data.get("bc", {})

    if msg.document:
        bc["file"] = {"kind": "document", "file_id": msg.document.file_id}
    elif msg.photo:
        bc["file"] = {"kind": "photo", "file_id": msg.photo[-1].file_id}
    elif msg.video:
        bc["file"] = {"kind": "video", "file_id": msg.video.file_id}
    else:
        await msg.reply_text("Я жду файл (документ/фото/видео). Или напиши /skip чтобы без файла.")
        return BC_FILE

    context.user_data["bc"] = bc

    keyboard = [
        [InlineKeyboardButton("✅ Отправить", callback_data="bc_send"), InlineKeyboardButton("❌ Отмена", callback_data="bc_cancel")],
    ]
    preview = f"<b>{escape_html(bc.get('title',''))}</b>\n{escape_html(bc.get('text',''))}"
    await msg.reply_text("Файл прикреплён ✅\n\nПредпросмотр:\n" + preview, parse_mode=ParseMode.HTML, reply_markup=InlineKeyboardMarkup(keyboard))
    return BC_CONFIRM


async def bc_skip(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    bc = context.user_data.get("bc", {})
    bc["file"] = None
    context.user_data["bc"] = bc

    keyboard = [
        [InlineKeyboardButton("✅ Отправить", callback_data="bc_send"), InlineKeyboardButton("❌ Отмена", callback_data="bc_cancel")],
    ]
    preview = f"<b>{escape_html(bc.get('title',''))}</b>\n{escape_html(bc.get('text',''))}"
    await update.effective_message.reply_text("Ок, без файла.\n\nПредпросмотр:\n" + preview, parse_mode=ParseMode.HTML, reply_markup=InlineKeyboardMarkup(keyboard))
    return BC_CONFIRM


async def bc_cancel_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    context.user_data.pop("bc", None)
    await update.effective_message.reply_text("Ок, отменил.")
    return ConversationHandler.END


async def handle_photoid_photo(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not is_private(update):
        return
    msg = update.effective_message
    if not msg or not msg.photo:
        return
    user = update.effective_user
    if not user or user.id not in awaiting_photoid:
        return
    awaiting_photoid.discard(user.id)
    await msg.reply_text(f"FILE_ID:\n{msg.photo[-1].file_id}")


async def handle_antispam(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    chat_id = get_target_chat_id()
    chat = update.effective_chat
    msg = update.effective_message
    user = update.effective_user
    if not chat_id or not chat or not msg or not user or user.is_bot:
        return
    if chat.id != chat_id:
        return

    u = get_username(update).lower()
    if u not in (SERJO_NICK.lower(), CHERNOV_NICK.lower()):
        return

    now = time.time()
    prev = last_messages.get(user.id)

    if prev is not None and (now - prev) <= SPAM_WINDOW_SECONDS:
        last_warn = last_spam_warn_ts.get(user.id, 0.0)
        if (now - last_warn) >= SPAM_COOLDOWN_SECONDS:
            if u == SERJO_NICK.lower():
                await msg.reply_text(f"@{SERJO_NICK} {random.choice(SERJO_SPAM_WARNINGS)}")
            else:
                warn = random.choice(CHERNOV_SPAM_WARNINGS).format(nick=CHERNOV_NICK)
                await msg.reply_text(f"@{CHERNOV_NICK} {warn}")
            last_spam_warn_ts[user.id] = now

    last_messages[user.id] = now


async def send_serjo_day(app: Application) -> None:
    chat_id = get_target_chat_id()
    if chat_id:
        await app.bot.send_message(chat_id=chat_id, text=build_from(SERJO_REMINDERS, SERJO_NICK))


async def send_serjo_night(app: Application) -> None:
    chat_id = get_target_chat_id()
    if chat_id:
        await app.bot.send_message(chat_id=chat_id, text=build_from(SERJO_NIGHT, SERJO_NICK))


async def send_chernov_hourly(app: Application) -> None:
    chat_id = get_target_chat_id()
    if chat_id:
        await app.bot.send_message(chat_id=chat_id, text=build_from(CHERNOV_REMINDERS, CHERNOV_NICK))


def main() -> None:
    app = Application.builder().token(BOT_TOKEN).build()

    # Conversation must be able to receive private messages -> group=0
    conv = ConversationHandler(
        entry_points=[CommandHandler("Button", cmd_button)],
        states={
            BC_MENU: [CallbackQueryHandler(bc_menu_click)],
            BC_TITLE: [MessageHandler(filters.TEXT & ~filters.COMMAND, bc_title)],
            BC_TEXT: [MessageHandler(filters.TEXT & ~filters.COMMAND, bc_text)],
            BC_FILE: [
                MessageHandler(filters.Document.ALL | filters.PHOTO | filters.VIDEO, bc_file),
                CommandHandler("skip", bc_skip),
            ],
            BC_CONFIRM: [CallbackQueryHandler(bc_confirm_click)],
        },
        fallbacks=[CommandHandler("cancel", bc_cancel_command)],
        per_user=True,
        per_chat=True,
        allow_reentry=True,
    )
    app.add_handler(conv, group=0)

    # Base commands
    app.add_handler(CommandHandler("start", cmd_start), group=0)
    app.add_handler(CommandHandler("setchat", cmd_setchat), group=0)
    app.add_handler(CommandHandler("photoid", cmd_photoid), group=0)

    # IMPORTANT: these handlers are in group=1 and have narrow filters,
    # so they do NOT steal Conversation messages in private chat.
    app.add_handler(MessageHandler(filters.ChatType.PRIVATE & filters.PHOTO, handle_photoid_photo), group=1)
    app.add_handler(MessageHandler(filters.ChatType.GROUPS & (filters.TEXT & ~filters.COMMAND), handle_antispam), group=1)

    # Scheduler
    scheduler = AsyncIOScheduler(timezone=TZ)
    scheduler.add_job(send_serjo_day, trigger=CronTrigger(hour="9-19/2", minute=0, timezone=TZ), kwargs={"app": app}, id="serjo_day", replace_existing=True)
    scheduler.add_job(send_serjo_night, trigger=CronTrigger(hour=21, minute=0, timezone=TZ), kwargs={"app": app}, id="serjo_night", replace_existing=True)
    scheduler.add_job(send_chernov_hourly, trigger=CronTrigger(hour="9-21", minute=0, timezone=TZ), kwargs={"app": app}, id="chernov_hourly", replace_existing=True)
    scheduler.start()

    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
