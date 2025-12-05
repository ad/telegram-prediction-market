package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"

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
	eventPermissionValidator *domain.EventPermissionValidator
	groupRepo                domain.GroupRepository
	groupMembershipRepo      domain.GroupMembershipRepository
	deepLinkService          *domain.DeepLinkService
	ratingRepo               domain.RatingRepository
	createGroupState         map[int64]bool // Tracks users in create_group flow
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
	eventPermissionValidator *domain.EventPermissionValidator,
	groupRepo domain.GroupRepository,
	groupMembershipRepo domain.GroupMembershipRepository,
	deepLinkService *domain.DeepLinkService,
	ratingRepo domain.RatingRepository,
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
		eventPermissionValidator: eventPermissionValidator,
		groupRepo:                groupRepo,
		groupMembershipRepo:      groupMembershipRepo,
		deepLinkService:          deepLinkService,
		ratingRepo:               ratingRepo,
		createGroupState:         make(map[int64]bool),
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
func (h *BotHandler) getUserDisplayName(ctx context.Context, userID int64) string {
	// Try to get user information from the bot API
	// Since we don't have direct access to the bot API's GetChat method for users,
	// we'll use the rating repository which stores username information
	rating, err := h.ratingCalculator.GetUserRating(ctx, userID)
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
				Text:   "❌ У вас нет прав для выполнения этой команды.",
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

// HandleHelp handles the /help command
func (h *BotHandler) HandleHelp(ctx context.Context, b *bot.Bot, update *models.Update) {
	helpText := `🤖 Telegram Prediction Market Bot

════════════════════
📋 ДОСТУПНЫЕ КОМАНДЫ
════════════════════

👤 Для всех пользователей:
  /help — Показать эту справку
  /rating — Топ-10 участников по очкам
  /my — Ваша статистика и ачивки
  /events — Список активных событий

👑 Для администраторов:
  /create_event — Создать новое событие
  /resolve_event — Завершить событие и подвести итоги
  /edit_event — Редактировать событие (только без голосов)

════════════════════
💰 ПРАВИЛА НАЧИСЛЕНИЯ ОЧКОВ
════════════════════

✅ За правильный прогноз:
  • Бинарное событие (Да/Нет): +10 очков
  • Множественный выбор (3-6 вариантов): +15 очков
  • Вероятностное событие: +15 очков

🎁 Бонусы:
  • Меньшинство (<40% голосов): +5 очков
  • Ранний голос (первые 12 часов): +3 очка
  • Участие в любом событии: +1 очко

❌ Штрафы:
  • Неправильный прогноз: -3 очка

════════════════════
🏆 АЧИВКИ
════════════════════

🎯 Меткий стрелок
   → 3 правильных прогноза подряд

🔮 Провидец
   → 10 правильных прогнозов подряд

🎲 Риск-мейкер
   → 3 правильных прогноза в меньшинстве подряд

📊 Аналитик недели
   → Больше всех очков за неделю

🏆 Старожил
   → Участие в 50 событиях

════════════════════
🎲 ТИПЫ СОБЫТИЙ
════════════════════

1️⃣ Бинарное
   → Да/Нет вопросы

2️⃣ Множественный выбор
   → 2-6 вариантов ответа

3️⃣ Вероятностное
   → Диапазоны вероятности
   (0-25%, 25-50%, 50-75%, 75-100%)

════════════════════

⏰ Голосуйте до дедлайна!
За 24 часа до окончания придёт напоминание 🔔`

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   helpText,
	})
	if err != nil {
		h.logger.Error("failed to send help message", "error", err)
	}
}

// HandleRating handles the /rating command
func (h *BotHandler) HandleRating(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Get top 10 ratings
	ratings, err := h.ratingCalculator.GetTopRatings(ctx, 10)
	if err != nil {
		h.logger.Error("failed to get top ratings", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Ошибка при получении рейтинга.",
		})
		return
	}

	if len(ratings) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "📊 Рейтинг пока пуст. Начните делать прогнозы!",
		})
		return
	}

	// Build rating message
	var sb strings.Builder
	sb.WriteString("🏆 ТОП-10 УЧАСТНИКОВ\n\n")

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

		sb.WriteString(fmt.Sprintf("%s%s — %d очков\n", medal, displayName, rating.Score))
		sb.WriteString(fmt.Sprintf("     📊 Точность: %.1f\n", accuracy))
		sb.WriteString(fmt.Sprintf("     🔥 Серия: %d\n", rating.Streak))
		sb.WriteString(fmt.Sprintf("     ✅ %d\n", rating.CorrectCount))
		sb.WriteString(fmt.Sprintf("     ❌ %d\n\n", rating.WrongCount))
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   sb.String(),
	})
	if err != nil {
		h.logger.Error("failed to send rating message", "error", err)
	}
}

// HandleMy handles the /my command
func (h *BotHandler) HandleMy(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID

	// Get user rating
	rating, err := h.ratingCalculator.GetUserRating(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get user rating", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Ошибка при получении статистики.",
		})
		return
	}

	// Get user achievements
	achievements, err := h.achievementTracker.GetUserAchievements(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get user achievements", "user_id", userID, "error", err)
		achievements = []*domain.Achievement{} // Continue with empty achievements
	}

	// Build stats message
	var sb strings.Builder
	sb.WriteString("📊 ВАША СТАТИСТИКА\n")

	total := rating.CorrectCount + rating.WrongCount
	accuracy := 0.0
	if total > 0 {
		accuracy = float64(rating.CorrectCount) / float64(total) * 100
	}

	sb.WriteString(fmt.Sprintf("💰 Очки: %d\n", rating.Score))
	sb.WriteString(fmt.Sprintf("✅ Правильных: %d\n", rating.CorrectCount))
	sb.WriteString(fmt.Sprintf("❌ Неправильных: %d\n", rating.WrongCount))
	sb.WriteString(fmt.Sprintf("📈 Точность: %.1f%%\n", accuracy))
	sb.WriteString(fmt.Sprintf("🔥 Текущая серия: %d\n", rating.Streak))
	sb.WriteString(fmt.Sprintf("📝 Всего прогнозов: %d\n\n", total))

	// Add achievements
	if len(achievements) > 0 {
		sb.WriteString("🏆 ВАШИ АЧИВКИ\n")
		achievementNames := map[domain.AchievementCode]string{
			domain.AchievementSharpshooter:  "🎯 Меткий стрелок",
			domain.AchievementProphet:       "🔮 Провидец",
			domain.AchievementRiskTaker:     "🎲 Риск-мейкер",
			domain.AchievementWeeklyAnalyst: "📊 Аналитик недели",
			domain.AchievementVeteran:       "🏆 Старожил",
		}
		for _, ach := range achievements {
			name := achievementNames[ach.Code]
			if name == "" {
				name = string(ach.Code)
			}
			sb.WriteString(fmt.Sprintf("  • %s\n", name))
		}
	} else {
		sb.WriteString("🏆 АЧИВКИ\n")
		sb.WriteString("Пока нет. Продолжайте делать прогнозы!")
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   sb.String(),
	})
	if err != nil {
		h.logger.Error("failed to send my stats message", "error", err)
	}
}

// HandleEvents handles the /events command
func (h *BotHandler) HandleEvents(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Get all active events
	events, err := h.eventManager.GetActiveEvents(ctx)
	if err != nil {
		h.logger.Error("failed to get active events", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Ошибка при получении списка событий.",
		})
		return
	}

	if len(events) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "📋 Нет активных событий. Ожидайте новых!",
		})
		return
	}

	// Build events list message
	var sb strings.Builder
	sb.WriteString("📋 АКТИВНЫЕ СОБЫТИЯ\n")
	sb.WriteString("════════════════════\n\n")

	for i, event := range events {
		sb.WriteString(fmt.Sprintf("▸ %d. %s\n\n", i+1, event.Question))

		// Event type
		typeStr := ""
		typeIcon := ""
		switch event.EventType {
		case domain.EventTypeBinary:
			typeStr = "Бинарное"
			typeIcon = "1️⃣"
		case domain.EventTypeMultiOption:
			typeStr = "Множественный выбор"
			typeIcon = "2️⃣"
		case domain.EventTypeProbability:
			typeStr = "Вероятностное"
			typeIcon = "3️⃣"
		}
		sb.WriteString(fmt.Sprintf("%s Тип: %s\n", typeIcon, typeStr))

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
		sb.WriteString("\n📊 Варианты:\n")
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
		sb.WriteString(fmt.Sprintf("\n👥 Всего проголосовало: %d\n", totalVotes))

		// Deadline
		timeUntil := time.Until(event.Deadline)
		deadlineStr := ""
		if timeUntil > 0 {
			hours := int(timeUntil.Hours())
			minutes := int(timeUntil.Minutes()) % 60
			if hours > 24 {
				days := hours / 24
				deadlineStr = fmt.Sprintf("⏰ Осталось: %d дн. %d ч.", days, hours%24)
			} else if hours > 0 {
				deadlineStr = fmt.Sprintf("⏰ Осталось: %d ч. %d мин.", hours, minutes)
			} else {
				deadlineStr = fmt.Sprintf("⏰ Осталось: %d мин.", minutes)
			}
			// Show deadline in local timezone
			localDeadline := event.Deadline.In(h.config.Timezone)
			deadlineStr += fmt.Sprintf(" (до %s)", localDeadline.Format("02.01 15:04"))
		} else {
			deadlineStr = "⏰ Дедлайн истёк"
		}
		sb.WriteString(deadlineStr + "\n")
		sb.WriteString("\n───────────────────────────\n\n")
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
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

	// Get all active events and find the matching one
	events, err := h.eventManager.GetActiveEvents(ctx)
	if err != nil {
		h.logger.Error("failed to get active events", "error", err)
		return
	}

	var event *domain.Event
	for _, e := range events {
		if e.PollID == pollID {
			event = e
			break
		}
	}

	if event == nil {
		h.logger.Warn("poll answer for unknown event", "poll_id", pollID)
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

		h.logger.Info("prediction updated", "user_id", userID, "event_id", event.ID, "option", selectedOption)
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

		h.logger.Info("prediction saved", "user_id", userID, "event_id", event.ID, "option", selectedOption)
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
	rating, err := h.ratingCalculator.GetUserRating(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get user rating", "user_id", userID, "error", err)
		return
	}

	// Update username if it's different or empty
	if rating.Username != username && username != "" {
		rating.Username = username
		if err := h.ratingCalculator.UpdateRatingUsername(ctx, rating); err != nil {
			h.logger.Error("failed to update username", "user_id", userID, "error", err)
		}
	}
}

// HandleCreateEvent handles the /create_event command (multi-step conversation)
func (h *BotHandler) HandleCreateEvent(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Check if user has permission to create events
	// Admins are exempt from participation requirement
	canCreate, participationCount, err := h.eventPermissionValidator.CanCreateEvent(ctx, userID, h.config.AdminUserIDs)
	if err != nil {
		h.logger.Error("failed to check event creation permission", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при проверке прав доступа. Попробуйте позже.",
		})
		return
	}

	if !canCreate {
		// User doesn't have enough participation
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Для создания событий нужно участвовать минимум в %d завершенных событиях. У вас: %d.", h.config.MinEventsToCreate, participationCount),
		})
		h.logger.Info("event creation denied due to insufficient participation", "user_id", userID, "participation_count", participationCount, "required", h.config.MinEventsToCreate)
		return
	}

	// Start FSM session for user
	if err := h.eventCreationFSM.Start(ctx, userID, chatID); err != nil {
		h.logger.Error("failed to start FSM session", "user_id", userID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при создании события. Попробуйте позже.",
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

	userID := update.Message.From.ID

	// Check if user is in create_group flow
	if h.createGroupState[userID] {
		h.handleCreateGroupInput(ctx, b, update)
		return
	}

	// Check if user has active FSM session
	hasSession, err := h.eventCreationFSM.HasSession(ctx, userID)
	if err != nil {
		h.logger.Error("failed to check FSM session", "user_id", userID, "error", err)
		return
	}

	if hasSession {
		// Route to FSM
		if err := h.eventCreationFSM.HandleMessage(ctx, update); err != nil {
			h.logger.Error("FSM message handling failed", "user_id", userID, "error", err)

			// Inform user to restart
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "❌ Произошла ошибка. Пожалуйста, начните заново с /create_event",
			})
		}
		return
	}

	// No active conversation - ignore message
}

// handleCreateGroupInput handles the group name input for create_group flow
func (h *BotHandler) handleCreateGroupInput(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	groupName := strings.TrimSpace(update.Message.Text)

	// Clear state
	delete(h.createGroupState, userID)

	// Validate group name
	if groupName == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Название группы не может быть пустым.",
		})
		return
	}

	// Create group
	group := &domain.Group{
		TelegramChatID: chatID,
		Name:           groupName,
		CreatedAt:      time.Now(),
		CreatedBy:      userID,
	}

	if err := group.Validate(); err != nil {
		h.logger.Error("group validation failed", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка валидации группы.",
		})
		return
	}

	if err := h.groupRepo.CreateGroup(ctx, group); err != nil {
		h.logger.Error("failed to create group", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка при создании группы.",
		})
		return
	}

	// Log the action
	h.logAdminAction(userID, "create_group", group.ID, fmt.Sprintf("Created group: %s", groupName))

	// Generate deep-link
	deepLink := h.deepLinkService.GenerateGroupInviteLink(group.ID)

	// Send success message
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: fmt.Sprintf("✅ Группа создана!\n\n"+
			"📋 Название: %s\n"+
			"🆔 ID: %d\n"+
			"🔗 Ссылка для приглашения:\n%s",
			groupName, group.ID, deepLink),
	})
	if err != nil {
		h.logger.Error("failed to send success message", "error", err)
	}

	h.logger.Info("group created", "group_id", group.ID, "name", groupName, "created_by", userID)
}

// HandleCallback handles callback queries (button clicks)
func (h *BotHandler) HandleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	userID := callback.From.ID
	data := callback.Data

	// Check if this is an FSM callback (event_type selection or confirmation)
	if strings.HasPrefix(data, "event_type:") || strings.HasPrefix(data, "confirm:") {
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

	// Answer callback query to remove loading state (for non-FSM callbacks)
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Handle resolve event callbacks
	if strings.HasPrefix(data, "resolve:") {
		h.handleResolveCallback(ctx, b, callback, userID, data)
		return
	}
}

// HandleResolveEvent handles the /resolve_event command
func (h *BotHandler) HandleResolveEvent(ctx context.Context, b *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID

	// Get all active events
	events, err := h.eventManager.GetActiveEvents(ctx)
	if err != nil {
		h.logger.Error("failed to get active events", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Ошибка при получении списка событий.",
		})
		return
	}

	if len(events) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "📋 Нет активных событий для завершения.",
		})
		return
	}

	// Filter events that user can manage
	var manageableEvents []*domain.Event
	for _, event := range events {
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
			ChatID: update.Message.Chat.ID,
			Text:   "❌ У вас нет прав для управления активными событиями.",
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

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "🏁 ЗАВЕРШЕНИЕ СОБЫТИЯ\n\nВыберите событие для завершения:",
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send resolve event selection", "error", err)
	}
}

func (h *BotHandler) handleResolveCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Parse event ID
	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		return
	}

	// Check if this is selecting the correct option
	if len(parts) == 4 && parts[1] == "option" {
		eventID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			h.logger.Error("failed to parse event ID", "error", err)
			return
		}

		optionIndex, err := strconv.Atoi(parts[3])
		if err != nil {
			h.logger.Error("failed to parse option index", "error", err)
			return
		}

		// Check if user can manage this event
		canManage, err := h.eventPermissionValidator.CanManageEvent(ctx, userID, eventID, h.config.AdminUserIDs)
		if err != nil {
			h.logger.Error("failed to check event management permission", "user_id", userID, "event_id", eventID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   "❌ Ошибка при проверке прав доступа.",
			})
			return
		}

		if !canManage {
			h.logger.Warn("unauthorized event resolution attempt", "user_id", userID, "event_id", eventID)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   "❌ У вас нет прав для управления этим событием.",
			})
			return
		}

		// Resolve the event
		if err := h.eventManager.ResolveEvent(ctx, eventID, optionIndex); err != nil {
			h.logger.Error("failed to resolve event", "event_id", eventID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   "❌ Ошибка при завершении события.",
			})
			return
		}

		// Get the event to show details
		event, err := h.eventManager.GetEvent(ctx, eventID)
		if err != nil {
			h.logger.Error("failed to get event", "event_id", eventID, "error", err)
			return
		}

		// Log the action (admin or creator)
		if h.isAdmin(userID) {
			h.logAdminAction(userID, "resolve_event", eventID, fmt.Sprintf("Correct option: %d (%s)", optionIndex, event.Options[optionIndex]))
		} else {
			h.logger.Info("creator resolved event", "user_id", userID, "event_id", eventID, "correct_option", optionIndex)
		}

		// Calculate scores
		if err := h.ratingCalculator.CalculateScores(ctx, eventID, optionIndex); err != nil {
			h.logger.Error("failed to calculate scores", "event_id", eventID, "error", err)
		}

		// Check and award achievements for all participants
		predictions, err := h.predictionRepo.GetPredictionsByEvent(ctx, eventID)
		if err == nil {
			for _, pred := range predictions {
				achievements, err := h.achievementTracker.CheckAndAwardAchievements(ctx, pred.UserID)
				if err != nil {
					h.logger.Error("failed to check achievements", "user_id", pred.UserID, "error", err)
					continue
				}

				// Send achievement notifications
				for _, ach := range achievements {
					h.sendAchievementNotification(ctx, b, pred.UserID, ach)
				}
			}
		}

		// Stop the poll
		if event.PollID != "" {
			_, _ = b.StopPoll(ctx, &bot.StopPollParams{
				ChatID:    h.config.GroupID,
				MessageID: 0, // We don't have message ID, poll will just be closed
			})
		}

		// Publish results to group
		h.publishEventResults(ctx, b, event, optionIndex, userID)

		// Send confirmation to user
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   fmt.Sprintf("✅ Событие завершено!\n\nПравильный ответ: %s", event.Options[optionIndex]),
		})

		return
	}

	// This is selecting the event, now show options
	eventIDStr := parts[1]
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil {
		h.logger.Error("failed to parse event ID", "error", err)
		return
	}

	// Check if user can manage this event
	canManage, err := h.eventPermissionValidator.CanManageEvent(ctx, userID, eventID, h.config.AdminUserIDs)
	if err != nil {
		h.logger.Error("failed to check event management permission", "user_id", userID, "event_id", eventID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "❌ Ошибка при проверке прав доступа.",
		})
		return
	}

	if !canManage {
		h.logger.Warn("unauthorized event management attempt", "user_id", userID, "event_id", eventID)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "❌ У вас нет прав для управления этим событием.",
		})
		return
	}

	// Get the event
	event, err := h.eventManager.GetEvent(ctx, eventID)
	if err != nil {
		h.logger.Error("failed to get event", "event_id", eventID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "❌ Ошибка при получении события.",
		})
		return
	}

	// Build inline keyboard with options
	var buttons [][]models.InlineKeyboardButton
	for i, option := range event.Options {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         option,
				CallbackData: fmt.Sprintf("resolve:option:%d:%d", eventID, i),
			},
		})
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      callback.Message.Message.Chat.ID,
		Text:        fmt.Sprintf("🎯 ВЫБОР ПРАВИЛЬНОГО ОТВЕТА\n\n▸ Событие: %s\n\nВыберите правильный ответ:", event.Question),
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send option selection", "error", err)
	}
}

func (h *BotHandler) publishEventResults(ctx context.Context, b *bot.Bot, event *domain.Event, correctOption int, resolverID int64) {
	// Get all predictions
	predictions, err := h.predictionRepo.GetPredictionsByEvent(ctx, event.ID)
	if err != nil {
		h.logger.Error("failed to get predictions for results", "event_id", event.ID, "error", err)
		return
	}

	// Count correct predictions
	correctCount := 0
	for _, pred := range predictions {
		if pred.Option == correctOption {
			correctCount++
		}
	}

	// Get top 5 participants by points earned (simplified - just show top 5 overall)
	topRatings, err := h.ratingCalculator.GetTopRatings(ctx, 5)
	if err != nil {
		h.logger.Error("failed to get top ratings", "error", err)
		topRatings = []*domain.Rating{}
	}

	// Get resolver display name
	resolverDisplayName := h.getUserDisplayName(ctx, resolverID)

	// Determine if resolver is admin or creator
	isAdmin := h.isAdmin(resolverID)
	isCreator := event.CreatedBy == resolverID

	// Build results message
	var sb strings.Builder
	sb.WriteString("🏁 СОБЫТИЕ ЗАВЕРШЕНО!\n\n")
	sb.WriteString(fmt.Sprintf("❓ Вопрос:\n%s\n\n", event.Question))
	sb.WriteString(fmt.Sprintf("✅ Правильный ответ:\n%s\n\n", event.Options[correctOption]))
	sb.WriteString(fmt.Sprintf("📊 Угадали: %d из %d участников\n", correctCount, len(predictions)))

	// Add resolver information with role distinction
	if isAdmin && !isCreator {
		sb.WriteString(fmt.Sprintf("\n👤 Завершил (администратор): %s\n", resolverDisplayName))
	} else {
		sb.WriteString(fmt.Sprintf("\n👤 Завершил (создатель): %s\n", resolverDisplayName))
	}

	if len(topRatings) > 0 {
		sb.WriteString("\n🏆 ТОП-5 УЧАСТНИКОВ\n\n")
		medals := []string{"🥇", "🥈", "🥉", "4.", "5."}
		for i, rating := range topRatings {
			sb.WriteString(fmt.Sprintf("%s %d очков\n", medals[i], rating.Score))
		}
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: h.config.GroupID,
		Text:   sb.String(),
	})
	if err != nil {
		h.logger.Error("failed to send results to group", "error", err)
	}
}

func (h *BotHandler) sendAchievementNotification(ctx context.Context, b *bot.Bot, userID int64, achievement *domain.Achievement) {
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

	// Send to user
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: userID,
		Text:   fmt.Sprintf("🎉 Поздравляем! Вы получили ачивку:\n\n%s", name),
	})
	if err != nil {
		h.logger.Error("failed to send achievement notification to user", "user_id", userID, "error", err)
	}

	// Get user display name for group announcement
	displayName := h.getUserDisplayName(ctx, userID)

	// Announce in group with username
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: h.config.GroupID,
		Text:   fmt.Sprintf("🎉 %s получил ачивку: %s!", displayName, name),
	})
	if err != nil {
		h.logger.Error("failed to send achievement announcement to group", "error", err)
	}
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
		Text:   "ℹ️ Редактирование событий временно недоступно. Пожалуйста, создайте новое событие с /create_event",
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

	// Prompt for group name
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "🏗️ СОЗДАНИЕ ГРУППЫ\n\nВведите название новой группы:",
	})
	if err != nil {
		h.logger.Error("failed to send create group prompt", "error", err)
		return
	}

	// Store state for this admin to expect group name input
	h.createGroupState[userID] = true
	h.logger.Info("create_group command initiated", "admin_user_id", userID)
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
			Text:   "❌ Ошибка при получении списка групп.",
		})
		return
	}

	if len(groups) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "📋 Нет созданных групп.",
		})
		return
	}

	// Build groups list message with deep-links
	var sb strings.Builder
	sb.WriteString("📋 СПИСОК ГРУПП\n")
	sb.WriteString("════════════════════\n\n")

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
		deepLink := h.deepLinkService.GenerateGroupInviteLink(group.ID)

		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, group.Name))
		sb.WriteString(fmt.Sprintf("   👥 Участников: %d\n", activeCount))
		sb.WriteString(fmt.Sprintf("   🔗 Ссылка: %s\n", deepLink))
		sb.WriteString(fmt.Sprintf("   🆔 ID: %d\n\n", group.ID))
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   sb.String(),
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
			Text:   "❌ Ошибка при получении списка групп.",
		})
		return
	}

	if len(groups) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "📋 Нет созданных групп.",
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
		Text:        "👥 УЧАСТНИКИ ГРУППЫ\n\nВыберите группу:",
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
			Text:   "❌ Ошибка при получении списка групп.",
		})
		return
	}

	if len(groups) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "📋 Нет созданных групп.",
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
		Text:        "🚫 УДАЛЕНИЕ УЧАСТНИКА\n\nВыберите группу:",
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
			Text:            "❌ У вас нет прав для выполнения этой команды.",
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
			Text:   "❌ Ошибка при получении группы.",
		})
		return
	}

	if group == nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "❌ Группа не найдена.",
		})
		return
	}

	// Get group members
	members, err := h.groupMembershipRepo.GetGroupMembers(ctx, groupID)
	if err != nil {
		h.logger.Error("failed to get group members", "group_id", groupID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "❌ Ошибка при получении участников группы.",
		})
		return
	}

	if len(members) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   fmt.Sprintf("📋 В группе \"%s\" пока нет участников.", group.Name),
		})
		return
	}

	// Build members list message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("👥 УЧАСТНИКИ ГРУППЫ \"%s\"\n", group.Name))
	sb.WriteString("════════════════════\n\n")

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

		// Get achievements count (note: achievements are currently not group-scoped in the tracker)
		achievements, err := h.achievementTracker.GetUserAchievements(ctx, member.UserID)
		if err != nil {
			h.logger.Error("failed to get user achievements", "user_id", member.UserID, "error", err)
			achievements = []*domain.Achievement{}
		}

		// Get display name
		displayName := h.getUserDisplayName(ctx, member.UserID)

		// Status indicator
		statusIcon := "✅"
		if member.Status == domain.MembershipStatusRemoved {
			statusIcon = "🚫"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, statusIcon, displayName))
		sb.WriteString(fmt.Sprintf("   💰 Очки: %d\n", rating.Score))
		sb.WriteString(fmt.Sprintf("   🏆 Ачивки: %d\n", len(achievements)))
		sb.WriteString(fmt.Sprintf("   📅 Присоединился: %s\n\n", member.JoinedAt.Format("02.01.2006")))
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

// handleRemoveMemberCallback handles the callback for removing a member
func (h *BotHandler) handleRemoveMemberCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "❌ У вас нет прав для выполнения этой команды.",
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
				Text:   "❌ Ошибка при получении группы.",
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   "❌ Группа не найдена.",
			})
			return
		}

		// Get active members
		members, err := h.groupMembershipRepo.GetGroupMembers(ctx, groupID)
		if err != nil {
			h.logger.Error("failed to get group members", "group_id", groupID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   "❌ Ошибка при получении участников группы.",
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
				Text:   fmt.Sprintf("📋 В группе \"%s\" нет активных участников.", group.Name),
			})
			return
		}

		// Build inline keyboard with members
		var buttons [][]models.InlineKeyboardButton
		for _, member := range activeMembers {
			displayName := h.getUserDisplayName(ctx, member.UserID)
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
			Text:        fmt.Sprintf("🚫 УДАЛЕНИЕ УЧАСТНИКА ИЗ \"%s\"\n\nВыберите участника:", group.Name),
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
				Text:   "❌ Ошибка при получении группы.",
			})
			return
		}

		if group == nil {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   "❌ Группа не найдена.",
			})
			return
		}

		// Update membership status to removed
		err = h.groupMembershipRepo.UpdateMembershipStatus(ctx, groupID, memberUserID, domain.MembershipStatusRemoved)
		if err != nil {
			h.logger.Error("failed to update membership status", "group_id", groupID, "user_id", memberUserID, "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: callback.Message.Message.Chat.ID,
				Text:   "❌ Ошибка при удалении участника.",
			})
			return
		}

		// Log the action
		h.logAdminAction(userID, "remove_member", groupID, fmt.Sprintf("Removed user %d from group %s", memberUserID, group.Name))

		// Get display name
		displayName := h.getUserDisplayName(ctx, memberUserID)

		// Send confirmation
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   fmt.Sprintf("✅ Участник %s удален из группы \"%s\".", displayName, group.Name),
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
