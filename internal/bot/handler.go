package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"telegram-prediction-bot/internal/config"
	"telegram-prediction-bot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// BotHandler handles all Telegram bot interactions
type BotHandler struct {
	bot                *bot.Bot
	eventManager       *domain.EventManager
	ratingCalculator   *domain.RatingCalculator
	achievementTracker *domain.AchievementTracker
	predictionRepo     domain.PredictionRepository
	config             *config.Config
	logger             domain.Logger
	conversationStates map[int64]*ConversationState
}

// ConversationState tracks multi-step conversation state for event creation/editing
type ConversationState struct {
	Step         string
	EventData    *domain.Event
	EventID      int64
	Options      []string
	LastUpdateAt time.Time
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
) *BotHandler {
	return &BotHandler{
		bot:                b,
		eventManager:       eventManager,
		ratingCalculator:   ratingCalculator,
		achievementTracker: achievementTracker,
		predictionRepo:     predictionRepo,
		config:             cfg,
		logger:             logger,
		conversationStates: make(map[int64]*ConversationState),
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

═══════════════════════════
📋 ДОСТУПНЫЕ КОМАНДЫ
═══════════════════════════

👤 Для всех пользователей:
  /help — Показать эту справку
  /rating — Топ-10 участников по очкам
  /my — Ваша статистика и ачивки
  /events — Список активных событий

👑 Для администраторов:
  /create_event — Создать новое событие
  /resolve_event — Завершить событие и подвести итоги
  /edit_event — Редактировать событие (только без голосов)

═══════════════════════════
💰 ПРАВИЛА НАЧИСЛЕНИЯ ОЧКОВ
═══════════════════════════

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

═══════════════════════════
🏆 АЧИВКИ
═══════════════════════════

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

═══════════════════════════
🎲 ТИПЫ СОБЫТИЙ
═══════════════════════════

1️⃣ Бинарное
   → Да/Нет вопросы

2️⃣ Множественный выбор
   → 2-6 вариантов ответа

3️⃣ Вероятностное
   → Диапазоны вероятности
   (0-25%, 25-50%, 50-75%, 75-100%)

═══════════════════════════

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
	sb.WriteString("🏆 ТОП-10 УЧАСТНИКОВ\n")
	sb.WriteString("═══════════════════════════\n\n")

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

		sb.WriteString(fmt.Sprintf("%s%d очков\n", medal, rating.Score))
		sb.WriteString(fmt.Sprintf("   📊 Точность: %.1f%%  |  🔥 Серия: %d\n", accuracy, rating.Streak))
		sb.WriteString(fmt.Sprintf("   ✅ %d  ❌ %d\n\n", rating.CorrectCount, rating.WrongCount))
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
	sb.WriteString("═══════════════════════════\n\n")

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
	sb.WriteString("═══════════════════════════\n")
	if len(achievements) > 0 {
		sb.WriteString("🏆 ВАШИ АЧИВКИ\n")
		sb.WriteString("═══════════════════════════\n\n")
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
		sb.WriteString("═══════════════════════════\n\n")
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
	sb.WriteString("═══════════════════════════\n\n")

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
}

// HandleCreateEvent handles the /create_event command (multi-step conversation)
func (h *BotHandler) HandleCreateEvent(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check admin authorization
	if !h.requireAdmin(ctx, update) {
		return
	}

	userID := update.Message.From.ID

	// Initialize conversation state
	h.conversationStates[userID] = &ConversationState{
		Step:         "ask_question",
		EventData:    &domain.Event{CreatedBy: userID},
		LastUpdateAt: time.Now(),
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "📝 СОЗДАНИЕ НОВОГО СОБЫТИЯ\n═══════════════════════════\n\nВведите вопрос для прогноза:",
	})
	if err != nil {
		h.logger.Error("failed to send create event message", "error", err)
	}
}

// HandleMessage handles regular text messages (for conversation flows)
func (h *BotHandler) HandleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	userID := update.Message.From.ID
	state, exists := h.conversationStates[userID]

	if !exists {
		return // No active conversation
	}

	// Check if conversation is stale (older than 10 minutes)
	if time.Since(state.LastUpdateAt) > 10*time.Minute {
		delete(h.conversationStates, userID)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "⏱ Время сессии истекло. Начните заново с /create_event",
		})
		return
	}

	state.LastUpdateAt = time.Now()

	switch state.Step {
	case "ask_question":
		h.handleQuestionInput(ctx, b, update, state)
	case "ask_options":
		h.handleOptionsInput(ctx, b, update, state)
	case "ask_deadline":
		h.handleDeadlineInput(ctx, b, update, state)
	case "ask_event_type":
		h.handleEventTypeInput(ctx, b, update, state)
	case "edit_ask_question":
		h.handleEditQuestionInput(ctx, b, update, state)
	case "edit_ask_options":
		h.handleEditOptionsInput(ctx, b, update, state)
	}
}

func (h *BotHandler) handleQuestionInput(ctx context.Context, b *bot.Bot, update *models.Update, state *ConversationState) {
	question := strings.TrimSpace(update.Message.Text)
	if question == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Вопрос не может быть пустым. Попробуйте снова:",
		})
		return
	}

	state.EventData.Question = question
	state.Step = "ask_event_type"

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

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "Выберите тип события:",
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send event type selection", "error", err)
	}
}

func (h *BotHandler) handleEventTypeInput(ctx context.Context, b *bot.Bot, update *models.Update, state *ConversationState) {
	// This is handled by callback query, not text message
}

func (h *BotHandler) handleOptionsInput(ctx context.Context, b *bot.Bot, update *models.Update, state *ConversationState) {
	optionsText := strings.TrimSpace(update.Message.Text)
	if optionsText == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Варианты не могут быть пустыми. Попробуйте снова:",
		})
		return
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

	// Validate option count based on event type
	minOptions := 2
	maxOptions := 6
	switch state.EventData.EventType {
	case domain.EventTypeBinary:
		minOptions = 2
		maxOptions = 2
	case domain.EventTypeProbability:
		minOptions = 4
		maxOptions = 4
	}

	if len(cleanOptions) < minOptions || len(cleanOptions) > maxOptions {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("❌ Для этого типа события нужно %d-%d вариантов. Попробуйте снова:", minOptions, maxOptions),
		})
		return
	}

	state.EventData.Options = cleanOptions
	state.Step = "ask_deadline"

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "📅 Введите дедлайн в формате:\n   ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: 25.12.2024 18:00",
	})
	if err != nil {
		h.logger.Error("failed to send deadline request", "error", err)
	}
}

func (h *BotHandler) handleDeadlineInput(ctx context.Context, b *bot.Bot, update *models.Update, state *ConversationState) {
	deadlineText := strings.TrimSpace(update.Message.Text)

	// Parse deadline in the configured timezone
	deadline, err := time.ParseInLocation("02.01.2006 15:04", deadlineText, h.config.Timezone)
	if err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Неверный формат даты. Используйте: ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: 25.12.2024 18:00",
		})
		return
	}

	// Check if deadline is in the future
	if deadline.Before(time.Now()) {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Дедлайн должен быть в будущем. Попробуйте снова:",
		})
		return
	}

	state.EventData.Deadline = deadline
	state.EventData.CreatedAt = time.Now()
	state.EventData.Status = domain.EventStatusActive

	// Create the event
	if err := h.eventManager.CreateEvent(ctx, state.EventData); err != nil {
		h.logger.Error("failed to create event", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Ошибка при создании события.",
		})
		delete(h.conversationStates, update.Message.From.ID)
		return
	}

	// Log admin action
	h.logAdminAction(update.Message.From.ID, "create_event", state.EventData.ID, fmt.Sprintf("Question: %s", state.EventData.Question))

	// Send the poll to the group
	// Convert options to InputPollOption
	pollOptions := make([]models.InputPollOption, len(state.EventData.Options))
	for i, opt := range state.EventData.Options {
		pollOptions[i] = models.InputPollOption{Text: opt}
	}

	isAnonymous := false
	pollMsg, err := b.SendPoll(ctx, &bot.SendPollParams{
		ChatID:                h.config.GroupID,
		Question:              state.EventData.Question,
		Options:               pollOptions,
		IsAnonymous:           &isAnonymous,
		AllowsMultipleAnswers: false,
	})
	if err != nil {
		h.logger.Error("failed to send poll", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Ошибка при публикации опроса.",
		})
		delete(h.conversationStates, update.Message.From.ID)
		return
	}

	// Update event with poll ID
	state.EventData.PollID = pollMsg.Poll.ID
	if err := h.eventManager.UpdateEvent(ctx, state.EventData); err != nil {
		h.logger.Error("failed to update event with poll ID", "error", err)
	}

	// Send confirmation
	localDeadline := state.EventData.Deadline.In(h.config.Timezone)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("✅ СОБЫТИЕ СОЗДАНО!\n\n▸ ID: %d\n▸ Вопрос: %s\n▸ Дедлайн: %s", state.EventData.ID, state.EventData.Question, localDeadline.Format("02.01.2006 15:04")),
	})

	// Clean up conversation state
	delete(h.conversationStates, update.Message.From.ID)
}

// HandleCallback handles callback queries (button clicks)
func (h *BotHandler) HandleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	userID := callback.From.ID
	data := callback.Data

	// Answer callback query to remove loading state
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	// Handle event type selection
	if strings.HasPrefix(data, "event_type:") {
		h.handleEventTypeCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle resolve event callbacks
	if strings.HasPrefix(data, "resolve:") {
		h.handleResolveCallback(ctx, b, callback, userID, data)
		return
	}

	// Handle edit event callbacks
	if strings.HasPrefix(data, "edit:") {
		h.handleEditCallback(ctx, b, callback, userID, data)
		return
	}
}

func (h *BotHandler) handleEventTypeCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	state, exists := h.conversationStates[userID]
	if !exists || state.Step != "ask_event_type" {
		return
	}

	eventType := strings.TrimPrefix(data, "event_type:")

	switch eventType {
	case "binary":
		state.EventData.EventType = domain.EventTypeBinary
		state.EventData.Options = []string{"Да", "Нет"}
		state.Step = "ask_deadline"

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "✅ Выбран бинарный тип (Да/Нет)\n\n📅 Введите дедлайн в формате:\n   ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: 25.12.2024 18:00",
		})

	case "multi":
		state.EventData.EventType = domain.EventTypeMultiOption
		state.Step = "ask_options"

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "✅ Выбран множественный выбор\n\nВведите варианты ответа (2-6 штук), каждый с новой строки:",
		})

	case "probability":
		state.EventData.EventType = domain.EventTypeProbability
		state.EventData.Options = []string{"0-25%", "25-50%", "50-75%", "75-100%"}
		state.Step = "ask_deadline"

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "✅ Выбран вероятностный тип\n\n📅 Введите дедлайн в формате:\n   ДД.ММ.ГГГГ ЧЧ:ММ\n\nНапример: 25.12.2024 18:00",
		})
	}

	state.LastUpdateAt = time.Now()
}

// HandleResolveEvent handles the /resolve_event command
func (h *BotHandler) HandleResolveEvent(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check admin authorization
	if !h.requireAdmin(ctx, update) {
		return
	}

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

	// Build inline keyboard with events
	var buttons [][]models.InlineKeyboardButton
	for _, event := range events {
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
		Text:        "🏁 ЗАВЕРШЕНИЕ СОБЫТИЯ\n═══════════════════════════\n\nВыберите событие для завершения:",
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send resolve event selection", "error", err)
	}
}

func (h *BotHandler) handleResolveCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		return
	}

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

		// Log admin action
		h.logAdminAction(userID, "resolve_event", eventID, fmt.Sprintf("Correct option: %d (%s)", optionIndex, event.Options[optionIndex]))

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
		h.publishEventResults(ctx, b, event, optionIndex)

		// Send confirmation to admin
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
		Text:        fmt.Sprintf("🎯 ВЫБОР ПРАВИЛЬНОГО ОТВЕТА\n═══════════════════════════\n\n▸ Событие: %s\n\nВыберите правильный ответ:", event.Question),
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send option selection", "error", err)
	}
}

func (h *BotHandler) publishEventResults(ctx context.Context, b *bot.Bot, event *domain.Event, correctOption int) {
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

	// Build results message
	var sb strings.Builder
	sb.WriteString("🏁 СОБЫТИЕ ЗАВЕРШЕНО!\n")
	sb.WriteString("═══════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("❓ Вопрос:\n%s\n\n", event.Question))
	sb.WriteString(fmt.Sprintf("✅ Правильный ответ:\n%s\n\n", event.Options[correctOption]))
	sb.WriteString(fmt.Sprintf("📊 Угадали: %d из %d участников\n", correctCount, len(predictions)))

	if len(topRatings) > 0 {
		sb.WriteString("\n═══════════════════════════\n")
		sb.WriteString("🏆 ТОП-5 УЧАСТНИКОВ\n")
		sb.WriteString("═══════════════════════════\n\n")
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
		domain.AchievementSharpshooter:  "🎯 Меткий стрелок",
		domain.AchievementProphet:       "🔮 Провидец",
		domain.AchievementRiskTaker:     "🎲 Риск-мейкер",
		domain.AchievementWeeklyAnalyst: "📊 Аналитик недели",
		domain.AchievementVeteran:       "🏆 Старожил",
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

	// Announce in group
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: h.config.GroupID,
		Text:   fmt.Sprintf("🎉 Участник получил ачивку: %s!", name),
	})
	if err != nil {
		h.logger.Error("failed to send achievement announcement to group", "error", err)
	}
}

// HandleEditEvent handles the /edit_event command
func (h *BotHandler) HandleEditEvent(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Check admin authorization
	if !h.requireAdmin(ctx, update) {
		return
	}

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
			Text:   "📋 Нет активных событий для редактирования.",
		})
		return
	}

	// Build inline keyboard with events
	var buttons [][]models.InlineKeyboardButton
	for _, event := range events {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         fmt.Sprintf("%s (ID: %d)", event.Question, event.ID),
				CallbackData: fmt.Sprintf("edit:%d", event.ID),
			},
		})
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "✏️ РЕДАКТИРОВАНИЕ СОБЫТИЯ\n═══════════════════════════\n\nВыберите событие для редактирования:",
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Error("failed to send edit event selection", "error", err)
	}
}

func (h *BotHandler) handleEditCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, userID int64, data string) {
	// Check admin authorization
	if !h.isAdmin(userID) {
		return
	}

	// Parse event ID
	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		return
	}

	eventIDStr := parts[1]
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil {
		h.logger.Error("failed to parse event ID", "error", err)
		return
	}

	// Check if event can be edited
	canEdit, err := h.eventManager.CanEditEvent(ctx, eventID)
	if err != nil {
		h.logger.Error("failed to check if event can be edited", "event_id", eventID, "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "❌ Ошибка при проверке возможности редактирования.",
		})
		return
	}

	if !canEdit {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: callback.Message.Message.Chat.ID,
			Text:   "❌ Это событие нельзя редактировать, так как уже есть голоса.",
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

	// Initialize edit conversation state
	h.conversationStates[userID] = &ConversationState{
		Step:         "edit_ask_question",
		EventData:    event,
		EventID:      eventID,
		LastUpdateAt: time.Now(),
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: callback.Message.Message.Chat.ID,
		Text:   fmt.Sprintf("✏️ РЕДАКТИРОВАНИЕ СОБЫТИЯ\n═══════════════════════════\n\n▸ Текущий вопрос:\n%s\n\nВведите новый вопрос или отправьте /cancel для отмены:", event.Question),
	})
	if err != nil {
		h.logger.Error("failed to send edit question prompt", "error", err)
	}
}

func (h *BotHandler) handleEditQuestionInput(ctx context.Context, b *bot.Bot, update *models.Update, state *ConversationState) {
	question := strings.TrimSpace(update.Message.Text)

	if question == "/cancel" {
		delete(h.conversationStates, update.Message.From.ID)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Редактирование отменено.",
		})
		return
	}

	if question == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Вопрос не может быть пустым. Попробуйте снова:",
		})
		return
	}

	state.EventData.Question = question

	// If it's a binary or probability event, skip options (they're fixed)
	if state.EventData.EventType == domain.EventTypeBinary || state.EventData.EventType == domain.EventTypeProbability {
		// Save the event
		if err := h.eventManager.UpdateEvent(ctx, state.EventData); err != nil {
			h.logger.Error("failed to update event", "error", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "❌ Ошибка при обновлении события.",
			})
			delete(h.conversationStates, update.Message.From.ID)
			return
		}

		// Log admin action
		h.logAdminAction(update.Message.From.ID, "edit_event", state.EventData.ID, fmt.Sprintf("Updated question to: %s", question))

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "✅ Событие обновлено!",
		})
		delete(h.conversationStates, update.Message.From.ID)
		return
	}

	// For multi-option events, ask for new options
	state.Step = "edit_ask_options"
	optionsText := strings.Join(state.EventData.Options, "\n")
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("▸ Текущие варианты:\n%s\n\nВведите новые варианты (каждый с новой строки) или отправьте /cancel:", optionsText),
	})
}

func (h *BotHandler) handleEditOptionsInput(ctx context.Context, b *bot.Bot, update *models.Update, state *ConversationState) {
	optionsText := strings.TrimSpace(update.Message.Text)

	if optionsText == "/cancel" {
		delete(h.conversationStates, update.Message.From.ID)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Редактирование отменено.",
		})
		return
	}

	if optionsText == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Варианты не могут быть пустыми. Попробуйте снова:",
		})
		return
	}

	// Parse options
	options := strings.Split(optionsText, "\n")
	var cleanOptions []string
	for _, opt := range options {
		opt = strings.TrimSpace(opt)
		if opt != "" {
			cleanOptions = append(cleanOptions, opt)
		}
	}

	if len(cleanOptions) < 2 || len(cleanOptions) > 6 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Нужно 2-6 вариантов. Попробуйте снова:",
		})
		return
	}

	state.EventData.Options = cleanOptions

	// Save the event
	if err := h.eventManager.UpdateEvent(ctx, state.EventData); err != nil {
		h.logger.Error("failed to update event", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Ошибка при обновлении события.",
		})
		delete(h.conversationStates, update.Message.From.ID)
		return
	}

	// Log admin action
	h.logAdminAction(update.Message.From.ID, "edit_event", state.EventData.ID, fmt.Sprintf("Updated options to: %v", cleanOptions))

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "✅ Событие обновлено!",
	})
	delete(h.conversationStates, update.Message.From.ID)
}
