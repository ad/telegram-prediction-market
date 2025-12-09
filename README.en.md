<div align="center">

# 🎯 Telegram Prediction Market Bot

**Create your prediction market right in Telegram**

[![Go Version](https://img.shields.io/badge/Go-1.25.5+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Bot-blue?logo=telegram)](https://telegram.org/)

English | [Русский](README.md)

[Features](#-features) • [Quick Start](#-quick-start) • [Usage](#-usage) • [Architecture](#-architecture)

</div>

---

## 🌟 About

Telegram Prediction Market Bot is a full-featured bot for creating prediction markets where users can make forecasts on various events and compete in accuracy. Perfect for teams, communities, and friend groups who want to add a competitive element to their discussions.

### 💡 Why Use It?

- **For Teams**: Predict sprint outcomes, releases, metrics
- **For Communities**: Create prediction tournaments on any topic
- **For Friends**: Compete in forecasts on sports events, weather, politics
- **For Learning**: Develop critical thinking and probability assessment skills

---

## ✨ Features

### 🏢 Multi-Group Architecture
- **Complete data isolation** between groups
- **Deep-link invitations** for easy joining
- **Unlimited participation** — users can be in multiple groups simultaneously
- **Independent ratings** and achievements in each group
- **🆕 Telegram Forums support** — send events to specific forum topics

### 🎲 Flexible Event Types
- **Binary** (Yes/No) — classic predictions
- **Multiple Choice** (2-6 options) — for complex scenarios
- **Probabilistic** (ranges 0-25%, 25-50%, 50-75%, 75-100%) — for confidence calibration

### 🎯 Smart Scoring System
```
✅ Correct Prediction:
   • Binary event: +10 points
   • Multiple choice: +15 points
   • Probabilistic: +15 points

🎁 Bonuses:
   • Minority (<40% votes): +5 points
   • Early vote (first 12 hours): +3 points
   • Participation: +1 point

❌ Penalties:
   • Wrong prediction: -3 points
```

### 🏆 Achievement System
- 🎯 **Sharpshooter** — 3 correct predictions in a row
- 🔮 **Oracle** — 10 correct predictions in a row
- 🎲 **Risk Taker** — 3 correct minority predictions in a row
- 📊 **Analyst of the Week** — most points in a week
- 🏆 **Veteran** — participated in 50 events

### 🔄 FSM-based Event Creation
- **Interactive step-by-step process** with validation at each step
- **Automatic message cleanup** for clean chat
- **Persistent sessions** — continue after bot restart
- **Conflict protection** — multiple admins can create events simultaneously

### 🔔 Smart Notifications
- Reminders 24 hours before deadline
- New event announcements
- Achievement notifications

### 💬 Telegram Forums Support (NEW!)
- **Send events to topics** — create events in specific forum topics
- **Default topic** — configure group to automatically send to a specific topic
- **Flexibility** — each event can be sent to its own topic
- **Backward compatibility** — regular groups continue to work as before

---

## 🚀 Quick Start

### Requirements

- Go 1.25.5+
- SQLite (embedded via modernc.org/sqlite)
- Telegram Bot Token (get from [@BotFather](https://t.me/BotFather))

### Installation

```bash
# Clone the repository
git clone https://github.com/ad/telegram-prediction-market.git
cd telegram-prediction-market

# Install dependencies
go mod download

# Build the bot
go build -o bin/bot ./cmd/bot
```

### Configuration

Create a `.env` file based on `.env.example`:

```bash
# Required parameters
TELEGRAM_TOKEN="your-bot-token-here"
ADMIN_USER_IDS="123456789,987654321"

# Optional parameters
DATABASE_PATH="./data/bot.db"
LOG_LEVEL="INFO"
DEFAULT_GROUP_NAME="Default Group"
MAX_GROUPS_PER_ADMIN="10"
MAX_MEMBERSHIPS_PER_USER="20"
```

### Running

```bash
# Run the bot
./bin/bot

# Or directly via Go
go run ./cmd/bot
```

---

## 📖 Usage

### For Users

```
/start    — Start working with the bot
/help     — Show help
/groups   — List your groups
/rating   — Top 10 participants
/my       — Your statistics
/events   — Active events
```

### For Administrators

#### 1. Create a Group
```
/create_group
```
The bot will guide you through the creation process and provide an invitation link.

#### 2. Invite Participants
Share the deep-link:
```
https://t.me/your_bot?start=group_abc123
```

#### 3. Create an Event
```
/create_event
```
Interactive process:
1. Select group (if you have multiple)
2. Enter question
3. Choose event type
4. Specify options (for multiple choice)
5. Set deadline
6. Confirm

#### 4. Resolve Event
```
/resolve_event
```
Select the correct answer, and the bot will automatically calculate points and update ratings.

### Additional Admin Commands

```
/list_groups     — List all groups with links
/group_members   — Group members
/remove_member   — Remove member
/edit_event      — Edit event (only without votes)
```

---

## 🏗 Architecture

### Technology Stack

- **Language**: Go 1.25.5
- **Database**: SQLite with WAL mode
- **Telegram API**: [go-telegram/bot](https://github.com/go-telegram/bot)
- **FSM**: Custom implementation with persistence
- **ID Encoding**: Custom Base-N encoder for short deep-links

### Project Structure

```
.
├── cmd/bot/              # Application entry point
├── internal/
│   ├── bot/             # Telegram handlers and FSM
│   │   ├── handler.go              # Main handler
│   │   ├── event_creation_fsm.go   # Event creation FSM
│   │   ├── event_resolution_fsm.go # Event resolution FSM
│   │   ├── group_creation_fsm.go   # Group creation FSM
│   │   └── message_deletion.go     # Cleanup utilities
│   ├── config/          # Configuration management
│   ├── domain/          # Business logic
│   │   ├── event_manager.go           # Event management
│   │   ├── rating_calculator.go       # Rating calculation
│   │   ├── achievement_tracker.go     # Achievement tracking
│   │   ├── deeplink_service.go        # Deep-link generation
│   │   └── group_context_resolver.go  # Group context resolution
│   ├── encoding/        # Base-N encoding for IDs
│   ├── logger/          # Structured logging
│   └── storage/         # Repositories and migrations
│       ├── fsm_storage.go                  # FSM persistence
│       ├── group_repository.go             # Group operations
│       ├── group_membership_repository.go  # Membership management
│       └── migrations.go                   # DB migrations
└── data/                # SQLite database
```

### Key Implementation Features

#### 🔄 FSM with Persistence
All interactive processes (event creation, group creation, event resolution) are implemented through finite state machines with state persistence in DB. This allows:
- Continue process after bot restart
- Avoid conflicts between sessions
- Automatically clean up stale sessions (>30 minutes)

#### 🔐 Data Isolation
Each group is a completely isolated space:
- Events visible only to group members
- Ratings maintained separately
- Achievements earned independently
- Users can have different positions in different groups

#### 📊 Smart Rating Calculation
The system considers:
- Event complexity (type)
- Choice popularity (minority bonus)
- Reaction speed (early vote bonus)
- History of correct predictions (streaks)

#### 🔗 Short Deep-links
Uses custom Base-N encoding to create short and readable invitation links instead of long numeric IDs.

---

## 🧪 Testing

```bash
# Run all tests
go test ./...

# With verbose output
go test ./... -v

# Only specific package
go test ./internal/bot -v

# With coverage
go test ./... -cover
```

The project includes:
- Unit tests for all components
- Integration tests for FSM
- Property-based tests (gopter) for encoding
- Tests for multi-group scenarios

---

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. 🐛 **Report bugs** via Issues
2. 💡 **Suggest new features** via Discussions
3. 🔧 **Submit Pull Requests**
4. 📖 **Improve documentation**
5. ⭐ **Star** the project

---

## 📝 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- [go-telegram/bot](https://github.com/go-telegram/bot) — excellent library for Telegram Bot API
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) — pure Go SQLite driver
- Go community for amazing tools and libraries

---

<div align="center">

**Made with ❤️ for communities who love predicting the future**

[⬆ Back to Top](#-telegram-prediction-market-bot)

</div>
