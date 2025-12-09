package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/locale"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// BotHandler handles all Telegram bot interactions
type BotHandler struct {
	bot                      *bot.Bot
	eventManager             *domain.EventManager
	ratingCalculator         *domain.RatingCalculator
	achievementTracker       *domain.AchievementTracker
	predictionRepo           domain.PredictionRepository
	config                   *config.Config
	logger                   domain.Logger
	eventCreationFSM         *EventCreationFSM
	eventResolutionFSM       *EventResolutionFSM
	groupCreationFSM         *GroupCreationFSM
	renameFSM                *RenameFSM
	eventEditFSM             *EventEditFSM
	eventPermissionValidator *domain.EventPermissionValidator
	groupRepo                domain.GroupRepository
	groupMembershipRepo      domain.GroupMembershipRepository
	forumTopicRepo           domain.ForumTopicRepository
	deepLinkService          *domain.DeepLinkService
	groupContextResolver     *domain.GroupContextResolver
	ratingRepo               domain.RatingRepository
	localizer                locale.Localizer
}

// NewBotHandler creates a new BotHandler with all dependencies
func NewBotHandler(
	b *bot.Bot,
	eventManager *domain.EventManager,
	ratingCalculator *domain.RatingCalculator,
	achievementTracker *domain.AchievementTracker,
	predictionRepo domain.PredictionRepository,
	cfg *config.Config,
	logger domain.Logger,
	eventCreationFSM *EventCreationFSM,
	eventResolutionFSM *EventResolutionFSM,
	groupCreationFSM *GroupCreationFSM,
	renameFSM *RenameFSM,
	eventEditFSM *EventEditFSM,
	eventPermissionValidator *domain.EventPermissionValidator,
	groupRepo domain.GroupRepository,
	groupMembershipRepo domain.GroupMembershipRepository,
	forumTopicRepo domain.ForumTopicRepository,
	deepLinkService *domain.DeepLinkService,
	groupContextResolver *domain.GroupContextResolver,
	ratingRepo domain.RatingRepository,
	localizer locale.Localizer,
) *BotHandler {
	return &BotHandler{
		bot:                      b,
		eventManager:             eventManager,
		ratingCalculator:         ratingCalculator,
		achievementTracker:       achievementTracker,
		predictionRepo:           predictionRepo,
		config:                   cfg,
		logger:                   logger,
		eventCreationFSM:         eventCreationFSM,
		eventResolutionFSM:       eventResolutionFSM,
		groupCreationFSM:         groupCreationFSM,
		renameFSM:                renameFSM,
		eventEditFSM:             eventEditFSM,
		eventPermissionValidator: eventPermissionValidator,
		groupRepo:                groupRepo,
		groupMembershipRepo:      groupMembershipRepo,
		forumTopicRepo:           forumTopicRepo,
		deepLinkService:          deepLinkService,
		groupContextResolver:     groupContextResolver,
		ratingRepo:               ratingRepo,
		localizer:                localizer,
	}
}

// isAdmin checks if a user ID is in the admin list
func (h *BotHandler) isAdmin(userID int64) bool {
	for _, adminID := range h.config.AdminUserIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}

// getUserDisplayName retrieves user display name (username, first name, or ID)
// It tries username first (format: @username), falls back to first name if username not available,
// and falls back to "User [UserID]" if neither available
func (h *BotHandler) getUserDisplayName(ctx context.Context, userID int64, groupID int64) string {
	// Try to get user information from the bot API
	// Since we don't have direct access to the bot API's GetChat method for users,
	// we'll use the rating repository which stores username information
	rating, err := h.ratingCalculator.GetUserRating(ctx, userID, groupID)
	if err != nil {
		// If we can't get the rating, fall back to user ID
		return fmt.Sprintf("User id%d", userID)
	}

	// Try username first
	if rating.Username != "" {
		// Check if username already has @ prefix
		if rating.Username[0] == '@' {
			return rating.Username
		}
		return fmt.Sprintf("@%s", rating.Username)
	}

	// Fall back to "User [UserID]"
	return fmt.Sprintf("User id%d", userID)
}

// requireAdmin is a middleware that checks if the user is an admin
// Returns true if authorized, false otherwise (and sends error message)
func (h *BotHandler) requireAdmin(ctx context.Context, update *models.Update) bool {
	var userID int64

	if update.Message != nil && update.Message.From != nil {
		userID = update.Message.From.ID
	} else if update.CallbackQuery != nil {
		userID = update.CallbackQuery.From.ID
	} else {
		return false
	}

	if !h.isAdmin(userID) {
		h.logger.Warn("unauthorized admin command attempt", "user_id", userID)

		// Send error message
		if update.Message != nil {
			_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ErrorUnauthorized),
			})
			if err != nil {
				h.logger.Error("failed to send unauthorized message", "error", err)
			}
		}

		return false
	}

	return true
}

// logAdminAction logs an admin action to the logger
func (h *BotHandler) logAdminAction(userID int64, action string, eventID int64, details string) {
	h.logger.Info("admin action",
		"admin_user_id", userID,
		"action", action,
		"event_id", eventID,
		"details", details,
		"timestamp", time.Now(),
	)
}

// notifyAdminsWithKeyboard sends a notification message with inline keyboard to all bot admins
func (h *BotHandler) notifyAdminsWithKeyboard(ctx context.Context, message string, keyboard *models.InlineKeyboardMarkup) {
	for _, adminID := range h.config.AdminUserIDs {
		_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      adminID,
			Text:        message,
			ReplyMarkup: keyboard,
			ParseMode:   models.ParseModeHTML,
		})
		if err != nil {
			h.logger.Error("failed to send admin notification with keyboard", "admin_id", adminID, "error", err)
		}
	}
}

// handleSessionConflictCallback handles user's choice when there's a conflicting session
func (h *BotHandler) handleSessionConflictCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Message.Chat.ID
	data := callback.Data

	// Answer callback query to remove loading state
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Delete the conflict message
	if callback.Message.Message != nil {
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: callback.Message.Message.ID,
		})
	}

	if data == "session_conflict:continue" {
		// User wants to continue the existing session
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.SessionContinuePrevious),
		})
		h.logger.Info("user chose to continue existing session", "user_id", userID)
		return
	}

	if strings.HasPrefix(data, "session_conflict:restart:") {
		// User wants to restart with a new session
		sessionType := strings.TrimPrefix(data, "session_conflict:restart:")

		// Delete the old session
		if err := h.eventCreationFSM.storage.Delete(ctx, userID); err != nil {
			h.logger.Error("failed to delete old session", "user_id", userID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.SessionErrorDelete),
			})
			return
		}

		h.logger.Info("old session deleted, starting new session", "user_id", userID, "new_type", sessionType)

		// Start the new session based on type
		switch sessionType {
		case "event_creation":
			// Recreate the update to call HandleCreateEvent
			newUpdate := &models.Update{
				Message: &models.Message{
					From: &callback.From,
					Chat: models.Chat{ID: chatID},
					Text: "/create_event",
				},
			}
			h.HandleCreateEvent(ctx, b, newUpdate)

		case "group_creation":
			// Recreate the update to call HandleCreateGroup
			newUpdate := &models.Update{
				Message: &models.Message{
					From: &callback.From,
					Chat: models.Chat{ID: chatID},
					Text: "/create_group",
				},
			}
			h.HandleCreateGroup(ctx, b, newUpdate)

		case "event_resolution":
			// Recreate the update to call HandleResolveEvent
			newUpdate := &models.Update{
				Message: &models.Message{
					From: &callback.From,
					Chat: models.Chat{ID: chatID},
					Text: "/resolve_event",
				},
			}
			h.HandleResolveEvent(ctx, b, newUpdate)

		default:
			h.logger.Error("unknown session type for restart", "type", sessionType)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.SessionErrorUnknown),
			})
		}
	}
}

// HandleStart handles the /start command
// Checks for deep-link parameter and either processes group join or displays help
func (h *BotHandler) HandleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check if there's a start parameter (deep-link)
	if update.Message != nil && update.Message.Text != "" {
		parts := strings.Fields(update.Message.Text)
		if len(parts) > 1 {
			// There's a parameter - process as deep-link
			startParam := parts[1]
			h.handleDeepLinkJoin(ctx, b, update, startParam)
			return
		}
	}

	// No parameter - display help message
	h.displayHelp(ctx, b, update)
}

// HandleHelp handles the /help command
func (h *BotHandler) HandleHelp(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.displayHelp(ctx, b, update)
}

// displayHelp displays the help message with role-based command visibility
func (h *BotHandler) displayHelp(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	isAdmin := h.isAdmin(userID)

	var helpText strings.Builder
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpBotTitle) + "\n\n")

	// User commands section
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpUserCommands) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandHelp) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandRating) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandMy) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandEvents) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandGroups) + "\n\n")

	// Admin commands section (only for admins)
	if isAdmin {
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpAdminCommands) + "\n")
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandCreateGroup) + "\n")
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandListGroups) + "\n")
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandGroupMembers) + "\n")
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandRemoveMember) + "\n")
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandCreateEvent) + "\n")
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandResolveEvent) + "\n")
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpCommandEditEvent) + "\n\n")
		helpText.WriteString(h.localizer.MustLocalize(locale.HelpListGroupsHint) + "\n\n")
	}

	// Rules and scoring information
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringRules) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringCorrect) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringBinary) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringMultiOption) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringProbability) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringBonuses) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringMinority) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringEarlyVote) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringParticipation) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringPenalties) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpScoringWrongPrediction) + "\n\n")

	// Achievements
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpAchievements) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpAchievementSharpshooter) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpAchievementProphet) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpAchievementRiskTaker) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpAchievementWeeklyAnalyst) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpAchievementVeteran) + "\n\n")

	// Event types
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpEventTypes) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpEventTypeBinary) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpEventTypeMultiOption) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpEventTypeProbability) + "\n\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpEventVoteReminder) + "\n")
	helpText.WriteString(h.localizer.MustLocalize(locale.HelpEventDeadlineReminder))

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   helpText.String(),
	})
	if err != nil {
		h.logger.Error("failed to send help message", "error", err)
	}
}

// handleDeepLinkJoin processes group join flow from deep-link
func (h *BotHandler) handleDeepLinkJoin(ctx context.Context, b *bot.Bot, update *models.Update, startParam string) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Parse group ID from start parameter
	groupID, err := h.deepLinkService.ParseGroupIDFromStart(startParam)
	if err != nil {
		h.logger.Warn("invalid deep-link parameter", "user_id", userID, "param", startParam, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.DeepLinkInvalidLink),
		})
		return
	}

	// Validate group exists
	group, err := h.groupRepo.GetGroup(ctx, groupID)
	if err != nil {
		h.logger.Error("failed to get group", "group_id", groupID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.DeepLinkErrorCheck),
		})
		return
	}

	if group == nil {
		h.logger.Warn("group not found", "group_id", groupID, "user_id", userID)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.DeepLinkGroupNotFound),
		})
		return
	}

	// Check if user already has membership
	existingMembership, err := h.groupMembershipRepo.GetMembership(ctx, groupID, userID)
	if err != nil {
		h.logger.Error("failed to check membership", "group_id", groupID, "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.DeepLinkErrorMembership),
		})
		return
	}

	// If membership exists and is active, inform user
	if existingMembership != nil && existingMembership.Status == domain.MembershipStatusActive {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.DeepLinkAlreadyMember, group.Name),
		})
		return
	}

	// If membership exists but was removed, reactivate it
	if existingMembership != nil && existingMembership.Status == domain.MembershipStatusRemoved {
		err = h.groupMembershipRepo.UpdateMembershipStatus(ctx, groupID, userID, domain.MembershipStatusActive)
		if err != nil {
			h.logger.Error("failed to reactivate membership", "group_id", groupID, "user_id", userID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.DeepLinkErrorReactivate),
			})
			return
		}

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.DeepLinkWelcomeBack, group.Name),
		})
		h.logger.Info("membership reactivated", "group_id", groupID, "user_id", userID)
		return
	}

	// Create new membership
	membership := &domain.GroupMembership{
		GroupID:  groupID,
		UserID:   userID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}

	if err := membership.Validate(); err != nil {
		h.logger.Error("membership validation failed", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.DeepLinkErrorValidation),
		})
		return
	}

	if err := h.groupMembershipRepo.CreateMembership(ctx, membership); err != nil {
		h.logger.Error("failed to create membership", "group_id", groupID, "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.DeepLinkErrorCreate),
		})
		return
	}

	// Initialize rating record for this group
	username := update.Message.From.Username
	if username == "" {
		if update.Message.From.FirstName != "" {
			username = update.Message.From.FirstName
		}
		if update.Message.From.LastName != "" {
			if username != "" {
				username += " " + update.Message.From.LastName
			} else {
				username = update.Message.From.LastName
			}
		}
	}

	rating := &domain.Rating{
		UserID:       userID,
		GroupID:      groupID,
		Username:     username,
		Score:        0,
		CorrectCount: 0,
		WrongCount:   0,
		Streak:       0,
	}

	if err := h.ratingRepo.UpdateRating(ctx, rating); err != nil {
		h.logger.Error("failed to initialize rating", "group_id", groupID, "user_id", userID, "error", err)
		// Don't fail the join - rating can be created later
	}

	// Send welcome message
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   h.localizer.MustLocalizeWithTemplate(locale.DeepLinkWelcome, group.Name),
	})
	if err != nil {
		h.logger.Error("failed to send welcome message", "error", err)
	}

	h.logger.Info("user joined group", "group_id", groupID, "user_id", userID, "group_name", group.Name)
}

// HandleRating handles the /rating command
func (h *BotHandler) HandleRating(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Determine user's current group context
	groupID, err := h.groupContextResolver.ResolveGroupForUser(ctx, userID)
	if err != nil {
		if err == domain.ErrNoGroupMembership {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.GroupContextNoMembership),
			})
			return
		}
		if err == domain.ErrMultipleGroupsNeedChoice {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.GroupContextMultipleGroups),
			})
			return
		}
		h.logger.Error("failed to resolve group context", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ErrorGeneric),
		})
		return
	}

	// Get group information
	group, err := h.groupRepo.GetGroup(ctx, groupID)
	if err != nil || group == nil {
		h.logger.Error("failed to get group", "group_id", groupID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ErrorGeneric),
		})
		return
	}

	// Get top 10 ratings for this group
	ratings, err := h.ratingCalculator.GetTopRatings(ctx, groupID, 10)
	if err != nil {
		h.logger.Error("failed to get top ratings", "group_id", groupID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ErrorGeneric),
		})
		return
	}

	if len(ratings) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.RatingEmpty),
		})
		return
	}

	// Build rating message
	var sb strings.Builder
	sb.WriteString(h.localizer.MustLocalize(locale.RatingTop10Title) + "\n")
	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.RatingGroupName, group.Name) + "\n\n")

	medals := []string{"🥇", "🥈", "🥉"}
	for i, rating := range ratings {
		medal := ""
		if i < 3 {
			medal = medals[i] + " "
		} else {
			medal = fmt.Sprintf("%d. ", i+1)
		}

		total := rating.CorrectCount + rating.WrongCount
		accuracy := 0.0
		if total > 0 {
			accuracy = float64(rating.CorrectCount) / float64(total) * 100
		}

		// Display username or user ID if username is not available
		displayName := rating.Username
		if displayName == "" {
			displayName = fmt.Sprintf("ID: %d", rating.UserID)
		} else {
			displayName = fmt.Sprintf("@%s", displayName)
		}

		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.RatingUserPoints, medal, displayName, fmt.Sprintf("%d", rating.Score)) + "\n")
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.RatingUserAccuracy, fmt.Sprintf("%.1f", accuracy)) + "\n")
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.RatingUserStreak, fmt.Sprintf("%d", rating.Streak)) + "\n")
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.RatingUserCorrect, fmt.Sprintf("%d", rating.CorrectCount)) + "\n")
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.RatingUserWrong, fmt.Sprintf("%d", rating.WrongCount)) + "\n\n")
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
	})
	if err != nil {
		h.logger.Error("failed to send rating message", "error", err)
	}
}

// HandleMy handles the /my command
func (h *BotHandler) HandleMy(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Determine user's current group context
	groupID, err := h.groupContextResolver.ResolveGroupForUser(ctx, userID)
	if err != nil {
		if err == domain.ErrNoGroupMembership {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.GroupContextNoMembership),
			})
			return
		}
		if err == domain.ErrMultipleGroupsNeedChoice {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.GroupContextMultipleGroups),
			})
			return
		}
		h.logger.Error("failed to resolve group context", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ErrorGeneric),
		})
		return
	}

	// Get group information
	group, err := h.groupRepo.GetGroup(ctx, groupID)
	if err != nil || group == nil {
		h.logger.Error("failed to get group", "group_id", groupID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ErrorGeneric),
		})
		return
	}

	// Get user rating for this group
	rating, err := h.ratingCalculator.GetUserRating(ctx, userID, groupID)
	if err != nil {
		h.logger.Error("failed to get user rating", "user_id", userID, "group_id", groupID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ErrorGeneric),
		})
		return
	}

	// Get user achievements for this group
	achievements, err := h.achievementTracker.GetUserAchievements(ctx, userID, groupID)
	if err != nil {
		h.logger.Error("failed to get user achievements", "user_id", userID, "group_id", groupID, "error", err)
		achievements = []*domain.Achievement{} // Continue with empty achievements
	}

	// Build stats message
	var sb strings.Builder
	sb.WriteString(h.localizer.MustLocalize(locale.MyStatsTitle2) + "\n")
	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.MyStatsGroupName, group.Name) + "\n\n")

	total := rating.CorrectCount + rating.WrongCount
	accuracy := 0.0
	if total > 0 {
		accuracy = float64(rating.CorrectCount) / float64(total) * 100
	}

	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.MyStatsPoints2, fmt.Sprintf("%d", rating.Score)) + "\n")
	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.MyStatsCorrect2, fmt.Sprintf("%d", rating.CorrectCount)) + "\n")
	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.MyStatsWrong2, fmt.Sprintf("%d", rating.WrongCount)) + "\n")
	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.MyStatsAccuracy2, fmt.Sprintf("%.1f", accuracy)) + "\n")
	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.MyStatsCurrentStreak, fmt.Sprintf("%d", rating.Streak)) + "\n")
	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.MyStatsTotalPreds, fmt.Sprintf("%d", total)) + "\n\n")

	// Add achievements
	if len(achievements) > 0 {
		sb.WriteString(h.localizer.MustLocalize(locale.MyStatsAchievements) + "\n")
		achievementNames := map[domain.AchievementCode]string{
			domain.AchievementSharpshooter:  h.localizer.MustLocalize(locale.AchievementSharpshooterName),
			domain.AchievementProphet:       h.localizer.MustLocalize(locale.AchievementProphetName),
			domain.AchievementRiskTaker:     h.localizer.MustLocalize(locale.AchievementRiskTakerName),
			domain.AchievementWeeklyAnalyst: h.localizer.MustLocalize(locale.AchievementWeeklyAnalystName),
			domain.AchievementVeteran:       h.localizer.MustLocalize(locale.AchievementVeteranName),
		}
		for _, ach := range achievements {
			name := achievementNames[ach.Code]
			if name == "" {
				name = string(ach.Code)
			}
			sb.WriteString(fmt.Sprintf("  • %s\n", name))
		}
	} else {
		sb.WriteString(h.localizer.MustLocalize(locale.MyStatsAchievements) + "\n")
		sb.WriteString(h.localizer.MustLocalize(locale.MyStatsNoAchievements2))
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
	})
	if err != nil {
		h.logger.Error("failed to send my stats message", "error", err)
	}
}

// HandleEvents handles the /events command
func (h *BotHandler) HandleEvents(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Get all groups where user has membership
	groups, err := h.groupRepo.GetUserGroups(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get user groups", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ErrorGeneric),
		})
		return
	}

	if len(groups) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.GroupContextNoMembership),
		})
		return
	}

	// Collect all active events from all user's groups
	var allEvents []*domain.Event
	groupNames := make(map[int64]string)
	for _, group := range groups {
		groupNames[group.ID] = group.Name
		events, err := h.eventManager.GetActiveEvents(ctx, group.ID)
		if err != nil {
			h.logger.Error("failed to get active events for group", "group_id", group.ID, "error", err)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	if len(allEvents) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.EventsNoActive),
		})
		return
	}

	// Build events list message
	var sb strings.Builder
	sb.WriteString(h.localizer.MustLocalize(locale.EventsActiveTitle) + "\n\n")

	for i, event := range allEvents {
		// Include group name for context
		groupName := groupNames[event.GroupID]
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.EventsItemNumber, fmt.Sprintf("%d", i+1), event.Question) + "\n")
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.EventsItemGroup, groupName) + "\n\n")

		// Event type
		typeStr := ""
		typeIcon := ""
		switch event.EventType {
		case domain.EventTypeBinary:
			typeStr = h.localizer.MustLocalize(locale.EventTypeBinaryLabel)
			typeIcon = h.localizer.MustLocalize(locale.EventTypeBinaryIcon)
		case domain.EventTypeMultiOption:
			typeStr = h.localizer.MustLocalize(locale.EventTypeMultiOptionLabel)
			typeIcon = h.localizer.MustLocalize(locale.EventTypeMultiOptionIcon)
		case domain.EventTypeProbability:
			typeStr = h.localizer.MustLocalize(locale.EventTypeProbabilityLabel)
			typeIcon = h.localizer.MustLocalize(locale.EventTypeProbabilityIcon)
		}
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.EventsItemType, typeIcon, typeStr) + "\n")

		// Get vote distribution for this event
		predictions, err := h.predictionRepo.GetPredictionsByEvent(ctx, event.ID)
		if err != nil {
			h.logger.Error("failed to get predictions for event", "event_id", event.ID, "error", err)
			predictions = []*domain.Prediction{} // Continue with empty predictions
		}

		// Calculate vote distribution
		voteDistribution := h.calculateVoteDistribution(predictions, len(event.Options))
		totalVotes := len(predictions)

		// Options with vote percentages
		sb.WriteString("\n" + h.localizer.MustLocalize(locale.EventsItemOptions) + "\n")
		for j, opt := range event.Options {
			percentage := voteDistribution[j]
			// Create a simple progress bar
			barLength := int(percentage / 10)
			if barLength > 10 {
				barLength = 10
			}
			bar := strings.Repeat("▰", barLength) + strings.Repeat("▱", 10-barLength)
			sb.WriteString(fmt.Sprintf("  %d) %s\n     %s %.1f%%\n", j+1, opt, bar, percentage))
		}
		sb.WriteString("\n" + h.localizer.MustLocalizeWithTemplate(locale.EventsItemVotes, fmt.Sprintf("%d", totalVotes)) + "\n")

		// Deadline
		timeUntil := time.Until(event.Deadline)
		deadlineStr := ""
		if timeUntil > 0 {
			hours := int(timeUntil.Hours())
			minutes := int(timeUntil.Minutes()) % 60
			if hours > 24 {
				days := hours / 24
				deadlineStr = h.localizer.MustLocalizeWithTemplate(locale.EventsItemTimeRemainingDays, fmt.Sprintf("%d", days), fmt.Sprintf("%d", hours%24))
			} else if hours > 0 {
				deadlineStr = h.localizer.MustLocalizeWithTemplate(locale.EventsItemTimeRemainingHours, fmt.Sprintf("%d", hours), fmt.Sprintf("%d", minutes))
			} else {
				deadlineStr = h.localizer.MustLocalizeWithTemplate(locale.EventsItemTimeRemainingMinutes, fmt.Sprintf("%d", minutes))
			}
			// Show deadline in local timezone
			localDeadline := event.Deadline.In(h.config.Timezone)
			deadlineStr += h.localizer.MustLocalizeWithTemplate(locale.EventsItemDeadlineFormat, localDeadline.Format("02.01 15:04"))
		} else {
			deadlineStr = h.localizer.MustLocalize(locale.EventsItemDeadlineExpired)
		}
		sb.WriteString(deadlineStr + "\n\n")
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
	})
	if err != nil {
		h.logger.Error("failed to send events message", "error", err)
	}
}

// calculateVoteDistribution calculates the percentage of votes for each option
// Returns a map of option index to percentage
func (h *BotHandler) calculateVoteDistribution(predictions []*domain.Prediction, numOptions int) map[int]float64 {
	distribution := make(map[int]float64)

	// Initialize all options to 0%
	for i := 0; i < numOptions; i++ {
		distribution[i] = 0.0
	}

	// If no votes, return all zeros
	if len(predictions) == 0 {
		return distribution
	}

	// Count votes for each option
	voteCounts := make(map[int]int)
	for _, pred := range predictions {
		voteCounts[pred.Option]++
	}

	// Calculate percentages
	totalVotes := float64(len(predictions))
	for option, count := range voteCounts {
		distribution[option] = (float64(count) / totalVotes) * 100.0
	}

	return distribution
}

// HandlePollAnswer handles poll answer updates (when users vote)
func (h *BotHandler) HandlePollAnswer(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.PollAnswer == nil {
		return
	}

	pollAnswer := update.PollAnswer
	userID := pollAnswer.User.ID
	pollID := pollAnswer.PollID

	// Get event by poll ID
	// event, err := h.eventManager.GetEvent(ctx, 0) // We need to find by poll ID
	// if err != nil {
	// 	h.logger.Error("failed to get event", "poll_id", pollID, "error", err)
	// 	return
	// }

	// Find event by poll ID - we need to search through user's groups
	groups, err := h.groupRepo.GetUserGroups(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get user groups", "user_id", userID, "error", err)
		return
	}

	var matchedEvent *domain.Event
	for _, group := range groups {
		events, err := h.eventManager.GetActiveEvents(ctx, group.ID)
		if err != nil {
			h.logger.Error("failed to get active events for group", "group_id", group.ID, "error", err)
			continue
		}
		for _, e := range events {
			if e.PollID == pollID {
				matchedEvent = e
				break
			}
		}
		if matchedEvent != nil {
			break
		}
	}

	if matchedEvent == nil {
		h.logger.Warn("poll answer for unknown or inaccessible event", "poll_id", pollID, "user_id", userID)
		return
	}

	event := matchedEvent

	// Verify user has active membership in the event's group
	hasActiveMembership, err := h.groupMembershipRepo.HasActiveMembership(ctx, event.GroupID, userID)
	if err != nil {
		h.logger.Error("failed to check group membership", "user_id", userID, "group_id", event.GroupID, "error", err)
		return
	}

	if !hasActiveMembership {
		h.logger.Warn("vote rejected: user not member of group", "user_id", userID, "event_id", event.ID, "group_id", event.GroupID)
		// Note: Telegram doesn't allow us to reject the vote in the UI, but we won't save it
		return
	}

	// Check if deadline has passed
	if time.Now().After(event.Deadline) {
		h.logger.Warn("vote after deadline", "user_id", userID, "event_id", event.ID)
		// Note: Telegram doesn't allow us to reject the vote, but we won't save it
		return
	}

	// Get the selected option (poll answers can have multiple options, but we use single-answer polls)
	if len(pollAnswer.OptionIDs) == 0 {
		h.logger.Warn("poll answer with no options", "user_id", userID, "event_id", event.ID)
		return
	}

	selectedOption := pollAnswer.OptionIDs[0]

	// Check if prediction already exists
	existingPrediction, err := h.predictionRepo.GetPredictionByUserAndEvent(ctx, userID, event.ID)
	if err != nil {
		h.logger.Error("failed to check existing prediction", "user_id", userID, "event_id", event.ID, "error", err)
		return
	}

	if existingPrediction != nil {
		// Update existing prediction
		existingPrediction.Option = selectedOption
		existingPrediction.Timestamp = time.Now()

		if err := h.predictionRepo.UpdatePrediction(ctx, existingPrediction); err != nil {
			h.logger.Error("failed to update prediction", "user_id", userID, "event_id", event.ID, "error", err)
			return
		}

		h.logger.Info("prediction updated", "user_id", userID, "event_id", event.ID, "group_id", event.GroupID, "option", selectedOption)
	} else {
		// Create new prediction
		prediction := &domain.Prediction{
			EventID:   event.ID,
			UserID:    userID,
			Option:    selectedOption,
			Timestamp: time.Now(),
		}

		if err := h.predictionRepo.SavePrediction(ctx, prediction); err != nil {
			h.logger.Error("failed to save prediction", "user_id", userID, "event_id", event.ID, "error", err)
			return
		}

		h.logger.Info("prediction saved", "user_id", userID, "event_id", event.ID, "group_id", event.GroupID, "option", selectedOption)
	}

	// Update or create user rating with username
	username := pollAnswer.User.Username
	if username == "" {
		// If username is not set, use first name or last name
		if pollAnswer.User.FirstName != "" {
			username = pollAnswer.User.FirstName
		}
		if pollAnswer.User.LastName != "" {
			if username != "" {
				username += " " + pollAnswer.User.LastName
			} else {
				username = pollAnswer.User.LastName
			}
		}
	}

	// Get or create rating to ensure username is saved
	rating, err := h.ratingCalculator.GetUserRating(ctx, userID, event.GroupID)
	if err != nil {
		h.logger.Error("failed to get user rating", "user_id", userID, "group_id", event.GroupID, "error", err)
		return
	}

	// Update username if it's different or empty
	if rating.Username != username && username != "" {
		rating.Username = username
		if err := h.ratingCalculator.UpdateRatingUsername(ctx, rating); err != nil {
			h.logger.Error("failed to update username", "user_id", userID, "group_id", event.GroupID, "error", err)
		}
	}
}

// checkConflictingSession checks if user has an active session of a different type
// Returns the conflicting session type name or empty string if no conflict
func (h *BotHandler) checkConflictingSession(ctx context.Context, userID int64, requestedType string) (string, error) {
	state, _, err := h.eventCreationFSM.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound || err == storage.ErrSessionExpired {
			return "", nil
		}
		return "", err
	}

	// Check which FSM owns this state
	switch state {
	case StateSelectGroup, StateAskQuestion, StateAskEventType, StateAskOptions, StateAskDeadline, StateConfirm, StateComplete:
		if requestedType != "event_creation" {
			return h.localizer.MustLocalize(locale.SessionTypeEventCreation), nil
		}
	case StateGroupAskName, StateGroupAskChatID, StateGroupComplete:
		if requestedType != "group_creation" {
			return h.localizer.MustLocalize(locale.SessionTypeGroupCreation), nil
		}
	case StateResolveSelectEvent, StateResolveSelectOption, StateResolveComplete:
		if requestedType != "event_resolution" {
			return h.localizer.MustLocalize(locale.SessionTypeEventResolution), nil
		}
	}

	return "", nil
}

// HandleCreateEvent handles the /create_event command (multi-step conversation)
func (h *BotHandler) HandleCreateEvent(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Check for conflicting sessions
	conflictType, err := h.checkConflictingSession(ctx, userID, "event_creation")
	if err != nil {
		h.logger.Error("failed to check conflicting session", "user_id", userID, "error", err)
	} else if conflictType != "" {
		// User has an active session of a different type
		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: h.localizer.MustLocalize(locale.SessionConflictContinueButton), CallbackData: "session_conflict:continue"},
				},
				{
					{Text: h.localizer.MustLocalize(locale.SessionConflictRestartButton), CallbackData: "session_conflict:restart:event_creation"},
				},
			},
		}

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        h.localizer.MustLocalizeWithTemplate(locale.SessionConflictWarning, conflictType),
			ReplyMarkup: kb,
		})
		return
	}

	// Admins are exempt from participation requirement
	if !h.isAdmin(userID) {
		// Get user's groups to check participation in each
		groups, err := h.groupRepo.GetUserGroups(ctx, userID)
		if err != nil {
			h.logger.Error("failed to get user groups", "user_id", userID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.EventResolutionErrorPermissionCheckRetry),
			})
			return
		}

		if len(groups) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.EventCreationErrorNoGroups),
			})
			return
		}

		// Check if user has sufficient participation in at least one group
		hasPermissionInAnyGroup := false
		maxParticipation := 0
		for _, group := range groups {
			canCreate, participationCount, err := h.eventPermissionValidator.CanCreateEvent(ctx, userID, group.ID, h.config.AdminUserIDs)
			if err != nil {
				h.logger.Error("failed to check event creation permission", "user_id", userID, "group_id", group.ID, "error", err)
				continue
			}
			if participationCount > maxParticipation {
				maxParticipation = participationCount
			}
			if canCreate {
				hasPermissionInAnyGroup = true
				break
			}
		}

		if !hasPermissionInAnyGroup {
			// User doesn't have enough participation in any group
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalizeWithTemplate(locale.EventCreationPermissionDenied, fmt.Sprintf("%d", h.config.MinEventsToCreate), fmt.Sprintf("%d", maxParticipation)),
			})
			h.logger.Info("event creation denied due to insufficient participation", "user_id", userID, "max_participation", maxParticipation, "required", h.config.MinEventsToCreate)
			return
		}
	}

	// Start FSM session for user
	if err := h.eventCreationFSM.Start(ctx, userID, chatID); err != nil {
		h.logger.Error("failed to start FSM session", "user_id", userID, "error", err)

		// Provide user-friendly error message based on error type
		var errorMsg string
		if err == domain.ErrNoGroupMembership {
			errorMsg = h.localizer.MustLocalize(locale.EventCreationErrorNoGroupsHelp)
		} else {
			errorMsg = h.localizer.MustLocalize(locale.EventCreationErrorStart)
		}

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   errorMsg,
		})
		return
	}

	h.logger.Info("event creation started via FSM", "user_id", userID, "chat_id", chatID)
}

// HandleMessage handles regular text messages (for conversation flows)
func (h *BotHandler) HandleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	// Log message_thread_id if this is a forum topic message
	if update.Message.MessageThreadID != 0 {
		textPreview := update.Message.Text
		if len(textPreview) > 50 {
			textPreview = textPreview[:50]
		}
		h.logger.Info("message received in forum topic",
			"chat_id", update.Message.Chat.ID,
			"chat_title", update.Message.Chat.Title,
			"message_thread_id", update.Message.MessageThreadID,
			"is_forum", update.Message.Chat.IsForum,
			"text_preview", textPreview,
		)
	}

	userID := update.Message.From.ID

	// Check if user has active group creation FSM session
	hasGroupSession, err := h.groupCreationFSM.HasSession(ctx, userID)
	if err != nil {
		h.logger.Error("failed to check group creation FSM session", "user_id", userID, "error", err)
	} else if hasGroupSession {
		// Route to group creation FSM
		if err := h.groupCreationFSM.HandleMessage(ctx, update); err != nil {
			h.logger.Error("group creation FSM message handling failed", "user_id", userID, "error", err)

			// Inform user to restart
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.FSMErrorRestartGroup),
			})
		}
		return
	}

	// Check if user has active event creation FSM session
	hasEventSession, err := h.eventCreationFSM.HasSession(ctx, userID)
	if err != nil {
		h.logger.Error("failed to check event creation FSM session", "user_id", userID, "error", err)
		return
	}

	if hasEventSession {
		// Route to event creation FSM
		if err := h.eventCreationFSM.HandleMessage(ctx, update); err != nil {
			h.logger.Error("event creation FSM message handling failed", "user_id", userID, "error", err)

			// Inform user to restart
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.FSMErrorRestartEvent),
			})
		}
		return
	}

	// Check if user has active rename FSM session
	hasRenameSession, err := h.renameFSM.HasSession(ctx, userID)
	if err != nil {
		h.logger.Error("failed to check rename FSM session", "user_id", userID, "error", err)
	} else if hasRenameSession {
		// Route to rename FSM
		if err := h.renameFSM.HandleMessage(ctx, update); err != nil {
			h.logger.Error("rename FSM message handling failed", "user_id", userID, "error", err)

			// Inform user to restart
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.FSMErrorRestartRename),
			})
		}
		return
	}

	// Check if user has active event edit FSM session
	hasEditSession, err := h.eventEditFSM.HasSession(ctx, userID)
	if err != nil {
		h.logger.Error("failed to check event edit FSM session", "user_id", userID, "error", err)
	} else if hasEditSession {
		// Route to event edit FSM
		if err := h.eventEditFSM.HandleMessage(ctx, update); err != nil {
			h.logger.Error("event edit FSM message handling failed", "user_id", userID, "error", err)

			// Inform user to restart
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.FSMErrorRestartEdit),
			})
		}
		return
	}

	// No active conversation - ignore message
}

// HandleCallback handles callback queries (button clicks)
func (h *BotHandler) HandleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	userID := callback.From.ID
	data := callback.Data

	// Handle session conflict resolution callbacks
	if strings.HasPrefix(data, "session_conflict:") {
		h.handleSessionConflictCallback(ctx, b, callback)
		return
	}

	// Check if this is an event creation FSM callback (group selection, event_type selection, deadline preset or confirmation)
	if strings.HasPrefix(data, "select_group:") || strings.HasPrefix(data, "event_type:") || strings.HasPrefix(data, "deadline_preset:") || strings.HasPrefix(data, "confirm:") {
		// Check if user has active FSM session
		hasSession, err := h.eventCreationFSM.HasSession(ctx, userID)
		if err != nil {
			h.logger.Error("failed to check FSM session for callback", "user_id", userID, "error", err)
			// Answer callback query to remove loading state
			_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: callback.ID,
			})
			return
		}

		if hasSession {
			// Route to FSM
			if err := h.eventCreationFSM.HandleCallback(ctx, callback); err != nil {
				h.logger.Error("FSM callback handling failed", "user_id", userID, "error", err)
			}
			return
		}
	}

	// Check if this is a group creation FSM callback
	if strings.HasPrefix(data, "group_is_forum:") {
		// Check if user has active group creation FSM session
		hasSession, err := h.groupCreationFSM.HasSession(ctx, userID)
		if err != nil {
			h.logger.Error("failed to check group creation FSM session for callback", "user_id", userID, "error", err)
			// Answer callback query to remove loading state
			_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: callback.ID,
			})
			return
		}

		if hasSession {
			// Route to group creation FSM
			if err := h.groupCreationFSM.HandleCallback(ctx, callback); err != nil {
				h.logger.Error("group creation FSM callback handling failed", "user_id", userID, "error", err)
			}
			return
		}
	}

	// Check if this is an event resolution FSM callback
	if strings.HasPrefix(data, "resolve:") {
		// Check if user has active resolution FSM session
		hasSession, err := h.eventResolutionFSM.HasSession(ctx, userID)
		if err != nil {
			h.logger.Error("failed to check resolution FSM session for callback", "user_id", userID, "error", err)
			// Answer callback query to remove loading state
			_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: callback.ID,
			})
			return
		}

		if hasSession {
			// Route to resolution FSM
			if err := h.eventResolutionFSM.HandleCallback(ctx, callback); err != nil {
				h.logger.Error("resolution FSM callback handling failed", "user_id", userID, "error", err)
			}
			return
		}

		// No active session - start a new resolution session for this event
		h.handleResolveEventFromCallback(ctx, b, callback)
		return
	}

	// Handle edit_event callbacks
	if strings.HasPrefix(data, "edit_event:") {
		h.handleEditEventCallback(ctx, b, callback)
		return
	}

	// Handle edit_field callbacks (from event edit FSM)
	if strings.HasPrefix(data, "edit_field:") || strings.HasPrefix(data, "edit_deadline_preset:") {
		if err := h.eventEditFSM.HandleCallback(ctx, callback); err != nil {
			h.logger.Error("event edit FSM callback handling failed", "user_id", userID, "error", err)
		}
		return
	}

	// Handle leave_group callbacks
	if strings.HasPrefix(data, "leave_group:") {
		h.handleLeaveGroupCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle group_members callbacks
	if strings.HasPrefix(data, "group_members:") {
		h.handleGroupMembersCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle remove_member callbacks
	if strings.HasPrefix(data, "remove_member_group:") || strings.HasPrefix(data, "remove_member_user:") {
		h.handleRemoveMemberCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle delete_group callbacks
	if strings.HasPrefix(data, "delete_group_") {
		h.handleDeleteGroupCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle delete_topic callbacks
	if strings.HasPrefix(data, "delete_topic_") {
		h.handleDeleteTopicCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle soft_delete_group callbacks
	if strings.HasPrefix(data, "soft_delete_group_") {
		h.handleSoftDeleteGroupCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle restore_group callbacks
	if strings.HasPrefix(data, "restore_group_") {
		h.handleRestoreGroupCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle rename_group callbacks
	if strings.HasPrefix(data, "rename_group_") {
		h.handleRenameGroupCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle rename_topic callbacks
	if strings.HasPrefix(data, "rename_topic_") {
		h.handleRenameTopicCallback(ctx, b, callback, userID, data)
		return
	}

	// Answer callback query to remove loading state (for non-FSM callbacks)
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})
}

// HandleResolveEvent handles the /resolve_event command
func (h *BotHandler) HandleResolveEvent(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Check for conflicting sessions
	conflictType, err := h.checkConflictingSession(ctx, userID, "event_resolution")
	if err != nil {
		h.logger.Error("failed to check conflicting session", "user_id", userID, "error", err)
	} else if conflictType != "" {
		// User has an active session of a different type
		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: h.localizer.MustLocalize(locale.SessionConflictContinueButton), CallbackData: "session_conflict:continue"},
				},
				{
					{Text: h.localizer.MustLocalize(locale.SessionConflictRestartButton), CallbackData: "session_conflict:restart:event_resolution"},
				},
			},
		}

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        h.localizer.MustLocalizeWithTemplate(locale.SessionConflictWarning, conflictType),
			ReplyMarkup: kb,
		})
		return
	}

	// Start FSM session for user
	if err := h.eventResolutionFSM.Start(ctx, userID, chatID); err != nil {
		h.logger.Error("failed to start resolution FSM session", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.EventResolutionErrorStart),
		})
		return
	}

	// Get all groups where user has access (admin sees all, others see their groups)
	var groups []*domain.Group
	if h.isAdmin(userID) {
		groups, err = h.groupRepo.GetAllGroups(ctx)
	} else {
		groups, err = h.groupRepo.GetUserGroups(ctx, userID)
	}
	if err != nil {
		h.logger.Error("failed to get groups", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.EventResolutionErrorGroups),
		})
		return
	}

	// Get all active events from accessible groups
	var allEvents []*domain.Event
	for _, group := range groups {
		events, err := h.eventManager.GetActiveEvents(ctx, group.ID)
		if err != nil {
			h.logger.Error("failed to get active events for group", "group_id", group.ID, "error", err)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	if len(allEvents) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.EventResolutionNoEvents),
		})
		return
	}

	// Filter events that user can manage
	var manageableEvents []*domain.Event
	for _, event := range allEvents {
		canManage, err := h.eventPermissionValidator.CanManageEvent(ctx, userID, event.ID, h.config.AdminUserIDs)
		if err != nil {
			h.logger.Error("failed to check event management permission", "user_id", userID, "event_id", event.ID, "error", err)
			continue
		}
		if canManage {
			manageableEvents = append(manageableEvents, event)
		}
	}

	if len(manageableEvents) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.EventResolutionNoPermission),
		})
		return
	}

	// Build inline keyboard with manageable events
	var buttons [][]models.InlineKeyboardButton
	for _, event := range manageableEvents {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         fmt.Sprintf("%s (ID: %d)", event.Question, event.ID),
				CallbackData: fmt.Sprintf("resolve:%d", event.ID),
			},
		})
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        h.localizer.MustLocalize(locale.EventResolutionTitle2) + "\n\n" + h.localizer.MustLocalize(locale.EventResolutionSelectPrompt),
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send resolve event selection", "error", err)
		return
	}

	// Store message ID in FSM context
	if msg != nil {
		// Get current context and add message ID
		state, contextData, err := h.eventResolutionFSM.storage.Get(ctx, userID)
		if err == nil {
			resolutionContext := &domain.EventResolutionContext{}
			if err := resolutionContext.FromMap(contextData); err == nil {
				resolutionContext.MessageIDs = append(resolutionContext.MessageIDs, msg.ID)
				_ = h.eventResolutionFSM.storage.Set(ctx, userID, state, resolutionContext.ToMap())
			}
		}
	}

	h.logger.Info("event resolution started via FSM", "user_id", userID, "chat_id", chatID)
}

// HandleEditEvent handles the /edit_event command
// Note: Edit functionality has been removed in favor of FSM-based event creation.
// Events can no longer be edited after creation to maintain data integrity.
func (h *BotHandler) HandleEditEvent(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check admin authorization
	if !h.requireAdmin(ctx, update) {
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   h.localizer.MustLocalize(locale.EditEventUnavailable),
	})
}

// HandleCreateGroup handles the /create_group command
func (h *BotHandler) HandleCreateGroup(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check admin authorization
	if !h.requireAdmin(ctx, update) {
		return
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Check for conflicting sessions
	conflictType, err := h.checkConflictingSession(ctx, userID, "group_creation")
	if err != nil {
		h.logger.Error("failed to check conflicting session", "user_id", userID, "error", err)
	} else if conflictType != "" {
		// User has an active session of a different type
		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: h.localizer.MustLocalize(locale.SessionConflictContinueButton), CallbackData: "session_conflict:continue"},
				},
				{
					{Text: h.localizer.MustLocalize(locale.SessionConflictRestartButton), CallbackData: "session_conflict:restart:group_creation"},
				},
			},
		}

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        h.localizer.MustLocalizeWithTemplate(locale.SessionConflictWarning, conflictType),
			ReplyMarkup: kb,
		})
		return
	}

	// Check if command was sent from a forum topic
	var messageThreadID *int
	isForum := false
	if update.Message.MessageThreadID != 0 {
		threadID := update.Message.MessageThreadID
		messageThreadID = &threadID
		isForum = true
		h.logger.Info("create_group command sent from forum topic",
			"user_id", userID,
			"chat_id", update.Message.Chat.ID,
			"message_thread_id", threadID,
		)
	}

	// Start FSM session for user with forum info
	if err := h.groupCreationFSM.StartWithForumInfo(ctx, userID, chatID, messageThreadID, isForum); err != nil {
		h.logger.Error("failed to start group creation FSM session", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.CreateGroupErrorStart),
		})
		return
	}

	// Build prompt message
	promptText := h.localizer.MustLocalize(locale.GroupCreationTitle) + "\n\n"
	if isForum && messageThreadID != nil {
		promptText += h.localizer.MustLocalizeWithTemplate(locale.GroupCreationForumDetectedFull, fmt.Sprintf("%d", *messageThreadID))
	}
	promptText += h.localizer.MustLocalize(locale.GroupCreationPromptName)

	// Prompt for group name
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   promptText,
	})
	if err != nil {
		h.logger.Error("failed to send create group prompt", "error", err)
		return
	}

	// Store message ID in FSM context
	if msg != nil {
		// Get current context and add message ID
		state, contextData, err := h.groupCreationFSM.storage.Get(ctx, userID)
		if err == nil {
			groupContext := &domain.GroupCreationContext{}
			if err := groupContext.FromMap(contextData); err == nil {
				groupContext.MessageIDs = append(groupContext.MessageIDs, msg.ID)
				_ = h.groupCreationFSM.storage.Set(ctx, userID, state, groupContext.ToMap())
			}
		}
	}

	h.logger.Info("group creation started via FSM", "user_id", userID, "chat_id", chatID)
}

// HandleListGroups handles the /list_groups command
func (h *BotHandler) HandleListGroups(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check admin authorization
	if !h.requireAdmin(ctx, update) {
		return
	}

	// Retrieve all groups
	groups, err := h.groupRepo.GetAllGroups(ctx)
	if err != nil {
		h.logger.Error("failed to get all groups", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
		})
		return
	}

	if len(groups) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.ListGroupsEmpty),
		})
		return
	}

	// Build groups list message with deep-links and topics
	var sb strings.Builder
	sb.WriteString(h.localizer.MustLocalize(locale.ListGroupsTitle) + "\n\n")

	for i, group := range groups {
		// Get member count
		members, err := h.groupMembershipRepo.GetGroupMembers(ctx, group.ID)
		if err != nil {
			h.logger.Error("failed to get group members", "group_id", group.ID, "error", err)
			continue
		}

		// Count active members
		activeCount := 0
		for _, member := range members {
			if member.Status == domain.MembershipStatusActive {
				activeCount++
			}
		}

		// Generate deep-link
		deepLink, err := h.deepLinkService.GenerateGroupInviteLink(group.ID)
		if err != nil {
			h.logger.Error("failed to generate deep-link", "group_id", group.ID, "error", err)
			deepLink = h.localizer.MustLocalize(locale.ListGroupsLinkError)
		}

		// Add status indicator
		statusIcon := "✅"
		statusText := ""
		if group.Status == domain.GroupStatusDeleted {
			statusIcon = "🗑"
			statusText = h.localizer.MustLocalize(locale.ListGroupsItemDeleted)
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s%s\n", i+1, statusIcon, group.Name, statusText))
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.ListGroupsItemMembersFormat, fmt.Sprintf("%d", activeCount)))
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.ListGroupsItemLinkFormat, deepLink))
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.ListGroupsItemID, fmt.Sprintf("%d", group.ID)))

		// If this is a forum, show topics
		if group.IsForum {
			sb.WriteString(h.localizer.MustLocalize(locale.ListGroupsItemTypeFormat))

			// Get forum topics for this group
			topics, err := h.forumTopicRepo.GetForumTopicsByGroup(ctx, group.ID)
			if err != nil {
				h.logger.Error("failed to get forum topics", "group_id", group.ID, "error", err)
			} else if len(topics) > 0 {
				sb.WriteString(h.localizer.MustLocalize(locale.ListGroupsItemTopicsHeader))
				for _, topic := range topics {
					sb.WriteString(fmt.Sprintf("      • %s (Thread ID: %d, ID: %d)\n", topic.Name, topic.MessageThreadID, topic.ID))
				}
			} else {
				sb.WriteString(h.localizer.MustLocalize(locale.ListGroupsItemNoTopics))
			}
		}

		sb.WriteString("\n")
	}

	// Add management buttons
	var buttons [][]models.InlineKeyboardButton
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: h.localizer.MustLocalize(locale.ListGroupsButtonRenameGroup), CallbackData: "rename_group_select"},
		{Text: h.localizer.MustLocalize(locale.ListGroupsButtonRenameTopic), CallbackData: "rename_topic_select"},
	})
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: h.localizer.MustLocalize(locale.ListGroupsButtonSoftDelete), CallbackData: "soft_delete_group_select"},
		{Text: h.localizer.MustLocalize(locale.ListGroupsButtonRestore), CallbackData: "restore_group_select"},
	})
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: h.localizer.MustLocalize(locale.ListGroupsButtonDeleteTopic), CallbackData: "delete_topic_select"},
	})

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        sb.String(),
		ReplyMarkup: kb,
		ParseMode:   models.ParseModeHTML,
	})
	if err != nil {
		h.logger.Error("failed to send groups list", "error", err)
	}
}

// HandleGroupMembers handles the /group_members command
func (h *BotHandler) HandleGroupMembers(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check admin authorization
	if !h.requireAdmin(ctx, update) {
		return
	}

	// Get all groups
	groups, err := h.groupRepo.GetAllGroups(ctx)
	if err != nil {
		h.logger.Error("failed to get all groups", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
		})
		return
	}

	if len(groups) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.ListGroupsEmpty),
		})
		return
	}

	// Build inline keyboard with groups
	var buttons [][]models.InlineKeyboardButton
	for _, group := range groups {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         group.Name,
				CallbackData: fmt.Sprintf("group_members:%d", group.ID),
			},
		})
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        h.localizer.MustLocalize(locale.GroupMembersTitle) + "\n\n" + h.localizer.MustLocalize(locale.GroupMembersSelectGroup),
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send group selection", "error", err)
	}
}

// HandleRemoveMember handles the /remove_member command
func (h *BotHandler) HandleRemoveMember(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check admin authorization
	if !h.requireAdmin(ctx, update) {
		return
	}

	// Get all groups
	groups, err := h.groupRepo.GetAllGroups(ctx)
	if err != nil {
		h.logger.Error("failed to get all groups", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
		})
		return
	}

	if len(groups) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.ListGroupsEmpty),
		})
		return
	}

	// Build inline keyboard with groups
	var buttons [][]models.InlineKeyboardButton
	for _, group := range groups {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         group.Name,
				CallbackData: fmt.Sprintf("remove_member_group:%d", group.ID),
			},
		})
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        h.localizer.MustLocalize(locale.RemoveMemberTitle) + "\n\n" + h.localizer.MustLocalize(locale.RemoveMemberSelectGroup),
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send group selection for removal", "error", err)
	}
}

// handleGroupMembersCallback handles the callback for viewing group members
func (h *BotHandler) handleGroupMembersCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	// Parse group ID
	parts := strings.Split(data, ":")
	if len(parts) != 2 {
		h.logger.Error("invalid group_members callback data", "data", data)
		return
	}

	groupID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		h.logger.Error("failed to parse group ID", "error", err)
		return
	}

	// Get group
	group, err := h.groupRepo.GetGroup(ctx, groupID)
	if err != nil {
		h.logger.Error("failed to get group", "group_id", groupID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
		})
		return
	}

	if group == nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
		})
		return
	}

	// Get group members
	members, err := h.groupMembershipRepo.GetGroupMembers(ctx, groupID)
	if err != nil {
		h.logger.Error("failed to get group members", "group_id", groupID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGet),
		})
		return
	}

	if len(members) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.GroupEmptyMembers, group.Name),
		})
		return
	}

	// Build members list message
	var sb strings.Builder
	sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.GroupMembersTitleWithName, group.Name))

	for i, member := range members {
		// Get user rating for this group
		rating, err := h.ratingRepo.GetRating(ctx, member.UserID, groupID)
		if err != nil {
			h.logger.Error("failed to get user rating", "user_id", member.UserID, "group_id", groupID, "error", err)
			// Continue with default values
			rating = &domain.Rating{
				UserID:  member.UserID,
				GroupID: groupID,
				Score:   0,
			}
		}

		// Get achievements count for this group
		achievements, err := h.achievementTracker.GetUserAchievements(ctx, member.UserID, groupID)
		if err != nil {
			h.logger.Error("failed to get user achievements", "user_id", member.UserID, "group_id", groupID, "error", err)
			achievements = []*domain.Achievement{}
		}

		// Get display name
		displayName := h.getUserDisplayName(ctx, member.UserID, groupID)

		// Status indicator
		statusIcon := "✅"
		if member.Status == domain.MembershipStatusRemoved {
			statusIcon = "🚫"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, statusIcon, displayName))
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.GroupMembersItemPointsFormat, fmt.Sprintf("%d", rating.Score)))
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.GroupMembersItemAchievementsFormat, fmt.Sprintf("%d", len(achievements))))
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.GroupMembersItemJoinedFormat, member.JoinedAt.Format("02.01.2006")))
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: callback.Message.Message.Chat.ID,
		Text:   sb.String(),
	})
	if err != nil {
		h.logger.Error("failed to send members list", "error", err)
	}

	// Answer callback query
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})
}

// HandleGroups handles the /groups command for users
func (h *BotHandler) HandleGroups(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Retrieve user's active group memberships
	groups, err := h.groupRepo.GetUserGroups(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get user groups", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
		})
		return
	}

	// Handle case of no memberships
	if len(groups) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: h.localizer.MustLocalize(locale.GroupsNoGroups) + "\n\n" +
				h.localizer.MustLocalize(locale.GroupsJoinInstructions),
		})
		return
	}

	// Build groups list message
	var sb strings.Builder
	sb.WriteString(h.localizer.MustLocalize(locale.GroupsYourGroups) + "\n\n")

	// Get memberships to access join dates (groups are already ordered by join date DESC)
	for i, group := range groups {
		// Get membership to access join date
		membership, err := h.groupMembershipRepo.GetMembership(ctx, group.ID, userID)
		if err != nil {
			h.logger.Error("failed to get membership", "group_id", group.ID, "user_id", userID, "error", err)
			continue
		}

		if membership == nil {
			continue
		}

		// Get member count for this group
		members, err := h.groupMembershipRepo.GetGroupMembers(ctx, group.ID)
		if err != nil {
			h.logger.Error("failed to get group members", "group_id", group.ID, "error", err)
			continue
		}

		// Count active members
		activeCount := 0
		for _, member := range members {
			if member.Status == domain.MembershipStatusActive {
				activeCount++
			}
		}

		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, group.Name))
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.GroupsItemMembersFormat, fmt.Sprintf("%d", activeCount)))
		sb.WriteString(h.localizer.MustLocalizeWithTemplate(locale.GroupsItemJoinedFormat, membership.JoinedAt.Format("02.01.2006")))
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
	})
	if err != nil {
		h.logger.Error("failed to send groups list", "error", err)
	}
}

// HandleMyChatMember handles updates when bot is added to or removed from a chat
func (h *BotHandler) HandleMyChatMember(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.MyChatMember == nil {
		return
	}

	chatMember := update.MyChatMember
	chat := chatMember.Chat
	newStatus := chatMember.NewChatMember.Type
	oldStatus := chatMember.OldChatMember.Type
	addedBy := chatMember.From

	// Check if bot was added to a group or supergroup
	if (chat.Type == "group" || chat.Type == "supergroup") &&
		(oldStatus == models.ChatMemberTypeLeft || oldStatus == models.ChatMemberTypeBanned) &&
		(newStatus == models.ChatMemberTypeMember || newStatus == models.ChatMemberTypeAdministrator) {

		// Bot was added to a group
		h.logger.Info("bot added to telegram group",
			"chat_id", chat.ID,
			"chat_title", chat.Title,
			"added_by_user_id", addedBy.ID,
			"added_by_username", addedBy.Username,
			"is_forum", chat.IsForum,
		)

		// Get display name for the user who added the bot
		displayName := addedBy.Username
		if displayName == "" {
			if addedBy.FirstName != "" {
				displayName = addedBy.FirstName
			}
			if addedBy.LastName != "" {
				if displayName != "" {
					displayName += " " + addedBy.LastName
				} else {
					displayName = addedBy.LastName
				}
			}
		}
		if displayName == "" {
			displayName = fmt.Sprintf("ID: %d", addedBy.ID)
		} else {
			displayName = fmt.Sprintf("@%s", displayName)
		}

		// Build notification message
		notificationMsg := h.localizer.MustLocalize(locale.BotAddedTitle) + "\n\n" +
			h.localizer.MustLocalizeWithTemplate(locale.BotAddedBy, displayName) +
			h.localizer.MustLocalizeWithTemplate(locale.BotAddedGroupNameFormat, chat.Title) +
			h.localizer.MustLocalizeWithTemplate(locale.BotAddedChatIDFormat, fmt.Sprintf("%d", chat.ID))

		// Add forum information if this is a forum
		if chat.IsForum {
			notificationMsg += "\n" + h.localizer.MustLocalize(locale.BotAddedTypeForum) + "\n"

			// Try to get forum topics using GetForumTopicIconStickers
			// Note: We can't directly list topics, but we can get chat info
			chatInfo, err := h.bot.GetChat(ctx, &bot.GetChatParams{
				ChatID: chat.ID,
			})
			if err != nil {
				h.logger.Error("failed to get chat info", "chat_id", chat.ID, "error", err)
			} else if chatInfo != nil {
				h.logger.Info("forum chat info retrieved",
					"chat_id", chat.ID,
					"is_forum", chatInfo.IsForum,
					"active_usernames", chatInfo.ActiveUsernames,
				)
			}

			notificationMsg += "\n" + h.localizer.MustLocalize(locale.BotAddedForumInstructions) + "\n"
			notificationMsg += h.localizer.MustLocalize(locale.BotAddedForumInstructionsStep1)
			notificationMsg += h.localizer.MustLocalize(locale.BotAddedForumInstructionsStep2)
			notificationMsg += h.localizer.MustLocalize(locale.BotAddedForumInstructionsStep3)
			notificationMsg += h.localizer.MustLocalize(locale.BotAddedForumSetup)
			notificationMsg += h.localizer.MustLocalize(locale.BotAddedForumEvents)
		} else {
			notificationMsg += h.localizer.MustLocalize(locale.BotAddedTypeRegular) + "\n"
		}

		notificationMsg += "\n" + h.localizer.MustLocalize(locale.BotAddedRegisterCommand)

		// Create inline keyboard with "Leave Group" button
		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text:         h.localizer.MustLocalize(locale.LeaveGroupButton),
						CallbackData: fmt.Sprintf("leave_group:%d", chat.ID),
					},
				},
			},
		}

		h.notifyAdminsWithKeyboard(ctx, notificationMsg, kb)

		// Check if the user who added the bot is a member of any group
		groups, err := h.groupRepo.GetUserGroups(ctx, addedBy.ID)
		if err != nil {
			h.logger.Error("failed to check user groups", "user_id", addedBy.ID, "error", err)
			return
		}

		// If user is a member, send them a notification
		if len(groups) > 0 {
			userNotificationMsg := h.localizer.MustLocalizeWithTemplate(locale.BotAddedUserNotification, chat.Title, fmt.Sprintf("%d", chat.ID)) + "\n\n"

			if chat.IsForum {
				userNotificationMsg += h.localizer.MustLocalize(locale.BotAddedTypeForum) + "\n\n"
				userNotificationMsg += h.localizer.MustLocalize(locale.BotAddedUserForumInstructions) + "\n"
				userNotificationMsg += h.localizer.MustLocalize(locale.BotAddedForumInstructionsStep1)
				userNotificationMsg += h.localizer.MustLocalize(locale.BotAddedUserForumStep2)
				userNotificationMsg += h.localizer.MustLocalize(locale.BotAddedForumInstructionsStep3)
				userNotificationMsg += h.localizer.MustLocalize(locale.BotAddedUserForumEvents)
			} else {
				userNotificationMsg += h.localizer.MustLocalize(locale.BotAddedTypeRegular) + "\n\n"
			}

			userNotificationMsg += h.localizer.MustLocalize(locale.BotAddedUserRegisterCommand)

			_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    addedBy.ID,
				Text:      userNotificationMsg,
				ParseMode: models.ParseModeHTML,
			})
			if err != nil {
				h.logger.Error("failed to send notification to user", "user_id", addedBy.ID, "error", err)
			}
		}
	}
}

// handleLeaveGroupCallback handles the callback for leaving a telegram group
func (h *BotHandler) handleLeaveGroupCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	// Parse chat ID
	parts := strings.Split(data, ":")
	if len(parts) != 2 {
		h.logger.Error("invalid leave_group callback data", "data", data)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorInvalidDataFormat),
		})
		return
	}

	chatID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		h.logger.Error("failed to parse chat ID", "error", err)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorInvalidChatID),
		})
		return
	}

	// Try to leave the chat
	_, err = b.LeaveChat(ctx, &bot.LeaveChatParams{
		ChatID: chatID,
	})
	if err != nil {
		h.logger.Error("failed to leave chat", "chat_id", chatID, "error", err)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.LeaveGroupError),
		})

		// Edit message to show error
		if callback.Message.Message != nil {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    callback.Message.Message.Chat.ID,
				MessageID: callback.Message.Message.ID,
				Text:      callback.Message.Message.Text + "\n\n" + h.localizer.MustLocalize(locale.LeaveGroupError),
			})
		}
		return
	}

	h.logger.Info("bot left telegram group", "chat_id", chatID, "admin_user_id", userID)

	// Answer callback query
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
		Text:            h.localizer.MustLocalize(locale.LeaveGroupSuccess),
	})

	// Edit message to show success
	if callback.Message.Message != nil {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    callback.Message.Message.Chat.ID,
			MessageID: callback.Message.Message.ID,
			Text:      callback.Message.Message.Text + "\n\n" + h.localizer.MustLocalize(locale.LeaveGroupSuccess),
		})
	}
}

// handleRemoveMemberCallback handles the callback for removing a member
func (h *BotHandler) handleRemoveMemberCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	// Check if this is group selection or user selection
	if strings.HasPrefix(data, "remove_member_group:") {
		// Parse group ID
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid remove_member_group callback data", "data", data)
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse group ID", "error", err)
			return
		}

		// Get group
		group, err := h.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
			})
			return
		}

		// Get active members
		members, err := h.groupMembershipRepo.GetGroupMembers(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group members", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGet),
			})
			return
		}

		// Filter active members
		var activeMembers []*domain.GroupMembership
		for _, member := range members {
			if member.Status == domain.MembershipStatusActive {
				activeMembers = append(activeMembers, member)
			}
		}

		if len(activeMembers) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalizeWithTemplate(locale.GroupEmptyActiveMembers, group.Name),
			})
			return
		}

		// Build inline keyboard with members
		var buttons [][]models.InlineKeyboardButton
		for _, member := range activeMembers {
			displayName := h.getUserDisplayName(ctx, member.UserID, groupID)
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         displayName,
					CallbackData: fmt.Sprintf("remove_member_user:%d:%d", groupID, member.UserID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalizeWithTemplate(locale.RemoveMemberPromptWithName, group.Name),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send member selection", "error", err)
		}

		// Answer callback query
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	// This is user selection
	if strings.HasPrefix(data, "remove_member_user:") {
		// Parse group ID and user ID
		parts := strings.Split(data, ":")
		if len(parts) != 3 {
			h.logger.Error("invalid remove_member_user callback data", "data", data)
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse group ID", "error", err)
			return
		}

		memberUserID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse user ID", "error", err)
			return
		}

		// Get group
		group, err := h.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
			})
			return
		}

		// Update membership status to removed
		err = h.groupMembershipRepo.UpdateMembershipStatus(ctx, groupID, memberUserID, domain.MembershipStatusRemoved)
		if err != nil {
			h.logger.Error("failed to update membership status", "group_id", groupID, "user_id", memberUserID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.RemoveMemberErrorUpdate),
			})
			return
		}

		// Log the action
		h.logAdminAction(userID, "remove_member", groupID, fmt.Sprintf("Removed user %d from group %s", memberUserID, group.Name))

		// Get display name
		displayName := h.getUserDisplayName(ctx, memberUserID, groupID)

		// Send confirmation
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.RemoveMemberSuccessFormat, displayName, group.Name),
		})
		if err != nil {
			h.logger.Error("failed to send confirmation", "error", err)
		}

		// Answer callback query
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}
}

// handleResolveEventFromCallback handles the resolve button click from event creation summary
func (h *BotHandler) handleResolveEventFromCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Message.Chat.ID

	// Answer callback query to remove loading state
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Parse event ID from callback data
	eventIDStr := strings.TrimPrefix(callback.Data, "resolve:")
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil {
		h.logger.Error("failed to parse event ID from callback", "user_id", userID, "data", callback.Data, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.ErrorRequestProcessing),
		})
		return
	}

	// Check if user can manage this event
	canManage, err := h.eventPermissionValidator.CanManageEvent(ctx, userID, eventID, h.config.AdminUserIDs)
	if err != nil {
		h.logger.Error("failed to check event management permission", "user_id", userID, "event_id", eventID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.EventResolutionErrorPermissionCheck),
		})
		return
	}

	if !canManage {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.EventResolutionErrorUnauthorized),
		})
		return
	}

	// Start resolution FSM session
	if err := h.eventResolutionFSM.Start(ctx, userID, chatID); err != nil {
		h.logger.Error("failed to start resolution FSM session", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   h.localizer.MustLocalize(locale.EventResolutionErrorStart),
		})
		return
	}

	// Create a new callback with the resolve: prefix to trigger FSM handling
	_ = h.eventResolutionFSM.HandleCallback(ctx, callback)
}

// handleEditEventCallback handles the edit button click from event creation summary
func (h *BotHandler) handleEditEventCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Message.Chat.ID

	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorEditEventNoPermission),
			ShowAlert:       true,
		})
		return
	}

	// Parse event ID from callback data: edit_event:EVENT_ID
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 2 {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorInvalidDataFormat),
			ShowAlert:       true,
		})
		return
	}

	eventID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorInvalidEventID),
			ShowAlert:       true,
		})
		return
	}

	// Check if event can be edited (no votes)
	canEdit, err := h.eventManager.CanEditEvent(ctx, eventID)
	if err != nil {
		h.logger.Error("failed to check if event can be edited", "event_id", eventID, "error", err)
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorRequestProcessing),
			ShowAlert:       true,
		})
		return
	}

	if !canEdit {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorEditEventHasVotes),
			ShowAlert:       true,
		})
		return
	}

	// Answer callback query
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Delete the message with buttons
	if callback.Message.Message != nil {
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: callback.Message.Message.ID,
		})
	}

	// Start edit FSM
	if err := h.eventEditFSM.Start(ctx, userID, chatID, eventID); err != nil {
		h.logger.Error("failed to start edit FSM", "user_id", userID, "event_id", eventID, "error", err)
		if err == domain.ErrEventHasVotes {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.ErrorEditEventHasVotes),
			})
		} else {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   h.localizer.MustLocalize(locale.ErrorEditEventStart),
			})
		}
		return
	}

	h.logger.Info("edit event FSM started", "user_id", userID, "event_id", eventID)
}

// handleDeleteGroupCallback handles the callback for deleting a group
func (h *BotHandler) handleDeleteGroupCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	// Check if this is group selection or confirmation
	if data == "delete_group_select" {
		// Get all groups
		groups, err := h.groupRepo.GetAllGroups(ctx)
		if err != nil {
			h.logger.Error("failed to get all groups", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
			})
			return
		}

		if len(groups) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.DeleteGroupEmpty),
			})
			return
		}

		// Build inline keyboard with groups
		var buttons [][]models.InlineKeyboardButton
		for _, group := range groups {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         group.Name,
					CallbackData: fmt.Sprintf("delete_group_confirm:%d", group.ID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalize(locale.DeleteGroupTitle) + "\n\n" + h.localizer.MustLocalize(locale.DeleteGroupSelectPrompt),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send group selection for deletion", "error", err)
		}

		// Answer callback query
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	// This is confirmation
	if strings.HasPrefix(data, "delete_group_confirm:") {
		// Parse group ID
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid delete_group_confirm callback data", "data", data)
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse group ID", "error", err)
			return
		}

		// Get group
		group, err := h.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
			})
			return
		}

		// Delete the group (this will cascade delete memberships, topics, etc.)
		err = h.groupRepo.DeleteGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to delete group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.DeleteGroupError),
			})
			return
		}

		// Log the action
		h.logAdminAction(userID, "delete_group", groupID, fmt.Sprintf("Deleted group %s", group.Name))

		// Send confirmation
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.GroupDeletedSuccess, group.Name),
		})
		if err != nil {
			h.logger.Error("failed to send confirmation", "error", err)
		}

		// Answer callback query
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}
}

// handleDeleteTopicCallback handles the callback for deleting a forum topic
func (h *BotHandler) handleDeleteTopicCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	// Check if this is group selection, topic selection, or confirmation
	if data == "delete_topic_select" {
		// Get all forum groups
		groups, err := h.groupRepo.GetAllGroups(ctx)
		if err != nil {
			h.logger.Error("failed to get all groups", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
			})
			return
		}

		// Filter forum groups
		var forumGroups []*domain.Group
		for _, group := range groups {
			if group.IsForum {
				forumGroups = append(forumGroups, group)
			}
		}

		if len(forumGroups) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.RenameTopicEmpty),
			})
			return
		}

		// Build inline keyboard with forum groups
		var buttons [][]models.InlineKeyboardButton
		for _, group := range forumGroups {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         group.Name,
					CallbackData: fmt.Sprintf("delete_topic_group:%d", group.ID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalize(locale.DeleteTopicTitle) + "\n\n" + h.localizer.MustLocalize(locale.DeleteTopicSelectForum),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send forum selection for topic deletion", "error", err)
		}

		// Answer callback query
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	// This is group selection
	if strings.HasPrefix(data, "delete_topic_group:") {
		// Parse group ID
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid delete_topic_group callback data", "data", data)
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse group ID", "error", err)
			return
		}

		// Get group
		group, err := h.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
			})
			return
		}

		// Get topics for this group
		topics, err := h.forumTopicRepo.GetForumTopicsByGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get forum topics", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ErrorRequestProcessing),
			})
			return
		}

		if len(topics) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalizeWithTemplate(locale.RenameTopicNoTopics, group.Name),
			})
			return
		}

		// Build inline keyboard with topics
		var buttons [][]models.InlineKeyboardButton
		for _, topic := range topics {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         fmt.Sprintf("%s (Thread ID: %d)", topic.Name, topic.MessageThreadID),
					CallbackData: fmt.Sprintf("delete_topic_confirm:%d", topic.ID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalizeWithTemplate(locale.DeleteTopicPromptWithName, group.Name),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send topic selection", "error", err)
		}

		// Answer callback query
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	// This is confirmation
	if strings.HasPrefix(data, "delete_topic_confirm:") {
		// Parse topic ID
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid delete_topic_confirm callback data", "data", data)
			return
		}

		topicID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse topic ID", "error", err)
			return
		}

		// Get topic
		topic, err := h.forumTopicRepo.GetForumTopic(ctx, topicID)
		if err != nil {
			h.logger.Error("failed to get topic", "topic_id", topicID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ErrorRequestProcessing),
			})
			return
		}

		if topic == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ErrorTopicNotFound),
			})
			return
		}

		// Delete the topic
		err = h.forumTopicRepo.DeleteForumTopic(ctx, topicID)
		if err != nil {
			h.logger.Error("failed to delete topic", "topic_id", topicID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.DeleteTopicError),
			})
			return
		}

		// Log the action
		h.logAdminAction(userID, "delete_topic", topic.GroupID, fmt.Sprintf("Deleted topic %s (ID: %d)", topic.Name, topicID))

		// Send confirmation
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.TopicDeletedSuccess, topic.Name),
		})
		if err != nil {
			h.logger.Error("failed to send confirmation", "error", err)
		}

		// Answer callback query
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}
}

// handleSoftDeleteGroupCallback handles soft delete (marking as deleted)
func (h *BotHandler) handleSoftDeleteGroupCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	if data == "soft_delete_group_select" {
		// Get all active groups
		groups, err := h.groupRepo.GetAllGroups(ctx)
		if err != nil {
			h.logger.Error("failed to get all groups", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
			})
			return
		}

		// Filter active groups
		var activeGroups []*domain.Group
		for _, group := range groups {
			if group.Status == domain.GroupStatusActive {
				activeGroups = append(activeGroups, group)
			}
		}

		if len(activeGroups) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.SoftDeleteGroupEmpty),
			})
			return
		}

		// Build inline keyboard with active groups
		var buttons [][]models.InlineKeyboardButton
		for _, group := range activeGroups {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         group.Name,
					CallbackData: fmt.Sprintf("soft_delete_group_confirm:%d", group.ID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalize(locale.SoftDeleteGroupTitle) + "\n\n" + h.localizer.MustLocalize(locale.SoftDeleteGroupSelectPrompt),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send group selection", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	if strings.HasPrefix(data, "soft_delete_group_confirm:") {
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid soft_delete_group_confirm callback data", "data", data)
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse group ID", "error", err)
			return
		}

		group, err := h.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
			})
			return
		}

		// Update status to deleted
		err = h.groupRepo.UpdateGroupStatus(ctx, groupID, domain.GroupStatusDeleted)
		if err != nil {
			h.logger.Error("failed to update group status", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.SoftDeleteGroupError),
			})
			return
		}

		h.logAdminAction(userID, "soft_delete_group", groupID, fmt.Sprintf("Marked group %s as deleted", group.Name))

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.GroupMarkedDeleted, group.Name),
		})
		if err != nil {
			h.logger.Error("failed to send confirmation", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}
}

// handleRestoreGroupCallback handles restoring deleted groups
func (h *BotHandler) handleRestoreGroupCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	if data == "restore_group_select" {
		// Get all groups
		groups, err := h.groupRepo.GetAllGroups(ctx)
		if err != nil {
			h.logger.Error("failed to get all groups", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
			})
			return
		}

		// Filter deleted groups
		var deletedGroups []*domain.Group
		for _, group := range groups {
			if group.Status == domain.GroupStatusDeleted {
				deletedGroups = append(deletedGroups, group)
			}
		}

		if len(deletedGroups) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.RestoreGroupEmpty),
			})
			return
		}

		// Build inline keyboard with deleted groups
		var buttons [][]models.InlineKeyboardButton
		for _, group := range deletedGroups {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         group.Name,
					CallbackData: fmt.Sprintf("restore_group_confirm:%d", group.ID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalize(locale.RestoreGroupTitle) + "\n\n" + h.localizer.MustLocalize(locale.RestoreGroupSelectPrompt),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send group selection", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	if strings.HasPrefix(data, "restore_group_confirm:") {
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid restore_group_confirm callback data", "data", data)
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse group ID", "error", err)
			return
		}

		group, err := h.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
			})
			return
		}

		// Update status to active
		err = h.groupRepo.UpdateGroupStatus(ctx, groupID, domain.GroupStatusActive)
		if err != nil {
			h.logger.Error("failed to update group status", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.SoftDeleteGroupError),
			})
			return
		}

		h.logAdminAction(userID, "restore_group", groupID, fmt.Sprintf("Restored group %s", group.Name))

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.GroupRestored, group.Name),
		})
		if err != nil {
			h.logger.Error("failed to send confirmation", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}
}

// handleRenameGroupCallback handles renaming groups
func (h *BotHandler) handleRenameGroupCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	if data == "rename_group_select" {
		groups, err := h.groupRepo.GetAllGroups(ctx)
		if err != nil {
			h.logger.Error("failed to get all groups", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
			})
			return
		}

		if len(groups) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.RenameGroupEmpty),
			})
			return
		}

		var buttons [][]models.InlineKeyboardButton
		for _, group := range groups {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         group.Name,
					CallbackData: fmt.Sprintf("rename_group_input:%d", group.ID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalize(locale.RenameGroupTitle) + "\n\n" + h.localizer.MustLocalize(locale.RenameGroupSelectPrompt),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send group selection", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	if strings.HasPrefix(data, "rename_group_input:") {
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid rename_group_input callback data", "data", data)
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse group ID", "error", err)
			return
		}

		group, err := h.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
			})
			return
		}

		// Start rename FSM session
		err = h.renameFSM.StartGroupRename(ctx, userID, callback.Message.Message.Chat.ID, groupID, group.Name)
		if err != nil {
			h.logger.Error("failed to start rename FSM", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.RenameGroupErrorStart),
			})
			return
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.RenameGroupPromptWithName, group.Name),
		})
		if err != nil {
			h.logger.Error("failed to send rename prompt", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}
}

// handleRenameTopicCallback handles renaming forum topics
func (h *BotHandler) handleRenameTopicCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            h.localizer.MustLocalize(locale.ErrorUnauthorized),
		})
		return
	}

	if data == "rename_topic_select" {
		groups, err := h.groupRepo.GetAllGroups(ctx)
		if err != nil {
			h.logger.Error("failed to get all groups", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ListGroupsErrorGet),
			})
			return
		}

		var forumGroups []*domain.Group
		for _, group := range groups {
			if group.IsForum {
				forumGroups = append(forumGroups, group)
			}
		}

		if len(forumGroups) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.RenameTopicEmpty),
			})
			return
		}

		var buttons [][]models.InlineKeyboardButton
		for _, group := range forumGroups {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         group.Name,
					CallbackData: fmt.Sprintf("rename_topic_group:%d", group.ID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalize(locale.RenameTopicTitle) + "\n\n" + h.localizer.MustLocalize(locale.RenameTopicSelectForum),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send forum selection", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	if strings.HasPrefix(data, "rename_topic_group:") {
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid rename_topic_group callback data", "data", data)
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse group ID", "error", err)
			return
		}

		group, err := h.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupMembersErrorGroup),
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.GroupErrorNotFound),
			})
			return
		}

		// Get topics for this group
		topics, err := h.forumTopicRepo.GetForumTopicsByGroup(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get forum topics", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ErrorRequestProcessing),
			})
			return
		}

		if len(topics) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalizeWithTemplate(locale.RenameTopicNoTopics, group.Name),
			})
			return
		}

		// Build inline keyboard with topics
		var buttons [][]models.InlineKeyboardButton
		for _, topic := range topics {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         fmt.Sprintf("%s (Thread ID: %d)", topic.Name, topic.MessageThreadID),
					CallbackData: fmt.Sprintf("rename_topic_input:%d", topic.ID),
				},
			})
		}

		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        h.localizer.MustLocalizeWithTemplate(locale.RenameTopicPromptWithName, group.Name),
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Error("failed to send topic selection", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}

	if strings.HasPrefix(data, "rename_topic_input:") {
		parts := strings.Split(data, ":")
		if len(parts) != 2 {
			h.logger.Error("invalid rename_topic_input callback data", "data", data)
			return
		}

		topicID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse topic ID", "error", err)
			return
		}

		topic, err := h.forumTopicRepo.GetForumTopic(ctx, topicID)
		if err != nil {
			h.logger.Error("failed to get topic", "topic_id", topicID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ErrorRequestProcessing),
			})
			return
		}

		if topic == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.ErrorTopicNotFound),
			})
			return
		}

		// Start rename FSM session
		err = h.renameFSM.StartTopicRename(ctx, userID, callback.Message.Message.Chat.ID, topicID, topic.Name)
		if err != nil {
			h.logger.Error("failed to start rename FSM", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   h.localizer.MustLocalize(locale.RenameGroupErrorStart),
			})
			return
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   h.localizer.MustLocalizeWithTemplate(locale.RenameTopicPromptWithName, topic.Name),
		})
		if err != nil {
			h.logger.Error("failed to send rename prompt", "error", err)
		}

		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		return
	}
}
