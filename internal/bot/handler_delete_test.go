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
	"github.com/go-telegram/bot"
	_ "modernc.org/sqlite"
)

func TestDeleteGroupCommand(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize DBQueue
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

	// Create test groups
	ctx := context.Background()
	group1 := &domain.Group{
		TelegramChatID: 12345,
		Name:           "Test Group 1",
		CreatedAt:      time.Now(),
		CreatedBy:      1,
		IsForum:        false,
	}
	if err := groupRepo.CreateGroup(ctx, group1); err != nil {
		t.Fatalf("Failed to create group 1: %v", err)
	}

	group2 := &domain.Group{
		TelegramChatID: 67890,
		Name:           "Test Forum",
		CreatedAt:      time.Now(),
		CreatedBy:      1,
		IsForum:        true,
	}
	if err := groupRepo.CreateGroup(ctx, group2); err != nil {
		t.Fatalf("Failed to create group 2: %v", err)
	}

	// Create forum topics for group2
	topic1 := &domain.ForumTopic{
		GroupID:         group2.ID,
		MessageThreadID: 100,
		Name:            "General",
		CreatedAt:       time.Now(),
		CreatedBy:       1,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic1); err != nil {
		t.Fatalf("Failed to create topic 1: %v", err)
	}

	topic2 := &domain.ForumTopic{
		GroupID:         group2.ID,
		MessageThreadID: 200,
		Name:            "Announcements",
		CreatedAt:       time.Now(),
		CreatedBy:       1,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic2); err != nil {
		t.Fatalf("Failed to create topic 2: %v", err)
	}

	// Create memberships
	membership1 := &domain.GroupMembership{
		GroupID:  group1.ID,
		UserID:   100,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := groupMembershipRepo.CreateMembership(ctx, membership1); err != nil {
		t.Fatalf("Failed to create membership 1: %v", err)
	}

	// Verify initial state
	groups, err := groupRepo.GetAllGroups(ctx)
	if err != nil {
		t.Fatalf("Failed to get all groups: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(groups))
	}

	topics, err := forumTopicRepo.GetForumTopicsByGroup(ctx, group2.ID)
	if err != nil {
		t.Fatalf("Failed to get topics: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("Expected 2 topics, got %d", len(topics))
	}

	// Test deleting a group
	if err := groupRepo.DeleteGroup(ctx, group1.ID); err != nil {
		t.Fatalf("Failed to delete group: %v", err)
	}

	// Verify group was deleted
	groups, err = groupRepo.GetAllGroups(ctx)
	if err != nil {
		t.Fatalf("Failed to get all groups after deletion: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("Expected 1 group after deletion, got %d", len(groups))
	}
	if groups[0].ID != group2.ID {
		t.Fatalf("Wrong group remained, expected ID %d, got %d", group2.ID, groups[0].ID)
	}

	// Test deleting a topic
	if err := forumTopicRepo.DeleteForumTopic(ctx, topic1.ID); err != nil {
		t.Fatalf("Failed to delete topic: %v", err)
	}

	// Verify topic was deleted
	topics, err = forumTopicRepo.GetForumTopicsByGroup(ctx, group2.ID)
	if err != nil {
		t.Fatalf("Failed to get topics after deletion: %v", err)
	}
	if len(topics) != 1 {
		t.Fatalf("Expected 1 topic after deletion, got %d", len(topics))
	}
	if topics[0].ID != topic2.ID {
		t.Fatalf("Wrong topic remained, expected ID %d, got %d", topic2.ID, topics[0].ID)
	}

	t.Log("✅ Delete operations work correctly")
}

func TestListGroupsWithTopics(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize DBQueue
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

	// Create test forum
	ctx := context.Background()
	forum := &domain.Group{
		TelegramChatID: 12345,
		Name:           "Test Forum",
		CreatedAt:      time.Now(),
		CreatedBy:      1,
		IsForum:        true,
	}
	if err := groupRepo.CreateGroup(ctx, forum); err != nil {
		t.Fatalf("Failed to create forum: %v", err)
	}

	// Create forum topics
	topic1 := &domain.ForumTopic{
		GroupID:         forum.ID,
		MessageThreadID: 100,
		Name:            "General Discussion",
		CreatedAt:       time.Now(),
		CreatedBy:       1,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic1); err != nil {
		t.Fatalf("Failed to create topic 1: %v", err)
	}

	topic2 := &domain.ForumTopic{
		GroupID:         forum.ID,
		MessageThreadID: 200,
		Name:            "Announcements",
		CreatedAt:       time.Now(),
		CreatedBy:       1,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic2); err != nil {
		t.Fatalf("Failed to create topic 2: %v", err)
	}

	// Create mock bot and handler
	mockBot := &bot.Bot{}
	cfg := &config.Config{
		AdminUserIDs: []int64{1},
	}
	log := logger.New(logger.INFO)

	handler := &BotHandler{
		bot:                 mockBot,
		groupRepo:           groupRepo,
		forumTopicRepo:      forumTopicRepo,
		groupMembershipRepo: groupMembershipRepo,
		config:              cfg,
		logger:              log,
	}

	// Verify topics can be retrieved
	topics, err := handler.forumTopicRepo.GetForumTopicsByGroup(ctx, forum.ID)
	if err != nil {
		t.Fatalf("Failed to get topics: %v", err)
	}

	if len(topics) != 2 {
		t.Fatalf("Expected 2 topics, got %d", len(topics))
	}

	// Verify topic details
	foundGeneral := false
	foundAnnouncements := false
	for _, topic := range topics {
		if topic.Name == "General Discussion" && topic.MessageThreadID == 100 {
			foundGeneral = true
		}
		if topic.Name == "Announcements" && topic.MessageThreadID == 200 {
			foundAnnouncements = true
		}
	}

	if !foundGeneral {
		t.Error("General Discussion topic not found")
	}
	if !foundAnnouncements {
		t.Error("Announcements topic not found")
	}

	t.Log("✅ List groups with topics works correctly")
}

func TestHandleDeleteGroupCallback(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize DBQueue
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

	// Create test group
	ctx := context.Background()
	group := &domain.Group{
		TelegramChatID: 12345,
		Name:           "Test Group",
		CreatedAt:      time.Now(),
		CreatedBy:      1,
		IsForum:        false,
	}
	if err := groupRepo.CreateGroup(ctx, group); err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Create mock bot
	mockBot := &bot.Bot{}
	cfg := &config.Config{
		AdminUserIDs: []int64{1},
	}
	log := logger.New(logger.INFO)

	handler := &BotHandler{
		bot:            mockBot,
		groupRepo:      groupRepo,
		forumTopicRepo: forumTopicRepo,
		config:         cfg,
		logger:         log,
	}

	// Test admin check
	if !handler.isAdmin(1) {
		t.Error("User 1 should be admin")
	}
	if handler.isAdmin(999) {
		t.Error("User 999 should not be admin")
	}

	// Verify group exists
	groups, err := groupRepo.GetAllGroups(ctx)
	if err != nil {
		t.Fatalf("Failed to get groups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}

	t.Log("✅ Delete group callback handler setup works correctly")
}

func TestHandleDeleteTopicCallback(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize DBQueue
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

	// Create test forum
	ctx := context.Background()
	forum := &domain.Group{
		TelegramChatID: 12345,
		Name:           "Test Forum",
		CreatedAt:      time.Now(),
		CreatedBy:      1,
		IsForum:        true,
	}
	if err := groupRepo.CreateGroup(ctx, forum); err != nil {
		t.Fatalf("Failed to create forum: %v", err)
	}

	// Create topic
	topic := &domain.ForumTopic{
		GroupID:         forum.ID,
		MessageThreadID: 100,
		Name:            "Test Topic",
		CreatedAt:       time.Now(),
		CreatedBy:       1,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic); err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	// Create mock bot
	mockBot := &bot.Bot{}
	cfg := &config.Config{
		AdminUserIDs: []int64{1},
	}
	log := logger.New(logger.INFO)

	handler := &BotHandler{
		bot:            mockBot,
		groupRepo:      groupRepo,
		forumTopicRepo: forumTopicRepo,
		config:         cfg,
		logger:         log,
	}

	// Verify topic exists
	topics, err := forumTopicRepo.GetForumTopicsByGroup(ctx, forum.ID)
	if err != nil {
		t.Fatalf("Failed to get topics: %v", err)
	}
	if len(topics) != 1 {
		t.Fatalf("Expected 1 topic, got %d", len(topics))
	}

	// Test admin check
	if !handler.isAdmin(1) {
		t.Error("User 1 should be admin")
	}

	t.Log("✅ Delete topic callback handler setup works correctly")
}

func TestRenameGroup(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize DBQueue
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

	// Create test group
	ctx := context.Background()
	group := &domain.Group{
		TelegramChatID: 12345,
		Name:           "Old Group Name",
		CreatedAt:      time.Now(),
		CreatedBy:      1,
		IsForum:        false,
		Status:         domain.GroupStatusActive,
	}
	if err := groupRepo.CreateGroup(ctx, group); err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Rename group
	newName := "New Group Name"
	if err := groupRepo.UpdateGroupName(ctx, group.ID, newName); err != nil {
		t.Fatalf("Failed to rename group: %v", err)
	}

	// Verify rename
	updatedGroup, err := groupRepo.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to get updated group: %v", err)
	}

	if updatedGroup.Name != newName {
		t.Errorf("Expected group name '%s', got '%s'", newName, updatedGroup.Name)
	}

	t.Log("✅ Group rename works correctly")
}

func TestRenameTopic(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize DBQueue
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

	// Create test forum
	ctx := context.Background()
	forum := &domain.Group{
		TelegramChatID: 12345,
		Name:           "Test Forum",
		CreatedAt:      time.Now(),
		CreatedBy:      1,
		IsForum:        true,
		Status:         domain.GroupStatusActive,
	}
	if err := groupRepo.CreateGroup(ctx, forum); err != nil {
		t.Fatalf("Failed to create forum: %v", err)
	}

	// Create topic
	topic := &domain.ForumTopic{
		GroupID:         forum.ID,
		MessageThreadID: 100,
		Name:            "Old Topic Name",
		CreatedAt:       time.Now(),
		CreatedBy:       1,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic); err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	// Rename topic
	newName := "New Topic Name"
	if err := forumTopicRepo.UpdateForumTopicName(ctx, topic.ID, newName); err != nil {
		t.Fatalf("Failed to rename topic: %v", err)
	}

	// Verify rename
	updatedTopic, err := forumTopicRepo.GetForumTopic(ctx, topic.ID)
	if err != nil {
		t.Fatalf("Failed to get updated topic: %v", err)
	}

	if updatedTopic.Name != newName {
		t.Errorf("Expected topic name '%s', got '%s'", newName, updatedTopic.Name)
	}

	t.Log("✅ Topic rename works correctly")
}

func TestSoftDeleteAndRestore(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize DBQueue
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

	// Create test group
	ctx := context.Background()
	group := &domain.Group{
		TelegramChatID: 12345,
		Name:           "Test Group",
		CreatedAt:      time.Now(),
		CreatedBy:      1,
		IsForum:        false,
		Status:         domain.GroupStatusActive,
	}
	if err := groupRepo.CreateGroup(ctx, group); err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Verify initial status
	if group.Status != domain.GroupStatusActive {
		t.Errorf("Expected status 'active', got '%s'", group.Status)
	}

	// Soft delete
	if err := groupRepo.UpdateGroupStatus(ctx, group.ID, domain.GroupStatusDeleted); err != nil {
		t.Fatalf("Failed to soft delete group: %v", err)
	}

	// Verify deleted status
	deletedGroup, err := groupRepo.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to get deleted group: %v", err)
	}
	if deletedGroup.Status != domain.GroupStatusDeleted {
		t.Errorf("Expected status 'deleted', got '%s'", deletedGroup.Status)
	}

	// Restore
	if err := groupRepo.UpdateGroupStatus(ctx, group.ID, domain.GroupStatusActive); err != nil {
		t.Fatalf("Failed to restore group: %v", err)
	}

	// Verify restored status
	restoredGroup, err := groupRepo.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to get restored group: %v", err)
	}
	if restoredGroup.Status != domain.GroupStatusActive {
		t.Errorf("Expected status 'active', got '%s'", restoredGroup.Status)
	}

	t.Log("✅ Soft delete and restore work correctly")
}
