package domain

import (
	"context"
	"fmt"
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

func (m *mockEventRepoForPermissions) GetActiveEvents(ctx context.Context, groupID int64) ([]*Event, error) {
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

func (m *mockEventRepoForPermissions) GetUserCreatedEventsCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	return 0, nil
}

func (m *mockEventRepoForPermissions) GetEventsByDeadlineRange(ctx context.Context, start, end time.Time) ([]*Event, error) {
	return nil, nil
}

// Mock GroupMembershipRepository for permission testing
type mockGroupMembershipRepoForPermissions struct {
	memberships map[string]bool // key: "groupID_userID"
}

func (m *mockGroupMembershipRepoForPermissions) CreateMembership(ctx context.Context, membership *GroupMembership) error {
	return nil
}

func (m *mockGroupMembershipRepoForPermissions) GetMembership(ctx context.Context, groupID int64, userID int64) (*GroupMembership, error) {
	return nil, nil
}

func (m *mockGroupMembershipRepoForPermissions) GetGroupMembers(ctx context.Context, groupID int64) ([]*GroupMembership, error) {
	return nil, nil
}

func (m *mockGroupMembershipRepoForPermissions) UpdateMembershipStatus(ctx context.Context, groupID int64, userID int64, status MembershipStatus) error {
	return nil
}

func (m *mockGroupMembershipRepoForPermissions) HasActiveMembership(ctx context.Context, groupID int64, userID int64) (bool, error) {
	if m.memberships == nil {
		return false, nil
	}
	key := formatMembershipKey(groupID, userID)
	return m.memberships[key], nil
}

func formatMembershipKey(groupID int64, userID int64) string {
	return fmt.Sprintf("%d_%d", groupID, userID)
}

// TestNonAuthorizedResolutionRejection tests: Non-authorized resolution rejection
func TestNonAuthorizedResolutionRejection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("non-creator non-admin users cannot manage events", prop.ForAll(
		func(creatorID int64, eventID int64, unauthorizedUserID int64, adminID int64, groupID int64) bool {
			// Ensure unauthorized user is neither creator nor admin
			if unauthorizedUserID == creatorID || unauthorizedUserID == adminID {
				return true // Skip this case
			}

			// Create mock repository with an event
			mockRepo := &mockEventRepoForPermissions{
				events: map[int64]*Event{
					eventID: {
						ID:        eventID,
						GroupID:   groupID,
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

			// Mock membership repo - unauthorized user has no membership
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: map[string]bool{},
			}

			validator := NewEventPermissionValidator(
				mockRepo,
				&mockPredictionRepo{},
				mockMembershipRepo,
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
		gen.Int64Range(1, 1000000),       // creatorID
		gen.Int64Range(1, 1000),          // eventID
		gen.Int64Range(1, 1000000),       // unauthorizedUserID
		gen.Int64Range(1000001, 2000000), // adminID (different range to avoid collision)
		gen.Int64Range(1, 100),           // groupID
	))

	properties.TestingRun(t)
}

// TestCreatorResolutionAuthorization tests: Creator resolution authorization
func TestCreatorResolutionAuthorization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("event creator can always manage their events", prop.ForAll(
		func(creatorID int64, eventID int64, adminID int64, groupID int64) bool {
			// Ensure creator is not the admin (to test creator-specific authorization)
			if creatorID == adminID {
				return true // Skip this case
			}

			// Create mock repository with an event
			mockRepo := &mockEventRepoForPermissions{
				events: map[int64]*Event{
					eventID: {
						ID:        eventID,
						GroupID:   groupID,
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

			// Mock membership repo - creator has membership
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: map[string]bool{
					formatMembershipKey(groupID, creatorID): true,
				},
			}

			validator := NewEventPermissionValidator(
				mockRepo,
				&mockPredictionRepo{},
				mockMembershipRepo,
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
		gen.Int64Range(1, 100),           // groupID
	))

	properties.TestingRun(t)
}

// TestAdministratorResolutionAuthorization tests: Administrator resolution authorization
func TestAdministratorResolutionAuthorization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("administrators can manage any event", prop.ForAll(
		func(creatorID int64, eventID int64, adminID int64, groupID int64) bool {
			// Ensure admin is not the creator (to test admin-specific authorization)
			if adminID == creatorID {
				return true // Skip this case
			}

			// Create mock repository with an event
			mockRepo := &mockEventRepoForPermissions{
				events: map[int64]*Event{
					eventID: {
						ID:        eventID,
						GroupID:   groupID,
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

			// Mock membership repo - admin has membership
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: map[string]bool{
					formatMembershipKey(groupID, adminID): true,
				},
			}

			validator := NewEventPermissionValidator(
				mockRepo,
				&mockPredictionRepo{},
				mockMembershipRepo,
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
		gen.Int64Range(1, 100),           // groupID
	))

	properties.TestingRun(t)
}

// TestInsufficientParticipationRejection tests: Insufficient participation rejection
func TestInsufficientParticipationRejection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("users with insufficient participation cannot create events", prop.ForAll(
		func(userID int64, participationCount int, minRequired int, adminID int64, groupID int64) bool {
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

			// Mock membership repo - user has membership
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: map[string]bool{
					formatMembershipKey(groupID, userID): true,
				},
			}

			validator := NewEventPermissionValidator(
				&mockEventRepoForPermissions{},
				mockPredRepo,
				mockMembershipRepo,
				minRequired,
				&mockLogger{},
			)

			// Check if user can create event
			canCreate, count, err := validator.CanCreateEvent(
				context.Background(),
				userID,
				groupID,
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
		gen.Int64Range(1, 100),           // groupID
	))

	properties.TestingRun(t)
}

// TestSufficientParticipationAuthorization tests: Sufficient participation authorization
func TestSufficientParticipationAuthorization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("users with sufficient participation can create events", prop.ForAll(
		func(userID int64, minRequired int, extraParticipation int, adminID int64, groupID int64) bool {
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

			// Mock membership repo - user has membership
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: map[string]bool{
					formatMembershipKey(groupID, userID): true,
				},
			}

			validator := NewEventPermissionValidator(
				&mockEventRepoForPermissions{},
				mockPredRepo,
				mockMembershipRepo,
				minRequired,
				&mockLogger{},
			)

			// Check if user can create event
			canCreate, count, err := validator.CanCreateEvent(
				context.Background(),
				userID,
				groupID,
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
		gen.Int64Range(1, 100),           // groupID
	))

	properties.TestingRun(t)
}

// Unit tests for EventPermissionValidator

// TestIsEventCreator_Success tests creator identification
func TestIsEventCreator_Success(t *testing.T) {
	creatorID := int64(12345)
	eventID := int64(1)
	groupID := int64(1)

	mockRepo := &mockEventRepoForPermissions{
		events: map[int64]*Event{
			eventID: {
				ID:        eventID,
				GroupID:   groupID,
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

	mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
		memberships: map[string]bool{},
	}

	validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, mockMembershipRepo, 3, &mockLogger{})

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
	groupID := int64(1)

	mockRepo := &mockEventRepoForPermissions{
		events: map[int64]*Event{
			eventID: {
				ID:        eventID,
				GroupID:   groupID,
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

	mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
		memberships: map[string]bool{},
	}

	validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, mockMembershipRepo, 3, &mockLogger{})

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

	mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
		memberships: map[string]bool{},
	}

	validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, mockMembershipRepo, 3, &mockLogger{})

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

	mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
		memberships: map[string]bool{},
	}

	validator := NewEventPermissionValidator(&mockEventRepoForPermissions{}, &mockPredictionRepo{}, mockMembershipRepo, 3, &mockLogger{})

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
	groupID := int64(1)

	mockRepo := &mockEventRepoForPermissions{
		events: map[int64]*Event{
			eventID: {
				ID:        eventID,
				GroupID:   groupID,
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

	testCases := []struct {
		name        string
		userID      int64
		adminIDs    []int64
		memberships map[string]bool
		expected    bool
	}{
		{
			name:     "creator can manage",
			userID:   creatorID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, creatorID): true,
			},
			expected: true,
		},
		{
			name:     "admin can manage",
			userID:   adminID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, adminID): true,
			},
			expected: true,
		},
		{
			name:     "other user cannot manage",
			userID:   otherUserID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, otherUserID): true,
			},
			expected: false,
		},
		{
			name:     "creator who is also admin can manage",
			userID:   creatorID,
			adminIDs: []int64{creatorID, adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, creatorID): true,
			},
			expected: true,
		},
		{
			name:        "creator without membership cannot manage",
			userID:      creatorID,
			adminIDs:    []int64{adminID},
			memberships: map[string]bool{},
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: tc.memberships,
			}

			validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, mockMembershipRepo, 3, &mockLogger{})

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
	groupID := int64(1)

	testCases := []struct {
		name               string
		userID             int64
		adminIDs           []int64
		participationCount int
		minRequired        int
		memberships        map[string]bool
		expectedCanCreate  bool
		expectedCount      int
	}{
		{
			name:     "admin can always create",
			userID:   adminID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, adminID): true,
			},
			participationCount: 0,
			minRequired:        3,
			expectedCanCreate:  true,
			expectedCount:      0,
		},
		{
			name:     "user with sufficient participation",
			userID:   userID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, userID): true,
			},
			participationCount: 5,
			minRequired:        3,
			expectedCanCreate:  true,
			expectedCount:      5,
		},
		{
			name:     "user with insufficient participation",
			userID:   userID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, userID): true,
			},
			participationCount: 2,
			minRequired:        3,
			expectedCanCreate:  false,
			expectedCount:      2,
		},
		{
			name:     "user with exact participation",
			userID:   userID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, userID): true,
			},
			participationCount: 3,
			minRequired:        3,
			expectedCanCreate:  true,
			expectedCount:      3,
		},
		{
			name:     "user with zero participation",
			userID:   userID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, userID): true,
			},
			participationCount: 0,
			minRequired:        3,
			expectedCanCreate:  false,
			expectedCount:      0,
		},
		{
			name:               "user without membership cannot create",
			userID:             userID,
			adminIDs:           []int64{adminID},
			memberships:        map[string]bool{},
			participationCount: 10,
			minRequired:        3,
			expectedCanCreate:  false,
			expectedCount:      0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPredRepo := &mockPredictionRepo{
				completedEventCount: tc.participationCount,
			}

			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: tc.memberships,
			}

			validator := NewEventPermissionValidator(&mockEventRepoForPermissions{}, mockPredRepo, mockMembershipRepo, tc.minRequired, &mockLogger{})

			canCreate, count, err := validator.CanCreateEvent(context.Background(), tc.userID, groupID, tc.adminIDs)
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

// TestAccessRevocationAfterRemoval tests Property 19: Access Revocation After Removal
// For any user removed from a group, attempts to access group events, ratings, or achievements should be rejected
func TestAccessRevocationAfterRemoval(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("removed users cannot access group events", prop.ForAll(
		func(userID int64, eventID int64, groupID int64, adminID int64) bool {
			// Ensure user is not admin
			if userID == adminID {
				return true // Skip this case
			}

			// Create mock repository with an event
			mockRepo := &mockEventRepoForPermissions{
				events: map[int64]*Event{
					eventID: {
						ID:        eventID,
						GroupID:   groupID,
						Question:  "Test question",
						Options:   []string{"Yes", "No"},
						CreatedAt: time.Now(),
						Deadline:  time.Now().Add(24 * time.Hour),
						Status:    EventStatusActive,
						EventType: EventTypeBinary,
						CreatedBy: userID,
					},
				},
			}

			// Mock membership repo - user has NO membership (simulating removal)
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: map[string]bool{},
			}

			validator := NewEventPermissionValidator(
				mockRepo,
				&mockPredictionRepo{},
				mockMembershipRepo,
				3,
				&mockLogger{},
			)

			// Test 1: User cannot manage their own event after removal
			canManage, err := validator.CanManageEvent(
				context.Background(),
				userID,
				eventID,
				[]int64{adminID},
			)

			if err != nil {
				t.Logf("Unexpected error checking manage permission: %v", err)
				return false
			}

			if canManage {
				t.Logf("Removed user %d was allowed to manage event %d in group %d",
					userID, eventID, groupID)
				return false
			}

			// Test 2: User cannot create events in the group after removal
			canCreate, _, err := validator.CanCreateEvent(
				context.Background(),
				userID,
				groupID,
				[]int64{adminID},
			)

			if err != nil {
				t.Logf("Unexpected error checking create permission: %v", err)
				return false
			}

			if canCreate {
				t.Logf("Removed user %d was allowed to create event in group %d",
					userID, groupID)
				return false
			}

			// Test 3: User does not have group membership
			hasMembership, err := validator.HasGroupMembership(
				context.Background(),
				userID,
				groupID,
			)

			if err != nil {
				t.Logf("Unexpected error checking membership: %v", err)
				return false
			}

			if hasMembership {
				t.Logf("Removed user %d still has membership in group %d",
					userID, groupID)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),       // userID
		gen.Int64Range(1, 1000),          // eventID
		gen.Int64Range(1, 100),           // groupID
		gen.Int64Range(1000001, 2000000), // adminID (different range)
	))

	properties.TestingRun(t)
}

// Unit tests for group membership validation

// TestCanManageEvent_RequiresGroupMembership tests that event management requires group membership
func TestCanManageEvent_RequiresGroupMembership(t *testing.T) {
	creatorID := int64(12345)
	adminID := int64(67890)
	eventID := int64(1)
	groupID := int64(1)

	mockRepo := &mockEventRepoForPermissions{
		events: map[int64]*Event{
			eventID: {
				ID:        eventID,
				GroupID:   groupID,
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

	testCases := []struct {
		name        string
		userID      int64
		adminIDs    []int64
		memberships map[string]bool
		expected    bool
		description string
	}{
		{
			name:     "creator with membership can manage",
			userID:   creatorID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, creatorID): true,
			},
			expected:    true,
			description: "Event creator with active membership should be able to manage event",
		},
		{
			name:        "creator without membership cannot manage",
			userID:      creatorID,
			adminIDs:    []int64{adminID},
			memberships: map[string]bool{},
			expected:    false,
			description: "Event creator without active membership should NOT be able to manage event",
		},
		{
			name:     "admin with membership can manage",
			userID:   adminID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, adminID): true,
			},
			expected:    true,
			description: "Admin with active membership should be able to manage event",
		},
		{
			name:        "admin without membership cannot manage",
			userID:      adminID,
			adminIDs:    []int64{adminID},
			memberships: map[string]bool{},
			expected:    false,
			description: "Admin without active membership should NOT be able to manage event",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: tc.memberships,
			}

			validator := NewEventPermissionValidator(mockRepo, &mockPredictionRepo{}, mockMembershipRepo, 3, &mockLogger{})

			canManage, err := validator.CanManageEvent(context.Background(), tc.userID, eventID, tc.adminIDs)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if canManage != tc.expected {
				t.Errorf("%s: Expected CanManageEvent = %v, got %v", tc.description, tc.expected, canManage)
			}
		})
	}
}

// TestCanCreateEvent_RequiresGroupMembership tests that event creation requires group membership
func TestCanCreateEvent_RequiresGroupMembership(t *testing.T) {
	userID := int64(12345)
	adminID := int64(67890)
	groupID := int64(1)

	testCases := []struct {
		name               string
		userID             int64
		adminIDs           []int64
		participationCount int
		minRequired        int
		memberships        map[string]bool
		expectedCanCreate  bool
		expectedCount      int
		description        string
	}{
		{
			name:     "user with membership and sufficient participation can create",
			userID:   userID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, userID): true,
			},
			participationCount: 5,
			minRequired:        3,
			expectedCanCreate:  true,
			expectedCount:      5,
			description:        "User with membership and sufficient participation should be able to create event",
		},
		{
			name:               "user without membership cannot create even with sufficient participation",
			userID:             userID,
			adminIDs:           []int64{adminID},
			memberships:        map[string]bool{},
			participationCount: 10,
			minRequired:        3,
			expectedCanCreate:  false,
			expectedCount:      0,
			description:        "User without membership should NOT be able to create event even with sufficient participation",
		},
		{
			name:     "admin with membership can create",
			userID:   adminID,
			adminIDs: []int64{adminID},
			memberships: map[string]bool{
				formatMembershipKey(groupID, adminID): true,
			},
			participationCount: 0,
			minRequired:        3,
			expectedCanCreate:  true,
			expectedCount:      0,
			description:        "Admin with membership should be able to create event",
		},
		{
			name:               "admin without membership cannot create",
			userID:             adminID,
			adminIDs:           []int64{adminID},
			memberships:        map[string]bool{},
			participationCount: 0,
			minRequired:        3,
			expectedCanCreate:  false,
			expectedCount:      0,
			description:        "Admin without membership should NOT be able to create event",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPredRepo := &mockPredictionRepo{
				completedEventCount: tc.participationCount,
			}

			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: tc.memberships,
			}

			validator := NewEventPermissionValidator(&mockEventRepoForPermissions{}, mockPredRepo, mockMembershipRepo, tc.minRequired, &mockLogger{})

			canCreate, count, err := validator.CanCreateEvent(context.Background(), tc.userID, groupID, tc.adminIDs)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if canCreate != tc.expectedCanCreate {
				t.Errorf("%s: Expected CanCreateEvent = %v, got %v", tc.description, tc.expectedCanCreate, canCreate)
			}

			if count != tc.expectedCount {
				t.Errorf("%s: Expected count = %d, got %d", tc.description, tc.expectedCount, count)
			}
		})
	}
}

// TestHasGroupMembership tests the HasGroupMembership method
func TestHasGroupMembership(t *testing.T) {
	userID := int64(12345)
	groupID := int64(1)

	testCases := []struct {
		name        string
		memberships map[string]bool
		expected    bool
		description string
	}{
		{
			name: "user has active membership",
			memberships: map[string]bool{
				formatMembershipKey(groupID, userID): true,
			},
			expected:    true,
			description: "User with active membership should return true",
		},
		{
			name:        "user has no membership",
			memberships: map[string]bool{},
			expected:    false,
			description: "User without membership should return false",
		},
		{
			name: "user has membership in different group",
			memberships: map[string]bool{
				formatMembershipKey(999, userID): true,
			},
			expected:    false,
			description: "User with membership in different group should return false",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockMembershipRepo := &mockGroupMembershipRepoForPermissions{
				memberships: tc.memberships,
			}

			validator := NewEventPermissionValidator(&mockEventRepoForPermissions{}, &mockPredictionRepo{}, mockMembershipRepo, 3, &mockLogger{})

			hasMembership, err := validator.HasGroupMembership(context.Background(), userID, groupID)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if hasMembership != tc.expected {
				t.Errorf("%s: Expected HasGroupMembership = %v, got %v", tc.description, tc.expected, hasMembership)
			}
		})
	}
}
