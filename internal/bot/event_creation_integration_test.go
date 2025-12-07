package bot

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/logger"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

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

// setupTestGroupAndDB creates a test database with a group and returns the group ID
func setupTestGroupAndDB(t *testing.T, chatID, userID int64) (*storage.DBQueue, int64) {
	ctx := context.Background()
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

	// Create test group
	groupRepo := storage.NewGroupRepository(queue)
	group := &domain.Group{
		TelegramChatID: chatID,
		Name:           "Test Group",
		CreatedBy:      userID,
		CreatedAt:      time.Now(),
	}
	if err := groupRepo.CreateGroup(ctx, group); err != nil {
		t.Fatalf("Failed to create test group: %v", err)
	}

	return queue, group.ID
}

// Integration test for complete event creation flow
func TestIntegration_CompleteEventCreationFlow(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
	userID := int64(12345)
	chatID := int64(67890)

	queue, groupID := setupTestGroupAndDB(t, chatID, userID)
	defer queue.Close()

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
		GroupID:   groupID,
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
	// Using groupID 1 as default for tests without multi-group setup
	events, err := eventManager.GetActiveEvents(ctx, 1)
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
	chatID := int64(67890)
	admin1ID := int64(11111)

	queue, groupID := setupTestGroupAndDB(t, chatID, admin1ID)
	defer queue.Close()

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
	admin2ID := int64(22222)
	admin3ID := int64(33333)

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
	if err := restored1.FromMap(data1); err != nil {
		t.Fatalf("Failed to restore context for admin1: %v", err)
	}
	restored2 := &domain.EventCreationContext{}
	if err := restored2.FromMap(data2); err != nil {
		t.Fatalf("Failed to restore context for admin2: %v", err)
	}
	restored3 := &domain.EventCreationContext{}
	if err := restored3.FromMap(data3); err != nil {
		t.Fatalf("Failed to restore context for admin3: %v", err)
	}

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
		GroupID:   groupID,
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
	if err := restored2After.FromMap(data2After); err != nil {
		t.Fatalf("Failed to restore context for admin2 after: %v", err)
	}
	restored3After := &domain.EventCreationContext{}
	if err := restored3After.FromMap(data3After); err != nil {
		t.Fatalf("Failed to restore context for admin3 after: %v", err)
	}

	if restored2After.Question != question2 {
		t.Errorf("Admin2 question changed after admin1 completed: expected %q, got %q", question2, restored2After.Question)
	}
	if restored3After.Question != question3 {
		t.Errorf("Admin3 question changed after admin1 completed: expected %q, got %q", question3, restored3After.Question)
	}

	// Verify event was created for admin1
	// Using groupID 1 as default for tests without multi-group setup
	events, err := eventManager.GetActiveEvents(ctx, 1)
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
	userID := int64(12345)
	chatID := int64(67890)

	queue, groupID := setupTestGroupAndDB(t, chatID, userID)
	defer queue.Close()

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)

	// Create event manager
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)

	// Create FSM storage (before "restart")
	fsmStorage1 := storage.NewFSMStorage(queue, log)
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
		GroupID:           groupID,
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
		GroupID:   groupID,
	}
	if err := eventManager.CreateEvent(ctx, event); err != nil {
		t.Fatalf("Failed to create event after restart: %v", err)
	}

	// Delete session
	if err := fsmStorage2.Delete(ctx, userID); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify event was created successfully
	// Using groupID 1 as default for tests without multi-group setup
	events, err := eventManager.GetActiveEvents(ctx, 1)
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
	// Using groupID 1 as default for tests without multi-group setup
	events, err := eventManager.GetActiveEvents(ctx, 1)
	if err != nil {
		t.Fatalf("Failed to get active events: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Expected 0 events after cancellation, got %d", len(events))
	}
}

// Integration test for event creation permission flow
func TestIntegration_EventCreationPermissionFlow(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
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

	// Run migrations to ensure all tables exist
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(queue)
	groupRepo := storage.NewGroupRepository(queue)

	// Test data
	regularUserID := int64(11111)
	adminUserID := int64(99999)
	adminIDs := []int64{adminUserID}
	chatID := int64(67890)

	// Create test group
	group := &domain.Group{
		TelegramChatID: chatID,
		Name:           "Test Group",
		CreatedBy:      adminUserID,
		CreatedAt:      time.Now(),
	}
	if err := groupRepo.CreateGroup(ctx, group); err != nil {
		t.Fatalf("Failed to create test group: %v", err)
	}
	groupID := group.ID

	// Add users to group
	regularMembership := &domain.GroupMembership{
		GroupID:  groupID,
		UserID:   regularUserID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := groupMembershipRepo.CreateMembership(ctx, regularMembership); err != nil {
		t.Fatalf("Failed to create regular user membership: %v", err)
	}

	adminMembership := &domain.GroupMembership{
		GroupID:  groupID,
		UserID:   adminUserID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := groupMembershipRepo.CreateMembership(ctx, adminMembership); err != nil {
		t.Fatalf("Failed to create admin membership: %v", err)
	}

	// Create event manager
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)

	// Create event permission validator with min events = 3
	minEventsToCreate := 3
	eventPermissionValidator := domain.NewEventPermissionValidator(
		eventRepo,
		predictionRepo,
		groupMembershipRepo,
		minEventsToCreate,
		log,
	)

	// Test 1: Regular user with insufficient participation (0 events)
	t.Run("Rejection with insufficient participation", func(t *testing.T) {
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, regularUserID, groupID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check event creation permission: %v", err)
		}
		if canCreate {
			t.Error("Expected user with 0 participation to be rejected")
		}
		if count != 0 {
			t.Errorf("Expected participation count 0, got %d", count)
		}
	})

	// Test 2: Create some events and have user participate
	// Create 3 events
	for i := 0; i < 3; i++ {
		event := &domain.Event{
			Question:  fmt.Sprintf("Test event %d", i+1),
			EventType: domain.EventTypeBinary,
			Options:   []string{"Да", "Нет"},
			Deadline:  time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			Status:    domain.EventStatusActive,
			CreatedBy: adminUserID,
			GroupID:   groupID,
		}
		if err := eventManager.CreateEvent(ctx, event); err != nil {
			t.Fatalf("Failed to create test event %d: %v", i+1, err)
		}

		// User makes a prediction
		prediction := &domain.Prediction{
			EventID:   event.ID,
			UserID:    regularUserID,
			Option:    0,
			Timestamp: time.Now(),
		}
		if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
			t.Fatalf("Failed to save prediction for event %d: %v", i+1, err)
		}
	}

	// Test 3: User still can't create events (events not resolved yet)
	t.Run("Rejection when events not resolved", func(t *testing.T) {
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, regularUserID, groupID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check event creation permission: %v", err)
		}
		if canCreate {
			t.Error("Expected user to be rejected when events not resolved")
		}
		if count != 0 {
			t.Errorf("Expected participation count 0 (no resolved events), got %d", count)
		}
	})

	// Test 4: Resolve the events
	events, err := eventManager.GetActiveEvents(ctx, groupID)
	if err != nil {
		t.Fatalf("Failed to get active events: %v", err)
	}
	for _, event := range events {
		if err := eventManager.ResolveEvent(ctx, event.ID, 0); err != nil {
			t.Fatalf("Failed to resolve event %d: %v", event.ID, err)
		}
	}

	// Test 5: User now has sufficient participation
	t.Run("Success with sufficient participation", func(t *testing.T) {
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, regularUserID, groupID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check event creation permission: %v", err)
		}
		if !canCreate {
			t.Errorf("Expected user with %d participation to be allowed to create events", count)
		}
		if count != 3 {
			t.Errorf("Expected participation count 3, got %d", count)
		}
	})

	// Test 6: Admin exemption (admin can create without participation)
	t.Run("Admin exemption", func(t *testing.T) {
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, adminUserID, groupID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check event creation permission for admin: %v", err)
		}
		if !canCreate {
			t.Error("Expected admin to be allowed to create events without participation")
		}
		// Count should be 0 for admin (they are exempt, so count is not checked)
		if count != 0 {
			t.Logf("Admin participation count: %d (not used for permission check)", count)
		}
	})

	// Test 7: User with exactly minimum participation
	t.Run("Success with exactly minimum participation", func(t *testing.T) {
		// Create a new user
		newUserID := int64(22222)

		// Add new user to group
		newMembership := &domain.GroupMembership{
			GroupID:  groupID,
			UserID:   newUserID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, newMembership); err != nil {
			t.Fatalf("Failed to create new user membership: %v", err)
		}

		// Create exactly 3 resolved events with this user's participation
		for i := 0; i < 3; i++ {
			event := &domain.Event{
				Question:  fmt.Sprintf("Exact test event %d", i+1),
				EventType: domain.EventTypeBinary,
				Options:   []string{"Да", "Нет"},
				Deadline:  time.Now().Add(24 * time.Hour),
				CreatedAt: time.Now(),
				Status:    domain.EventStatusActive,
				CreatedBy: adminUserID,
				GroupID:   groupID,
			}
			if err := eventManager.CreateEvent(ctx, event); err != nil {
				t.Fatalf("Failed to create exact test event %d: %v", i+1, err)
			}

			// User makes a prediction
			prediction := &domain.Prediction{
				EventID:   event.ID,
				UserID:    newUserID,
				Option:    0,
				Timestamp: time.Now(),
			}
			if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
				t.Fatalf("Failed to save prediction for exact test event %d: %v", i+1, err)
			}

			// Resolve the event
			if err := eventManager.ResolveEvent(ctx, event.ID, 0); err != nil {
				t.Fatalf("Failed to resolve exact test event %d: %v", i+1, err)
			}
		}

		// Check permission
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, newUserID, groupID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check event creation permission: %v", err)
		}
		if !canCreate {
			t.Errorf("Expected user with exactly %d participation to be allowed", minEventsToCreate)
		}
		if count != 3 {
			t.Errorf("Expected participation count 3, got %d", count)
		}
	})

	// Test 8: User with one less than minimum participation
	t.Run("Rejection with one less than minimum", func(t *testing.T) {
		// Create a new user
		almostUserID := int64(33333)

		// Add almost user to group
		almostMembership := &domain.GroupMembership{
			GroupID:  groupID,
			UserID:   almostUserID,
			JoinedAt: time.Now(),
			Status:   domain.MembershipStatusActive,
		}
		if err := groupMembershipRepo.CreateMembership(ctx, almostMembership); err != nil {
			t.Fatalf("Failed to create almost user membership: %v", err)
		}

		// Create exactly 2 resolved events with this user's participation
		for i := 0; i < 2; i++ {
			event := &domain.Event{
				Question:  fmt.Sprintf("Almost test event %d", i+1),
				EventType: domain.EventTypeBinary,
				Options:   []string{"Да", "Нет"},
				Deadline:  time.Now().Add(24 * time.Hour),
				CreatedAt: time.Now(),
				Status:    domain.EventStatusActive,
				CreatedBy: adminUserID,
				GroupID:   groupID,
			}
			if err := eventManager.CreateEvent(ctx, event); err != nil {
				t.Fatalf("Failed to create almost test event %d: %v", i+1, err)
			}

			// User makes a prediction
			prediction := &domain.Prediction{
				EventID:   event.ID,
				UserID:    almostUserID,
				Option:    0,
				Timestamp: time.Now(),
			}
			if err := predictionRepo.SavePrediction(ctx, prediction); err != nil {
				t.Fatalf("Failed to save prediction for almost test event %d: %v", i+1, err)
			}

			// Resolve the event
			if err := eventManager.ResolveEvent(ctx, event.ID, 0); err != nil {
				t.Fatalf("Failed to resolve almost test event %d: %v", i+1, err)
			}
		}

		// Check permission
		canCreate, count, err := eventPermissionValidator.CanCreateEvent(ctx, almostUserID, groupID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check event creation permission: %v", err)
		}
		if canCreate {
			t.Error("Expected user with 2 participation to be rejected (minimum is 3)")
		}
		if count != 2 {
			t.Errorf("Expected participation count 2, got %d", count)
		}
	})
}

// Integration test for event resolution permissions
func TestIntegration_EventResolutionPermissions(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
	creatorUserID := int64(11111)
	chatID := int64(67890)

	queue, groupID := setupTestGroupAndDB(t, chatID, creatorUserID)
	defer queue.Close()

	// Run migrations to ensure all tables exist
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(queue)

	// Create event manager
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)

	// Create event permission validator
	eventPermissionValidator := domain.NewEventPermissionValidator(
		eventRepo,
		predictionRepo,
		groupMembershipRepo,
		3,
		log,
	)

	// Test data
	adminUserID := int64(99999)
	otherUserID := int64(22222)
	adminIDs := []int64{adminUserID}

	// Add users to group
	creatorMembership := &domain.GroupMembership{
		GroupID:  groupID,
		UserID:   creatorUserID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := groupMembershipRepo.CreateMembership(ctx, creatorMembership); err != nil {
		t.Fatalf("Failed to create creator membership: %v", err)
	}

	adminMembership := &domain.GroupMembership{
		GroupID:  groupID,
		UserID:   adminUserID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := groupMembershipRepo.CreateMembership(ctx, adminMembership); err != nil {
		t.Fatalf("Failed to create admin membership: %v", err)
	}

	otherMembership := &domain.GroupMembership{
		GroupID:  groupID,
		UserID:   otherUserID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := groupMembershipRepo.CreateMembership(ctx, otherMembership); err != nil {
		t.Fatalf("Failed to create other user membership: %v", err)
	}

	// Create an event by the creator
	event := &domain.Event{
		Question:  "Test event for resolution permissions",
		EventType: domain.EventTypeBinary,
		Options:   []string{"Да", "Нет"},
		Deadline:  time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
		Status:    domain.EventStatusActive,
		CreatedBy: creatorUserID,
		GroupID:   groupID,
	}
	if err := eventManager.CreateEvent(ctx, event); err != nil {
		t.Fatalf("Failed to create test event: %v", err)
	}

	// Test 1: Creator can resolve their own event
	t.Run("Creator can resolve", func(t *testing.T) {
		canManage, err := eventPermissionValidator.CanManageEvent(ctx, creatorUserID, event.ID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check if creator can manage event: %v", err)
		}
		if !canManage {
			t.Error("Expected creator to be able to manage their own event")
		}
	})

	// Test 2: Admin can resolve any event
	t.Run("Admin can resolve", func(t *testing.T) {
		canManage, err := eventPermissionValidator.CanManageEvent(ctx, adminUserID, event.ID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check if admin can manage event: %v", err)
		}
		if !canManage {
			t.Error("Expected admin to be able to manage any event")
		}
	})

	// Test 3: Non-authorized user cannot resolve
	t.Run("Non-authorized user cannot resolve", func(t *testing.T) {
		canManage, err := eventPermissionValidator.CanManageEvent(ctx, otherUserID, event.ID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check if other user can manage event: %v", err)
		}
		if canManage {
			t.Error("Expected non-authorized user to NOT be able to manage event")
		}
	})

	// Test 4: Creator who is also admin can resolve
	t.Run("Creator who is also admin can resolve", func(t *testing.T) {
		// Create an event by an admin
		adminEvent := &domain.Event{
			Question:  "Event created by admin",
			EventType: domain.EventTypeBinary,
			Options:   []string{"Да", "Нет"},
			Deadline:  time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			Status:    domain.EventStatusActive,
			CreatedBy: adminUserID,
			GroupID:   groupID,
		}
		if err := eventManager.CreateEvent(ctx, adminEvent); err != nil {
			t.Fatalf("Failed to create admin event: %v", err)
		}

		canManage, err := eventPermissionValidator.CanManageEvent(ctx, adminUserID, adminEvent.ID, adminIDs)
		if err != nil {
			t.Fatalf("Failed to check if admin-creator can manage event: %v", err)
		}
		if !canManage {
			t.Error("Expected admin-creator to be able to manage their own event")
		}
	})

	// Test 5: Multiple non-authorized users cannot resolve
	t.Run("Multiple non-authorized users cannot resolve", func(t *testing.T) {
		unauthorizedUsers := []int64{33333, 44444, 55555}
		for _, userID := range unauthorizedUsers {
			canManage, err := eventPermissionValidator.CanManageEvent(ctx, userID, event.ID, adminIDs)
			if err != nil {
				t.Fatalf("Failed to check if user %d can manage event: %v", userID, err)
			}
			if canManage {
				t.Errorf("Expected user %d to NOT be able to manage event", userID)
			}
		}
	})

	// Test 6: Permission check for non-existent event
	t.Run("Permission check for non-existent event", func(t *testing.T) {
		nonExistentEventID := int64(999999)
		_, err := eventPermissionValidator.CanManageEvent(ctx, creatorUserID, nonExistentEventID, adminIDs)
		if err == nil {
			t.Error("Expected error when checking permissions for non-existent event")
		}
	})

	// Test 7: Multiple admins can all resolve
	t.Run("Multiple admins can all resolve", func(t *testing.T) {
		multipleAdminIDs := []int64{adminUserID, 88888, 77777}

		// Add other admins to group
		for _, adminID := range multipleAdminIDs {
			if adminID == adminUserID {
				continue // Already added
			}
			membership := &domain.GroupMembership{
				GroupID:  groupID,
				UserID:   adminID,
				JoinedAt: time.Now(),
				Status:   domain.MembershipStatusActive,
			}
			if err := groupMembershipRepo.CreateMembership(ctx, membership); err != nil {
				t.Fatalf("Failed to create admin %d membership: %v", adminID, err)
			}
		}

		for _, adminID := range multipleAdminIDs {
			canManage, err := eventPermissionValidator.CanManageEvent(ctx, adminID, event.ID, multipleAdminIDs)
			if err != nil {
				t.Fatalf("Failed to check if admin %d can manage event: %v", adminID, err)
			}
			if !canManage {
				t.Errorf("Expected admin %d to be able to manage event", adminID)
			}
		}
	})
}

// Integration test for creator achievement flow
func TestIntegration_CreatorAchievementFlow(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
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

	// Run migrations to ensure all tables exist
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create repositories
	eventRepo := storage.NewEventRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	ratingRepo := storage.NewRatingRepository(queue)
	achievementRepo := storage.NewAchievementRepository(queue)

	// Create event manager
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)

	// Create achievement tracker
	achievementTracker := domain.NewAchievementTracker(
		achievementRepo,
		ratingRepo,
		predictionRepo,
		eventRepo,
		log,
	)

	// Test data
	creatorUserID := int64(11111)
	groupID := int64(1) // Default group for testing

	// Test 1: Achievement awarded after first event
	t.Run("Achievement awarded after first event", func(t *testing.T) {
		// Create first event
		event1 := &domain.Event{
			Question:  "First event by creator",
			EventType: domain.EventTypeBinary,
			Options:   []string{"Да", "Нет"},
			Deadline:  time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			Status:    domain.EventStatusActive,
			CreatedBy: creatorUserID,
			GroupID:   groupID,
		}
		if err := eventManager.CreateEvent(ctx, event1); err != nil {
			t.Fatalf("Failed to create first event: %v", err)
		}

		// Check and award creator achievements
		achievements, err := achievementTracker.CheckCreatorAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to check creator achievements: %v", err)
		}

		// Should have Event Organizer achievement
		if len(achievements) != 1 {
			t.Fatalf("Expected 1 achievement after first event, got %d", len(achievements))
		}
		if achievements[0].Code != domain.AchievementEventOrganizer {
			t.Errorf("Expected Event Organizer achievement, got %s", achievements[0].Code)
		}

		// Verify achievement is persisted
		userAchievements, err := achievementTracker.GetUserAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to get user achievements: %v", err)
		}
		if len(userAchievements) != 1 {
			t.Errorf("Expected 1 persisted achievement, got %d", len(userAchievements))
		}
	})

	// Test 2: Achievement awarded after 5th event
	t.Run("Achievement awarded after 5th event", func(t *testing.T) {
		// Create 4 more events (total 5)
		for i := 2; i <= 5; i++ {
			event := &domain.Event{
				Question:  fmt.Sprintf("Event %d by creator", i),
				EventType: domain.EventTypeBinary,
				Options:   []string{"Да", "Нет"},
				Deadline:  time.Now().Add(24 * time.Hour),
				CreatedAt: time.Now(),
				Status:    domain.EventStatusActive,
				CreatedBy: creatorUserID,
				GroupID:   groupID,
			}
			if err := eventManager.CreateEvent(ctx, event); err != nil {
				t.Fatalf("Failed to create event %d: %v", i, err)
			}
		}

		// Check and award creator achievements
		achievements, err := achievementTracker.CheckCreatorAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to check creator achievements: %v", err)
		}

		// Should have Active Organizer achievement (Event Organizer already exists)
		if len(achievements) != 1 {
			t.Fatalf("Expected 1 new achievement after 5th event, got %d", len(achievements))
		}
		if achievements[0].Code != domain.AchievementActiveOrganizer {
			t.Errorf("Expected Active Organizer achievement, got %s", achievements[0].Code)
		}

		// Verify both achievements are persisted
		userAchievements, err := achievementTracker.GetUserAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to get user achievements: %v", err)
		}
		if len(userAchievements) != 2 {
			t.Errorf("Expected 2 persisted achievements, got %d", len(userAchievements))
		}

		// Verify we have both Event Organizer and Active Organizer
		achievementCodes := make(map[domain.AchievementCode]bool)
		for _, ach := range userAchievements {
			achievementCodes[ach.Code] = true
		}
		if !achievementCodes[domain.AchievementEventOrganizer] {
			t.Error("Expected Event Organizer achievement to be persisted")
		}
		if !achievementCodes[domain.AchievementActiveOrganizer] {
			t.Error("Expected Active Organizer achievement to be persisted")
		}
	})

	// Test 3: Achievement awarded after 25th event
	t.Run("Achievement awarded after 25th event", func(t *testing.T) {
		// Create 20 more events (total 25)
		for i := 6; i <= 25; i++ {
			event := &domain.Event{
				Question:  fmt.Sprintf("Event %d by creator", i),
				EventType: domain.EventTypeBinary,
				Options:   []string{"Да", "Нет"},
				Deadline:  time.Now().Add(24 * time.Hour),
				CreatedAt: time.Now(),
				Status:    domain.EventStatusActive,
				CreatedBy: creatorUserID,
				GroupID:   groupID,
			}
			if err := eventManager.CreateEvent(ctx, event); err != nil {
				t.Fatalf("Failed to create event %d: %v", i, err)
			}
		}

		// Check and award creator achievements
		achievements, err := achievementTracker.CheckCreatorAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to check creator achievements: %v", err)
		}

		// Should have Master Organizer achievement (others already exist)
		if len(achievements) != 1 {
			t.Fatalf("Expected 1 new achievement after 25th event, got %d", len(achievements))
		}
		if achievements[0].Code != domain.AchievementMasterOrganizer {
			t.Errorf("Expected Master Organizer achievement, got %s", achievements[0].Code)
		}

		// Verify all three achievements are persisted
		userAchievements, err := achievementTracker.GetUserAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to get user achievements: %v", err)
		}
		if len(userAchievements) != 3 {
			t.Errorf("Expected 3 persisted achievements, got %d", len(userAchievements))
		}

		// Verify we have all three creator achievements
		achievementCodes := make(map[domain.AchievementCode]bool)
		for _, ach := range userAchievements {
			achievementCodes[ach.Code] = true
		}
		if !achievementCodes[domain.AchievementEventOrganizer] {
			t.Error("Expected Event Organizer achievement to be persisted")
		}
		if !achievementCodes[domain.AchievementActiveOrganizer] {
			t.Error("Expected Active Organizer achievement to be persisted")
		}
		if !achievementCodes[domain.AchievementMasterOrganizer] {
			t.Error("Expected Master Organizer achievement to be persisted")
		}
	})

	// Test 4: No duplicate achievements
	t.Run("No duplicate achievements", func(t *testing.T) {
		// Check achievements again - should return empty list (no new achievements)
		achievements, err := achievementTracker.CheckCreatorAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to check creator achievements: %v", err)
		}

		if len(achievements) != 0 {
			t.Errorf("Expected 0 new achievements (all already awarded), got %d", len(achievements))
		}

		// Verify still only 3 achievements in database
		userAchievements, err := achievementTracker.GetUserAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to get user achievements: %v", err)
		}
		if len(userAchievements) != 3 {
			t.Errorf("Expected 3 persisted achievements (no duplicates), got %d", len(userAchievements))
		}
	})

	// Test 5: Different user gets separate achievements
	t.Run("Different user gets separate achievements", func(t *testing.T) {
		anotherUserID := int64(22222)

		// Create first event by another user
		event := &domain.Event{
			Question:  "First event by another creator",
			EventType: domain.EventTypeBinary,
			Options:   []string{"Да", "Нет"},
			Deadline:  time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			Status:    domain.EventStatusActive,
			CreatedBy: anotherUserID,
			GroupID:   groupID,
		}
		if err := eventManager.CreateEvent(ctx, event); err != nil {
			t.Fatalf("Failed to create event by another user: %v", err)
		}

		// Check achievements for another user
		achievements, err := achievementTracker.CheckCreatorAchievements(ctx, anotherUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to check creator achievements for another user: %v", err)
		}

		// Should have Event Organizer achievement
		if len(achievements) != 1 {
			t.Fatalf("Expected 1 achievement for another user, got %d", len(achievements))
		}
		if achievements[0].Code != domain.AchievementEventOrganizer {
			t.Errorf("Expected Event Organizer achievement for another user, got %s", achievements[0].Code)
		}

		// Verify original user still has 3 achievements
		originalUserAchievements, err := achievementTracker.GetUserAchievements(ctx, creatorUserID, groupID)
		if err != nil {
			t.Fatalf("Failed to get original user achievements: %v", err)
		}
		if len(originalUserAchievements) != 3 {
			t.Errorf("Expected original user to still have 3 achievements, got %d", len(originalUserAchievements))
		}
	})
}

// Integration test for deadline preset buttons
func TestIntegration_DeadlinePresetButtons(t *testing.T) {
	// Setup in-memory database
	ctx := context.Background()
	userID := int64(12345)
	chatID := int64(67890)

	queue, groupID := setupTestGroupAndDB(t, chatID, userID)
	defer queue.Close()

	// Create dependencies
	log := logger.New(logger.ERROR)

	// Create FSM storage
	fsmStorage := storage.NewFSMStorage(queue, log)

	// Create config with timezone
	cfg := &config.Config{
		Timezone: time.UTC,
	}

	// Test data
	question := "Will it rain next week?"

	// Test each preset button by simulating the deadline calculation logic
	presets := []struct {
		name     string
		preset   string
		expected time.Duration
	}{
		{"1 day", "1d", 24 * time.Hour},
		{"3 days", "3d", 3 * 24 * time.Hour},
		{"7 days", "7d", 7 * 24 * time.Hour},
		{"2 weeks", "14d", 14 * 24 * time.Hour},
		{"1 month", "30d", 30 * 24 * time.Hour},
		{"3 months", "90d", 90 * 24 * time.Hour},
		{"6 months", "180d", 180 * 24 * time.Hour},
		{"1 year", "365d", 365 * 24 * time.Hour},
	}

	for _, tc := range presets {
		t.Run(tc.name, func(t *testing.T) {
			// Create a session in ask_deadline state
			sessionContext := &domain.EventCreationContext{
				Question:          question,
				EventType:         domain.EventTypeBinary,
				Options:           []string{"Да", "Нет"},
				LastBotMessageID:  100,
				LastUserMessageID: 101,
				ChatID:            chatID,
				GroupID:           groupID,
			}
			if err := fsmStorage.Set(ctx, userID, StateAskDeadline, sessionContext.ToMap()); err != nil {
				t.Fatalf("Failed to create session: %v", err)
			}

			// Simulate deadline calculation based on preset (same logic as in handleDeadlinePresetCallback)
			now := time.Now().In(cfg.Timezone)
			var deadline time.Time

			switch tc.preset {
			case "1d":
				deadline = now.AddDate(0, 0, 1)
			case "3d":
				deadline = now.AddDate(0, 0, 3)
			case "7d":
				deadline = now.AddDate(0, 0, 7)
			case "14d":
				deadline = now.AddDate(0, 0, 14)
			case "30d":
				deadline = now.AddDate(0, 1, 0)
			case "90d":
				deadline = now.AddDate(0, 3, 0)
			case "180d":
				deadline = now.AddDate(0, 6, 0)
			case "365d":
				deadline = now.AddDate(1, 0, 0)
			}

			// Set time to 12:00
			deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 12, 0, 0, 0, cfg.Timezone)

			// Store deadline in context
			sessionContext.Deadline = deadline
			if err := fsmStorage.Set(ctx, userID, StateConfirm, sessionContext.ToMap()); err != nil {
				t.Fatalf("Failed to update session with deadline: %v", err)
			}

			// Verify state transitioned to confirm
			state, data, err := fsmStorage.Get(ctx, userID)
			if err != nil {
				t.Fatalf("Failed to get session after preset: %v", err)
			}
			if state != StateConfirm {
				t.Errorf("Expected state %s after preset, got %s", StateConfirm, state)
			}

			// Load context and verify deadline
			restoredContext := &domain.EventCreationContext{}
			if err := restoredContext.FromMap(data); err != nil {
				t.Fatalf("Failed to restore context: %v", err)
			}

			// Check deadline is in the future and approximately correct
			actualDeadline := restoredContext.Deadline

			// Verify deadline is in the future
			if !actualDeadline.After(now) {
				t.Errorf("Deadline should be in the future, got %s", actualDeadline.Format("02.01.2006 15:04"))
			}

			// For month-based presets, allow more tolerance due to varying month lengths
			tolerance := time.Minute
			if tc.preset == "30d" || tc.preset == "90d" || tc.preset == "180d" || tc.preset == "365d" {
				tolerance = 72 * time.Hour // 3 days tolerance for month-based calculations
			}

			expectedDeadline := now.Add(tc.expected)
			// Set to 12:00
			expectedDeadline = time.Date(expectedDeadline.Year(), expectedDeadline.Month(), expectedDeadline.Day(), 12, 0, 0, 0, cfg.Timezone)

			diff := actualDeadline.Sub(expectedDeadline)
			if diff < 0 {
				diff = -diff
			}
			if diff > tolerance {
				t.Errorf("Deadline mismatch for preset %s: expected around %s, got %s (diff: %s)",
					tc.preset, expectedDeadline.Format("02.01.2006 15:04"), actualDeadline.Format("02.01.2006 15:04"), diff)
			}

			// Verify deadline is at 12:00
			if actualDeadline.Hour() != 12 || actualDeadline.Minute() != 0 {
				t.Errorf("Expected deadline at 12:00, got %02d:%02d", actualDeadline.Hour(), actualDeadline.Minute())
			}

			// Clean up session for next test
			if err := fsmStorage.Delete(ctx, userID); err != nil {
				t.Fatalf("Failed to delete session: %v", err)
			}
		})
	}
}
