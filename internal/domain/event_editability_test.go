package domain_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"telegram-prediction-bot/internal/domain"
	"telegram-prediction-bot/internal/storage"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

// mockLogger implements the domain.Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Info(msg string, args ...interface{})  {}
func (m *mockLogger) Error(msg string, args ...interface{}) {}
func (m *mockLogger) Debug(msg string, args ...interface{}) {}
func (m *mockLogger) Warn(msg string, args ...interface{})  {}

// setupTestDB creates an in-memory database with schema for testing
func setupTestDB(t *testing.T) (*storage.DBQueue, *domain.EventManager) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	queue := storage.NewDBQueue(db)

	// Initialize schema
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	logger := &mockLogger{}

	manager := domain.NewEventManager(eventRepo, predictionRepo, logger)

	return queue, manager
}

func TestEventEditabilityWithoutVotes(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("events without votes can be edited", prop.ForAll(
		func(question string, createdBy int64) bool {
			queue, manager := setupTestDB(t)
			defer queue.Close()

			ctx := context.Background()

			// Create an event
			now := time.Now().Truncate(time.Second)
			event := &domain.Event{
				Question:  question,
				Options:   []string{"Yes", "No"},
				CreatedAt: now,
				Deadline:  now.Add(24 * time.Hour),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: createdBy,
				PollID:    "test_poll_" + question[:min(5, len(question))],
			}

			// Skip invalid events
			if err := event.Validate(); err != nil {
				return true
			}

			// Create the event
			if err := manager.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Check if event can be edited (should be true - no votes)
			canEdit, err := manager.CanEditEvent(ctx, event.ID)
			if err != nil {
				t.Logf("Failed to check if event can be edited: %v", err)
				return false
			}

			if !canEdit {
				t.Logf("Event without votes should be editable, but CanEditEvent returned false")
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 200 }),
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

func TestEventImmutabilityWithVotes(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("events with votes cannot be edited", prop.ForAll(
		func(question string, createdBy int64, userID int64, option int) bool {
			queue, manager := setupTestDB(t)
			defer queue.Close()

			ctx := context.Background()

			// Create an event
			now := time.Now().Truncate(time.Second)
			event := &domain.Event{
				Question:  question,
				Options:   []string{"Yes", "No"},
				CreatedAt: now,
				Deadline:  now.Add(24 * time.Hour),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: createdBy,
				PollID:    "test_poll_" + question[:min(5, len(question))],
			}

			// Skip invalid events
			if err := event.Validate(); err != nil {
				return true
			}

			// Create the event
			if err := manager.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Create a prediction (vote) for this event
			prediction := &domain.Prediction{
				EventID:   event.ID,
				UserID:    userID,
				Option:    option % 2, // Ensure option is 0 or 1 for binary event
				Timestamp: now,
			}

			// Skip invalid predictions
			if err := prediction.Validate(); err != nil {
				return true
			}

			// Get prediction repository to save the vote
			predictionRepo := storage.NewPredictionRepository(queue)
			if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
				t.Logf("Failed to save prediction: %v", err)
				return false
			}

			// Check if event can be edited (should be false - has votes)
			canEdit, err := manager.CanEditEvent(ctx, event.ID)
			if err != nil {
				t.Logf("Failed to check if event can be edited: %v", err)
				return false
			}

			if canEdit {
				t.Logf("Event with votes should not be editable, but CanEditEvent returned true")
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 200 }),
		gen.Int64Range(1, 1000000),
		gen.Int64Range(1, 1000000),
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
