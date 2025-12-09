package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/locale"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// FSM state constants for rename operations
const (
	StateRenameGroupAwaitName = "rename_group_await_name"
	StateRenameTopicAwaitName = "rename_topic_await_name"
)

// RenameFSM manages the rename state machine
type RenameFSM struct {
	storage        *storage.FSMStorage
	bot            *bot.Bot
	groupRepo      domain.GroupRepository
	forumTopicRepo domain.ForumTopicRepository
	logger         domain.Logger
	localizer      locale.Localizer
}

// NewRenameFSM creates a new FSM for rename operations
func NewRenameFSM(
	storage *storage.FSMStorage,
	b *bot.Bot,
	groupRepo domain.GroupRepository,
	forumTopicRepo domain.ForumTopicRepository,
	logger domain.Logger,
	localizer locale.Localizer,
) *RenameFSM {
	return &RenameFSM{
		storage:        storage,
		bot:            b,
		groupRepo:      groupRepo,
		forumTopicRepo: forumTopicRepo,
		logger:         logger,
		localizer:      localizer,
	}
}

// StartGroupRename initializes a new FSM session for group renaming
func (f *RenameFSM) StartGroupRename(ctx context.Context, userID int64, chatID int64, groupID int64, oldName string) error {
	renameContext := map[string]interface{}{
		"chat_id":  chatID,
		"group_id": groupID,
		"old_name": oldName,
	}

	if err := f.storage.Set(ctx, userID, StateRenameGroupAwaitName, renameContext); err != nil {
		f.logger.Error("failed to start group rename FSM session", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("group rename FSM session started", "user_id", userID, "group_id", groupID)
	return nil
}

// StartTopicRename initializes a new FSM session for topic renaming
func (f *RenameFSM) StartTopicRename(ctx context.Context, userID int64, chatID int64, topicID int64, oldName string) error {
	renameContext := map[string]interface{}{
		"chat_id":  chatID,
		"topic_id": topicID,
		"old_name": oldName,
	}

	if err := f.storage.Set(ctx, userID, StateRenameTopicAwaitName, renameContext); err != nil {
		f.logger.Error("failed to start topic rename FSM session", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("topic rename FSM session started", "user_id", userID, "topic_id", topicID)
	return nil
}

// HasSession checks if user has an active rename FSM session
func (f *RenameFSM) HasSession(ctx context.Context, userID int64) (bool, error) {
	state, _, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return false, nil
		}
		return false, err
	}

	// Only return true if the state is a rename state
	switch state {
	case StateRenameGroupAwaitName, StateRenameTopicAwaitName:
		return true, nil
	}

	return false, nil
}

// HandleMessage processes text messages for rename flow
func (f *RenameFSM) HandleMessage(ctx context.Context, update *models.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	newName := strings.TrimSpace(update.Message.Text)

	// Get current state and context
	state, contextData, err := f.storage.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get FSM state: %w", err)
	}

	// Validate new name
	if newName == "" {
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   f.localizer.MustLocalize(locale.RenameErrorEmptyName),
		})
		return nil
	}

	if len(newName) > 100 {
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   f.localizer.MustLocalize(locale.RenameErrorNameTooLong),
		})
		return nil
	}

	switch state {
	case StateRenameGroupAwaitName:
		return f.handleGroupRename(ctx, userID, chatID, newName, contextData)
	case StateRenameTopicAwaitName:
		return f.handleTopicRename(ctx, userID, chatID, newName, contextData)
	}

	return nil
}

// handleGroupRename processes group rename
func (f *RenameFSM) handleGroupRename(ctx context.Context, userID int64, chatID int64, newName string, contextData map[string]interface{}) error {
	// Get group ID from context
	groupIDFloat, ok := contextData["group_id"].(float64)
	if !ok {
		f.logger.Error("failed to get group_id from context", "context", contextData)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   f.localizer.MustLocalize(locale.RenameErrorGetContext),
		})
		_ = f.storage.Delete(ctx, userID)
		return fmt.Errorf("invalid group_id in context")
	}
	groupID := int64(groupIDFloat)

	oldName, _ := contextData["old_name"].(string)

	// Update group name
	err := f.groupRepo.UpdateGroupName(ctx, groupID, newName)
	if err != nil {
		f.logger.Error("failed to update group name", "group_id", groupID, "error", err)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   f.localizer.MustLocalize(locale.RenameErrorUpdateGroup),
		})
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	f.logger.Info("group renamed", "user_id", userID, "group_id", groupID, "old_name", oldName, "new_name", newName)

	// Send confirmation
	_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   f.localizer.MustLocalizeWithTemplate(locale.RenameGroupSuccess, oldName, newName),
	})

	// Clear session
	_ = f.storage.Delete(ctx, userID)
	return nil
}

// handleTopicRename processes topic rename
func (f *RenameFSM) handleTopicRename(ctx context.Context, userID int64, chatID int64, newName string, contextData map[string]interface{}) error {
	// Get topic ID from context
	topicIDFloat, ok := contextData["topic_id"].(float64)
	if !ok {
		f.logger.Error("failed to get topic_id from context", "context", contextData)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   f.localizer.MustLocalize(locale.RenameErrorGetContext),
		})
		_ = f.storage.Delete(ctx, userID)
		return fmt.Errorf("invalid topic_id in context")
	}
	topicID := int64(topicIDFloat)

	oldName, _ := contextData["old_name"].(string)

	// Update topic name
	err := f.forumTopicRepo.UpdateForumTopicName(ctx, topicID, newName)
	if err != nil {
		f.logger.Error("failed to update topic name", "topic_id", topicID, "error", err)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   f.localizer.MustLocalize(locale.RenameErrorUpdateTopic),
		})
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	f.logger.Info("topic renamed", "user_id", userID, "topic_id", topicID, "old_name", oldName, "new_name", newName)

	// Send confirmation
	_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   f.localizer.MustLocalizeWithTemplate(locale.RenameTopicSuccess, oldName, newName),
	})

	// Clear session
	_ = f.storage.Delete(ctx, userID)
	return nil
}
