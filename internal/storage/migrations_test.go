package storage

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunMigrations(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "test_migrations_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Open database
	db, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create DBQueue
	queue := NewDBQueue(db)
	defer queue.Close()

	// Run migrations
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify schema_migrations table exists and has correct number of migrations
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}

	expectedCount := len(migrations)
	if count != expectedCount {
		t.Errorf("Expected %d migrations, got %d", expectedCount, count)
	}

	// Verify fsm_sessions table exists
	err = db.QueryRow("SELECT COUNT(*) FROM fsm_sessions").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query fsm_sessions: %v", err)
	}

	// Run migrations again (should be idempotent)
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations second time: %v", err)
	}

	// Verify migration count hasn't changed
	var newCount int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&newCount)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations after second run: %v", err)
	}

	if newCount != expectedCount {
		t.Errorf("Migration count changed after second run: expected %d, got %d", expectedCount, newCount)
	}
}

func TestFSMSessionsTableStructure(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "test_fsm_table_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Open database
	db, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create DBQueue
	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema and run migrations
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify table structure by attempting to insert and query
	_, err = db.Exec(`
		INSERT INTO fsm_sessions (user_id, state, context_json)
		VALUES (123, 'ask_question', '{"question":"test"}')
	`)
	if err != nil {
		t.Fatalf("Failed to insert into fsm_sessions: %v", err)
	}

	// Query the inserted row
	var userID int
	var state string
	var contextJSON string
	err = db.QueryRow("SELECT user_id, state, context_json FROM fsm_sessions WHERE user_id = 123").
		Scan(&userID, &state, &contextJSON)
	if err != nil {
		t.Fatalf("Failed to query fsm_sessions: %v", err)
	}

	if userID != 123 {
		t.Errorf("Expected user_id 123, got %d", userID)
	}
	if state != "ask_question" {
		t.Errorf("Expected state 'ask_question', got %s", state)
	}
	if contextJSON != `{"question":"test"}` {
		t.Errorf("Expected context_json '{\"question\":\"test\"}', got %s", contextJSON)
	}
}
