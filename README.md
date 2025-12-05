# Telegram Prediction Market Bot

A Telegram bot for prediction markets where users can make forecasts on various events and compete in accuracy.

## Features

- **Event Creation**: Admins can create binary, multi-option, and probability-based prediction events
- FSM-based multi-step creation flow with persistent state
- Automatic message cleanup for clean chat experience
- Session recovery after bot restarts
- Confirmation step before publishing
- **Voting System**: Non-anonymous polls with real-time vote distribution
- **Rating System**: Points-based scoring with bonuses for minority predictions and early voting
- **Achievements**: Badges for streaks, participation, and weekly top performers
- **Notifications**: Deadline reminders and event announcements
- **Admin Controls**: Event management with audit logging

## Requirements

- Go 1.25.5 or higher
- SQLite (via modernc.org/sqlite)
- Telegram Bot Token (from @BotFather)
- Telegram Supergroup with bot as admin

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd telegram-prediction-market
```

2. Install dependencies:
```bash
go mod download
```

3. Build the bot:
```bash
go build -o bin/bot ./cmd/bot
```

## Configuration

Set the following environment variables:

```bash
# Required
export TELEGRAM_TOKEN="your-bot-token"
export GROUP_ID="-1001234567890"  # Your supergroup ID
export ADMIN_USER_IDS="123456789,987654321"  # Comma-separated admin user IDs

# Optional
export DATABASE_PATH="./data/bot.db"  # Default: ./data/bot.db
export LOG_LEVEL="INFO"  # Default: INFO (options: DEBUG, INFO, WARN, ERROR)
```

Or create a `.env` file (see `.env.example`).

## Running

```bash
./bin/bot
```

Or run directly with Go:
```bash
go run ./cmd/bot
```

## Usage

### User Commands

- `/help` - Show help and available commands
- `/rating` - View top 10 participants
- `/my` - View your personal statistics and achievements
- `/events` - List all active events

### Admin Commands

- `/create_event` - Create a new prediction event (interactive multi-step flow)
- `/resolve_event` - Resolve an event and calculate scores
- `/edit_event` - Edit an event (only if no votes exist)

### Event Creation Flow

The `/create_event` command starts an interactive, multi-step process:

1. **Question**: Enter the prediction question
2. **Event Type**: Choose between Binary (Yes/No), Multi-option (2-6 choices), or Probability (0-25%, 25-50%, 50-75%, 75-100%)
3. **Options** (for multi-option only): Enter answer options (one per line)
4. **Deadline**: Enter deadline in format `DD.MM.YYYY HH:MM` (e.g., `25.12.2024 18:00`)
5. **Confirmation**: Review all details and confirm or cancel

**Key Features:**
- **Clean Chat**: All intermediate messages are automatically deleted, leaving only the final result
- **Persistent Sessions**: Your progress is saved - if the bot restarts, you can continue where you left off
- **Validation**: Input is validated at each step with helpful error messages
- **Session Timeout**: Sessions expire after 30 minutes of inactivity
- **Concurrent Creation**: Multiple admins can create events simultaneously without interference

## Scoring Rules

### Base Points
- Binary event (Yes/No): **+10 points**
- Multi-option event (3-6 options): **+15 points**
- Probability event: **+15 points**

### Bonuses
- Minority prediction (<40% votes): **+5 points**
- Early voting (first 12 hours): **+3 points**
- Participation: **+1 point**

### Penalties
- Incorrect prediction: **-3 points**

## Achievements

- 🎯 **Меткий стрелок** - 3 correct predictions in a row
- 🔮 **Провидец** - 10 correct predictions in a row
- 🎲 **Риск-мейкер** - 3 correct minority predictions in a row
- 📊 **Аналитик недели** - Most points in a week
- 🏆 **Старожил** - Participation in 50 events

## Development

### Running Tests

```bash
go test ./...
```

### Running with Verbose Output

```bash
go test ./... -v
```

### Project Structure

```
.
├── cmd/bot/           # Main application entry point
├── internal/
│   ├── bot/          # Telegram bot handlers and FSM
│   │   ├── handler.go              # Main bot handler
│   │   ├── event_creation_fsm.go   # FSM-based event creation
│   │   └── message_deletion.go     # Message cleanup utilities
│   ├── config/       # Configuration management
│   ├── domain/       # Business logic and domain models
│   │   └── event_creation_context.go  # FSM context data
│   ├── logger/       # Structured logging
│   └── storage/      # Database repositories and schema
│       └── fsm_storage.go          # FSM state persistence
└── .kiro/specs/      # Feature specifications and design docs
```

### Technical Architecture

**Event Creation State Machine:**
- Uses `github.com/go-telegram/fsm` library for state management
- States: `ask_question` → `ask_event_type` → `ask_options` (conditional) → `ask_deadline` → `confirm` → `complete`
- State and context data persisted in SQLite (`fsm_sessions` table)
- Automatic cleanup of stale sessions (>30 minutes inactive)
- Message IDs tracked in context for cleanup on completion

**Database Schema:**
- `fsm_sessions`: Stores FSM state, context JSON, and timestamps per user
- Indexed on `updated_at` for efficient stale session cleanup
- Atomic transactions ensure data consistency

## License

See LICENSE file for details.