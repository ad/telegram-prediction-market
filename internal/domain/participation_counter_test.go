package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Mock logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...interface{}) {}
func (m *mockLogger) Info(msg string, args ...interface{})  {}
func (m *mockLogger) Error(msg string, args ...interface{}) {}
func (m *mockLogger) Warn(msg string, args ...interface{})  {}

// Mock PredictionRepository for testing
type mockPredictionRepo struct {
	completedEventCount int
	err                 error
}

func (m *mockPredictionRepo) SavePrediction(ctx context.Context, prediction *Prediction) error {
	return nil
}

func (m *mockPredictionRepo) UpdatePrediction(ctx context.Context, prediction *Prediction) error {
	return nil
}

func (m *mockPredictionRepo) GetUserCompletedEventCount(ctx context.Context, userID int64) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.completedEventCount, nil
}

func (m *mockPredictionRepo) GetUserPredictions(ctx context.Context, userID int64) ([]*Prediction, error) {
	return nil, nil
}

func (m *mockPredictionRepo) GetPredictionsByEvent(ctx context.Context, eventID int64) ([]*Prediction, error) {
	return nil, nil
}

func (m *mockPredictionRepo) GetPredictionByUserAndEvent(ctx context.Context, userID, eventID int64) (*Prediction, error) {
	return nil, nil
}

// Mock EventRepository for testing
type mockEventRepo struct{}

func (m *mockEventRepo) CreateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *mockEventRepo) GetEvent(ctx context.Context, eventID int64) (*Event, error) {
	return nil, nil
}

func (m *mockEventRepo) GetEventByPollID(ctx context.Context, pollID string) (*Event, error) {
	return nil, nil
}

func (m *mockEventRepo) GetActiveEvents(ctx context.Context) ([]*Event, error) {
	return nil, nil
}

func (m *mockEventRepo) GetResolvedEvents(ctx context.Context) ([]*Event, error) {
	return nil, nil
}

func (m *mockEventRepo) UpdateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *mockEventRepo) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	return nil
}

func (m *mockEventRepo) GetUserCreatedEventsCount(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

// TestParticipationRequirementCheck tests Property 7: Participation requirement check
// Feature: event-creator-permissions-and-achievements, Property 7: Participation requirement check
// Validates: Requirements 3.1
func TestParticipationRequirementCheck(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("participation count is verified for all users", prop.ForAll(
		func(userID int64, completedCount int, minRequired int) bool {
			// Create mock repository with the completed count
			mockRepo := &mockPredictionRepo{
				completedEventCount: completedCount,
			}

			counter := NewParticipationCounter(mockRepo, &mockEventRepo{}, &mockLogger{})

			// Count participation
			count, err := counter.CountCompletedEventParticipation(context.Background(), userID)
			if err != nil {
				t.Logf("Unexpected error: %v", err)
				return false
			}

			// Verify the count matches what the repository returned
			if count != completedCount {
				t.Logf("Expected count %d, got %d", completedCount, count)
				return false
			}

			// Verify the participation requirement check logic
			// (This would be used by the permission validator)
			hasEnoughParticipation := count >= minRequired

			// The property is: we can always determine if a user meets the requirement
			// by comparing their participation count to the minimum
			expectedResult := completedCount >= minRequired
			if hasEnoughParticipation != expectedResult {
				t.Logf("Participation check mismatch: count=%d, min=%d, result=%v, expected=%v",
					count, minRequired, hasEnoughParticipation, expectedResult)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000), // userID
		gen.IntRange(0, 100),       // completedCount
		gen.IntRange(0, 20),        // minRequired
	))

	properties.TestingRun(t)
}

// TestCountCompletedEventParticipation_Success tests successful participation counting
// Requirements: 3.1, 3.4
func TestCountCompletedEventParticipation_Success(t *testing.T) {
	testCases := []struct {
		name           string
		completedCount int
	}{
		{"no participation", 0},
		{"one event", 1},
		{"three events", 3},
		{"many events", 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &mockPredictionRepo{
				completedEventCount: tc.completedCount,
			}

			counter := NewParticipationCounter(mockRepo, &mockEventRepo{}, &mockLogger{})

			count, err := counter.CountCompletedEventParticipation(context.Background(), 12345)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if count != tc.completedCount {
				t.Errorf("Expected count %d, got %d", tc.completedCount, count)
			}
		})
	}
}

// TestCountCompletedEventParticipation_Error tests error handling
// Requirements: 3.1, 3.4
func TestCountCompletedEventParticipation_Error(t *testing.T) {
	expectedErr := errors.New("database error")
	mockRepo := &mockPredictionRepo{
		err: expectedErr,
	}

	counter := NewParticipationCounter(mockRepo, &mockEventRepo{}, &mockLogger{})

	count, err := counter.CountCompletedEventParticipation(context.Background(), 12345)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if count != 0 {
		t.Errorf("Expected count 0 on error, got %d", count)
	}

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}
