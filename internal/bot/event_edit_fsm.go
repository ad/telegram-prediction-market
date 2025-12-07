package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// FSM state constants for event editing
const (
	StateEditSelectField = "edit_select_field"
	StateEditQuestion    = "edit_question"
	StateEditOptions     = "edit_options"
	StateEditDeadline    = "edit_deadline"
	StateEditConfirm     = "edit_confirm"
)

// EventEditContext holds data during event editing flow
type EventEditContext struct {
	EventID            int64            `json:"event_id"`
	ChatID             int64            `json:"chat_id"`
	OriginalQuestion   string           `json:"original_question"`
	OriginalOptions    []string         `json:"original_options"`
	OriginalDeadline   time.Time        `json:"original_deadline"`
	NewQuestion        string           `json:"new_question"`
	NewOptions         []string         `json:"new_options"`
	NewDeadline        time.Time        `json:"new_deadline"`
	LastBotMessageID   int              `json:"last_bot_message_id"`
	LastErrorMessageID int              `json:"last_error_message_id"`
	GroupID            int64            `json:"group_id"`
	EventType          domain.EventType `json:"event_type"`
}

// ToMap converts EventEditContext to a map for JSON serialization
func (c *EventEditContext) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"event_id":              c.EventID,
		"chat_id":               c.ChatID,
		"original_question":     c.OriginalQuestion,
		"original_options":      c.OriginalOptions,
		"original_deadline":     c.OriginalDeadline.Format(time.RFC3339),
		"new_question":          c.NewQuestion,
		"new_options":           c.NewOptions,
		"new_deadline":          c.NewDeadline.Format(time.RFC3339),
		"last_bot_message_id":   c.LastBotMessageID,
		"last_error_message_id": c.LastErrorMessageID,
		"group_id":              c.GroupID,
		"event_type":            string(c.EventType),
	}
}

// FromMap populates EventEditContext from a map after JSON deserialization
func (c *EventEditContext) FromMap(data map[string]interface{}) error {
	if data == nil {
		return domain.ErrInvalidContextData
	}

	if eventID, ok := data["event_id"].(float64); ok {
		c.EventID = int64(eventID)
	}
	if chatID, ok := data["chat_id"].(float64); ok {
		c.ChatID = int64(chatID)
	}
	if groupID, ok := data["group_id"].(float64); ok {
		c.GroupID = int64(groupID)
	}
	if question, ok := data["original_question"].(string); ok {
		c.OriginalQuestion = question
	}
	if question, ok := data["new_question"].(string); ok {
		c.NewQuestion = question
	}
	if eventType, ok := data["event_type"].(string); ok {
		c.EventType = domain.EventType(eventType)
	}

	// Parse options
	if options, ok := data["original_options"].([]interface{}); ok {
		c.OriginalOptions = make([]string, len(options))
		for i, opt := range options {
			if optStr, ok := opt.(string); ok {
				c.OriginalOptions[i] = optStr
			}
		}
	}
	if options, ok := data["new_options"].([]interface{}); ok {
		c.NewOptions = make([]string, len(options))
		for i, opt := range options {
			if optStr, ok := opt.(string); ok {
				c.NewOptions[i] = optStr
			}
		}
	}

	// Parse deadlines
	if deadlineStr, ok := data["original_deadline"].(string); ok {
		if deadline, err := time.Parse(time.RFC3339, deadlineStr); err == nil {
			c.OriginalDeadline = deadline
		}
	}
	if deadlineStr, ok := data["new_deadline"].(string); ok {
		if deadline, err := time.Parse(time.RFC3339, deadlineStr); err == nil {
			c.NewDeadline = deadline
		}
	}

	if msgID, ok := data["last_bot_message_id"].(float64); ok {
		c.LastBotMessageID = int(msgID)
	}
	if msgID, ok := data["last_error_message_id"].(float64); ok {
		c.LastErrorMessageID = int(msgID)
	}

	return nil
}

// EventEditFSM manages the event editing state machine
type EventEditFSM struct {
	storage        *storage.FSMStorage
	bot            *bot.Bot
	eventManager   *domain.EventManager
	groupRepo      domain.GroupRepository
	forumTopicRepo domain.ForumTopicRepository
	config         *config.Config
	logger         domain.Logger
}

// NewEventEditFSM creates a new FSM for event editing
func NewEventEditFSM(
	storage *storage.FSMStorage,
	b *bot.Bot,
	eventManager *domain.EventManager,
	groupRepo domain.GroupRepository,
	forumTopicRepo domain.ForumTopicRepository,
	cfg *config.Config,
	logger domain.Logger,
) *EventEditFSM {
	return &EventEditFSM{
		storage:        storage,
		bot:            b,
		eventManager:   eventManager,
		groupRepo:      groupRepo,
		forumTopicRepo: forumTopicRepo,
		config:         cfg,
		logger:         logger,
	}
}

// Start initializes a new FSM session for editing an event
func (f *EventEditFSM) Start(ctx context.Context, userID int64, chatID int64, eventID int64) error {
	// Get the event
	event, err := f.eventManager.GetEvent(ctx, eventID)
	if err != nil {
		f.logger.Error("failed to get event for editing", "event_id", eventID, "error", err)
		return err
	}

	// Check if event can be edited (no votes)
	canEdit, err := f.eventManager.CanEditEvent(ctx, eventID)
	if err != nil {
		f.logger.Error("failed to check if event can be edited", "event_id", eventID, "error", err)
		return err
	}

	if !canEdit {
		f.logger.Info("event cannot be edited - has votes", "event_id", eventID)
		return domain.ErrEventHasVotes
	}

	// Initialize context
	editContext := &EventEditContext{
		EventID:          eventID,
		ChatID:           chatID,
		OriginalQuestion: event.Question,
		OriginalOptions:  event.Options,
		OriginalDeadline: event.Deadline,
		NewQuestion:      event.Question,
		NewOptions:       event.Options,
		NewDeadline:      event.Deadline,
		GroupID:          event.GroupID,
		EventType:        event.EventType,
	}

	if err := f.storage.Set(ctx, userID, StateEditSelectField, editContext.ToMap()); err != nil {
		f.logger.Error("failed to start edit FSM session", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("edit FSM session started", "user_id", userID, "event_id", eventID)

	// Send field selection menu
	return f.sendFieldSelectionMenu(ctx, userID, chatID, editContext)
}

// sendFieldSelectionMenu sends the menu to select which field to edit
func (f *EventEditFSM) sendFieldSelectionMenu(ctx context.Context, userID int64, chatID int64, editCtx *EventEditContext) error {
	// Build current state summary
	var sb strings.Builder
	sb.WriteString("✏️ РЕДАКТИРОВАНИЕ СОБЫТИЯ\n\n")
	sb.WriteString(fmt.Sprintf("❓ Вопрос:\n%s\n\n", editCtx.NewQuestion))

	// Only show options for multi-option events
	if editCtx.EventType == domain.EventTypeMultiOption {
		sb.WriteString("📊 Варианты:\n")
		for i, opt := range editCtx.NewOptions {
			sb.WriteString(fmt.Sprintf("  %d) %s\n", i+1, opt))
		}
		sb.WriteString("\n")
	}

	localDeadline := editCtx.NewDeadline.In(f.config.Timezone)
	sb.WriteString(fmt.Sprintf("⏰ Дедлайн: %s\n\n", localDeadline.Format("02.01.2006 15:04")))
	sb.WriteString("Выберите, что хотите изменить:")

	// Build keyboard based on event type
	var buttons [][]models.InlineKeyboardButton
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "📝 Вопрос", CallbackData: fmt.Sprintf("edit_field:question:%d", editCtx.EventID)},
	})

	// Only allow editing options for multi-option events
	if editCtx.EventType == domain.EventTypeMultiOption {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: "📊 Варианты", CallbackData: fmt.Sprintf("edit_field:options:%d", editCtx.EventID)},
		})
	}

	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "⏰ Дедлайн", CallbackData: fmt.Sprintf("edit_field:deadline:%d", editCtx.EventID)},
	})
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "✅ Сохранить", CallbackData: fmt.Sprintf("edit_field:save:%d", editCtx.EventID)},
		{Text: "❌ Отмена", CallbackData: fmt.Sprintf("edit_field:cancel:%d", editCtx.EventID)},
	})

	kb := &models.InlineKeyboardMarkup{InlineKeyboard: buttons}

	msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        sb.String(),
		ReplyMarkup: kb,
	})
	if err != nil {
		f.logger.Error("failed to send field selection menu", "error", err)
		return err
	}

	// Update context with message ID
	editCtx.LastBotMessageID = msg.ID
	if err := f.storage.Set(ctx, userID, StateEditSelectField, editCtx.ToMap()); err != nil {
		f.logger.Error("failed to update context with message ID", "error", err)
	}

	return nil
}

// HasSession checks if a user has an active edit FSM session
func (f *EventEditFSM) HasSession(ctx context.Context, userID int64) (bool, error) {
	state, _, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return false, nil
		}
		return false, err
	}

	switch state {
	case StateEditSelectField, StateEditQuestion, StateEditOptions, StateEditDeadline, StateEditConfirm:
		return true, nil
	}
	return false, nil
}

// HandleCallback routes callback queries to the appropriate handler
func (f *EventEditFSM) HandleCallback(ctx context.Context, callback *models.CallbackQuery) error {
	userID := callback.From.ID
	data := callback.Data

	// Get current state
	state, contextData, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return nil
		}
		if err == storage.ErrSessionExpired {
			_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: callback.ID,
				Text:            "⏱ Время сессии истекло",
			})
			return nil
		}
		return err
	}

	// Load context
	editCtx := &EventEditContext{}
	if err := editCtx.FromMap(contextData); err != nil {
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Handle field selection callbacks
	if strings.HasPrefix(data, "edit_field:") && state == StateEditSelectField {
		return f.handleFieldSelectionCallback(ctx, userID, callback, editCtx)
	}

	// Handle deadline preset callbacks
	if strings.HasPrefix(data, "edit_deadline_preset:") && state == StateEditDeadline {
		return f.handleDeadlinePresetCallback(ctx, userID, callback, editCtx)
	}

	return nil
}

// handleFieldSelectionCallback processes field selection
func (f *EventEditFSM) handleFieldSelectionCallback(ctx context.Context, userID int64, callback *models.CallbackQuery, editCtx *EventEditContext) error {
	_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Parse field from callback data: edit_field:FIELD:EVENT_ID
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid callback data")
	}
	field := parts[1]
	chatID := callback.Message.Message.Chat.ID

	// Delete previous message
	if callback.Message.Message != nil {
		deleteMessages(ctx, f.bot, f.logger, chatID, callback.Message.Message.ID)
	}

	switch field {
	case "question":
		return f.promptEditQuestion(ctx, userID, chatID, editCtx)
	case "options":
		return f.promptEditOptions(ctx, userID, chatID, editCtx)
	case "deadline":
		return f.promptEditDeadline(ctx, userID, chatID, editCtx)
	case "save":
		return f.saveChanges(ctx, userID, chatID, editCtx)
	case "cancel":
		return f.cancelEdit(ctx, userID, chatID)
	}

	return nil
}

func (f *EventEditFSM) promptEditQuestion(ctx context.Context, userID int64, chatID int64, editCtx *EventEditContext) error {
	msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("📝 Текущий вопрос:\n%s\n\nВведите новый вопрос:", editCtx.NewQuestion),
	})
	if err != nil {
		return err
	}

	editCtx.LastBotMessageID = msg.ID
	return f.storage.Set(ctx, userID, StateEditQuestion, editCtx.ToMap())
}

func (f *EventEditFSM) promptEditOptions(ctx context.Context, userID int64, chatID int64, editCtx *EventEditContext) error {
	var sb strings.Builder
	sb.WriteString("📊 Текущие варианты:\n")
	for i, opt := range editCtx.NewOptions {
		sb.WriteString(fmt.Sprintf("  %d) %s\n", i+1, opt))
	}
	sb.WriteString("\nВведите новые варианты (2-6 штук), каждый с новой строки:")

	msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
	})
	if err != nil {
		return err
	}

	editCtx.LastBotMessageID = msg.ID
	return f.storage.Set(ctx, userID, StateEditOptions, editCtx.ToMap())
}

func (f *EventEditFSM) promptEditDeadline(ctx context.Context, userID int64, chatID int64, editCtx *EventEditContext) error {
	localDeadline := editCtx.NewDeadline.In(f.config.Timezone)
	exampleDate := time.Now().In(f.config.Timezone).AddDate(0, 0, 7)
	exampleDate = time.Date(exampleDate.Year(), exampleDate.Month(), exampleDate.Day(), 12, 0, 0, 0, f.config.Timezone)

	text := fmt.Sprintf("⏰ Текущий дедлайн: %s\n\n📅 Введите новый дедлайн в формате:\nДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: <code>%s</code>\n\nИли выберите готовый период:",
		localDeadline.Format("02.01.2006 15:04"),
		exampleDate.Format("02.01.2006 15:04"))

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "1 день", CallbackData: fmt.Sprintf("edit_deadline_preset:1d:%d", editCtx.EventID)}},
			{{Text: "3 дня", CallbackData: fmt.Sprintf("edit_deadline_preset:3d:%d", editCtx.EventID)}},
			{{Text: "1 неделя", CallbackData: fmt.Sprintf("edit_deadline_preset:7d:%d", editCtx.EventID)}},
			{{Text: "2 недели", CallbackData: fmt.Sprintf("edit_deadline_preset:14d:%d", editCtx.EventID)}},
			{{Text: "1 месяц", CallbackData: fmt.Sprintf("edit_deadline_preset:30d:%d", editCtx.EventID)}},
		},
	}

	msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: kb,
	})
	if err != nil {
		return err
	}

	editCtx.LastBotMessageID = msg.ID
	return f.storage.Set(ctx, userID, StateEditDeadline, editCtx.ToMap())
}

func (f *EventEditFSM) handleDeadlinePresetCallback(ctx context.Context, userID int64, callback *models.CallbackQuery, editCtx *EventEditContext) error {
	_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Parse preset: edit_deadline_preset:PRESET:EVENT_ID
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid callback data")
	}
	preset := parts[1]
	chatID := callback.Message.Message.Chat.ID

	now := time.Now().In(f.config.Timezone)
	var deadline time.Time

	switch preset {
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
	default:
		return fmt.Errorf("unknown preset: %s", preset)
	}

	deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 12, 0, 0, 0, f.config.Timezone)
	editCtx.NewDeadline = deadline

	// Delete previous message
	if callback.Message.Message != nil {
		deleteMessages(ctx, f.bot, f.logger, chatID, callback.Message.Message.ID)
	}

	// Return to field selection
	return f.sendFieldSelectionMenu(ctx, userID, chatID, editCtx)
}

// HandleMessage routes messages to the appropriate state handler
func (f *EventEditFSM) HandleMessage(ctx context.Context, update *models.Update) error {
	if update.Message == nil || update.Message.From == nil {
		return nil
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)

	state, contextData, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound || err == storage.ErrSessionExpired {
			return nil
		}
		return err
	}

	editCtx := &EventEditContext{}
	if err := editCtx.FromMap(contextData); err != nil {
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	switch state {
	case StateEditQuestion:
		return f.handleQuestionInput(ctx, userID, chatID, text, update.Message.ID, editCtx)
	case StateEditOptions:
		return f.handleOptionsInput(ctx, userID, chatID, text, update.Message.ID, editCtx)
	case StateEditDeadline:
		return f.handleDeadlineInput(ctx, userID, chatID, text, update.Message.ID, editCtx)
	}

	return nil
}

func (f *EventEditFSM) handleQuestionInput(ctx context.Context, userID int64, chatID int64, text string, userMsgID int, editCtx *EventEditContext) error {
	// Delete bot and user messages
	deleteMessages(ctx, f.bot, f.logger, chatID, editCtx.LastBotMessageID, userMsgID)

	if text == "" {
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Вопрос не может быть пустым. Попробуйте снова:",
		})
		editCtx.LastErrorMessageID = msg.ID
		return f.storage.Set(ctx, userID, StateEditQuestion, editCtx.ToMap())
	}

	editCtx.NewQuestion = text
	return f.sendFieldSelectionMenu(ctx, userID, chatID, editCtx)
}

func (f *EventEditFSM) handleOptionsInput(ctx context.Context, userID int64, chatID int64, text string, userMsgID int, editCtx *EventEditContext) error {
	// Delete bot and user messages
	deleteMessages(ctx, f.bot, f.logger, chatID, editCtx.LastBotMessageID, userMsgID)

	if text == "" {
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Варианты не могут быть пустыми. Попробуйте снова:",
		})
		editCtx.LastErrorMessageID = msg.ID
		return f.storage.Set(ctx, userID, StateEditOptions, editCtx.ToMap())
	}

	// Parse options
	lines := strings.Split(text, "\n")
	var options []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			options = append(options, line)
		}
	}

	if len(options) < 2 || len(options) > 6 {
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Нужно 2-6 вариантов. Попробуйте снова:",
		})
		editCtx.LastErrorMessageID = msg.ID
		return f.storage.Set(ctx, userID, StateEditOptions, editCtx.ToMap())
	}

	editCtx.NewOptions = options
	return f.sendFieldSelectionMenu(ctx, userID, chatID, editCtx)
}

func (f *EventEditFSM) handleDeadlineInput(ctx context.Context, userID int64, chatID int64, text string, userMsgID int, editCtx *EventEditContext) error {
	// Delete bot and user messages
	deleteMessages(ctx, f.bot, f.logger, chatID, editCtx.LastBotMessageID, userMsgID)

	deadline, err := time.ParseInLocation("02.01.2006 15:04", text, f.config.Timezone)
	if err != nil {
		exampleDate := time.Now().In(f.config.Timezone).AddDate(0, 0, 7)
		exampleStr := exampleDate.Format("02.01.2006 15:04")
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("❌ Неверный формат. Используйте: ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: <code>%s</code>", exampleStr),
			ParseMode: models.ParseModeHTML,
		})
		editCtx.LastErrorMessageID = msg.ID
		return f.storage.Set(ctx, userID, StateEditDeadline, editCtx.ToMap())
	}

	if deadline.Before(time.Now()) {
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Дедлайн должен быть в будущем. Попробуйте снова:",
		})
		editCtx.LastErrorMessageID = msg.ID
		return f.storage.Set(ctx, userID, StateEditDeadline, editCtx.ToMap())
	}

	editCtx.NewDeadline = deadline
	return f.sendFieldSelectionMenu(ctx, userID, chatID, editCtx)
}

func (f *EventEditFSM) saveChanges(ctx context.Context, userID int64, chatID int64, editCtx *EventEditContext) error {
	// Get the event
	event, err := f.eventManager.GetEvent(ctx, editCtx.EventID)
	if err != nil {
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при получении события.",
		})
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Check again if event can be edited
	canEdit, err := f.eventManager.CanEditEvent(ctx, editCtx.EventID)
	if err != nil || !canEdit {
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Событие больше нельзя редактировать — появились голоса.",
		})
		_ = f.storage.Delete(ctx, userID)
		return domain.ErrEventHasVotes
	}

	// Update event fields
	event.Question = editCtx.NewQuestion
	event.Options = editCtx.NewOptions
	event.Deadline = editCtx.NewDeadline

	if err := f.eventManager.UpdateEvent(ctx, event); err != nil {
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при сохранении изменений.",
		})
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Update poll in group if needed
	if err := f.updatePollInGroup(ctx, event); err != nil {
		f.logger.Error("failed to update poll in group", "event_id", event.ID, "error", err)
		// Don't fail - event is already updated
	}

	// Send success message
	var sb strings.Builder
	sb.WriteString("✅ СОБЫТИЕ ОБНОВЛЕНО!\n\n")
	sb.WriteString(fmt.Sprintf("🆔 ID: %d\n\n", event.ID))
	sb.WriteString(fmt.Sprintf("❓ Вопрос:\n%s\n\n", event.Question))
	sb.WriteString("📊 Варианты:\n")
	for i, opt := range event.Options {
		sb.WriteString(fmt.Sprintf("  %d) %s\n", i+1, opt))
	}
	localDeadline := event.Deadline.In(f.config.Timezone)
	sb.WriteString(fmt.Sprintf("\n⏰ Дедлайн: %s\n", localDeadline.Format("02.01.2006 15:04")))

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✏️ Изменить", CallbackData: fmt.Sprintf("edit_event:%d", event.ID)},
				{Text: "🏁 Завершить", CallbackData: fmt.Sprintf("resolve:%d", event.ID)},
			},
		},
	}

	_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        sb.String(),
		ReplyMarkup: kb,
	})

	f.logger.Info("event edited successfully", "user_id", userID, "event_id", editCtx.EventID)
	_ = f.storage.Delete(ctx, userID)
	return nil
}

func (f *EventEditFSM) updatePollInGroup(ctx context.Context, event *domain.Event) error {
	// Get group to retrieve Telegram chat ID
	group, err := f.groupRepo.GetGroup(ctx, event.GroupID)
	if err != nil {
		return err
	}

	// Get MessageThreadID from ForumTopic if event has one
	var messageThreadID *int
	if event.ForumTopicID != nil {
		topic, err := f.forumTopicRepo.GetForumTopic(ctx, *event.ForumTopicID)
		if err != nil {
			f.logger.Error("failed to get forum topic", "forum_topic_id", *event.ForumTopicID, "error", err)
		} else if topic != nil {
			messageThreadID = &topic.MessageThreadID
			f.logger.Debug("found forum topic for event", "event_id", event.ID, "message_thread_id", topic.MessageThreadID)
		}
	}

	// Delete the old poll message (cleaner than stopping it)
	if event.PollMessageID != 0 {
		_, err := f.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    group.TelegramChatID,
			MessageID: event.PollMessageID,
		})
		if err != nil {
			f.logger.Warn("failed to delete old poll", "event_id", event.ID, "error", err)
			// Continue - we'll try to send a new poll anyway
		}
	}

	// Create new poll with updated data
	pollOptions := make([]models.InputPollOption, len(event.Options))
	for i, opt := range event.Options {
		pollOptions[i] = models.InputPollOption{Text: opt}
	}

	isAnonymous := false
	disableNotification := true
	pollParams := &bot.SendPollParams{
		ChatID:                group.TelegramChatID,
		Question:              event.Question,
		Options:               pollOptions,
		IsAnonymous:           &isAnonymous,
		AllowsMultipleAnswers: false,
		DisableNotification:   disableNotification,
		ProtectContent:        true,
	}

	// Add MessageThreadID for forum groups
	if messageThreadID != nil && *messageThreadID != 0 {
		pollParams.MessageThreadID = *messageThreadID
		f.logger.Debug("sending updated poll to forum topic", "event_id", event.ID, "message_thread_id", *messageThreadID)
	}

	pollMsg, err := f.bot.SendPoll(ctx, pollParams)
	if err != nil {
		return err
	}

	// Update event with new poll ID and message ID
	event.PollID = pollMsg.Poll.ID
	event.PollMessageID = pollMsg.ID
	if err := f.eventManager.UpdateEvent(ctx, event); err != nil {
		f.logger.Error("failed to update event with new poll ID", "event_id", event.ID, "error", err)
	}

	f.logger.Info("poll updated in group", "event_id", event.ID, "new_poll_id", event.PollID, "new_message_id", event.PollMessageID)
	return nil
}

func (f *EventEditFSM) cancelEdit(ctx context.Context, userID int64, chatID int64) error {
	_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "❌ Редактирование отменено.",
	})
	f.logger.Info("event edit cancelled", "user_id", userID)
	return f.storage.Delete(ctx, userID)
}

// deleteMessages is a helper to delete multiple messages
func (f *EventEditFSM) deleteMessages(ctx context.Context, chatID int64, messageIDs ...int) {
	deleteMessages(ctx, f.bot, f.logger, chatID, messageIDs...)
}
