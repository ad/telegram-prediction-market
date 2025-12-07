package storage

import (
	"database/sql"
	"fmt"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	SQL         string
}

// migrations contains all database migrations in order
var migrations = []Migration{
	{
		Version:     1,
		Description: "Add fsm_sessions table for event creation state management",
		SQL: `
CREATE TABLE IF NOT EXISTS fsm_sessions (
    user_id INTEGER PRIMARY KEY,
    state TEXT NOT NULL,
    context_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_fsm_sessions_updated ON fsm_sessions(updated_at);
`,
	},
	{
		Version:     2,
		Description: "Add groups and group_memberships tables for multi-group support",
		SQL: `
CREATE TABLE IF NOT EXISTS groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_chat_id INTEGER NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL
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
`,
	},
	{
		Version:     3,
		Description: "Add group_id column to existing tables",
		SQL: `
-- Add group_id to events table (nullable for now, will be populated in migration 4)
ALTER TABLE events ADD COLUMN group_id INTEGER REFERENCES groups(id);
CREATE INDEX IF NOT EXISTS idx_events_group_id ON events(group_id);

-- Add group_id to achievements table (nullable for now, will be populated in migration 4)
ALTER TABLE achievements ADD COLUMN group_id INTEGER REFERENCES groups(id);
CREATE INDEX IF NOT EXISTS idx_achievements_group_id ON achievements(group_id);

-- Add group_id to fsm_sessions table (nullable for now, will be populated in migration 4)
ALTER TABLE fsm_sessions ADD COLUMN group_id INTEGER REFERENCES groups(id);
CREATE INDEX IF NOT EXISTS idx_fsm_sessions_group_id ON fsm_sessions(group_id);

-- Recreate ratings table with composite primary key (user_id, group_id)
-- SQLite doesn't support modifying primary keys, so we need to recreate the table
-- Note: group_id is nullable temporarily to allow data migration
CREATE TABLE ratings_new (
    user_id INTEGER NOT NULL,
    group_id INTEGER,
    username TEXT NOT NULL DEFAULT '',
    score INTEGER NOT NULL DEFAULT 0,
    correct_count INTEGER NOT NULL DEFAULT 0,
    wrong_count INTEGER NOT NULL DEFAULT 0,
    streak INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (group_id) REFERENCES groups(id)
);

-- Copy existing data (will be populated with group_id in next migration)
INSERT INTO ratings_new (user_id, username, score, correct_count, wrong_count, streak, group_id)
SELECT user_id, username, score, correct_count, wrong_count, streak, NULL FROM ratings;

-- Drop old table and rename new one
DROP TABLE ratings;
ALTER TABLE ratings_new RENAME TO ratings;

-- Create index on group_id for ratings
CREATE INDEX IF NOT EXISTS idx_ratings_group_id ON ratings(group_id);
`,
	},
	{
		Version:     4,
		Description: "Add poll_message_id column to events table for stopping polls",
		SQL: `
-- Add poll_message_id to events table to store Telegram message ID of the poll
ALTER TABLE events ADD COLUMN poll_message_id INTEGER;
`,
	},
	{
		Version:     5,
		Description: "Add forum support: message_thread_id to events and groups, is_forum flag to groups",
		SQL: `
-- Add message_thread_id to events table for forum topic support
ALTER TABLE events ADD COLUMN message_thread_id INTEGER;

-- Add message_thread_id to groups table for default forum topic
ALTER TABLE groups ADD COLUMN message_thread_id INTEGER;

-- Add is_forum flag to groups table
ALTER TABLE groups ADD COLUMN is_forum INTEGER NOT NULL DEFAULT 0;
`,
	},
	{
		Version:     6,
		Description: "Add forum_topics table for managing forum topics separately from groups",
		SQL: `
-- Create forum_topics table to store individual forum topics
-- A forum (group) can have multiple topics, but shares the same rating/stats
CREATE TABLE IF NOT EXISTS forum_topics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL,
    message_thread_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    FOREIGN KEY (group_id) REFERENCES groups(id),
    UNIQUE(group_id, message_thread_id)
);

CREATE INDEX IF NOT EXISTS idx_forum_topics_group_id ON forum_topics(group_id);
CREATE INDEX IF NOT EXISTS idx_forum_topics_message_thread_id ON forum_topics(message_thread_id);
`,
	},
	{
		Version:     7,
		Description: "Refactor forum support: add forum_topic_id to events, deprecate message_thread_id columns",
		SQL: `
-- Add forum_topic_id to events table (nullable for backward compatibility)
ALTER TABLE events ADD COLUMN forum_topic_id INTEGER REFERENCES forum_topics(id);
CREATE INDEX IF NOT EXISTS idx_events_forum_topic_id ON events(forum_topic_id);

-- Note: We keep message_thread_id columns in events and groups for backward compatibility
-- but new code should use forum_topic_id and get message_thread_id from forum_topics table
`,
	},
	{
		Version:     8,
		Description: "Remove deprecated message_thread_id columns from events and groups",
		SQL: `
-- SQLite doesn't support DROP COLUMN directly in older versions
-- We need to recreate tables without the deprecated columns

-- 1. Recreate events table without message_thread_id
CREATE TABLE events_new (
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
    group_id INTEGER REFERENCES groups(id),
    poll_message_id INTEGER,
    forum_topic_id INTEGER REFERENCES forum_topics(id)
);

-- Copy data from old table
INSERT INTO events_new (id, question, options_json, created_at, deadline, status, event_type, correct_option, created_by, poll_id, group_id, poll_message_id, forum_topic_id)
SELECT id, question, options_json, created_at, deadline, status, event_type, correct_option, created_by, poll_id, group_id, poll_message_id, forum_topic_id
FROM events;

-- Drop old table and rename new one
DROP TABLE events;
ALTER TABLE events_new RENAME TO events;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_events_group_id ON events(group_id);
CREATE INDEX IF NOT EXISTS idx_events_forum_topic_id ON events(forum_topic_id);

-- 2. Recreate groups table without message_thread_id
CREATE TABLE groups_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_chat_id INTEGER NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    is_forum INTEGER NOT NULL DEFAULT 0
);

-- Copy data from old table
INSERT INTO groups_new (id, telegram_chat_id, name, created_at, created_by, is_forum)
SELECT id, telegram_chat_id, name, created_at, created_by, is_forum
FROM groups;

-- Drop old table and rename new one
DROP TABLE groups;
ALTER TABLE groups_new RENAME TO groups;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_groups_telegram_chat_id ON groups(telegram_chat_id);
`,
	},
	{
		Version:     9,
		Description: "Add status column to groups table for soft delete support",
		SQL: `
-- Add status column to groups table (default 'active')
ALTER TABLE groups ADD COLUMN status TEXT NOT NULL DEFAULT 'active';

-- Create index on status for faster queries
CREATE INDEX IF NOT EXISTS idx_groups_status ON groups(status);
`,
	},
}

// columnExists checks if a column exists in a table
func columnExists(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, rows.Err()
}

// RunMigrations executes all pending migrations
func RunMigrations(queue *DBQueue) error {
	return queue.Execute(func(db *sql.DB) error {
		// Create migrations table if it doesn't exist
		_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    description TEXT NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`)
		if err != nil {
			return fmt.Errorf("failed to create migrations table: %w", err)
		}

		// Get current version
		var currentVersion int
		err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
		if err != nil {
			return fmt.Errorf("failed to get current migration version: %w", err)
		}

		// Apply pending migrations
		for _, migration := range migrations {
			if migration.Version <= currentVersion {
				continue
			}

			// Special handling for migration 3 - check if columns already exist
			if migration.Version == 3 {
				// Check if group_id already exists in events table
				exists, err := columnExists(db, "events", "group_id")
				if err != nil {
					return fmt.Errorf("failed to check column existence: %w", err)
				}
				if exists {
					// Columns already exist, just mark migration as complete
					_, err = db.Exec(
						"INSERT OR IGNORE INTO schema_migrations (version, description) VALUES (?, ?)",
						migration.Version,
						migration.Description,
					)
					if err != nil {
						return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
					}
					continue
				}
			}

			// Special handling for migration 4 - check if column already exists
			if migration.Version == 4 {
				// Check if poll_message_id already exists in events table
				exists, err := columnExists(db, "events", "poll_message_id")
				if err != nil {
					return fmt.Errorf("failed to check column existence: %w", err)
				}
				if exists {
					// Column already exists, just mark migration as complete
					_, err = db.Exec(
						"INSERT OR IGNORE INTO schema_migrations (version, description) VALUES (?, ?)",
						migration.Version,
						migration.Description,
					)
					if err != nil {
						return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
					}
					continue
				}
			}

			// Special handling for migration 5 - check if columns already exist
			if migration.Version == 5 {
				// Check if message_thread_id already exists in events table
				exists, err := columnExists(db, "events", "message_thread_id")
				if err != nil {
					return fmt.Errorf("failed to check column existence: %w", err)
				}
				if exists {
					// Columns already exist, just mark migration as complete
					_, err = db.Exec(
						"INSERT OR IGNORE INTO schema_migrations (version, description) VALUES (?, ?)",
						migration.Version,
						migration.Description,
					)
					if err != nil {
						return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
					}
					continue
				}
			}

			// Special handling for migration 6 - check if table already exists
			if migration.Version == 6 {
				// Check if forum_topics table already exists
				var tableName string
				err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='forum_topics'").Scan(&tableName)
				if err == nil {
					// Table already exists, just mark migration as complete
					_, err = db.Exec(
						"INSERT OR IGNORE INTO schema_migrations (version, description) VALUES (?, ?)",
						migration.Version,
						migration.Description,
					)
					if err != nil {
						return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
					}
					continue
				}
			}

			// Special handling for migration 7 - check if column already exists
			if migration.Version == 7 {
				// Check if forum_topic_id already exists in events table
				exists, err := columnExists(db, "events", "forum_topic_id")
				if err != nil {
					return fmt.Errorf("failed to check column existence: %w", err)
				}
				if exists {
					// Column already exists, just mark migration as complete
					_, err = db.Exec(
						"INSERT OR IGNORE INTO schema_migrations (version, description) VALUES (?, ?)",
						migration.Version,
						migration.Description,
					)
					if err != nil {
						return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
					}
					continue
				}
			}

			// Special handling for migration 8 - check if message_thread_id was already removed
			if migration.Version == 8 {
				// Check if message_thread_id still exists in events table
				exists, err := columnExists(db, "events", "message_thread_id")
				if err != nil {
					return fmt.Errorf("failed to check column existence: %w", err)
				}
				if !exists {
					// Column already removed, just mark migration as complete
					_, err = db.Exec(
						"INSERT OR IGNORE INTO schema_migrations (version, description) VALUES (?, ?)",
						migration.Version,
						migration.Description,
					)
					if err != nil {
						return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
					}
					continue
				}
			}

			// Special handling for migration 9 - check if column already exists
			if migration.Version == 9 {
				// Check if status already exists in groups table
				exists, err := columnExists(db, "groups", "status")
				if err != nil {
					return fmt.Errorf("failed to check column existence: %w", err)
				}
				if exists {
					// Column already exists, just mark migration as complete
					_, err = db.Exec(
						"INSERT OR IGNORE INTO schema_migrations (version, description) VALUES (?, ?)",
						migration.Version,
						migration.Description,
					)
					if err != nil {
						return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
					}
					continue
				}
			}

			// Start transaction
			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
			}

			// Execute migration SQL (skip if empty)
			if migration.SQL != "" && len(migration.SQL) > 10 {
				_, err = tx.Exec(migration.SQL)
				if err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("failed to execute migration %d (%s): %w", migration.Version, migration.Description, err)
				}
			}

			// Record migration
			_, err = tx.Exec(
				"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
				migration.Version,
				migration.Description,
			)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
			}

			// Commit transaction
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
			}
		}

		return nil
	})
}
