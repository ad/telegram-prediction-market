package storage

import (
	"database/sql"
	"fmt"
	"time"
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

// MigrateExistingDataToDefaultGroup migrates existing data to a default group
// This should be called after RunMigrations if migration version 3 was just applied
func MigrateExistingDataToDefaultGroup(queue *DBQueue, defaultGroupName string, createdBy int64) error {
	return queue.Execute(func(db *sql.DB) error {
		// Check if migration 4 has already been applied
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 4").Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}
		if count > 0 {
			// Migration already applied
			return nil
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Check if there's any existing data that needs migration
		var hasEvents, hasRatings, hasAchievements bool
		err = tx.QueryRow("SELECT COUNT(*) > 0 FROM events WHERE group_id IS NULL").Scan(&hasEvents)
		if err != nil {
			return fmt.Errorf("failed to check events: %w", err)
		}
		err = tx.QueryRow("SELECT COUNT(*) > 0 FROM ratings WHERE group_id IS NULL").Scan(&hasRatings)
		if err != nil {
			return fmt.Errorf("failed to check ratings: %w", err)
		}
		err = tx.QueryRow("SELECT COUNT(*) > 0 FROM achievements WHERE group_id IS NULL").Scan(&hasAchievements)
		if err != nil {
			return fmt.Errorf("failed to check achievements: %w", err)
		}

		// Only create default group if there's data to migrate
		if !hasEvents && !hasRatings && !hasAchievements {
			// Record migration as complete even though no data was migrated
			_, err = tx.Exec(
				"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
				4,
				"Migrate existing data to default group",
			)
			if err != nil {
				return fmt.Errorf("failed to record migration: %w", err)
			}
			return tx.Commit()
		}

		// Create default group with a special telegram_chat_id for migrated data
		// Using a special negative ID that won't conflict with real Telegram chat IDs
		result, err := tx.Exec(
			"INSERT INTO groups (telegram_chat_id, name, created_at, created_by) VALUES (?, ?, ?, ?)",
			-1, // Special ID for default migrated group
			defaultGroupName,
			time.Now(),
			createdBy,
		)
		if err != nil {
			return fmt.Errorf("failed to create default group: %w", err)
		}

		defaultGroupID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get default group ID: %w", err)
		}

		// Associate all existing events with default group
		_, err = tx.Exec("UPDATE events SET group_id = ? WHERE group_id IS NULL", defaultGroupID)
		if err != nil {
			return fmt.Errorf("failed to update events: %w", err)
		}

		// Associate all existing ratings with default group and recreate table with proper constraints
		_, err = tx.Exec("UPDATE ratings SET group_id = ? WHERE group_id IS NULL", defaultGroupID)
		if err != nil {
			return fmt.Errorf("failed to update ratings: %w", err)
		}

		// Now recreate ratings table with proper primary key constraint
		_, err = tx.Exec(`
CREATE TABLE ratings_final (
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

INSERT INTO ratings_final (user_id, group_id, username, score, correct_count, wrong_count, streak)
SELECT user_id, group_id, username, score, correct_count, wrong_count, streak FROM ratings;

DROP TABLE ratings;
ALTER TABLE ratings_final RENAME TO ratings;

CREATE INDEX IF NOT EXISTS idx_ratings_group_id ON ratings(group_id);
`)
		if err != nil {
			return fmt.Errorf("failed to recreate ratings table: %w", err)
		}

		// Associate all existing achievements with default group
		_, err = tx.Exec("UPDATE achievements SET group_id = ? WHERE group_id IS NULL", defaultGroupID)
		if err != nil {
			return fmt.Errorf("failed to update achievements: %w", err)
		}

		// Create group memberships for all users who have participated
		// Get all unique user IDs from ratings, events, and predictions
		_, err = tx.Exec(`
INSERT OR IGNORE INTO group_memberships (group_id, user_id, joined_at, status)
SELECT DISTINCT ?, user_id, ?, 'active'
FROM (
    SELECT user_id FROM ratings WHERE group_id = ?
    UNION
    SELECT created_by as user_id FROM events WHERE group_id = ?
    UNION
    SELECT user_id FROM predictions WHERE event_id IN (SELECT id FROM events WHERE group_id = ?)
)`, defaultGroupID, time.Now(), defaultGroupID, defaultGroupID, defaultGroupID)
		if err != nil {
			return fmt.Errorf("failed to create group memberships: %w", err)
		}

		// Record migration
		_, err = tx.Exec(
			"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
			4,
			"Migrate existing data to default group",
		)
		if err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		return tx.Commit()
	})
}
