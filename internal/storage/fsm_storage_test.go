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

func TestCorruptedSessionCleanup(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("corrupted JSON data causes session deletion and error logging", prop.ForAll(
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

			// Insert a session with corrupted JSON directly into the database
			corruptedJSON := "{invalid json data: this is not valid JSON"
			err = queue.Execute(func(db *sql.DB) error {
				_, err := db.ExecContext(ctx, `
					INSERT INTO fsm_sessions (user_id, state, context_json, created_at, updated_at)
					VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				`, userID, "ask_question", corruptedJSON)
				return err
			})
			if err != nil {
				t.Logf("Failed to insert corrupted session: %v", err)
				return false
			}

			// Verify session exists in database
			var count int
			err = queue.Execute(func(db *sql.DB) error {
				return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fsm_sessions WHERE user_id = ?", userID).Scan(&count)
			})
			if err != nil {
				t.Logf("Failed to query database: %v", err)
				return false
			}
			if count != 1 {
				t.Logf("Expected 1 session before Get, got %d", count)
				return false
			}

			// Try to get the session - should fail and delete the corrupted session
			_, _, err = storage.Get(ctx, userID)
			if err == nil {
				t.Logf("Expected error when getting corrupted session, got nil")
				return false
			}

			// Verify session was deleted
			err = queue.Execute(func(db *sql.DB) error {
				return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fsm_sessions WHERE user_id = ?", userID).Scan(&count)
			})
			if err != nil {
				t.Logf("Failed to query database after cleanup: %v", err)
				return false
			}
			if count != 0 {
				t.Logf("Expected 0 sessions after cleanup, got %d", count)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

// Feature: event-creation-ux-improvement, Property 17: Stale session cleanup logging
func TestStaleSessionCleanupLogging(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("stale session deletion creates log entry with user_id and cleanup reason", prop.ForAll(
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

			// Create a logger that captures output
			logOutput := &captureWriter{}
			log := logger.NewWithWriter(logger.INFO, logOutput)
			storage := NewFSMStorage(queue, log)
			ctx := context.Background()

			// Create stale sessions for all user IDs
			for _, userID := range dedupedUserIDs {
				state := "ask_question"
				context := map[string]interface{}{
					"question": "Test question?",
				}

				err = storage.Set(ctx, userID, state, context)
				if err != nil {
					t.Logf("Failed to set session: %v", err)
					return false
				}

				// Make the session stale
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
			}

			// Clear log output before cleanup
			logOutput.Reset()

			// Run cleanup
			err = storage.CleanupStale(ctx)
			if err != nil {
				t.Logf("Failed to cleanup stale sessions: %v", err)
				return false
			}

			// Verify log output contains entries for each stale session
			logContent := logOutput.String()

			for _, userID := range dedupedUserIDs {
				// Check that log contains user_id
				userIDStr := formatInt64(userID)
				if !containsString(logContent, userIDStr) {
					t.Logf("Log does not contain user_id %d", userID)
					return false
				}

				// Check that log contains cleanup reason
				if !containsString(logContent, "session_expired") && !containsString(logContent, "stale") {
					t.Logf("Log does not contain cleanup reason for user_id %d", userID)
					return false
				}
			}

			// Verify that cleanup completion is logged
			if !containsString(logContent, "cleanup completed") && !containsString(logContent, "cleaned up") {
				t.Logf("Log does not contain cleanup completion message")
				return false
			}

			return true
		},
		gen.SliceOf(gen.Int64Range(1, 1000000)),
	))

	properties.TestingRun(t)
}

func TestSessionIsolationByUser(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("concurrent sessions are isolated by user_id with no cross-contamination", prop.ForAll(
		func(userIDs []int64, questions []string, eventTypes []string) bool {
			if len(userIDs) == 0 || len(questions) == 0 || len(eventTypes) == 0 {
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

			// Create sessions for each user with unique data
			sessionData := make(map[int64]map[string]interface{})
			for i, userID := range dedupedUserIDs {
				question := questions[i%len(questions)]
				eventType := eventTypes[i%len(eventTypes)]
				state := "ask_question"

				context := map[string]interface{}{
					"question":   question,
					"event_type": eventType,
					"user_id":    userID, // Store user_id in context to verify isolation
				}

				err = storage.Set(ctx, userID, state, context)
				if err != nil {
					t.Logf("Failed to set session for user %d: %v", userID, err)
					return false
				}

				// Store expected data for verification
				sessionData[userID] = context
			}

			// Verify each session is isolated and contains correct data
			for _, userID := range dedupedUserIDs {
				state, data, err := storage.Get(ctx, userID)
				if err != nil {
					t.Logf("Failed to get session for user %d: %v", userID, err)
					return false
				}

				// Verify state
				if state != "ask_question" {
					t.Logf("State mismatch for user %d: expected ask_question, got %s", userID, state)
					return false
				}

				// Verify context data matches what we stored for this user
				expectedData := sessionData[userID]
				if data["question"] != expectedData["question"] {
					t.Logf("Question mismatch for user %d: expected %v, got %v", userID, expectedData["question"], data["question"])
					return false
				}

				if data["event_type"] != expectedData["event_type"] {
					t.Logf("Event type mismatch for user %d: expected %v, got %v", userID, expectedData["event_type"], data["event_type"])
					return false
				}

				// Verify user_id in context matches (no cross-contamination)
				contextUserID, ok := data["user_id"].(float64) // JSON unmarshals numbers as float64
				if !ok {
					t.Logf("user_id not found in context for user %d", userID)
					return false
				}

				if int64(contextUserID) != userID {
					t.Logf("Cross-contamination detected: user %d has context with user_id %d", userID, int64(contextUserID))
					return false
				}
			}

			// Verify total number of sessions matches number of unique users
			var count int
			err = queue.Execute(func(db *sql.DB) error {
				return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fsm_sessions").Scan(&count)
			})
			if err != nil {
				t.Logf("Failed to count sessions: %v", err)
				return false
			}

			if count != len(dedupedUserIDs) {
				t.Logf("Session count mismatch: expected %d, got %d", len(dedupedUserIDs), count)
				return false
			}

			return true
		},
		gen.SliceOf(gen.Int64Range(1, 1000000)),
		gen.SliceOf(gen.AlphaString()),
		gen.SliceOf(gen.OneConstOf("binary", "multi_option", "probability")),
	))

	properties.TestingRun(t)
}

// captureWriter is a simple writer that captures output for testing
type captureWriter struct {
	content string
}

func (w *captureWriter) Write(p []byte) (n int, err error) {
	w.content += string(p)
	return len(p), nil
}

func (w *captureWriter) String() string {
	return w.content
}

func (w *captureWriter) Reset() {
	w.content = ""
}

// Helper functions for string operations
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstringInString(s, substr)
}

func findSubstringInString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func formatInt64(n int64) string {
	// Simple int64 to string conversion
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}
