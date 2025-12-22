package domain

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/locale"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestAchievementNotification(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("achievement notification sends to user and group", prop.ForAll(
		func(userID int64, achievementCode string) bool {
			// Map string to achievement code
			var code AchievementCode
			switch achievementCode {
			case "sharpshooter":
				code = AchievementSharpshooter
			case "prophet":
				code = AchievementProphet
			case "risk_taker":
				code = AchievementRiskTaker
			case "weekly_analyst":
				code = AchievementWeeklyAnalyst
			case "veteran":
				code = AchievementVeteran
			default:
				return true // Skip invalid codes
			}

			// Create mock dependencies
			mockBot := &MockNotificationBot{
				sentMessages: make([]MockNotificationMessage, 0),
			}
			mockEventRepo := &MockEventRepo{}
			mockPredictionRepo := &MockPredictionRepo{}
			mockRatingRepo := &MockRatingRepo{}
			mockReminderRepo := &MockReminderRepo{}
			mockLogger := &MockLogger{}

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				mockLogger,
				&MockLocalizer{},
			)

			// Create achievement
			achievement := &Achievement{
				ID:        1,
				UserID:    userID,
				Code:      code,
				Timestamp: time.Now(),
			}

			// Send achievement notification
			ctx := context.Background()
			err := ns.SendAchievementNotification(ctx, userID, achievement)
			if err != nil {
				return false
			}

			// Verify that exactly 2 messages were sent (one to user, one to group)
			if len(mockBot.sentMessages) != 2 {
				return false
			}

			// Verify first message was sent to user
			if mockBot.sentMessages[0].ChatID != userID {
				return false
			}

			// Verify both messages contain achievement information
			userMsg := mockBot.sentMessages[0].Text
			groupMsg := mockBot.sentMessages[1].Text

			if userMsg == "" || groupMsg == "" {
				return false
			}

			// Both messages should contain achievement-related text
			hasUserNotification := strings.Contains(userMsg, "ачивку") || strings.Contains(userMsg, "Поздравляем")
			hasGroupAnnouncement := strings.Contains(groupMsg, "ачивку") || strings.Contains(groupMsg, "получил")

			return hasUserNotification && hasGroupAnnouncement
		},
		gen.Int64Range(1, 1000000),
		gen.OneConstOf("sharpshooter", "prophet", "risk_taker", "weekly_analyst", "veteran"),
	))

	properties.TestingRun(t)
}

// Mock implementations for testing

type MockNotificationBot struct {
	sentMessages []MockNotificationMessage
}

type MockNotificationMessage struct {
	ChatID int64
	Text   string
}

func (m *MockNotificationBot) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	msg := MockNotificationMessage{
		ChatID: params.ChatID.(int64),
		Text:   params.Text,
	}
	m.sentMessages = append(m.sentMessages, msg)
	return &models.Message{}, nil
}

type MockEventRepo struct{}

func (m *MockEventRepo) CreateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *MockEventRepo) GetEvent(ctx context.Context, eventID int64) (*Event, error) {
	return &Event{}, nil
}

func (m *MockEventRepo) GetEventByPollID(ctx context.Context, pollID string) (*Event, error) {
	return &Event{}, nil
}

func (m *MockEventRepo) GetActiveEvents(ctx context.Context, groupID int64) ([]*Event, error) {
	return []*Event{}, nil
}

func (m *MockEventRepo) UpdateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *MockEventRepo) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	return nil
}

func (m *MockEventRepo) GetResolvedEvents(ctx context.Context) ([]*Event, error) {
	return []*Event{}, nil
}

func (m *MockEventRepo) GetUserCreatedEventsCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	return 0, nil
}

func (m *MockEventRepo) GetEventsByDeadlineRange(ctx context.Context, start, end time.Time) ([]*Event, error) {
	return []*Event{}, nil
}

type MockPredictionRepo struct{}

func (m *MockPredictionRepo) SavePrediction(ctx context.Context, prediction *Prediction) error {
	return nil
}

func (m *MockPredictionRepo) UpdatePrediction(ctx context.Context, prediction *Prediction) error {
	return nil
}

func (m *MockPredictionRepo) GetPredictionsByEvent(ctx context.Context, eventID int64) ([]*Prediction, error) {
	return []*Prediction{}, nil
}

func (m *MockPredictionRepo) GetPredictionByUserAndEvent(ctx context.Context, userID, eventID int64) (*Prediction, error) {
	return nil, nil
}

func (m *MockPredictionRepo) GetUserPredictions(ctx context.Context, userID int64) ([]*Prediction, error) {
	return []*Prediction{}, nil
}

func (m *MockPredictionRepo) GetUserCompletedEventCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	return 0, nil
}

type MockRatingRepo struct{}

func (m *MockRatingRepo) GetRating(ctx context.Context, userID int64, groupID int64) (*Rating, error) {
	return &Rating{}, nil
}

func (m *MockRatingRepo) UpdateRating(ctx context.Context, rating *Rating) error {
	return nil
}

func (m *MockRatingRepo) GetTopRatings(ctx context.Context, groupID int64, limit int) ([]*Rating, error) {
	return []*Rating{}, nil
}

func (m *MockRatingRepo) UpdateStreak(ctx context.Context, userID int64, groupID int64, streak int) error {
	return nil
}

type MockLogger struct{}

func (m *MockLogger) Info(msg string, args ...interface{}) {}

func (m *MockLogger) Error(msg string, args ...interface{}) {}

func (m *MockLogger) Debug(msg string, args ...interface{}) {}

func (m *MockLogger) Warn(msg string, args ...interface{}) {}

type MockLocalizer struct {
	localizeCallCount         int
	localizeWithTemplateCount int
	lastLocalizeID            string
	lastTemplateID            string
}

func (m *MockLocalizer) GetLocale() string {
	return "ru"
}

func (m *MockLocalizer) MustLocalize(id string) string {
	m.localizeCallCount++
	m.lastLocalizeID = id

	// Return realistic Russian translations for testing
	translations := map[string]string{
		locale.NotificationNewEventTitle:    "🆕 НОВОЕ СОБЫТИЕ ДЛЯ ПРОГНОЗА!",
		locale.NotificationNewEventOptions:  "📊 Варианты:",
		locale.NotificationNewEventCTA:      "Голосуйте в опросе выше! 🗳",
		locale.EventTypeBinaryLabel:         "Бинарное",
		locale.EventTypeMultiOptionLabel:    "Множественный выбор",
		locale.EventTypeProbabilityLabel:    "Вероятностное",
		locale.EventTypeBinaryIcon:          "1️⃣",
		locale.EventTypeMultiOptionIcon:     "2️⃣",
		locale.EventTypeProbabilityIcon:     "3️⃣",
		locale.DeadlineExpired:              "⏰ Дедлайн: истёк",
		locale.AchievementSharpshooterName:  "🎯 Меткий стрелок",
		locale.AchievementProphetName:       "🔮 Провидец",
		locale.AchievementRiskTakerName:     "🎲 Риск-мейкер",
		locale.AchievementWeeklyAnalystName: "📊 Аналитик недели",
		locale.AchievementVeteranName:       "🏆 Старожил",
		locale.NotificationResultsTitle:     "🏁 СОБЫТИЕ ЗАВЕРШЕНО!",
		locale.NotificationResultsTopTitle:  "🏆 ТОП УЧАСТНИКОВ",
		locale.NotificationReminderTitle:    "⏰ НАПОМИНАНИЕ!",
		locale.NotificationReminderCTA:      "Не забудьте проголосовать! 🗳",
	}

	if val, ok := translations[id]; ok {
		return val
	}
	return id
}

func (m *MockLocalizer) MustLocalizeWithTemplate(id string, fields ...string) string {
	m.localizeWithTemplateCount++
	m.lastTemplateID = id

	// Return realistic Russian translations with template substitution
	switch id {
	case locale.NotificationNewEventQuestion:
		if len(fields) > 0 {
			return fmt.Sprintf("❓ Вопрос:\n%s", fields[0])
		}
	case locale.NotificationNewEventType:
		if len(fields) >= 2 {
			return fmt.Sprintf("%s Тип: %s", fields[0], fields[1])
		}
	case locale.OptionListItem:
		if len(fields) >= 2 {
			return fmt.Sprintf("  %s) %s", fields[0], fields[1])
		}
	case locale.DeadlineDaysHours:
		if len(fields) >= 2 {
			return fmt.Sprintf("⏰ Дедлайн: %s дн. %s ч.", fields[0], fields[1])
		}
	case locale.DeadlineHoursOnly:
		if len(fields) > 0 {
			return fmt.Sprintf("⏰ Дедлайн: %s ч.", fields[0])
		}
	case locale.NotificationAchievementCongrats:
		if len(fields) > 0 {
			return fmt.Sprintf("🎉 Поздравляем! Вы получили ачивку:\n\n%s", fields[0])
		}
	case locale.NotificationAchievementAnnouncement:
		if len(fields) > 0 {
			return fmt.Sprintf("🎉 Участник получил ачивку: %s!", fields[0])
		}
	case locale.NotificationResultsQuestion:
		if len(fields) > 0 {
			return fmt.Sprintf("❓ Вопрос:\n%s", fields[0])
		}
	case locale.NotificationResultsCorrectAnswer:
		if len(fields) > 0 {
			return fmt.Sprintf("✅ Правильный ответ:\n%s", fields[0])
		}
	case locale.NotificationResultsStats:
		if len(fields) >= 2 {
			return fmt.Sprintf("📊 Угадали: %s из %s участников", fields[0], fields[1])
		}
	case locale.UserIDFormat:
		if len(fields) > 0 {
			return fmt.Sprintf("User id%s", fields[0])
		}
	case locale.RatingTopEntry:
		if len(fields) >= 3 {
			return fmt.Sprintf("%s %s - %s очков", fields[0], fields[1], fields[2])
		}
	case locale.NotificationReminderTime:
		if len(fields) > 0 {
			return fmt.Sprintf("До дедлайна события осталось ~%s часов", fields[0])
		}
	case locale.NotificationReminderQuestion:
		if len(fields) > 0 {
			return fmt.Sprintf("❓ %s", fields[0])
		}
	}

	return id
}

var _ locale.Localizer = (*MockLocalizer)(nil)

func TestResultsContainCorrectCount(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("results contain correct count of participants", prop.ForAll(
		func(eventID int64, correctOption int, numCorrect int, numWrong int) bool {
			// Ensure valid inputs
			if numCorrect < 0 || numWrong < 0 || correctOption < 0 || correctOption >= 4 {
				return true // Skip invalid inputs
			}

			// Create mock dependencies
			mockBot := &MockNotificationBot{
				sentMessages: make([]MockNotificationMessage, 0),
			}

			// Create mock event
			event := &Event{
				ID:       eventID,
				GroupID:  1,
				Question: "Test question",
				Options:  []string{"Option 0", "Option 1", "Option 2", "Option 3"},
			}

			// Create predictions
			predictions := make([]*Prediction, 0)
			for i := 0; i < numCorrect; i++ {
				predictions = append(predictions, &Prediction{
					ID:      int64(i),
					EventID: eventID,
					UserID:  int64(i + 1),
					Option:  correctOption,
				})
			}
			for i := 0; i < numWrong; i++ {
				wrongOption := (correctOption + 1) % 4
				predictions = append(predictions, &Prediction{
					ID:      int64(numCorrect + i),
					EventID: eventID,
					UserID:  int64(numCorrect + i + 1),
					Option:  wrongOption,
				})
			}

			mockEventRepo := &MockEventRepoWithData{event: event}
			mockPredictionRepo := &MockPredictionRepoWithData{predictions: predictions}
			mockRatingRepo := &MockRatingRepo{}
			mockReminderRepo := &MockReminderRepo{}
			mockLogger := &MockLogger{}

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				mockLogger,
				&MockLocalizer{},
			)

			// Publish results
			ctx := context.Background()
			telegramChatID := int64(12345) // Mock Telegram chat ID
			mockForumTopicRepo := &MockForumTopicRepo{topics: make(map[int64]*ForumTopic)}
			err := ns.PublishEventResults(ctx, eventID, correctOption, telegramChatID, mockForumTopicRepo)
			if err != nil {
				return false
			}

			// Verify message was sent
			if len(mockBot.sentMessages) != 1 {
				return false
			}

			message := mockBot.sentMessages[0].Text

			// Verify message contains correct count
			totalPredictions := numCorrect + numWrong
			expectedText := fmt.Sprintf("%d из %d", numCorrect, totalPredictions)

			return strings.Contains(message, expectedText)
		},
		gen.Int64Range(1, 1000),
		gen.IntRange(0, 3),
		gen.IntRange(0, 20),
		gen.IntRange(0, 20),
	))

	properties.TestingRun(t)
}

func TestResultsContainTop5(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("results contain top 5 participants when available", prop.ForAll(
		func(eventID int64, correctOption int, numParticipants int) bool {
			// Ensure valid inputs
			if numParticipants < 5 || numParticipants > 20 || correctOption < 0 || correctOption >= 4 {
				return true // Skip invalid inputs
			}

			// Create mock dependencies
			mockBot := &MockNotificationBot{
				sentMessages: make([]MockNotificationMessage, 0),
			}

			// Create mock event
			event := &Event{
				ID:       eventID,
				GroupID:  1,
				Question: "Test question",
				Options:  []string{"Option 0", "Option 1", "Option 2", "Option 3"},
			}

			// Create predictions
			predictions := make([]*Prediction, 0)
			for i := 0; i < numParticipants; i++ {
				predictions = append(predictions, &Prediction{
					ID:      int64(i),
					EventID: eventID,
					UserID:  int64(i + 1),
					Option:  correctOption,
				})
			}

			// Create top 5 ratings
			topRatings := make([]*Rating, 5)
			for i := 0; i < 5; i++ {
				topRatings[i] = &Rating{
					UserID:  int64(i + 1),
					GroupID: 1,
					Score:   100 - i*10,
				}
			}

			mockEventRepo := &MockEventRepoWithData{event: event}
			mockPredictionRepo := &MockPredictionRepoWithData{predictions: predictions}
			mockRatingRepo := &MockRatingRepoWithData{topRatings: topRatings}
			mockReminderRepo := &MockReminderRepo{}
			mockLogger := &MockLogger{}

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				mockLogger,
				&MockLocalizer{},
			)

			// Publish results
			ctx := context.Background()
			telegramChatID := int64(12345) // Mock Telegram chat ID
			mockForumTopicRepo := &MockForumTopicRepo{topics: make(map[int64]*ForumTopic)}
			err := ns.PublishEventResults(ctx, eventID, correctOption, telegramChatID, mockForumTopicRepo)
			if err != nil {
				return false
			}

			// Verify message was sent
			if len(mockBot.sentMessages) != 1 {
				return false
			}

			message := mockBot.sentMessages[0].Text

			// Verify message contains "ТОП УЧАСТНИКОВ" section
			if !strings.Contains(message, "ТОП УЧАСТНИКОВ") {
				return false
			}

			// Verify all 5 ratings are present
			for i := 0; i < 5; i++ {
				scoreText := fmt.Sprintf("%d очков", topRatings[i].Score)
				if !strings.Contains(message, scoreText) {
					return false
				}
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.IntRange(0, 3),
		gen.IntRange(5, 20),
	))

	properties.TestingRun(t)
}

// Mock implementations with data

type MockEventRepoWithData struct {
	event *Event
}

func (m *MockEventRepoWithData) CreateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *MockEventRepoWithData) GetEvent(ctx context.Context, eventID int64) (*Event, error) {
	return m.event, nil
}

func (m *MockEventRepoWithData) GetEventByPollID(ctx context.Context, pollID string) (*Event, error) {
	return m.event, nil
}

func (m *MockEventRepoWithData) GetActiveEvents(ctx context.Context, groupID int64) ([]*Event, error) {
	return []*Event{m.event}, nil
}

func (m *MockEventRepoWithData) UpdateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *MockEventRepoWithData) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	return nil
}

func (m *MockEventRepoWithData) GetResolvedEvents(ctx context.Context) ([]*Event, error) {
	return []*Event{m.event}, nil
}

func (m *MockEventRepoWithData) GetUserCreatedEventsCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	return 0, nil
}

func (m *MockEventRepoWithData) GetEventsByDeadlineRange(ctx context.Context, start, end time.Time) ([]*Event, error) {
	return []*Event{m.event}, nil
}

type MockPredictionRepoWithData struct {
	predictions []*Prediction
}

func (m *MockPredictionRepoWithData) SavePrediction(ctx context.Context, prediction *Prediction) error {
	return nil
}

func (m *MockPredictionRepoWithData) UpdatePrediction(ctx context.Context, prediction *Prediction) error {
	return nil
}

func (m *MockPredictionRepoWithData) GetPredictionsByEvent(ctx context.Context, eventID int64) ([]*Prediction, error) {
	return m.predictions, nil
}

func (m *MockPredictionRepoWithData) GetPredictionByUserAndEvent(ctx context.Context, userID, eventID int64) (*Prediction, error) {
	return nil, nil
}

func (m *MockPredictionRepoWithData) GetUserPredictions(ctx context.Context, userID int64) ([]*Prediction, error) {
	return m.predictions, nil
}

func (m *MockPredictionRepoWithData) GetUserCompletedEventCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	return 0, nil
}

type MockRatingRepoWithData struct {
	topRatings []*Rating
}

func (m *MockRatingRepoWithData) GetRating(ctx context.Context, userID int64, groupID int64) (*Rating, error) {
	return &Rating{}, nil
}

func (m *MockRatingRepoWithData) UpdateRating(ctx context.Context, rating *Rating) error {
	return nil
}

func (m *MockRatingRepoWithData) GetTopRatings(ctx context.Context, groupID int64, limit int) ([]*Rating, error) {
	return m.topRatings, nil
}

func (m *MockRatingRepoWithData) UpdateStreak(ctx context.Context, userID int64, groupID int64, streak int) error {
	return nil
}

type MockReminderRepo struct{}

func (m *MockReminderRepo) WasReminderSent(ctx context.Context, eventID int64) (bool, error) {
	return false, nil
}

func (m *MockReminderRepo) MarkReminderSent(ctx context.Context, eventID int64) error {
	return nil
}

func (m *MockReminderRepo) WasOrganizerNotificationSent(ctx context.Context, eventID int64) (bool, error) {
	return false, nil
}

func (m *MockReminderRepo) MarkOrganizerNotificationSent(ctx context.Context, eventID int64) error {
	return nil
}

func TestNotificationServiceUsesLocalizer(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Test SendNewEventNotification uses Localizer
	properties.Property("SendNewEventNotification uses Localizer for all messages", prop.ForAll(
		func(eventID int64, question string, eventType int) bool {
			// Ensure valid inputs
			if question == "" || eventType < 0 || eventType > 2 {
				return true // Skip invalid inputs
			}

			// Map int to EventType
			var eType EventType
			switch eventType {
			case 0:
				eType = EventTypeBinary
			case 1:
				eType = EventTypeMultiOption
			case 2:
				eType = EventTypeProbability
			}

			// Create mock dependencies
			mockBot := &MockNotificationBot{
				sentMessages: make([]MockNotificationMessage, 0),
			}

			// Create mock event
			event := &Event{
				ID:        eventID,
				GroupID:   1,
				Question:  question,
				EventType: eType,
				Options:   []string{"Option 1", "Option 2"},
				Deadline:  time.Now().Add(48 * time.Hour),
			}

			mockEventRepo := &MockEventRepoWithData{event: event}
			mockPredictionRepo := &MockPredictionRepo{}
			mockRatingRepo := &MockRatingRepo{}
			mockReminderRepo := &MockReminderRepo{}
			mockLogger := &MockLogger{}
			mockLocalizer := &MockLocalizer{}

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				mockLogger,
				mockLocalizer,
			)

			// Send new event notification
			ctx := context.Background()
			err := ns.SendNewEventNotification(ctx, eventID)
			if err != nil {
				return false
			}

			// Verify that Localizer was called
			// Should call MustLocalize at least once and MustLocalizeWithTemplate at least once
			if mockLocalizer.localizeCallCount == 0 {
				t.Logf("Expected MustLocalize to be called, but it wasn't")
				return false
			}

			if mockLocalizer.localizeWithTemplateCount == 0 {
				t.Logf("Expected MustLocalizeWithTemplate to be called, but it wasn't")
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 200 }),
		gen.IntRange(0, 2),
	))

	// Test SendAchievementNotification uses Localizer
	properties.Property("SendAchievementNotification uses Localizer for all messages", prop.ForAll(
		func(userID int64, achievementCode string) bool {
			// Map string to achievement code
			var code AchievementCode
			switch achievementCode {
			case "sharpshooter":
				code = AchievementSharpshooter
			case "prophet":
				code = AchievementProphet
			case "risk_taker":
				code = AchievementRiskTaker
			case "weekly_analyst":
				code = AchievementWeeklyAnalyst
			case "veteran":
				code = AchievementVeteran
			default:
				return true // Skip invalid codes
			}

			// Create mock dependencies
			mockBot := &MockNotificationBot{
				sentMessages: make([]MockNotificationMessage, 0),
			}
			mockEventRepo := &MockEventRepo{}
			mockPredictionRepo := &MockPredictionRepo{}
			mockRatingRepo := &MockRatingRepo{}
			mockReminderRepo := &MockReminderRepo{}
			mockLogger := &MockLogger{}
			mockLocalizer := &MockLocalizer{}

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				mockLogger,
				mockLocalizer,
			)

			// Create achievement
			achievement := &Achievement{
				ID:        1,
				UserID:    userID,
				Code:      code,
				Timestamp: time.Now(),
			}

			// Send achievement notification
			ctx := context.Background()
			err := ns.SendAchievementNotification(ctx, userID, achievement)
			if err != nil {
				return false
			}

			// Verify that Localizer was called
			// Should call MustLocalize at least once for achievement names
			// and MustLocalizeWithTemplate at least once for messages
			if mockLocalizer.localizeCallCount == 0 {
				t.Logf("Expected MustLocalize to be called, but it wasn't")
				return false
			}

			if mockLocalizer.localizeWithTemplateCount == 0 {
				t.Logf("Expected MustLocalizeWithTemplate to be called, but it wasn't")
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.OneConstOf("sharpshooter", "prophet", "risk_taker", "weekly_analyst", "veteran"),
	))

	// Test PublishEventResults uses Localizer
	properties.Property("PublishEventResults uses Localizer for all messages", prop.ForAll(
		func(eventID int64, correctOption int, numCorrect int, numWrong int) bool {
			// Ensure valid inputs
			if numCorrect < 0 || numWrong < 0 || correctOption < 0 || correctOption >= 4 {
				return true // Skip invalid inputs
			}

			// Create mock dependencies
			mockBot := &MockNotificationBot{
				sentMessages: make([]MockNotificationMessage, 0),
			}

			// Create mock event
			event := &Event{
				ID:       eventID,
				GroupID:  1,
				Question: "Test question",
				Options:  []string{"Option 0", "Option 1", "Option 2", "Option 3"},
			}

			// Create predictions
			predictions := make([]*Prediction, 0)
			for i := 0; i < numCorrect; i++ {
				predictions = append(predictions, &Prediction{
					ID:      int64(i),
					EventID: eventID,
					UserID:  int64(i + 1),
					Option:  correctOption,
				})
			}
			for i := 0; i < numWrong; i++ {
				wrongOption := (correctOption + 1) % 4
				predictions = append(predictions, &Prediction{
					ID:      int64(numCorrect + i),
					EventID: eventID,
					UserID:  int64(numCorrect + i + 1),
					Option:  wrongOption,
				})
			}

			mockEventRepo := &MockEventRepoWithData{event: event}
			mockPredictionRepo := &MockPredictionRepoWithData{predictions: predictions}
			mockRatingRepo := &MockRatingRepo{}
			mockReminderRepo := &MockReminderRepo{}
			mockLogger := &MockLogger{}
			mockLocalizer := &MockLocalizer{}

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				mockLogger,
				mockLocalizer,
			)

			// Publish results
			ctx := context.Background()
			telegramChatID := int64(12345)
			mockForumTopicRepo := &MockForumTopicRepo{topics: make(map[int64]*ForumTopic)}
			err := ns.PublishEventResults(ctx, eventID, correctOption, telegramChatID, mockForumTopicRepo)
			if err != nil {
				return false
			}

			// Verify that Localizer was called
			if mockLocalizer.localizeCallCount == 0 {
				t.Logf("Expected MustLocalize to be called, but it wasn't")
				return false
			}

			if mockLocalizer.localizeWithTemplateCount == 0 {
				t.Logf("Expected MustLocalizeWithTemplate to be called, but it wasn't")
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.IntRange(0, 3),
		gen.IntRange(0, 20),
		gen.IntRange(0, 20),
	))

	// Test SendDeadlineReminder uses Localizer
	properties.Property("SendDeadlineReminder uses Localizer for all messages", prop.ForAll(
		func(eventID int64, question string, hoursUntilDeadline int) bool {
			// Ensure valid inputs
			if question == "" || hoursUntilDeadline < 1 || hoursUntilDeadline > 100 {
				return true // Skip invalid inputs
			}

			// Create mock dependencies
			mockBot := &MockNotificationBot{
				sentMessages: make([]MockNotificationMessage, 0),
			}

			// Create mock event
			event := &Event{
				ID:       eventID,
				GroupID:  1,
				Question: question,
				Status:   EventStatusActive,
				Deadline: time.Now().Add(time.Duration(hoursUntilDeadline) * time.Hour),
			}

			// Create mock ratings (users who could receive reminders)
			ratings := []*Rating{
				{UserID: 1, GroupID: 1, Score: 100},
				{UserID: 2, GroupID: 1, Score: 90},
			}

			mockEventRepo := &MockEventRepoWithData{event: event}
			mockPredictionRepo := &MockPredictionRepo{}
			mockRatingRepo := &MockRatingRepoWithData{topRatings: ratings}
			mockReminderRepo := &MockReminderRepo{}
			mockLogger := &MockLogger{}
			mockLocalizer := &MockLocalizer{}

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				mockLogger,
				mockLocalizer,
			)

			// Send deadline reminder
			ctx := context.Background()
			err := ns.SendDeadlineReminder(ctx, eventID)
			if err != nil {
				return false
			}

			// Verify that Localizer was called
			if mockLocalizer.localizeCallCount == 0 {
				t.Logf("Expected MustLocalize to be called, but it wasn't")
				return false
			}

			if mockLocalizer.localizeWithTemplateCount == 0 {
				t.Logf("Expected MustLocalizeWithTemplate to be called, but it wasn't")
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 200 }),
		gen.IntRange(1, 100),
	))

	properties.TestingRun(t)
}
