package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"telegram-prediction-bot/internal/config"
	"telegram-prediction-bot/internal/domain"
	"telegram-prediction-bot/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// FSM state constants
const (
	StateAskQuestion  = "ask_question"
	StateAskEventType = "ask_event_type"
	StateAskOptions   = "ask_options"
	StateAskDeadline  = "ask_deadline"
	StateConfirm      = "confirm"
	StateComplete     = "complete"
)

// EventCreationFSM manages the event creation state machine
type EventCreationFSM struct {
	storage      *storage.FSMStorage
	bot          *bot.Bot
	eventManager *domain.EventManager
	config       *config.Config
	logger       domain.Logger
}

// NewEventCreationFSM creates a new FSM for event creation
func NewEventCreationFSM(
	storage *storage.FSMStorage,
	b *bot.Bot,
	eventManager *domain.EventManager,
	cfg *config.Config,
	logger domain.Logger,
) *EventCreationFSM {
	return &EventCreationFSM{
		storage:      storage,
		bot:          b,
		eventManager: eventManager,
		config:       cfg,
		logger:       logger,
	}
}

// Start initializes a new FSM session for a user
func (f *EventCreationFSM) Start(ctx context.Context, userID int64, chatID int64) error {
	// Initialize context with chat ID
	initialContext := &domain.EventCreationContext{
		ChatID: chatID,
	}

	// Store initial state
	if err := f.storage.Set(ctx, userID, StateAskQuestion, initialContext.ToMap()); err != nil {
		f.logger.Error("failed to start FSM session", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("FSM session started", "user_id", userID, "state", StateAskQuestion)

	// Send initial message
	return f.handleAskQuestion(ctx, userID, chatID)
}

// HasSession checks if a user has an active FSM session
func (f *EventCreationFSM) HasSession(ctx context.Context, userID int64) (bool, error) {
	_, _, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// HandleMessage routes messages to the appropriate state handler
func (f *EventCreationFSM) HandleMessage(ctx context.Context, update *models.Update) error {
	if update.Message == nil || update.Message.From == nil {
		return nil
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Get current state
	state, data, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			f.logger.Debug("no active session for user", "user_id", userID)
			return nil
		}
		f.logger.Error("failed to get session", "user_id", userID, "error", err)
		return err
	}

	// Load context
	context := &domain.EventCreationContext{}
	if err := context.FromMap(data); err != nil {
		f.logger.Error("failed to load context", "user_id", userID, "error", err)
		// Delete corrupted session
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Route to appropriate handler based on state
	switch state {
	case StateAskQuestion:
		return f.handleQuestionInput(ctx, userID, chatID, update.Message.Text, update.Message.ID, context)
	case StateAskOptions:
		return f.handleOptionsInput(ctx, userID, chatID, update.Message.Text, update.Message.ID, context)
	case StateAskDeadline:
		return f.handleDeadlineInput(ctx, userID, chatID, update.Message.Text, update.Message.ID, context)
	default:
		f.logger.Warn("unexpected state for message", "user_id", userID, "state", state)
		return nil
	}
}

// HandleCallback routes callback queries to the appropriate handler
func (f *EventCreationFSM) HandleCallback(ctx context.Context, callback *models.CallbackQuery) error {
	userID := callback.From.ID
	data := callback.Data

	// Get current state
	state, contextData, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			f.logger.Debug("no active session for callback", "user_id", userID)
			return nil
		}
		f.logger.Error("failed to get session for callback", "user_id", userID, "error", err)
		return err
	}

	// Load context
	context := &domain.EventCreationContext{}
	if err := context.FromMap(contextData); err != nil {
		f.logger.Error("failed to load context for callback", "user_id", userID, "error", err)
		// Delete corrupted session
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Route based on callback data and state
	if strings.HasPrefix(data, "event_type:") && state == StateAskEventType {
		return f.handleEventTypeCallback(ctx, userID, callback, context)
	}

	if strings.HasPrefix(data, "confirm:") && state == StateConfirm {
		return f.handleConfirmCallback(ctx, userID, callback, context)
	}

	f.logger.Warn("unexpected callback", "user_id", userID, "state", state, "data", data)
	return nil
}

// deleteMessages is a helper to delete multiple messages
func (f *EventCreationFSM) deleteMessages(ctx context.Context, chatID int64, messageIDs ...int) {
	deleteMessages(ctx, f.bot, f.logger, chatID, messageIDs...)
}

// sendMessage is a helper to send a message and track its ID
func (f *EventCreationFSM) sendMessage(ctx context.Context, chatID int64, text string, replyMarkup models.ReplyMarkup) (int, error) {
	msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		f.logger.Error("failed to send message", "chat_id", chatID, "error", err)
		return 0, err
	}
	return msg.ID, nil
}

// handleAskQuestion sends the initial question prompt
func (f *EventCreationFSM) handleAskQuestion(ctx context.Context, userID int64, chatID int64) error {
	// Send message
	messageID, err := f.sendMessage(ctx, chatID, "📝 СОЗДАНИЕ НОВОГО СОБЫТИЯ\n\nВведите вопрос для прогноза:", nil)
	if err != nil {
		return err
	}

	// Update context with message ID
	state, data, err := f.storage.Get(ctx, userID)
	if err != nil {
		return err
	}

	context := &domain.EventCreationContext{}
	if err := context.FromMap(data); err != nil {
		return err
	}

	context.LastBotMessageID = messageID

	// Save updated context
	if err := f.storage.Set(ctx, userID, state, context.ToMap()); err != nil {
		f.logger.Error("failed to update context with message ID", "user_id", userID, "error", err)
		return err
	}

	f.logger.Debug("sent question prompt", "user_id", userID, "message_id", messageID)
	return nil
}

// handleQuestionInput processes the user's question input
func (f *EventCreationFSM) handleQuestionInput(ctx context.Context, userID int64, chatID int64, text string, userMessageID int, context *domain.EventCreationContext) error {
	// Validate question is not empty
	question := strings.TrimSpace(text)
	if question == "" {
		_, err := f.sendMessage(ctx, chatID, "❌ Вопрос не может быть пустым. Попробуйте снова:", nil)
		return err
	}

	// Store question in context
	context.Question = question
	context.LastUserMessageID = userMessageID

	// Delete bot message and user message
	f.deleteMessages(ctx, chatID, context.LastBotMessageID, userMessageID)

	// Send event type selection with inline keyboard
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Бинарное (Да/Нет)", CallbackData: "event_type:binary"},
			},
			{
				{Text: "Множественный выбор", CallbackData: "event_type:multi"},
			},
			{
				{Text: "Вероятностное", CallbackData: "event_type:probability"},
			},
		},
	}

	messageID, err := f.sendMessage(ctx, chatID, "Выберите тип события:", kb)
	if err != nil {
		return err
	}

	// Update context with new message ID
	context.LastBotMessageID = messageID

	// Transition to ask_event_type state
	if err := f.storage.Set(ctx, userID, StateAskEventType, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to ask_event_type", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("question stored, transitioned to ask_event_type", "user_id", userID, "question", question)
	return nil
}

// handleEventTypeCallback processes the event type selection
func (f *EventCreationFSM) handleEventTypeCallback(ctx context.Context, userID int64, callback *models.CallbackQuery, context *domain.EventCreationContext) error {
	// Answer callback query to remove loading state
	_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Parse event type from callback data
	eventType := strings.TrimPrefix(callback.Data, "event_type:")

	// Delete bot message
	if callback.Message.Message != nil {
		f.deleteMessages(ctx, callback.Message.Message.Chat.ID, callback.Message.Message.ID)
	}

	var nextState string
	var messageText string

	switch eventType {
	case "binary":
		context.EventType = domain.EventTypeBinary
		context.Options = []string{"Да", "Нет"}
		nextState = StateAskDeadline
		messageText = "✅ Выбран бинарный тип (Да/Нет)\n\n📅 Введите дедлайн в формате:\n   ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: 25.12.2024 18:00"

	case "probability":
		context.EventType = domain.EventTypeProbability
		context.Options = []string{"0-25%", "25-50%", "50-75%", "75-100%"}
		nextState = StateAskDeadline
		messageText = "✅ Выбран вероятностный тип\n\n📅 Введите дедлайн в формате:\n   ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: 25.12.2024 18:00"

	case "multi":
		context.EventType = domain.EventTypeMultiOption
		nextState = StateAskOptions
		messageText = "✅ Выбран множественный выбор\n\nВведите варианты ответа (2-6 штук), каждый с новой строки:"

	default:
		f.logger.Error("unknown event type", "user_id", userID, "event_type", eventType)
		return fmt.Errorf("unknown event type: %s", eventType)
	}

	// Send next message
	chatID := callback.Message.Message.Chat.ID
	messageID, err := f.sendMessage(ctx, chatID, messageText, nil)
	if err != nil {
		return err
	}

	// Update context with new message ID
	context.LastBotMessageID = messageID

	// Transition to next state
	if err := f.storage.Set(ctx, userID, nextState, context.ToMap()); err != nil {
		f.logger.Error("failed to transition state", "user_id", userID, "next_state", nextState, "error", err)
		return err
	}

	f.logger.Info("event type selected, transitioned", "user_id", userID, "event_type", eventType, "next_state", nextState)
	return nil
}

// handleOptionsInput processes the user's options input for multi-option events
func (f *EventCreationFSM) handleOptionsInput(ctx context.Context, userID int64, chatID int64, text string, userMessageID int, context *domain.EventCreationContext) error {
	optionsText := strings.TrimSpace(text)
	if optionsText == "" {
		_, err := f.sendMessage(ctx, chatID, "❌ Варианты не могут быть пустыми. Попробуйте снова:", nil)
		return err
	}

	// Parse options (one per line)
	options := strings.Split(optionsText, "\n")
	var cleanOptions []string
	for _, opt := range options {
		opt = strings.TrimSpace(opt)
		if opt != "" {
			cleanOptions = append(cleanOptions, opt)
		}
	}

	// Validate options count (2-6)
	if len(cleanOptions) < 2 || len(cleanOptions) > 6 {
		_, err := f.sendMessage(ctx, chatID, "❌ Для этого типа события нужно 2-6 вариантов. Попробуйте снова:", nil)
		return err
	}

	// Store options in context
	context.Options = cleanOptions
	context.LastUserMessageID = userMessageID

	// Delete bot message and user message
	f.deleteMessages(ctx, chatID, context.LastBotMessageID, userMessageID)

	// Send deadline request
	messageID, err := f.sendMessage(ctx, chatID, "📅 Введите дедлайн в формате:\n   ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: 25.12.2024 18:00", nil)
	if err != nil {
		return err
	}

	// Update context with new message ID
	context.LastBotMessageID = messageID

	// Transition to ask_deadline state
	if err := f.storage.Set(ctx, userID, StateAskDeadline, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to ask_deadline", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("options stored, transitioned to ask_deadline", "user_id", userID, "options_count", len(cleanOptions))
	return nil
}

// handleDeadlineInput processes the user's deadline input
func (f *EventCreationFSM) handleDeadlineInput(ctx context.Context, userID int64, chatID int64, text string, userMessageID int, context *domain.EventCreationContext) error {
	deadlineText := strings.TrimSpace(text)

	// Parse deadline in the configured timezone
	deadline, err := time.ParseInLocation("02.01.2006 15:04", deadlineText, f.config.Timezone)
	if err != nil {
		_, sendErr := f.sendMessage(ctx, chatID, "❌ Неверный формат даты. Используйте: ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: 25.12.2024 18:00", nil)
		return sendErr
	}

	// Validate deadline is in future
	if deadline.Before(time.Now()) {
		_, sendErr := f.sendMessage(ctx, chatID, "❌ Дедлайн должен быть в будущем. Попробуйте снова:", nil)
		return sendErr
	}

	// Store deadline in context
	context.Deadline = deadline
	context.LastUserMessageID = userMessageID

	// Delete bot message and user message
	f.deleteMessages(ctx, chatID, context.LastBotMessageID, userMessageID)

	// Build summary message with all event details
	summary := f.buildEventSummary(context)

	// Send summary with confirmation keyboard
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Подтвердить", CallbackData: "confirm:yes"},
				{Text: "❌ Отменить", CallbackData: "confirm:no"},
			},
		},
	}

	messageID, err := f.sendMessage(ctx, chatID, summary, kb)
	if err != nil {
		return err
	}

	// Update context with new message ID
	context.LastBotMessageID = messageID

	// Transition to confirm state
	if err := f.storage.Set(ctx, userID, StateConfirm, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to confirm", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("deadline stored, transitioned to confirm", "user_id", userID, "deadline", deadline)
	return nil
}

// buildEventSummary creates a summary message with all event details
func (f *EventCreationFSM) buildEventSummary(context *domain.EventCreationContext) string {
	var sb strings.Builder
	sb.WriteString("📋 ПОДТВЕРЖДЕНИЕ СОБЫТИЯ\n")
	sb.WriteString("════════════════════\n\n")

	sb.WriteString(fmt.Sprintf("❓ Вопрос:\n%s\n\n", context.Question))

	// Event type
	typeStr := ""
	switch context.EventType {
	case domain.EventTypeBinary:
		typeStr = "Бинарное (Да/Нет)"
	case domain.EventTypeMultiOption:
		typeStr = "Множественный выбор"
	case domain.EventTypeProbability:
		typeStr = "Вероятностное"
	}
	sb.WriteString(fmt.Sprintf("🎯 Тип: %s\n\n", typeStr))

	// Options
	sb.WriteString("📊 Варианты:\n")
	for i, opt := range context.Options {
		sb.WriteString(fmt.Sprintf("  %d) %s\n", i+1, opt))
	}
	sb.WriteString("\n")

	// Deadline
	localDeadline := context.Deadline.In(f.config.Timezone)
	sb.WriteString(fmt.Sprintf("⏰ Дедлайн: %s\n\n", localDeadline.Format("02.01.2006 15:04")))

	sb.WriteString("════════════════════\n")
	sb.WriteString("Подтвердите создание события:")

	return sb.String()
}

// handleConfirmCallback processes the confirmation or cancellation
func (f *EventCreationFSM) handleConfirmCallback(ctx context.Context, userID int64, callback *models.CallbackQuery, context *domain.EventCreationContext) error {
	// Answer callback query to remove loading state
	_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	chatID := callback.Message.Message.Chat.ID
	action := strings.TrimPrefix(callback.Data, "confirm:")

	if action == "yes" {
		// Create the event
		event := &domain.Event{
			Question:  context.Question,
			EventType: context.EventType,
			Options:   context.Options,
			Deadline:  context.Deadline,
			CreatedAt: time.Now(),
			Status:    domain.EventStatusActive,
			CreatedBy: userID,
		}

		if err := f.eventManager.CreateEvent(ctx, event); err != nil {
			f.logger.Error("failed to create event", "user_id", userID, "error", err)
			_, _ = f.sendMessage(ctx, chatID, "❌ Ошибка при создании события.", nil)
			// Delete session
			_ = f.storage.Delete(ctx, userID)
			return err
		}

		// Publish poll to group
		pollOptions := make([]models.InputPollOption, len(event.Options))
		for i, opt := range event.Options {
			pollOptions[i] = models.InputPollOption{Text: opt}
		}

		isAnonymous := false
		pollMsg, err := f.bot.SendPoll(ctx, &bot.SendPollParams{
			ChatID:                f.config.GroupID,
			Question:              event.Question,
			Options:               pollOptions,
			IsAnonymous:           &isAnonymous,
			AllowsMultipleAnswers: false,
		})
		if err != nil {
			f.logger.Error("failed to send poll", "event_id", event.ID, "error", err)
			_, _ = f.sendMessage(ctx, chatID, "❌ Ошибка при публикации опроса.", nil)
			// Delete session
			_ = f.storage.Delete(ctx, userID)
			return err
		}

		// Update event with poll ID
		event.PollID = pollMsg.Poll.ID
		if err := f.eventManager.UpdateEvent(ctx, event); err != nil {
			f.logger.Error("failed to update event with poll ID", "event_id", event.ID, "error", err)
		}

		// Send final summary to admin
		localDeadline := event.Deadline.In(f.config.Timezone)
		summary := fmt.Sprintf("✅ СОБЫТИЕ СОЗДАНО!\n\n▸ ID: %d\n▸ Вопрос: %s\n▸ Дедлайн: %s\n▸ Опрос опубликован в группе",
			event.ID, event.Question, localDeadline.Format("02.01.2006 15:04"))
		_, _ = f.sendMessage(ctx, chatID, summary, nil)

		f.logger.Info("event created and published", "user_id", userID, "event_id", event.ID, "poll_id", event.PollID)

		// Delete session
		if err := f.storage.Delete(ctx, userID); err != nil {
			f.logger.Error("failed to delete session after completion", "user_id", userID, "error", err)
		}

		return nil
	}

	if action == "no" {
		// Send cancellation message
		_, _ = f.sendMessage(ctx, chatID, "❌ Создание события отменено.", nil)

		f.logger.Info("event creation cancelled", "user_id", userID)

		// Delete session
		if err := f.storage.Delete(ctx, userID); err != nil {
			f.logger.Error("failed to delete session after cancellation", "user_id", userID, "error", err)
			return err
		}

		return nil
	}

	f.logger.Warn("unknown confirmation action", "user_id", userID, "action", action)
	return nil
}
