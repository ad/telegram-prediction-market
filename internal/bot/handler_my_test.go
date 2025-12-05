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

func TestPersonalStatsCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("personal stats contain score, correct_count, wrong_count, streak, and achievements", prop.ForAll(
		func(userID int64, score int, correctCount int, wrongCount int, streak int, achievementCodes []domain.AchievementCode) bool {
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

			ratingRepo := storage.NewRatingRepository(queue)
			achievementRepo := storage.NewAchievementRepository(queue)
			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)
			logger := &mockLogger{}

			ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)
			achievementTracker := domain.NewAchievementTracker(achievementRepo, ratingRepo, predictionRepo, eventRepo, logger)

			ctx := context.Background()

			// Create rating
			rating := &domain.Rating{
				UserID:       userID,
				Score:        score,
				CorrectCount: correctCount,
				WrongCount:   wrongCount,
				Streak:       streak,
			}

			// Skip invalid ratings
			if err := rating.Validate(); err != nil {
				return true
			}

			if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
				t.Logf("Failed to create rating: %v", err)
				return false
			}

			// Create achievements (deduplicate codes)
			uniqueCodes := make(map[domain.AchievementCode]bool)
			for _, code := range achievementCodes {
				uniqueCodes[code] = true
			}

			expectedAchievementCount := 0
			for code := range uniqueCodes {
				achievement := &domain.Achievement{
					UserID: userID,
					Code:   code,
				}

				// Skip invalid achievements
				if err := achievement.Validate(); err != nil {
					continue
				}

				if err := achievementRepo.SaveAchievement(ctx, achievement); err != nil {
					t.Logf("Failed to save achievement: %v", err)
					continue
				}
				expectedAchievementCount++
			}

			// Get user rating
			retrievedRating, err := ratingCalc.GetUserRating(ctx, userID, 1)
			if err != nil {
				t.Logf("Failed to get user rating: %v", err)
				return false
			}

			// Verify rating fields
			if retrievedRating.UserID != userID {
				t.Logf("UserID mismatch: expected %d, got %d", userID, retrievedRating.UserID)
				return false
			}
			if retrievedRating.Score != score {
				t.Logf("Score mismatch: expected %d, got %d", score, retrievedRating.Score)
				return false
			}
			if retrievedRating.CorrectCount != correctCount {
				t.Logf("CorrectCount mismatch: expected %d, got %d", correctCount, retrievedRating.CorrectCount)
				return false
			}
			if retrievedRating.WrongCount != wrongCount {
				t.Logf("WrongCount mismatch: expected %d, got %d", wrongCount, retrievedRating.WrongCount)
				return false
			}
			if retrievedRating.Streak != streak {
				t.Logf("Streak mismatch: expected %d, got %d", streak, retrievedRating.Streak)
				return false
			}

			// Get user achievements
			retrievedAchievements, err := achievementTracker.GetUserAchievements(ctx, userID, 1)
			if err != nil {
				t.Logf("Failed to get user achievements: %v", err)
				return false
			}

			// Verify achievement count
			if len(retrievedAchievements) != expectedAchievementCount {
				t.Logf("Achievement count mismatch: expected %d, got %d", expectedAchievementCount, len(retrievedAchievements))
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.IntRange(-100, 1000),
		gen.IntRange(0, 100),
		gen.IntRange(0, 100),
		gen.IntRange(0, 50),
		gen.SliceOfN(5, gen.OneConstOf(
			domain.AchievementSharpshooter,
			domain.AchievementProphet,
			domain.AchievementRiskTaker,
			domain.AchievementWeeklyAnalyst,
			domain.AchievementVeteran,
		)),
	))

	properties.TestingRun(t)
}
