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
	StateGroupAskName   = "group_ask_name"
	StateGroupAskChatID = "group_ask_chat_id"
	StateGroupComplete  = "group_complete"
)

// GroupCreationFSM manages the group creation state machine
type GroupCreationFSM struct {
	storage         *storage.FSMStorage
	bot             *bot.Bot
	groupRepo       domain.GroupRepository
	deepLinkService *domain.DeepLinkService
	config          *config.Config
	logger          domain.Logger
}

// NewGroupCreationFSM creates a new FSM for group creation
func NewGroupCreationFSM(
	storage *storage.FSMStorage,
	b *bot.Bot,
	groupRepo domain.GroupRepository,
	deepLinkService *domain.DeepLinkService,
	cfg *config.Config,
	logger domain.Logger,
) *GroupCreationFSM {
	return &GroupCreationFSM{
		storage:         storage,
		bot:             b,
		groupRepo:       groupRepo,
		deepLinkService: deepLinkService,
		config:          cfg,
		logger:          logger,
	}
}

// Start initializes a new FSM session for group creation
func (f *GroupCreationFSM) Start(ctx context.Context, userID int64, chatID int64) error {
	// Initialize context with chat ID
	initialContext := &domain.GroupCreationContext{
		ChatID:     chatID,
		MessageIDs: []int{},
	}

	// Store initial state
	if err := f.storage.Set(ctx, userID, StateGroupAskName, initialContext.ToMap()); err != nil {
		f.logger.Error("failed to start group creation FSM session", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("group creation FSM session started", "user_id", userID, "state", StateGroupAskName)
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
	case StateGroupAskName, StateGroupAskChatID, StateGroupComplete:
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
			"Шаг 2/2: Введите ID группового чата Telegram, к которому будет привязана эта группа.\n\n" +
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

	// Delete all accumulated messages
	f.deleteMessages(ctx, chatID, context.MessageIDs...)

	// Create group
	group := &domain.Group{
		TelegramChatID: telegramChatID,
		Name:           context.GroupName,
		CreatedAt:      time.Now(),
		CreatedBy:      userID,
	}

	if err := group.Validate(); err != nil {
		f.logger.Error("group validation failed", "error", err)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка валидации группы: " + err.Error(),
		})
		// Clean up session
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	if err := f.groupRepo.CreateGroup(ctx, group); err != nil {
		f.logger.Error("failed to create group", "error", err)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при создании группы: " + err.Error(),
		})
		// Clean up session
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	f.logger.Info("group created", "user_id", userID, "group_id", group.ID, "group_name", context.GroupName)

	// Notify admins about new group creation
	f.notifyAdminsAboutGroupCreation(ctx, userID, group)

	// Generate deep-link
	deepLink, err := f.deepLinkService.GenerateGroupInviteLink(group.ID)
	if err != nil {
		f.logger.Error("failed to generate deep-link", "error", err)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при создании ссылки для приглашения",
		})
		// Clean up session
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Send success message (final message - not deleted)
	_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: fmt.Sprintf("✅ Группа создана!\n\n"+
			"📋 Название: %s\n"+
			"🆔 ID группы: %d\n"+
			"💬 ID чата: %d\n\n"+
			"🔗 Ссылка для приглашения:\n%s\n\n"+
			"Отправьте эту ссылку пользователям для присоединения к группе.",
			context.GroupName, group.ID, telegramChatID, deepLink),
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
