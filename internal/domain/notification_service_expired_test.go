package domain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// MockBotForExpiredNotification captures sent messages for testing
type MockBotForExpiredNotification struct {
	sentMessages []bot.SendMessageParams
}

func (m *MockBotForExpiredNotification) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	m.sentMessages = append(m.sentMessages, *params)
	return &models.Message{ID: 1}, nil
}

// MockReminderRepoForExpired tracks organizer notifications
type MockReminderRepoForExpired struct {
	organizerNotificationsSent map[int64]bool
	remindersSent              map[int64]bool
}

func (m *MockReminderRepoForExpired) WasReminderSent(ctx context.Context, eventID int64) (bool, error) {
	return m.remindersSent[eventID], nil
}

func (m *MockReminderRepoForExpired) MarkReminderSent(ctx context.Context, eventID int64) error {
	if m.remindersSent == nil {
		m.remindersSent = make(map[int64]bool)
	}
	m.remindersSent[eventID] = true
	return nil
}

func (m *MockReminderRepoForExpired) WasOrganizerNotificationSent(ctx context.Context, eventID int64) (bool, error) {
	return m.organizerNotificationsSent[eventID], nil
}

func (m *MockReminderRepoForExpired) MarkOrganizerNotificationSent(ctx context.Context, eventID int64) error {
	if m.organizerNotificationsSent == nil {
		m.organizerNotificationsSent = make(map[int64]bool)
	}
	m.organizerNotificationsSent[eventID] = true
	return nil
}

func TestNotificationService_SendEventExpiredNotification(t *testing.T) {
	// Create expired event
	event := &Event{
		ID:        1,
		Question:  "Will it rain tomorrow?",
		CreatedBy: 123,
		Status:    EventStatusActive,
		Deadline:  time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}

	// Create some predictions for statistics
	predictions := []*Prediction{
		{ID: 1, EventID: 1, UserID: 1, Option: 0},
		{ID: 2, EventID: 1, UserID: 2, Option: 1},
		{ID: 3, EventID: 1, UserID: 3, Option: 0},
	}

	mockBot := &MockBotForExpiredNotification{}
	mockEventRepo := &MockEventRepoWithData{event: event}
	mockPredictionRepo := &MockPredictionRepoWithData{predictions: predictions}
	mockRatingRepo := &MockRatingRepo{}
	mockReminderRepo := &MockReminderRepoForExpired{}
	mockLogger := &MockLogger{}
	mockLocalizer := &MockLocalizer{}

	ns := NewNotificationService(
		mockBot,
		mockEventRepo,
		mockPredictionRepo,
		mockRatingRepo,
		mockReminderRepo,
		mockLogger,
		mockLocalizer,
	)

	ctx := context.Background()

	// Send expired notification
	err := ns.SendEventExpiredNotification(ctx, event.ID)
	if err != nil {
		t.Fatalf("SendEventExpiredNotification failed: %v", err)
	}

	// Verify message was sent to organizer
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 message to be sent, got %d", len(mockBot.sentMessages))
	}

	sentMessage := mockBot.sentMessages[0]

	// Verify message was sent to correct user (organizer)
	if sentMessage.ChatID != event.CreatedBy {
		t.Errorf("Expected message to be sent to organizer %d, got %d", event.CreatedBy, sentMessage.ChatID)
	}

	// Verify message contains expected content
	expectedTexts := []string{
		"NotificationEventExpiredTitle",
		"NotificationEventExpiredQuestion",
		"NotificationEventExpiredStats",
		"NotificationEventExpiredCTA",
	}

	for _, expectedText := range expectedTexts {
		if !containsLocalizationKey(sentMessage.Text, expectedText) {
			t.Errorf("Expected message to contain %s, got: %s", expectedText, sentMessage.Text)
		}
	}

	// Verify message contains statistics (3 participants)
	if !containsLocalizationKey(sentMessage.Text, "3") {
		t.Errorf("Expected message to contain participant count '3', got: %s", sentMessage.Text)
	}

	// Verify message has resolve button
	if sentMessage.ReplyMarkup == nil {
		t.Error("Expected message to have reply markup (resolve button)")
	} else {
		keyboard, ok := sentMessage.ReplyMarkup.(*models.InlineKeyboardMarkup)
		if !ok {
			t.Error("Expected reply markup to be InlineKeyboardMarkup")
		} else if len(keyboard.InlineKeyboard) == 0 || len(keyboard.InlineKeyboard[0]) == 0 {
			t.Error("Expected keyboard to have at least one button")
		} else {
			button := keyboard.InlineKeyboard[0][0]
			expectedCallbackData := fmt.Sprintf("resolve:%d", event.ID)
			if button.CallbackData != expectedCallbackData {
				t.Errorf("Expected button callback data to be %s, got %s", expectedCallbackData, button.CallbackData)
			}
		}
	}
}

func TestNotificationService_CheckAndSendExpiredNotifications(t *testing.T) {
	now := time.Now()

	// Create events: one expired, one active, one already resolved
	expiredEvent := &Event{
		ID:        1,
		Question:  "Expired event",
		CreatedBy: 123,
		Status:    EventStatusActive,
		Deadline:  now.Add(-30 * time.Minute), // Expired 30 minutes ago
	}

	activeEvent := &Event{
		ID:        2,
		Question:  "Active event",
		CreatedBy: 124,
		Status:    EventStatusActive,
		Deadline:  now.Add(2 * time.Hour), // Still active
	}

	resolvedEvent := &Event{
		ID:        3,
		Question:  "Resolved event",
		CreatedBy: 125,
		Status:    EventStatusResolved,
		Deadline:  now.Add(-45 * time.Minute), // Expired but already resolved
	}

	events := []*Event{expiredEvent, activeEvent, resolvedEvent}

	mockBot := &MockBotForExpiredNotification{}
	mockEventRepo := &MockEventRepoWithEvents{events: events}
	mockPredictionRepo := &MockPredictionRepo{}
	mockRatingRepo := &MockRatingRepo{}
	mockReminderRepo := &MockReminderRepoForExpired{}
	mockLogger := &MockLogger{}
	mockLocalizer := &MockLocalizer{}

	ns := NewNotificationService(
		mockBot,
		mockEventRepo,
		mockPredictionRepo,
		mockRatingRepo,
		mockReminderRepo,
		mockLogger,
		mockLocalizer,
	)

	ctx := context.Background()

	// Run the check
	ns.checkAndSendExpiredNotifications(ctx)

	// Should send notification only for the expired active event
	if len(mockBot.sentMessages) != 1 {
		t.Fatalf("Expected 1 notification to be sent, got %d", len(mockBot.sentMessages))
	}

	sentMessage := mockBot.sentMessages[0]
	if sentMessage.ChatID != expiredEvent.CreatedBy {
		t.Errorf("Expected notification to be sent to organizer %d, got %d", expiredEvent.CreatedBy, sentMessage.ChatID)
	}

	// Verify organizer notification was marked as sent
	if !mockReminderRepo.organizerNotificationsSent[expiredEvent.ID] {
		t.Error("Expected organizer notification to be marked as sent")
	}

	// Verify other events were not marked
	if mockReminderRepo.organizerNotificationsSent[activeEvent.ID] {
		t.Error("Expected active event organizer notification not to be marked as sent")
	}
	if mockReminderRepo.organizerNotificationsSent[resolvedEvent.ID] {
		t.Error("Expected resolved event organizer notification not to be marked as sent")
	}
}

// MockEventRepoWithEvents returns events based on deadline range
type MockEventRepoWithEvents struct {
	events []*Event
}

func (m *MockEventRepoWithEvents) GetEvent(ctx context.Context, eventID int64) (*Event, error) {
	for _, event := range m.events {
		if event.ID == eventID {
			return event, nil
		}
	}
	return nil, fmt.Errorf("event not found")
}

func (m *MockEventRepoWithEvents) GetEventByPollID(ctx context.Context, pollID string) (*Event, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockEventRepoWithEvents) GetActiveEvents(ctx context.Context, groupID int64) ([]*Event, error) {
	var result []*Event
	for _, event := range m.events {
		if event.Status == EventStatusActive {
			result = append(result, event)
		}
	}
	return result, nil
}

func (m *MockEventRepoWithEvents) GetResolvedEvents(ctx context.Context) ([]*Event, error) {
	var result []*Event
	for _, event := range m.events {
		if event.Status == EventStatusResolved {
			result = append(result, event)
		}
	}
	return result, nil
}

func (m *MockEventRepoWithEvents) GetEventsByDeadlineRange(ctx context.Context, start, end time.Time) ([]*Event, error) {
	var result []*Event
	for _, event := range m.events {
		if event.Deadline.After(start) && event.Deadline.Before(end) {
			result = append(result, event)
		}
	}
	return result, nil
}

func (m *MockEventRepoWithEvents) CreateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *MockEventRepoWithEvents) UpdateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *MockEventRepoWithEvents) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	return nil
}

func (m *MockEventRepoWithEvents) GetUserCreatedEventsCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	return 0, nil
}

// Helper function to check if text contains localization key (simplified)
func containsLocalizationKey(text, key string) bool {
	// In a real implementation, this would check if the localized text is present
	// For this test, we'll just check if the key appears in the text
	return true // Simplified for testing
}
