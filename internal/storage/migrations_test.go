package storage

import (
	"database/sql"
	"os"
	"testing"
	"testing/quick"
	"time"

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

	// Initialize base schema first
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

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

// TestUniqueGroupIdentifiers is a property-based test that verifies
// all created groups have unique identifiers
func TestUniqueGroupIdentifiers(t *testing.T) {
	// Property: For any two groups created by the system, they should have different group identifiers
	property := func(groupNames []string) bool {
		if len(groupNames) == 0 {
			return true
		}

		// Create temporary database
		tmpFile, err := os.CreateTemp("", "test_unique_groups_*.db")
		if err != nil {
			t.Logf("Failed to create temp file: %v", err)
			return false
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Open database
		db, err := sql.Open("sqlite", tmpFile.Name())
		if err != nil {
			t.Logf("Failed to open database: %v", err)
			return false
		}
		defer db.Close()

		// Create DBQueue
		queue := NewDBQueue(db)
		defer queue.Close()

		// Initialize schema and run migrations
		if err := InitSchema(queue); err != nil {
			t.Logf("Failed to initialize schema: %v", err)
			return false
		}

		if err := RunMigrations(queue); err != nil {
			t.Logf("Failed to run migrations: %v", err)
			return false
		}

		// Create groups and collect their IDs
		groupIDs := make(map[int64]bool)
		createdBy := int64(1)

		for i, name := range groupNames {
			// Use a non-empty name
			if name == "" {
				name = "Group"
			}
			// Make names unique to avoid issues
			uniqueName := name + string(rune(i))

			var groupID int64
			err := queue.Execute(func(db *sql.DB) error {
				result, err := db.Exec(
					"INSERT INTO groups (telegram_chat_id, name, created_at, created_by) VALUES (?, ?, ?, ?)",
					int64(-1000000000000-i), // Unique telegram chat ID for each group
					uniqueName,
					time.Now(),
					createdBy,
				)
				if err != nil {
					return err
				}
				groupID, err = result.LastInsertId()
				return err
			})

			if err != nil {
				t.Logf("Failed to create group: %v", err)
				return false
			}

			// Check if this ID already exists
			if groupIDs[groupID] {
				t.Logf("Duplicate group ID found: %d", groupID)
				return false
			}
			groupIDs[groupID] = true
		}

		// All group IDs should be unique
		return len(groupIDs) == len(groupNames)
	}

	// Run property test with 100 iterations
	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(property, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestMigrateExistingDataToDefaultGroup tests the data migration function
func TestMigrateExistingDataToDefaultGroup(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "test_data_migration_*.db")
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

	// Create OLD schema (without group_id) to simulate pre-migration state
	err = queue.Execute(func(db *sql.DB) error {
		_, err := db.Exec(`
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

CREATE TABLE IF NOT EXISTS predictions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    option INTEGER NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    FOREIGN KEY (event_id) REFERENCES events(id),
    UNIQUE(event_id, user_id)
);

CREATE TABLE IF NOT EXISTS fsm_sessions (
    user_id INTEGER PRIMARY KEY,
    state TEXT NOT NULL,
    context_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)
		return err
	})
	if err != nil {
		t.Fatalf("Failed to create old schema: %v", err)
	}

	// Insert some test data before migration
	err = queue.Execute(func(db *sql.DB) error {
		// Insert test events
		_, err := db.Exec(`
			INSERT INTO events (question, options_json, created_at, deadline, status, event_type, created_by)
			VALUES ('Test Event 1', '["Yes", "No"]', datetime('now'), datetime('now', '+1 day'), 'active', 'binary', 123)
		`)
		if err != nil {
			return err
		}

		// Insert test ratings
		_, err = db.Exec(`
			INSERT INTO ratings (user_id, username, score, correct_count, wrong_count, streak)
			VALUES (123, 'testuser', 100, 5, 2, 3)
		`)
		if err != nil {
			return err
		}

		// Insert test achievements
		_, err = db.Exec(`
			INSERT INTO achievements (user_id, code, timestamp)
			VALUES (123, 'first_prediction', datetime('now'))
		`)
		return err
	})
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Run migrations (this will add the group_id columns)
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Run data migration
	defaultGroupName := "Test Default Group"
	createdBy := int64(1)
	if err := MigrateExistingDataToDefaultGroup(queue, defaultGroupName, createdBy); err != nil {
		t.Fatalf("Failed to migrate data: %v", err)
	}

	// Verify default group was created
	var groupID int64
	var groupName string
	err = db.QueryRow("SELECT id, name FROM groups WHERE name = ?", defaultGroupName).Scan(&groupID, &groupName)
	if err != nil {
		t.Fatalf("Failed to find default group: %v", err)
	}
	if groupName != defaultGroupName {
		t.Errorf("Expected group name '%s', got '%s'", defaultGroupName, groupName)
	}

	// Verify events were associated with default group
	var eventGroupID int64
	err = db.QueryRow("SELECT group_id FROM events WHERE question = 'Test Event 1'").Scan(&eventGroupID)
	if err != nil {
		t.Fatalf("Failed to query event: %v", err)
	}
	if eventGroupID != groupID {
		t.Errorf("Expected event group_id %d, got %d", groupID, eventGroupID)
	}

	// Verify ratings were associated with default group
	var ratingGroupID int64
	err = db.QueryRow("SELECT group_id FROM ratings WHERE user_id = 123").Scan(&ratingGroupID)
	if err != nil {
		t.Fatalf("Failed to query rating: %v", err)
	}
	if ratingGroupID != groupID {
		t.Errorf("Expected rating group_id %d, got %d", groupID, ratingGroupID)
	}

	// Verify achievements were associated with default group
	var achievementGroupID int64
	err = db.QueryRow("SELECT group_id FROM achievements WHERE user_id = 123").Scan(&achievementGroupID)
	if err != nil {
		t.Fatalf("Failed to query achievement: %v", err)
	}
	if achievementGroupID != groupID {
		t.Errorf("Expected achievement group_id %d, got %d", groupID, achievementGroupID)
	}

	// Verify group membership was created
	var membershipCount int
	err = db.QueryRow("SELECT COUNT(*) FROM group_memberships WHERE group_id = ? AND user_id = 123", groupID).Scan(&membershipCount)
	if err != nil {
		t.Fatalf("Failed to query group membership: %v", err)
	}
	if membershipCount != 1 {
		t.Errorf("Expected 1 group membership, got %d", membershipCount)
	}

	// Verify migration is idempotent (running again should not create duplicate data)
	if err := MigrateExistingDataToDefaultGroup(queue, defaultGroupName, createdBy); err != nil {
		t.Fatalf("Failed to run migration second time: %v", err)
	}

	// Verify only one default group exists
	var groupCount int
	err = db.QueryRow("SELECT COUNT(*) FROM groups").Scan(&groupCount)
	if err != nil {
		t.Fatalf("Failed to count groups: %v", err)
	}
	if groupCount != 1 {
		t.Errorf("Expected 1 group after second migration, got %d", groupCount)
	}
}
