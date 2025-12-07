package bot

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	_ "modernc.org/sqlite"
)

// Test that forum topics are correctly stored and can be retrieved
func TestEventCreation_ForumTopicsDisplayed(t *testing.T) {
	ctx := context.Background()
	userID := int64(12345)
	chatID := int64(67890)

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

	// Create repositories
	groupRepo := storage.NewGroupRepository(queue)
	forumTopicRepo := storage.NewForumTopicRepository(queue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(queue)

	// Create a forum group
	forumGroup := &domain.Group{
		TelegramChatID: chatID,
		Name:           "Test Forum",
		CreatedBy:      userID,
		CreatedAt:      time.Now(),
		IsForum:        true,
	}
	if err := groupRepo.CreateGroup(ctx, forumGroup); err != nil {
		t.Fatalf("Failed to create forum group: %v", err)
	}

	// Add user to group
	membership := &domain.GroupMembership{
		GroupID:  forumGroup.ID,
		UserID:   userID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := groupMembershipRepo.CreateMembership(ctx, membership); err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Create forum topics
	topic1 := &domain.ForumTopic{
		GroupID:         forumGroup.ID,
		MessageThreadID: 432,
		Name:            "Topic 1",
		CreatedAt:       time.Now(),
		CreatedBy:       userID,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic1); err != nil {
		t.Fatalf("Failed to create topic 1: %v", err)
	}

	topic2 := &domain.ForumTopic{
		GroupID:         forumGroup.ID,
		MessageThreadID: 471,
		Name:            "Topic 2",
		CreatedAt:       time.Now(),
		CreatedBy:       userID,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic2); err != nil {
		t.Fatalf("Failed to create topic 2: %v", err)
	}

	// Verify topics were created
	topics, err := forumTopicRepo.GetForumTopicsByGroup(ctx, forumGroup.ID)
	if err != nil {
		t.Fatalf("Failed to get forum topics: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("Expected 2 topics, got %d", len(topics))
	}

	// Verify topics can be retrieved
	retrievedTopics, err := forumTopicRepo.GetForumTopicsByGroup(ctx, forumGroup.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve forum topics: %v", err)
	}

	if len(retrievedTopics) != 2 {
		t.Fatalf("Expected 2 topics, got %d", len(retrievedTopics))
	}

	// Verify topic details
	topicNames := make(map[string]int)
	topicThreadIDs := make(map[int]bool)

	for _, topic := range retrievedTopics {
		topicNames[topic.Name] = topic.MessageThreadID
		topicThreadIDs[topic.MessageThreadID] = true

		if topic.GroupID != forumGroup.ID {
			t.Errorf("Expected topic group_id %d, got %d", forumGroup.ID, topic.GroupID)
		}
	}

	// Verify both topics are present
	if threadID, ok := topicNames["Topic 1"]; !ok {
		t.Error("Expected to find Topic 1")
	} else if threadID != 432 {
		t.Errorf("Expected Topic 1 thread ID 432, got %d", threadID)
	}

	if threadID, ok := topicNames["Topic 2"]; !ok {
		t.Error("Expected to find Topic 2")
	} else if threadID != 471 {
		t.Errorf("Expected Topic 2 thread ID 471, got %d", threadID)
	}

	// Verify thread IDs
	if !topicThreadIDs[432] {
		t.Error("Expected thread ID 432 to be present")
	}
	if !topicThreadIDs[471] {
		t.Error("Expected thread ID 471 to be present")
	}

	// Test the group context resolver
	groupContextResolver := domain.NewGroupContextResolver(groupRepo)
	userGroups, err := groupContextResolver.GetUserGroupChoices(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get user group choices: %v", err)
	}

	if len(userGroups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(userGroups))
	}

	if userGroups[0].ID != forumGroup.ID {
		t.Errorf("Expected group ID %d, got %d", forumGroup.ID, userGroups[0].ID)
	}

	if !userGroups[0].IsForum {
		t.Error("Expected group to be marked as forum")
	}
}
