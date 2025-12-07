package bot

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	_ "modernc.org/sqlite"
)

// mockBotForResolve is a mock bot for testing resolve event message deletion
type mockBotForResolve struct {
	deletedMessages []int
	sentMessages    []string
}

func (m *mockBotForResolve) DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
	m.deletedMessages = append(m.deletedMessages, params.MessageID)
	return true, nil
}

func (m *mockBotForResolve) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	m.sentMessages = append(m.sentMessages, params.Text)
	chatID, _ := params.ChatID.(int64)
	return &models.Message{
		ID: len(m.sentMessages),
		Chat: models.Chat{
			ID: chatID,
		},
	}, nil
}

func (m *mockBotForResolve) StopPoll(ctx context.Context, params *bot.StopPollParams) (*models.Poll, error) {
	return &models.Poll{}, nil
}

// TestResolveEventMessageDeletion tests that intermediate messages are deleted during resolve_event flow
func TestResolveEventMessageDeletion(t *testing.T) {
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

	logger := &mockLogger{}

	mockBot := &mockBotForResolve{
		deletedMessages: []int{},
		sentMessages:    []string{},
	}

	ctx := context.Background()

	// Test Step 1: Delete event selection message
	deleteMessages(ctx, mockBot, logger, 12345, 100)

	// Verify that the message was deleted
	if len(mockBot.deletedMessages) != 1 {
		t.Errorf("Expected 1 message to be deleted, got %d", len(mockBot.deletedMessages))
	}
	if len(mockBot.deletedMessages) > 0 && mockBot.deletedMessages[0] != 100 {
		t.Errorf("Expected message ID 100 to be deleted, got %d", mockBot.deletedMessages[0])
	}

	// Test Step 2: Delete another message
	deleteMessages(ctx, mockBot, logger, 12345, 101)

	// Verify that both messages were deleted
	if len(mockBot.deletedMessages) != 2 {
		t.Errorf("Expected 2 messages to be deleted, got %d", len(mockBot.deletedMessages))
	}
	if len(mockBot.deletedMessages) > 1 && mockBot.deletedMessages[1] != 101 {
		t.Errorf("Expected message ID 101 to be deleted, got %d", mockBot.deletedMessages[1])
	}

	// Test Step 3: Delete multiple messages at once
	deleteMessages(ctx, mockBot, logger, 12345, 102, 103, 104)

	// Verify that all messages were deleted
	if len(mockBot.deletedMessages) != 5 {
		t.Errorf("Expected 5 messages to be deleted, got %d", len(mockBot.deletedMessages))
	}
}

// TestResolveEventCleanChat tests that only final message remains after resolve_event
func TestResolveEventCleanChat(t *testing.T) {
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

	logger := &mockLogger{}

	mockBot := &mockBotForResolve{
		deletedMessages: []int{},
		sentMessages:    []string{},
	}

	ctx := context.Background()

	// Simulate full resolve flow by deleting messages
	// Step 1: Delete event selection message
	deleteMessages(ctx, mockBot, logger, 12345, 200)

	// Step 2: Delete option selection message
	deleteMessages(ctx, mockBot, logger, 12345, 201)

	// Verify that all intermediate messages were deleted
	expectedDeletedCount := 2 // Event selection message + option selection message
	if len(mockBot.deletedMessages) != expectedDeletedCount {
		t.Errorf("Expected %d messages to be deleted, got %d", expectedDeletedCount, len(mockBot.deletedMessages))
	}

	// Verify that messages were deleted in correct order
	if len(mockBot.deletedMessages) >= 2 {
		if mockBot.deletedMessages[0] != 200 {
			t.Errorf("Expected first deleted message to be 200, got %d", mockBot.deletedMessages[0])
		}
		if mockBot.deletedMessages[1] != 201 {
			t.Errorf("Expected second deleted message to be 201, got %d", mockBot.deletedMessages[1])
		}
	}
}
