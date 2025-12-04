package storage

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"telegram-prediction-bot/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

// Feature: telegram-prediction-bot, Property 15: Event data round-trip
// Validates: Requirements 8.2
func TestEventDataRoundTrip(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	repo := NewEventRepository(queue)

	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("event round-trip preserves all fields", prop.ForAll(
		func(question string, optionCount int, eventType domain.EventType, createdBy int64) bool {
			// Generate valid options based on event type
			var options []string
			switch eventType {
			case domain.EventTypeBinary:
				options = []string{"Yes", "No"}
			case domain.EventTypeProbability:
				options = []string{"0-25%", "25-50%", "50-75%", "75-100%"}
			case domain.EventTypeMultiOption:
				// Use optionCount to determine number of options (2-6)
				count := 2 + (optionCount % 5) // ensures 2-6 options
				options = make([]string, count)
				for i := 0; i < count; i++ {
					options[i] = fmt.Sprintf("Option %d", i+1)
				}
			}

			// Create event with valid data
			now := time.Now().Truncate(time.Second)
			deadline := now.Add(24 * time.Hour)

			event := &domain.Event{
				Question:  question,
				Options:   options,
				CreatedAt: now,
				Deadline:  deadline,
				Status:    domain.EventStatusActive,
				EventType: eventType,
				CreatedBy: createdBy,
				PollID:    "test_poll_" + question[:min(10, len(question))],
			}

			// Skip invalid events
			if err := event.Validate(); err != nil {
				return true // Skip invalid inputs
			}

			ctx := context.Background()

			// Save event
			if err := repo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Retrieve event
			retrieved, err := repo.GetEvent(ctx, event.ID)
			if err != nil {
				t.Logf("Failed to get event: %v", err)
				return false
			}

			// Verify all fields match
			if retrieved.ID != event.ID {
				t.Logf("ID mismatch: expected %d, got %d", event.ID, retrieved.ID)
				return false
			}
			if retrieved.Question != event.Question {
				t.Logf("Question mismatch: expected %s, got %s", event.Question, retrieved.Question)
				return false
			}
			if len(retrieved.Options) != len(event.Options) {
				t.Logf("Options length mismatch: expected %d, got %d", len(event.Options), len(retrieved.Options))
				return false
			}
			for i := range event.Options {
				if retrieved.Options[i] != event.Options[i] {
					t.Logf("Option %d mismatch: expected %s, got %s", i, event.Options[i], retrieved.Options[i])
					return false
				}
			}
			if !retrieved.CreatedAt.Equal(event.CreatedAt) {
				t.Logf("CreatedAt mismatch: expected %v, got %v", event.CreatedAt, retrieved.CreatedAt)
				return false
			}
			if !retrieved.Deadline.Equal(event.Deadline) {
				t.Logf("Deadline mismatch: expected %v, got %v", event.Deadline, retrieved.Deadline)
				return false
			}
			if retrieved.Status != event.Status {
				t.Logf("Status mismatch: expected %s, got %s", event.Status, retrieved.Status)
				return false
			}
			if retrieved.EventType != event.EventType {
				t.Logf("EventType mismatch: expected %s, got %s", event.EventType, retrieved.EventType)
				return false
			}
			if retrieved.CreatedBy != event.CreatedBy {
				t.Logf("CreatedBy mismatch: expected %d, got %d", event.CreatedBy, retrieved.CreatedBy)
				return false
			}
			if retrieved.PollID != event.PollID {
				t.Logf("PollID mismatch: expected %s, got %s", event.PollID, retrieved.PollID)
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 500 }),
		gen.IntRange(0, 10),
		gen.OneConstOf(domain.EventTypeBinary, domain.EventTypeMultiOption, domain.EventTypeProbability),
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
