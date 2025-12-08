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

// FSM state constants for group creation
const (
	StateGroupAskName     = "group_ask_name"
	StateGroupAskChatID   = "group_ask_chat_id"
	StateGroupAskIsForum  = "group_ask_is_forum"
	StateGroupAskThreadID = "group_ask_thread_id"
	StateGroupComplete    = "group_complete"
)

// GroupCreationFSM manages the group creation state machine
type GroupCreationFSM struct {
	storage         *storage.FSMStorage
	bot             *bot.Bot
	groupRepo       domain.GroupRepository
	forumTopicRepo  domain.ForumTopicRepository
	deepLinkService *domain.DeepLinkService
	config          *config.Config
	logger          domain.Logger
}

// NewGroupCreationFSM creates a new FSM for group creation
func NewGroupCreationFSM(
	storage *storage.FSMStorage,
	b *bot.Bot,
	groupRepo domain.GroupRepository,
	forumTopicRepo domain.ForumTopicRepository,
	deepLinkService *domain.DeepLinkService,
	cfg *config.Config,
	logger domain.Logger,
) *GroupCreationFSM {
	return &GroupCreationFSM{
		storage:         storage,
		bot:             b,
		groupRepo:       groupRepo,
		forumTopicRepo:  forumTopicRepo,
		deepLinkService: deepLinkService,
		config:          cfg,
		logger:          logger,
	}
}

// Start initializes a new FSM session for group creation
func (f *GroupCreationFSM) Start(ctx context.Context, userID int64, chatID int64) error {
	return f.StartWithForumInfo(ctx, userID, chatID, nil, false)
}

// StartWithForumInfo initializes a new FSM session for group creation with forum information
func (f *GroupCreationFSM) StartWithForumInfo(ctx context.Context, userID int64, chatID int64, messageThreadID *int, isForum bool) error {
	// Initialize context with chat ID and forum info
	initialContext := &domain.GroupCreationContext{
		ChatID:          chatID,
		MessageIDs:      []int{},
		MessageThreadID: messageThreadID,
		IsForum:         isForum,
	}

	// Store initial state
	if err := f.storage.Set(ctx, userID, StateGroupAskName, initialContext.ToMap()); err != nil {
		f.logger.Error("failed to start group creation FSM session", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("group creation FSM session started",
		"user_id", userID,
		"state", StateGroupAskName,
		"is_forum", isForum,
		"message_thread_id", messageThreadID,
	)
	return nil
}

// HasSession checks if user has an active FSM session
func (f *GroupCreationFSM) HasSession(ctx context.Context, userID int64) (bool, error) {
	state, _, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return false, nil
		}
		return false, err
	}

	// Only return true if the state is a group creation state
	switch state {
	case StateGroupAskName, StateGroupAskChatID, StateGroupAskIsForum, StateGroupAskThreadID, StateGroupComplete:
		return true, nil
	default:
		return false, nil
	}
}

// HandleMessage processes text messages for the group creation flow
func (f *GroupCreationFSM) HandleMessage(ctx context.Context, update *models.Update) error {
	userID := update.Message.From.ID

	// Get current state and context
	state, contextData, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			f.logger.Warn("no active group creation session", "user_id", userID)
			return nil
		}
		return err
	}

	// Parse context
	groupContext := &domain.GroupCreationContext{}
	if err := groupContext.FromMap(contextData); err != nil {
		f.logger.Error("failed to parse group creation context", "user_id", userID, "error", err)
		return err
	}

	// Route based on state
	switch state {
	case StateGroupAskName:
		return f.handleGroupNameInput(ctx, update, userID, groupContext)
	case StateGroupAskChatID:
		return f.handleChatIDInput(ctx, update, userID, groupContext)
	case StateGroupAskThreadID:
		return f.handleThreadIDInput(ctx, update, userID, groupContext)
	default:
		f.logger.Warn("unknown group creation state", "user_id", userID, "state", state)
		return nil
	}
}

// handleGroupNameInput processes group name input
func (f *GroupCreationFSM) handleGroupNameInput(ctx context.Context, update *models.Update, userID int64, context *domain.GroupCreationContext) error {
	chatID := update.Message.Chat.ID
	input := strings.TrimSpace(update.Message.Text)

	// Validate group name
	if input == "" {
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Название группы не может быть пустым. Попробуйте снова:",
		})
		if msg != nil {
			context.MessageIDs = append(context.MessageIDs, msg.ID)
			// Update context with new message ID
			_ = f.storage.Set(ctx, userID, StateGroupAskName, context.ToMap())
		}
		return nil
	}

	// Store group name
	context.GroupName = input

	// Send confirmation and ask for chat ID
	msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: "✅ Название сохранено: " + input + "\n\n" +
			"Введите ID группового чата Telegram, к которому будет привязана эта группа.\n\n" +
			"💡 Как получить ID чата:\n" +
			"1. Добавьте бота @userinfobot в ваш групповой чат\n" +
			"2. Он отправит ID чата (например: -1001234567890)\n" +
			"3. Скопируйте и отправьте этот ID сюда",
	})
	if err != nil {
		f.logger.Error("failed to send chat ID prompt", "error", err)
		return err
	}

	if msg != nil {
		context.MessageIDs = append(context.MessageIDs, msg.ID)
	}

	// Transition to chat ID input state
	if err := f.storage.Set(ctx, userID, StateGroupAskChatID, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to chat ID input", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("state transition", "user_id", userID, "old_state", StateGroupAskName, "new_state", StateGroupAskChatID)
	return nil
}

// handleChatIDInput processes chat ID input and creates the group
func (f *GroupCreationFSM) handleChatIDInput(ctx context.Context, update *models.Update, userID int64, context *domain.GroupCreationContext) error {
	chatID := update.Message.Chat.ID
	input := strings.TrimSpace(update.Message.Text)

	// Validate chat ID
	telegramChatID, err := strconv.ParseInt(input, 10, 64)
	if err != nil || telegramChatID == 0 {
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Неверный формат ID чата. Введите числовой ID (например: -1001234567890):",
		})
		if msg != nil {
			context.MessageIDs = append(context.MessageIDs, msg.ID)
			// Update context with new message ID
			_ = f.storage.Set(ctx, userID, StateGroupAskChatID, context.ToMap())
		}
		return nil
	}

	// Store telegram chat ID
	context.TelegramChatID = telegramChatID

	// Delete user's message
	f.deleteMessages(ctx, chatID, update.Message.ID)

	// If forum info was not auto-detected (command was sent from private chat),
	// ask if this is a forum
	if !context.IsForum && context.MessageThreadID == nil {
		// Ask if this is a forum
		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "💬 Форум", CallbackData: "group_is_forum:yes"},
					{Text: "👥 Обычная группа", CallbackData: "group_is_forum:no"},
				},
			},
		}

		msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "🗂 Является ли эта группа форумом (супергруппа с темами)?",
			ReplyMarkup: kb,
		})
		if err != nil {
			f.logger.Error("failed to send forum question", "error", err)
			return err
		}

		if msg != nil {
			context.MessageIDs = append(context.MessageIDs, msg.ID)
		}

		// Transition to ask_is_forum state
		if err := f.storage.Set(ctx, userID, StateGroupAskIsForum, context.ToMap()); err != nil {
			f.logger.Error("failed to transition to ask_is_forum", "user_id", userID, "error", err)
			return err
		}

		return nil
	}

	// Forum info was auto-detected or not needed, proceed to create group
	return f.createGroup(ctx, userID, chatID, context)
}

// createGroup creates the group with all collected information
func (f *GroupCreationFSM) createGroup(ctx context.Context, userID int64, chatID int64, context *domain.GroupCreationContext) error {
	// Delete all accumulated messages
	f.deleteMessages(ctx, chatID, context.MessageIDs...)

	// Check if group already exists for this telegram chat ID
	existingGroup, err := f.groupRepo.GetGroupByTelegramChatID(ctx, context.TelegramChatID)
	if err != nil {
		f.logger.Error("failed to check existing group", "error", err)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при проверке существующей группы: " + err.Error(),
		})
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	var group *domain.Group
	isNewGroup := existingGroup == nil

	if isNewGroup {
		// Create new group
		group = &domain.Group{
			TelegramChatID: context.TelegramChatID,
			Name:           context.GroupName,
			CreatedAt:      time.Now(),
			CreatedBy:      userID,
			IsForum:        context.IsForum,
		}

		if err := group.Validate(); err != nil {
			f.logger.Error("group validation failed", "error", err)
			_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Ошибка валидации группы: " + err.Error(),
			})
			_ = f.storage.Delete(ctx, userID)
			return err
		}

		if err := f.groupRepo.CreateGroup(ctx, group); err != nil {
			f.logger.Error("failed to create group", "error", err)
			_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Ошибка при создании группы: " + err.Error(),
			})
			_ = f.storage.Delete(ctx, userID)
			return err
		}

		f.logger.Info("group created", "user_id", userID, "group_id", group.ID, "group_name", context.GroupName)
		f.notifyAdminsAboutGroupCreation(ctx, userID, group)
	} else {
		// Use existing group
		group = existingGroup
		f.logger.Info("using existing group", "user_id", userID, "group_id", group.ID, "group_name", group.Name)
	}

	// If this is a forum and we have a thread ID, create/update forum topic
	var topicCreated bool
	if context.IsForum && context.MessageThreadID != nil {
		// Check if topic already exists
		existingTopic, err := f.forumTopicRepo.GetForumTopicByGroupAndThread(ctx, group.ID, *context.MessageThreadID)
		if err != nil {
			f.logger.Error("failed to check existing forum topic", "error", err)
		} else if existingTopic == nil {
			// Create new forum topic
			topic := &domain.ForumTopic{
				GroupID:         group.ID,
				MessageThreadID: *context.MessageThreadID,
				Name:            context.GroupName,
				CreatedAt:       time.Now(),
				CreatedBy:       userID,
			}

			if err := topic.Validate(); err != nil {
				f.logger.Error("forum topic validation failed", "error", err)
			} else if err := f.forumTopicRepo.CreateForumTopic(ctx, topic); err != nil {
				f.logger.Error("failed to create forum topic", "error", err)
			} else {
				f.logger.Info("forum topic created",
					"topic_id", topic.ID,
					"group_id", group.ID,
					"message_thread_id", *context.MessageThreadID,
				)
				topicCreated = true
			}
		} else {
			f.logger.Info("forum topic already exists",
				"topic_id", existingTopic.ID,
				"group_id", group.ID,
				"message_thread_id", *context.MessageThreadID,
			)
			topicCreated = true
		}
	}

	// Generate deep-link
	deepLink, err := f.deepLinkService.GenerateGroupInviteLink(group.ID)
	if err != nil {
		f.logger.Error("failed to generate deep-link", "error", err)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при создании ссылки для приглашения",
		})
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Build success message
	var successMsg string
	if isNewGroup {
		successMsg = fmt.Sprintf("✅ Группа создана!\n\n"+
			"📋 Название: %s\n"+
			"🆔 ID группы: %d\n"+
			"🆔 ID чата: %d\n",
			context.GroupName, group.ID, context.TelegramChatID)
	} else {
		successMsg = fmt.Sprintf("✅ Используется существующая группа!\n\n"+
			"📋 Название: %s\n"+
			"🆔 ID группы: %d\n"+
			"🆔 ID чата: %d\n",
			group.Name, group.ID, context.TelegramChatID)
	}

	if context.IsForum {
		successMsg += "🗂 Тип: Форум\n"
		if context.MessageThreadID != nil {
			successMsg += fmt.Sprintf("📍 ID темы: %d\n", *context.MessageThreadID)
			if topicCreated {
				successMsg += "\n✨ Тема форума зарегистрирована! События будут отправляться в эту тему.\n"
			}
		}
	} else {
		successMsg += "🗂 Тип: Обычная группа\n"
	}

	successMsg += fmt.Sprintf("\n🔗 Ссылка для присоединения к группе:\n%s\n\n"+
		"👉 Перейдите по этой ссылке для создания Событий в группе.\n"+
		"Отправьте эту ссылку пользователям для присоединения к группе.", deepLink)

	// Send success message (final message - not deleted)
	_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   successMsg,
	})

	// Clean up session
	if err := f.storage.Delete(ctx, userID); err != nil {
		f.logger.Error("failed to delete group creation session", "user_id", userID, "error", err)
	}

	f.logger.Info("group creation FSM session completed", "user_id", userID, "group_id", group.ID)
	return nil
}

// deleteMessages is a helper to delete multiple messages
func (f *GroupCreationFSM) deleteMessages(ctx context.Context, chatID int64, messageIDs ...int) {
	deleteMessages(ctx, f.bot, f.logger, chatID, messageIDs...)
}

// notifyAdminsAboutGroupCreation sends notification to all admins about new group creation
func (f *GroupCreationFSM) notifyAdminsAboutGroupCreation(ctx context.Context, creatorUserID int64, group *domain.Group) {
	// Get creator's username from bot API if possible
	creatorName := fmt.Sprintf("ID: %d", creatorUserID)

	// Try to get user info
	chat, err := f.bot.GetChat(ctx, &bot.GetChatParams{ChatID: creatorUserID})
	if err == nil && chat != nil {
		if chat.Username != "" {
			creatorName = fmt.Sprintf("@%s", chat.Username)
		} else if chat.FirstName != "" {
			creatorName = chat.FirstName
			if chat.LastName != "" {
				creatorName += " " + chat.LastName
			}
		}
	}

	notificationMsg := fmt.Sprintf(
		"🎉 СОЗДАНА НОВАЯ ГРУППА\n\n"+
			"👤 Создатель: %s\n"+
			"📋 Название: %s\n"+
			"🆔 ID группы: %d\n"+
			"💬 ID чата: %d",
		creatorName,
		group.Name,
		group.ID,
		group.TelegramChatID,
	)

	// Send notification to all admins
	for _, adminID := range f.config.AdminUserIDs {
		_, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: adminID,
			Text:   notificationMsg,
		})
		if err != nil {
			f.logger.Error("failed to send admin notification about group creation", "admin_id", adminID, "error", err)
		}
	}
}

// HandleCallback handles callback queries for group creation flow
func (f *GroupCreationFSM) HandleCallback(ctx context.Context, callback *models.CallbackQuery) error {
	userID := callback.From.ID
	data := callback.Data

	// Get current state
	state, contextData, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			f.logger.Debug("no active group creation session for callback", "user_id", userID)
			return nil
		}
		return err
	}

	// Load context
	groupContext := &domain.GroupCreationContext{}
	if err := groupContext.FromMap(contextData); err != nil {
		f.logger.Error("failed to parse group creation context for callback", "user_id", userID, "error", err)
		return err
	}

	// Route based on callback data and state
	if strings.HasPrefix(data, "group_is_forum:") && state == StateGroupAskIsForum {
		return f.handleIsForumCallback(ctx, userID, callback, groupContext)
	}

	f.logger.Warn("unexpected callback in group creation", "user_id", userID, "state", state, "data", data)
	return nil
}

// handleIsForumCallback handles the forum yes/no callback
func (f *GroupCreationFSM) handleIsForumCallback(ctx context.Context, userID int64, callback *models.CallbackQuery, context *domain.GroupCreationContext) error {
	chatID := callback.Message.Message.Chat.ID
	answer := strings.TrimPrefix(callback.Data, "group_is_forum:")

	// Answer callback query
	_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Delete the question message
	if callback.Message.Message != nil {
		f.deleteMessages(ctx, chatID, callback.Message.Message.ID)
	}

	if answer == "yes" {
		// This is a forum - ask for thread ID
		context.IsForum = true

		msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: "📍 Введите ID темы форума:\n\n" +
				"Чтобы узнать ID темы:\n" +
				"1. Откройте тему в веб-версии Telegram (web.telegram.org)\n" +
				"2. URL будет вида: https://web.telegram.org/k/#-1001234567890_123\n" +
				"3. Число после '_' - это ID темы (в примере: 123)\n\n" +
				"Или отправьте 0, если хотите пропустить (события будут в основной чат):",
		})
		if err != nil {
			f.logger.Error("failed to send thread ID prompt", "error", err)
			return err
		}

		if msg != nil {
			context.MessageIDs = append(context.MessageIDs, msg.ID)
		}

		// Transition to ask_thread_id state
		if err := f.storage.Set(ctx, userID, StateGroupAskThreadID, context.ToMap()); err != nil {
			f.logger.Error("failed to transition to ask_thread_id", "user_id", userID, "error", err)
			return err
		}

		return nil
	}

	// Not a forum - proceed to create group
	context.IsForum = false
	context.MessageThreadID = nil

	return f.createGroup(ctx, userID, chatID, context)
}

// handleThreadIDInput processes thread ID input
func (f *GroupCreationFSM) handleThreadIDInput(ctx context.Context, update *models.Update, userID int64, context *domain.GroupCreationContext) error {
	chatID := update.Message.Chat.ID
	input := strings.TrimSpace(update.Message.Text)

	// Parse thread ID
	threadID, err := strconv.Atoi(input)
	if err != nil {
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Неверный формат ID темы. Введите число (например: 123) или 0 для пропуска:",
		})
		if msg != nil {
			context.MessageIDs = append(context.MessageIDs, msg.ID)
			_ = f.storage.Set(ctx, userID, StateGroupAskThreadID, context.ToMap())
		}
		return nil
	}

	// Store thread ID (if not 0)
	if threadID != 0 {
		context.MessageThreadID = &threadID
	} else {
		context.MessageThreadID = nil
	}

	// Delete user's message
	f.deleteMessages(ctx, chatID, update.Message.ID)

	// Proceed to create group
	return f.createGroup(ctx, userID, chatID, context)
}
