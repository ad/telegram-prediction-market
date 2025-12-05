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

// TestMultipleMemberships tests Property 11: Multiple Memberships
func TestMultipleMemberships(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("user can have active memberships in multiple groups", prop.ForAll(
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
			ctx := context.Background()

			userID := int64(12345)

			// Create N groups and memberships for the same user
			createdMemberships := make([]*domain.GroupMembership, groupCount)
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

				createdMemberships[i] = membership
			}

			// Verify all memberships exist and are active
			for _, created := range createdMemberships {
				retrieved, err := membershipRepo.GetMembership(ctx, created.GroupID, userID)
				if err != nil {
					t.Logf("Failed to get membership: %v", err)
					return false
				}

				if retrieved == nil {
					t.Logf("Membership not found for group %d", created.GroupID)
					return false
				}

				if retrieved.Status != domain.MembershipStatusActive {
					t.Logf("Expected active status, got %s", retrieved.Status)
					return false
				}

				// Verify HasActiveMembership returns true
				hasActive, err := membershipRepo.HasActiveMembership(ctx, created.GroupID, userID)
				if err != nil {
					t.Logf("Failed to check active membership: %v", err)
					return false
				}

				if !hasActive {
					t.Logf("Expected active membership for group %d", created.GroupID)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 20), // Test with 1 to 20 groups
	))

	properties.TestingRun(t)
}

// TestMembershipIdempotence tests Property 8: Idempotent Membership
func TestMembershipIdempotence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("attempting to join same group twice does not create duplicate membership", prop.ForAll(
		func(userID int64, groupCreatorID int64) bool {
			// Ensure valid IDs
			if userID == 0 {
				userID = 1
			}
			if groupCreatorID == 0 {
				groupCreatorID = 1
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
			ctx := context.Background()

			// Create a group
			group := &domain.Group{
				TelegramChatID: -1001234567890,
				Name:           "Test Group",
				CreatedAt:      time.Now().Truncate(time.Second),
				CreatedBy:      groupCreatorID,
			}

			if err := groupRepo.CreateGroup(ctx, group); err != nil {
				t.Logf("Failed to create group: %v", err)
				return false
			}

			// Create first membership
			membership1 := &domain.GroupMembership{
				GroupID:  group.ID,
				UserID:   userID,
				JoinedAt: time.Now().Truncate(time.Second),
				Status:   domain.MembershipStatusActive,
			}

			if err := membershipRepo.CreateMembership(ctx, membership1); err != nil {
				t.Logf("Failed to create first membership: %v", err)
				return false
			}

			// Attempt to create duplicate membership (should fail due to unique constraint)
			membership2 := &domain.GroupMembership{
				GroupID:  group.ID,
				UserID:   userID,
				JoinedAt: time.Now().Add(time.Minute).Truncate(time.Second),
				Status:   domain.MembershipStatusActive,
			}

			err = membershipRepo.CreateMembership(ctx, membership2)
			// We expect this to fail due to unique constraint
			if err == nil {
				t.Logf("Expected error when creating duplicate membership, but succeeded")
				return false
			}

			// Verify only one membership exists
			retrieved, err := membershipRepo.GetMembership(ctx, group.ID, userID)
			if err != nil {
				t.Logf("Failed to get membership: %v", err)
				return false
			}

			if retrieved == nil {
				t.Logf("Expected membership to exist")
				return false
			}

			// Verify it's the first membership (by checking the ID)
			if retrieved.ID != membership1.ID {
				t.Logf("Expected first membership ID %d, got %d", membership1.ID, retrieved.ID)
				return false
			}

			// Verify the join date is from the first membership
			if !retrieved.JoinedAt.Equal(membership1.JoinedAt) {
				t.Logf("Expected first membership join date, got different date")
				return false
			}

			return true
		},
		gen.Int64Range(1, 100000),
		gen.Int64Range(1, 100000),
	))

	properties.TestingRun(t)
}

// Unit tests for GroupMembershipRepository

func TestCreateMembership(t *testing.T) {
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
	ctx := context.Background()

	// Create a group first
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

	// Test creating a valid membership
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   67890,
		JoinedAt: time.Now().Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	if membership.ID == 0 {
		t.Error("Expected membership ID to be set after creation")
	}

	// Verify the membership was created
	retrieved, err := membershipRepo.GetMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve membership: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected to retrieve created membership")
	}

	if retrieved.GroupID != membership.GroupID {
		t.Errorf("GroupID mismatch: expected %d, got %d", membership.GroupID, retrieved.GroupID)
	}
	if retrieved.UserID != membership.UserID {
		t.Errorf("UserID mismatch: expected %d, got %d", membership.UserID, retrieved.UserID)
	}
	if retrieved.Status != membership.Status {
		t.Errorf("Status mismatch: expected %s, got %s", membership.Status, retrieved.Status)
	}
}

func TestGetMembership(t *testing.T) {
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

	// Create a membership
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   67890,
		JoinedAt: time.Now().Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Test retrieving existing membership
	retrieved, err := membershipRepo.GetMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve membership: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected to retrieve membership")
	}

	if retrieved.ID != membership.ID {
		t.Errorf("ID mismatch: expected %d, got %d", membership.ID, retrieved.ID)
	}

	// Test retrieving non-existent membership
	nonExistent, err := membershipRepo.GetMembership(ctx, group.ID, 99999)
	if err != nil {
		t.Fatalf("Expected no error for non-existent membership, got: %v", err)
	}

	if nonExistent != nil {
		t.Error("Expected nil for non-existent membership")
	}
}

func TestGetGroupMembers(t *testing.T) {
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

	// Test with no members
	members, err := membershipRepo.GetGroupMembers(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if len(members) != 0 {
		t.Errorf("Expected 0 members, got %d", len(members))
	}

	// Create multiple memberships with different join times
	membership1 := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   100,
		JoinedAt: time.Now().Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}
	membership2 := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   200,
		JoinedAt: time.Now().Add(time.Minute).Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}
	membership3 := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   300,
		JoinedAt: time.Now().Add(2 * time.Minute).Truncate(time.Second),
		Status:   domain.MembershipStatusRemoved,
	}

	if err := membershipRepo.CreateMembership(ctx, membership1); err != nil {
		t.Fatalf("Failed to create membership1: %v", err)
	}
	if err := membershipRepo.CreateMembership(ctx, membership2); err != nil {
		t.Fatalf("Failed to create membership2: %v", err)
	}
	if err := membershipRepo.CreateMembership(ctx, membership3); err != nil {
		t.Fatalf("Failed to create membership3: %v", err)
	}

	// Test retrieving all members (including removed)
	members, err = membershipRepo.GetGroupMembers(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if len(members) != 3 {
		t.Errorf("Expected 3 members, got %d", len(members))
	}

	// Verify ordering by join date (most recent first)
	if len(members) >= 2 {
		if members[0].UserID != 300 {
			t.Errorf("Expected most recent member (300) first, got %d", members[0].UserID)
		}
		if members[1].UserID != 200 {
			t.Errorf("Expected second most recent member (200) second, got %d", members[1].UserID)
		}
		if members[2].UserID != 100 {
			t.Errorf("Expected oldest member (100) last, got %d", members[2].UserID)
		}
	}
}

func TestUpdateMembershipStatus(t *testing.T) {
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

	// Create a membership
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   67890,
		JoinedAt: time.Now().Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Verify initial status
	retrieved, err := membershipRepo.GetMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve membership: %v", err)
	}

	if retrieved.Status != domain.MembershipStatusActive {
		t.Errorf("Expected active status, got %s", retrieved.Status)
	}

	// Update status to removed
	err = membershipRepo.UpdateMembershipStatus(ctx, group.ID, membership.UserID, domain.MembershipStatusRemoved)
	if err != nil {
		t.Fatalf("Failed to update membership status: %v", err)
	}

	// Verify status was updated
	updated, err := membershipRepo.GetMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated membership: %v", err)
	}

	if updated.Status != domain.MembershipStatusRemoved {
		t.Errorf("Expected removed status, got %s", updated.Status)
	}

	// Update back to active
	err = membershipRepo.UpdateMembershipStatus(ctx, group.ID, membership.UserID, domain.MembershipStatusActive)
	if err != nil {
		t.Fatalf("Failed to update membership status back to active: %v", err)
	}

	// Verify status was updated again
	reactivated, err := membershipRepo.GetMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve reactivated membership: %v", err)
	}

	if reactivated.Status != domain.MembershipStatusActive {
		t.Errorf("Expected active status, got %s", reactivated.Status)
	}
}

func TestHasActiveMembership(t *testing.T) {
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

	// Test user with no membership
	hasActive, err := membershipRepo.HasActiveMembership(ctx, group.ID, 99999)
	if err != nil {
		t.Fatalf("Failed to check active membership: %v", err)
	}

	if hasActive {
		t.Error("Expected no active membership for non-member user")
	}

	// Create an active membership
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   67890,
		JoinedAt: time.Now().Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Test user with active membership
	hasActive, err = membershipRepo.HasActiveMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to check active membership: %v", err)
	}

	if !hasActive {
		t.Error("Expected active membership for member user")
	}

	// Update status to removed
	err = membershipRepo.UpdateMembershipStatus(ctx, group.ID, membership.UserID, domain.MembershipStatusRemoved)
	if err != nil {
		t.Fatalf("Failed to update membership status: %v", err)
	}

	// Test user with removed membership
	hasActive, err = membershipRepo.HasActiveMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to check active membership: %v", err)
	}

	if hasActive {
		t.Error("Expected no active membership for removed user")
	}
}

// TestMembershipRemoval tests Property 18: Membership Removal
func TestMembershipRemoval(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("removing a member marks membership as removed instead of deleting", prop.ForAll(
		func(userID int64, groupCreatorID int64) bool {
			// Ensure valid IDs
			if userID == 0 {
				userID = 1
			}
			if groupCreatorID == 0 {
				groupCreatorID = 1
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
			ctx := context.Background()

			// Create a group
			group := &domain.Group{
				TelegramChatID: -1001234567890,
				Name:           "Test Group",
				CreatedAt:      time.Now().Truncate(time.Second),
				CreatedBy:      groupCreatorID,
			}

			if err := groupRepo.CreateGroup(ctx, group); err != nil {
				t.Logf("Failed to create group: %v", err)
				return false
			}

			// Create an active membership
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

			originalID := membership.ID
			originalJoinedAt := membership.JoinedAt

			// Remove the member by updating status
			if err := membershipRepo.UpdateMembershipStatus(ctx, group.ID, userID, domain.MembershipStatusRemoved); err != nil {
				t.Logf("Failed to update membership status: %v", err)
				return false
			}

			// Verify membership still exists but is marked as removed
			retrieved, err := membershipRepo.GetMembership(ctx, group.ID, userID)
			if err != nil {
				t.Logf("Failed to get membership: %v", err)
				return false
			}

			if retrieved == nil {
				t.Logf("Membership was deleted instead of marked as removed")
				return false
			}

			if retrieved.Status != domain.MembershipStatusRemoved {
				t.Logf("Expected removed status, got %s", retrieved.Status)
				return false
			}

			// Verify the membership ID and join date are preserved
			if retrieved.ID != originalID {
				t.Logf("Membership ID changed after removal")
				return false
			}

			if !retrieved.JoinedAt.Equal(originalJoinedAt) {
				t.Logf("Join date changed after removal")
				return false
			}

			// Verify HasActiveMembership returns false
			hasActive, err := membershipRepo.HasActiveMembership(ctx, group.ID, userID)
			if err != nil {
				t.Logf("Failed to check active membership: %v", err)
				return false
			}

			if hasActive {
				t.Logf("HasActiveMembership returned true for removed member")
				return false
			}

			return true
		},
		gen.Int64Range(1, 100000),
		gen.Int64Range(1, 100000),
	))

	properties.TestingRun(t)
}

// TestHistoricalDataPreservation tests Property 20: Historical Data Preservation
func TestHistoricalDataPreservation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("removing a member preserves membership record with historical data", prop.ForAll(
		func(userID int64, groupCreatorID int64) bool {
			// Ensure valid IDs
			if userID == 0 {
				userID = 1
			}
			if groupCreatorID == 0 {
				groupCreatorID = 1
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
			ctx := context.Background()

			// Create a group
			group := &domain.Group{
				TelegramChatID: -1001234567890,
				Name:           "Test Group",
				CreatedAt:      time.Now().Truncate(time.Second),
				CreatedBy:      groupCreatorID,
			}

			if err := groupRepo.CreateGroup(ctx, group); err != nil {
				t.Logf("Failed to create group: %v", err)
				return false
			}

			// Create an active membership
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

			originalID := membership.ID
			originalJoinedAt := membership.JoinedAt

			// Remove the member
			if err := membershipRepo.UpdateMembershipStatus(ctx, group.ID, userID, domain.MembershipStatusRemoved); err != nil {
				t.Logf("Failed to update membership status: %v", err)
				return false
			}

			// Verify membership record is still present (not deleted)
			retrievedMembership, err := membershipRepo.GetMembership(ctx, group.ID, userID)
			if err != nil {
				t.Logf("Failed to get membership after removal: %v", err)
				return false
			}

			if retrievedMembership == nil {
				t.Logf("Membership was deleted instead of marked as removed")
				return false
			}

			// Verify membership is marked as removed
			if retrievedMembership.Status != domain.MembershipStatusRemoved {
				t.Logf("Expected removed status, got %s", retrievedMembership.Status)
				return false
			}

			// Verify historical data is preserved (ID and join date unchanged)
			if retrievedMembership.ID != originalID {
				t.Logf("Membership ID changed after removal")
				return false
			}

			if !retrievedMembership.JoinedAt.Equal(originalJoinedAt) {
				t.Logf("Join date changed after removal")
				return false
			}

			// Verify the membership is still retrievable via GetGroupMembers
			// (which returns all members including removed ones)
			allMembers, err := membershipRepo.GetGroupMembers(ctx, group.ID)
			if err != nil {
				t.Logf("Failed to get group members: %v", err)
				return false
			}

			found := false
			for _, member := range allMembers {
				if member.UserID == userID && member.Status == domain.MembershipStatusRemoved {
					found = true
					break
				}
			}

			if !found {
				t.Logf("Removed membership not found in group members list")
				return false
			}

			return true
		},
		gen.Int64Range(1, 100000),
		gen.Int64Range(1, 100000),
	))

	properties.TestingRun(t)
}

// TestRejoinAfterRemoval tests Property 22: Rejoin After Removal
func TestRejoinAfterRemoval(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("removed user can rejoin group by reactivating membership", prop.ForAll(
		func(userID int64, groupCreatorID int64) bool {
			// Ensure valid IDs
			if userID == 0 {
				userID = 1
			}
			if groupCreatorID == 0 {
				groupCreatorID = 1
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
			ctx := context.Background()

			// Create a group
			group := &domain.Group{
				TelegramChatID: -1001234567890,
				Name:           "Test Group",
				CreatedAt:      time.Now().Truncate(time.Second),
				CreatedBy:      groupCreatorID,
			}

			if err := groupRepo.CreateGroup(ctx, group); err != nil {
				t.Logf("Failed to create group: %v", err)
				return false
			}

			// Create an active membership
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

			originalID := membership.ID
			originalJoinedAt := membership.JoinedAt

			// Remove the member
			if err := membershipRepo.UpdateMembershipStatus(ctx, group.ID, userID, domain.MembershipStatusRemoved); err != nil {
				t.Logf("Failed to update membership status: %v", err)
				return false
			}

			// Verify membership is removed
			removed, err := membershipRepo.GetMembership(ctx, group.ID, userID)
			if err != nil {
				t.Logf("Failed to get membership: %v", err)
				return false
			}

			if removed == nil || removed.Status != domain.MembershipStatusRemoved {
				t.Logf("Membership not properly removed")
				return false
			}

			// Verify user cannot access group (no active membership)
			hasActive, err := membershipRepo.HasActiveMembership(ctx, group.ID, userID)
			if err != nil {
				t.Logf("Failed to check active membership: %v", err)
				return false
			}

			if hasActive {
				t.Logf("User still has active membership after removal")
				return false
			}

			// Rejoin by reactivating membership
			if err := membershipRepo.UpdateMembershipStatus(ctx, group.ID, userID, domain.MembershipStatusActive); err != nil {
				t.Logf("Failed to reactivate membership: %v", err)
				return false
			}

			// Verify membership is now active
			rejoined, err := membershipRepo.GetMembership(ctx, group.ID, userID)
			if err != nil {
				t.Logf("Failed to get membership after rejoin: %v", err)
				return false
			}

			if rejoined == nil {
				t.Logf("Membership not found after rejoin")
				return false
			}

			if rejoined.Status != domain.MembershipStatusActive {
				t.Logf("Expected active status after rejoin, got %s", rejoined.Status)
				return false
			}

			// Verify the same membership record is used (ID preserved)
			if rejoined.ID != originalID {
				t.Logf("Membership ID changed after rejoin")
				return false
			}

			// Verify original join date is preserved
			if !rejoined.JoinedAt.Equal(originalJoinedAt) {
				t.Logf("Join date changed after rejoin")
				return false
			}

			// Verify user now has active membership
			hasActiveAfterRejoin, err := membershipRepo.HasActiveMembership(ctx, group.ID, userID)
			if err != nil {
				t.Logf("Failed to check active membership after rejoin: %v", err)
				return false
			}

			if !hasActiveAfterRejoin {
				t.Logf("User does not have active membership after rejoin")
				return false
			}

			return true
		},
		gen.Int64Range(1, 100000),
		gen.Int64Range(1, 100000),
	))

	properties.TestingRun(t)
}

// Unit tests for removal and rejoin functionality

func TestMembershipMarkedAsRemoved(t *testing.T) {
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

	// Create an active membership
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   67890,
		JoinedAt: time.Now().Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Remove the member
	err = membershipRepo.UpdateMembershipStatus(ctx, group.ID, membership.UserID, domain.MembershipStatusRemoved)
	if err != nil {
		t.Fatalf("Failed to update membership status: %v", err)
	}

	// Verify membership is marked as removed
	retrieved, err := membershipRepo.GetMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve membership: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Membership was deleted instead of marked as removed")
	}

	if retrieved.Status != domain.MembershipStatusRemoved {
		t.Errorf("Expected removed status, got %s", retrieved.Status)
	}
}

func TestHistoricalDataPreservedAfterRemoval(t *testing.T) {
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

	// Create an active membership
	joinTime := time.Now().Truncate(time.Second)
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   67890,
		JoinedAt: joinTime,
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	originalID := membership.ID

	// Remove the member
	err = membershipRepo.UpdateMembershipStatus(ctx, group.ID, membership.UserID, domain.MembershipStatusRemoved)
	if err != nil {
		t.Fatalf("Failed to update membership status: %v", err)
	}

	// Verify historical data is preserved
	retrieved, err := membershipRepo.GetMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve membership: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Membership was deleted")
	}

	if retrieved.ID != originalID {
		t.Errorf("Membership ID changed: expected %d, got %d", originalID, retrieved.ID)
	}

	if !retrieved.JoinedAt.Equal(joinTime) {
		t.Errorf("Join date changed: expected %v, got %v", joinTime, retrieved.JoinedAt)
	}

	// Verify membership appears in group members list
	members, err := membershipRepo.GetGroupMembers(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	found := false
	for _, member := range members {
		if member.UserID == membership.UserID && member.Status == domain.MembershipStatusRemoved {
			found = true
			break
		}
	}

	if !found {
		t.Error("Removed membership not found in group members list")
	}
}

func TestRemovedUserCannotAccessGroup(t *testing.T) {
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

	// Create an active membership
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   67890,
		JoinedAt: time.Now().Truncate(time.Second),
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Verify user has active membership
	hasActive, err := membershipRepo.HasActiveMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to check active membership: %v", err)
	}

	if !hasActive {
		t.Error("Expected active membership before removal")
	}

	// Remove the member
	err = membershipRepo.UpdateMembershipStatus(ctx, group.ID, membership.UserID, domain.MembershipStatusRemoved)
	if err != nil {
		t.Fatalf("Failed to update membership status: %v", err)
	}

	// Verify user no longer has active membership
	hasActive, err = membershipRepo.HasActiveMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to check active membership after removal: %v", err)
	}

	if hasActive {
		t.Error("User still has active membership after removal")
	}
}

func TestRemovedUserCanRejoin(t *testing.T) {
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

	// Create an active membership
	joinTime := time.Now().Truncate(time.Second)
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   67890,
		JoinedAt: joinTime,
		Status:   domain.MembershipStatusActive,
	}

	err = membershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	originalID := membership.ID

	// Remove the member
	err = membershipRepo.UpdateMembershipStatus(ctx, group.ID, membership.UserID, domain.MembershipStatusRemoved)
	if err != nil {
		t.Fatalf("Failed to update membership status: %v", err)
	}

	// Verify user cannot access group
	hasActive, err := membershipRepo.HasActiveMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to check active membership: %v", err)
	}

	if hasActive {
		t.Error("User still has active membership after removal")
	}

	// Rejoin by reactivating membership
	err = membershipRepo.UpdateMembershipStatus(ctx, group.ID, membership.UserID, domain.MembershipStatusActive)
	if err != nil {
		t.Fatalf("Failed to reactivate membership: %v", err)
	}

	// Verify user can now access group
	hasActive, err = membershipRepo.HasActiveMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to check active membership after rejoin: %v", err)
	}

	if !hasActive {
		t.Error("User does not have active membership after rejoin")
	}

	// Verify membership record is the same (ID and join date preserved)
	rejoined, err := membershipRepo.GetMembership(ctx, group.ID, membership.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve membership after rejoin: %v", err)
	}

	if rejoined == nil {
		t.Fatal("Membership not found after rejoin")
	}

	if rejoined.ID != originalID {
		t.Errorf("Membership ID changed after rejoin: expected %d, got %d", originalID, rejoined.ID)
	}

	if !rejoined.JoinedAt.Equal(joinTime) {
		t.Errorf("Join date changed after rejoin: expected %v, got %v", joinTime, rejoined.JoinedAt)
	}

	if rejoined.Status != domain.MembershipStatusActive {
		t.Errorf("Expected active status after rejoin, got %s", rejoined.Status)
	}
}
