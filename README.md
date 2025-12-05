# Telegram Prediction Market Bot

A Telegram bot for prediction markets where users can make forecasts on various events and compete in accuracy.

## Features

- **Multi-Group Support**: Host multiple independent prediction market communities in a single bot instance
  - Each group maintains isolated events, ratings, and achievements
  - Deep-link invitation system for easy group joining
  - Users can participate in multiple groups simultaneously
- **Event Creation**: Admins can create binary, multi-option, and probability-based prediction events
  - FSM-based multi-step creation flow with persistent state
  - Group selection for multi-group users
  - Automatic message cleanup for clean chat experience
  - Session recovery after bot restarts
  - Confirmation step before publishing
- **Voting System**: Non-anonymous polls with real-time vote distribution
- **Rating System**: Points-based scoring with bonuses for minority predictions and early voting
  - Separate ratings maintained per group
- **Achievements**: Badges for streaks, participation, and weekly top performers
  - Group-specific achievement tracking
  - Same achievements can be earned independently in different groups
- **Notifications**: Deadline reminders and event announcements
- **Admin Controls**: Event management with audit logging
  - Group creation and management
  - Member management with removal capabilities
  - Deep-link generation for invitations

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
export GROUP_ID="-1001234567890"  # Your supergroup ID (deprecated, kept for backward compatibility)
export ADMIN_USER_IDS="123456789,987654321"  # Comma-separated admin user IDs

# Optional
export DATABASE_PATH="./data/bot.db"  # Default: ./data/bot.db
export LOG_LEVEL="INFO"  # Default: INFO (options: DEBUG, INFO, WARN, ERROR)

# Multi-Group Settings
export DEFAULT_GROUP_NAME="Default Group"  # Name for default group during migration (default: "Default Group")
export MAX_GROUPS_PER_ADMIN="10"  # Maximum groups an admin can create (default: 10)
export MAX_MEMBERSHIPS_PER_USER="20"  # Maximum groups a user can join (default: 20)
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

- `/start` - Entry point: shows help or processes group invitation
- `/help` - Show help and available commands (role-based display)
- `/groups` - List all groups you're a member of
- `/rating` - View top 10 participants in your current group
- `/my` - View your personal statistics and achievements for your current group
- `/events` - List all active events from your groups

### Admin Commands

- `/create_group` - Create a new prediction market group
- `/list_groups` - List all groups with invitation links
- `/group_members` - View members of a specific group
- `/remove_member` - Remove a user from a group
- `/create_event` - Create a new prediction event (interactive multi-step flow with group selection)
- `/resolve_event` - Resolve an event and calculate scores
- `/edit_event` - Edit an event (only if no votes exist)

### Multi-Group Features

#### Joining Groups via Deep-Link Invitations

1. Admin creates a group using `/create_group`
2. Admin generates invitation link using `/list_groups`
3. Admin shares the deep-link (format: `https://t.me/your_bot?start=group_123`)
4. User clicks the link and starts the bot
5. Bot automatically adds user to the group and initializes their rating/achievements

**Group Isolation:**
- Each group maintains completely separate data:
  - Events are only visible to group members
  - Ratings are tracked independently per group
  - Achievements are earned separately in each group
  - Users can participate in multiple groups with different standings in each

**Membership Management:**
- Admins can view group members with `/group_members`
- Admins can remove members with `/remove_member`
- Removed users can rejoin via a new invitation link
- Historical data is preserved when users are removed

### Event Creation Flow

The `/create_event` command starts an interactive, multi-step process:

1. **Group Selection** (if you're in multiple groups): Choose which group the event is for
2. **Question**: Enter the prediction question
3. **Event Type**: Choose between Binary (Yes/No), Multi-option (2-6 choices), or Probability (0-25%, 25-50%, 50-75%, 75-100%)
4. **Options** (for multi-option only): Enter answer options (one per line)
5. **Deadline**: Enter deadline in format `DD.MM.YYYY HH:MM` (e.g., `25.12.2024 18:00`)
6. **Confirmation**: Review all details and confirm or cancel

**Key Features:**
- **Group Context**: Events are automatically scoped to the selected group
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
│   │   ├── event_creation_context.go  # FSM context data
│   │   ├── deeplink_service.go        # Deep-link generation/parsing
│   │   └── group_context_resolver.go  # Group context resolution
│   ├── logger/       # Structured logging
│   └── storage/      # Database repositories and schema
│       ├── fsm_storage.go             # FSM state persistence
│       ├── group_repository.go        # Group data access
│       └── group_membership_repository.go  # Membership management
└── .kiro/specs/      # Feature specifications and design docs
```

### Technical Architecture

**Multi-Group Architecture:**
- Each group maintains isolated data contexts
- Group identifiers stored with all group-scoped entities (events, ratings, achievements)
- Deep-link format: `https://t.me/{bot_username}?start=group_{groupID}`
- Automatic group context resolution for single-group users
- Group selection prompt for multi-group users during event creation

**Event Creation State Machine:**
- Uses `github.com/go-telegram/fsm` library for state management
- States: `select_group` (conditional) → `ask_question` → `ask_event_type` → `ask_options` (conditional) → `ask_deadline` → `confirm` → `complete`
- State and context data persisted in SQLite (`fsm_sessions` table)
- Automatic cleanup of stale sessions (>30 minutes inactive)
- Message IDs tracked in context for cleanup on completion

**Database Schema:**
- `groups`: Group metadata (id, name, created_at, created_by)
- `group_memberships`: User-group relationships (id, group_id, user_id, joined_at, status)
- `events`: Includes `group_id` for isolation
- `ratings`: Composite key (user_id, group_id) for per-group ratings
- `achievements`: Includes `group_id` for group-specific tracking
- `fsm_sessions`: Stores FSM state, context JSON (including group_id), and timestamps per user
- Indexed on `group_id` columns for efficient filtering
- Foreign key constraints ensure referential integrity

## License

See LICENSE file for details.