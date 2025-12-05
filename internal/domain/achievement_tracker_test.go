package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Mock AchievementRepository for testing
type mockAchievementRepo struct {
	achievements map[int64]map[AchievementCode]*Achievement
	saveError    error
	getError     error
}

func newMockAchievementRepo() *mockAchievementRepo {
	return &mockAchievementRepo{
		achievements: make(map[int64]map[AchievementCode]*Achievement),
	}
}

func (m *mockAchievementRepo) SaveAchievement(ctx context.Context, achievement *Achievement) error {
	if m.saveError != nil {
		return m.saveError
	}

	if m.achievements[achievement.UserID] == nil {
		m.achievements[achievement.UserID] = make(map[AchievementCode]*Achievement)
	}

	// Simulate database ID assignment
	achievement.ID = int64(len(m.achievements[achievement.UserID]) + 1)
	m.achievements[achievement.UserID][achievement.Code] = achievement
	return nil
}

func (m *mockAchievementRepo) GetUserAchievements(ctx context.Context, userID int64) ([]*Achievement, error) {
	if m.getError != nil {
		return nil, m.getError
	}

	var result []*Achievement
	if userAchievements, exists := m.achievements[userID]; exists {
		for _, achievement := range userAchievements {
			result = append(result, achievement)
		}
	}
	return result, nil
}

func (m *mockAchievementRepo) CheckAchievementExists(ctx context.Context, userID int64, code AchievementCode) (bool, error) {
	if m.getError != nil {
		return false, m.getError
	}

	if userAchievements, exists := m.achievements[userID]; exists {
		_, exists := userAchievements[code]
		return exists, nil
	}
	return false, nil
}

// Mock RatingRepository for testing
type mockRatingRepo struct{}

func (m *mockRatingRepo) GetRating(ctx context.Context, userID int64) (*Rating, error) {
	return &Rating{UserID: userID}, nil
}

func (m *mockRatingRepo) UpdateRating(ctx context.Context, rating *Rating) error {
	return nil
}

func (m *mockRatingRepo) GetTopRatings(ctx context.Context, limit int) ([]*Rating, error) {
	return nil, nil
}

func (m *mockRatingRepo) UpdateStreak(ctx context.Context, userID int64, streak int) error {
	return nil
}

// Mock EventRepository for creator achievements testing
type mockEventRepoForCreator struct {
	createdEventsCount int
	err                error
}

func (m *mockEventRepoForCreator) CreateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *mockEventRepoForCreator) GetEvent(ctx context.Context, eventID int64) (*Event, error) {
	return nil, nil
}

func (m *mockEventRepoForCreator) GetEventByPollID(ctx context.Context, pollID string) (*Event, error) {
	return nil, nil
}

func (m *mockEventRepoForCreator) GetActiveEvents(ctx context.Context) ([]*Event, error) {
	return nil, nil
}

func (m *mockEventRepoForCreator) GetResolvedEvents(ctx context.Context) ([]*Event, error) {
	return nil, nil
}

func (m *mockEventRepoForCreator) UpdateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *mockEventRepoForCreator) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	return nil
}

func (m *mockEventRepoForCreator) GetUserCreatedEventsCount(ctx context.Context, userID int64) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.createdEventsCount, nil
}

// TestAchievementPersistence tests: Achievement persistence
func TestAchievementPersistence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("saved achievements are retrievable with same data", prop.ForAll(
		func(userID int64, createdCount int) bool {
			ctx := context.Background()
			achievementRepo := newMockAchievementRepo()
			eventRepo := &mockEventRepoForCreator{createdEventsCount: createdCount}

			tracker := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepo{},
				eventRepo,
				&mockLogger{},
			)

			// Award creator achievements
			newAchievements, err := tracker.CheckCreatorAchievements(ctx, userID)
			if err != nil {
				t.Logf("Unexpected error: %v", err)
				return false
			}

			// Verify each awarded achievement can be retrieved
			for _, awarded := range newAchievements {
				// Check that the achievement exists
				exists, err := achievementRepo.CheckAchievementExists(ctx, userID, awarded.Code)
				if err != nil {
					t.Logf("Error checking achievement existence: %v", err)
					return false
				}
				if !exists {
					t.Logf("Achievement not found after saving: user=%d, code=%s", userID, awarded.Code)
					return false
				}

				// Retrieve all user achievements
				allAchievements, err := achievementRepo.GetUserAchievements(ctx, userID)
				if err != nil {
					t.Logf("Error retrieving achievements: %v", err)
					return false
				}

				// Find the specific achievement
				found := false
				for _, retrieved := range allAchievements {
					if retrieved.Code == awarded.Code && retrieved.UserID == userID {
						// Verify the data matches
						if retrieved.UserID != awarded.UserID {
							t.Logf("UserID mismatch: expected %d, got %d", awarded.UserID, retrieved.UserID)
							return false
						}
						if retrieved.Code != awarded.Code {
							t.Logf("Code mismatch: expected %s, got %s", awarded.Code, retrieved.Code)
							return false
						}
						// Timestamp should be set (not zero)
						if retrieved.Timestamp.IsZero() {
							t.Logf("Timestamp is zero")
							return false
						}
						found = true
						break
					}
				}

				if !found {
					t.Logf("Achievement not found in retrieved list: user=%d, code=%s", userID, awarded.Code)
					return false
				}
			}

			return true
		},
		gen.Int64Range(1, 1000000), // userID
		gen.IntRange(0, 30),        // createdCount (0-30 events)
	))

	properties.TestingRun(t)
}

// TestCheckCreatorAchievements_Thresholds tests achievement thresholds
func TestCheckCreatorAchievements_Thresholds(t *testing.T) {
	testCases := []struct {
		name                 string
		createdCount         int
		expectedAchievements []AchievementCode
	}{
		{
			name:                 "no events created",
			createdCount:         0,
			expectedAchievements: []AchievementCode{},
		},
		{
			name:         "first event - Event Organizer",
			createdCount: 1,
			expectedAchievements: []AchievementCode{
				AchievementEventOrganizer,
			},
		},
		{
			name:         "four events - only Event Organizer",
			createdCount: 4,
			expectedAchievements: []AchievementCode{
				AchievementEventOrganizer,
			},
		},
		{
			name:         "five events - Event Organizer and Active Organizer",
			createdCount: 5,
			expectedAchievements: []AchievementCode{
				AchievementEventOrganizer,
				AchievementActiveOrganizer,
			},
		},
		{
			name:         "nineteen events - Event Organizer and Active Organizer",
			createdCount: 19,
			expectedAchievements: []AchievementCode{
				AchievementEventOrganizer,
				AchievementActiveOrganizer,
			},
		},
		{
			name:         "twenty events - all three achievements",
			createdCount: 25,
			expectedAchievements: []AchievementCode{
				AchievementEventOrganizer,
				AchievementActiveOrganizer,
				AchievementMasterOrganizer,
			},
		},
		{
			name:         "many events - all three achievements",
			createdCount: 50,
			expectedAchievements: []AchievementCode{
				AchievementEventOrganizer,
				AchievementActiveOrganizer,
				AchievementMasterOrganizer,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			achievementRepo := newMockAchievementRepo()
			eventRepo := &mockEventRepoForCreator{createdEventsCount: tc.createdCount}

			tracker := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepo{},
				eventRepo,
				&mockLogger{},
			)

			achievements, err := tracker.CheckCreatorAchievements(ctx, 12345)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(achievements) != len(tc.expectedAchievements) {
				t.Errorf("Expected %d achievements, got %d", len(tc.expectedAchievements), len(achievements))
			}

			// Check that all expected achievements were awarded
			awardedCodes := make(map[AchievementCode]bool)
			for _, achievement := range achievements {
				awardedCodes[achievement.Code] = true
			}

			for _, expectedCode := range tc.expectedAchievements {
				if !awardedCodes[expectedCode] {
					t.Errorf("Expected achievement %s was not awarded", expectedCode)
				}
			}
		})
	}
}

// TestCheckCreatorAchievements_DuplicatePrevention tests that achievements are not awarded twice
func TestCheckCreatorAchievements_DuplicatePrevention(t *testing.T) {
	ctx := context.Background()
	achievementRepo := newMockAchievementRepo()
	eventRepo := &mockEventRepoForCreator{createdEventsCount: 25}

	tracker := NewAchievementTracker(
		achievementRepo,
		&mockRatingRepo{},
		&mockPredictionRepo{},
		eventRepo,
		&mockLogger{},
	)

	// First call - should award all three achievements
	achievements1, err := tracker.CheckCreatorAchievements(ctx, 12345)
	if err != nil {
		t.Fatalf("Unexpected error on first call: %v", err)
	}

	if len(achievements1) != 3 {
		t.Errorf("Expected 3 achievements on first call, got %d", len(achievements1))
	}

	// Second call - should not award any achievements (duplicates)
	achievements2, err := tracker.CheckCreatorAchievements(ctx, 12345)
	if err != nil {
		t.Fatalf("Unexpected error on second call: %v", err)
	}

	if len(achievements2) != 0 {
		t.Errorf("Expected 0 achievements on second call (duplicates), got %d", len(achievements2))
	}

	// Verify total achievements in repository is still 3
	allAchievements, err := achievementRepo.GetUserAchievements(ctx, 12345)
	if err != nil {
		t.Fatalf("Error getting achievements: %v", err)
	}

	if len(allAchievements) != 3 {
		t.Errorf("Expected 3 total achievements in repository, got %d", len(allAchievements))
	}
}

// TestCheckCreatorAchievements_CountingLogic tests the counting logic
func TestCheckCreatorAchievements_CountingLogic(t *testing.T) {
	testCases := []struct {
		name          string
		createdCount  int
		expectedCount int
	}{
		{"zero events", 0, 0},
		{"one event", 1, 1},
		{"five events", 5, 2},
		{"twenty five events", 25, 3},
		{"hundred events", 100, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			achievementRepo := newMockAchievementRepo()
			eventRepo := &mockEventRepoForCreator{createdEventsCount: tc.createdCount}

			tracker := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepo{},
				eventRepo,
				&mockLogger{},
			)

			achievements, err := tracker.CheckCreatorAchievements(ctx, 12345)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(achievements) != tc.expectedCount {
				t.Errorf("Expected %d achievements for %d events, got %d",
					tc.expectedCount, tc.createdCount, len(achievements))
			}
		})
	}
}

// TestCheckCreatorAchievements_Error tests error handling
func TestCheckCreatorAchievements_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("database error")
	achievementRepo := newMockAchievementRepo()
	eventRepo := &mockEventRepoForCreator{err: expectedErr}

	tracker := NewAchievementTracker(
		achievementRepo,
		&mockRatingRepo{},
		&mockPredictionRepo{},
		eventRepo,
		&mockLogger{},
	)

	achievements, err := tracker.CheckCreatorAchievements(ctx, 12345)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if achievements != nil {
		t.Errorf("Expected nil achievements on error, got %v", achievements)
	}
}
