package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

func TestPredictionDataRoundTrip(t *testing.T) {
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

func TestResolvedEventsOnlyCounting(t *testing.T) {
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

	predictionRepo := NewPredictionRepository(queue)
	eventRepo := NewEventRepository(queue)

	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("participation count includes only resolved events", prop.ForAll(
		func(groupID int64, userID int64, resolvedCount int, activeCount int, cancelledCount int) bool {
			ctx := context.Background()

			// Create resolved events with predictions
			for i := 0; i < resolvedCount; i++ {
				event := &domain.Event{
					GroupID:   groupID,
					Question:  "Resolved question " + time.Now().Format(time.RFC3339Nano),
					Options:   []string{"Yes", "No"},
					CreatedAt: time.Now().Truncate(time.Second),
					Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
					Status:    domain.EventStatusResolved,
					EventType: domain.EventTypeBinary,
					CreatedBy: 1,
					PollID:    "poll_resolved_" + time.Now().Format(time.RFC3339Nano),
				}

				if err := eventRepo.CreateEvent(ctx, event); err != nil {
					t.Logf("Failed to create resolved event: %v", err)
					return false
				}

				prediction := &domain.Prediction{
					EventID:   event.ID,
					UserID:    userID,
					Option:    0,
					Timestamp: time.Now().Truncate(time.Second),
				}

				if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
					t.Logf("Failed to save prediction for resolved event: %v", err)
					return false
				}
			}

			// Create active events with predictions (should not be counted)
			for i := 0; i < activeCount; i++ {
				event := &domain.Event{
					GroupID:   groupID,
					Question:  "Active question " + time.Now().Format(time.RFC3339Nano),
					Options:   []string{"Yes", "No"},
					CreatedAt: time.Now().Truncate(time.Second),
					Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
					Status:    domain.EventStatusActive,
					EventType: domain.EventTypeBinary,
					CreatedBy: 1,
					PollID:    "poll_active_" + time.Now().Format(time.RFC3339Nano),
				}

				if err := eventRepo.CreateEvent(ctx, event); err != nil {
					t.Logf("Failed to create active event: %v", err)
					return false
				}

				prediction := &domain.Prediction{
					EventID:   event.ID,
					UserID:    userID,
					Option:    0,
					Timestamp: time.Now().Truncate(time.Second),
				}

				if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
					t.Logf("Failed to save prediction for active event: %v", err)
					return false
				}
			}

			// Create cancelled events with predictions (should not be counted)
			for i := 0; i < cancelledCount; i++ {
				event := &domain.Event{
					GroupID:   groupID,
					Question:  "Cancelled question " + time.Now().Format(time.RFC3339Nano),
					Options:   []string{"Yes", "No"},
					CreatedAt: time.Now().Truncate(time.Second),
					Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
					Status:    domain.EventStatusCancelled,
					EventType: domain.EventTypeBinary,
					CreatedBy: 1,
					PollID:    "poll_cancelled_" + time.Now().Format(time.RFC3339Nano),
				}

				if err := eventRepo.CreateEvent(ctx, event); err != nil {
					t.Logf("Failed to create cancelled event: %v", err)
					return false
				}

				prediction := &domain.Prediction{
					EventID:   event.ID,
					UserID:    userID,
					Option:    0,
					Timestamp: time.Now().Truncate(time.Second),
				}

				if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
					t.Logf("Failed to save prediction for cancelled event: %v", err)
					return false
				}
			}

			// Get the count of completed events
			count, err := predictionRepo.GetUserCompletedEventCount(ctx, userID, groupID)
			if err != nil {
				t.Logf("Failed to get completed event count: %v", err)
				return false
			}

			// Verify count matches only resolved events
			if count != resolvedCount {
				t.Logf("Count mismatch: expected %d resolved events, got %d", resolvedCount, count)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.Int64Range(1, 1000000),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}

func TestGetUserCompletedEventCount(t *testing.T) {
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

	predictionRepo := NewPredictionRepository(queue)
	eventRepo := NewEventRepository(queue)
	ctx := context.Background()

	t.Run("returns zero when user has no predictions", func(t *testing.T) {
		count, err := predictionRepo.GetUserCompletedEventCount(ctx, 999, 1)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})

	t.Run("counts only resolved events", func(t *testing.T) {
		userID := int64(100)
		groupID := int64(1)

		// Create resolved event with prediction
		resolvedEvent := &domain.Event{
			GroupID:   groupID,
			Question:  "Resolved question",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now().Truncate(time.Second),
			Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
			Status:    domain.EventStatusResolved,
			EventType: domain.EventTypeBinary,
			CreatedBy: 1,
			PollID:    "poll_resolved",
		}
		if err := eventRepo.CreateEvent(ctx, resolvedEvent); err != nil {
			t.Fatalf("Failed to create resolved event: %v", err)
		}
		if err := predictionRepo.SavePrediction(ctx, &domain.Prediction{
			EventID:   resolvedEvent.ID,
			UserID:    userID,
			Option:    0,
			Timestamp: time.Now().Truncate(time.Second),
		}); err != nil {
			t.Fatalf("Failed to save prediction: %v", err)
		}

		// Create active event with prediction
		activeEvent := &domain.Event{
			GroupID:   groupID,
			Question:  "Active question",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now().Truncate(time.Second),
			Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
			Status:    domain.EventStatusActive,
			EventType: domain.EventTypeBinary,
			CreatedBy: 1,
			PollID:    "poll_active",
		}
		if err := eventRepo.CreateEvent(ctx, activeEvent); err != nil {
			t.Fatalf("Failed to create active event: %v", err)
		}
		if err := predictionRepo.SavePrediction(ctx, &domain.Prediction{
			EventID:   activeEvent.ID,
			UserID:    userID,
			Option:    0,
			Timestamp: time.Now().Truncate(time.Second),
		}); err != nil {
			t.Fatalf("Failed to save prediction: %v", err)
		}

		// Create cancelled event with prediction
		cancelledEvent := &domain.Event{
			GroupID:   groupID,
			Question:  "Cancelled question",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now().Truncate(time.Second),
			Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
			Status:    domain.EventStatusCancelled,
			EventType: domain.EventTypeBinary,
			CreatedBy: 1,
			PollID:    "poll_cancelled",
		}
		if err := eventRepo.CreateEvent(ctx, cancelledEvent); err != nil {
			t.Fatalf("Failed to create cancelled event: %v", err)
		}
		if err := predictionRepo.SavePrediction(ctx, &domain.Prediction{
			EventID:   cancelledEvent.ID,
			UserID:    userID,
			Option:    0,
			Timestamp: time.Now().Truncate(time.Second),
		}); err != nil {
			t.Fatalf("Failed to save prediction: %v", err)
		}

		count, err := predictionRepo.GetUserCompletedEventCount(ctx, userID, groupID)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected count 1 (only resolved), got %d", count)
		}
	})

	t.Run("counts correctly for multiple users", func(t *testing.T) {
		user1 := int64(200)
		user2 := int64(201)
		groupID := int64(1)

		// Create resolved event with predictions from both users
		event := &domain.Event{
			GroupID:   groupID,
			Question:  "Multi-user question",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now().Truncate(time.Second),
			Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
			Status:    domain.EventStatusResolved,
			EventType: domain.EventTypeBinary,
			CreatedBy: 1,
			PollID:    "poll_multi",
		}
		if err := eventRepo.CreateEvent(ctx, event); err != nil {
			t.Fatalf("Failed to create event: %v", err)
		}

		if err := predictionRepo.SavePrediction(ctx, &domain.Prediction{
			EventID:   event.ID,
			UserID:    user1,
			Option:    0,
			Timestamp: time.Now().Truncate(time.Second),
		}); err != nil {
			t.Fatalf("Failed to save prediction for user1: %v", err)
		}

		if err := predictionRepo.SavePrediction(ctx, &domain.Prediction{
			EventID:   event.ID,
			UserID:    user2,
			Option:    1,
			Timestamp: time.Now().Truncate(time.Second),
		}); err != nil {
			t.Fatalf("Failed to save prediction for user2: %v", err)
		}

		count1, err := predictionRepo.GetUserCompletedEventCount(ctx, user1, groupID)
		if err != nil {
			t.Fatalf("Failed to get count for user1: %v", err)
		}
		if count1 != 1 {
			t.Errorf("Expected count 1 for user1, got %d", count1)
		}

		count2, err := predictionRepo.GetUserCompletedEventCount(ctx, user2, groupID)
		if err != nil {
			t.Fatalf("Failed to get count for user2: %v", err)
		}
		if count2 != 1 {
			t.Errorf("Expected count 1 for user2, got %d", count2)
		}
	})
}
