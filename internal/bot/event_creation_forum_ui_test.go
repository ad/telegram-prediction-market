package bot

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	_ "modernc.org/sqlite"
)

// Test that verifies the UI structure for forum groups with topics
func TestEventCreation_ForumUIStructure(t *testing.T) {
	ctx := context.Background()
	userID := int64(12345)
	chatID := int64(67890)

	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

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

	// Create a regular group
	regularGroup := &domain.Group{
		TelegramChatID: chatID + 1,
		Name:           "Regular Group",
		CreatedBy:      userID,
		CreatedAt:      time.Now(),
		IsForum:        false,
	}
	if err := groupRepo.CreateGroup(ctx, regularGroup); err != nil {
		t.Fatalf("Failed to create regular group: %v", err)
	}

	// Add user to both groups
	for _, groupID := range []int64{forumGroup.ID, regularGroup.ID} {
		membership := &domain.GroupMembership{
			GroupID:  groupID,
			UserID:   userID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, membership); err != nil {
			t.Fatalf("Failed to create membership for group %d: %v", groupID, err)
		}
	}

	// Create forum topics
	topic1 := &domain.ForumTopic{
		GroupID:         forumGroup.ID,
		MessageThreadID: 432,
		Name:            "General Discussion",
		CreatedAt:       time.Now(),
		CreatedBy:       userID,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic1); err != nil {
		t.Fatalf("Failed to create topic 1: %v", err)
	}

	topic2 := &domain.ForumTopic{
		GroupID:         forumGroup.ID,
		MessageThreadID: 471,
		Name:            "Announcements",
		CreatedAt:       time.Now(),
		CreatedBy:       userID,
	}
	if err := forumTopicRepo.CreateForumTopic(ctx, topic2); err != nil {
		t.Fatalf("Failed to create topic 2: %v", err)
	}

	// Simulate the button generation logic from handleSelectGroup
	groupContextResolver := domain.NewGroupContextResolver(groupRepo)
	groups, err := groupContextResolver.GetUserGroupChoices(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get user group choices: %v", err)
	}

	// Expected button structure:
	// 1. "Test Forum" (main forum button, no thread)
	// 2. "  ↳ General Discussion" (topic 1)
	// 3. "  ↳ Announcements" (topic 2)
	// 4. "Regular Group" (regular group)

	type Button struct {
		Text         string
		CallbackData string
	}

	var buttons []Button

	for _, group := range groups {
		if group.IsForum {
			// Add main group button
			buttons = append(buttons, Button{
				Text:         group.Name,
				CallbackData: fmt.Sprintf("select_group:%d", group.ID),
			})

			// Get forum topics
			topics, err := forumTopicRepo.GetForumTopicsByGroup(ctx, group.ID)
			if err != nil {
				t.Fatalf("Failed to get forum topics: %v", err)
			}

			// Add topic buttons
			for _, topic := range topics {
				buttons = append(buttons, Button{
					Text:         fmt.Sprintf("  ↳ %s", topic.Name),
					CallbackData: fmt.Sprintf("select_group:%d:%d", group.ID, topic.MessageThreadID),
				})
			}
		} else {
			// Regular group
			buttons = append(buttons, Button{
				Text:         group.Name,
				CallbackData: fmt.Sprintf("select_group:%d", group.ID),
			})
		}
	}

	// Verify button count
	expectedButtonCount := 4 // 1 forum + 2 topics + 1 regular
	if len(buttons) != expectedButtonCount {
		t.Fatalf("Expected %d buttons, got %d", expectedButtonCount, len(buttons))
	}

	// Verify button structure (order may vary)
	foundForumMain := false
	foundTopic1 := false
	foundTopic2 := false
	foundRegular := false

	for _, btn := range buttons {
		switch {
		case btn.Text == "Test Forum" && btn.CallbackData == fmt.Sprintf("select_group:%d", forumGroup.ID):
			foundForumMain = true
			t.Logf("✓ Found forum main button: '%s' -> %s", btn.Text, btn.CallbackData)
		case btn.Text == "  ↳ General Discussion" && btn.CallbackData == fmt.Sprintf("select_group:%d:432", forumGroup.ID):
			foundTopic1 = true
			t.Logf("✓ Found topic 1 button: '%s' -> %s", btn.Text, btn.CallbackData)
		case btn.Text == "  ↳ Announcements" && btn.CallbackData == fmt.Sprintf("select_group:%d:471", forumGroup.ID):
			foundTopic2 = true
			t.Logf("✓ Found topic 2 button: '%s' -> %s", btn.Text, btn.CallbackData)
		case btn.Text == "Regular Group" && btn.CallbackData == fmt.Sprintf("select_group:%d", regularGroup.ID):
			foundRegular = true
			t.Logf("✓ Found regular group button: '%s' -> %s", btn.Text, btn.CallbackData)
		default:
			t.Logf("? Unexpected button: '%s' -> %s", btn.Text, btn.CallbackData)
		}
	}

	if !foundForumMain {
		t.Error("❌ Forum main button not found")
	}
	if !foundTopic1 {
		t.Error("❌ Topic 1 button not found")
	}
	if !foundTopic2 {
		t.Error("❌ Topic 2 button not found")
	}
	if !foundRegular {
		t.Error("❌ Regular group button not found")
	}

	if foundForumMain && foundTopic1 && foundTopic2 && foundRegular {
		t.Log("✅ UI structure is correct:")
		t.Log("   • Forum main button (no thread)")
		t.Log("   • Forum topic 1 (with ↳ symbol)")
		t.Log("   • Forum topic 2 (with ↳ symbol)")
		t.Log("   • Regular group button")
	}
}
