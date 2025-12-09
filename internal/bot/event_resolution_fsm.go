package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/locale"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// FSM state constants for event resolution
const (
	StateResolveSelectEvent  = "resolve_select_event"
	StateResolveSelectOption = "resolve_select_option"
	StateResolveComplete     = "resolve_complete"
)

// EventResolutionFSM manages the event resolution state machine
type EventResolutionFSM struct {
	storage                  *storage.FSMStorage
	bot                      *bot.Bot
	eventManager             *domain.EventManager
	achievementTracker       *domain.AchievementTracker
	ratingCalculator         *domain.RatingCalculator
	predictionRepo           domain.PredictionRepository
	groupRepo                domain.GroupRepository
	forumTopicRepo           domain.ForumTopicRepository
	eventPermissionValidator *domain.EventPermissionValidator
	notificationService      *domain.NotificationService
	config                   *config.Config
	logger                   domain.Logger
	localizer                locale.Localizer
}

// NewEventResolutionFSM creates a new FSM for event resolution
func NewEventResolutionFSM(
	storage *storage.FSMStorage,
	b *bot.Bot,
	eventManager *domain.EventManager,
	achievementTracker *domain.AchievementTracker,
	ratingCalculator *domain.RatingCalculator,
	predictionRepo domain.PredictionRepository,
	groupRepo domain.GroupRepository,
	forumTopicRepo domain.ForumTopicRepository,
	eventPermissionValidator *domain.EventPermissionValidator,
	notificationService *domain.NotificationService,
	cfg *config.Config,
	logger domain.Logger,
	localizer locale.Localizer,
) *EventResolutionFSM {
	return &EventResolutionFSM{
		storage:                  storage,
		bot:                      b,
		eventManager:             eventManager,
		achievementTracker:       achievementTracker,
		ratingCalculator:         ratingCalculator,
		predictionRepo:           predictionRepo,
		groupRepo:                groupRepo,
		forumTopicRepo:           forumTopicRepo,
		eventPermissionValidator: eventPermissionValidator,
		notificationService:      notificationService,
		config:                   cfg,
		logger:                   logger,
		localizer:                localizer,
	}
}

// Start initializes a new FSM session for event resolution
func (f *EventResolutionFSM) Start(ctx context.Context, userID int64, chatID int64) error {
	// Initialize context with chat ID
	initialContext := &domain.EventResolutionContext{
		ChatID:     chatID,
		MessageIDs: []int{},
	}

	// Store initial state
	if err := f.storage.Set(ctx, userID, StateResolveSelectEvent, initialContext.ToMap()); err != nil {
		f.logger.Error("failed to start resolution FSM session", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("resolution FSM session started", "user_id", userID, "state", StateResolveSelectEvent)
	return nil
}

// HasSession checks if user has an active FSM session
func (f *EventResolutionFSM) HasSession(ctx context.Context, userID int64) (bool, error) {
	state, _, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return false, nil
		}
		return false, err
	}

	// Only return true if the state is an event resolution state
	switch state {
	case StateResolveSelectEvent, StateResolveSelectOption, StateResolveComplete:
		return true, nil
	default:
		return false, nil
	}
}

// HandleCallback processes callback queries for the resolution flow
func (f *EventResolutionFSM) HandleCallback(ctx context.Context, callback *models.CallbackQuery) error {
	userID := callback.From.ID

	// Get current state and context
	state, contextData, err := f.storage.Get(ctx, userID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			f.logger.Warn("no active resolution session", "user_id", userID)
			return nil
		}
		return err
	}

	// Parse context
	resolutionContext := &domain.EventResolutionContext{}
	if err := resolutionContext.FromMap(contextData); err != nil {
		f.logger.Error("failed to parse resolution context", "user_id", userID, "error", err)
		return err
	}

	// Route based on state
	switch state {
	case StateResolveSelectEvent:
		return f.handleEventSelection(ctx, callback, userID, resolutionContext)
	case StateResolveSelectOption:
		return f.handleOptionSelection(ctx, callback, userID, resolutionContext)
	default:
		f.logger.Warn("unknown resolution state", "user_id", userID, "state", state)
		return nil
	}
}

// handleEventSelection processes event selection callback
func (f *EventResolutionFSM) handleEventSelection(ctx context.Context, callback *models.CallbackQuery, userID int64, context *domain.EventResolutionContext) error {
	// Answer callback query
	_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Parse event ID from callback data (format: "resolve:eventID")
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid callback data format")
	}

	eventID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		f.logger.Error("failed to parse event ID", "error", err)
		return err
	}

	// Check if user can manage this event
	canManage, err := f.eventPermissionValidator.CanManageEvent(ctx, userID, eventID, f.config.AdminUserIDs)
	if err != nil {
		f.logger.Error("failed to check event management permission", "user_id", userID, "event_id", eventID, "error", err)
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: context.ChatID,
			Text:   f.localizer.MustLocalize(locale.EventResolutionErrorPermissionCheck),
		})
		if msg != nil {
			context.MessageIDs = append(context.MessageIDs, msg.ID)
		}
		return err
	}

	if !canManage {
		f.logger.Warn("unauthorized event management attempt", "user_id", userID, "event_id", eventID)
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: context.ChatID,
			Text:   f.localizer.MustLocalize(locale.EventResolutionErrorUnauthorized),
		})
		if msg != nil {
			context.MessageIDs = append(context.MessageIDs, msg.ID)
		}
		return nil
	}

	// Get the event
	event, err := f.eventManager.GetEvent(ctx, eventID)
	if err != nil {
		f.logger.Error("failed to get event", "event_id", eventID, "error", err)
		msg, _ := f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: context.ChatID,
			Text:   f.localizer.MustLocalize(locale.EventResolutionErrorGetEvent),
		})
		if msg != nil {
			context.MessageIDs = append(context.MessageIDs, msg.ID)
		}
		return err
	}

	// Store event ID in context
	context.EventID = eventID

	// Build inline keyboard with options
	var buttons [][]models.InlineKeyboardButton
	for i, option := range event.Options {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         option,
				CallbackData: fmt.Sprintf("resolve:option:%d", i),
			},
		})
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	msg, err := f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      context.ChatID,
		Text:        f.localizer.MustLocalizeWithTemplate(locale.EventResolutionSelectCorrectAnswer, event.Question),
		ReplyMarkup: kb,
	})
	if err != nil {
		f.logger.Error("failed to send option selection", "error", err)
		return err
	}

	if msg != nil {
		context.MessageIDs = append(context.MessageIDs, msg.ID)
	}

	// Transition to option selection state
	if err := f.storage.Set(ctx, userID, StateResolveSelectOption, context.ToMap()); err != nil {
		f.logger.Error("failed to transition to option selection", "user_id", userID, "error", err)
		return err
	}

	f.logger.Info("state transition", "user_id", userID, "old_state", StateResolveSelectEvent, "new_state", StateResolveSelectOption)
	return nil
}

// handleOptionSelection processes option selection callback
func (f *EventResolutionFSM) handleOptionSelection(ctx context.Context, callback *models.CallbackQuery, userID int64, context *domain.EventResolutionContext) error {
	// Answer callback query
	_, _ = f.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Parse option index from callback data (format: "resolve:option:index")
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid callback data format")
	}

	optionIndex, err := strconv.Atoi(parts[2])
	if err != nil {
		f.logger.Error("failed to parse option index", "error", err)
		return err
	}

	// Delete all accumulated messages
	f.deleteMessages(ctx, context.ChatID, context.MessageIDs...)

	// Resolve the event
	if err := f.eventManager.ResolveEvent(ctx, context.EventID, optionIndex); err != nil {
		f.logger.Error("failed to resolve event", "event_id", context.EventID, "error", err)
		_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: context.ChatID,
			Text:   f.localizer.MustLocalize(locale.EventResolutionErrorResolve),
		})
		// Clean up session
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Get the event to show details
	event, err := f.eventManager.GetEvent(ctx, context.EventID)
	if err != nil {
		f.logger.Error("failed to get event", "event_id", context.EventID, "error", err)
		// Clean up session
		_ = f.storage.Delete(ctx, userID)
		return err
	}

	// Log the action
	isAdmin := false
	for _, adminID := range f.config.AdminUserIDs {
		if adminID == userID {
			isAdmin = true
			break
		}
	}

	if isAdmin {
		f.logger.Info("admin resolved event", "user_id", userID, "event_id", context.EventID, "correct_option", optionIndex)
	} else {
		f.logger.Info("creator resolved event", "user_id", userID, "event_id", context.EventID, "correct_option", optionIndex)
	}

	// Calculate scores
	if err := f.ratingCalculator.CalculateScores(ctx, context.EventID, optionIndex); err != nil {
		f.logger.Error("failed to calculate scores", "event_id", context.EventID, "error", err)
	}

	// Check and award achievements for all participants
	predictions, err := f.predictionRepo.GetPredictionsByEvent(ctx, context.EventID)
	if err == nil {
		for _, pred := range predictions {
			// Check if user just gained event creation permission
			f.checkAndNotifyEventCreationPermission(ctx, pred.UserID, event.GroupID)

			achievements, err := f.achievementTracker.CheckAndAwardAchievements(ctx, pred.UserID, event.GroupID)
			if err != nil {
				f.logger.Error("failed to check achievements", "user_id", pred.UserID, "group_id", event.GroupID, "error", err)
				continue
			}

			// Send achievement notifications
			for _, ach := range achievements {
				f.sendAchievementNotification(ctx, pred.UserID, ach)
			}
		}
	}

	// Stop the poll
	if event.PollID != "" && event.PollMessageID != 0 {
		// Get the group to obtain Telegram chat ID
		group, err := f.groupRepo.GetGroup(ctx, event.GroupID)
		if err != nil {
			f.logger.Error("failed to get group for stopping poll", "event_id", event.ID, "group_id", event.GroupID, "error", err)
		} else {
			_, err := f.bot.StopPoll(ctx, &bot.StopPollParams{
				ChatID:    group.TelegramChatID,
				MessageID: event.PollMessageID,
			})
			if err != nil {
				f.logger.Error("failed to stop poll", "event_id", event.ID, "poll_id", event.PollID, "message_id", event.PollMessageID, "telegram_chat_id", group.TelegramChatID, "error", err)
			} else {
				f.logger.Info("poll stopped", "event_id", event.ID, "poll_id", event.PollID, "message_id", event.PollMessageID, "telegram_chat_id", group.TelegramChatID)
			}
		}
	}

	// Publish event results to the group
	group, err := f.groupRepo.GetGroup(ctx, event.GroupID)
	if err != nil {
		f.logger.Error("failed to get group for publishing results", "event_id", event.ID, "group_id", event.GroupID, "error", err)
	} else {
		if err := f.notificationService.PublishEventResults(ctx, context.EventID, optionIndex, group.TelegramChatID, f.forumTopicRepo); err != nil {
			f.logger.Error("failed to publish event results", "event_id", context.EventID, "error", err)
		}
	}

	// Send confirmation to user (final message - not deleted)
	_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: context.ChatID,
		Text:   f.localizer.MustLocalize(locale.EventResolutionSuccess),
	})

	// Clean up session
	if err := f.storage.Delete(ctx, userID); err != nil {
		f.logger.Error("failed to delete resolution session", "user_id", userID, "error", err)
	}

	f.logger.Info("resolution FSM session completed", "user_id", userID, "event_id", context.EventID)
	return nil
}

// deleteMessages is a helper to delete multiple messages
func (f *EventResolutionFSM) deleteMessages(ctx context.Context, chatID int64, messageIDs ...int) {
	deleteMessages(ctx, f.bot, f.logger, chatID, messageIDs...)
}

// checkAndNotifyEventCreationPermission checks if user just gained permission to create events
// and sends a notification with instructions
func (f *EventResolutionFSM) checkAndNotifyEventCreationPermission(ctx context.Context, userID int64, groupID int64) {
	// Skip admins - they always have permission
	isAdmin := false
	for _, adminID := range f.config.AdminUserIDs {
		if adminID == userID {
			isAdmin = true
			break
		}
	}
	if isAdmin {
		return
	}

	// Check current participation count
	canCreate, participationCount, err := f.eventPermissionValidator.CanCreateEvent(ctx, userID, groupID, f.config.AdminUserIDs)
	if err != nil {
		f.logger.Error("failed to check event creation permission", "user_id", userID, "group_id", groupID, "error", err)
		return
	}

	// If user just reached the minimum required participation (exactly at the threshold)
	if canCreate && participationCount == f.config.MinEventsToCreate {
		// Get group information
		group, err := f.groupRepo.GetGroup(ctx, groupID)
		if err != nil {
			f.logger.Error("failed to get group for permission notification", "group_id", groupID, "error", err)
			return
		}

		groupName := f.localizer.MustLocalize(locale.GroupReferenceDefault)
		if group != nil && group.Name != "" {
			groupName = f.localizer.MustLocalizeWithTemplate(locale.GroupReferenceNamed, group.Name)
		}

		// Send notification with instructions
		message := f.localizer.MustLocalizeWithTemplate(
			locale.EventResolutionPermissionGranted,
			fmt.Sprintf("%d", participationCount),
			groupName,
		) + "\n\n" + f.localizer.MustLocalize(locale.EventResolutionPermissionInstructions)

		_, err = f.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: userID,
			Text:   message,
		})
		if err != nil {
			f.logger.Error("failed to send event creation permission notification", "user_id", userID, "group_id", groupID, "error", err)
		} else {
			f.logger.Info("sent event creation permission notification", "user_id", userID, "group_id", groupID, "participation_count", participationCount)
		}
	}
}

// sendAchievementNotification sends achievement notification to user
func (f *EventResolutionFSM) sendAchievementNotification(ctx context.Context, userID int64, achievement *domain.Achievement) {
	achievementNames := map[domain.AchievementCode]string{
		domain.AchievementSharpshooter:    f.localizer.MustLocalize(locale.AchievementSharpshooterName),
		domain.AchievementProphet:         f.localizer.MustLocalize(locale.AchievementProphetName),
		domain.AchievementRiskTaker:       f.localizer.MustLocalize(locale.AchievementRiskTakerName),
		domain.AchievementWeeklyAnalyst:   f.localizer.MustLocalize(locale.AchievementWeeklyAnalystName),
		domain.AchievementVeteran:         f.localizer.MustLocalize(locale.AchievementVeteranName),
		domain.AchievementEventOrganizer:  f.localizer.MustLocalize(locale.AchievementEventOrganizerName),
		domain.AchievementActiveOrganizer: f.localizer.MustLocalize(locale.AchievementActiveOrganizerName),
		domain.AchievementMasterOrganizer: f.localizer.MustLocalize(locale.AchievementMasterOrganizerName),
	}

	name := achievementNames[achievement.Code]
	if name == "" {
		name = string(achievement.Code)
	}

	// Get group information
	group, err := f.groupRepo.GetGroup(ctx, achievement.GroupID)
	if err != nil {
		f.logger.Error("failed to get group for achievement notification", "group_id", achievement.GroupID, "error", err)
	}

	groupName := f.localizer.MustLocalize(locale.LabelGroup)
	if group != nil && group.Name != "" {
		groupName = f.localizer.MustLocalizeWithTemplate(locale.AchievementNotificationGroup, group.Name)
	}

	// Send to user with group context
	_, _ = f.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: userID,
		Text:   f.localizer.MustLocalizeWithTemplate(locale.EventResolutionAchievementNotification, groupName, name),
	})
}
