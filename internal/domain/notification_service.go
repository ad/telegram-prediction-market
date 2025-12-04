package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// BotInterface defines the interface for bot operations needed by NotificationService
type BotInterface interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
}

// ReminderRepository interface for reminder log operations
type ReminderRepository interface {
	WasReminderSent(ctx context.Context, eventID int64) (bool, error)
	MarkReminderSent(ctx context.Context, eventID int64) error
}

// NotificationService handles sending notifications to users and groups
type NotificationService struct {
	bot            BotInterface
	eventRepo      EventRepository
	predictionRepo PredictionRepository
	ratingRepo     RatingRepository
	reminderRepo   ReminderRepository
	groupID        int64
	logger         Logger
}

// NewNotificationService creates a new NotificationService
func NewNotificationService(
	b BotInterface,
	eventRepo EventRepository,
	predictionRepo PredictionRepository,
	ratingRepo RatingRepository,
	reminderRepo ReminderRepository,
	groupID int64,
	logger Logger,
) *NotificationService {
	return &NotificationService{
		bot:            b,
		eventRepo:      eventRepo,
		predictionRepo: predictionRepo,
		ratingRepo:     ratingRepo,
		reminderRepo:   reminderRepo,
		groupID:        groupID,
		logger:         logger,
	}
}

// SendNewEventNotification sends a notification to all participants when a new event is published
func (ns *NotificationService) SendNewEventNotification(ctx context.Context, eventID int64) error {
	// Get the event
	event, err := ns.eventRepo.GetEvent(ctx, eventID)
	if err != nil {
		ns.logger.Error("failed to get event for notification", "event_id", eventID, "error", err)
		return err
	}

	// Build notification message
	var sb strings.Builder
	sb.WriteString("🆕 *Новое событие для прогноза!*\n\n")
	sb.WriteString(fmt.Sprintf("❓ *Вопрос:* %s\n\n", event.Question))

	// Event type
	typeStr := ""
	switch event.EventType {
	case EventTypeBinary:
		typeStr = "Бинарное"
	case EventTypeMultiOption:
		typeStr = "Множественный выбор"
	case EventTypeProbability:
		typeStr = "Вероятностное"
	}
	sb.WriteString(fmt.Sprintf("📋 *Тип:* %s\n\n", typeStr))

	// Options
	sb.WriteString("*Варианты:*\n")
	for i, opt := range event.Options {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, opt))
	}
	sb.WriteString("\n")

	// Deadline
	timeUntil := time.Until(event.Deadline)
	deadlineStr := ""
	if timeUntil > 0 {
		hours := int(timeUntil.Hours())
		if hours > 24 {
			days := hours / 24
			deadlineStr = fmt.Sprintf("⏰ *Дедлайн:* %d дн. %d ч.", days, hours%24)
		} else {
			deadlineStr = fmt.Sprintf("⏰ *Дедлайн:* %d ч.", hours)
		}
	} else {
		deadlineStr = "⏰ *Дедлайн:* истёк"
	}
	sb.WriteString(deadlineStr + "\n\n")
	sb.WriteString("Голосуйте в опросе выше! 🗳")

	// Send notification to group
	_, err = ns.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: ns.groupID,
		Text:   sb.String(),
	})
	if err != nil {
		ns.logger.Error("failed to send new event notification", "event_id", eventID, "error", err)
		return err
	}

	ns.logger.Info("new event notification sent", "event_id", eventID)
	return nil
}

// SendAchievementNotification sends a notification to the user and publishes an announcement in the group
func (ns *NotificationService) SendAchievementNotification(ctx context.Context, userID int64, achievement *Achievement) error {
	// Map achievement codes to display names
	achievementNames := map[AchievementCode]string{
		AchievementSharpshooter:  "🎯 Меткий стрелок",
		AchievementProphet:       "🔮 Провидец",
		AchievementRiskTaker:     "🎲 Риск-мейкер",
		AchievementWeeklyAnalyst: "📊 Аналитик недели",
		AchievementVeteran:       "🏆 Старожил",
	}

	name := achievementNames[achievement.Code]
	if name == "" {
		name = string(achievement.Code)
	}

	// Send notification to user
	_, err := ns.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: userID,
		Text:   fmt.Sprintf("🎉 Поздравляем! Вы получили ачивку:\n\n%s", name),
	})
	if err != nil {
		ns.logger.Error("failed to send achievement notification to user", "user_id", userID, "achievement", achievement.Code, "error", err)
		// Don't return error, continue to send group announcement
	}

	// Publish announcement in group
	_, err = ns.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: ns.groupID,
		Text:   fmt.Sprintf("🎉 Участник получил ачивку: %s!", name),
	})
	if err != nil {
		ns.logger.Error("failed to send achievement announcement to group", "user_id", userID, "achievement", achievement.Code, "error", err)
		return err
	}

	ns.logger.Info("achievement notification sent", "user_id", userID, "achievement", achievement.Code)
	return nil
}

// PublishEventResults publishes event results to the group with outcome, correct count, top 5, rating changes, and achievements
func (ns *NotificationService) PublishEventResults(ctx context.Context, eventID int64, correctOption int) error {
	// Get the event
	event, err := ns.eventRepo.GetEvent(ctx, eventID)
	if err != nil {
		ns.logger.Error("failed to get event for results", "event_id", eventID, "error", err)
		return err
	}

	// Get all predictions for this event
	predictions, err := ns.predictionRepo.GetPredictionsByEvent(ctx, eventID)
	if err != nil {
		ns.logger.Error("failed to get predictions for results", "event_id", eventID, "error", err)
		return err
	}

	// Count correct predictions
	correctCount := 0
	for _, pred := range predictions {
		if pred.Option == correctOption {
			correctCount++
		}
	}

	// Get top 5 participants by overall rating
	topRatings, err := ns.ratingRepo.GetTopRatings(ctx, 5)
	if err != nil {
		ns.logger.Error("failed to get top ratings", "error", err)
		topRatings = []*Rating{} // Continue with empty list
	}

	// Build results message
	var sb strings.Builder
	sb.WriteString("🏁 *Событие завершено!*\n\n")
	sb.WriteString(fmt.Sprintf("❓ *Вопрос:* %s\n\n", event.Question))
	sb.WriteString(fmt.Sprintf("✅ *Правильный ответ:* %s\n\n", event.Options[correctOption]))
	sb.WriteString(fmt.Sprintf("📊 *Угадали:* %d из %d участников\n\n", correctCount, len(predictions)))

	if len(topRatings) > 0 {
		sb.WriteString("🏆 *Топ-5 участников:*\n")
		medals := []string{"🥇", "🥈", "🥉", "4.", "5."}
		for i, rating := range topRatings {
			sb.WriteString(fmt.Sprintf("%s %d очков\n", medals[i], rating.Score))
		}
	}

	// Send results to group
	_, err = ns.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: ns.groupID,
		Text:   sb.String(),
	})
	if err != nil {
		ns.logger.Error("failed to send results to group", "event_id", eventID, "error", err)
		return err
	}

	ns.logger.Info("event results published", "event_id", eventID, "correct_count", correctCount, "total_predictions", len(predictions))
	return nil
}

// SendDeadlineReminder sends reminders to participants who haven't voted yet
func (ns *NotificationService) SendDeadlineReminder(ctx context.Context, eventID int64) error {
	// Get the event
	event, err := ns.eventRepo.GetEvent(ctx, eventID)
	if err != nil {
		ns.logger.Error("failed to get event for reminder", "event_id", eventID, "error", err)
		return err
	}

	// Check if event is still active
	if event.Status != EventStatusActive {
		ns.logger.Debug("skipping reminder for non-active event", "event_id", eventID, "status", event.Status)
		return nil
	}

	// Get all predictions for this event
	predictions, err := ns.predictionRepo.GetPredictionsByEvent(ctx, eventID)
	if err != nil {
		ns.logger.Error("failed to get predictions for reminder", "event_id", eventID, "error", err)
		return err
	}

	// Create a map of users who have voted
	votedUsers := make(map[int64]bool)
	for _, pred := range predictions {
		votedUsers[pred.UserID] = true
	}

	// Get all participants (users who have ratings)
	// For simplicity, we'll send reminders to all users in the rating system who haven't voted
	allRatings, err := ns.ratingRepo.GetTopRatings(ctx, 1000) // Get up to 1000 users
	if err != nil {
		ns.logger.Error("failed to get all ratings for reminder", "error", err)
		return err
	}

	// Build reminder message
	timeUntil := time.Until(event.Deadline)
	hours := int(timeUntil.Hours())

	reminderText := fmt.Sprintf("⏰ *Напоминание!*\n\n"+
		"До дедлайна события осталось ~%d часов:\n\n"+
		"❓ %s\n\n"+
		"Не забудьте проголосовать! 🗳", hours, event.Question)

	// Send reminders to users who haven't voted
	sentCount := 0
	for _, rating := range allRatings {
		if !votedUsers[rating.UserID] {
			_, err := ns.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: rating.UserID,
				Text:   reminderText,
			})
			if err != nil {
				ns.logger.Warn("failed to send reminder to user", "user_id", rating.UserID, "error", err)
				// Continue sending to other users
			} else {
				sentCount++
			}
		}
	}

	ns.logger.Info("deadline reminders sent", "event_id", eventID, "sent_count", sentCount)
	return nil
}

// StartScheduler starts the notification scheduler with hourly checks for deadline reminders
func (ns *NotificationService) StartScheduler(ctx context.Context) error {
	// Perform startup recovery first
	if err := ns.performStartupRecovery(ctx); err != nil {
		ns.logger.Error("startup recovery failed", "error", err)
		// Don't return error, continue with scheduler
	}

	// Start the scheduler
	go ns.runScheduler(ctx)

	ns.logger.Info("notification scheduler started")
	return nil
}

// runScheduler runs the scheduler loop
func (ns *NotificationService) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ns.logger.Info("notification scheduler stopped")
			return
		case <-ticker.C:
			ns.checkAndSendReminders(ctx)
		}
	}
}

// checkAndSendReminders checks for events with deadline in 24-25 hours and sends reminders
func (ns *NotificationService) checkAndSendReminders(ctx context.Context) {
	now := time.Now()
	start := now.Add(24 * time.Hour)
	end := now.Add(25 * time.Hour)

	// Get events with deadline in the 24-25 hour window
	events, err := ns.getEventsByDeadlineRange(ctx, start, end)
	if err != nil {
		ns.logger.Error("failed to get events for reminders", "error", err)
		return
	}

	for _, event := range events {
		// Skip if event is no longer active
		if event.Status != EventStatusActive {
			continue
		}

		// Check if reminder was already sent
		if ns.wasReminderSent(ctx, event.ID) {
			continue
		}

		// Send reminder
		if err := ns.SendDeadlineReminder(ctx, event.ID); err != nil {
			ns.logger.Error("failed to send deadline reminder", "event_id", event.ID, "error", err)
			continue
		}

		// Mark reminder as sent
		if err := ns.markReminderSent(ctx, event.ID); err != nil {
			ns.logger.Error("failed to mark reminder as sent", "event_id", event.ID, "error", err)
		}
	}
}

// performStartupRecovery checks for missed reminders during downtime
func (ns *NotificationService) performStartupRecovery(ctx context.Context) error {
	now := time.Now()
	start := now.Add(24 * time.Hour)
	end := now.Add(48 * time.Hour)

	// Get events with deadline in the next 24-48 hours
	events, err := ns.getEventsByDeadlineRange(ctx, start, end)
	if err != nil {
		ns.logger.Error("failed to get events for startup recovery", "error", err)
		return err
	}

	recoveredCount := 0
	for _, event := range events {
		// Skip if event is no longer active
		if event.Status != EventStatusActive {
			continue
		}

		// Check if reminder was already sent
		if ns.wasReminderSent(ctx, event.ID) {
			continue
		}

		// Send reminder immediately
		if err := ns.SendDeadlineReminder(ctx, event.ID); err != nil {
			ns.logger.Error("failed to send recovery reminder", "event_id", event.ID, "error", err)
			continue
		}

		// Mark reminder as sent
		if err := ns.markReminderSent(ctx, event.ID); err != nil {
			ns.logger.Error("failed to mark recovery reminder as sent", "event_id", event.ID, "error", err)
		}

		recoveredCount++
	}

	if recoveredCount > 0 {
		ns.logger.Info("startup recovery completed", "recovered_reminders", recoveredCount)
	}

	return nil
}

// getEventsByDeadlineRange retrieves events with deadline in the specified range
// This is a helper method that wraps the repository call
func (ns *NotificationService) getEventsByDeadlineRange(ctx context.Context, start, end time.Time) ([]*Event, error) {
	// We need to add this method to the EventRepository interface
	// For now, we'll get all active events and filter
	events, err := ns.eventRepo.GetActiveEvents(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*Event
	for _, event := range events {
		if event.Deadline.After(start) && event.Deadline.Before(end) {
			filtered = append(filtered, event)
		}
	}

	return filtered, nil
}

// wasReminderSent checks if a reminder was already sent for an event
func (ns *NotificationService) wasReminderSent(ctx context.Context, eventID int64) bool {
	sent, err := ns.reminderRepo.WasReminderSent(ctx, eventID)
	if err != nil {
		ns.logger.Error("failed to check if reminder was sent", "event_id", eventID, "error", err)
		return false // Assume not sent on error
	}
	return sent
}

// markReminderSent marks a reminder as sent for an event
func (ns *NotificationService) markReminderSent(ctx context.Context, eventID int64) error {
	return ns.reminderRepo.MarkReminderSent(ctx, eventID)
}
