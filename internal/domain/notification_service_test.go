package domain

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: telegram-prediction-bot, Property 12: Achievement notification
// **Validates: Requirements 5.6**
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

			groupID := int64(12345)

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				groupID,
				mockLogger,
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

			// Verify second message was sent to group
			if mockBot.sentMessages[1].ChatID != groupID {
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

func (m *MockEventRepo) GetActiveEvents(ctx context.Context) ([]*Event, error) {
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

type MockRatingRepo struct{}

func (m *MockRatingRepo) GetRating(ctx context.Context, userID int64) (*Rating, error) {
	return &Rating{}, nil
}

func (m *MockRatingRepo) UpdateRating(ctx context.Context, rating *Rating) error {
	return nil
}

func (m *MockRatingRepo) GetTopRatings(ctx context.Context, limit int) ([]*Rating, error) {
	return []*Rating{}, nil
}

func (m *MockRatingRepo) UpdateStreak(ctx context.Context, userID int64, streak int) error {
	return nil
}

type MockLogger struct{}

func (m *MockLogger) Info(msg string, args ...interface{}) {}

func (m *MockLogger) Error(msg string, args ...interface{}) {}

func (m *MockLogger) Debug(msg string, args ...interface{}) {}

func (m *MockLogger) Warn(msg string, args ...interface{}) {}

// Feature: telegram-prediction-bot, Property 26: Results contain correct count
// **Validates: Requirements 11.2**
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

			groupID := int64(12345)

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				groupID,
				mockLogger,
			)

			// Publish results
			ctx := context.Background()
			err := ns.PublishEventResults(ctx, eventID, correctOption)
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

// Feature: telegram-prediction-bot, Property 27: Results contain top 5
// **Validates: Requirements 11.3**
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
					UserID: int64(i + 1),
					Score:  100 - i*10,
				}
			}

			mockEventRepo := &MockEventRepoWithData{event: event}
			mockPredictionRepo := &MockPredictionRepoWithData{predictions: predictions}
			mockRatingRepo := &MockRatingRepoWithData{topRatings: topRatings}
			mockReminderRepo := &MockReminderRepo{}
			mockLogger := &MockLogger{}

			groupID := int64(12345)

			// Create notification service
			ns := NewNotificationService(
				mockBot,
				mockEventRepo,
				mockPredictionRepo,
				mockRatingRepo,
				mockReminderRepo,
				groupID,
				mockLogger,
			)

			// Publish results
			ctx := context.Background()
			err := ns.PublishEventResults(ctx, eventID, correctOption)
			if err != nil {
				return false
			}

			// Verify message was sent
			if len(mockBot.sentMessages) != 1 {
				return false
			}

			message := mockBot.sentMessages[0].Text

			// Verify message contains "Топ-5" section
			if !strings.Contains(message, "Топ-5") {
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

func (m *MockEventRepoWithData) GetActiveEvents(ctx context.Context) ([]*Event, error) {
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

type MockRatingRepoWithData struct {
	topRatings []*Rating
}

func (m *MockRatingRepoWithData) GetRating(ctx context.Context, userID int64) (*Rating, error) {
	return &Rating{}, nil
}

func (m *MockRatingRepoWithData) UpdateRating(ctx context.Context, rating *Rating) error {
	return nil
}

func (m *MockRatingRepoWithData) GetTopRatings(ctx context.Context, limit int) ([]*Rating, error) {
	return m.topRatings, nil
}

func (m *MockRatingRepoWithData) UpdateStreak(ctx context.Context, userID int64, streak int) error {
	return nil
}

type MockReminderRepo struct{}

func (m *MockReminderRepo) WasReminderSent(ctx context.Context, eventID int64) (bool, error) {
	return false, nil
}

func (m *MockReminderRepo) MarkReminderSent(ctx context.Context, eventID int64) error {
	return nil
}
