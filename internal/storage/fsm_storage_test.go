package storage

import (
	"context"
	"database/sql"
	"testing"

	"telegram-prediction-bot/internal/logger"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

// Feature: event-creation-ux-improvement, Property 3: FSM state persistence on start
// Validates: Requirements 2.1
func TestFSMStatePersistenceOnStart(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("starting event creation creates database record with user_id, initial state, and empty context", prop.ForAll(
		func(userID int64) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			log := logger.New(logger.ERROR)
			storage := NewFSMStorage(queue, log)
			ctx := context.Background()

			// Start a new session with initial state and empty context
			initialState := "ask_question"
			emptyContext := make(map[string]interface{})

			err = storage.Set(ctx, userID, initialState, emptyContext)
			if err != nil {
				t.Logf("Failed to set session: %v", err)
				return false
			}

			// Verify the session was created
			state, data, err := storage.Get(ctx, userID)
			if err != nil {
				t.Logf("Failed to get session: %v", err)
				return false
			}

			// Check that state matches
			if state != initialState {
				t.Logf("State mismatch: expected %s, got %s", initialState, state)
				return false
			}

			// Check that context is empty (or at least not nil)
			if data == nil {
				t.Logf("Context is nil")
				return false
			}

			// Verify database record exists
			var count int
			err = queue.Execute(func(db *sql.DB) error {
				return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fsm_sessions WHERE user_id = ?", userID).Scan(&count)
			})
			if err != nil {
				t.Logf("Failed to query database: %v", err)
				return false
			}

			if count != 1 {
				t.Logf("Expected 1 record, got %d", count)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

// Feature: event-creation-ux-improvement, Property 4: FSM state persistence on transitions
// Validates: Requirements 2.2
func TestFSMStatePersistenceOnTransitions(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("state transitions update database with new state and context", prop.ForAll(
		func(userID int64, question string, eventType string) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			log := logger.New(logger.ERROR)
			storage := NewFSMStorage(queue, log)
			ctx := context.Background()

			// Start with initial state
			initialState := "ask_question"
			initialContext := make(map[string]interface{})

			err = storage.Set(ctx, userID, initialState, initialContext)
			if err != nil {
				t.Logf("Failed to set initial session: %v", err)
				return false
			}

			// Transition to next state with updated context
			nextState := "ask_event_type"
			updatedContext := map[string]interface{}{
				"question":   question,
				"event_type": eventType,
			}

			err = storage.Set(ctx, userID, nextState, updatedContext)
			if err != nil {
				t.Logf("Failed to update session: %v", err)
				return false
			}

			// Verify the session was updated
			state, data, err := storage.Get(ctx, userID)
			if err != nil {
				t.Logf("Failed to get session: %v", err)
				return false
			}

			// Check that state was updated
			if state != nextState {
				t.Logf("State not updated: expected %s, got %s", nextState, state)
				return false
			}

			// Check that context was updated
			if data["question"] != question {
				t.Logf("Question not updated: expected %s, got %v", question, data["question"])
				return false
			}

			if data["event_type"] != eventType {
				t.Logf("Event type not updated: expected %s, got %v", eventType, data["event_type"])
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.AlphaString(),
		gen.OneConstOf("binary", "multi_option", "probability"),
	))

	properties.TestingRun(t)
}

// Feature: event-creation-ux-improvement, Property 7: FSM session cleanup on completion
// Validates: Requirements 2.5
func TestFSMSessionCleanupOnCompletion(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("completing or cancelling event creation deletes FSM session", prop.ForAll(
		func(userID int64) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			log := logger.New(logger.ERROR)
			storage := NewFSMStorage(queue, log)
			ctx := context.Background()

			// Create a session
			state := "confirm"
			context := map[string]interface{}{
				"question":   "Test question?",
				"event_type": "binary",
			}

			err = storage.Set(ctx, userID, state, context)
			if err != nil {
				t.Logf("Failed to set session: %v", err)
				return false
			}

			// Verify session exists
			_, _, err = storage.Get(ctx, userID)
			if err != nil {
				t.Logf("Session should exist but got error: %v", err)
				return false
			}

			// Complete/cancel by deleting the session
			err = storage.Delete(ctx, userID)
			if err != nil {
				t.Logf("Failed to delete session: %v", err)
				return false
			}

			// Verify session no longer exists
			_, _, err = storage.Get(ctx, userID)
			if err != ErrSessionNotFound {
				t.Logf("Session should not exist, expected ErrSessionNotFound, got: %v", err)
				return false
			}

			// Verify database record is gone
			var count int
			err = queue.Execute(func(db *sql.DB) error {
				return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fsm_sessions WHERE user_id = ?", userID).Scan(&count)
			})
			if err != nil {
				t.Logf("Failed to query database: %v", err)
				return false
			}

			if count != 0 {
				t.Logf("Expected 0 records, got %d", count)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

// Feature: event-creation-ux-improvement, Property 15: Stale session expiration
// Validates: Requirements 5.1
func TestStaleSessionExpiration(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("sessions inactive for more than 30 minutes are automatically deleted", prop.ForAll(
		func(userID int64) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			log := logger.New(logger.ERROR)
			storage := NewFSMStorage(queue, log)
			ctx := context.Background()

			// Create a session
			state := "ask_question"
			context := map[string]interface{}{
				"question": "Test question?",
			}

			err = storage.Set(ctx, userID, state, context)
			if err != nil {
				t.Logf("Failed to set session: %v", err)
				return false
			}

			// Manually update the updated_at timestamp to be older than 30 minutes
			staleTime := "datetime('now', '-31 minutes')"
			err = queue.Execute(func(db *sql.DB) error {
				_, err := db.ExecContext(ctx,
					"UPDATE fsm_sessions SET updated_at = "+staleTime+" WHERE user_id = ?",
					userID)
				return err
			})
			if err != nil {
				t.Logf("Failed to update timestamp: %v", err)
				return false
			}

			// Run cleanup
			err = storage.CleanupStale(ctx)
			if err != nil {
				t.Logf("Failed to cleanup stale sessions: %v", err)
				return false
			}

			// Verify session was deleted
			_, _, err = storage.Get(ctx, userID)
			if err != ErrSessionNotFound {
				t.Logf("Stale session should be deleted, expected ErrSessionNotFound, got: %v", err)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

// Feature: event-creation-ux-improvement, Property 16: Startup stale session cleanup
// Validates: Requirements 5.2
func TestStartupStaleSessionCleanup(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("bot startup deletes all sessions with last_update older than 30 minutes", prop.ForAll(
		func(userIDs []int64) bool {
			if len(userIDs) == 0 {
				return true // Skip empty case
			}

			// Deduplicate user IDs to avoid overwriting sessions
			uniqueUserIDs := make(map[int64]bool)
			var dedupedUserIDs []int64
			for _, userID := range userIDs {
				if userID > 0 && !uniqueUserIDs[userID] {
					uniqueUserIDs[userID] = true
					dedupedUserIDs = append(dedupedUserIDs, userID)
				}
			}

			if len(dedupedUserIDs) == 0 {
				return true // Skip if no valid user IDs
			}

			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			log := logger.New(logger.ERROR)
			storage := NewFSMStorage(queue, log)
			ctx := context.Background()

			// Create multiple sessions, some stale and some fresh
			staleCount := 0
			freshCount := 0

			for i, userID := range dedupedUserIDs {

				state := "ask_question"
				context := map[string]interface{}{
					"question": "Test question?",
				}

				err = storage.Set(ctx, userID, state, context)
				if err != nil {
					t.Logf("Failed to set session: %v", err)
					return false
				}

				// Make half of them stale
				if i%2 == 0 {
					staleTime := "datetime('now', '-31 minutes')"
					err = queue.Execute(func(db *sql.DB) error {
						_, err := db.ExecContext(ctx,
							"UPDATE fsm_sessions SET updated_at = "+staleTime+" WHERE user_id = ?",
							userID)
						return err
					})
					if err != nil {
						t.Logf("Failed to update timestamp: %v", err)
						return false
					}
					staleCount++
				} else {
					freshCount++
				}
			}

			// Simulate bot startup by running cleanup
			err = storage.CleanupStale(ctx)
			if err != nil {
				t.Logf("Failed to cleanup stale sessions: %v", err)
				return false
			}

			// Count remaining sessions
			var remainingCount int
			err = queue.Execute(func(db *sql.DB) error {
				return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fsm_sessions").Scan(&remainingCount)
			})
			if err != nil {
				t.Logf("Failed to count sessions: %v", err)
				return false
			}

			// Should only have fresh sessions remaining
			if remainingCount != freshCount {
				t.Logf("Expected %d fresh sessions, got %d", freshCount, remainingCount)
				return false
			}

			return true
		},
		gen.SliceOf(gen.Int64Range(1, 1000000)),
	))

	properties.TestingRun(t)
}
