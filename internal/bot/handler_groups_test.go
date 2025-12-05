package bot

import (
	"context"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

// Property 36: User Groups List Accuracy
// For any user with N active memberships, the /groups command should display exactly N groups with their names, join dates, and member counts
// Validates: Requirements 13.1, 13.2
func TestProperty_UserGroupsListAccuracy(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("user groups list contains exactly N groups for N active memberships", prop.ForAll(
		func(userID int64, numGroups uint8) bool {
			// Limit number of groups to reasonable range (1-10)
			if numGroups == 0 {
				numGroups = 1
			}
			if numGroups > 10 {
				numGroups = 10
			}

			ctx := context.Background()

			// Setup test database
			dbQueue, cleanup := setupTestDB(t)
			defer cleanup()

			// Create repositories
			groupRepo := storage.NewGroupRepository(dbQueue)
			groupMembershipRepo := storage.NewGroupMembershipRepository(dbQueue)

			// Create N groups and memberships for the user
			createdGroups := make([]*domain.Group, numGroups)
			for i := uint8(0); i < numGroups; i++ {
				groupName, _ := gen.Identifier().Sample()
				group := &domain.Group{
					TelegramChatID: int64(100 + i),
					Name:           groupName.(string),
					CreatedAt:      time.Now(),
					CreatedBy:      999,
				}
				err := groupRepo.CreateGroup(ctx, group)
				if err != nil {
					return false
				}
				createdGroups[i] = group

				// Create active membership
				membership := &domain.GroupMembership{
					GroupID:  group.ID,
					UserID:   userID,
					JoinedAt: time.Now().Add(-time.Duration(i) * time.Hour), // Different join times
					Status:   domain.MembershipStatusActive,
				}
				err = groupMembershipRepo.CreateMembership(ctx, membership)
				if err != nil {
					return false
				}
			}

			// Also create some removed memberships that should not appear
			removedGroup := &domain.Group{
				TelegramChatID: 999,
				Name:           "Removed Group",
				CreatedAt:      time.Now(),
				CreatedBy:      999,
			}
			err := groupRepo.CreateGroup(ctx, removedGroup)
			if err != nil {
				return false
			}

			removedMembership := &domain.GroupMembership{
				GroupID:  removedGroup.ID,
				UserID:   userID,
				JoinedAt: time.Now(),
				Status:   domain.MembershipStatusRemoved,
			}
			err = groupMembershipRepo.CreateMembership(ctx, removedMembership)
			if err != nil {
				return false
			}

			// Retrieve user's groups (simulating what HandleGroups does)
			userGroups, err := groupRepo.GetUserGroups(ctx, userID)
			if err != nil {
				return false
			}

			// Verify exactly N groups are returned (not including removed membership)
			if len(userGroups) != int(numGroups) {
				return false
			}

			// Verify all returned groups are in the created groups list
			for _, userGroup := range userGroups {
				found := false
				for _, createdGroup := range createdGroups {
					if userGroup.ID == createdGroup.ID {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}

			// Verify each group has the correct data
			for _, userGroup := range userGroups {
				// Verify we can get membership info
				membership, err := groupMembershipRepo.GetMembership(ctx, userGroup.ID, userID)
				if err != nil || membership == nil {
					return false
				}

				// Verify membership is active
				if membership.Status != domain.MembershipStatusActive {
					return false
				}

				// Verify we can get member count
				members, err := groupMembershipRepo.GetGroupMembers(ctx, userGroup.ID)
				if err != nil {
					return false
				}

				// Should have at least this user as a member
				if len(members) == 0 {
					return false
				}
			}

			return true
		},
		gen.Int64(),
		gen.UInt8(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 37: Groups List Ordering
// For any user with memberships having different join dates, the groups list should be ordered with most recently joined groups first
// Validates: Requirements 13.4
func TestProperty_GroupsListOrdering(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("groups list is ordered by join date descending", prop.ForAll(
		func(userID int64) bool {
			ctx := context.Background()

			// Setup test database
			dbQueue, cleanup := setupTestDB(t)
			defer cleanup()

			// Create repositories
			groupRepo := storage.NewGroupRepository(dbQueue)
			groupMembershipRepo := storage.NewGroupMembershipRepository(dbQueue)

			// Create 5 groups with different join times
			numGroups := 5
			joinTimes := make([]time.Time, numGroups)
			for i := 0; i < numGroups; i++ {
				groupName, _ := gen.Identifier().Sample()
				group := &domain.Group{
					TelegramChatID: int64(100 + i),
					Name:           groupName.(string),
					CreatedAt:      time.Now(),
					CreatedBy:      999,
				}
				err := groupRepo.CreateGroup(ctx, group)
				if err != nil {
					return false
				}

				// Create membership with distinct join time (older to newer)
				joinTime := time.Now().Add(-time.Duration(numGroups-i) * time.Hour)
				joinTimes[i] = joinTime

				membership := &domain.GroupMembership{
					GroupID:  group.ID,
					UserID:   userID,
					JoinedAt: joinTime,
					Status:   domain.MembershipStatusActive,
				}
				err = groupMembershipRepo.CreateMembership(ctx, membership)
				if err != nil {
					return false
				}
			}

			// Retrieve user's groups (should be ordered by join date DESC)
			userGroups, err := groupRepo.GetUserGroups(ctx, userID)
			if err != nil {
				return false
			}

			if len(userGroups) != numGroups {
				return false
			}

			// Verify ordering: most recent join first
			var prevJoinTime *time.Time
			for _, group := range userGroups {
				membership, err := groupMembershipRepo.GetMembership(ctx, group.ID, userID)
				if err != nil || membership == nil {
					return false
				}

				if prevJoinTime != nil {
					// Current join time should be before or equal to previous
					if membership.JoinedAt.After(*prevJoinTime) {
						return false
					}
				}
				prevJoinTime = &membership.JoinedAt
			}

			return true
		},
		gen.Int64(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestGroupsCommandWithMemberships tests displaying groups for user with memberships
func TestGroupsCommandWithMemberships(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	dbQueue, cleanup := setupTestDB(t)
	defer cleanup()

	// Create repositories
	groupRepo := storage.NewGroupRepository(dbQueue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(dbQueue)

	userID := int64(12345)

	// Create two groups
	group1 := &domain.Group{
		TelegramChatID: 100,
		Name:           "Test Group 1",
		CreatedAt:      time.Now(),
		CreatedBy:      999,
	}
	err := groupRepo.CreateGroup(ctx, group1)
	if err != nil {
		t.Fatalf("Failed to create group 1: %v", err)
	}

	group2 := &domain.Group{
		TelegramChatID: 200,
		Name:           "Test Group 2",
		CreatedAt:      time.Now(),
		CreatedBy:      999,
	}
	err = groupRepo.CreateGroup(ctx, group2)
	if err != nil {
		t.Fatalf("Failed to create group 2: %v", err)
	}

	// Create memberships
	membership1 := &domain.GroupMembership{
		GroupID:  group1.ID,
		UserID:   userID,
		JoinedAt: time.Now().Add(-24 * time.Hour),
		Status:   domain.MembershipStatusActive,
	}
	err = groupMembershipRepo.CreateMembership(ctx, membership1)
	if err != nil {
		t.Fatalf("Failed to create membership 1: %v", err)
	}

	membership2 := &domain.GroupMembership{
		GroupID:  group2.ID,
		UserID:   userID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	err = groupMembershipRepo.CreateMembership(ctx, membership2)
	if err != nil {
		t.Fatalf("Failed to create membership 2: %v", err)
	}

	// Retrieve user's groups
	groups, err := groupRepo.GetUserGroups(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get user groups: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}

	// Verify groups are returned
	foundGroup1 := false
	foundGroup2 := false
	for _, g := range groups {
		if g.Name == "Test Group 1" {
			foundGroup1 = true
		}
		if g.Name == "Test Group 2" {
			foundGroup2 = true
		}
	}

	if !foundGroup1 {
		t.Error("Group 1 not found in user's groups")
	}
	if !foundGroup2 {
		t.Error("Group 2 not found in user's groups")
	}

	t.Logf("Successfully retrieved %d groups for user", len(groups))
}

// TestGroupsCommandWithNoMemberships tests displaying message for user with no memberships
func TestGroupsCommandWithNoMemberships(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	dbQueue, cleanup := setupTestDB(t)
	defer cleanup()

	// Create repositories
	groupRepo := storage.NewGroupRepository(dbQueue)

	userID := int64(12345)

	// Retrieve user's groups (should be empty)
	groups, err := groupRepo.GetUserGroups(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get user groups: %v", err)
	}

	if len(groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(groups))
	}

	t.Log("Successfully handled user with no group memberships")
}

// TestGroupsCommandOrdering tests ordering by join date
func TestGroupsCommandOrdering(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	dbQueue, cleanup := setupTestDB(t)
	defer cleanup()

	// Create repositories
	groupRepo := storage.NewGroupRepository(dbQueue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(dbQueue)

	userID := int64(12345)

	// Create three groups with different join times
	group1 := &domain.Group{
		TelegramChatID: 100,
		Name:           "Oldest Group",
		CreatedAt:      time.Now(),
		CreatedBy:      999,
	}
	err := groupRepo.CreateGroup(ctx, group1)
	if err != nil {
		t.Fatalf("Failed to create group 1: %v", err)
	}

	group2 := &domain.Group{
		TelegramChatID: 200,
		Name:           "Middle Group",
		CreatedAt:      time.Now(),
		CreatedBy:      999,
	}
	err = groupRepo.CreateGroup(ctx, group2)
	if err != nil {
		t.Fatalf("Failed to create group 2: %v", err)
	}

	group3 := &domain.Group{
		TelegramChatID: 300,
		Name:           "Newest Group",
		CreatedAt:      time.Now(),
		CreatedBy:      999,
	}
	err = groupRepo.CreateGroup(ctx, group3)
	if err != nil {
		t.Fatalf("Failed to create group 3: %v", err)
	}

	// Create memberships with different join times
	membership1 := &domain.GroupMembership{
		GroupID:  group1.ID,
		UserID:   userID,
		JoinedAt: time.Now().Add(-48 * time.Hour), // Oldest
		Status:   domain.MembershipStatusActive,
	}
	err = groupMembershipRepo.CreateMembership(ctx, membership1)
	if err != nil {
		t.Fatalf("Failed to create membership 1: %v", err)
	}

	membership2 := &domain.GroupMembership{
		GroupID:  group2.ID,
		UserID:   userID,
		JoinedAt: time.Now().Add(-24 * time.Hour), // Middle
		Status:   domain.MembershipStatusActive,
	}
	err = groupMembershipRepo.CreateMembership(ctx, membership2)
	if err != nil {
		t.Fatalf("Failed to create membership 2: %v", err)
	}

	membership3 := &domain.GroupMembership{
		GroupID:  group3.ID,
		UserID:   userID,
		JoinedAt: time.Now(), // Newest
		Status:   domain.MembershipStatusActive,
	}
	err = groupMembershipRepo.CreateMembership(ctx, membership3)
	if err != nil {
		t.Fatalf("Failed to create membership 3: %v", err)
	}

	// Retrieve user's groups (should be ordered by join date DESC)
	groups, err := groupRepo.GetUserGroups(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get user groups: %v", err)
	}

	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	// Verify ordering: newest first
	if groups[0].Name != "Newest Group" {
		t.Errorf("Expected first group to be 'Newest Group', got '%s'", groups[0].Name)
	}
	if groups[1].Name != "Middle Group" {
		t.Errorf("Expected second group to be 'Middle Group', got '%s'", groups[1].Name)
	}
	if groups[2].Name != "Oldest Group" {
		t.Errorf("Expected third group to be 'Oldest Group', got '%s'", groups[2].Name)
	}

	t.Log("Successfully verified groups are ordered by join date (most recent first)")
}
