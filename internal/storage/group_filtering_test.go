package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"

	_ "modernc.org/sqlite"
)

// TestEventFilteringByGroup tests that events are properly filtered by group
func TestEventFilteringByGroup(t *testing.T) {
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

	repo := NewEventRepository(queue)
	ctx := context.Background()

	group1ID := int64(1)
	group2ID := int64(2)

	// Create events for group 1
	event1 := &domain.Event{
		GroupID:   group1ID,
		Question:  "Group 1 Question 1",
		Options:   []string{"Yes", "No"},
		CreatedAt: time.Now().Truncate(time.Second),
		Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
		Status:    domain.EventStatusActive,
		EventType: domain.EventTypeBinary,
		CreatedBy: 1,
		PollID:    "poll_g1_1",
	}
	if err := repo.CreateEvent(ctx, event1); err != nil {
		t.Fatalf("Failed to create event for group 1: %v", err)
	}

	event2 := &domain.Event{
		GroupID:   group1ID,
		Question:  "Group 1 Question 2",
		Options:   []string{"Yes", "No"},
		CreatedAt: time.Now().Truncate(time.Second),
		Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
		Status:    domain.EventStatusActive,
		EventType: domain.EventTypeBinary,
		CreatedBy: 1,
		PollID:    "poll_g1_2",
	}
	if err := repo.CreateEvent(ctx, event2); err != nil {
		t.Fatalf("Failed to create event for group 1: %v", err)
	}

	// Create events for group 2
	event3 := &domain.Event{
		GroupID:   group2ID,
		Question:  "Group 2 Question 1",
		Options:   []string{"Yes", "No"},
		CreatedAt: time.Now().Truncate(time.Second),
		Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
		Status:    domain.EventStatusActive,
		EventType: domain.EventTypeBinary,
		CreatedBy: 1,
		PollID:    "poll_g2_1",
	}
	if err := repo.CreateEvent(ctx, event3); err != nil {
		t.Fatalf("Failed to create event for group 2: %v", err)
	}

	// Get active events for group 1
	group1Events, err := repo.GetActiveEvents(ctx, group1ID)
	if err != nil {
		t.Fatalf("Failed to get events for group 1: %v", err)
	}

	if len(group1Events) != 2 {
		t.Errorf("Expected 2 events for group 1, got %d", len(group1Events))
	}

	for _, event := range group1Events {
		if event.GroupID != group1ID {
			t.Errorf("Event %d has wrong group ID: expected %d, got %d", event.ID, group1ID, event.GroupID)
		}
	}

	// Get active events for group 2
	group2Events, err := repo.GetActiveEvents(ctx, group2ID)
	if err != nil {
		t.Fatalf("Failed to get events for group 2: %v", err)
	}

	if len(group2Events) != 1 {
		t.Errorf("Expected 1 event for group 2, got %d", len(group2Events))
	}

	for _, event := range group2Events {
		if event.GroupID != group2ID {
			t.Errorf("Event %d has wrong group ID: expected %d, got %d", event.ID, group2ID, event.GroupID)
		}
	}
}

// TestRatingIsolationByGroup tests that ratings are properly isolated by group
func TestRatingIsolationByGroup(t *testing.T) {
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

	repo := NewRatingRepository(queue)
	ctx := context.Background()

	userID := int64(100)
	group1ID := int64(1)
	group2ID := int64(2)

	// Create rating for user in group 1
	rating1 := &domain.Rating{
		UserID:       userID,
		GroupID:      group1ID,
		Username:     "testuser",
		Score:        100,
		CorrectCount: 10,
		WrongCount:   2,
		Streak:       3,
	}
	if err := repo.UpdateRating(ctx, rating1); err != nil {
		t.Fatalf("Failed to create rating for group 1: %v", err)
	}

	// Create rating for same user in group 2
	rating2 := &domain.Rating{
		UserID:       userID,
		GroupID:      group2ID,
		Username:     "testuser",
		Score:        200,
		CorrectCount: 20,
		WrongCount:   1,
		Streak:       5,
	}
	if err := repo.UpdateRating(ctx, rating2); err != nil {
		t.Fatalf("Failed to create rating for group 2: %v", err)
	}

	// Retrieve rating for group 1
	retrievedRating1, err := repo.GetRating(ctx, userID, group1ID)
	if err != nil {
		t.Fatalf("Failed to get rating for group 1: %v", err)
	}

	if retrievedRating1.Score != 100 {
		t.Errorf("Group 1 rating score mismatch: expected 100, got %d", retrievedRating1.Score)
	}
	if retrievedRating1.GroupID != group1ID {
		t.Errorf("Group 1 rating group ID mismatch: expected %d, got %d", group1ID, retrievedRating1.GroupID)
	}

	// Retrieve rating for group 2
	retrievedRating2, err := repo.GetRating(ctx, userID, group2ID)
	if err != nil {
		t.Fatalf("Failed to get rating for group 2: %v", err)
	}

	if retrievedRating2.Score != 200 {
		t.Errorf("Group 2 rating score mismatch: expected 200, got %d", retrievedRating2.Score)
	}
	if retrievedRating2.GroupID != group2ID {
		t.Errorf("Group 2 rating group ID mismatch: expected %d, got %d", group2ID, retrievedRating2.GroupID)
	}

	// Test GetTopRatings filtering
	topRatings1, err := repo.GetTopRatings(ctx, group1ID, 10)
	if err != nil {
		t.Fatalf("Failed to get top ratings for group 1: %v", err)
	}

	for _, rating := range topRatings1 {
		if rating.GroupID != group1ID {
			t.Errorf("Top ratings for group 1 contains rating from group %d", rating.GroupID)
		}
	}

	topRatings2, err := repo.GetTopRatings(ctx, group2ID, 10)
	if err != nil {
		t.Fatalf("Failed to get top ratings for group 2: %v", err)
	}

	for _, rating := range topRatings2 {
		if rating.GroupID != group2ID {
			t.Errorf("Top ratings for group 2 contains rating from group %d", rating.GroupID)
		}
	}
}

// TestAchievementFilteringByGroup tests that achievements are properly filtered by group
func TestAchievementFilteringByGroup(t *testing.T) {
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

	repo := NewAchievementRepository(queue)
	ctx := context.Background()

	userID := int64(100)
	group1ID := int64(1)
	group2ID := int64(2)

	// Create achievement for user in group 1
	achievement1 := &domain.Achievement{
		UserID:    userID,
		GroupID:   group1ID,
		Code:      domain.AchievementSharpshooter,
		Timestamp: time.Now(),
	}
	if err := repo.SaveAchievement(ctx, achievement1); err != nil {
		t.Fatalf("Failed to create achievement for group 1: %v", err)
	}

	// Create achievement for same user in group 2
	achievement2 := &domain.Achievement{
		UserID:    userID,
		GroupID:   group2ID,
		Code:      domain.AchievementVeteran,
		Timestamp: time.Now(),
	}
	if err := repo.SaveAchievement(ctx, achievement2); err != nil {
		t.Fatalf("Failed to create achievement for group 2: %v", err)
	}

	// Get achievements for group 1
	achievements1, err := repo.GetUserAchievements(ctx, userID, group1ID)
	if err != nil {
		t.Fatalf("Failed to get achievements for group 1: %v", err)
	}

	if len(achievements1) != 1 {
		t.Errorf("Expected 1 achievement for group 1, got %d", len(achievements1))
	}

	if achievements1[0].Code != domain.AchievementSharpshooter {
		t.Errorf("Wrong achievement for group 1: expected %s, got %s", domain.AchievementSharpshooter, achievements1[0].Code)
	}

	if achievements1[0].GroupID != group1ID {
		t.Errorf("Achievement has wrong group ID: expected %d, got %d", group1ID, achievements1[0].GroupID)
	}

	// Get achievements for group 2
	achievements2, err := repo.GetUserAchievements(ctx, userID, group2ID)
	if err != nil {
		t.Fatalf("Failed to get achievements for group 2: %v", err)
	}

	if len(achievements2) != 1 {
		t.Errorf("Expected 1 achievement for group 2, got %d", len(achievements2))
	}

	if achievements2[0].Code != domain.AchievementVeteran {
		t.Errorf("Wrong achievement for group 2: expected %s, got %s", domain.AchievementVeteran, achievements2[0].Code)
	}

	if achievements2[0].GroupID != group2ID {
		t.Errorf("Achievement has wrong group ID: expected %d, got %d", group2ID, achievements2[0].GroupID)
	}

	// Test CheckAchievementExists filtering
	exists1, err := repo.CheckAchievementExists(ctx, userID, group1ID, domain.AchievementSharpshooter)
	if err != nil {
		t.Fatalf("Failed to check achievement existence for group 1: %v", err)
	}
	if !exists1 {
		t.Error("Achievement should exist in group 1")
	}

	exists2, err := repo.CheckAchievementExists(ctx, userID, group1ID, domain.AchievementVeteran)
	if err != nil {
		t.Fatalf("Failed to check achievement existence for group 1: %v", err)
	}
	if exists2 {
		t.Error("Achievement should not exist in group 1")
	}
}

// TestPredictionGroupDerivation tests that predictions respect group boundaries
func TestPredictionGroupDerivation(t *testing.T) {
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
	predictionRepo := NewPredictionRepository(queue)
	ctx := context.Background()

	userID := int64(100)
	group1ID := int64(1)
	group2ID := int64(2)

	// Create event in group 1
	event1 := &domain.Event{
		GroupID:   group1ID,
		Question:  "Group 1 Question",
		Options:   []string{"Yes", "No"},
		CreatedAt: time.Now().Truncate(time.Second),
		Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
		Status:    domain.EventStatusResolved,
		EventType: domain.EventTypeBinary,
		CreatedBy: 1,
		PollID:    "poll_g1",
	}
	if err := eventRepo.CreateEvent(ctx, event1); err != nil {
		t.Fatalf("Failed to create event for group 1: %v", err)
	}

	// Create prediction for event in group 1
	prediction1 := &domain.Prediction{
		EventID:   event1.ID,
		UserID:    userID,
		Option:    0,
		Timestamp: time.Now().Truncate(time.Second),
	}
	if err := predictionRepo.SavePrediction(ctx, prediction1); err != nil {
		t.Fatalf("Failed to save prediction: %v", err)
	}

	// Create event in group 2
	event2 := &domain.Event{
		GroupID:   group2ID,
		Question:  "Group 2 Question",
		Options:   []string{"Yes", "No"},
		CreatedAt: time.Now().Truncate(time.Second),
		Deadline:  time.Now().Add(24 * time.Hour).Truncate(time.Second),
		Status:    domain.EventStatusResolved,
		EventType: domain.EventTypeBinary,
		CreatedBy: 1,
		PollID:    "poll_g2",
	}
	if err := eventRepo.CreateEvent(ctx, event2); err != nil {
		t.Fatalf("Failed to create event for group 2: %v", err)
	}

	// Create prediction for event in group 2
	prediction2 := &domain.Prediction{
		EventID:   event2.ID,
		UserID:    userID,
		Option:    1,
		Timestamp: time.Now().Truncate(time.Second),
	}
	if err := predictionRepo.SavePrediction(ctx, prediction2); err != nil {
		t.Fatalf("Failed to save prediction: %v", err)
	}

	// Test GetUserCompletedEventCount respects group boundaries
	count1, err := predictionRepo.GetUserCompletedEventCount(ctx, userID, group1ID)
	if err != nil {
		t.Fatalf("Failed to get completed event count for group 1: %v", err)
	}
	if count1 != 1 {
		t.Errorf("Expected 1 completed event for group 1, got %d", count1)
	}

	count2, err := predictionRepo.GetUserCompletedEventCount(ctx, userID, group2ID)
	if err != nil {
		t.Fatalf("Failed to get completed event count for group 2: %v", err)
	}
	if count2 != 1 {
		t.Errorf("Expected 1 completed event for group 2, got %d", count2)
	}

	// Test GetUserPredictionsByGroup
	predictions1, err := predictionRepo.GetUserPredictionsByGroup(ctx, userID, group1ID)
	if err != nil {
		t.Fatalf("Failed to get predictions for group 1: %v", err)
	}
	if len(predictions1) != 1 {
		t.Errorf("Expected 1 prediction for group 1, got %d", len(predictions1))
	}
	if predictions1[0].Option != 0 {
		t.Errorf("Wrong prediction option for group 1: expected 0, got %d", predictions1[0].Option)
	}

	predictions2, err := predictionRepo.GetUserPredictionsByGroup(ctx, userID, group2ID)
	if err != nil {
		t.Fatalf("Failed to get predictions for group 2: %v", err)
	}
	if len(predictions2) != 1 {
		t.Errorf("Expected 1 prediction for group 2, got %d", len(predictions2))
	}
	if predictions2[0].Option != 1 {
		t.Errorf("Wrong prediction option for group 2: expected 1, got %d", predictions2[0].Option)
	}
}
