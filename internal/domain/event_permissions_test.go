package domain

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Mock EventRepository for permission testing
type mockEventRepoForPermissions struct {
	events map[int64]*Event
}

func (m *mockEventRepoForPermissions) CreateEvent(ctx context.Context, event *Event) error {
	if m.events == nil {
		m.events = make(map[int64]*Event)
	}
	m.events[event.ID] = event
	return nil
}

func (m *mockEventRepoForPermissions) GetEvent(ctx context.Context, eventID int64) (*Event, error) {
	if m.events == nil {
		return nil, ErrEventNotFound
	}
	event, ok := m.events[eventID]
	if !ok {
		return nil, ErrEventNotFound
	}
	return event, nil
}

func (m *mockEventRepoForPermissions) GetEventByPollID(ctx context.Context, pollID string) (*Event, error) {
	return nil, nil
}

func (m *mockEventRepoForPermissions) GetActiveEvents(ctx context.Context) ([]*Event, error) {
	return nil, nil
}

func (m *mockEventRepoForPermissions) GetResolvedEvents(ctx context.Context) ([]*Event, error) {
	return nil, nil
}

func (m *mockEventRepoForPermissions) UpdateEvent(ctx context.Context, event *Event) error {
	return nil
}

func (m *mockEventRepoForPermissions) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	return nil
}

func (m *mockEventRepoForPermissions) GetUserCreatedEventsCount(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

// TestNonAuthorizedResolutionRejection tests: Non-authorized resolution rejection
func TestNonAuthorizedResolutionRejection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("non-creator non-admin users cannot manage events", prop.ForAll(
		func(creatorID int64, eventID int64, unauthorizedUserID int64, adminID int64) bool {
			// Ensure unauthorized user is neither creator nor admin
			if unauthorizedUserID == creatorID || unauthorizedUserID == adminID {
				return true // Skip this case
			}

			// Create mock repository with an event
			mockRepo := &mockEventRepoForPermissions{
				events: map[int64]*Event{
					eventID: {
						ID:        eventID,
						Question:  "Test question",
						Options:   []string{"Yes", "No"},
						CreatedAt: time.Now(),
						Deadline:  time.Now().Add(24 * time.Hour),
						Status:    EventStatusActive,
						EventType: EventTypeBinary,
						CreatedBy: creatorID,
					},
				},
			}

			validator := NewEventPermissionValidator(
				mockRepo,
				&mockPredictionRepo{},
				3,
				&mockLogger{},
			)

			// Check if unauthorized user can manage the event
			canManage, err := validator.CanManageEvent(
				context.Background(),
				unauthorizedUserID,
				eventID,
				[]int64{adminID},
			)

			if err != nil {
				t.Logf("Unexpected error: %v", err)
				return false
			}

			// Unauthorized user should NOT be able to manage the event
			if canManage {
				t.Logf("Unauthorized user %d was allowed to manage event %d (creator: %d, admin: %d)",
					unauthorizedUserID, eventID, creatorID, adminID)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),  // creatorID
		gen.Int64Range(1, 1000),     // eventID
		gen.Int64Range(1, 1000000),  // unauthorizedUserID
		gen.Int64Range(1000001, 2000000), // adminID (different range to avoid collision)
	))

	properties.TestingRun(t)
}

// TestCreatorResolutionAuthorization tests: Creator resolution authorization
func TestCreatorResolutionAuthorization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("event creator can always manage their events", prop.ForAll(
		func(creatorID int64, eventID int64, adminID int64) bool {
			// Ensure creator is not the admin (to test creator-specific authorization)
			if creatorID == adminID {
				return true // Skip this case
			}

			// Create mock repository with an event
			mockRepo := &mockEventRepoForPermissions{
				events: map[int64]*Event{
					eventID: {
						ID:        eventID,
						Question:  "Test question",
						Options:   []string{"Yes", "No"},
						CreatedAt: time.Now(),
						Deadline:  time.Now().Add(24 * time.Hour),
						Status:    EventStatusActive,
						EventType: EventTypeBinary,
						CreatedBy: creatorID,
					},
				},
			}

			validator := NewEventPermissionValidator(
				mockRepo,
				&mockPredictionRepo{},
				3,
				&mockLogger{},
			)

			// Check if creator can manage their event
			canManage, err := validator.CanManageEvent(
				context.Background(),
				creatorID,
				eventID,
				[]int64{adminID},
			)

			if err != nil {
				t.Logf("Unexpected error: %v", err)
				return false
			}

			// Creator should ALWAYS be able to manage their event
			if !canManage {
				t.Logf("Creator %d was not allowed to manage their event %d", creatorID, eventID)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),       // creatorID
		gen.Int64Range(1, 1000),          // eventID
		gen.Int64Range(1000001, 2000000), // adminID (different range)
	))

	properties.TestingRun(t)
}

// TestAdministratorResolutionAuthorization tests: Administrator resolution authorization
func TestAdministratorResolutionAuthorization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("administrators can manage any event", prop.ForAll(
		func(creatorID int64, eventID int64, adminID int64) bool {
			// Ensure admin is not the creator (to test admin-specific authorization)
			if adminID == creatorID {
				return true // Skip this case
			}

			// Create mock repository with an event
			mockRepo := &mockEventRepoForPermissions{
				events: map[int64]*Event{
					eventID: {
						ID:        eventID,
						Question:  "Test question",
						Options:   []string{"Yes", "No"},
						CreatedAt: time.Now(),
						Deadline:  time.Now().Add(24 * time.Hour),
						Status:    EventStatusActive,
						EventType: EventTypeBinary,
						CreatedBy: creatorID,
					},
				},
			}

			validator := NewEventPermissionValidator(
				mockRepo,
				&mockPredictionRepo{},
				3,
				&mockLogger{},
			)

			// Check if admin can manage the event
			canManage, err := validator.CanManageEvent(
				context.Background(),
				adminID,
				eventID,
				[]int64{adminID},
			)

			if err != nil {
				t.Logf("Unexpected error: %v", err)
				return false
			}

			// Admin should ALWAYS be able to manage any event
			if !canManage {
				t.Logf("Admin %d was not allowed to manage event %d (creator: %d)", adminID, eventID, creatorID)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),       // creatorID
		gen.Int64Range(1, 1000),          // eventID
		gen.Int64Range(1000001, 2000000), // adminID (different range)
	))

	properties.TestingRun(t)
}

// TestInsufficientParticipationRejection tests: Insufficient participation rejection
func TestInsufficientParticipationRejection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("users with insufficient participation cannot create events", prop.ForAll(
		func(userID int64, participationCount int, minRequired int, adminID int64) bool {
			// Ensure user is not admin
			if userID == adminID {
				return true // Skip this case
			}

			// Ensure participation is insufficient
			if participationCount >= minRequired {
				return true // Skip this case
			}

			// Create mock repository with participation count
			mockPredRepo := &mockPredictionRepo{
				completedEventCount: participationCount,
			}

			validator := NewEventPermissionValidator(
				&mockEventRepoForPermissions{},
				mockPredRepo,
				minRequired,
				&mockLogger{},
			)

			// Check if user can create event
			canCreate, count, err := validator.CanCreateEvent(
				context.Background(),
				userID,
				[]int64{adminID},
			)

			if err != nil {
				t.Logf("Unexpected error: %v", err)
				return false
			}

			// Verify count matches
			if count != participationCount {
				t.Logf("Expected count %d, got %d", participationCount, count)
				return false
			}

			// User with insufficient participation should NOT be able to create events
			if canCreate {
				t.Logf("User %d with %d participation (required: %d) was allowed to create event",
					userID, participationCount, minRequired)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),       // userID
		gen.IntRange(0, 10),              // participationCount (low)
		gen.IntRange(3, 20),              // minRequired (higher than participationCount)
		gen.Int64Range(1000001, 2000000), // adminID (different range)
	))

	properties.TestingRun(t)
}

// TestSufficientParticipationAuthorization tests: Sufficient participation authorization
func TestSufficientParticipationAuthorization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("users with sufficient participation can create events", prop.ForAll(
		func(userID int64, minRequired int, extraParticipation int, adminID int64) bool {
			// Ensure user is not admin
			if userID == adminID {
				return true // Skip this case
			}

			// Calculate participation count that meets or exceeds requirement
			participationCount := minRequired + extraParticipation

			// Create mock repository with participation count
			mockPredRepo := &mockPredictionRepo{
				completedEventCount: participationCount,
			}

			validator := NewEventPermissionValidator(
				&mockEventRepoForPermissions{},
				mockPredRepo,
				minRequired,
				&mockLogger{},
			)

			// Check if user can create event
			canCreate, count, err := validator.CanCreateEvent(
				context.Background(),
				userID,
				[]int64{adminID},
			)

			if err != nil {
				t.Logf("Unexpected error: %v", err)
				return false
			}

			// Verify count matches
			if count != participationCount {
				t.Logf("Expected count %d, got %d", participationCount, count)
				return false
			}

			// User with sufficient participation should be able to create events
			if !canCreate {
				t.Logf("User %d with %d participation (required: %d) was not allowed to create event",
					userID, participationCount, minRequired)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),       // userID
		gen.IntRange(1, 10),              // minRequired
		gen.IntRange(0, 50),              // extraParticipation
		gen.Int64Range(1000001, 2000000), // adminID (different range)
	))

	properties.TestingRun(t)
}

// Unit tests for EventPermissionValidator

// TestIsEventCreator_Success tests creator identification
func TestIsEventCreator_Success(t *testing.T) {
	creatorID := int64(12345)
	eventID := int64(1)

	mockRepo := &mockEventRepoForPermissions{
		events: map[int64]*Event{
			eventID: {
				ID:        eventID,
				Question:  "Test question",
				Options:   []string{"Yes", "No"},
				CreatedAt: time.Now(),
				Deadline:  time.Now().Add(24 * time.Hour),
				Status:    EventStatusActive,
				EventType: EventTypeBinary,
				CreatedBy: creatorID,
			},
		},
	}

	validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, 3, &mockLogger{})

	isCreator, err := validator.IsEventCreator(context.Background(), creatorID, eventID)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !isCreator {
		t.Errorf("Expected user %d to be creator of event %d", creatorID, eventID)
	}
}

// TestIsEventCreator_NotCreator tests non-creator identification
func TestIsEventCreator_NotCreator(t *testing.T) {
	creatorID := int64(12345)
	otherUserID := int64(67890)
	eventID := int64(1)

	mockRepo := &mockEventRepoForPermissions{
		events: map[int64]*Event{
			eventID: {
				ID:        eventID,
				Question:  "Test question",
				Options:   []string{"Yes", "No"},
				CreatedAt: time.Now(),
				Deadline:  time.Now().Add(24 * time.Hour),
				Status:    EventStatusActive,
				EventType: EventTypeBinary,
				CreatedBy: creatorID,
			},
		},
	}

	validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, 3, &mockLogger{})

	isCreator, err := validator.IsEventCreator(context.Background(), otherUserID, eventID)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if isCreator {
		t.Errorf("Expected user %d to NOT be creator of event %d", otherUserID, eventID)
	}
}

// TestIsEventCreator_EventNotFound tests error handling
func TestIsEventCreator_EventNotFound(t *testing.T) {
	mockRepo := &mockEventRepoForPermissions{
		events: map[int64]*Event{},
	}

	validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, 3, &mockLogger{})

	_, err := validator.IsEventCreator(context.Background(), 12345, 999)
	if err == nil {
		t.Fatal("Expected error for non-existent event, got nil")
	}
}

// TestIsAdmin tests admin identification
func TestIsAdmin(t *testing.T) {
	testCases := []struct {
		name     string
		userID   int64
		adminIDs []int64
		expected bool
	}{
		{
			name:     "user is admin",
			userID:   12345,
			adminIDs: []int64{12345, 67890},
			expected: true,
		},
		{
			name:     "user is not admin",
			userID:   11111,
			adminIDs: []int64{12345, 67890},
			expected: false,
		},
		{
			name:     "empty admin list",
			userID:   12345,
			adminIDs: []int64{},
			expected: false,
		},
		{
			name:     "single admin match",
			userID:   12345,
			adminIDs: []int64{12345},
			expected: true,
		},
	}

	validator := NewEventPermissionValidator(&mockEventRepoForPermissions{}, &mockPredictionRepo{}, 3, &mockLogger{})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.IsAdmin(tc.userID, tc.adminIDs)
			if result != tc.expected {
				t.Errorf("Expected IsAdmin(%d, %v) = %v, got %v", tc.userID, tc.adminIDs, tc.expected, result)
			}
		})
	}
}

// TestCanManageEvent_VariousScenarios tests various permission scenarios
func TestCanManageEvent_VariousScenarios(t *testing.T) {
	creatorID := int64(12345)
	adminID := int64(67890)
	otherUserID := int64(11111)
	eventID := int64(1)

	mockRepo := &mockEventRepoForPermissions{
		events: map[int64]*Event{
			eventID: {
				ID:        eventID,
				Question:  "Test question",
				Options:   []string{"Yes", "No"},
				CreatedAt: time.Now(),
				Deadline:  time.Now().Add(24 * time.Hour),
				Status:    EventStatusActive,
				EventType: EventTypeBinary,
				CreatedBy: creatorID,
			},
		},
	}

	validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, 3, &mockLogger{})

	testCases := []struct {
		name     string
		userID   int64
		adminIDs []int64
		expected bool
	}{
		{
			name:     "creator can manage",
			userID:   creatorID,
			adminIDs: []int64{adminID},
			expected: true,
		},
		{
			name:     "admin can manage",
			userID:   adminID,
			adminIDs: []int64{adminID},
			expected: true,
		},
		{
			name:     "other user cannot manage",
			userID:   otherUserID,
			adminIDs: []int64{adminID},
			expected: false,
		},
		{
			name:     "creator who is also admin can manage",
			userID:   creatorID,
			adminIDs: []int64{creatorID, adminID},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canManage, err := validator.CanManageEvent(context.Background(), tc.userID, eventID, tc.adminIDs)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if canManage != tc.expected {
				t.Errorf("Expected CanManageEvent = %v, got %v", tc.expected, canManage)
			}
		})
	}
}

// TestCanCreateEvent_VariousScenarios tests various creation permission scenarios
func TestCanCreateEvent_VariousScenarios(t *testing.T) {
	adminID := int64(67890)
	userID := int64(12345)

	testCases := []struct {
		name                string
		userID              int64
		adminIDs            []int64
		participationCount  int
		minRequired         int
		expectedCanCreate   bool
		expectedCount       int
	}{
		{
			name:                "admin can always create",
			userID:              adminID,
			adminIDs:            []int64{adminID},
			participationCount:  0,
			minRequired:         3,
			expectedCanCreate:   true,
			expectedCount:       0,
		},
		{
			name:                "user with sufficient participation",
			userID:              userID,
			adminIDs:            []int64{adminID},
			participationCount:  5,
			minRequired:         3,
			expectedCanCreate:   true,
			expectedCount:       5,
		},
		{
			name:                "user with insufficient participation",
			userID:              userID,
			adminIDs:            []int64{adminID},
			participationCount:  2,
			minRequired:         3,
			expectedCanCreate:   false,
			expectedCount:       2,
		},
		{
			name:                "user with exact participation",
			userID:              userID,
			adminIDs:            []int64{adminID},
			participationCount:  3,
			minRequired:         3,
			expectedCanCreate:   true,
			expectedCount:       3,
		},
		{
			name:                "user with zero participation",
			userID:              userID,
			adminIDs:            []int64{adminID},
			participationCount:  0,
			minRequired:         3,
			expectedCanCreate:   false,
			expectedCount:       0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPredRepo := &mockPredictionRepo{
				completedEventCount: tc.participationCount,
			}

			validator := NewEventPermissionValidator(&mockEventRepoForPermissions{}, mockPredRepo, tc.minRequired, &mockLogger{})

			canCreate, count, err := validator.CanCreateEvent(context.Background(), tc.userID, tc.adminIDs)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if canCreate != tc.expectedCanCreate {
				t.Errorf("Expected CanCreateEvent = %v, got %v", tc.expectedCanCreate, canCreate)
			}

			if count != tc.expectedCount {
				t.Errorf("Expected count = %d, got %d", tc.expectedCount, count)
			}
		})
	}
}
