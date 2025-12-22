package storage

import "database/sql"

const schema = `
CREATE TABLE IF NOT EXISTS groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_chat_id INTEGER NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    message_thread_id INTEGER,
    is_forum INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_groups_telegram_chat_id ON groups(telegram_chat_id);

CREATE TABLE IF NOT EXISTS group_memberships (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'active',
    FOREIGN KEY (group_id) REFERENCES groups(id),
    UNIQUE(group_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_group_memberships_group_id ON group_memberships(group_id);
CREATE INDEX IF NOT EXISTS idx_group_memberships_user_id ON group_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_group_memberships_status ON group_memberships(status);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    question TEXT NOT NULL,
    options_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    deadline TIMESTAMP NOT NULL,
    status TEXT NOT NULL,
    event_type TEXT NOT NULL,
    correct_option INTEGER,
    created_by INTEGER NOT NULL,
    poll_id TEXT,
    poll_message_id INTEGER,
    group_id INTEGER NOT NULL,
    message_thread_id INTEGER,
    FOREIGN KEY (group_id) REFERENCES groups(id)
);

CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
CREATE INDEX IF NOT EXISTS idx_events_deadline ON events(deadline);
CREATE INDEX IF NOT EXISTS idx_events_poll_id ON events(poll_id);
CREATE INDEX IF NOT EXISTS idx_events_group_id ON events(group_id);

CREATE TABLE IF NOT EXISTS predictions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    option INTEGER NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    FOREIGN KEY (event_id) REFERENCES events(id),
    UNIQUE(event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_predictions_event ON predictions(event_id);
CREATE INDEX IF NOT EXISTS idx_predictions_user ON predictions(user_id);

CREATE TABLE IF NOT EXISTS ratings (
    user_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    score INTEGER NOT NULL DEFAULT 0,
    correct_count INTEGER NOT NULL DEFAULT 0,
    wrong_count INTEGER NOT NULL DEFAULT 0,
    streak INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, group_id),
    FOREIGN KEY (group_id) REFERENCES groups(id)
);

CREATE INDEX IF NOT EXISTS idx_ratings_group_id ON ratings(group_id);

CREATE TABLE IF NOT EXISTS achievements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    code TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    group_id INTEGER NOT NULL,
    FOREIGN KEY (group_id) REFERENCES groups(id),
    UNIQUE(user_id, code, group_id)
);

CREATE INDEX IF NOT EXISTS idx_achievements_user ON achievements(user_id);
CREATE INDEX IF NOT EXISTS idx_achievements_group_id ON achievements(group_id);

CREATE TABLE IF NOT EXISTS admin_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_user_id INTEGER NOT NULL,
    action TEXT NOT NULL,
    event_id INTEGER,
    details TEXT,
    timestamp TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_logs_timestamp ON admin_logs(timestamp);

CREATE TABLE IF NOT EXISTS reminder_log (
    event_id INTEGER PRIMARY KEY,
    sent_at TIMESTAMP NOT NULL,
    FOREIGN KEY (event_id) REFERENCES events(id)
);

CREATE TABLE IF NOT EXISTS organizer_notifications (
    event_id INTEGER PRIMARY KEY,
    sent_at TIMESTAMP NOT NULL,
    FOREIGN KEY (event_id) REFERENCES events(id)
);

CREATE TABLE IF NOT EXISTS fsm_sessions (
    user_id INTEGER PRIMARY KEY,
    state TEXT NOT NULL,
    context_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    group_id INTEGER,
    FOREIGN KEY (group_id) REFERENCES groups(id)
);

CREATE INDEX IF NOT EXISTS idx_fsm_sessions_updated ON fsm_sessions(updated_at);
CREATE INDEX IF NOT EXISTS idx_fsm_sessions_group_id ON fsm_sessions(group_id);
`

// InitSchema initializes the database schema
func InitSchema(queue *DBQueue) error {
	return queue.Execute(func(db *sql.DB) error {
		_, err := db.Exec(schema)
		return err
	})
}
