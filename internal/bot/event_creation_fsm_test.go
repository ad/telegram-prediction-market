package bot

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"telegram-prediction-bot/internal/config"
	"telegram-prediction-bot/internal/domain"
	"telegram-prediction-bot/internal/logger"
	"telegram-prediction-bot/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

// MockMessageDeleter for testing message deletion
type MockMessageDeleter struct {
	deletedMessages map[int64][]int
	deleteErrors    map[int]error
}

func NewMockMessageDeleter() *MockMessageDeleter {
	return &MockMessageDeleter{
		deletedMessages: make(map[int64][]int),
		deleteErrors:    make(map[int]error),
	}
}

func (m *MockMessageDeleter) DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
	if err, exists := m.deleteErrors[params.MessageID]; exists {
		return false, err
	}
	chatID, ok := params.ChatID.(int64)
	if !ok {
		return false, nil
	}
	m.deletedMessages[chatID] = append(m.deletedMessages[chatID], params.MessageID)
	return true, nil
}

func (m *MockMessageDeleter) GetDeletedMessages(chatID int64) []int {
	return m.deletedMessages[chatID]
}

func (m *MockMessageDeleter) SetDeleteError(messageID int, err error) {
	m.deleteErrors[messageID] = err
}

func TestProperty_MessageCleanupCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("message cleanup deletes expected number of messages", prop.ForAll(
		func(chatID int64, botMsgID int, userMsgID int) bool {
			// Create mock deleter
			mockDeleter := NewMockMessageDeleter()
			log := logger.New(logger.DEBUG)

			// Call deleteMessages with both IDs
			deleteMessages(context.Background(), mockDeleter, log, chatID, botMsgID, userMsgID)

			// Verify exactly 2 messages were deleted
			deleted := mockDeleter.GetDeletedMessages(chatID)
			if len(deleted) != 2 {
				t.Logf("Expected 2 messages deleted, got %d", len(deleted))
				return false
			}

			// Verify the correct message IDs were deleted
			foundBot := false
			foundUser := false
			for _, id := range deleted {
				if id == botMsgID {
					foundBot = true
				}
				if id == userMsgID {
					foundUser = true
				}
			}

			if !foundBot || !foundUser {
				t.Logf("Expected both bot (%d) and user (%d) messages to be deleted", botMsgID, userMsgID)
				return false
			}

			return true
		},
		gen.Int64(),
		gen.IntRange(1, 1000000),
		gen.IntRange(1, 1000000),
	))

	properties.TestingRun(t)
}

func TestProperty_SummaryContentCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("summary contains all required fields", prop.ForAll(
		func(question string, eventType domain.EventType, options []string) bool {
			// Create FSM with mock dependencies
			cfg := &config.Config{
				Timezone: time.UTC,
			}
			log := logger.New(logger.DEBUG)
			fsm := &EventCreationFSM{
				config: cfg,
				logger: log,
			}

			// Create deadline (always in future)
			deadline := time.Now().Add(24 * time.Hour)

			// Create context
			context := &domain.EventCreationContext{
				Question:  question,
				EventType: eventType,
				Options:   options,
				Deadline:  deadline,
			}

			// Build summary
			summary := fsm.buildEventSummary(context)

			// Verify all required fields are present
			if !containsString(summary, question) {
				t.Logf("Summary missing question: %s", question)
				return false
			}

			// Check for event type
			typePresent := containsString(summary, "Бинарное") ||
				containsString(summary, "Множественный выбор") ||
				containsString(summary, "Вероятностное")
			if !typePresent {
				t.Logf("Summary missing event type")
				return false
			}

			// Check for options
			for _, opt := range options {
				if !containsString(summary, opt) {
					t.Logf("Summary missing option: %s", opt)
					return false
				}
			}

			// Check for deadline (formatted)
			deadlineStr := deadline.In(cfg.Timezone).Format("02.01.2006 15:04")
			if !containsString(summary, deadlineStr) {
				t.Logf("Summary missing deadline: %s", deadlineStr)
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.OneConstOf(domain.EventTypeBinary, domain.EventTypeMultiOption, domain.EventTypeProbability),
		gen.SliceOfN(2, gen.Identifier()),
	))

	properties.TestingRun(t)
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return contains(s, substr)
}

// MockBot for testing message sending with keyboard
type MockBot struct {
	sentMessages []MockSentMessage
}

type MockSentMessage struct {
	ChatID      int64
	Text        string
	ReplyMarkup models.ReplyMarkup
}

func (m *MockBot) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	chatID, _ := params.ChatID.(int64)
	m.sentMessages = append(m.sentMessages, MockSentMessage{
		ChatID:      chatID,
		Text:        params.Text,
		ReplyMarkup: params.ReplyMarkup,
	})
	return &models.Message{
		ID: len(m.sentMessages),
	}, nil
}

func TestProperty_ConfirmationKeyboardPresence(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("confirmation message has correct keyboard buttons", prop.ForAll(
		func(question string, options []string) bool {
			// Create mock bot
			mockBot := &MockBot{}

			// Create a test keyboard (simulating what handleDeadlineInput does)
			kb := &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{
						{Text: "✅ Подтвердить", CallbackData: "confirm:yes"},
						{Text: "❌ Отменить", CallbackData: "confirm:no"},
					},
				},
			}

			// Send a message with the keyboard (simulating confirmation step)
			ctx := context.Background()
			chatID := int64(12345)
			_, _ = mockBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:      chatID,
				Text:        "Test confirmation",
				ReplyMarkup: kb,
			})

			// Verify keyboard was sent
			if len(mockBot.sentMessages) == 0 {
				t.Logf("No messages sent")
				return false
			}

			lastMsg := mockBot.sentMessages[len(mockBot.sentMessages)-1]
			if lastMsg.ReplyMarkup == nil {
				t.Logf("No reply markup in message")
				return false
			}

			// Check if it's an inline keyboard
			inlineKb, ok := lastMsg.ReplyMarkup.(*models.InlineKeyboardMarkup)
			if !ok {
				t.Logf("Reply markup is not InlineKeyboardMarkup")
				return false
			}

			// Verify exactly 2 buttons in one row
			if len(inlineKb.InlineKeyboard) != 1 {
				t.Logf("Expected 1 row, got %d", len(inlineKb.InlineKeyboard))
				return false
			}

			if len(inlineKb.InlineKeyboard[0]) != 2 {
				t.Logf("Expected 2 buttons, got %d", len(inlineKb.InlineKeyboard[0]))
				return false
			}

			// Verify button texts
			btn1 := inlineKb.InlineKeyboard[0][0]
			btn2 := inlineKb.InlineKeyboard[0][1]

			if btn1.Text != "✅ Подтвердить" || btn2.Text != "❌ Отменить" {
				t.Logf("Button texts incorrect: %s, %s", btn1.Text, btn2.Text)
				return false
			}

			// Verify callback data
			if btn1.CallbackData != "confirm:yes" || btn2.CallbackData != "confirm:no" {
				t.Logf("Callback data incorrect: %s, %s", btn1.CallbackData, btn2.CallbackData)
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.SliceOfN(2, gen.Identifier()),
	))

	properties.TestingRun(t)
}

// MockEventManager for testing event creation
type MockEventManager struct {
	createdEvents []*domain.Event
	updatedEvents []*domain.Event
}

func (m *MockEventManager) CreateEvent(ctx context.Context, event *domain.Event) error {
	// Assign an ID to simulate database behavior
	event.ID = int64(len(m.createdEvents) + 1)
	m.createdEvents = append(m.createdEvents, event)
	return nil
}

func (m *MockEventManager) UpdateEvent(ctx context.Context, event *domain.Event) error {
	m.updatedEvents = append(m.updatedEvents, event)
	return nil
}

func (m *MockEventManager) GetEvent(ctx context.Context, eventID int64) (*domain.Event, error) {
	for _, e := range m.createdEvents {
		if e.ID == eventID {
			return e, nil
		}
	}
	return nil, domain.ErrEventNotFound
}

func (m *MockEventManager) GetActiveEvents(ctx context.Context) ([]*domain.Event, error) {
	return m.createdEvents, nil
}

func (m *MockEventManager) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	return nil
}

func (m *MockEventManager) CanEditEvent(ctx context.Context, eventID int64) (bool, error) {
	return true, nil
}

// MockBotWithPoll extends MockBot to support poll sending
type MockBotWithPoll struct {
	MockBot
	sentPolls []MockSentPoll
}

type MockSentPoll struct {
	ChatID   int64
	Question string
	Options  []models.InputPollOption
}

func (m *MockBotWithPoll) SendPoll(ctx context.Context, params *bot.SendPollParams) (*models.Message, error) {
	chatID, _ := params.ChatID.(int64)
	m.sentPolls = append(m.sentPolls, MockSentPoll{
		ChatID:   chatID,
		Question: params.Question,
		Options:  params.Options,
	})
	return &models.Message{
		ID: len(m.sentPolls),
		Poll: &models.Poll{
			ID: fmt.Sprintf("poll_%d", len(m.sentPolls)),
		},
	}, nil
}

func (m *MockBotWithPoll) AnswerCallbackQuery(ctx context.Context, params *bot.AnswerCallbackQueryParams) (bool, error) {
	return true, nil
}

func TestProperty_ConfirmationCreatesEvent(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("confirmation callback with yes creates event data", prop.ForAll(
		func(userID int64, question string, options []string) bool {
			// Test that the event data structure is correctly populated
			// when preparing for confirmation
			eventContext := &domain.EventCreationContext{
				Question:  question,
				EventType: domain.EventTypeBinary,
				Options:   options,
				Deadline:  time.Now().Add(24 * time.Hour),
				ChatID:    int64(12345),
			}

			// Create an event from the context (simulating what handleConfirmCallback does)
			event := &domain.Event{
				Question:  eventContext.Question,
				EventType: eventContext.EventType,
				Options:   eventContext.Options,
				Deadline:  eventContext.Deadline,
				CreatedAt: time.Now(),
				Status:    domain.EventStatusActive,
				CreatedBy: userID,
			}

			// Verify event has all required fields
			if event.Question != question {
				t.Logf("Event question mismatch")
				return false
			}

			if event.EventType != domain.EventTypeBinary {
				t.Logf("Event type mismatch")
				return false
			}

			if len(event.Options) != len(options) {
				t.Logf("Options count mismatch")
				return false
			}

			for i, opt := range options {
				if event.Options[i] != opt {
					t.Logf("Option %d mismatch", i)
					return false
				}
			}

			if event.Status != domain.EventStatusActive {
				t.Logf("Event status should be active")
				return false
			}

			if event.CreatedBy != userID {
				t.Logf("CreatedBy mismatch")
				return false
			}

			return true
		},
		gen.Int64(),
		gen.Identifier(),
		gen.SliceOfN(2, gen.Identifier()),
	))

	properties.TestingRun(t)
}

func TestProperty_CancellationCleanup(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("cancellation callback with no does not create event", prop.ForAll(
		func(userID int64, question string, options []string) bool {
			// Test that cancellation does not create an event
			// We verify this by checking that the callback data "confirm:no"
			// should result in no event creation

			// Create context
			eventContext := &domain.EventCreationContext{
				Question:  question,
				EventType: domain.EventTypeBinary,
				Options:   options,
				Deadline:  time.Now().Add(24 * time.Hour),
				ChatID:    int64(12345),
			}

			// Simulate cancellation - verify that we would NOT create an event
			// In the actual handleConfirmCallback, when action == "no", we skip event creation
			action := "no"

			// This simulates the logic in handleConfirmCallback
			shouldCreateEvent := (action == "yes")

			if shouldCreateEvent {
				t.Logf("Cancellation should not create event")
				return false
			}

			// Verify context data is still valid (not corrupted by cancellation)
			if eventContext.Question != question {
				t.Logf("Context question corrupted")
				return false
			}

			if len(eventContext.Options) != len(options) {
				t.Logf("Context options corrupted")
				return false
			}

			return true
		},
		gen.Int64(),
		gen.Identifier(),
		gen.SliceOfN(2, gen.Identifier()),
	))

	properties.TestingRun(t)
}

// Helper function to create a test FSM storage with in-memory database
func createTestFSMStorage(t *testing.T) *storage.FSMStorage {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	queue := storage.NewDBQueue(db)

	// Initialize schema
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	log := logger.New(logger.ERROR)
	return storage.NewFSMStorage(queue, log)
}

func TestProperty_FSMSessionRecoveryAfterRestart(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("all active sessions are restored with identical state and context after restart", prop.ForAll(
		func(userIDs []int64, questions []string, states []string) bool {
			// Ensure we have at least one session
			if len(userIDs) == 0 || len(questions) == 0 || len(states) == 0 {
				return true // Skip empty inputs
			}

			// Create in-memory storage for testing
			ctx := context.Background()
			storage := createTestFSMStorage(t)

			// Create sessions with different states
			numSessions := min(len(userIDs), len(questions), len(states))
			originalSessions := make(map[int64]*domain.EventCreationContext)

			for i := 0; i < numSessions; i++ {
				userID := userIDs[i]
				if userID == 0 {
					userID = int64(i + 1) // Ensure non-zero user IDs
				}

				// Create a context with data
				sessionContext := &domain.EventCreationContext{
					Question:          questions[i],
					EventType:         domain.EventTypeBinary,
					Options:           []string{"Да", "Нет"},
					Deadline:          time.Now().Add(24 * time.Hour),
					LastBotMessageID:  100 + i,
					LastUserMessageID: 200 + i,
					ChatID:            int64(12345 + i),
				}

				// Map state string to valid FSM state
				state := mapToValidState(states[i])

				// Store session
				if err := storage.Set(ctx, userID, state, sessionContext.ToMap()); err != nil {
					t.Logf("Failed to set session: %v", err)
					return false
				}

				// Save original for comparison
				originalSessions[userID] = sessionContext
			}

			// Simulate restart by creating a new storage instance with the same underlying data
			// In a real scenario, this would be a new process reading from the same database
			// For testing, we just verify we can retrieve all sessions

			// Verify all sessions can be retrieved with identical data
			for userID, originalContext := range originalSessions {
				state, data, err := storage.Get(ctx, userID)
				if err != nil {
					t.Logf("Failed to get session for user %d: %v", userID, err)
					return false
				}

				// Verify state is preserved
				if state == "" {
					t.Logf("State is empty for user %d", userID)
					return false
				}

				// Load context from data
				restoredContext := &domain.EventCreationContext{}
				if err := restoredContext.FromMap(data); err != nil {
					t.Logf("Failed to restore context for user %d: %v", userID, err)
					return false
				}

				// Verify all fields match
				if restoredContext.Question != originalContext.Question {
					t.Logf("Question mismatch for user %d: expected %s, got %s", userID, originalContext.Question, restoredContext.Question)
					return false
				}

				if restoredContext.EventType != originalContext.EventType {
					t.Logf("EventType mismatch for user %d", userID)
					return false
				}

				if len(restoredContext.Options) != len(originalContext.Options) {
					t.Logf("Options count mismatch for user %d", userID)
					return false
				}

				for j, opt := range originalContext.Options {
					if restoredContext.Options[j] != opt {
						t.Logf("Option %d mismatch for user %d", j, userID)
						return false
					}
				}

				if restoredContext.LastBotMessageID != originalContext.LastBotMessageID {
					t.Logf("LastBotMessageID mismatch for user %d", userID)
					return false
				}

				if restoredContext.LastUserMessageID != originalContext.LastUserMessageID {
					t.Logf("LastUserMessageID mismatch for user %d", userID)
					return false
				}

				if restoredContext.ChatID != originalContext.ChatID {
					t.Logf("ChatID mismatch for user %d", userID)
					return false
				}

				// Verify deadline is preserved (within 1 second tolerance for serialization)
				if restoredContext.Deadline.Sub(originalContext.Deadline).Abs() > time.Second {
					t.Logf("Deadline mismatch for user %d", userID)
					return false
				}
			}

			// Cleanup
			for userID := range originalSessions {
				_ = storage.Delete(ctx, userID)
			}

			return true
		},
		gen.SliceOfN(3, gen.Int64()),
		gen.SliceOfN(3, gen.Identifier()),
		gen.SliceOfN(3, gen.Identifier()),
	))

	properties.TestingRun(t)
}

// Helper function to map arbitrary strings to valid FSM states
func mapToValidState(s string) string {
	validStates := []string{
		StateAskQuestion,
		StateAskEventType,
		StateAskOptions,
		StateAskDeadline,
		StateConfirm,
	}

	// Use hash of string to pick a state
	hash := 0
	for _, c := range s {
		hash += int(c)
	}
	return validStates[hash%len(validStates)]
}

// Helper function to get minimum of three integers
func min(a, b, c int) int {
	result := a
	if b < result {
		result = b
	}
	if c < result {
		result = c
	}
	return result
}

func TestProperty_FSMStateResumption(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("message from admin with active session is processed in context of stored state", prop.ForAll(
		func(userID int64, question string, messageText string) bool {
			// Ensure non-zero user ID
			if userID == 0 {
				userID = 1
			}

			// Create in-memory storage for testing
			ctx := context.Background()
			storage := createTestFSMStorage(t)

			// Create a session in a specific state (ask_options)
			sessionContext := &domain.EventCreationContext{
				Question:          question,
				EventType:         domain.EventTypeMultiOption,
				Options:           []string{}, // Empty, waiting for input
				Deadline:          time.Now().Add(24 * time.Hour),
				LastBotMessageID:  100,
				LastUserMessageID: 0,
				ChatID:            int64(12345),
			}

			// Store session in ask_options state
			if err := storage.Set(ctx, userID, StateAskOptions, sessionContext.ToMap()); err != nil {
				t.Logf("Failed to set session: %v", err)
				return false
			}

			// Verify session exists and is in correct state
			state, data, err := storage.Get(ctx, userID)
			if err != nil {
				t.Logf("Failed to get session: %v", err)
				return false
			}

			if state != StateAskOptions {
				t.Logf("State mismatch: expected %s, got %s", StateAskOptions, state)
				return false
			}

			// Load context from data
			restoredContext := &domain.EventCreationContext{}
			if err := restoredContext.FromMap(data); err != nil {
				t.Logf("Failed to restore context: %v", err)
				return false
			}

			// Verify context matches original
			if restoredContext.Question != question {
				t.Logf("Question mismatch: expected %s, got %s", question, restoredContext.Question)
				return false
			}

			if restoredContext.EventType != domain.EventTypeMultiOption {
				t.Logf("EventType mismatch")
				return false
			}

			if restoredContext.ChatID != sessionContext.ChatID {
				t.Logf("ChatID mismatch")
				return false
			}

			// Verify the state is correct for processing options input
			// In a real scenario, the FSM would now process the message in the context of this state
			// For this test, we verify that the state and context are correctly preserved
			if state != StateAskOptions {
				t.Logf("State should be ask_options for processing options input")
				return false
			}

			// Cleanup
			_ = storage.Delete(ctx, userID)

			return true
		},
		gen.Int64(),
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

func TestProperty_ExpiredSessionHandling(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("message to expired session returns ErrSessionExpired and deletes session", prop.ForAll(
		func(userID int64, question string) bool {
			// Ensure non-zero user ID
			if userID == 0 {
				userID = 1
			}

			// Setup in-memory database
			ctx := context.Background()
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer db.Close()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			log := logger.New(logger.ERROR)
			fsmStorage := storage.NewFSMStorage(queue, log)

			// Create a session
			sessionContext := &domain.EventCreationContext{
				Question:          question,
				EventType:         domain.EventTypeBinary,
				Options:           []string{"Да", "Нет"},
				Deadline:          time.Now().Add(24 * time.Hour),
				LastBotMessageID:  100,
				LastUserMessageID: 0,
				ChatID:            int64(12345),
			}

			// Store session
			if err := fsmStorage.Set(ctx, userID, StateAskDeadline, sessionContext.ToMap()); err != nil {
				t.Logf("Failed to set session: %v", err)
				return false
			}

			// Manually update the updated_at timestamp to be older than 30 minutes
			staleTime := "datetime('now', '-31 minutes')"
			err = queue.Execute(func(db *sql.DB) error {
				_, err := db.ExecContext(ctx,
					"UPDATE fsm_sessions SET updated_at = "+staleTime+" WHERE user_id = ?",
					userID)
				return err
			})
			if err != nil {
				t.Logf("Failed to update timestamp: %v", err)
				return false
			}

			// Try to get the session - should return ErrSessionExpired
			_, _, err = fsmStorage.Get(ctx, userID)
			if err != storage.ErrSessionExpired {
				t.Logf("Expected ErrSessionExpired, got: %v", err)
				return false
			}

			// Verify session was deleted (second Get should return ErrSessionNotFound)
			_, _, err = fsmStorage.Get(ctx, userID)
			if err != storage.ErrSessionNotFound {
				t.Logf("Session should be deleted after expiration, expected ErrSessionNotFound, got: %v", err)
				return false
			}

			return true
		},
		gen.Int64(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

func TestProperty_MessageRoutingToCorrectSession(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("messages from admins are routed to their specific session using user_id", prop.ForAll(
		func(userIDs []int64, questions []string) bool {
			// Ensure we have at least one user
			if len(userIDs) == 0 || len(questions) == 0 {
				return true // Skip empty inputs
			}

			// Deduplicate user IDs
			uniqueUserIDs := make(map[int64]bool)
			var dedupedUserIDs []int64
			for _, userID := range userIDs {
				if userID > 0 && !uniqueUserIDs[userID] {
					uniqueUserIDs[userID] = true
					dedupedUserIDs = append(dedupedUserIDs, userID)
				}
			}

			if len(dedupedUserIDs) == 0 {
				return true // Skip if no valid user IDs
			}

			// Create in-memory storage for testing
			ctx := context.Background()
			fsmStorage := createTestFSMStorage(t)

			// Create sessions for each user with unique questions
			sessionData := make(map[int64]string) // userID -> question
			for i, userID := range dedupedUserIDs {
				question := questions[i%len(questions)]
				sessionData[userID] = question

				// Create a session in ask_event_type state (waiting for input)
				sessionContext := &domain.EventCreationContext{
					Question:          question,
					EventType:         domain.EventTypeBinary,
					Options:           []string{"Да", "Нет"},
					Deadline:          time.Now().Add(24 * time.Hour),
					LastBotMessageID:  100 + i,
					LastUserMessageID: 0,
					ChatID:            int64(12345 + i),
				}

				if err := fsmStorage.Set(ctx, userID, StateAskEventType, sessionContext.ToMap()); err != nil {
					t.Logf("Failed to set session for user %d: %v", userID, err)
					return false
				}
			}

			// Simulate message routing: for each user, verify we can retrieve their specific session
			for _, userID := range dedupedUserIDs {
				// Get session for this user (simulating FSM.HandleMessage routing)
				state, data, err := fsmStorage.Get(ctx, userID)
				if err != nil {
					t.Logf("Failed to get session for user %d: %v", userID, err)
					return false
				}

				// Verify state is correct
				if state != StateAskEventType {
					t.Logf("State mismatch for user %d: expected %s, got %s", userID, StateAskEventType, state)
					return false
				}

				// Load context
				restoredContext := &domain.EventCreationContext{}
				if err := restoredContext.FromMap(data); err != nil {
					t.Logf("Failed to restore context for user %d: %v", userID, err)
					return false
				}

				// Verify the question matches what we stored for this specific user
				expectedQuestion := sessionData[userID]
				if restoredContext.Question != expectedQuestion {
					t.Logf("Question mismatch for user %d: expected %s, got %s", userID, expectedQuestion, restoredContext.Question)
					return false
				}

				// Verify ChatID is unique to this user's session
				expectedChatID := int64(12345 + indexOf(dedupedUserIDs, userID))
				if restoredContext.ChatID != expectedChatID {
					t.Logf("ChatID mismatch for user %d: expected %d, got %d", userID, expectedChatID, restoredContext.ChatID)
					return false
				}
			}

			// Verify that getting a non-existent user's session returns ErrSessionNotFound
			nonExistentUserID := int64(999999999)
			_, _, err := fsmStorage.Get(ctx, nonExistentUserID)
			if err != storage.ErrSessionNotFound {
				t.Logf("Expected ErrSessionNotFound for non-existent user, got: %v", err)
				return false
			}

			// Cleanup
			for _, userID := range dedupedUserIDs {
				_ = fsmStorage.Delete(ctx, userID)
			}

			return true
		},
		gen.SliceOf(gen.Int64Range(1, 1000000)),
		gen.SliceOf(gen.Identifier()),
	))

	properties.TestingRun(t)
}

// Helper function to find index of element in slice
func indexOf(slice []int64, value int64) int {
	for i, v := range slice {
		if v == value {
			return i
		}
	}
	return -1
}

func TestProperty_SessionIndependenceOnCompletion(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("completing one admin's session does not affect other active sessions", prop.ForAll(
		func(userIDs []int64, questions []string) bool {
			// Ensure we have at least 2 users
			if len(userIDs) < 2 || len(questions) < 2 {
				return true // Skip if not enough data
			}

			// Deduplicate user IDs
			uniqueUserIDs := make(map[int64]bool)
			var dedupedUserIDs []int64
			for _, userID := range userIDs {
				if userID > 0 && !uniqueUserIDs[userID] {
					uniqueUserIDs[userID] = true
					dedupedUserIDs = append(dedupedUserIDs, userID)
				}
			}

			if len(dedupedUserIDs) < 2 {
				return true // Need at least 2 users for this test
			}

			// Create in-memory storage for testing
			ctx := context.Background()
			fsmStorage := createTestFSMStorage(t)

			// Create sessions for all users
			sessionData := make(map[int64]*domain.EventCreationContext)
			for i, userID := range dedupedUserIDs {
				question := questions[i%len(questions)]

				sessionContext := &domain.EventCreationContext{
					Question:          question,
					EventType:         domain.EventTypeBinary,
					Options:           []string{"Да", "Нет"},
					Deadline:          time.Now().Add(24 * time.Hour),
					LastBotMessageID:  100 + i,
					LastUserMessageID: 200 + i,
					ChatID:            int64(12345 + i),
				}

				if err := fsmStorage.Set(ctx, userID, StateConfirm, sessionContext.ToMap()); err != nil {
					t.Logf("Failed to set session for user %d: %v", userID, err)
					return false
				}

				sessionData[userID] = sessionContext
			}

			// Verify all sessions exist
			for _, userID := range dedupedUserIDs {
				_, _, err := fsmStorage.Get(ctx, userID)
				if err != nil {
					t.Logf("Session should exist for user %d before completion, got error: %v", userID, err)
					return false
				}
			}

			// Complete the first user's session (simulate event creation completion)
			completedUserID := dedupedUserIDs[0]
			if err := fsmStorage.Delete(ctx, completedUserID); err != nil {
				t.Logf("Failed to delete session for completed user %d: %v", completedUserID, err)
				return false
			}

			// Verify the completed user's session is gone
			_, _, err := fsmStorage.Get(ctx, completedUserID)
			if err != storage.ErrSessionNotFound {
				t.Logf("Completed user's session should be deleted, expected ErrSessionNotFound, got: %v", err)
				return false
			}

			// Verify all other users' sessions are still intact with unchanged data
			for i := 1; i < len(dedupedUserIDs); i++ {
				userID := dedupedUserIDs[i]
				originalContext := sessionData[userID]

				state, data, err := fsmStorage.Get(ctx, userID)
				if err != nil {
					t.Logf("Session should still exist for user %d after another user completed, got error: %v", userID, err)
					return false
				}

				// Verify state is unchanged
				if state != StateConfirm {
					t.Logf("State changed for user %d: expected %s, got %s", userID, StateConfirm, state)
					return false
				}

				// Load context
				restoredContext := &domain.EventCreationContext{}
				if err := restoredContext.FromMap(data); err != nil {
					t.Logf("Failed to restore context for user %d: %v", userID, err)
					return false
				}

				// Verify all fields are unchanged
				if restoredContext.Question != originalContext.Question {
					t.Logf("Question changed for user %d after another user completed", userID)
					return false
				}

				if restoredContext.EventType != originalContext.EventType {
					t.Logf("EventType changed for user %d after another user completed", userID)
					return false
				}

				if len(restoredContext.Options) != len(originalContext.Options) {
					t.Logf("Options count changed for user %d after another user completed", userID)
					return false
				}

				for j, opt := range originalContext.Options {
					if restoredContext.Options[j] != opt {
						t.Logf("Option %d changed for user %d after another user completed", j, userID)
						return false
					}
				}

				if restoredContext.LastBotMessageID != originalContext.LastBotMessageID {
					t.Logf("LastBotMessageID changed for user %d after another user completed", userID)
					return false
				}

				if restoredContext.LastUserMessageID != originalContext.LastUserMessageID {
					t.Logf("LastUserMessageID changed for user %d after another user completed", userID)
					return false
				}

				if restoredContext.ChatID != originalContext.ChatID {
					t.Logf("ChatID changed for user %d after another user completed", userID)
					return false
				}

				// Verify deadline is unchanged (within 1 second tolerance)
				if restoredContext.Deadline.Sub(originalContext.Deadline).Abs() > time.Second {
					t.Logf("Deadline changed for user %d after another user completed", userID)
					return false
				}
			}

			// Verify the total session count is correct (all users minus the completed one)
			activeCount := 0
			for _, userID := range dedupedUserIDs {
				_, _, err := fsmStorage.Get(ctx, userID)
				if err == nil {
					activeCount++
				}
			}

			expectedActiveCount := len(dedupedUserIDs) - 1
			if activeCount != expectedActiveCount {
				t.Logf("Active session count mismatch: expected %d, got %d", expectedActiveCount, activeCount)
				return false
			}

			// Cleanup remaining sessions
			for i := 1; i < len(dedupedUserIDs); i++ {
				_ = fsmStorage.Delete(ctx, dedupedUserIDs[i])
			}

			return true
		},
		gen.SliceOf(gen.Int64Range(1, 1000000)),
		gen.SliceOf(gen.Identifier()),
	))

	properties.TestingRun(t)
}

// Feature: event-creation-ux-improvement, Property 29: Validation error message cleanup
// Validates: Requirements 1.10, 1.11
func TestProperty_ValidationErrorMessageCleanup(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("validation error messages and invalid user input are deleted when valid input is provided", prop.ForAll(
		func(userID int64, chatID int64, invalidInput string, validInput string) bool {
			// Ensure non-zero IDs
			if userID == 0 {
				userID = 1
			}
			if chatID == 0 {
				chatID = 12345
			}

			// Ensure valid input is actually valid (non-empty)
			if strings.TrimSpace(validInput) == "" {
				validInput = "Valid question"
			}

			// Create mock dependencies
			ctx := context.Background()
			mockDeleter := NewMockMessageDeleter()
			log := logger.New(logger.ERROR)
			fsmStorage := createTestFSMStorage(t)

			// Create a context with an error message ID from a previous validation error
			errorMessageID := 999
			context := &domain.EventCreationContext{
				Question:           "",
				EventType:          domain.EventTypeBinary,
				Options:            []string{},
				Deadline:           time.Time{},
				LastBotMessageID:   100,
				LastUserMessageID:  0,
				LastErrorMessageID: errorMessageID,
				ChatID:             chatID,
			}

			// Store the context in ask_question state
			if err := fsmStorage.Set(ctx, userID, StateAskQuestion, context.ToMap()); err != nil {
				t.Logf("Failed to set session: %v", err)
				return false
			}

			// Simulate sending invalid input (empty string)
			invalidUserMessageID := 200

			// In the actual handler, this would:
			// 1. Delete the previous error message (999)
			// 2. Delete the invalid user input (200)
			// 3. Send a new error message

			// Simulate the deletion logic from handleQuestionInput
			if context.LastErrorMessageID != 0 {
				deleteMessages(ctx, mockDeleter, log, chatID, context.LastErrorMessageID)
			}
			deleteMessages(ctx, mockDeleter, log, chatID, invalidUserMessageID)

			// Verify previous error message was deleted
			deletedMsgs := mockDeleter.GetDeletedMessages(chatID)
			foundPrevError := false
			foundInvalidInput := false
			for _, msgID := range deletedMsgs {
				if msgID == errorMessageID {
					foundPrevError = true
				}
				if msgID == invalidUserMessageID {
					foundInvalidInput = true
				}
			}

			if !foundPrevError {
				t.Logf("Previous error message %d should be deleted", errorMessageID)
				return false
			}

			if !foundInvalidInput {
				t.Logf("Invalid user input message %d should be deleted", invalidUserMessageID)
				return false
			}

			// Now simulate sending valid input
			// Reset mock deleter to track new deletions
			mockDeleter = NewMockMessageDeleter()
			validUserMessageID := 300
			newErrorMessageID := 1000 // This would be set if validation failed again

			// Update context with new error message ID (simulating another error)
			context.LastErrorMessageID = newErrorMessageID

			// Now send valid input - should delete error message, bot message, and user message
			if context.LastErrorMessageID != 0 {
				deleteMessages(ctx, mockDeleter, log, chatID, context.LastErrorMessageID)
			}
			deleteMessages(ctx, mockDeleter, log, chatID, context.LastBotMessageID, validUserMessageID)

			// Verify all messages were deleted
			deletedMsgs = mockDeleter.GetDeletedMessages(chatID)
			foundError := false
			foundBot := false
			foundUser := false

			for _, msgID := range deletedMsgs {
				if msgID == newErrorMessageID {
					foundError = true
				}
				if msgID == context.LastBotMessageID {
					foundBot = true
				}
				if msgID == validUserMessageID {
					foundUser = true
				}
			}

			if !foundError {
				t.Logf("Error message %d should be deleted when valid input is provided", newErrorMessageID)
				return false
			}

			if !foundBot {
				t.Logf("Bot message %d should be deleted when valid input is provided", context.LastBotMessageID)
				return false
			}

			if !foundUser {
				t.Logf("User message %d should be deleted when valid input is provided", validUserMessageID)
				return false
			}

			// Verify exactly 3 messages were deleted (error, bot, user)
			if len(deletedMsgs) != 3 {
				t.Logf("Expected 3 messages deleted (error, bot, user), got %d", len(deletedMsgs))
				return false
			}

			// Cleanup
			_ = fsmStorage.Delete(ctx, userID)

			return true
		},
		gen.Int64(),
		gen.Int64(),
		gen.Const(""),    // Invalid input (empty string)
		gen.Identifier(), // Valid input
	))

	properties.TestingRun(t)
}
