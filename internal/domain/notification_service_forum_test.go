package domain

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// TestPublishEventResults_ForumGroup tests that MessageThreadID is correctly passed for forum groups
func TestPublishEventResults_ForumGroup(t *testing.T) {
	// Create mock dependencies
	mockBot := &MockForumBot{
		sentMessages: make([]MockForumMessage, 0),
	}

	// Create mock event
	event := &Event{
		ID:       1,
		GroupID:  1,
		Question: "Test question",
		Options:  []string{"Option 0", "Option 1"},
	}

	// Create predictions
	predictions := []*Prediction{
		{ID: 1, EventID: 1, UserID: 1, Option: 0},
		{ID: 2, EventID: 1, UserID: 2, Option: 1},
	}

	mockEventRepo := &MockEventRepoWithData{event: event}
	mockPredictionRepo := &MockPredictionRepoWithData{predictions: predictions}
	mockRatingRepo := &MockRatingRepo{}
	mockReminderRepo := &MockReminderRepo{}
	mockLogger := &MockLogger{}

	// Create notification service
	ns := NewNotificationService(
		mockBot,
		mockEventRepo,
		mockPredictionRepo,
		mockRatingRepo,
		mockReminderRepo,
		mockLogger,
	)

	// Test with forum group (MessageThreadID set)
	ctx := context.Background()
	telegramChatID := int64(12345)
	messageThreadID := 67890
	err := ns.PublishEventResults(ctx, 1, 0, telegramChatID, &messageThreadID)
	if err != nil {
		t.Fatalf("PublishEventResults failed: %v", err)
	}

	// Verify message was sent
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(mockBot.sentMessages))
	}

	// Verify MessageThreadID was set
	if mockBot.sentMessages[0].MessageThreadID == nil {
		t.Fatal("Expected MessageThreadID to be set, but it was nil")
	}

	if *mockBot.sentMessages[0].MessageThreadID != messageThreadID {
		t.Fatalf("Expected MessageThreadID %d, got %d", messageThreadID, *mockBot.sentMessages[0].MessageThreadID)
	}

	// Test with regular group (MessageThreadID nil)
	mockBot.sentMessages = make([]MockForumMessage, 0)
	err = ns.PublishEventResults(ctx, 1, 0, telegramChatID, nil)
	if err != nil {
		t.Fatalf("PublishEventResults failed: %v", err)
	}

	// Verify message was sent
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(mockBot.sentMessages))
	}

	// Verify MessageThreadID was not set
	if mockBot.sentMessages[0].MessageThreadID != nil {
		t.Fatalf("Expected MessageThreadID to be nil, but got %d", *mockBot.sentMessages[0].MessageThreadID)
	}

	// Test with MessageThreadID = 0 (should not be set)
	mockBot.sentMessages = make([]MockForumMessage, 0)
	zeroThreadID := 0
	err = ns.PublishEventResults(ctx, 1, 0, telegramChatID, &zeroThreadID)
	if err != nil {
		t.Fatalf("PublishEventResults failed: %v", err)
	}

	// Verify message was sent
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(mockBot.sentMessages))
	}

	// Verify MessageThreadID was not set (0 should be treated as nil)
	if mockBot.sentMessages[0].MessageThreadID != nil {
		t.Fatalf("Expected MessageThreadID to be nil for zero value, but got %d", *mockBot.sentMessages[0].MessageThreadID)
	}
}

// MockForumBot is a mock bot that tracks MessageThreadID
type MockForumBot struct {
	sentMessages []MockForumMessage
}

type MockForumMessage struct {
	ChatID          int64
	Text            string
	MessageThreadID *int
}

func (m *MockForumBot) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	var threadID *int
	if params.MessageThreadID != 0 {
		tid := params.MessageThreadID
		threadID = &tid
	}
	msg := MockForumMessage{
		ChatID:          params.ChatID.(int64),
		Text:            params.Text,
		MessageThreadID: threadID,
	}
	m.sentMessages = append(m.sentMessages, msg)
	return &models.Message{}, nil
}
