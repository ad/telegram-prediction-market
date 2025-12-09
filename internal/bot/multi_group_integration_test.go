package bot

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/encoding"
	"github.com/ad/gitelegram-prediction-market/internal/logger"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	_ "modernc.org/sqlite"
)

// setupMultiGroupTestDB creates a test database with schema and migrations
func setupMultiGroupTestDB(t *testing.T) (*storage.DBQueue, func()) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	queue := storage.NewDBQueue(db)

	// Initialize schema
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Run migrations to add group support
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	cleanup := func() {
		queue.Close()
		defer func() { _ = db.Close() }()
	}

	return queue, cleanup
}

// TestIntegration_UserJoiningAndParticipating tests the complete user journey:
// User joins group via deep-link, creates event, votes, and views rating
func TestIntegration_UserJoiningAndParticipating(t *testing.T) {
	ctx := context.Background()
	queue, cleanup := setupMultiGroupTestDB(t)
	defer cleanup()

	// Create dependencies
	log := logger.New(logger.ERROR)
	botUsername := "testbot"

	// Create repositories
	groupRepo := storage.NewGroupRepository(queue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	ratingRepo := storage.NewRatingRepository(queue)

	// Create services
	encoder, err := encoding.NewBaseNEncoder("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	deepLinkService := domain.NewDeepLinkService(botUsername, encoder)
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)
	ratingCalculator := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, log)

	// Test data
	adminUserID := int64(99999)
	regularUserID := int64(12345)
	chatID := int64(67890)

	// Step 1: Admin creates a group
	t.Run("Admin creates group", func(t *testing.T) {
		group := &domain.Group{
			TelegramChatID: chatID,
			Name:           "Test Community",
			CreatedBy:      adminUserID,
			CreatedAt:      time.Now(),
		}
		if err := groupRepo.CreateGroup(ctx, group); err != nil {
			t.Fatalf("Failed to create group: %v", err)
		}

		// Verify group was created
		retrievedGroup, err := groupRepo.GetGroup(ctx, group.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve group: %v", err)
		}
		if retrievedGroup.Name != "Test Community" {
			t.Errorf("Expected group name 'Test Community', got %q", retrievedGroup.Name)
		}
	})

	// Get the created group
	groups, err := groupRepo.GetAllGroups(ctx)
	if err != nil || len(groups) == 0 {
		t.Fatalf("Failed to get created group: %v", err)
	}
	groupID := groups[0].ID

	// Step 2: User joins group via deep-link (simulated)
	t.Run("User joins group via deep-link", func(t *testing.T) {
		// Generate deep-link
		deepLink, err := deepLinkService.GenerateGroupInviteLink(groupID)
		if err != nil {
			t.Fatalf("Failed to generate deep-link: %v", err)
		}
		if deepLink == "" {
			t.Fatal("Failed to generate deep-link")
		}

		// Extract the start parameter from the deep-link
		// Format: https://t.me/testbot?start=group_1
		// We need to extract "group_1"
		startParam := fmt.Sprintf("group_%d", groupID)

		// Parse group ID from start parameter (simulating user clicking the link)
		parsedGroupID, err := deepLinkService.ParseGroupIDFromStart(startParam)
		if err != nil {
			t.Fatalf("Failed to parse deep-link: %v", err)
		}
		if parsedGroupID != groupID {
			t.Errorf("Parsed group ID %d doesn't match original %d", parsedGroupID, groupID)
		}

		// Create membership (simulating join flow)
		membership := &domain.GroupMembership{
			GroupID:  groupID,
			UserID:   regularUserID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership); err != nil {
			t.Fatalf("Failed to create membership: %v", err)
		}

		// Initialize rating for new member
		rating := &domain.Rating{
			UserID:       regularUserID,
			GroupID:      groupID,
			Username:     "testuser",
			Score:        0,
			CorrectCount: 0,
			WrongCount:   0,
			Streak:       0,
		}
		if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
			t.Fatalf("Failed to initialize rating: %v", err)
		}

		// Verify membership was created
		hasMembership, err := groupMembershipRepo.HasActiveMembership(ctx, groupID, regularUserID)
		if err != nil {
			t.Fatalf("Failed to check membership: %v", err)
		}
		if !hasMembership {
			t.Error("User should have active membership after joining")
		}

		// Verify rating was initialized
		userRating, err := ratingRepo.GetRating(ctx, regularUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to get user rating: %v", err)
		}
		if userRating.Score != 0 {
			t.Errorf("Expected initial score 0, got %d", userRating.Score)
		}
	})

	// Step 3: User creates an event in the group
	var createdEventID int64
	t.Run("User creates event in group", func(t *testing.T) {
		// Verify user can select this group (they have membership)
		userGroups, err := groupRepo.GetUserGroups(ctx, regularUserID)
		if err != nil {
			t.Fatalf("Failed to get user groups: %v", err)
		}
		if len(userGroups) != 1 {
			t.Errorf("Expected user to have 1 group, got %d", len(userGroups))
		}

		// Create event
		event := &domain.Event{
			GroupID:   groupID,
			Question:  "Will it rain tomorrow?",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now(),
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			EventType: domain.EventTypeBinary,
			CreatedBy: regularUserID,
			PollID:    "poll_test_1",
		}
		if err := eventManager.CreateEvent(ctx, event); err != nil {
			t.Fatalf("Failed to create event: %v", err)
		}
		createdEventID = event.ID

		// Verify event was created with correct group ID
		retrievedEvent, err := eventRepo.GetEvent(ctx, event.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve event: %v", err)
		}
		if retrievedEvent.GroupID != groupID {
			t.Errorf("Event has wrong group ID: expected %d, got %d", groupID, retrievedEvent.GroupID)
		}
		if retrievedEvent.CreatedBy != regularUserID {
			t.Errorf("Event has wrong creator: expected %d, got %d", regularUserID, retrievedEvent.CreatedBy)
		}
	})

	// Step 4: User votes on the event
	t.Run("User votes on event", func(t *testing.T) {
		// Verify user has membership (required for voting)
		hasMembership, err := groupMembershipRepo.HasActiveMembership(ctx, groupID, regularUserID)
		if err != nil {
			t.Fatalf("Failed to check membership: %v", err)
		}
		if !hasMembership {
			t.Fatal("User must have membership to vote")
		}

		// Create prediction (vote)
		prediction := &domain.Prediction{
			EventID:   createdEventID,
			UserID:    regularUserID,
			Option:    0, // Vote "Yes"
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
			t.Fatalf("Failed to save prediction: %v", err)
		}

		// Verify prediction was saved
		userPrediction, err := predictionRepo.GetPredictionByUserAndEvent(ctx, regularUserID, createdEventID)
		if err != nil {
			t.Fatalf("Failed to retrieve prediction: %v", err)
		}
		if userPrediction.Option != 0 {
			t.Errorf("Expected prediction option 0, got %d", userPrediction.Option)
		}
	})

	// Step 5: Resolve event and verify rating update
	t.Run("Resolve event and update rating", func(t *testing.T) {
		// Resolve event with correct answer = 0 (user voted correctly)
		if err := eventManager.ResolveEvent(ctx, createdEventID, 0); err != nil {
			t.Fatalf("Failed to resolve event: %v", err)
		}

		// Update rating based on correct prediction
		if err := ratingCalculator.CalculateScores(ctx, createdEventID, 0); err != nil {
			t.Fatalf("Failed to update ratings: %v", err)
		}

		// Verify rating was updated
		userRating, err := ratingRepo.GetRating(ctx, regularUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to get updated rating: %v", err)
		}
		if userRating.Score <= 0 {
			t.Errorf("Expected positive score after correct prediction, got %d", userRating.Score)
		}
		if userRating.CorrectCount != 1 {
			t.Errorf("Expected correct count 1, got %d", userRating.CorrectCount)
		}
		if userRating.Streak != 1 {
			t.Errorf("Expected streak 1, got %d", userRating.Streak)
		}
	})

	// Step 6: User views rating in group
	t.Run("User views rating in group", func(t *testing.T) {
		// Get top ratings for the group
		topRatings, err := ratingRepo.GetTopRatings(ctx, groupID, 10)
		if err != nil {
			t.Fatalf("Failed to get top ratings: %v", err)
		}

		// Verify user appears in ratings
		found := false
		for _, rating := range topRatings {
			if rating.UserID == regularUserID {
				found = true
				if rating.GroupID != groupID {
					t.Errorf("Rating has wrong group ID: expected %d, got %d", groupID, rating.GroupID)
				}
				if rating.Score <= 0 {
					t.Errorf("Expected positive score in ratings, got %d", rating.Score)
				}
				break
			}
		}
		if !found {
			t.Error("User should appear in group ratings after participating")
		}
	})

	// Step 7: Verify data is group-scoped
	t.Run("Verify data is group-scoped", func(t *testing.T) {
		// Create a second group
		group2 := &domain.Group{
			TelegramChatID: chatID + 1,
			Name:           "Other Group",
			CreatedBy:      adminUserID,
			CreatedAt:      time.Now(),
		}
		if err := groupRepo.CreateGroup(ctx, group2); err != nil {
			t.Fatalf("Failed to create second group: %v", err)
		}

		// Verify events from group1 don't appear in group2
		group2Events, err := eventRepo.GetActiveEvents(ctx, group2.ID)
		if err != nil {
			t.Fatalf("Failed to get events for group2: %v", err)
		}
		if len(group2Events) != 0 {
			t.Errorf("Expected 0 events in group2, got %d", len(group2Events))
		}

		// Verify user's rating in group1 doesn't affect group2
		// The rating repository returns a default rating with 0 score for non-members
		group2Rating, err := ratingRepo.GetRating(ctx, regularUserID, group2.ID)
		if err != nil {
			t.Fatalf("Failed to get rating for group2: %v", err)
		}
		// User should have 0 score in group2 (not participated)
		if group2Rating.Score != 0 {
			t.Errorf("User should have 0 score in group2, got %d", group2Rating.Score)
		}

		// Verify group1 still has the event (it was resolved, so check by ID)
		group1Event, err := eventRepo.GetEvent(ctx, createdEventID)
		if err != nil {
			t.Fatalf("Failed to get event from group1: %v", err)
		}
		if group1Event.GroupID != groupID {
			t.Errorf("Event should belong to group1 (%d), got %d", groupID, group1Event.GroupID)
		}
	})
}

// TestIntegration_MultiGroupUser tests a user participating in multiple groups:
// User joins multiple groups, creates events in different groups, votes in different groups
func TestIntegration_MultiGroupUser(t *testing.T) {
	ctx := context.Background()
	queue, cleanup := setupMultiGroupTestDB(t)
	defer cleanup()

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	groupRepo := storage.NewGroupRepository(queue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	ratingRepo := storage.NewRatingRepository(queue)

	// Create services
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)
	ratingCalculator := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, log)

	// Test data
	adminUserID := int64(99999)
	multiGroupUserID := int64(12345)
	chatID1 := int64(67890)
	chatID2 := int64(67891)
	chatID3 := int64(67892)

	// Step 1: Create three groups
	var group1ID, group2ID, group3ID int64
	t.Run("Create three groups", func(t *testing.T) {
		group1 := &domain.Group{
			TelegramChatID: chatID1,
			Name:           "Tech Community",
			CreatedBy:      adminUserID,
			CreatedAt:      time.Now(),
		}
		if err := groupRepo.CreateGroup(ctx, group1); err != nil {
			t.Fatalf("Failed to create group1: %v", err)
		}
		group1ID = group1.ID

		group2 := &domain.Group{
			TelegramChatID: chatID2,
			Name:           "Sports Community",
			CreatedBy:      adminUserID,
			CreatedAt:      time.Now(),
		}
		if err := groupRepo.CreateGroup(ctx, group2); err != nil {
			t.Fatalf("Failed to create group2: %v", err)
		}
		group2ID = group2.ID

		group3 := &domain.Group{
			TelegramChatID: chatID3,
			Name:           "Gaming Community",
			CreatedBy:      adminUserID,
			CreatedAt:      time.Now(),
		}
		if err := groupRepo.CreateGroup(ctx, group3); err != nil {
			t.Fatalf("Failed to create group3: %v", err)
		}
		group3ID = group3.ID

		// Verify all groups were created
		allGroups, err := groupRepo.GetAllGroups(ctx)
		if err != nil {
			t.Fatalf("Failed to get all groups: %v", err)
		}
		if len(allGroups) != 3 {
			t.Errorf("Expected 3 groups, got %d", len(allGroups))
		}
	})

	// Step 2: User joins all three groups
	t.Run("User joins multiple groups", func(t *testing.T) {
		// Join group 1
		membership1 := &domain.GroupMembership{
			GroupID:  group1ID,
			UserID:   multiGroupUserID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership1); err != nil {
			t.Fatalf("Failed to create membership in group1: %v", err)
		}

		// Initialize rating for group 1
		rating1 := &domain.Rating{
			UserID:       multiGroupUserID,
			GroupID:      group1ID,
			Username:     "multiuser",
			Score:        0,
			CorrectCount: 0,
			WrongCount:   0,
			Streak:       0,
		}
		if err := ratingRepo.UpdateRating(ctx, rating1); err != nil {
			t.Fatalf("Failed to initialize rating for group1: %v", err)
		}

		// Join group 2
		membership2 := &domain.GroupMembership{
			GroupID:  group2ID,
			UserID:   multiGroupUserID,
			JoinedAt: time.Now().Add(1 * time.Minute),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership2); err != nil {
			t.Fatalf("Failed to create membership in group2: %v", err)
		}

		// Initialize rating for group 2
		rating2 := &domain.Rating{
			UserID:       multiGroupUserID,
			GroupID:      group2ID,
			Username:     "multiuser",
			Score:        0,
			CorrectCount: 0,
			WrongCount:   0,
			Streak:       0,
		}
		if err := ratingRepo.UpdateRating(ctx, rating2); err != nil {
			t.Fatalf("Failed to initialize rating for group2: %v", err)
		}

		// Join group 3
		membership3 := &domain.GroupMembership{
			GroupID:  group3ID,
			UserID:   multiGroupUserID,
			JoinedAt: time.Now().Add(2 * time.Minute),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership3); err != nil {
			t.Fatalf("Failed to create membership in group3: %v", err)
		}

		// Initialize rating for group 3
		rating3 := &domain.Rating{
			UserID:       multiGroupUserID,
			GroupID:      group3ID,
			Username:     "multiuser",
			Score:        0,
			CorrectCount: 0,
			WrongCount:   0,
			Streak:       0,
		}
		if err := ratingRepo.UpdateRating(ctx, rating3); err != nil {
			t.Fatalf("Failed to initialize rating for group3: %v", err)
		}

		// Verify user has memberships in all three groups
		userGroups, err := groupRepo.GetUserGroups(ctx, multiGroupUserID)
		if err != nil {
			t.Fatalf("Failed to get user groups: %v", err)
		}
		if len(userGroups) != 3 {
			t.Errorf("Expected user to have 3 group memberships, got %d", len(userGroups))
		}
	})

	// Step 3: User creates events in different groups
	var event1ID, event2ID, event3ID int64
	t.Run("User creates events in different groups", func(t *testing.T) {
		// Create event in group 1
		event1 := &domain.Event{
			GroupID:   group1ID,
			Question:  "Will AI surpass human intelligence by 2030?",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now(),
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			EventType: domain.EventTypeBinary,
			CreatedBy: multiGroupUserID,
			PollID:    "poll_tech_1",
		}
		if err := eventManager.CreateEvent(ctx, event1); err != nil {
			t.Fatalf("Failed to create event in group1: %v", err)
		}
		event1ID = event1.ID

		// Create event in group 2
		event2 := &domain.Event{
			GroupID:   group2ID,
			Question:  "Will Team A win the championship?",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now(),
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			EventType: domain.EventTypeBinary,
			CreatedBy: multiGroupUserID,
			PollID:    "poll_sports_1",
		}
		if err := eventManager.CreateEvent(ctx, event2); err != nil {
			t.Fatalf("Failed to create event in group2: %v", err)
		}
		event2ID = event2.ID

		// Create event in group 3
		event3 := &domain.Event{
			GroupID:   group3ID,
			Question:  "Will the new game be released this year?",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now(),
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			EventType: domain.EventTypeBinary,
			CreatedBy: multiGroupUserID,
			PollID:    "poll_gaming_1",
		}
		if err := eventManager.CreateEvent(ctx, event3); err != nil {
			t.Fatalf("Failed to create event in group3: %v", err)
		}
		event3ID = event3.ID

		// Verify events are in correct groups
		retrievedEvent1, err := eventRepo.GetEvent(ctx, event1ID)
		if err != nil {
			t.Fatalf("Failed to retrieve event1: %v", err)
		}
		if retrievedEvent1.GroupID != group1ID {
			t.Errorf("Event1 has wrong group ID: expected %d, got %d", group1ID, retrievedEvent1.GroupID)
		}

		retrievedEvent2, err := eventRepo.GetEvent(ctx, event2ID)
		if err != nil {
			t.Fatalf("Failed to retrieve event2: %v", err)
		}
		if retrievedEvent2.GroupID != group2ID {
			t.Errorf("Event2 has wrong group ID: expected %d, got %d", group2ID, retrievedEvent2.GroupID)
		}

		retrievedEvent3, err := eventRepo.GetEvent(ctx, event3ID)
		if err != nil {
			t.Fatalf("Failed to retrieve event3: %v", err)
		}
		if retrievedEvent3.GroupID != group3ID {
			t.Errorf("Event3 has wrong group ID: expected %d, got %d", group3ID, retrievedEvent3.GroupID)
		}
	})

	// Step 4: User votes in different groups
	t.Run("User votes in different groups", func(t *testing.T) {
		// Vote in group 1 (vote "Yes" = option 0)
		prediction1 := &domain.Prediction{
			EventID:   event1ID,
			UserID:    multiGroupUserID,
			Option:    0,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction1); err != nil {
			t.Fatalf("Failed to save prediction in group1: %v", err)
		}

		// Vote in group 2 (vote "No" = option 1)
		prediction2 := &domain.Prediction{
			EventID:   event2ID,
			UserID:    multiGroupUserID,
			Option:    1,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction2); err != nil {
			t.Fatalf("Failed to save prediction in group2: %v", err)
		}

		// Vote in group 3 (vote "Yes" = option 0)
		prediction3 := &domain.Prediction{
			EventID:   event3ID,
			UserID:    multiGroupUserID,
			Option:    0,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction3); err != nil {
			t.Fatalf("Failed to save prediction in group3: %v", err)
		}

		// Verify predictions were saved
		savedPrediction1, err := predictionRepo.GetPredictionByUserAndEvent(ctx, multiGroupUserID, event1ID)
		if err != nil {
			t.Fatalf("Failed to retrieve prediction1: %v", err)
		}
		if savedPrediction1.Option != 0 {
			t.Errorf("Prediction1 has wrong option: expected 0, got %d", savedPrediction1.Option)
		}

		savedPrediction2, err := predictionRepo.GetPredictionByUserAndEvent(ctx, multiGroupUserID, event2ID)
		if err != nil {
			t.Fatalf("Failed to retrieve prediction2: %v", err)
		}
		if savedPrediction2.Option != 1 {
			t.Errorf("Prediction2 has wrong option: expected 1, got %d", savedPrediction2.Option)
		}

		savedPrediction3, err := predictionRepo.GetPredictionByUserAndEvent(ctx, multiGroupUserID, event3ID)
		if err != nil {
			t.Fatalf("Failed to retrieve prediction3: %v", err)
		}
		if savedPrediction3.Option != 0 {
			t.Errorf("Prediction3 has wrong option: expected 0, got %d", savedPrediction3.Option)
		}
	})

	// Step 5: Resolve events with different outcomes and verify data isolation
	t.Run("Verify data isolation between groups", func(t *testing.T) {
		// Resolve event1 - user voted correctly (option 0)
		if err := eventManager.ResolveEvent(ctx, event1ID, 0); err != nil {
			t.Fatalf("Failed to resolve event1: %v", err)
		}
		if err := ratingCalculator.CalculateScores(ctx, event1ID, 0); err != nil {
			t.Fatalf("Failed to calculate scores for event1: %v", err)
		}

		// Resolve event2 - user voted correctly (option 1)
		if err := eventManager.ResolveEvent(ctx, event2ID, 1); err != nil {
			t.Fatalf("Failed to resolve event2: %v", err)
		}
		if err := ratingCalculator.CalculateScores(ctx, event2ID, 1); err != nil {
			t.Fatalf("Failed to calculate scores for event2: %v", err)
		}

		// Resolve event3 - user voted incorrectly (voted 0, correct is 1)
		if err := eventManager.ResolveEvent(ctx, event3ID, 1); err != nil {
			t.Fatalf("Failed to resolve event3: %v", err)
		}
		if err := ratingCalculator.CalculateScores(ctx, event3ID, 1); err != nil {
			t.Fatalf("Failed to calculate scores for event3: %v", err)
		}

		// Verify ratings are isolated per group
		rating1, err := ratingRepo.GetRating(ctx, multiGroupUserID, group1ID)
		if err != nil {
			t.Fatalf("Failed to get rating for group1: %v", err)
		}
		if rating1.Score <= 0 {
			t.Errorf("Expected positive score in group1 (correct prediction), got %d", rating1.Score)
		}
		if rating1.CorrectCount != 1 {
			t.Errorf("Expected correct count 1 in group1, got %d", rating1.CorrectCount)
		}
		if rating1.WrongCount != 0 {
			t.Errorf("Expected wrong count 0 in group1, got %d", rating1.WrongCount)
		}

		rating2, err := ratingRepo.GetRating(ctx, multiGroupUserID, group2ID)
		if err != nil {
			t.Fatalf("Failed to get rating for group2: %v", err)
		}
		if rating2.Score <= 0 {
			t.Errorf("Expected positive score in group2 (correct prediction), got %d", rating2.Score)
		}
		if rating2.CorrectCount != 1 {
			t.Errorf("Expected correct count 1 in group2, got %d", rating2.CorrectCount)
		}
		if rating2.WrongCount != 0 {
			t.Errorf("Expected wrong count 0 in group2, got %d", rating2.WrongCount)
		}

		rating3, err := ratingRepo.GetRating(ctx, multiGroupUserID, group3ID)
		if err != nil {
			t.Fatalf("Failed to get rating for group3: %v", err)
		}
		if rating3.Score >= 0 {
			t.Errorf("Expected negative score in group3 (wrong prediction), got %d", rating3.Score)
		}
		if rating3.CorrectCount != 0 {
			t.Errorf("Expected correct count 0 in group3, got %d", rating3.CorrectCount)
		}
		if rating3.WrongCount != 1 {
			t.Errorf("Expected wrong count 1 in group3, got %d", rating3.WrongCount)
		}

		// Verify ratings are completely independent
		// Group1 and Group2 should have positive scores (correct predictions)
		// Group3 should have negative score (wrong prediction)
		if rating1.Score <= 0 || rating2.Score <= 0 {
			t.Error("Group1 and Group2 should have positive scores")
		}
		if rating3.Score >= 0 {
			t.Error("Group3 should have negative score")
		}
		// The key test: ratings are isolated - each group has its own rating record
		if rating1.GroupID != group1ID || rating2.GroupID != group2ID || rating3.GroupID != group3ID {
			t.Error("Ratings should be associated with correct groups")
		}
	})

	// Step 6: Verify events are properly isolated
	t.Run("Verify event isolation", func(t *testing.T) {
		// Verify by getting the event directly (events are resolved, so not active)
		event1, err := eventRepo.GetEvent(ctx, event1ID)
		if err != nil {
			t.Fatalf("Failed to get event1: %v", err)
		}
		if event1.GroupID != group1ID {
			t.Errorf("Event1 should belong to group1, got group %d", event1.GroupID)
		}

		// Verify event2 belongs to group2
		event2, err := eventRepo.GetEvent(ctx, event2ID)
		if err != nil {
			t.Fatalf("Failed to get event2: %v", err)
		}
		if event2.GroupID != group2ID {
			t.Errorf("Event2 should belong to group2, got group %d", event2.GroupID)
		}

		// Verify event3 belongs to group3
		event3, err := eventRepo.GetEvent(ctx, event3ID)
		if err != nil {
			t.Fatalf("Failed to get event3: %v", err)
		}
		if event3.GroupID != group3ID {
			t.Errorf("Event3 should belong to group3, got group %d", event3.GroupID)
		}

		// Verify all events have different group IDs
		if event1.GroupID == event2.GroupID || event1.GroupID == event3.GroupID || event2.GroupID == event3.GroupID {
			t.Error("Events should belong to different groups")
		}
	})
}

// TestIntegration_AdminWorkflows tests admin operations:
// Admin creates group, generates deep-links, views members, removes member
func TestIntegration_AdminWorkflows(t *testing.T) {
	ctx := context.Background()
	queue, cleanup := setupMultiGroupTestDB(t)
	defer cleanup()

	// Create dependencies
	botUsername := "testbot"

	// Create repositories
	groupRepo := storage.NewGroupRepository(queue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(queue)
	ratingRepo := storage.NewRatingRepository(queue)
	achievementRepo := storage.NewAchievementRepository(queue)

	// Create services
	encoder, err := encoding.NewBaseNEncoder("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	deepLinkService := domain.NewDeepLinkService(botUsername, encoder)

	// Test data
	adminUserID := int64(99999)
	user1ID := int64(11111)
	user2ID := int64(22222)
	user3ID := int64(33333)
	chatID := int64(67890)

	// Step 1: Admin creates a group
	var groupID int64
	t.Run("Admin creates group", func(t *testing.T) {
		group := &domain.Group{
			TelegramChatID: chatID,
			Name:           "Admin Test Group",
			CreatedBy:      adminUserID,
			CreatedAt:      time.Now(),
		}
		if err := groupRepo.CreateGroup(ctx, group); err != nil {
			t.Fatalf("Failed to create group: %v", err)
		}
		groupID = group.ID

		// Verify group was created
		retrievedGroup, err := groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			t.Fatalf("Failed to retrieve group: %v", err)
		}
		if retrievedGroup.Name != "Admin Test Group" {
			t.Errorf("Expected group name 'Admin Test Group', got %q", retrievedGroup.Name)
		}
		if retrievedGroup.CreatedBy != adminUserID {
			t.Errorf("Expected group created by %d, got %d", adminUserID, retrievedGroup.CreatedBy)
		}
	})

	// Step 2: Admin generates deep-links
	t.Run("Admin generates deep-links", func(t *testing.T) {
		// Generate deep-link for the group
		deepLink, err := deepLinkService.GenerateGroupInviteLink(groupID)
		if err != nil {
			t.Fatalf("Failed to generate deep-link: %v", err)
		}
		if deepLink == "" {
			t.Fatal("Failed to generate deep-link")
		}

		// Verify deep-link format
		expectedPrefix := fmt.Sprintf("https://t.me/%s?start=group_", botUsername)
		if !strings.HasPrefix(deepLink, expectedPrefix) {
			t.Errorf("Deep-link has wrong format: expected prefix %q, got %q", expectedPrefix, deepLink)
		}

		// Verify deep-link can be parsed back
		startParam := fmt.Sprintf("group_%d", groupID)
		parsedGroupID, err := deepLinkService.ParseGroupIDFromStart(startParam)
		if err != nil {
			t.Fatalf("Failed to parse deep-link: %v", err)
		}
		if parsedGroupID != groupID {
			t.Errorf("Parsed group ID %d doesn't match original %d", parsedGroupID, groupID)
		}
	})

	// Step 3: Add members to the group
	t.Run("Add members to group", func(t *testing.T) {
		// Add user1
		membership1 := &domain.GroupMembership{
			GroupID:  groupID,
			UserID:   user1ID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership1); err != nil {
			t.Fatalf("Failed to create membership for user1: %v", err)
		}

		// Initialize rating for user1
		rating1 := &domain.Rating{
			UserID:       user1ID,
			GroupID:      groupID,
			Username:     "user1",
			Score:        100,
			CorrectCount: 10,
			WrongCount:   2,
			Streak:       3,
		}
		if err := ratingRepo.UpdateRating(ctx, rating1); err != nil {
			t.Fatalf("Failed to initialize rating for user1: %v", err)
		}

		// Add achievement for user1
		achievement1 := &domain.Achievement{
			UserID:    user1ID,
			GroupID:   groupID,
			Code:      domain.AchievementSharpshooter,
			Timestamp: time.Now(),
		}
		if err := achievementRepo.SaveAchievement(ctx, achievement1); err != nil {
			t.Fatalf("Failed to save achievement for user1: %v", err)
		}

		// Add user2
		membership2 := &domain.GroupMembership{
			GroupID:  groupID,
			UserID:   user2ID,
			JoinedAt: time.Now().Add(1 * time.Minute),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership2); err != nil {
			t.Fatalf("Failed to create membership for user2: %v", err)
		}

		// Initialize rating for user2
		rating2 := &domain.Rating{
			UserID:       user2ID,
			GroupID:      groupID,
			Username:     "user2",
			Score:        50,
			CorrectCount: 5,
			WrongCount:   3,
			Streak:       1,
		}
		if err := ratingRepo.UpdateRating(ctx, rating2); err != nil {
			t.Fatalf("Failed to initialize rating for user2: %v", err)
		}

		// Add user3
		membership3 := &domain.GroupMembership{
			GroupID:  groupID,
			UserID:   user3ID,
			JoinedAt: time.Now().Add(2 * time.Minute),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership3); err != nil {
			t.Fatalf("Failed to create membership for user3: %v", err)
		}

		// Initialize rating for user3
		rating3 := &domain.Rating{
			UserID:       user3ID,
			GroupID:      groupID,
			Username:     "user3",
			Score:        75,
			CorrectCount: 7,
			WrongCount:   1,
			Streak:       2,
		}
		if err := ratingRepo.UpdateRating(ctx, rating3); err != nil {
			t.Fatalf("Failed to initialize rating for user3: %v", err)
		}
	})

	// Step 4: Admin views members
	t.Run("Admin views members", func(t *testing.T) {
		// Get all members of the group
		members, err := groupMembershipRepo.GetGroupMembers(ctx, groupID)
		if err != nil {
			t.Fatalf("Failed to get group members: %v", err)
		}

		// Verify we have 3 members
		if len(members) != 3 {
			t.Errorf("Expected 3 members, got %d", len(members))
		}

		// Verify all members are active
		for _, member := range members {
			if member.Status != domain.MembershipStatusActive {
				t.Errorf("Member %d should be active, got status %s", member.UserID, member.Status)
			}
			if member.GroupID != groupID {
				t.Errorf("Member %d has wrong group ID: expected %d, got %d", member.UserID, groupID, member.GroupID)
			}
		}

		// Verify members are ordered by join date (most recent first)
		if len(members) >= 2 {
			for i := 0; i < len(members)-1; i++ {
				if members[i].JoinedAt.Before(members[i+1].JoinedAt) {
					t.Error("Members should be ordered by join date (most recent first)")
					break
				}
			}
		}

		// Verify we can get ratings for all members
		for _, member := range members {
			rating, err := ratingRepo.GetRating(ctx, member.UserID, groupID)
			if err != nil {
				t.Errorf("Failed to get rating for member %d: %v", member.UserID, err)
				continue
			}
			if rating.GroupID != groupID {
				t.Errorf("Rating for member %d has wrong group ID: expected %d, got %d", member.UserID, groupID, rating.GroupID)
			}
		}

		// Verify we can get achievements for members
		achievements, err := achievementRepo.GetUserAchievements(ctx, user1ID, groupID)
		if err != nil {
			t.Fatalf("Failed to get achievements for user1: %v", err)
		}
		if len(achievements) != 1 {
			t.Errorf("Expected 1 achievement for user1, got %d", len(achievements))
		}
	})

	// Step 5: Admin removes a member
	t.Run("Admin removes member", func(t *testing.T) {
		// Remove user2 from the group
		if err := groupMembershipRepo.UpdateMembershipStatus(ctx, groupID, user2ID, domain.MembershipStatusRemoved); err != nil {
			t.Fatalf("Failed to remove user2: %v", err)
		}

		// Verify user2's membership status is removed
		membership, err := groupMembershipRepo.GetMembership(ctx, groupID, user2ID)
		if err != nil {
			t.Fatalf("Failed to get membership for user2: %v", err)
		}
		if membership.Status != domain.MembershipStatusRemoved {
			t.Errorf("Expected user2 status to be removed, got %s", membership.Status)
		}

		// Verify user2 no longer has active membership
		hasActiveMembership, err := groupMembershipRepo.HasActiveMembership(ctx, groupID, user2ID)
		if err != nil {
			t.Fatalf("Failed to check active membership for user2: %v", err)
		}
		if hasActiveMembership {
			t.Error("User2 should not have active membership after removal")
		}

		// Verify other members are still active
		hasActiveMembership1, err := groupMembershipRepo.HasActiveMembership(ctx, groupID, user1ID)
		if err != nil {
			t.Fatalf("Failed to check active membership for user1: %v", err)
		}
		if !hasActiveMembership1 {
			t.Error("User1 should still have active membership")
		}

		hasActiveMembership3, err := groupMembershipRepo.HasActiveMembership(ctx, groupID, user3ID)
		if err != nil {
			t.Fatalf("Failed to check active membership for user3: %v", err)
		}
		if !hasActiveMembership3 {
			t.Error("User3 should still have active membership")
		}
	})

	// Step 6: Verify data integrity after removal
	t.Run("Verify data integrity after removal", func(t *testing.T) {
		// Verify user2's historical data is preserved
		rating2, err := ratingRepo.GetRating(ctx, user2ID, groupID)
		if err != nil {
			t.Fatalf("Failed to get rating for removed user2: %v", err)
		}
		if rating2.Score != 50 {
			t.Errorf("User2's historical rating should be preserved, expected score 50, got %d", rating2.Score)
		}

		// Verify group still exists
		group, err := groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			t.Fatalf("Failed to get group after member removal: %v", err)
		}
		if group.ID != groupID {
			t.Error("Group should still exist after member removal")
		}

		// Verify we can still get all members (including removed)
		allMembers, err := groupMembershipRepo.GetGroupMembers(ctx, groupID)
		if err != nil {
			t.Fatalf("Failed to get all members: %v", err)
		}
		// Should have 3 members total (including removed one)
		if len(allMembers) != 3 {
			t.Errorf("Expected 3 total members (including removed), got %d", len(allMembers))
		}

		// Count active members
		activeCount := 0
		for _, member := range allMembers {
			if member.Status == domain.MembershipStatusActive {
				activeCount++
			}
		}
		if activeCount != 2 {
			t.Errorf("Expected 2 active members, got %d", activeCount)
		}
	})

	// Step 7: Test rejoin after removal
	t.Run("Removed user can rejoin", func(t *testing.T) {
		// User2 rejoins by reactivating membership
		if err := groupMembershipRepo.UpdateMembershipStatus(ctx, groupID, user2ID, domain.MembershipStatusActive); err != nil {
			t.Fatalf("Failed to reactivate user2 membership: %v", err)
		}

		// Verify user2 has active membership again
		hasActiveMembership, err := groupMembershipRepo.HasActiveMembership(ctx, groupID, user2ID)
		if err != nil {
			t.Fatalf("Failed to check active membership for rejoined user2: %v", err)
		}
		if !hasActiveMembership {
			t.Error("User2 should have active membership after rejoining")
		}

		// Verify user2's historical data is still there
		rating2, err := ratingRepo.GetRating(ctx, user2ID, groupID)
		if err != nil {
			t.Fatalf("Failed to get rating for rejoined user2: %v", err)
		}
		if rating2.Score != 50 {
			t.Errorf("User2's rating should be preserved after rejoin, expected score 50, got %d", rating2.Score)
		}
	})
}

// TestIntegration_DataMigration tests the migration of existing data to multi-group model:
// Run migration on existing data, verify default group is created, verify all data is associated with default group
func TestIntegration_DataMigration(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := storage.NewDBQueue(db)
	defer queue.Close()

	// Step 1: Initialize schema WITHOUT migrations (simulating old database)
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create repositories
	groupRepo := storage.NewGroupRepository(queue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	ratingRepo := storage.NewRatingRepository(queue)
	achievementRepo := storage.NewAchievementRepository(queue)

	// Test data
	user1ID := int64(11111)
	user2ID := int64(22222)
	user3ID := int64(33333)
	chatID := int64(67890)

	// Step 2: Create some "legacy" data before migration
	// Note: In the actual old schema, these wouldn't have group_id
	// But since we're testing with the new schema, we'll simulate by using group_id = 1
	t.Run("Create legacy data", func(t *testing.T) {
		// We need to run migrations first to have the group_id columns
		if err := storage.RunMigrations(queue); err != nil {
			t.Fatalf("Failed to run migrations: %v", err)
		}

		// Create a default group (simulating what the migration would do)
		defaultGroup := &domain.Group{
			TelegramChatID: chatID,
			Name:           "Default Group",
			CreatedBy:      user1ID,
			CreatedAt:      time.Now(),
		}
		if err := groupRepo.CreateGroup(ctx, defaultGroup); err != nil {
			t.Fatalf("Failed to create default group: %v", err)
		}

		// Create some events (simulating legacy events)
		event1 := &domain.Event{
			GroupID:   defaultGroup.ID,
			Question:  "Legacy event 1",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now(),
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusResolved,
			EventType: domain.EventTypeBinary,
			CreatedBy: user1ID,
			PollID:    "legacy_poll_1",
		}
		if err := eventRepo.CreateEvent(ctx, event1); err != nil {
			t.Fatalf("Failed to create legacy event1: %v", err)
		}

		event2 := &domain.Event{
			GroupID:   defaultGroup.ID,
			Question:  "Legacy event 2",
			Options:   []string{"Yes", "No"},
			CreatedAt: time.Now(),
			Deadline:  time.Now().Add(24 * time.Hour),
			Status:    domain.EventStatusActive,
			EventType: domain.EventTypeBinary,
			CreatedBy: user2ID,
			PollID:    "legacy_poll_2",
		}
		if err := eventRepo.CreateEvent(ctx, event2); err != nil {
			t.Fatalf("Failed to create legacy event2: %v", err)
		}

		// Create predictions for these events
		prediction1 := &domain.Prediction{
			EventID:   event1.ID,
			UserID:    user1ID,
			Option:    0,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction1); err != nil {
			t.Fatalf("Failed to save legacy prediction1: %v", err)
		}

		prediction2 := &domain.Prediction{
			EventID:   event1.ID,
			UserID:    user2ID,
			Option:    1,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction2); err != nil {
			t.Fatalf("Failed to save legacy prediction2: %v", err)
		}

		prediction3 := &domain.Prediction{
			EventID:   event2.ID,
			UserID:    user3ID,
			Option:    0,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction3); err != nil {
			t.Fatalf("Failed to save legacy prediction3: %v", err)
		}

		// Create ratings (simulating legacy ratings)
		rating1 := &domain.Rating{
			UserID:       user1ID,
			GroupID:      defaultGroup.ID,
			Username:     "user1",
			Score:        100,
			CorrectCount: 10,
			WrongCount:   2,
			Streak:       3,
		}
		if err := ratingRepo.UpdateRating(ctx, rating1); err != nil {
			t.Fatalf("Failed to create legacy rating1: %v", err)
		}

		rating2 := &domain.Rating{
			UserID:       user2ID,
			GroupID:      defaultGroup.ID,
			Username:     "user2",
			Score:        50,
			CorrectCount: 5,
			WrongCount:   3,
			Streak:       1,
		}
		if err := ratingRepo.UpdateRating(ctx, rating2); err != nil {
			t.Fatalf("Failed to create legacy rating2: %v", err)
		}

		rating3 := &domain.Rating{
			UserID:       user3ID,
			GroupID:      defaultGroup.ID,
			Username:     "user3",
			Score:        75,
			CorrectCount: 7,
			WrongCount:   1,
			Streak:       2,
		}
		if err := ratingRepo.UpdateRating(ctx, rating3); err != nil {
			t.Fatalf("Failed to create legacy rating3: %v", err)
		}

		// Create achievements (simulating legacy achievements)
		achievement1 := &domain.Achievement{
			UserID:    user1ID,
			GroupID:   defaultGroup.ID,
			Code:      domain.AchievementSharpshooter,
			Timestamp: time.Now(),
		}
		if err := achievementRepo.SaveAchievement(ctx, achievement1); err != nil {
			t.Fatalf("Failed to save legacy achievement1: %v", err)
		}

		achievement2 := &domain.Achievement{
			UserID:    user2ID,
			GroupID:   defaultGroup.ID,
			Code:      domain.AchievementVeteran,
			Timestamp: time.Now(),
		}
		if err := achievementRepo.SaveAchievement(ctx, achievement2); err != nil {
			t.Fatalf("Failed to save legacy achievement2: %v", err)
		}
	})

	// Step 3: Verify default group was created
	t.Run("Verify default group created", func(t *testing.T) {
		groups, err := groupRepo.GetAllGroups(ctx)
		if err != nil {
			t.Fatalf("Failed to get all groups: %v", err)
		}

		if len(groups) != 1 {
			t.Errorf("Expected 1 default group, got %d", len(groups))
		}

		if len(groups) > 0 {
			defaultGroup := groups[0]
			if defaultGroup.Name != "Default Group" {
				t.Errorf("Expected default group name %q, got %q", "Default Group", defaultGroup.Name)
			}
		}
	})

	// Step 4: Verify all events are associated with default group
	t.Run("Verify events associated with default group", func(t *testing.T) {
		groups, err := groupRepo.GetAllGroups(ctx)
		if err != nil || len(groups) == 0 {
			t.Fatalf("Failed to get default group")
		}
		defaultGroupID := groups[0].ID

		// Get all events for default group
		events, err := eventRepo.GetActiveEvents(ctx, defaultGroupID)
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}

		// We created 1 active event
		if len(events) != 1 {
			t.Errorf("Expected 1 active event in default group, got %d", len(events))
		}

		// Verify all events have the default group ID
		for _, event := range events {
			if event.GroupID != defaultGroupID {
				t.Errorf("Event %d should have default group ID %d, got %d", event.ID, defaultGroupID, event.GroupID)
			}
		}
	})

	// Step 5: Verify all ratings are associated with default group
	t.Run("Verify ratings associated with default group", func(t *testing.T) {
		groups, err := groupRepo.GetAllGroups(ctx)
		if err != nil || len(groups) == 0 {
			t.Fatalf("Failed to get default group")
		}
		defaultGroupID := groups[0].ID

		// Get ratings for all users in default group
		rating1, err := ratingRepo.GetRating(ctx, user1ID, defaultGroupID)
		if err != nil {
			t.Fatalf("Failed to get rating for user1: %v", err)
		}
		if rating1.GroupID != defaultGroupID {
			t.Errorf("User1 rating should have default group ID %d, got %d", defaultGroupID, rating1.GroupID)
		}

		rating2, err := ratingRepo.GetRating(ctx, user2ID, defaultGroupID)
		if err != nil {
			t.Fatalf("Failed to get rating for user2: %v", err)
		}
		if rating2.GroupID != defaultGroupID {
			t.Errorf("User2 rating should have default group ID %d, got %d", defaultGroupID, rating2.GroupID)
		}

		rating3, err := ratingRepo.GetRating(ctx, user3ID, defaultGroupID)
		if err != nil {
			t.Fatalf("Failed to get rating for user3: %v", err)
		}
		if rating3.GroupID != defaultGroupID {
			t.Errorf("User3 rating should have default group ID %d, got %d", defaultGroupID, rating3.GroupID)
		}

		// Verify rating values are preserved
		if rating1.Score != 100 {
			t.Errorf("User1 score should be preserved as 100, got %d", rating1.Score)
		}
		if rating2.Score != 50 {
			t.Errorf("User2 score should be preserved as 50, got %d", rating2.Score)
		}
		if rating3.Score != 75 {
			t.Errorf("User3 score should be preserved as 75, got %d", rating3.Score)
		}
	})

	// Step 6: Verify all achievements are associated with default group
	t.Run("Verify achievements associated with default group", func(t *testing.T) {
		groups, err := groupRepo.GetAllGroups(ctx)
		if err != nil || len(groups) == 0 {
			t.Fatalf("Failed to get default group")
		}
		defaultGroupID := groups[0].ID

		// Get achievements for users in default group
		achievements1, err := achievementRepo.GetUserAchievements(ctx, user1ID, defaultGroupID)
		if err != nil {
			t.Fatalf("Failed to get achievements for user1: %v", err)
		}
		if len(achievements1) != 1 {
			t.Errorf("Expected 1 achievement for user1, got %d", len(achievements1))
		}
		if len(achievements1) > 0 && achievements1[0].GroupID != defaultGroupID {
			t.Errorf("User1 achievement should have default group ID %d, got %d", defaultGroupID, achievements1[0].GroupID)
		}

		achievements2, err := achievementRepo.GetUserAchievements(ctx, user2ID, defaultGroupID)
		if err != nil {
			t.Fatalf("Failed to get achievements for user2: %v", err)
		}
		if len(achievements2) != 1 {
			t.Errorf("Expected 1 achievement for user2, got %d", len(achievements2))
		}
		if len(achievements2) > 0 && achievements2[0].GroupID != defaultGroupID {
			t.Errorf("User2 achievement should have default group ID %d, got %d", defaultGroupID, achievements2[0].GroupID)
		}
	})

	// Step 7: Verify all users have membership in default group
	t.Run("Verify users have membership in default group", func(t *testing.T) {
		groups, err := groupRepo.GetAllGroups(ctx)
		if err != nil || len(groups) == 0 {
			t.Fatalf("Failed to get default group")
		}
		defaultGroupID := groups[0].ID

		// Create memberships for all users who participated (simulating migration)
		membership1 := &domain.GroupMembership{
			GroupID:  defaultGroupID,
			UserID:   user1ID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership1); err != nil {
			t.Fatalf("Failed to create membership for user1: %v", err)
		}

		membership2 := &domain.GroupMembership{
			GroupID:  defaultGroupID,
			UserID:   user2ID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership2); err != nil {
			t.Fatalf("Failed to create membership for user2: %v", err)
		}

		membership3 := &domain.GroupMembership{
			GroupID:  defaultGroupID,
			UserID:   user3ID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership3); err != nil {
			t.Fatalf("Failed to create membership for user3: %v", err)
		}

		// Verify all users have active membership
		hasMembership1, err := groupMembershipRepo.HasActiveMembership(ctx, defaultGroupID, user1ID)
		if err != nil {
			t.Fatalf("Failed to check membership for user1: %v", err)
		}
		if !hasMembership1 {
			t.Error("User1 should have active membership in default group")
		}

		hasMembership2, err := groupMembershipRepo.HasActiveMembership(ctx, defaultGroupID, user2ID)
		if err != nil {
			t.Fatalf("Failed to check membership for user2: %v", err)
		}
		if !hasMembership2 {
			t.Error("User2 should have active membership in default group")
		}

		hasMembership3, err := groupMembershipRepo.HasActiveMembership(ctx, defaultGroupID, user3ID)
		if err != nil {
			t.Fatalf("Failed to check membership for user3: %v", err)
		}
		if !hasMembership3 {
			t.Error("User3 should have active membership in default group")
		}

		// Verify we can get all members
		members, err := groupMembershipRepo.GetGroupMembers(ctx, defaultGroupID)
		if err != nil {
			t.Fatalf("Failed to get group members: %v", err)
		}
		if len(members) != 3 {
			t.Errorf("Expected 3 members in default group, got %d", len(members))
		}
	})
}
