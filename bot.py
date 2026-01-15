import logging
from telegram import Update, ReplyKeyboardMarkup, KeyboardButton
from telegram.ext import ApplicationBuilder, CommandHandler, MessageHandler, ContextTypes, filters

# Enable logging
logging.basicConfig(
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    level=logging.INFO
)

# Start command
async def start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    keyboard = [
        [KeyboardButton("🎬 Drama A"), KeyboardButton("🎭 Drama B")],
        [KeyboardButton("🔥 Trending"), KeyboardButton("🔍 Search")]
    ]
    reply_markup = ReplyKeyboardMarkup(keyboard, resize_keyboard=True)
    await update.message.reply_text("Pilih drama yang ingin kamu tonton:", reply_markup=reply_markup)

# Handle button responses
async def handle_message(update: Update, context: ContextTypes.DEFAULT_TYPE):
    user_input = update.message.text
    if user_input == "🎬 Drama A":
        await update.message.reply_text("Kamu memilih Drama A.")
    elif user_input == "🎭 Drama B":
        await update.message.reply_text("Kamu memilih Drama B.")
    elif user_input == "🔥 Trending":
        await update.message.reply_text("Berikut drama yang sedang trending...")
    elif user_input == "🔍 Search":
        await update.message.reply_text("Ketik nama drama yang ingin kamu cari.")
    else:
        await update.message.reply_text("Silakan pilih dari tombol yang tersedia.")

# Main function
if __name__ == '__main__':
    import asyncio

    async def main():
        app = ApplicationBuilder().token("8178315689:AAGN8NW0eO966KwYHxXcosVLJLYtjd7Kvtw").build()

        app.add_handler(CommandHandler("start", start))
        app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_message))

        print("🤖 Bot is running...")
        await app.run_polling()

    asyncio.run(main())
vv