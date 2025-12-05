package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"telegram-prediction-bot/internal/logger"
)

var (
	// ErrSessionNotFound is returned when a session is not found
	ErrSessionNotFound = errors.New("session not found")
)

// FSMStorage implements persistent storage for FSM sessions
type FSMStorage struct {
	queue  *DBQueue
	logger *logger.Logger
}

// NewFSMStorage creates a new FSM storage backed by SQLite
func NewFSMStorage(queue *DBQueue, log *logger.Logger) *FSMStorage {
	return &FSMStorage{
		queue:  queue,
		logger: log,
	}
}

// Get retrieves FSM state and context for a user
func (s *FSMStorage) Get(ctx context.Context, userID int64) (state string, data map[string]interface{}, err error) {
	var contextJSON string
	var updatedAt time.Time

	err = s.queue.Execute(func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, `
			SELECT state, context_json, updated_at
			FROM fsm_sessions
			WHERE user_id = ?
		`, userID)

		return row.Scan(&state, &contextJSON, &updatedAt)
	})

	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Debug("session not found", "user_id", userID)
			return "", nil, ErrSessionNotFound
		}
		s.logger.Error("failed to get session", "user_id", userID, "error", err)
		return "", nil, err
	}

	// Deserialize context JSON
	if err := json.Unmarshal([]byte(contextJSON), &data); err != nil {
		s.logger.Error("failed to unmarshal context", "user_id", userID, "error", err)
		// Delete corrupted session
		_ = s.Delete(ctx, userID)
		return "", nil, err
	}

	s.logger.Debug("session retrieved", "user_id", userID, "state", state)
	return state, data, nil
}

// Set stores FSM state and context for a user using atomic transaction
func (s *FSMStorage) Set(ctx context.Context, userID int64, state string, data map[string]interface{}) error {
	// Serialize context to JSON
	contextJSON, err := json.Marshal(data)
	if err != nil {
		s.logger.Error("failed to marshal context", "user_id", userID, "error", err)
		return err
	}

	err = s.queue.Execute(func(db *sql.DB) error {
		// Use transaction for atomic update
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		// Insert or replace session
		_, err = tx.ExecContext(ctx, `
			INSERT INTO fsm_sessions (user_id, state, context_json, created_at, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET
				state = excluded.state,
				context_json = excluded.context_json,
				updated_at = CURRENT_TIMESTAMP
		`, userID, state, string(contextJSON))

		if err != nil {
			return err
		}

		return tx.Commit()
	})

	if err != nil {
		s.logger.Error("failed to set session", "user_id", userID, "state", state, "error", err)
		return err
	}

	s.logger.Debug("session stored", "user_id", userID, "state", state)
	return nil
}

// Delete removes FSM session for a user
func (s *FSMStorage) Delete(ctx context.Context, userID int64) error {
	err := s.queue.Execute(func(db *sql.DB) error {
		// Use transaction for atomic delete
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		result, err := tx.ExecContext(ctx, `
			DELETE FROM fsm_sessions WHERE user_id = ?
		`, userID)

		if err != nil {
			return err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if rowsAffected == 0 {
			s.logger.Debug("session not found for deletion", "user_id", userID)
		}

		return tx.Commit()
	})

	if err != nil {
		s.logger.Error("failed to delete session", "user_id", userID, "error", err)
		return err
	}

	s.logger.Debug("session deleted", "user_id", userID)
	return nil
}

// CleanupStale removes sessions older than 30 minutes
func (s *FSMStorage) CleanupStale(ctx context.Context) error {
	var deletedCount int64
	err := s.queue.Execute(func(db *sql.DB) error {
		// Use transaction for atomic cleanup
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		// Use SQLite's datetime function to calculate the threshold
		result, err := tx.ExecContext(ctx, `
			DELETE FROM fsm_sessions
			WHERE updated_at < datetime('now', '-30 minutes')
		`)

		if err != nil {
			return err
		}

		deletedCount, err = result.RowsAffected()
		if err != nil {
			return err
		}

		return tx.Commit()
	})

	if err != nil {
		s.logger.Error("failed to cleanup stale sessions", "error", err)
		return err
	}

	if deletedCount > 0 {
		s.logger.Info("cleaned up stale sessions", "count", deletedCount)
	} else {
		s.logger.Debug("no stale sessions to cleanup")
	}

	return nil
}
