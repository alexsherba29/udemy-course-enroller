# Udemy Course Notifier Bot

A legitimate Telegram bot that monitors public sources for free Udemy courses and posts them to a Telegram channel. This application respects platform terms of service and only shares publicly available course information.

## Features

- 🔍 **Course Monitoring**: Periodically scans public course listing websites
- 📱 **Telegram Integration**: Posts course notifications with interactive buttons
- 🎯 **Smart Filtering**: User-configurable filters for categories, keywords, and ratings
- ⭐ **Wishlist System**: Save interesting courses for later review
- ❌ **Interest Management**: Mark courses as "not interested" to improve recommendations
- 🚫 **Duplicate Prevention**: Automatically prevents reposting the same courses
- 📊 **User Statistics**: Track wishlist, ignored courses, and preferences

## Setup

### Prerequisites

- Go 1.21 or later
- Telegram Bot Token (from @BotFather)
- Telegram Channel or Group ID

### Installation

1. Clone the repository
2. Copy `.env.example` to `.env` and configure:
   ```bash
   cp .env.example .env
   ```

3. Set your Telegram credentials in `.env`:
   ```
   TELEGRAM_BOT_TOKEN=your_bot_token_here
   TELEGRAM_CHANNEL_ID=@your_channel_or_chat_id
   ```

4. Install dependencies:
   ```bash
   go mod tidy
   ```

5. Run the bot:
   ```bash
   go run main.go
   ```

## Configuration

Edit `config.yaml` to customize:

- **Scraping interval**: How often to check for new courses
- **Source URLs**: Websites to monitor for free courses
- **Rate limiting**: Delay between requests
- **Default filters**: Categories and rating thresholds

## Usage

### Bot Commands

- `/start` - Welcome message and setup
- `/filter` - Configure course preferences
- `/wishlist` - View saved courses
- `/stats` - View activity statistics
- `/help` - Show help message

### Interactive Features

- **⭐ Save Button**: Add courses to your personal wishlist
- **❌ Not Interested**: Hide courses and improve future recommendations
- **🔗 View Course**: Direct link to the Udemy course page

### Filter Format

Configure preferences using this format:
```
Categories | MinRating | Keywords | ExcludedKeywords
```

Example:
```
Development, Business | 4.0 | programming, web | crypto, trading
```

## Project Structure

```
├── main.go              # Application entry point
├── config/              # Configuration management
├── database/            # SQLite database operations
├── scraper/             # Web scraping functionality
├── telegram/            # Telegram bot implementation
├── filters/             # Course filtering system
├── config.yaml          # Main configuration file
└── courses.db           # SQLite database (created automatically)
```

## Legal and Ethical Use

This application is designed for legitimate educational purposes:

- ✅ Monitors publicly available course information
- ✅ Respects website robots.txt and rate limiting
- ✅ Does not automate course enrollment
- ✅ Encourages manual course selection by users
- ✅ Complies with platform terms of service

## License

This project is for educational and personal use only.
