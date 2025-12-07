package bot

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

// mockLogger implements domain.Logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...interface{}) {}
func (m *mockLogger) Info(msg string, args ...interface{})  {}
func (m *mockLogger) Warn(msg string, args ...interface{})  {}
func (m *mockLogger) Error(msg string, args ...interface{}) {}

func TestRatingDisplayCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("rating display contains exactly top 10 participants ordered by score", prop.ForAll(
		func(ratingCount int) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer func() { _ = db.Close() }()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			// Run migrations
			if err := storage.RunMigrations(queue); err != nil {
				t.Fatalf("Failed to run migrations: %v", err)
			}

			ratingRepo := storage.NewRatingRepository(queue)
			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)
			logger := &mockLogger{}
			ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

			ctx := context.Background()
			groupID := int64(1)

			// Create ratings (between 1 and 25 to test both < 10 and > 10 cases)
			actualCount := 1 + (ratingCount % 25)
			expectedCount := actualCount
			if expectedCount > 10 {
				expectedCount = 10
			}

			// Create ratings with different scores
			for i := 0; i < actualCount; i++ {
				rating := &domain.Rating{
					UserID:       int64(i + 1),
					Score:        100 - i, // Descending scores
					CorrectCount: 10,
					WrongCount:   5,
					Streak:       2,
					GroupID:      groupID,
				}
				if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
					t.Logf("Failed to create rating: %v", err)
					return false
				}
			}

			// Get top ratings
			topRatings, err := ratingCalc.GetTopRatings(ctx, groupID, 10)
			if err != nil {
				t.Logf("Failed to get top ratings: %v", err)
				return false
			}

			// Verify count
			if len(topRatings) != expectedCount {
				t.Logf("Expected %d ratings, got %d", expectedCount, len(topRatings))
				return false
			}

			return true
		},
		gen.IntRange(0, 25),
	))

	properties.TestingRun(t)
}

func TestRatingOrderingConsistency(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("ratings are ordered by score in descending order", prop.ForAll(
		func(scores []int) bool {
			// Need at least 2 ratings to test ordering
			if len(scores) < 2 {
				return true
			}

			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer func() { _ = db.Close() }()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			// Run migrations
			if err := storage.RunMigrations(queue); err != nil {
				t.Fatalf("Failed to run migrations: %v", err)
			}

			ratingRepo := storage.NewRatingRepository(queue)
			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)
			logger := &mockLogger{}
			ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

			ctx := context.Background()
			groupID := int64(1)

			// Create ratings with random scores
			for i, score := range scores {
				rating := &domain.Rating{
					UserID:       int64(i + 1),
					Score:        score,
					CorrectCount: 10,
					WrongCount:   5,
					Streak:       2,
					GroupID:      groupID,
				}
				if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
					t.Logf("Failed to create rating: %v", err)
					return false
				}
			}

			// Get top ratings
			topRatings, err := ratingCalc.GetTopRatings(ctx, groupID, 10)
			if err != nil {
				t.Logf("Failed to get top ratings: %v", err)
				return false
			}

			// Verify ordering (descending by score)
			for i := 1; i < len(topRatings); i++ {
				if topRatings[i].Score > topRatings[i-1].Score {
					t.Logf("Ratings not in descending order: position %d has score %d, position %d has score %d",
						i-1, topRatings[i-1].Score, i, topRatings[i].Score)
					return false
				}
			}

			return true
		},
		gen.SliceOfN(20, gen.IntRange(-100, 1000)),
	))

	properties.TestingRun(t)
}
