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

			// Start transaction
			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
			}

			// Execute migration SQL
			_, err = tx.Exec(migration.SQL)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed to execute migration %d (%s): %w", migration.Version, migration.Description, err)
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
