package storage

import "database/sql"

const schema = `
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
    poll_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
CREATE INDEX IF NOT EXISTS idx_events_deadline ON events(deadline);
CREATE INDEX IF NOT EXISTS idx_events_poll_id ON events(poll_id);

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
    user_id INTEGER PRIMARY KEY,
    username TEXT NOT NULL DEFAULT '',
    score INTEGER NOT NULL DEFAULT 0,
    correct_count INTEGER NOT NULL DEFAULT 0,
    wrong_count INTEGER NOT NULL DEFAULT 0,
    streak INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS achievements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    code TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    UNIQUE(user_id, code)
);

CREATE INDEX IF NOT EXISTS idx_achievements_user ON achievements(user_id);

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
`

// InitSchema initializes the database schema
func InitSchema(queue *DBQueue) error {
	return queue.Execute(func(db *sql.DB) error {
		_, err := db.Exec(schema)
		return err
	})
}
