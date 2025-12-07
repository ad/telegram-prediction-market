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

// TestDataIsolation tests Property 4: Data Isolation
// For any two different groups, querying events, ratings, or achievements for one group
// should never return data associated with the other group.
func TestDataIsolation(t *testing.T) {
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

	// Run migrations to add group_id columns and forum_topic_id
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	eventRepo := NewEventRepository(queue)
	ratingRepo := NewRatingRepository(queue)
	achievementRepo := NewAchievementRepository(queue)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("data isolation between groups", prop.ForAll(
		func(group1ID int64, group2ID int64, userID int64, eventCount int, ratingScore int) bool {
			// Ensure groups are different
			if group1ID == group2ID {
				return true // Skip when groups are the same
			}

			ctx := context.Background()

			// Create events for group1
			group1EventIDs := make([]int64, 0)
			for i := 0; i < eventCount; i++ {
				event := &domain.Event{
					GroupID:   group1ID,
					Question:  fmt.Sprintf("Group1 question %d %s", i, time.Now().Format(time.RFC3339Nano)),
					Options:   []string{"Yes", "No"},
					CreatedAt: time.Now().Truncate(time.Second),
					Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
					Status:    domain.EventStatusActive,
					EventType: domain.EventTypeBinary,
					CreatedBy: userID,
					PollID:    fmt.Sprintf("poll_g1_%d_%s", i, time.Now().Format(time.RFC3339Nano)),
				}

				if err := eventRepo.CreateEvent(ctx, event); err != nil {
					t.Logf("Failed to create event for group1: %v", err)
					return false
				}
				group1EventIDs = append(group1EventIDs, event.ID)
			}

			// Create events for group2
			group2EventIDs := make([]int64, 0)
			for i := 0; i < eventCount; i++ {
				event := &domain.Event{
					GroupID:   group2ID,
					Question:  fmt.Sprintf("Group2 question %d %s", i, time.Now().Format(time.RFC3339Nano)),
					Options:   []string{"Yes", "No"},
					CreatedAt: time.Now().Truncate(time.Second),
					Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
					Status:    domain.EventStatusActive,
					EventType: domain.EventTypeBinary,
					CreatedBy: userID,
					PollID:    fmt.Sprintf("poll_g2_%d_%s", i, time.Now().Format(time.RFC3339Nano)),
				}

				if err := eventRepo.CreateEvent(ctx, event); err != nil {
					t.Logf("Failed to create event for group2: %v", err)
					return false
				}
				group2EventIDs = append(group2EventIDs, event.ID)
			}

			// Test event isolation: Get active events for group1
			group1Events, err := eventRepo.GetActiveEvents(ctx, group1ID)
			if err != nil {
				t.Logf("Failed to get events for group1: %v", err)
				return false
			}

			// Verify all returned events belong to group1
			for _, event := range group1Events {
				if event.GroupID != group1ID {
					t.Logf("Event isolation violated: event %d has group_id %d, expected %d", event.ID, event.GroupID, group1ID)
					return false
				}
				// Verify this event is not from group2
				for _, g2ID := range group2EventIDs {
					if event.ID == g2ID {
						t.Logf("Event isolation violated: group1 query returned group2 event %d", event.ID)
						return false
					}
				}
			}

			// Test event isolation: Get active events for group2
			group2Events, err := eventRepo.GetActiveEvents(ctx, group2ID)
			if err != nil {
				t.Logf("Failed to get events for group2: %v", err)
				return false
			}

			// Verify all returned events belong to group2
			for _, event := range group2Events {
				if event.GroupID != group2ID {
					t.Logf("Event isolation violated: event %d has group_id %d, expected %d", event.ID, event.GroupID, group2ID)
					return false
				}
				// Verify this event is not from group1
				for _, g1ID := range group1EventIDs {
					if event.ID == g1ID {
						t.Logf("Event isolation violated: group2 query returned group1 event %d", event.ID)
						return false
					}
				}
			}

			// Create ratings for both groups
			rating1 := &domain.Rating{
				UserID:       userID,
				GroupID:      group1ID,
				Username:     "testuser",
				Score:        ratingScore,
				CorrectCount: ratingScore / 10,
				WrongCount:   0,
				Streak:       1,
			}
			if err := ratingRepo.UpdateRating(ctx, rating1); err != nil {
				t.Logf("Failed to create rating for group1: %v", err)
				return false
			}

			rating2 := &domain.Rating{
				UserID:       userID,
				GroupID:      group2ID,
				Username:     "testuser",
				Score:        ratingScore * 2,
				CorrectCount: ratingScore / 5,
				WrongCount:   1,
				Streak:       2,
			}
			if err := ratingRepo.UpdateRating(ctx, rating2); err != nil {
				t.Logf("Failed to create rating for group2: %v", err)
				return false
			}

			// Test rating isolation
			retrievedRating1, err := ratingRepo.GetRating(ctx, userID, group1ID)
			if err != nil {
				t.Logf("Failed to get rating for group1: %v", err)
				return false
			}
			if retrievedRating1.GroupID != group1ID {
				t.Logf("Rating isolation violated: rating has group_id %d, expected %d", retrievedRating1.GroupID, group1ID)
				return false
			}
			if retrievedRating1.Score != ratingScore {
				t.Logf("Rating isolation violated: got score %d for group1, expected %d", retrievedRating1.Score, ratingScore)
				return false
			}

			retrievedRating2, err := ratingRepo.GetRating(ctx, userID, group2ID)
			if err != nil {
				t.Logf("Failed to get rating for group2: %v", err)
				return false
			}
			if retrievedRating2.GroupID != group2ID {
				t.Logf("Rating isolation violated: rating has group_id %d, expected %d", retrievedRating2.GroupID, group2ID)
				return false
			}
			if retrievedRating2.Score != ratingScore*2 {
				t.Logf("Rating isolation violated: got score %d for group2, expected %d", retrievedRating2.Score, ratingScore*2)
				return false
			}

			// Create achievements for both groups
			achievement1 := &domain.Achievement{
				UserID:    userID,
				GroupID:   group1ID,
				Code:      domain.AchievementSharpshooter,
				Timestamp: time.Now(),
			}
			if err := achievementRepo.SaveAchievement(ctx, achievement1); err != nil {
				t.Logf("Failed to create achievement for group1: %v", err)
				return false
			}

			achievement2 := &domain.Achievement{
				UserID:    userID,
				GroupID:   group2ID,
				Code:      domain.AchievementVeteran,
				Timestamp: time.Now(),
			}
			if err := achievementRepo.SaveAchievement(ctx, achievement2); err != nil {
				t.Logf("Failed to create achievement for group2: %v", err)
				return false
			}

			// Test achievement isolation
			achievements1, err := achievementRepo.GetUserAchievements(ctx, userID, group1ID)
			if err != nil {
				t.Logf("Failed to get achievements for group1: %v", err)
				return false
			}
			for _, ach := range achievements1 {
				if ach.GroupID != group1ID {
					t.Logf("Achievement isolation violated: achievement has group_id %d, expected %d", ach.GroupID, group1ID)
					return false
				}
				if ach.Code == domain.AchievementVeteran {
					t.Logf("Achievement isolation violated: group1 query returned group2 achievement")
					return false
				}
			}

			achievements2, err := achievementRepo.GetUserAchievements(ctx, userID, group2ID)
			if err != nil {
				t.Logf("Failed to get achievements for group2: %v", err)
				return false
			}
			for _, ach := range achievements2 {
				if ach.GroupID != group2ID {
					t.Logf("Achievement isolation violated: achievement has group_id %d, expected %d", ach.GroupID, group2ID)
					return false
				}
				if ach.Code == domain.AchievementSharpshooter {
					t.Logf("Achievement isolation violated: group2 query returned group1 achievement")
					return false
				}
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.Int64Range(1001, 2000),
		gen.Int64Range(1, 10000),
		gen.IntRange(1, 5),
		gen.IntRange(10, 100),
	))

	properties.TestingRun(t)
}

// TestGroupIdentifierPersistence tests Property 27: Group Identifier Persistence
// For any event, rating, or achievement created, the stored record should include the group identifier.
func TestGroupIdentifierPersistence(t *testing.T) {
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

	// Run migrations to add group_id columns and forum_topic_id
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	eventRepo := NewEventRepository(queue)
	ratingRepo := NewRatingRepository(queue)
	achievementRepo := NewAchievementRepository(queue)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("group identifier persists in all entities", prop.ForAll(
		func(groupID int64, userID int64, score int) bool {
			ctx := context.Background()

			// Test event group identifier persistence
			event := &domain.Event{
				GroupID:   groupID,
				Question:  fmt.Sprintf("Test question %s", time.Now().Format(time.RFC3339Nano)),
				Options:   []string{"Yes", "No"},
				CreatedAt: time.Now().Truncate(time.Second),
				Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: userID,
				PollID:    fmt.Sprintf("poll_%s", time.Now().Format(time.RFC3339Nano)),
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			retrievedEvent, err := eventRepo.GetEvent(ctx, event.ID)
			if err != nil {
				t.Logf("Failed to retrieve event: %v", err)
				return false
			}

			if retrievedEvent.GroupID != groupID {
				t.Logf("Event group ID mismatch: expected %d, got %d", groupID, retrievedEvent.GroupID)
				return false
			}

			// Test rating group identifier persistence
			rating := &domain.Rating{
				UserID:       userID,
				GroupID:      groupID,
				Username:     "testuser",
				Score:        score,
				CorrectCount: 0,
				WrongCount:   0,
				Streak:       0,
			}

			if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
				t.Logf("Failed to create rating: %v", err)
				return false
			}

			retrievedRating, err := ratingRepo.GetRating(ctx, userID, groupID)
			if err != nil {
				t.Logf("Failed to retrieve rating: %v", err)
				return false
			}

			if retrievedRating.GroupID != groupID {
				t.Logf("Rating group ID mismatch: expected %d, got %d", groupID, retrievedRating.GroupID)
				return false
			}

			// Test achievement group identifier persistence
			achievement := &domain.Achievement{
				UserID:    userID,
				GroupID:   groupID,
				Code:      domain.AchievementSharpshooter,
				Timestamp: time.Now(),
			}

			if err := achievementRepo.SaveAchievement(ctx, achievement); err != nil {
				t.Logf("Failed to create achievement: %v", err)
				return false
			}

			achievements, err := achievementRepo.GetUserAchievements(ctx, userID, groupID)
			if err != nil {
				t.Logf("Failed to retrieve achievements: %v", err)
				return false
			}

			if len(achievements) == 0 {
				t.Logf("No achievements retrieved")
				return false
			}

			found := false
			for _, ach := range achievements {
				if ach.ID == achievement.ID {
					if ach.GroupID != groupID {
						t.Logf("Achievement group ID mismatch: expected %d, got %d", groupID, ach.GroupID)
						return false
					}
					found = true
					break
				}
			}

			if !found {
				t.Logf("Achievement not found in retrieved list")
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.Int64Range(1, 10000),
		gen.IntRange(0, 100),
	))

	properties.TestingRun(t)
}
