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

	// Test with forum group (event has ForumTopicID)
	messageThreadID := 67890
	forumTopicID := int64(1)
	event := &Event{
		ID:           1,
		GroupID:      1,
		ForumTopicID: &forumTopicID,
		Question:     "Test question",
		Options:      []string{"Option 0", "Option 1"},
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

	// Create mock forum topic repo with a topic
	mockForumTopicRepo := &MockForumTopicRepo{
		topics: map[int64]*ForumTopic{
			1: {
				ID:              1,
				GroupID:         1,
				MessageThreadID: messageThreadID,
				Name:            "Test Topic",
			},
		},
	}

	// Create notification service
	ns := NewNotificationService(
		mockBot,
		mockEventRepo,
		mockPredictionRepo,
		mockRatingRepo,
		mockReminderRepo,
		mockLogger,
	)

	ctx := context.Background()
	telegramChatID := int64(12345)
	err := ns.PublishEventResults(ctx, 1, 0, telegramChatID, mockForumTopicRepo)
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

	// Test with regular group (no ForumTopicID)
	mockBot.sentMessages = make([]MockForumMessage, 0)
	eventNoForum := &Event{
		ID:           2,
		GroupID:      1,
		ForumTopicID: nil,
		Question:     "Test question",
		Options:      []string{"Option 0", "Option 1"},
	}
	mockEventRepo.event = eventNoForum

	err = ns.PublishEventResults(ctx, 2, 0, telegramChatID, mockForumTopicRepo)
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

// MockForumTopicRepo is a mock forum topic repository
type MockForumTopicRepo struct {
	topics map[int64]*ForumTopic
}

func (m *MockForumTopicRepo) CreateForumTopic(ctx context.Context, topic *ForumTopic) error {
	topic.ID = int64(len(m.topics) + 1)
	m.topics[topic.ID] = topic
	return nil
}

func (m *MockForumTopicRepo) GetForumTopic(ctx context.Context, topicID int64) (*ForumTopic, error) {
	if topic, ok := m.topics[topicID]; ok {
		return topic, nil
	}
	return nil, nil
}

func (m *MockForumTopicRepo) GetForumTopicByGroupAndThread(ctx context.Context, groupID int64, messageThreadID int) (*ForumTopic, error) {
	for _, topic := range m.topics {
		if topic.GroupID == groupID && topic.MessageThreadID == messageThreadID {
			return topic, nil
		}
	}
	return nil, nil
}

func (m *MockForumTopicRepo) GetForumTopicsByGroup(ctx context.Context, groupID int64) ([]*ForumTopic, error) {
	var topics []*ForumTopic
	for _, topic := range m.topics {
		if topic.GroupID == groupID {
			topics = append(topics, topic)
		}
	}
	return topics, nil
}

func (m *MockForumTopicRepo) DeleteForumTopic(ctx context.Context, topicID int64) error {
	delete(m.topics, topicID)
	return nil
}
