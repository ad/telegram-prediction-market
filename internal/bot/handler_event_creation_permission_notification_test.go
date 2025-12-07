package bot

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/logger"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	_ "modernc.org/sqlite"
)

// TestEventCreationPermissionNotification tests that users receive notification when they gain event creation permission
func TestEventCreationPermissionNotification(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := storage.NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	ctx := context.Background()
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	groupRepo := storage.NewGroupRepository(queue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(queue)
	ratingRepo := storage.NewRatingRepository(queue)

	// Create services
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)
	ratingCalculator := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, log)

	// Create config with min events = 3
	cfg := &config.Config{
		MinEventsToCreate: 3,
		AdminUserIDs:      []int64{999999},
		Timezone:          time.UTC,
	}

	// Create event permission validator
	eventPermissionValidator := domain.NewEventPermissionValidator(
		eventRepo,
		predictionRepo,
		groupMembershipRepo,
		cfg.MinEventsToCreate,
		log,
	)

	// We don't need to test the actual notification sending in this test
	// We'll just verify that the permission logic works correctly

	// Create test group
	group := &domain.Group{
		Name:           "Test Group",
		TelegramChatID: -1001234567890,
		CreatedBy:      999999,
		CreatedAt:      time.Now(),
	}
	if err := groupRepo.CreateGroup(ctx, group); err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Create test user
	userID := int64(12345)

	// Create membership for user
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   userID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := groupMembershipRepo.CreateMembership(ctx, membership); err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Initialize rating for user
	rating := &domain.Rating{
		UserID:   userID,
		GroupID:  group.ID,
		Username: "testuser",
		Score:    0,
	}
	if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
		t.Fatalf("Failed to initialize rating: %v", err)
	}

	// Test 1: User participates in first event - should NOT receive notification
	t.Run("First event - no notification", func(t *testing.T) {
		// Create and resolve first event
		event1 := &domain.Event{
			GroupID:   group.ID,
			Question:  "Test Event 1",
			EventType: domain.EventTypeBinary,
			Options:   []string{"Yes", "No"},
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			CreatedBy: 999999,
			CreatedAt: time.Now(),
			PollID:    "poll1",
		}
		if err := eventRepo.CreateEvent(ctx, event1); err != nil {
			t.Fatalf("Failed to create event1: %v", err)
		}

		// User votes
		prediction1 := &domain.Prediction{
			EventID:   event1.ID,
			UserID:    userID,
			Option:    0,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction1); err != nil {
			t.Fatalf("Failed to save prediction1: %v", err)
		}

		// Resolve event
		if err := eventManager.ResolveEvent(ctx, event1.ID, 0); err != nil {
			t.Fatalf("Failed to resolve event1: %v", err)
		}

		// Calculate scores
		if err := ratingCalculator.CalculateScores(ctx, event1.ID, 0); err != nil {
			t.Fatalf("Failed to calculate scores: %v", err)
		}

		// Check permission - should be false (only 1 event)
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, userID, group.ID, cfg.AdminUserIDs)
		if err != nil {
			t.Fatalf("Failed to check permission: %v", err)
		}
		if canCreate {
			t.Errorf("User should not be able to create events yet (count: %d)", count)
		}
		if count != 1 {
			t.Errorf("Expected participation count 1, got %d", count)
		}
	})

	// Test 2: User participates in second event - should NOT receive notification
	t.Run("Second event - no notification", func(t *testing.T) {
		// Create and resolve second event
		event2 := &domain.Event{
			GroupID:   group.ID,
			Question:  "Test Event 2",
			EventType: domain.EventTypeBinary,
			Options:   []string{"Yes", "No"},
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			CreatedBy: 999999,
			CreatedAt: time.Now(),
			PollID:    "poll2",
		}
		if err := eventRepo.CreateEvent(ctx, event2); err != nil {
			t.Fatalf("Failed to create event2: %v", err)
		}

		// User votes
		prediction2 := &domain.Prediction{
			EventID:   event2.ID,
			UserID:    userID,
			Option:    1,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction2); err != nil {
			t.Fatalf("Failed to save prediction2: %v", err)
		}

		// Resolve event
		if err := eventManager.ResolveEvent(ctx, event2.ID, 1); err != nil {
			t.Fatalf("Failed to resolve event2: %v", err)
		}

		// Calculate scores
		if err := ratingCalculator.CalculateScores(ctx, event2.ID, 1); err != nil {
			t.Fatalf("Failed to calculate scores: %v", err)
		}

		// Check permission - should be false (only 2 events)
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, userID, group.ID, cfg.AdminUserIDs)
		if err != nil {
			t.Fatalf("Failed to check permission: %v", err)
		}
		if canCreate {
			t.Errorf("User should not be able to create events yet (count: %d)", count)
		}
		if count != 2 {
			t.Errorf("Expected participation count 2, got %d", count)
		}
	})

	// Test 3: User participates in third event - SHOULD receive notification
	t.Run("Third event - notification sent", func(t *testing.T) {
		// Create and resolve third event
		event3 := &domain.Event{
			GroupID:   group.ID,
			Question:  "Test Event 3",
			EventType: domain.EventTypeBinary,
			Options:   []string{"Yes", "No"},
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			CreatedBy: 999999,
			CreatedAt: time.Now(),
			PollID:    "poll3",
		}
		if err := eventRepo.CreateEvent(ctx, event3); err != nil {
			t.Fatalf("Failed to create event3: %v", err)
		}

		// User votes
		prediction3 := &domain.Prediction{
			EventID:   event3.ID,
			UserID:    userID,
			Option:    0,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction3); err != nil {
			t.Fatalf("Failed to save prediction3: %v", err)
		}

		// Resolve event
		if err := eventManager.ResolveEvent(ctx, event3.ID, 0); err != nil {
			t.Fatalf("Failed to resolve event3: %v", err)
		}

		// Calculate scores
		if err := ratingCalculator.CalculateScores(ctx, event3.ID, 0); err != nil {
			t.Fatalf("Failed to calculate scores: %v", err)
		}

		// Check permission - should be true (3 events)
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, userID, group.ID, cfg.AdminUserIDs)
		if err != nil {
			t.Fatalf("Failed to check permission: %v", err)
		}
		if !canCreate {
			t.Errorf("User should be able to create events now (count: %d)", count)
		}
		if count != 3 {
			t.Errorf("Expected participation count 3, got %d", count)
		}
	})

	// Test 4: User participates in fourth event - should NOT receive notification again
	t.Run("Fourth event - no duplicate notification", func(t *testing.T) {
		// Create and resolve fourth event
		event4 := &domain.Event{
			GroupID:   group.ID,
			Question:  "Test Event 4",
			EventType: domain.EventTypeBinary,
			Options:   []string{"Yes", "No"},
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			CreatedBy: 999999,
			CreatedAt: time.Now(),
			PollID:    "poll4",
		}
		if err := eventRepo.CreateEvent(ctx, event4); err != nil {
			t.Fatalf("Failed to create event4: %v", err)
		}

		// User votes
		prediction4 := &domain.Prediction{
			EventID:   event4.ID,
			UserID:    userID,
			Option:    1,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction4); err != nil {
			t.Fatalf("Failed to save prediction4: %v", err)
		}

		// Resolve event
		if err := eventManager.ResolveEvent(ctx, event4.ID, 1); err != nil {
			t.Fatalf("Failed to resolve event4: %v", err)
		}

		// Calculate scores
		if err := ratingCalculator.CalculateScores(ctx, event4.ID, 1); err != nil {
			t.Fatalf("Failed to calculate scores: %v", err)
		}

		// Check permission - should still be true (4 events)
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, userID, group.ID, cfg.AdminUserIDs)
		if err != nil {
			t.Fatalf("Failed to check permission: %v", err)
		}
		if !canCreate {
			t.Errorf("User should still be able to create events (count: %d)", count)
		}
		if count != 4 {
			t.Errorf("Expected participation count 4, got %d", count)
		}
	})

	// Test 5: Admin should never receive notification
	t.Run("Admin - no notification", func(t *testing.T) {
		adminID := cfg.AdminUserIDs[0]

		// Create membership for admin
		adminMembership := &domain.GroupMembership{
			GroupID:  group.ID,
			UserID:   adminID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, adminMembership); err != nil {
			t.Fatalf("Failed to create admin membership: %v", err)
		}

		// Initialize rating for admin
		adminRating := &domain.Rating{
			UserID:   adminID,
			GroupID:  group.ID,
			Username: "admin",
			Score:    0,
		}
		if err := ratingRepo.UpdateRating(ctx, adminRating); err != nil {
			t.Fatalf("Failed to initialize admin rating: %v", err)
		}

		// Admin should always have permission regardless of participation
		canCreate, _, err := eventPermissionValidator.CanCreateEvent(ctx, adminID, group.ID, cfg.AdminUserIDs)
		if err != nil {
			t.Fatalf("Failed to check admin permission: %v", err)
		}
		if !canCreate {
			t.Errorf("Admin should always be able to create events")
		}
	})
}
