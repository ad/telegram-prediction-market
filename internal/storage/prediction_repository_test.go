package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"telegram-prediction-bot/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

// Feature: telegram-prediction-bot, Property 16: Prediction data round-trip
// Validates: Requirements 8.3
func TestPredictionDataRoundTrip(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	predictionRepo := NewPredictionRepository(queue)
	eventRepo := NewEventRepository(queue)

	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("prediction round-trip preserves all fields", prop.ForAll(
		func(userID int64, option int, timestampOffset int64) bool {
			ctx := context.Background()

			// First create an event (required for foreign key)
			event := &domain.Event{
				Question:  "Test question",
				Options:   []string{"Option 1", "Option 2"},
				CreatedAt: time.Now().Truncate(time.Second),
				Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: 1,
				PollID:    "test_poll",
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Create prediction with valid data
			timestamp := time.Now().Add(time.Duration(timestampOffset) * time.Second).Truncate(time.Second)

			prediction := &domain.Prediction{
				EventID:   event.ID,
				UserID:    userID,
				Option:    option,
				Timestamp: timestamp,
			}

			// Skip invalid predictions
			if err := prediction.Validate(); err != nil {
				return true // Skip invalid inputs
			}

			// Save prediction
			if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
				t.Logf("Failed to save prediction: %v", err)
				return false
			}

			// Retrieve prediction
			retrieved, err := predictionRepo.GetPredictionByUserAndEvent(ctx, userID, event.ID)
			if err != nil {
				t.Logf("Failed to get prediction: %v", err)
				return false
			}

			if retrieved == nil {
				t.Logf("Retrieved prediction is nil")
				return false
			}

			// Verify all fields match
			if retrieved.ID != prediction.ID {
				t.Logf("ID mismatch: expected %d, got %d", prediction.ID, retrieved.ID)
				return false
			}
			if retrieved.EventID != prediction.EventID {
				t.Logf("EventID mismatch: expected %d, got %d", prediction.EventID, retrieved.EventID)
				return false
			}
			if retrieved.UserID != prediction.UserID {
				t.Logf("UserID mismatch: expected %d, got %d", prediction.UserID, retrieved.UserID)
				return false
			}
			if retrieved.Option != prediction.Option {
				t.Logf("Option mismatch: expected %d, got %d", prediction.Option, retrieved.Option)
				return false
			}
			if !retrieved.Timestamp.Equal(prediction.Timestamp) {
				t.Logf("Timestamp mismatch: expected %v, got %v", prediction.Timestamp, retrieved.Timestamp)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.IntRange(0, 5),
		gen.Int64Range(-3600, 3600),
	))

	properties.TestingRun(t)
}
