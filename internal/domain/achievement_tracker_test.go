package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// mockLoggerForAchievements implements the Logger interface for testing
type mockLoggerForAchievements struct{}

func (m *mockLoggerForAchievements) Debug(msg string, args ...interface{}) {}
func (m *mockLoggerForAchievements) Info(msg string, args ...interface{})  {}
func (m *mockLoggerForAchievements) Warn(msg string, args ...interface{})  {}
func (m *mockLoggerForAchievements) Error(msg string, args ...interface{}) {}

// mockPredictionRepoForAchievements implements PredictionRepository for testing
type mockPredictionRepoForAchievements struct{}

func (m *mockPredictionRepoForAchievements) SavePrediction(ctx context.Context, prediction *Prediction) error {
	return nil
}

func (m *mockPredictionRepoForAchievements) UpdatePrediction(ctx context.Context, prediction *Prediction) error {
	return nil
}

func (m *mockPredictionRepoForAchievements) GetUserCompletedEventCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	return 0, nil
}

func (m *mockPredictionRepoForAchievements) GetUserPredictions(ctx context.Context, userID int64) ([]*Prediction, error) {
	return nil, nil
}

func (m *mockPredictionRepoForAchievements) GetPredictionsByEvent(ctx context.Context, eventID int64) ([]*Prediction, error) {
	return nil, nil
}

func (m *mockPredictionRepoForAchievements) GetPredictionByUserAndEvent(ctx context.Context, userID, eventID int64) (*Prediction, error) {
	return nil, nil
}

// Mock AchievementRepository for testing
type mockAchievementRepo struct {
	achievements map[int64]map[int64]map[AchievementCode]*Achievement // userID -> groupID -> code -> achievement
	saveError    error
	getError     error
}

func newMockAchievementRepo() *mockAchievementRepo {
	return &mockAchievementRepo{
		achievements: make(map[int64]map[int64]map[AchievementCode]*Achievement),
	}
}

func (m *mockAchievementRepo) SaveAchievement(ctx context.Context, achievement *Achievement) error {
	if m.saveError != nil {
		return m.saveError
	}

	if m.achievements[achievement.UserID] == nil {
		m.achievements[achievement.UserID] = make(map[int64]map[AchievementCode]*Achievement)
	}
	if m.achievements[achievement.UserID][achievement.GroupID] == nil {
		m.achievements[achievement.UserID][achievement.GroupID] = make(map[AchievementCode]*Achievement)
	}

	// Simulate database ID assignment
	achievement.ID = int64(len(m.achievements[achievement.UserID][achievement.GroupID]) + 1)
	m.achievements[achievement.UserID][achievement.GroupID][achievement.Code] = achievement
	return nil
}

func (m *mockAchievementRepo) GetUserAchievements(ctx context.Context, userID int64, groupID int64) ([]*Achievement, error) {
	if m.getError != nil {
		return nil, m.getError
	}

	var result []*Achievement
	if userGroups, exists := m.achievements[userID]; exists {
		if groupAchievements, exists := userGroups[groupID]; exists {
			for _, achievement := range groupAchievements {
				result = append(result, achievement)
			}
		}
	}
	return result, nil
}

func (m *mockAchievementRepo) CheckAchievementExists(ctx context.Context, userID int64, groupID int64, code AchievementCode) (bool, error) {
	if m.getError != nil {
		return false, m.getError
	}

	if userGroups, exists := m.achievements[userID]; exists {
		if groupAchievements, exists := userGroups[groupID]; exists {
			_, exists := groupAchievements[code]
			return exists, nil
		}
	}
	return false, nil
}

// Mock RatingRepository for testing
type mockRatingRepo struct{}

func (m *mockRatingRepo) GetRating(ctx context.Context, userID int64, groupID int64) (*Rating, error) {
	return &Rating{UserID: userID, GroupID: groupID}, nil
}

func (m *mockRatingRepo) UpdateRating(ctx context.Context, rating *Rating) error {
	return nil
}

func (m *mockRatingRepo) GetTopRatings(ctx context.Context, groupID int64, limit int) ([]*Rating, error) {
	return nil, nil
}

func (m *mockRatingRepo) UpdateStreak(ctx context.Context, userID int64, groupID int64, streak int) error {
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

func (m *mockEventRepoForCreator) GetActiveEvents(ctx context.Context, groupID int64) ([]*Event, error) {
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

func (m *mockEventRepoForCreator) GetUserCreatedEventsCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.createdEventsCount, nil
}

func (m *mockEventRepoForCreator) GetEventsByDeadlineRange(ctx context.Context, start, end time.Time) ([]*Event, error) {
	return nil, nil
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
				&mockPredictionRepoForAchievements{},
				eventRepo,
				&mockLoggerForAchievements{},
			)

			// Use a fixed groupID for testing
			groupID := int64(1)

			// Award creator achievements
			newAchievements, err := tracker.CheckCreatorAchievements(ctx, userID, groupID)
			if err != nil {
				t.Logf("Unexpected error: %v", err)
				return false
			}

			// Verify each awarded achievement can be retrieved
			for _, awarded := range newAchievements {
				// Check that the achievement exists
				exists, err := achievementRepo.CheckAchievementExists(ctx, userID, groupID, awarded.Code)
				if err != nil {
					t.Logf("Error checking achievement existence: %v", err)
					return false
				}
				if !exists {
					t.Logf("Achievement not found after saving: user=%d, group=%d, code=%s", userID, groupID, awarded.Code)
					return false
				}

				// Retrieve all user achievements
				allAchievements, err := achievementRepo.GetUserAchievements(ctx, userID, groupID)
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
				&mockPredictionRepoForAchievements{},
				eventRepo,
				&mockLoggerForAchievements{},
			)

			achievements, err := tracker.CheckCreatorAchievements(ctx, 12345, 1)
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
		&mockPredictionRepoForAchievements{},
		eventRepo,
		&mockLoggerForAchievements{},
	)

	// First call - should award all three achievements
	achievements1, err := tracker.CheckCreatorAchievements(ctx, 12345, 1)
	if err != nil {
		t.Fatalf("Unexpected error on first call: %v", err)
	}

	if len(achievements1) != 3 {
		t.Errorf("Expected 3 achievements on first call, got %d", len(achievements1))
	}

	// Second call - should not award any achievements (duplicates)
	achievements2, err := tracker.CheckCreatorAchievements(ctx, 12345, 1)
	if err != nil {
		t.Fatalf("Unexpected error on second call: %v", err)
	}

	if len(achievements2) != 0 {
		t.Errorf("Expected 0 achievements on second call (duplicates), got %d", len(achievements2))
	}

	// Verify total achievements in repository is still 3
	allAchievements, err := achievementRepo.GetUserAchievements(ctx, 12345, 1)
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
				&mockPredictionRepoForAchievements{},
				eventRepo,
				&mockLoggerForAchievements{},
			)

			achievements, err := tracker.CheckCreatorAchievements(ctx, 12345, 1)
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
		&mockPredictionRepoForAchievements{},
		eventRepo,
		&mockLoggerForAchievements{},
	)

	achievements, err := tracker.CheckCreatorAchievements(ctx, 12345, 1)
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

// TestAchievementCalculationIsolation tests Property 30: Achievement Calculation Isolation
// For any user with participation in multiple groups, achievement eligibility calculations
// should only consider events and predictions within each specific group independently.
// Validates: Requirements 10.1, 10.2, 10.3, 10.4
func TestAchievementCalculationIsolation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("achievement calculations are isolated per group", prop.ForAll(
		func(userID int64, group1EventCount int, group2EventCount int) bool {
			ctx := context.Background()
			achievementRepo := newMockAchievementRepo()

			// Create separate event repos for each group
			eventRepo1 := &mockEventRepoForCreator{createdEventsCount: group1EventCount}
			eventRepo2 := &mockEventRepoForCreator{createdEventsCount: group2EventCount}

			// Create tracker for group 1
			tracker1 := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepoForAchievements{},
				eventRepo1,
				&mockLoggerForAchievements{},
			)

			// Create tracker for group 2
			tracker2 := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepoForAchievements{},
				eventRepo2,
				&mockLoggerForAchievements{},
			)

			// Check achievements for group 1
			achievements1, err := tracker1.CheckCreatorAchievements(ctx, userID, 1)
			if err != nil {
				t.Logf("Error checking achievements for group 1: %v", err)
				return false
			}

			// Check achievements for group 2
			achievements2, err := tracker2.CheckCreatorAchievements(ctx, userID, 2)
			if err != nil {
				t.Logf("Error checking achievements for group 2: %v", err)
				return false
			}

			// Verify achievements are calculated independently
			// Group 1 achievements should only depend on group1EventCount
			expectedCount1 := 0
			if group1EventCount >= EventOrganizerThreshold {
				expectedCount1++
			}
			if group1EventCount >= ActiveOrganizerThreshold {
				expectedCount1++
			}
			if group1EventCount >= MasterOrganizerThreshold {
				expectedCount1++
			}

			if len(achievements1) != expectedCount1 {
				t.Logf("Group 1: expected %d achievements for %d events, got %d",
					expectedCount1, group1EventCount, len(achievements1))
				return false
			}

			// Group 2 achievements should only depend on group2EventCount
			expectedCount2 := 0
			if group2EventCount >= EventOrganizerThreshold {
				expectedCount2++
			}
			if group2EventCount >= ActiveOrganizerThreshold {
				expectedCount2++
			}
			if group2EventCount >= MasterOrganizerThreshold {
				expectedCount2++
			}

			if len(achievements2) != expectedCount2 {
				t.Logf("Group 2: expected %d achievements for %d events, got %d",
					expectedCount2, group2EventCount, len(achievements2))
				return false
			}

			// Verify all achievements have correct group IDs
			for _, ach := range achievements1 {
				if ach.GroupID != 1 {
					t.Logf("Achievement from group 1 has wrong group ID: %d", ach.GroupID)
					return false
				}
			}

			for _, ach := range achievements2 {
				if ach.GroupID != 2 {
					t.Logf("Achievement from group 2 has wrong group ID: %d", ach.GroupID)
					return false
				}
			}

			return true
		},
		gen.Int64Range(1, 1000000), // userID
		gen.IntRange(0, 30),        // group1EventCount
		gen.IntRange(0, 30),        // group2EventCount
	))

	properties.TestingRun(t)
}

// TestIndependentAchievementEarning tests Property 31: Independent Achievement Earning
// For any user with memberships in multiple groups, earning the same achievement type
// in one group should not prevent earning it in another group.
// Validates: Requirements 10.5
func TestIndependentAchievementEarning(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("same achievement can be earned in multiple groups", prop.ForAll(
		func(userID int64, eventCount int) bool {
			ctx := context.Background()
			achievementRepo := newMockAchievementRepo()
			eventRepo := &mockEventRepoForCreator{createdEventsCount: eventCount}

			// Create tracker for group 1
			tracker1 := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepoForAchievements{},
				eventRepo,
				&mockLoggerForAchievements{},
			)

			// Create tracker for group 2
			tracker2 := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepoForAchievements{},
				eventRepo,
				&mockLoggerForAchievements{},
			)

			// Award achievements in group 1
			achievements1, err := tracker1.CheckCreatorAchievements(ctx, userID, 1)
			if err != nil {
				t.Logf("Error checking achievements for group 1: %v", err)
				return false
			}

			// Award achievements in group 2
			achievements2, err := tracker2.CheckCreatorAchievements(ctx, userID, 2)
			if err != nil {
				t.Logf("Error checking achievements for group 2: %v", err)
				return false
			}

			// Both groups should award the same achievements independently
			if len(achievements1) != len(achievements2) {
				t.Logf("Different number of achievements: group1=%d, group2=%d",
					len(achievements1), len(achievements2))
				return false
			}

			// Verify achievement codes match
			codes1 := make(map[AchievementCode]bool)
			for _, ach := range achievements1 {
				codes1[ach.Code] = true
			}

			codes2 := make(map[AchievementCode]bool)
			for _, ach := range achievements2 {
				codes2[ach.Code] = true
			}

			for code := range codes1 {
				if !codes2[code] {
					t.Logf("Achievement %s in group 1 but not in group 2", code)
					return false
				}
			}

			// Verify each achievement has the correct group ID
			for _, ach := range achievements1 {
				if ach.GroupID != 1 {
					t.Logf("Achievement in group 1 has wrong group ID: %d", ach.GroupID)
					return false
				}
			}

			for _, ach := range achievements2 {
				if ach.GroupID != 2 {
					t.Logf("Achievement in group 2 has wrong group ID: %d", ach.GroupID)
					return false
				}
			}

			// Verify achievements are stored separately in the repository
			allAchievements1, err := achievementRepo.GetUserAchievements(ctx, userID, 1)
			if err != nil {
				t.Logf("Error getting achievements for group 1: %v", err)
				return false
			}

			allAchievements2, err := achievementRepo.GetUserAchievements(ctx, userID, 2)
			if err != nil {
				t.Logf("Error getting achievements for group 2: %v", err)
				return false
			}

			if len(allAchievements1) != len(achievements1) {
				t.Logf("Stored achievements count mismatch for group 1: expected %d, got %d",
					len(achievements1), len(allAchievements1))
				return false
			}

			if len(allAchievements2) != len(achievements2) {
				t.Logf("Stored achievements count mismatch for group 2: expected %d, got %d",
					len(achievements2), len(allAchievements2))
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000), // userID
		gen.IntRange(0, 30),        // eventCount (same for both groups)
	))

	properties.TestingRun(t)
}

// TestAchievementGroupAssociation tests Property 24: Achievement-Group Association
// For any achievement earned by a user, the achievement record should contain
// the group identifier where it was earned.
// Validates: Requirements 7.2
func TestAchievementGroupAssociation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("achievements contain correct group identifier", prop.ForAll(
		func(userID int64, groupID int64, eventCount int) bool {
			// Ensure groupID is positive
			if groupID <= 0 {
				groupID = 1
			}

			ctx := context.Background()
			achievementRepo := newMockAchievementRepo()
			eventRepo := &mockEventRepoForCreator{createdEventsCount: eventCount}

			tracker := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepoForAchievements{},
				eventRepo,
				&mockLoggerForAchievements{},
			)

			// Award achievements for the specified group
			achievements, err := tracker.CheckCreatorAchievements(ctx, userID, groupID)
			if err != nil {
				t.Logf("Error checking achievements: %v", err)
				return false
			}

			// Verify every achievement has the correct group ID
			for _, ach := range achievements {
				if ach.GroupID != groupID {
					t.Logf("Achievement has wrong group ID: expected %d, got %d",
						groupID, ach.GroupID)
					return false
				}

				// Verify the achievement is stored with the correct group ID
				exists, err := achievementRepo.CheckAchievementExists(ctx, userID, groupID, ach.Code)
				if err != nil {
					t.Logf("Error checking achievement existence: %v", err)
					return false
				}

				if !exists {
					t.Logf("Achievement not found in repository: user=%d, group=%d, code=%s",
						userID, groupID, ach.Code)
					return false
				}

				// Verify the achievement is NOT stored in a different group
				differentGroupID := groupID + 1
				existsInDifferentGroup, err := achievementRepo.CheckAchievementExists(ctx, userID, differentGroupID, ach.Code)
				if err != nil {
					t.Logf("Error checking achievement in different group: %v", err)
					return false
				}

				if existsInDifferentGroup {
					t.Logf("Achievement incorrectly found in different group: user=%d, group=%d, code=%s",
						userID, differentGroupID, ach.Code)
					return false
				}
			}

			// Retrieve all achievements for the group and verify group IDs
			allAchievements, err := achievementRepo.GetUserAchievements(ctx, userID, groupID)
			if err != nil {
				t.Logf("Error getting user achievements: %v", err)
				return false
			}

			for _, ach := range allAchievements {
				if ach.GroupID != groupID {
					t.Logf("Retrieved achievement has wrong group ID: expected %d, got %d",
						groupID, ach.GroupID)
					return false
				}
			}

			return true
		},
		gen.Int64Range(1, 1000000), // userID
		gen.Int64Range(1, 100),     // groupID
		gen.IntRange(0, 30),        // eventCount
	))

	properties.TestingRun(t)
}

// TestAchievementsCalculatedPerGroup tests that achievements are calculated per-group
// Validates: Requirements 10.1
func TestAchievementsCalculatedPerGroup(t *testing.T) {
	ctx := context.Background()
	achievementRepo := newMockAchievementRepo()

	// User has 5 events in group 1 (should get Event Organizer and Active Organizer)
	eventRepo1 := &mockEventRepoForCreator{createdEventsCount: 5}
	tracker1 := NewAchievementTracker(
		achievementRepo,
		&mockRatingRepo{},
		&mockPredictionRepoForAchievements{},
		eventRepo1,
		&mockLoggerForAchievements{},
	)

	// User has 1 event in group 2 (should get only Event Organizer)
	eventRepo2 := &mockEventRepoForCreator{createdEventsCount: 1}
	tracker2 := NewAchievementTracker(
		achievementRepo,
		&mockRatingRepo{},
		&mockPredictionRepoForAchievements{},
		eventRepo2,
		&mockLoggerForAchievements{},
	)

	userID := int64(12345)

	// Check achievements for group 1
	achievements1, err := tracker1.CheckCreatorAchievements(ctx, userID, 1)
	if err != nil {
		t.Fatalf("Error checking achievements for group 1: %v", err)
	}

	if len(achievements1) != 2 {
		t.Errorf("Expected 2 achievements for group 1, got %d", len(achievements1))
	}

	// Check achievements for group 2
	achievements2, err := tracker2.CheckCreatorAchievements(ctx, userID, 2)
	if err != nil {
		t.Fatalf("Error checking achievements for group 2: %v", err)
	}

	if len(achievements2) != 1 {
		t.Errorf("Expected 1 achievement for group 2, got %d", len(achievements2))
	}

	// Verify achievements are stored separately
	storedAchievements1, err := achievementRepo.GetUserAchievements(ctx, userID, 1)
	if err != nil {
		t.Fatalf("Error getting achievements for group 1: %v", err)
	}

	if len(storedAchievements1) != 2 {
		t.Errorf("Expected 2 stored achievements for group 1, got %d", len(storedAchievements1))
	}

	storedAchievements2, err := achievementRepo.GetUserAchievements(ctx, userID, 2)
	if err != nil {
		t.Fatalf("Error getting achievements for group 2: %v", err)
	}

	if len(storedAchievements2) != 1 {
		t.Errorf("Expected 1 stored achievement for group 2, got %d", len(storedAchievements2))
	}
}

// TestEventCountsPerGroup tests that event counts are tracked per-group
// Validates: Requirements 10.2
func TestEventCountsPerGroup(t *testing.T) {
	ctx := context.Background()
	achievementRepo := newMockAchievementRepo()
	userID := int64(12345)

	testCases := []struct {
		name         string
		groupID      int64
		eventCount   int
		expectedAchs int
	}{
		{"group 1 with 1 event", 1, 1, 1},
		{"group 2 with 5 events", 2, 5, 2},
		{"group 3 with 25 events", 3, 25, 3},
		{"group 4 with 0 events", 4, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			eventRepo := &mockEventRepoForCreator{createdEventsCount: tc.eventCount}
			tracker := NewAchievementTracker(
				achievementRepo,
				&mockRatingRepo{},
				&mockPredictionRepoForAchievements{},
				eventRepo,
				&mockLoggerForAchievements{},
			)

			achievements, err := tracker.CheckCreatorAchievements(ctx, userID, tc.groupID)
			if err != nil {
				t.Fatalf("Error checking achievements: %v", err)
			}

			if len(achievements) != tc.expectedAchs {
				t.Errorf("Expected %d achievements, got %d", tc.expectedAchs, len(achievements))
			}

			// Verify all achievements have correct group ID
			for _, ach := range achievements {
				if ach.GroupID != tc.groupID {
					t.Errorf("Achievement has wrong group ID: expected %d, got %d", tc.groupID, ach.GroupID)
				}
			}
		})
	}
}

// TestSameAchievementInMultipleGroups tests that the same achievement can be earned in multiple groups
// Validates: Requirements 10.5
func TestSameAchievementInMultipleGroups(t *testing.T) {
	ctx := context.Background()
	achievementRepo := newMockAchievementRepo()
	userID := int64(12345)

	// Create trackers for 3 different groups, all with 5 events
	groups := []int64{1, 2, 3}
	for _, groupID := range groups {
		eventRepo := &mockEventRepoForCreator{createdEventsCount: 5}
		tracker := NewAchievementTracker(
			achievementRepo,
			&mockRatingRepo{},
			&mockPredictionRepoForAchievements{},
			eventRepo,
			&mockLoggerForAchievements{},
		)

		achievements, err := tracker.CheckCreatorAchievements(ctx, userID, groupID)
		if err != nil {
			t.Fatalf("Error checking achievements for group %d: %v", groupID, err)
		}

		// Should get Event Organizer and Active Organizer in each group
		if len(achievements) != 2 {
			t.Errorf("Group %d: expected 2 achievements, got %d", groupID, len(achievements))
		}

		// Verify achievement codes
		expectedCodes := map[AchievementCode]bool{
			AchievementEventOrganizer:  true,
			AchievementActiveOrganizer: true,
		}

		for _, ach := range achievements {
			if !expectedCodes[ach.Code] {
				t.Errorf("Group %d: unexpected achievement code %s", groupID, ach.Code)
			}
			if ach.GroupID != groupID {
				t.Errorf("Group %d: achievement has wrong group ID %d", groupID, ach.GroupID)
			}
		}
	}

	// Verify each group has its own set of achievements
	for _, groupID := range groups {
		storedAchievements, err := achievementRepo.GetUserAchievements(ctx, userID, groupID)
		if err != nil {
			t.Fatalf("Error getting achievements for group %d: %v", groupID, err)
		}

		if len(storedAchievements) != 2 {
			t.Errorf("Group %d: expected 2 stored achievements, got %d", groupID, len(storedAchievements))
		}

		// Verify all stored achievements have correct group ID
		for _, ach := range storedAchievements {
			if ach.GroupID != groupID {
				t.Errorf("Group %d: stored achievement has wrong group ID %d", groupID, ach.GroupID)
			}
		}
	}
}
