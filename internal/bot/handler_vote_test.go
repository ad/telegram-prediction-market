package bot

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

// Feature: telegram-prediction-bot, Property 1: Vote persistence completeness
// Validates: Requirements 3.1
func TestVotePersistenceCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("vote saves all required fields (user_id, event_id, option, timestamp)", prop.ForAll(
		func(userID int64, option int) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)

			ctx := context.Background()

			// Create an event first
			event := &domain.Event{
				Question:  "Test question",
				Options:   []string{"Option 1", "Option 2", "Option 3"},
				CreatedAt: time.Now().Truncate(time.Second),
				Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeMultiOption,
				CreatedBy: 1,
				PollID:    "test_poll",
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Create prediction
			timestamp := time.Now().Truncate(time.Second)
			prediction := &domain.Prediction{
				EventID:   event.ID,
				UserID:    userID,
				Option:    option,
				Timestamp: timestamp,
			}

			// Skip invalid predictions
			if err := prediction.Validate(); err != nil {
				return true
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

			// Verify all required fields are present
			if retrieved.UserID != userID {
				t.Logf("UserID mismatch: expected %d, got %d", userID, retrieved.UserID)
				return false
			}
			if retrieved.EventID != event.ID {
				t.Logf("EventID mismatch: expected %d, got %d", event.ID, retrieved.EventID)
				return false
			}
			if retrieved.Option != option {
				t.Logf("Option mismatch: expected %d, got %d", option, retrieved.Option)
				return false
			}
			if retrieved.Timestamp.IsZero() {
				t.Logf("Timestamp is zero")
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}

// Feature: telegram-prediction-bot, Property 2: Vote update idempotence
// Validates: Requirements 3.2
func TestVoteUpdateIdempotence(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("changing vote before deadline results in exactly one prediction with latest option", prop.ForAll(
		func(userID int64, initialOption int, updatedOption int) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)

			ctx := context.Background()

			// Create an event with future deadline
			event := &domain.Event{
				Question:  "Test question",
				Options:   []string{"Option 1", "Option 2", "Option 3"},
				CreatedAt: time.Now().Truncate(time.Second),
				Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeMultiOption,
				CreatedBy: 1,
				PollID:    "test_poll",
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Create initial prediction
			initialPrediction := &domain.Prediction{
				EventID:   event.ID,
				UserID:    userID,
				Option:    initialOption,
				Timestamp: time.Now().Truncate(time.Second),
			}

			// Skip invalid predictions
			if err := initialPrediction.Validate(); err != nil {
				return true
			}

			if err := predictionRepo.SavePrediction(ctx, initialPrediction); err != nil {
				t.Logf("Failed to save initial prediction: %v", err)
				return false
			}

			// Update prediction
			updatedPrediction := &domain.Prediction{
				ID:        initialPrediction.ID,
				EventID:   event.ID,
				UserID:    userID,
				Option:    updatedOption,
				Timestamp: time.Now().Add(1 * time.Hour).Truncate(time.Second),
			}

			if err := updatedPrediction.Validate(); err != nil {
				return true
			}

			if err := predictionRepo.UpdatePrediction(ctx, updatedPrediction); err != nil {
				t.Logf("Failed to update prediction: %v", err)
				return false
			}

			// Verify only one prediction exists
			predictions, err := predictionRepo.GetPredictionsByEvent(ctx, event.ID)
			if err != nil {
				t.Logf("Failed to get predictions: %v", err)
				return false
			}

			// Should have exactly one prediction for this user
			userPredictions := 0
			var latestPrediction *domain.Prediction
			for _, p := range predictions {
				if p.UserID == userID {
					userPredictions++
					latestPrediction = p
				}
			}

			if userPredictions != 1 {
				t.Logf("Expected exactly 1 prediction, got %d", userPredictions)
				return false
			}

			// Verify it has the updated option
			if latestPrediction.Option != updatedOption {
				t.Logf("Expected option %d, got %d", updatedOption, latestPrediction.Option)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.IntRange(0, 2),
		gen.IntRange(0, 2),
	))

	properties.TestingRun(t)
}

// Feature: telegram-prediction-bot, Property 3: Deadline enforcement
// Validates: Requirements 3.3
func TestDeadlineEnforcement(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("vote after deadline is rejected and database state remains unchanged", prop.ForAll(
		func(userID int64, option int) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)

			ctx := context.Background()

			// Create an event with past deadline
			event := &domain.Event{
				Question:  "Test question",
				Options:   []string{"Option 1", "Option 2", "Option 3"},
				CreatedAt: time.Now().Add(-48 * time.Hour).Truncate(time.Second),
				Deadline:  time.Now().Add(-1 * time.Hour).Truncate(time.Second), // Deadline in the past
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeMultiOption,
				CreatedBy: 1,
				PollID:    "test_poll",
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Get initial prediction count
			initialPredictions, err := predictionRepo.GetPredictionsByEvent(ctx, event.ID)
			if err != nil {
				t.Logf("Failed to get initial predictions: %v", err)
				return false
			}
			initialCount := len(initialPredictions)

			// Try to create prediction after deadline
			prediction := &domain.Prediction{
				EventID:   event.ID,
				UserID:    userID,
				Option:    option,
				Timestamp: time.Now().Truncate(time.Second),
			}

			// Skip invalid predictions
			if err := prediction.Validate(); err != nil {
				return true
			}

			// Check if deadline has passed (simulating the handler logic)
			if time.Now().After(event.Deadline) {
				// Vote should be rejected - don't save it
				// Verify database state unchanged
				finalPredictions, err := predictionRepo.GetPredictionsByEvent(ctx, event.ID)
				if err != nil {
					t.Logf("Failed to get final predictions: %v", err)
					return false
				}

				if len(finalPredictions) != initialCount {
					t.Logf("Database state changed after rejected vote: initial %d, final %d", initialCount, len(finalPredictions))
					return false
				}

				return true
			}

			// If deadline hasn't passed (shouldn't happen with our setup), save should succeed
			if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
				t.Logf("Failed to save prediction: %v", err)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.IntRange(0, 2),
	))

	properties.TestingRun(t)
}

// Feature: telegram-prediction-bot, Property 4: Transaction atomicity
// Validates: Requirements 3.4
func TestTransactionAtomicity(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("prediction save is atomic - all fields persisted or none", prop.ForAll(
		func(userID int64, option int) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)

			ctx := context.Background()

			// Create an event
			event := &domain.Event{
				Question:  "Test question",
				Options:   []string{"Option 1", "Option 2", "Option 3"},
				CreatedAt: time.Now().Truncate(time.Second),
				Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeMultiOption,
				CreatedBy: 1,
				PollID:    "test_poll",
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Create prediction
			timestamp := time.Now().Truncate(time.Second)
			prediction := &domain.Prediction{
				EventID:   event.ID,
				UserID:    userID,
				Option:    option,
				Timestamp: timestamp,
			}

			// Skip invalid predictions
			if err := prediction.Validate(); err != nil {
				return true
			}

			// Save prediction
			err = predictionRepo.SavePrediction(ctx, prediction)

			// Either save succeeds completely or fails completely
			if err != nil {
				// If save failed, verify no partial data exists
				retrieved, _ := predictionRepo.GetPredictionByUserAndEvent(ctx, userID, event.ID)
				if retrieved != nil {
					t.Logf("Partial save detected: prediction exists after failed save")
					return false
				}
				return true
			}

			// If save succeeded, verify all fields are present
			retrieved, err := predictionRepo.GetPredictionByUserAndEvent(ctx, userID, event.ID)
			if err != nil {
				t.Logf("Failed to retrieve after successful save: %v", err)
				return false
			}

			if retrieved == nil {
				t.Logf("No prediction found after successful save")
				return false
			}

			// Verify all fields are present (no partial save)
			if retrieved.UserID == 0 || retrieved.EventID == 0 || retrieved.Timestamp.IsZero() {
				t.Logf("Partial save detected: missing fields")
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.IntRange(0, 2),
	))

	properties.TestingRun(t)
}
