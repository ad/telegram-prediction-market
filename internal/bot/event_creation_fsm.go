package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// FSM state constants
const (
	StateSelectGroup  = "select_group"
	StateAskQuestion  = "ask_question"
	StateAskEventType = "ask_event_type"
	StateAskOptions   = "ask_options"
	StateAskDeadline  = "ask_deadline"
	StateConfirm      = "confirm"
	StateComplete     = "complete"
)

// EventCreationFSM manages the event creation state machine
type EventCreationFSM struct {
	storage              *storage.FSMStorage
	bot                  *bot.Bot
	eventManager         *domain.EventManager
	achievementTracker   *domain.AchievementTracker
	groupContextResolver *domain.GroupContextResolver
	groupRepo            domain.GroupRepository
	forumTopicRepo       domain.ForumTopicRepository
	ratingRepo           domain.RatingRepository
	config               *config.Config
	logger               domain.Logger
}

// NewEventCreationFSM creates a new FSM for event creation
func NewEventCreationFSM(
	storage *storage.FSMStorage,
	b *bot.Bot,
	eventManager *domain.EventManager,
	achievementTracker *domain.AchievementTracker,
	groupContextResolver *domain.GroupContextResolver,
	groupRepo domain.GroupRepository,
	forumTopicRepo domain.ForumTopicRepository,
	ratingRepo domain.RatingRepository,
	cfg *config.Config,
	logger domain.Logger,
) *EventCreationFSM {
	return &EventCreationFSM{
		storage:              storage,
		bot:                  b,
		eventManager:         eventManager,
		achievementTracker:   achievementTracker,
		groupContextResolver: groupContextResolver,
		groupRepo:            groupRepo,
		forumTopicRepo:       forumTopicRepo,
		ratingRepo:           ratingRepo,
		config:               cfg,
		logger:               logger,
	}
}

// Start initializes a new FSM session for a user
func (f *EventCreationFSM) Start(ctx context.Context, userID int64, chatID int64) error {
	// Initialize context with chat ID
	initialContext := &domain.EventCreationContext{
		ChatID: chatID,
	}

	// Try to resolve group for user
	groupID, err := f.groupContextResolver.ResolveGroupForUser(ctx, userID)
	switch err {
	case nil:
		// User has exactly one group - auto-select it
		initialContext.GroupID = groupID

		// Store initial state and skip to question
		if err := f.storage.Set(ctx, userID, StateAskQuestion, initialContext.ToMap()); err != nil {
			f.logger.Error("failed to start FSM session", "user_id", userID, "error", err)
			return err
		}

		f.logger.Info("FSM session started with auto-selected group", "user_id", userID, "group_id", groupID, "state", StateAskQuestion)

		// Send initial message
		return f.handleAskQuestion(ctx, userID, chatID)
	case domain.ErrMultipleGroupsNeedChoice:
		// User has multiple groups - need to prompt for selection
		if err := f.storage.Set(ctx, userID, StateSelectGroup, initialContext.ToMap()); err != nil {
			f.logger.Error("failed to start FSM session", "user_id", userID, "error", err)
			return err
		}

		f.logger.Info("FSM session started with group selection", "user_id", userID, "state", StateSelectGroup)

		// Send group selection prompt
		return f.handleSelectGroup(ctx, userID, chatID)
	default:
		// Error or no groups
		f.logger.Error("failed to resolve group for user", "user_id", userID, "error", err)
		return err
	}
}

// HasSession checks if a user has an active FSM session
func (f *EventCreationFSM) HasSession(ctx context.Context, userID int64) (bool, error) {
	state, _, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return false, nil
		}
		return false, err
	}

	// Only return true if the state is an event creation state
	switch state {
	case StateSelectGroup, StateAskQuestion, StateAskEventType, StateAskOptions, StateAskDeadline, StateConfirm, StateComplete:
		return true, nil
	default:
		return false, nil
	}
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
		if err == storage.ErrSessionExpired {
			f.logger.Info("session expired for user", "user_id", userID)
			// Send expiration message
			_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "⏱ Время сессии истекло. Начните заново с /create_event",
			})
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
		if err == storage.ErrSessionExpired {
			f.logger.Info("session expired for callback", "user_id", userID)
			// Answer callback query and send expiration message
			_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: callback.ID,
				Text:            "⏱ Время сессии истекло",
			})
			if callback.Message.Message != nil {
				_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: callback.Message.Message.Chat.ID,
					Text:   "⏱ Время сессии истекло. Начните заново с /create_event",
				})
			}
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
	if strings.HasPrefix(data, "select_group:") && state == StateSelectGroup {
		return f.handleGroupSelectionCallback(ctx, userID, callback, context)
	}

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

// sendMessageHTML is a helper to send a message with HTML formatting and track its ID
func (f *EventCreationFSM) sendMessageHTML(ctx context.Context, chatID int64, text string, replyMarkup models.ReplyMarkup) (int, error) {
	msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		f.logger.Error("failed to send message", "chat_id", chatID, "error", err)
		return 0, err
	}
	return msg.ID, nil
}

// handleSelectGroup sends the group selection prompt with inline keyboard
func (f *EventCreationFSM) handleSelectGroup(ctx context.Context, userID int64, chatID int64) error {
	// Get user's group choices
	groups, err := f.groupContextResolver.GetUserGroupChoices(ctx, userID)
	if err != nil {
		f.logger.Error("failed to get user group choices", "user_id", userID, "error", err)
		return err
	}

	if len(groups) == 0 {
		// This shouldn't happen as we check in Start, but handle it gracefully
		_, _ = f.sendMessage(ctx, chatID, "❌ У вас нет доступных групп для создания события.", nil)
		_ = f.storage.Delete(ctx, userID)
		return fmt.Errorf("no groups available for user")
	}

	// Build inline keyboard with group choices
	var buttons [][]models.InlineKeyboardButton
	for _, group := range groups {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         group.Name,
				CallbackData: fmt.Sprintf("select_group:%d", group.ID),
			},
		})
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	// Send message
	messageID, err := f.sendMessage(ctx, chatID, "📝 СОЗДАНИЕ НОВОГО СОБЫТИЯ\n\nВыберите группу для события:", kb)
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

	f.logger.Debug("sent group selection prompt", "user_id", userID, "message_id", messageID)
	return nil
}

// handleGroupSelectionCallback processes the group selection
func (f *EventCreationFSM) handleGroupSelectionCallback(ctx context.Context, userID int64, callback *models.CallbackQuery, context *domain.EventCreationContext) error {
	// Answer callback query to remove loading state
	_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Parse group ID from callback data
	groupIDStr := strings.TrimPrefix(callback.Data, "select_group:")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		f.logger.Error("failed to parse group ID", "user_id", userID, "data", callback.Data, "error", err)
		return err
	}

	// Store group ID in context
	context.GroupID = groupID

	// Delete the group selection message
	if callback.Message.Message != nil {
		f.deleteMessages(ctx, callback.Message.Message.Chat.ID, callback.Message.Message.ID)
	}

	// Transition to ask_question state
	chatID := callback.Message.Message.Chat.ID
	f.logger.Info("state transition", "user_id", userID, "old_state", StateSelectGroup, "new_state", StateAskQuestion)
	if err := f.storage.Set(ctx, userID, StateAskQuestion, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to ask_question", "user_id", userID, "error", err)
		return err
	}

	f.logger.Debug("group selected", "user_id", userID, "group_id", groupID)

	// Send question prompt
	return f.handleAskQuestion(ctx, userID, chatID)
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
		// Delete previous error message if it exists
		if context.LastErrorMessageID != 0 {
			f.deleteMessages(ctx, chatID, context.LastErrorMessageID)
		}

		// Delete invalid user input message
		f.deleteMessages(ctx, chatID, userMessageID)

		// Send error message and store its ID
		errorMessageID, err := f.sendMessage(ctx, chatID, "❌ Вопрос не может быть пустым. Попробуйте снова:", nil)
		if err != nil {
			return err
		}

		// Store error message ID in context
		context.LastErrorMessageID = errorMessageID

		// Save updated context
		state, _, err := f.storage.Get(ctx, userID)
		if err != nil {
			return err
		}
		if err := f.storage.Set(ctx, userID, state, context.ToMap()); err != nil {
			f.logger.Error("failed to update context with error message ID", "user_id", userID, "error", err)
			return err
		}

		return nil
	}

	// Store question in context
	context.Question = question
	context.LastUserMessageID = userMessageID

	// Delete bot message, user message, and any previous error message
	messagesToDelete := []int{context.LastBotMessageID, userMessageID}
	if context.LastErrorMessageID != 0 {
		messagesToDelete = append(messagesToDelete, context.LastErrorMessageID)
		context.LastErrorMessageID = 0 // Clear error message ID
	}
	f.deleteMessages(ctx, chatID, messagesToDelete...)

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
	f.logger.Info("state transition", "user_id", userID, "old_state", StateAskQuestion, "new_state", StateAskEventType)
	if err := f.storage.Set(ctx, userID, StateAskEventType, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to ask_event_type", "user_id", userID, "error", err)
		return err
	}

	f.logger.Debug("question stored", "user_id", userID, "question", question)
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

	var useHTML bool

	switch eventType {
	case "binary":
		context.EventType = domain.EventTypeBinary
		context.Options = []string{"Да", "Нет"}
		nextState = StateAskDeadline
		messageText = "✅ Выбран бинарный тип (Да/Нет)\n\n" + f.getDeadlinePromptMessage()
		useHTML = true

	case "probability":
		context.EventType = domain.EventTypeProbability
		context.Options = []string{"0-25%", "25-50%", "50-75%", "75-100%"}
		nextState = StateAskDeadline
		messageText = "✅ Выбран вероятностный тип\n\n" + f.getDeadlinePromptMessage()
		useHTML = true

	case "multi":
		context.EventType = domain.EventTypeMultiOption
		nextState = StateAskOptions
		messageText = "✅ Выбран множественный выбор\n\nВведите варианты ответа (2-6 штук), каждый с новой строки:"
		useHTML = false

	default:
		f.logger.Error("unknown event type", "user_id", userID, "event_type", eventType)
		return fmt.Errorf("unknown event type: %s", eventType)
	}

	// Send next message
	chatID := callback.Message.Message.Chat.ID
	var messageID int
	var err error
	if useHTML {
		messageID, err = f.sendMessageHTML(ctx, chatID, messageText, nil)
	} else {
		messageID, err = f.sendMessage(ctx, chatID, messageText, nil)
	}
	if err != nil {
		return err
	}

	// Update context with new message ID
	context.LastBotMessageID = messageID

	// Transition to next state
	f.logger.Info("state transition", "user_id", userID, "old_state", StateAskEventType, "new_state", nextState)
	if err := f.storage.Set(ctx, userID, nextState, context.ToMap()); err != nil {
		f.logger.Error("failed to transition state", "user_id", userID, "next_state", nextState, "error", err)
		return err
	}

	f.logger.Debug("event type selected", "user_id", userID, "event_type", eventType)
	return nil
}

// handleOptionsInput processes the user's options input for multi-option events
func (f *EventCreationFSM) handleOptionsInput(ctx context.Context, userID int64, chatID int64, text string, userMessageID int, context *domain.EventCreationContext) error {
	optionsText := strings.TrimSpace(text)
	if optionsText == "" {
		// Delete previous error message if it exists
		if context.LastErrorMessageID != 0 {
			f.deleteMessages(ctx, chatID, context.LastErrorMessageID)
		}

		// Delete invalid user input message
		f.deleteMessages(ctx, chatID, userMessageID)

		// Send error message and store its ID
		errorMessageID, err := f.sendMessage(ctx, chatID, "❌ Варианты не могут быть пустыми. Попробуйте снова:", nil)
		if err != nil {
			return err
		}

		// Store error message ID in context
		context.LastErrorMessageID = errorMessageID

		// Save updated context
		state, _, err := f.storage.Get(ctx, userID)
		if err != nil {
			return err
		}
		if err := f.storage.Set(ctx, userID, state, context.ToMap()); err != nil {
			f.logger.Error("failed to update context with error message ID", "user_id", userID, "error", err)
			return err
		}

		return nil
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
		// Delete previous error message if it exists
		if context.LastErrorMessageID != 0 {
			f.deleteMessages(ctx, chatID, context.LastErrorMessageID)
		}

		// Delete invalid user input message
		f.deleteMessages(ctx, chatID, userMessageID)

		// Send error message and store its ID
		errorMessageID, err := f.sendMessage(ctx, chatID, "❌ Для этого типа события нужно 2-6 вариантов. Попробуйте снова:", nil)
		if err != nil {
			return err
		}

		// Store error message ID in context
		context.LastErrorMessageID = errorMessageID

		// Save updated context
		state, _, err := f.storage.Get(ctx, userID)
		if err != nil {
			return err
		}
		if err := f.storage.Set(ctx, userID, state, context.ToMap()); err != nil {
			f.logger.Error("failed to update context with error message ID", "user_id", userID, "error", err)
			return err
		}

		return nil
	}

	// Store options in context
	context.Options = cleanOptions
	context.LastUserMessageID = userMessageID

	// Delete bot message, user message, and any previous error message
	messagesToDelete := []int{context.LastBotMessageID, userMessageID}
	if context.LastErrorMessageID != 0 {
		messagesToDelete = append(messagesToDelete, context.LastErrorMessageID)
		context.LastErrorMessageID = 0 // Clear error message ID
	}
	f.deleteMessages(ctx, chatID, messagesToDelete...)

	// Send deadline request (with HTML for example date)
	messageID, err := f.sendMessageHTML(ctx, chatID, f.getDeadlinePromptMessage(), nil)
	if err != nil {
		return err
	}

	// Update context with new message ID
	context.LastBotMessageID = messageID

	// Transition to ask_deadline state
	f.logger.Info("state transition", "user_id", userID, "old_state", StateAskOptions, "new_state", StateAskDeadline)
	if err := f.storage.Set(ctx, userID, StateAskDeadline, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to ask_deadline", "user_id", userID, "error", err)
		return err
	}

	f.logger.Debug("options stored", "user_id", userID, "options_count", len(cleanOptions))
	return nil
}

// handleDeadlineInput processes the user's deadline input
func (f *EventCreationFSM) handleDeadlineInput(ctx context.Context, userID int64, chatID int64, text string, userMessageID int, context *domain.EventCreationContext) error {
	deadlineText := strings.TrimSpace(text)

	// Parse deadline in the configured timezone
	deadline, err := time.ParseInLocation("02.01.2006 15:04", deadlineText, f.config.Timezone)
	if err != nil {
		// Delete previous error message if it exists
		if context.LastErrorMessageID != 0 {
			f.deleteMessages(ctx, chatID, context.LastErrorMessageID)
		}

		// Delete invalid user input message
		f.deleteMessages(ctx, chatID, userMessageID)

		// Send error message and store its ID
		exampleDate := time.Now().In(f.config.Timezone).AddDate(0, 0, 7)
		exampleDate = time.Date(exampleDate.Year(), exampleDate.Month(), exampleDate.Day(), 12, 0, 0, 0, f.config.Timezone)
		exampleStr := exampleDate.Format("02.01.2006 15:04")
		errorMessageID, sendErr := f.sendMessageHTML(ctx, chatID, fmt.Sprintf("❌ Неверный формат даты. Используйте: ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: <code>%s</code>", exampleStr), nil)
		if sendErr != nil {
			return sendErr
		}

		// Store error message ID in context
		context.LastErrorMessageID = errorMessageID

		// Save updated context
		state, _, err := f.storage.Get(ctx, userID)
		if err != nil {
			return err
		}
		if err := f.storage.Set(ctx, userID, state, context.ToMap()); err != nil {
			f.logger.Error("failed to update context with error message ID", "user_id", userID, "error", err)
			return err
		}

		return nil
	}

	// Validate deadline is in future
	if deadline.Before(time.Now()) {
		// Delete previous error message if it exists
		if context.LastErrorMessageID != 0 {
			f.deleteMessages(ctx, chatID, context.LastErrorMessageID)
		}

		// Delete invalid user input message
		f.deleteMessages(ctx, chatID, userMessageID)

		// Send error message and store its ID
		errorMessageID, sendErr := f.sendMessage(ctx, chatID, "❌ Дедлайн должен быть в будущем. Попробуйте снова:", nil)
		if sendErr != nil {
			return sendErr
		}

		// Store error message ID in context
		context.LastErrorMessageID = errorMessageID

		// Save updated context
		state, _, err := f.storage.Get(ctx, userID)
		if err != nil {
			return err
		}
		if err := f.storage.Set(ctx, userID, state, context.ToMap()); err != nil {
			f.logger.Error("failed to update context with error message ID", "user_id", userID, "error", err)
			return err
		}

		return nil
	}

	// Store deadline in context
	context.Deadline = deadline
	context.LastUserMessageID = userMessageID

	// Delete bot message, user message, and any previous error message
	messagesToDelete := []int{context.LastBotMessageID, userMessageID}
	if context.LastErrorMessageID != 0 {
		messagesToDelete = append(messagesToDelete, context.LastErrorMessageID)
		context.LastErrorMessageID = 0 // Clear error message ID
	}
	f.deleteMessages(ctx, chatID, messagesToDelete...)

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

	// Update context with confirmation message ID
	context.ConfirmationMessageID = messageID
	context.LastBotMessageID = messageID

	// Transition to confirm state
	f.logger.Info("state transition", "user_id", userID, "old_state", StateAskDeadline, "new_state", StateConfirm)
	if err := f.storage.Set(ctx, userID, StateConfirm, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to confirm", "user_id", userID, "error", err)
		return err
	}

	f.logger.Debug("deadline stored", "user_id", userID, "deadline", deadline)
	return nil
}

// getDeadlinePromptMessage returns the deadline prompt message with a dynamic example
func (f *EventCreationFSM) getDeadlinePromptMessage() string {
	// Calculate example date: current date + 7 days at 12:00
	exampleDate := time.Now().In(f.config.Timezone).AddDate(0, 0, 7)
	exampleDate = time.Date(exampleDate.Year(), exampleDate.Month(), exampleDate.Day(), 12, 0, 0, 0, f.config.Timezone)
	exampleStr := exampleDate.Format("02.01.2006 15:04")

	return fmt.Sprintf("📅 Введите дедлайн в формате:\nДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: <code>%s</code>", exampleStr)
}

// buildEventSummary creates a summary message with all event details (for confirmation)
func (f *EventCreationFSM) buildEventSummary(context *domain.EventCreationContext) string {
	var sb strings.Builder
	sb.WriteString("📋 ПОДТВЕРЖДЕНИЕ СОБЫТИЯ\n\n")

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

	return sb.String()
}

// buildFinalEventSummary creates a final summary message with event ID and poll reference
func (f *EventCreationFSM) buildFinalEventSummary(event *domain.Event, pollReference string) string {
	var sb strings.Builder
	sb.WriteString("✅ СОБЫТИЕ СОЗДАНО!\n\n")

	// Event ID
	sb.WriteString(fmt.Sprintf("🆔 ID: %d\n\n", event.ID))

	// Question
	sb.WriteString(fmt.Sprintf("❓ Вопрос:\n%s\n\n", event.Question))

	// Event type
	typeStr := ""
	switch event.EventType {
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
	for i, opt := range event.Options {
		sb.WriteString(fmt.Sprintf("  %d) %s\n", i+1, opt))
	}
	sb.WriteString("\n")

	// Deadline (formatted in configured timezone)
	localDeadline := event.Deadline.In(f.config.Timezone)
	sb.WriteString(fmt.Sprintf("⏰ Дедлайн: %s\n\n", localDeadline.Format("02.01.2006 15:04")))

	// Poll reference
	if pollReference != "" {
		sb.WriteString(fmt.Sprintf("📊 %s\n", pollReference))
	}

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

	// Delete the confirmation message (with buttons)
	if context.ConfirmationMessageID != 0 {
		f.deleteMessages(ctx, chatID, context.ConfirmationMessageID)
	}

	if action == "yes" {
		// Create the event
		event := &domain.Event{
			GroupID:   context.GroupID,
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

		// Get group to retrieve Telegram chat ID
		group, err := f.groupRepo.GetGroup(ctx, context.GroupID)
		if err != nil {
			f.logger.Error("failed to get group for poll", "group_id", context.GroupID, "error", err)
			_, _ = f.sendMessage(ctx, chatID, "❌ Ошибка при получении информации о группе.", nil)
			// Delete session
			_ = f.storage.Delete(ctx, userID)
			return err
		}

		// Publish poll to group using Telegram chat ID
		pollOptions := make([]models.InputPollOption, len(event.Options))
		for i, opt := range event.Options {
			pollOptions[i] = models.InputPollOption{Text: opt}
		}

		// Handle forum topic if MessageThreadID is provided
		var messageThreadID *int
		if context.MessageThreadID != nil {
			messageThreadID = context.MessageThreadID

			// Find or create forum topic
			topic, err := f.forumTopicRepo.GetForumTopicByGroupAndThread(ctx, context.GroupID, *context.MessageThreadID)
			if err != nil {
				f.logger.Error("failed to get forum topic", "group_id", context.GroupID, "message_thread_id", *context.MessageThreadID, "error", err)
			} else if topic == nil {
				// Create new forum topic
				topic = &domain.ForumTopic{
					GroupID:         context.GroupID,
					MessageThreadID: *context.MessageThreadID,
					Name:            fmt.Sprintf("Topic %d", *context.MessageThreadID),
					CreatedAt:       time.Now(),
					CreatedBy:       userID,
				}
				if err := f.forumTopicRepo.CreateForumTopic(ctx, topic); err != nil {
					f.logger.Error("failed to create forum topic", "error", err)
				} else {
					event.ForumTopicID = &topic.ID
					f.logger.Info("forum topic created for event", "topic_id", topic.ID, "message_thread_id", *context.MessageThreadID)
				}
			} else {
				event.ForumTopicID = &topic.ID
				f.logger.Info("using existing forum topic for event", "topic_id", topic.ID, "message_thread_id", *context.MessageThreadID)
			}
		}

		isAnonymous := false
		pollParams := &bot.SendPollParams{
			ChatID:                group.TelegramChatID,
			Question:              event.Question,
			Options:               pollOptions,
			IsAnonymous:           &isAnonymous,
			AllowsMultipleAnswers: false,
		}

		// Add MessageThreadID if this is a forum group
		if messageThreadID != nil {
			pollParams.MessageThreadID = *messageThreadID
		}

		pollMsg, err := f.bot.SendPoll(ctx, pollParams)
		if err != nil {
			f.logger.Error("failed to send poll", "event_id", event.ID, "group_id", context.GroupID, "telegram_chat_id", group.TelegramChatID, "message_thread_id", messageThreadID, "error", err)
			_, _ = f.sendMessage(ctx, chatID, "❌ Ошибка при публикации опроса.", nil)
			// Delete session
			_ = f.storage.Delete(ctx, userID)
			return err
		}

		// Update event with poll ID and message ID
		event.PollID = pollMsg.Poll.ID
		event.PollMessageID = pollMsg.ID
		if err := f.eventManager.UpdateEvent(ctx, event); err != nil {
			f.logger.Error("failed to update event with poll ID and message ID", "event_id", event.ID, "error", err)
		}

		// Send final summary to admin with poll reference and action buttons
		pollReference := "Опрос опубликован в группе"
		summary := f.buildFinalEventSummary(event, pollReference)

		// Add action buttons for editing and resolving
		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "✏️ Редактировать", CallbackData: fmt.Sprintf("edit_event:%d", event.ID)},
					{Text: "🏁 Завершить", CallbackData: fmt.Sprintf("resolve:%d", event.ID)},
				},
			},
		}

		_, _ = f.sendMessage(ctx, chatID, summary, kb)

		f.logger.Info("event created and published", "user_id", userID, "event_id", event.ID, "poll_id", event.PollID)

		// Check and award creator achievements (non-blocking)
		// Handle errors gracefully - don't block event creation
		achievements, err := f.achievementTracker.CheckCreatorAchievements(ctx, userID, event.GroupID)
		if err != nil {
			f.logger.Error("failed to check creator achievements", "user_id", userID, "group_id", event.GroupID, "error", err)
			// Continue - achievement check failure should not block event creation
		} else if len(achievements) > 0 {
			// Send achievement notifications
			for _, ach := range achievements {
				if err := f.sendAchievementNotification(ctx, userID, ach); err != nil {
					f.logger.Error("failed to send achievement notification", "user_id", userID, "achievement", ach.Code, "error", err)
					// Continue - notification failure should not block event creation
				}
			}
		}

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

// sendAchievementNotification sends achievement notification to user and group
func (f *EventCreationFSM) sendAchievementNotification(ctx context.Context, userID int64, achievement *domain.Achievement) error {
	achievementNames := map[domain.AchievementCode]string{
		domain.AchievementSharpshooter:    "🎯 Меткий стрелок",
		domain.AchievementProphet:         "🔮 Провидец",
		domain.AchievementRiskTaker:       "🎲 Риск-мейкер",
		domain.AchievementWeeklyAnalyst:   "📊 Аналитик недели",
		domain.AchievementVeteran:         "🏆 Старожил",
		domain.AchievementEventOrganizer:  "🎪 Организатор событий",
		domain.AchievementActiveOrganizer: "🎭 Активный организатор",
		domain.AchievementMasterOrganizer: "🎬 Мастер организатор",
	}

	name := achievementNames[achievement.Code]
	if name == "" {
		name = string(achievement.Code)
	}

	// Get group information
	group, err := f.groupRepo.GetGroup(ctx, achievement.GroupID)
	if err != nil {
		f.logger.Error("failed to get group for achievement notification", "group_id", achievement.GroupID, "error", err)
		// Continue with notification even if we can't get group name
	}

	groupName := "группе"
	if group != nil && group.Name != "" {
		groupName = fmt.Sprintf("группе \"%s\"", group.Name)
	}

	// Send to user with group context
	_, err = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: userID,
		Text:   fmt.Sprintf("🎉 Поздравляем! Вы получили ачивку в %s:\n\n%s", groupName, name),
	})
	if err != nil {
		f.logger.Error("failed to send achievement notification to user", "user_id", userID, "error", err)
		return err
	}

	// Get user display name for group announcement
	displayName := fmt.Sprintf("User id%d", userID)
	rating, err := f.ratingRepo.GetRating(ctx, userID, achievement.GroupID)
	if err == nil && rating != nil && rating.Username != "" {
		if rating.Username[0] == '@' {
			displayName = rating.Username
		} else {
			displayName = fmt.Sprintf("@%s", rating.Username)
		}
	}

	// Get the Telegram chat ID for the group to send announcement
	var telegramChatID int64
	if group != nil {
		telegramChatID = group.TelegramChatID
	} else {
		f.logger.Error("failed to send achievement announcement to group, group not provided", "error")
		return nil
	}

	// Announce in group
	// Note: Achievement notifications for event organizers are sent to the main group chat,
	// not to specific forum topics, as they are not tied to a specific event
	msgParams := &bot.SendMessageParams{
		ChatID: telegramChatID,
		Text:   fmt.Sprintf("🎉 %s получил ачивку: %s!", displayName, name),
	}

	_, err = f.bot.SendMessage(ctx, msgParams)
	if err != nil {
		f.logger.Error("failed to send achievement announcement to group", "error", err)
		return err
	}

	return nil
}
