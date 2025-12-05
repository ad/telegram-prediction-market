package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "modernc.org/sqlite"
)

// TestGroupSelectionListAccuracy tests Property 12: Group Selection List Accuracy
func TestGroupSelectionListAccuracy(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("user with N memberships sees exactly N groups in selection list", prop.ForAll(
		func(groupCount int) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer func() { _ = db.Close() }()

			queue := NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			// Run migrations to create tables
			if err := RunMigrations(queue); err != nil {
				t.Logf("Failed to run migrations: %v", err)
				return false
			}

			groupRepo := NewGroupRepository(queue)
			membershipRepo := NewGroupMembershipRepository(queue)
			resolver := domain.NewGroupContextResolver(groupRepo)
			ctx := context.Background()

			userID := int64(12345)

			// Create N groups and memberships for the user
			createdGroupIDs := make(map[int64]bool)
			for i := 0; i < groupCount; i++ {
				// Create group
				group := &domain.Group{
					TelegramChatID: int64(-1000000000000 - i),
					Name:           "Test Group " + string(rune('A'+i)),
					CreatedAt:      time.Now().Add(time.Duration(i) * time.Minute).Truncate(time.Second),
					CreatedBy:      int64(1000 + i),
				}

				if err := groupRepo.CreateGroup(ctx, group); err != nil {
					t.Logf("Failed to create group: %v", err)
					return false
				}

				createdGroupIDs[group.ID] = true

				// Create membership for the user in this group
				membership := &domain.GroupMembership{
					GroupID:  group.ID,
					UserID:   userID,
					JoinedAt: time.Now().Add(time.Duration(i) * time.Minute).Truncate(time.Second),
					Status:   domain.MembershipStatusActive,
				}

				if err := membershipRepo.CreateMembership(ctx, membership); err != nil {
					t.Logf("Failed to create membership: %v", err)
					return false
				}
			}

			// Get user group choices
			choices, err := resolver.GetUserGroupChoices(ctx, userID)
			if err != nil {
				t.Logf("Failed to get user group choices: %v", err)
				return false
			}

			// Verify exactly N groups are returned
			if len(choices) != groupCount {
				t.Logf("Expected %d groups, got %d", groupCount, len(choices))
				return false
			}

			// Verify all returned groups are the ones we created
			returnedGroupIDs := make(map[int64]bool)
			for _, group := range choices {
				returnedGroupIDs[group.ID] = true
			}

			// Check that all created groups are in the returned list
			for groupID := range createdGroupIDs {
				if !returnedGroupIDs[groupID] {
					t.Logf("Created group %d not found in returned choices", groupID)
					return false
				}
			}

			// Check that no extra groups are in the returned list
			for groupID := range returnedGroupIDs {
				if !createdGroupIDs[groupID] {
					t.Logf("Unexpected group %d found in returned choices", groupID)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 20), // Test with 1 to 20 groups
	))

	properties.TestingRun(t)
}

// TestAutomaticGroupSelection tests Property 13: Automatic Group Selection
func TestAutomaticGroupSelection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("user with exactly one group membership has that group auto-selected", prop.ForAll(
		func(userID int64, creatorID int64, telegramChatID int64) bool {
			// Ensure valid IDs
			if userID == 0 {
				userID = 1
			}
			if creatorID == 0 {
				creatorID = 1
			}
			if telegramChatID >= 0 {
				telegramChatID = -1000000000000 - telegramChatID
			}

			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer func() { _ = db.Close() }()

			queue := NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			// Run migrations to create tables
			if err := RunMigrations(queue); err != nil {
				t.Logf("Failed to run migrations: %v", err)
				return false
			}

			groupRepo := NewGroupRepository(queue)
			membershipRepo := NewGroupMembershipRepository(queue)
			resolver := domain.NewGroupContextResolver(groupRepo)
			ctx := context.Background()

			// Create exactly one group
			group := &domain.Group{
				TelegramChatID: telegramChatID,
				Name:           "Single Test Group",
				CreatedAt:      time.Now().Truncate(time.Second),
				CreatedBy:      creatorID,
			}

			if err := groupRepo.CreateGroup(ctx, group); err != nil {
				t.Logf("Failed to create group: %v", err)
				return false
			}

			// Create membership for the user in this group
			membership := &domain.GroupMembership{
				GroupID:  group.ID,
				UserID:   userID,
				JoinedAt: time.Now().Truncate(time.Second),
				Status:   domain.MembershipStatusActive,
			}

			if err := membershipRepo.CreateMembership(ctx, membership); err != nil {
				t.Logf("Failed to create membership: %v", err)
				return false
			}

			// Resolve group for user - should auto-select the single group
			resolvedGroupID, err := resolver.ResolveGroupForUser(ctx, userID)
			if err != nil {
				t.Logf("Expected no error for single-group user, got: %v", err)
				return false
			}

			// Verify the resolved group ID matches the created group
			if resolvedGroupID != group.ID {
				t.Logf("Expected group ID %d, got %d", group.ID, resolvedGroupID)
				return false
			}

			return true
		},
		gen.Int64Range(1, 100000),
		gen.Int64Range(1, 100000),
		gen.Int64Range(1, 100000),
	))

	properties.TestingRun(t)
}

// Unit tests for GroupContextResolver

func TestResolveGroupForSingleMembershipUser(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations to create tables
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	groupRepo := NewGroupRepository(queue)
	membershipRepo := NewGroupMembershipRepository(queue)
	resolver := domain.NewGroupContextResolver(groupRepo)
	ctx := context.Background()

	// Create a group
	group := &domain.Group{
		TelegramChatID: -1001234567890,
		Name:           "Test Group",
		CreatedAt:      time.Now().Truncate(time.Second),
		CreatedBy:      12345,
	}

	err = groupRepo.CreateGroup(ctx, group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Create membership for user
	userID := int64(67890)
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   userID,
		JoinedAt: time.Now().Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Test resolving group for single-membership user
	resolvedGroupID, err := resolver.ResolveGroupForUser(ctx, userID)
	if err != nil {
		t.Fatalf("Expected no error for single-membership user, got: %v", err)
	}

	if resolvedGroupID != group.ID {
		t.Errorf("Expected group ID %d, got %d", group.ID, resolvedGroupID)
	}
}

func TestResolveGroupForMultiMembershipUser(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations to create tables
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	groupRepo := NewGroupRepository(queue)
	membershipRepo := NewGroupMembershipRepository(queue)
	resolver := domain.NewGroupContextResolver(groupRepo)
	ctx := context.Background()

	userID := int64(67890)

	// Create multiple groups and memberships
	for i := 0; i < 3; i++ {
		group := &domain.Group{
			TelegramChatID: int64(-1001234567890 - i),
			Name:           "Test Group " + string(rune('A'+i)),
			CreatedAt:      time.Now().Add(time.Duration(i) * time.Minute).Truncate(time.Second),
			CreatedBy:      int64(12345 + i),
		}

		err = groupRepo.CreateGroup(ctx, group)
		if err != nil {
			t.Fatalf("Failed to create group: %v", err)
		}

		membership := &domain.GroupMembership{
			GroupID:  group.ID,
			UserID:   userID,
			JoinedAt: time.Now().Add(time.Duration(i) * time.Minute).Truncate(time.Second),
			Status:   domain.MembershipStatusActive,
		}

		err = membershipRepo.CreateMembership(ctx, membership)
		if err != nil {
			t.Fatalf("Failed to create membership: %v", err)
		}
	}

	// Test resolving group for multi-membership user
	_, err = resolver.ResolveGroupForUser(ctx, userID)
	if err != domain.ErrMultipleGroupsNeedChoice {
		t.Errorf("Expected ErrMultipleGroupsNeedChoice, got: %v", err)
	}
}

func TestResolveGroupForUserWithNoMemberships(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations to create tables
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	groupRepo := NewGroupRepository(queue)
	resolver := domain.NewGroupContextResolver(groupRepo)
	ctx := context.Background()

	// Test resolving group for user with no memberships
	userID := int64(99999)
	_, err = resolver.ResolveGroupForUser(ctx, userID)
	if err != domain.ErrNoGroupMembership {
		t.Errorf("Expected ErrNoGroupMembership, got: %v", err)
	}
}

func TestGetUserGroupChoices(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations to create tables
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	groupRepo := NewGroupRepository(queue)
	membershipRepo := NewGroupMembershipRepository(queue)
	resolver := domain.NewGroupContextResolver(groupRepo)
	ctx := context.Background()

	userID := int64(67890)

	// Create multiple groups and memberships
	expectedGroups := make(map[int64]string)
	for i := 0; i < 3; i++ {
		group := &domain.Group{
			TelegramChatID: int64(-1001234567890 - i),
			Name:           "Test Group " + string(rune('A'+i)),
			CreatedAt:      time.Now().Add(time.Duration(i) * time.Minute).Truncate(time.Second),
			CreatedBy:      int64(12345 + i),
		}

		err = groupRepo.CreateGroup(ctx, group)
		if err != nil {
			t.Fatalf("Failed to create group: %v", err)
		}

		expectedGroups[group.ID] = group.Name

		membership := &domain.GroupMembership{
			GroupID:  group.ID,
			UserID:   userID,
			JoinedAt: time.Now().Add(time.Duration(i) * time.Minute).Truncate(time.Second),
			Status:   domain.MembershipStatusActive,
		}

		err = membershipRepo.CreateMembership(ctx, membership)
		if err != nil {
			t.Fatalf("Failed to create membership: %v", err)
		}
	}

	// Test getting group choices
	choices, err := resolver.GetUserGroupChoices(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get user group choices: %v", err)
	}

	if len(choices) != len(expectedGroups) {
		t.Errorf("Expected %d groups, got %d", len(expectedGroups), len(choices))
	}

	// Verify all expected groups are in the choices
	for _, group := range choices {
		expectedName, exists := expectedGroups[group.ID]
		if !exists {
			t.Errorf("Unexpected group ID %d in choices", group.ID)
		}
		if group.Name != expectedName {
			t.Errorf("Expected group name %s, got %s", expectedName, group.Name)
		}
	}
}

func TestGetUserGroupChoicesForUserWithNoMemberships(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations to create tables
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	groupRepo := NewGroupRepository(queue)
	resolver := domain.NewGroupContextResolver(groupRepo)
	ctx := context.Background()

	// Test getting group choices for user with no memberships
	userID := int64(99999)
	choices, err := resolver.GetUserGroupChoices(ctx, userID)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(choices) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(choices))
	}
}
