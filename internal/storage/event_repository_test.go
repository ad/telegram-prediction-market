package storage

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

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

func TestCreatorEventCounting(t *testing.T) {
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
	properties.Property("count includes only events created by user", prop.ForAll(
		func(targetUserID int64, otherUserID int64, targetUserEvents int, otherUserEvents int) bool {
			// Ensure users are different
			if targetUserID == otherUserID {
				return true // Skip when users are the same
			}

			ctx := context.Background()

			// Create events for target user
			for i := 0; i < targetUserEvents; i++ {
				event := &domain.Event{
					Question:  fmt.Sprintf("Target user question %d %s", i, time.Now().Format(time.RFC3339Nano)),
					Options:   []string{"Yes", "No"},
					CreatedAt: time.Now().Truncate(time.Second),
					Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
					Status:    domain.EventStatusActive,
					EventType: domain.EventTypeBinary,
					CreatedBy: targetUserID,
					PollID:    fmt.Sprintf("poll_target_%d_%s", i, time.Now().Format(time.RFC3339Nano)),
				}

				if err := repo.CreateEvent(ctx, event); err != nil {
					t.Logf("Failed to create event for target user: %v", err)
					return false
				}
			}

			// Create events for other user (should not be counted)
			for i := 0; i < otherUserEvents; i++ {
				event := &domain.Event{
					Question:  fmt.Sprintf("Other user question %d %s", i, time.Now().Format(time.RFC3339Nano)),
					Options:   []string{"Yes", "No"},
					CreatedAt: time.Now().Truncate(time.Second),
					Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
					Status:    domain.EventStatusActive,
					EventType: domain.EventTypeBinary,
					CreatedBy: otherUserID,
					PollID:    fmt.Sprintf("poll_other_%d_%s", i, time.Now().Format(time.RFC3339Nano)),
				}

				if err := repo.CreateEvent(ctx, event); err != nil {
					t.Logf("Failed to create event for other user: %v", err)
					return false
				}
			}

			// Get the count of events created by target user
			count, err := repo.GetUserCreatedEventsCount(ctx, targetUserID)
			if err != nil {
				t.Logf("Failed to get created events count: %v", err)
				return false
			}

			// Verify count matches only target user's events
			if count != targetUserEvents {
				t.Logf("Count mismatch: expected %d events for user %d, got %d", targetUserEvents, targetUserID, count)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.Int64Range(1000001, 2000000),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}

func TestGetUserCreatedEventsCount(t *testing.T) {
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
	ctx := context.Background()

	t.Run("returns zero when user has created no events", func(t *testing.T) {
		count, err := repo.GetUserCreatedEventsCount(ctx, 999)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})

	t.Run("counts events with various statuses", func(t *testing.T) {
		userID := int64(300)

		// Create resolved event
		resolvedEvent := &domain.Event{
			Question:  "Resolved by user",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now().Truncate(time.Second),
			Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
			Status:    domain.EventStatusResolved,
			EventType: domain.EventTypeBinary,
			CreatedBy: userID,
			PollID:    "poll_user_resolved",
		}
		if err := repo.CreateEvent(ctx, resolvedEvent); err != nil {
			t.Fatalf("Failed to create resolved event: %v", err)
		}

		// Create active event
		activeEvent := &domain.Event{
			Question:  "Active by user",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now().Truncate(time.Second),
			Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
			Status:    domain.EventStatusActive,
			EventType: domain.EventTypeBinary,
			CreatedBy: userID,
			PollID:    "poll_user_active",
		}
		if err := repo.CreateEvent(ctx, activeEvent); err != nil {
			t.Fatalf("Failed to create active event: %v", err)
		}

		// Create cancelled event
		cancelledEvent := &domain.Event{
			Question:  "Cancelled by user",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now().Truncate(time.Second),
			Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
			Status:    domain.EventStatusCancelled,
			EventType: domain.EventTypeBinary,
			CreatedBy: userID,
			PollID:    "poll_user_cancelled",
		}
		if err := repo.CreateEvent(ctx, cancelledEvent); err != nil {
			t.Fatalf("Failed to create cancelled event: %v", err)
		}

		count, err := repo.GetUserCreatedEventsCount(ctx, userID)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected count 3 (all statuses), got %d", count)
		}
	})

	t.Run("counts correctly for multiple users", func(t *testing.T) {
		user1 := int64(400)
		user2 := int64(401)

		// Create 2 events for user1
		for i := 0; i < 2; i++ {
			event := &domain.Event{
				Question:  fmt.Sprintf("User1 question %d", i),
				Options:   []string{"Yes", "No"},
				CreatedAt: time.Now().Truncate(time.Second),
				Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: user1,
				PollID:    fmt.Sprintf("poll_user1_%d", i),
			}
			if err := repo.CreateEvent(ctx, event); err != nil {
				t.Fatalf("Failed to create event for user1: %v", err)
			}
		}

		// Create 3 events for user2
		for i := 0; i < 3; i++ {
			event := &domain.Event{
				Question:  fmt.Sprintf("User2 question %d", i),
				Options:   []string{"Yes", "No"},
				CreatedAt: time.Now().Truncate(time.Second),
				Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: user2,
				PollID:    fmt.Sprintf("poll_user2_%d", i),
			}
			if err := repo.CreateEvent(ctx, event); err != nil {
				t.Fatalf("Failed to create event for user2: %v", err)
			}
		}

		count1, err := repo.GetUserCreatedEventsCount(ctx, user1)
		if err != nil {
			t.Fatalf("Failed to get count for user1: %v", err)
		}
		if count1 != 2 {
			t.Errorf("Expected count 2 for user1, got %d", count1)
		}

		count2, err := repo.GetUserCreatedEventsCount(ctx, user2)
		if err != nil {
			t.Fatalf("Failed to get count for user2: %v", err)
		}
		if count2 != 3 {
			t.Errorf("Expected count 3 for user2, got %d", count2)
		}
	})
}
