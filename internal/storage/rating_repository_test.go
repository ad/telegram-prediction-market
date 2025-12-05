package storage

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

func TestRatingDataRoundTrip(t *testing.T) {
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

	repo := NewRatingRepository(queue)

	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("rating round-trip preserves all fields", prop.ForAll(
		func(groupID int64, userID int64, score int, correctCount int, wrongCount int, streak int) bool {
			ctx := context.Background()

			// Create rating with valid data
			rating := &domain.Rating{
				UserID:       userID,
				GroupID:      groupID,
				Score:        score,
				CorrectCount: correctCount,
				WrongCount:   wrongCount,
				Streak:       streak,
			}

			// Skip invalid ratings
			if err := rating.Validate(); err != nil {
				return true // Skip invalid inputs
			}

			// Save rating
			if err := repo.UpdateRating(ctx, rating); err != nil {
				t.Logf("Failed to update rating: %v", err)
				return false
			}

			// Retrieve rating
			retrieved, err := repo.GetRating(ctx, userID, groupID)
			if err != nil {
				t.Logf("Failed to get rating: %v", err)
				return false
			}

			if retrieved == nil {
				t.Logf("Retrieved rating is nil")
				return false
			}

			// Verify all fields match
			if retrieved.UserID != rating.UserID {
				t.Logf("UserID mismatch: expected %d, got %d", rating.UserID, retrieved.UserID)
				return false
			}
			if retrieved.Score != rating.Score {
				t.Logf("Score mismatch: expected %d, got %d", rating.Score, retrieved.Score)
				return false
			}
			if retrieved.CorrectCount != rating.CorrectCount {
				t.Logf("CorrectCount mismatch: expected %d, got %d", rating.CorrectCount, retrieved.CorrectCount)
				return false
			}
			if retrieved.WrongCount != rating.WrongCount {
				t.Logf("WrongCount mismatch: expected %d, got %d", rating.WrongCount, retrieved.WrongCount)
				return false
			}
			if retrieved.Streak != rating.Streak {
				t.Logf("Streak mismatch: expected %d, got %d", rating.Streak, retrieved.Streak)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.Int64Range(1, 1000000),
		gen.IntRange(-1000, 1000),
		gen.IntRange(0, 100),
		gen.IntRange(0, 100),
		gen.IntRange(-10, 50),
	))

	properties.TestingRun(t)
}
