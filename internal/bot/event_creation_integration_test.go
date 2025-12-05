package bot

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"telegram-prediction-bot/internal/domain"
	"telegram-prediction-bot/internal/logger"
	"telegram-prediction-bot/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	_ "modernc.org/sqlite"
)

// MockBotForIntegration implements the bot methods needed for integration tests
type MockBotForIntegration struct {
	sentMessages []MockSentMessage
	sentPolls    []MockSentPoll
}

func (m *MockBotForIntegration) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
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

func (m *MockBotForIntegration) SendPoll(ctx context.Context, params *bot.SendPollParams) (*models.Message, error) {
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

func (m *MockBotForIntegration) AnswerCallbackQuery(ctx context.Context, params *bot.AnswerCallbackQueryParams) (bool, error) {
	return true, nil
}

func (m *MockBotForIntegration) DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
	return true, nil
}

// Integration test for complete event creation flow
func TestIntegration_CompleteEventCreationFlow(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
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

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)

	// Create event manager
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)

	// Create FSM storage
	fsmStorage := storage.NewFSMStorage(queue, log)

	// Test data
	userID := int64(12345)
	chatID := int64(67890)
	question := "Will it snow in December?"
	eventType := domain.EventTypeBinary
	deadline := time.Now().Add(48 * time.Hour)

	// Step 1: Create a session manually in storage
	initialContext := &domain.EventCreationContext{
		ChatID: chatID,
	}
	if err := fsmStorage.Set(ctx, userID, StateAskQuestion, initialContext.ToMap()); err != nil {
		t.Fatalf("Failed to create initial session: %v", err)
	}

	// Step 2: Update session with question
	initialContext.Question = question
	initialContext.LastBotMessageID = 100
	initialContext.LastUserMessageID = 101
	if err := fsmStorage.Set(ctx, userID, StateAskEventType, initialContext.ToMap()); err != nil {
		t.Fatalf("Failed to update session with question: %v", err)
	}

	// Step 3: Update session with event type
	initialContext.EventType = eventType
	initialContext.Options = []string{"Да", "Нет"}
	if err := fsmStorage.Set(ctx, userID, StateAskDeadline, initialContext.ToMap()); err != nil {
		t.Fatalf("Failed to update session with event type: %v", err)
	}

	// Step 4: Update session with deadline
	initialContext.Deadline = deadline
	if err := fsmStorage.Set(ctx, userID, StateConfirm, initialContext.ToMap()); err != nil {
		t.Fatalf("Failed to update session with deadline: %v", err)
	}

	// Step 5: Create event (simulating confirmation)
	event := &domain.Event{
		Question:  initialContext.Question,
		EventType: initialContext.EventType,
		Options:   initialContext.Options,
		Deadline:  initialContext.Deadline,
		CreatedAt: time.Now(),
		Status:    domain.EventStatusActive,
		CreatedBy: userID,
	}

	if err := eventManager.CreateEvent(ctx, event); err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Step 6: Delete session (simulating completion)
	if err := fsmStorage.Delete(ctx, userID); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify event was created in database
	events, err := eventManager.GetActiveEvents(ctx)
	if err != nil {
		t.Fatalf("Failed to get active events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	createdEvent := events[0]
	if createdEvent.Question != question {
		t.Errorf("Expected question %q, got %q", question, createdEvent.Question)
	}
	if createdEvent.EventType != eventType {
		t.Errorf("Expected event type %v, got %v", eventType, createdEvent.EventType)
	}
	if len(createdEvent.Options) != 2 {
		t.Errorf("Expected 2 options for binary event, got %d", len(createdEvent.Options))
	}
	if createdEvent.CreatedBy != userID {
		t.Errorf("Expected created by %d, got %d", userID, createdEvent.CreatedBy)
	}

	// Verify FSM session was deleted
	_, _, err = fsmStorage.Get(ctx, userID)
	if err != storage.ErrSessionNotFound {
		t.Errorf("Expected session to be deleted, got error: %v", err)
	}
}

// Integration test for concurrent sessions
func TestIntegration_ConcurrentSessions(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
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

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)

	// Create event manager
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)

	// Create FSM storage
	fsmStorage := storage.NewFSMStorage(queue, log)

	// Test data for 3 concurrent admins
	admin1ID := int64(11111)
	admin2ID := int64(22222)
	admin3ID := int64(33333)
	chatID := int64(67890)

	question1 := "Will it rain tomorrow?"
	question2 := "Will it snow next week?"
	question3 := "Will the sun shine on Friday?"

	// Create sessions for all 3 admins
	ctx1 := &domain.EventCreationContext{
		Question:          question1,
		EventType:         domain.EventTypeBinary,
		Options:           []string{"Да", "Нет"},
		Deadline:          time.Now().Add(24 * time.Hour),
		LastBotMessageID:  100,
		LastUserMessageID: 101,
		ChatID:            chatID,
	}
	if err := fsmStorage.Set(ctx, admin1ID, StateAskEventType, ctx1.ToMap()); err != nil {
		t.Fatalf("Failed to create session for admin1: %v", err)
	}

	ctx2 := &domain.EventCreationContext{
		Question:          question2,
		EventType:         domain.EventTypeBinary,
		Options:           []string{"Да", "Нет"},
		Deadline:          time.Now().Add(24 * time.Hour),
		LastBotMessageID:  200,
		LastUserMessageID: 201,
		ChatID:            chatID,
	}
	if err := fsmStorage.Set(ctx, admin2ID, StateAskEventType, ctx2.ToMap()); err != nil {
		t.Fatalf("Failed to create session for admin2: %v", err)
	}

	ctx3 := &domain.EventCreationContext{
		Question:          question3,
		EventType:         domain.EventTypeBinary,
		Options:           []string{"Да", "Нет"},
		Deadline:          time.Now().Add(24 * time.Hour),
		LastBotMessageID:  300,
		LastUserMessageID: 301,
		ChatID:            chatID,
	}
	if err := fsmStorage.Set(ctx, admin3ID, StateAskEventType, ctx3.ToMap()); err != nil {
		t.Fatalf("Failed to create session for admin3: %v", err)
	}

	// Verify all sessions exist
	_, data1, err := fsmStorage.Get(ctx, admin1ID)
	if err != nil {
		t.Fatalf("Failed to get session for admin1: %v", err)
	}
	_, data2, err := fsmStorage.Get(ctx, admin2ID)
	if err != nil {
		t.Fatalf("Failed to get session for admin2: %v", err)
	}
	_, data3, err := fsmStorage.Get(ctx, admin3ID)
	if err != nil {
		t.Fatalf("Failed to get session for admin3: %v", err)
	}

	// Verify each session has its own question
	restored1 := &domain.EventCreationContext{}
	restored1.FromMap(data1)
	restored2 := &domain.EventCreationContext{}
	restored2.FromMap(data2)
	restored3 := &domain.EventCreationContext{}
	restored3.FromMap(data3)

	if restored1.Question != question1 {
		t.Errorf("Admin1 question mismatch: expected %q, got %q", question1, restored1.Question)
	}
	if restored2.Question != question2 {
		t.Errorf("Admin2 question mismatch: expected %q, got %q", question2, restored2.Question)
	}
	if restored3.Question != question3 {
		t.Errorf("Admin3 question mismatch: expected %q, got %q", question3, restored3.Question)
	}

	// Complete admin1's session
	event1 := &domain.Event{
		Question:  ctx1.Question,
		EventType: ctx1.EventType,
		Options:   ctx1.Options,
		Deadline:  ctx1.Deadline,
		CreatedAt: time.Now(),
		Status:    domain.EventStatusActive,
		CreatedBy: admin1ID,
	}
	if err := eventManager.CreateEvent(ctx, event1); err != nil {
		t.Fatalf("Failed to create event for admin1: %v", err)
	}

	// Delete admin1's session
	if err := fsmStorage.Delete(ctx, admin1ID); err != nil {
		t.Fatalf("Failed to delete session for admin1: %v", err)
	}

	// Verify admin1's session is deleted
	_, _, err = fsmStorage.Get(ctx, admin1ID)
	if err != storage.ErrSessionNotFound {
		t.Error("Expected admin1's session to be deleted")
	}

	// Verify admin2 and admin3 sessions are still active and unchanged
	_, data2After, err := fsmStorage.Get(ctx, admin2ID)
	if err != nil {
		t.Fatalf("Expected admin2's session to still exist: %v", err)
	}
	_, data3After, err := fsmStorage.Get(ctx, admin3ID)
	if err != nil {
		t.Fatalf("Expected admin3's session to still exist: %v", err)
	}

	restored2After := &domain.EventCreationContext{}
	restored2After.FromMap(data2After)
	restored3After := &domain.EventCreationContext{}
	restored3After.FromMap(data3After)

	if restored2After.Question != question2 {
		t.Errorf("Admin2 question changed after admin1 completed: expected %q, got %q", question2, restored2After.Question)
	}
	if restored3After.Question != question3 {
		t.Errorf("Admin3 question changed after admin1 completed: expected %q, got %q", question3, restored3After.Question)
	}

	// Verify event was created for admin1
	events, err := eventManager.GetActiveEvents(ctx)
	if err != nil {
		t.Fatalf("Failed to get active events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event after admin1 completion, got %d", len(events))
	}
	if events[0].Question != question1 {
		t.Errorf("Expected event question %q, got %q", question1, events[0].Question)
	}
}

// Integration test for restart recovery
func TestIntegration_RestartRecovery(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
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

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)

	// Create event manager
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)

	// Create FSM storage (before "restart")
	fsmStorage1 := storage.NewFSMStorage(queue, log)

	// Test data
	userID := int64(12345)
	chatID := int64(67890)
	question := "Will it be sunny tomorrow?"

	// Create a session in ask_event_type state
	sessionContext := &domain.EventCreationContext{
		Question:          question,
		EventType:         domain.EventTypeBinary,
		Options:           []string{},
		Deadline:          time.Time{},
		LastBotMessageID:  100,
		LastUserMessageID: 101,
		ChatID:            chatID,
	}
	if err := fsmStorage1.Set(ctx, userID, StateAskEventType, sessionContext.ToMap()); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session exists before "restart"
	state1, data1, err := fsmStorage1.Get(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get session before restart: %v", err)
	}
	if state1 != StateAskEventType {
		t.Errorf("Expected state %s before restart, got %s", StateAskEventType, state1)
	}

	// Load context before restart
	ctx1 := &domain.EventCreationContext{}
	if err := ctx1.FromMap(data1); err != nil {
		t.Fatalf("Failed to load context before restart: %v", err)
	}

	// Simulate bot restart by creating new FSM storage instance
	// The database connection remains the same, simulating persistence
	fsmStorage2 := storage.NewFSMStorage(queue, log)

	// Verify session still exists after "restart"
	state2, data2, err := fsmStorage2.Get(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get session after restart: %v", err)
	}
	if state2 != state1 {
		t.Errorf("State changed after restart: expected %s, got %s", state1, state2)
	}

	// Load context after restart
	ctx2 := &domain.EventCreationContext{}
	if err := ctx2.FromMap(data2); err != nil {
		t.Fatalf("Failed to load context after restart: %v", err)
	}

	// Verify all data is preserved
	if ctx2.Question != ctx1.Question {
		t.Errorf("Question changed after restart: expected %q, got %q", ctx1.Question, ctx2.Question)
	}
	if ctx2.EventType != ctx1.EventType {
		t.Errorf("EventType changed after restart")
	}
	if ctx2.ChatID != ctx1.ChatID {
		t.Errorf("ChatID changed after restart: expected %d, got %d", ctx1.ChatID, ctx2.ChatID)
	}
	if ctx2.LastBotMessageID != ctx1.LastBotMessageID {
		t.Errorf("LastBotMessageID changed after restart: expected %d, got %d", ctx1.LastBotMessageID, ctx2.LastBotMessageID)
	}

	// Continue the session after restart
	// Update session with event type and deadline
	ctx2.EventType = domain.EventTypeBinary
	ctx2.Options = []string{"Да", "Нет"}
	ctx2.Deadline = time.Now().Add(48 * time.Hour)
	if err := fsmStorage2.Set(ctx, userID, StateConfirm, ctx2.ToMap()); err != nil {
		t.Fatalf("Failed to update session after restart: %v", err)
	}

	// Create event (simulating confirmation)
	event := &domain.Event{
		Question:  ctx2.Question,
		EventType: ctx2.EventType,
		Options:   ctx2.Options,
		Deadline:  ctx2.Deadline,
		CreatedAt: time.Now(),
		Status:    domain.EventStatusActive,
		CreatedBy: userID,
	}
	if err := eventManager.CreateEvent(ctx, event); err != nil {
		t.Fatalf("Failed to create event after restart: %v", err)
	}

	// Delete session
	if err := fsmStorage2.Delete(ctx, userID); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify event was created successfully
	events, err := eventManager.GetActiveEvents(ctx)
	if err != nil {
		t.Fatalf("Failed to get active events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event after completion, got %d", len(events))
	}
	if events[0].Question != question {
		t.Errorf("Expected event question %q, got %q", question, events[0].Question)
	}

	// Verify session was deleted
	_, _, err = fsmStorage2.Get(ctx, userID)
	if err != storage.ErrSessionNotFound {
		t.Error("Expected session to be deleted after completion")
	}
}

// Integration test for cancellation flow
func TestIntegration_CancellationFlow(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
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

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)

	// Create event manager
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)

	// Create FSM storage
	fsmStorage := storage.NewFSMStorage(queue, log)

	// Test data
	userID := int64(12345)
	chatID := int64(67890)
	question := "Will it rain next week?"

	// Create a session in confirm state (ready for confirmation)
	sessionContext := &domain.EventCreationContext{
		Question:              question,
		EventType:             domain.EventTypeBinary,
		Options:               []string{"Да", "Нет"},
		Deadline:              time.Now().Add(48 * time.Hour),
		LastBotMessageID:      100,
		LastUserMessageID:     101,
		ConfirmationMessageID: 102,
		ChatID:                chatID,
	}
	if err := fsmStorage.Set(ctx, userID, StateConfirm, sessionContext.ToMap()); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session is in confirm state
	state, _, err := fsmStorage.Get(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if state != StateConfirm {
		t.Errorf("Expected state %s, got %s", StateConfirm, state)
	}

	// Simulate cancellation by deleting the session without creating an event
	if err := fsmStorage.Delete(ctx, userID); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify session was deleted
	_, _, err = fsmStorage.Get(ctx, userID)
	if err != storage.ErrSessionNotFound {
		t.Errorf("Expected session to be deleted, got error: %v", err)
	}

	// Verify no event was created
	events, err := eventManager.GetActiveEvents(ctx)
	if err != nil {
		t.Fatalf("Failed to get active events: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Expected 0 events after cancellation, got %d", len(events))
	}
}
