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

func TestAchievementDataRoundTrip(t *testing.T) {
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

	repo := NewAchievementRepository(queue)

	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("achievement round-trip preserves all fields", prop.ForAll(
		func(userID int64, groupID int64, code domain.AchievementCode, timestampOffset int64) bool {
			ctx := context.Background()

			// Create achievement with valid data
			timestamp := time.Now().Add(time.Duration(timestampOffset) * time.Second).Truncate(time.Second)

			achievement := &domain.Achievement{
				UserID:    userID,
				GroupID:   groupID,
				Code:      code,
				Timestamp: timestamp,
			}

			// Skip invalid achievements
			if err := achievement.Validate(); err != nil {
				return true // Skip invalid inputs
			}

			// Save achievement
			if err := repo.SaveAchievement(ctx, achievement); err != nil {
				t.Logf("Failed to save achievement: %v", err)
				return false
			}

			// Retrieve achievements for user
			retrieved, err := repo.GetUserAchievements(ctx, userID, groupID)
			if err != nil {
				t.Logf("Failed to get achievements: %v", err)
				return false
			}

			if len(retrieved) == 0 {
				t.Logf("No achievements retrieved")
				return false
			}

			// Find the achievement we just saved
			var found *domain.Achievement
			for _, a := range retrieved {
				if a.ID == achievement.ID {
					found = a
					break
				}
			}

			if found == nil {
				t.Logf("Achievement not found in retrieved list")
				return false
			}

			// Verify all fields match
			if found.ID != achievement.ID {
				t.Logf("ID mismatch: expected %d, got %d", achievement.ID, found.ID)
				return false
			}
			if found.UserID != achievement.UserID {
				t.Logf("UserID mismatch: expected %d, got %d", achievement.UserID, found.UserID)
				return false
			}
			if found.Code != achievement.Code {
				t.Logf("Code mismatch: expected %s, got %s", achievement.Code, found.Code)
				return false
			}
			if !found.Timestamp.Equal(achievement.Timestamp) {
				t.Logf("Timestamp mismatch: expected %v, got %v", achievement.Timestamp, found.Timestamp)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.Int64Range(1, 1000),
		gen.OneConstOf(
			domain.AchievementSharpshooter,
			domain.AchievementWeeklyAnalyst,
			domain.AchievementProphet,
			domain.AchievementRiskTaker,
			domain.AchievementVeteran,
		),
		gen.Int64Range(-3600, 3600),
	))

	properties.TestingRun(t)
}
