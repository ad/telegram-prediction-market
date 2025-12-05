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

// TestGroupListCompleteness tests Property 2: Group List Completeness
// Validates: Requirements 1.2
func TestGroupListCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("group list contains all created groups", prop.ForAll(
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

			// Run migrations to create groups table
			if err := RunMigrations(queue); err != nil {
				t.Logf("Failed to run migrations: %v", err)
				return false
			}

			repo := NewGroupRepository(queue)
			ctx := context.Background()

			// Create N groups
			createdGroups := make([]*domain.Group, groupCount)
			for i := 0; i < groupCount; i++ {
				group := &domain.Group{
					TelegramChatID: int64(-1000000000000 - i), // Telegram group chat IDs are negative
					Name:           "Test Group " + string(rune('A'+i)),
					CreatedAt:      time.Now().Add(time.Duration(i) * time.Minute).Truncate(time.Second),
					CreatedBy:      int64(1000 + i),
				}

				if err := repo.CreateGroup(ctx, group); err != nil {
					t.Logf("Failed to create group: %v", err)
					return false
				}

				createdGroups[i] = group
			}

			// Retrieve all groups
			retrievedGroups, err := repo.GetAllGroups(ctx)
			if err != nil {
				t.Logf("Failed to get all groups: %v", err)
				return false
			}

			// Verify count matches
			if len(retrievedGroups) != groupCount {
				t.Logf("Group count mismatch: expected %d, got %d", groupCount, len(retrievedGroups))
				return false
			}

			// Verify all created groups are in the retrieved list
			for _, created := range createdGroups {
				found := false
				for _, retrieved := range retrievedGroups {
					if retrieved.ID == created.ID &&
						retrieved.Name == created.Name &&
						retrieved.CreatedAt.Equal(created.CreatedAt) &&
						retrieved.CreatedBy == created.CreatedBy {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Created group not found in retrieved list: %+v", created)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 20), // Test with 0 to 20 groups
	))

	properties.TestingRun(t)
}

// Unit tests for GroupRepository

func TestCreateGroup(t *testing.T) {
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

	// Run migrations to create groups table
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewGroupRepository(queue)
	ctx := context.Background()

	// Test creating a valid group
	group := &domain.Group{
		TelegramChatID: -1001234567890, // Telegram group chat IDs are negative
		Name:           "Test Group",
		CreatedAt:      time.Now().Truncate(time.Second),
		CreatedBy:      12345,
	}

	err = repo.CreateGroup(ctx, group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	if group.ID == 0 {
		t.Error("Expected group ID to be set after creation")
	}

	// Verify the group was created
	retrieved, err := repo.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve group: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected to retrieve created group")
	}

	if retrieved.Name != group.Name {
		t.Errorf("Name mismatch: expected %s, got %s", group.Name, retrieved.Name)
	}
	if retrieved.CreatedBy != group.CreatedBy {
		t.Errorf("CreatedBy mismatch: expected %d, got %d", group.CreatedBy, retrieved.CreatedBy)
	}
}

func TestGetGroup(t *testing.T) {
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

	// Run migrations to create groups table
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewGroupRepository(queue)
	ctx := context.Background()

	// Create a group
	group := &domain.Group{
		TelegramChatID: -1001234567890,
		Name:           "Test Group",
		CreatedAt:      time.Now().Truncate(time.Second),
		CreatedBy:      12345,
	}

	err = repo.CreateGroup(ctx, group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Test retrieving existing group
	retrieved, err := repo.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve group: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected to retrieve group")
	}

	if retrieved.ID != group.ID {
		t.Errorf("ID mismatch: expected %d, got %d", group.ID, retrieved.ID)
	}

	// Test retrieving non-existent group
	nonExistent, err := repo.GetGroup(ctx, 99999)
	if err != nil {
		t.Fatalf("Expected no error for non-existent group, got: %v", err)
	}

	if nonExistent != nil {
		t.Error("Expected nil for non-existent group")
	}
}

func TestGetAllGroups(t *testing.T) {
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

	// Run migrations to create groups table
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewGroupRepository(queue)
	ctx := context.Background()

	// Test with no groups
	groups, err := repo.GetAllGroups(ctx)
	if err != nil {
		t.Fatalf("Failed to get all groups: %v", err)
	}

	if len(groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(groups))
	}

	// Create multiple groups
	group1 := &domain.Group{
		TelegramChatID: -1001111111111,
		Name:           "Group 1",
		CreatedAt:      time.Now().Truncate(time.Second),
		CreatedBy:      1,
	}
	group2 := &domain.Group{
		TelegramChatID: -1002222222222,
		Name:           "Group 2",
		CreatedAt:      time.Now().Add(time.Minute).Truncate(time.Second),
		CreatedBy:      2,
	}

	if err := repo.CreateGroup(ctx, group1); err != nil {
		t.Fatalf("Failed to create group1: %v", err)
	}
	if err := repo.CreateGroup(ctx, group2); err != nil {
		t.Fatalf("Failed to create group2: %v", err)
	}

	// Test retrieving all groups
	groups, err = repo.GetAllGroups(ctx)
	if err != nil {
		t.Fatalf("Failed to get all groups: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}
}

func TestGetUserGroups(t *testing.T) {
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

	// Run migrations to create groups table
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewGroupRepository(queue)
	ctx := context.Background()

	// Create groups
	group1 := &domain.Group{
		TelegramChatID: -1001111111111,
		Name:           "Group 1",
		CreatedAt:      time.Now().Truncate(time.Second),
		CreatedBy:      1,
	}
	group2 := &domain.Group{
		TelegramChatID: -1002222222222,
		Name:           "Group 2",
		CreatedAt:      time.Now().Add(time.Minute).Truncate(time.Second),
		CreatedBy:      2,
	}

	if err := repo.CreateGroup(ctx, group1); err != nil {
		t.Fatalf("Failed to create group1: %v", err)
	}
	if err := repo.CreateGroup(ctx, group2); err != nil {
		t.Fatalf("Failed to create group2: %v", err)
	}

	// Create memberships for user 100
	_, err = db.ExecContext(ctx,
		`INSERT INTO group_memberships (group_id, user_id, joined_at, status) VALUES (?, ?, ?, ?)`,
		group1.ID, 100, time.Now(), domain.MembershipStatusActive,
	)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO group_memberships (group_id, user_id, joined_at, status) VALUES (?, ?, ?, ?)`,
		group2.ID, 100, time.Now().Add(time.Minute), domain.MembershipStatusActive,
	)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Test getting user's groups
	userGroups, err := repo.GetUserGroups(ctx, 100)
	if err != nil {
		t.Fatalf("Failed to get user groups: %v", err)
	}

	if len(userGroups) != 2 {
		t.Errorf("Expected 2 groups for user, got %d", len(userGroups))
	}

	// Test user with no groups
	emptyGroups, err := repo.GetUserGroups(ctx, 999)
	if err != nil {
		t.Fatalf("Failed to get user groups: %v", err)
	}

	if len(emptyGroups) != 0 {
		t.Errorf("Expected 0 groups for user with no memberships, got %d", len(emptyGroups))
	}

	// Test that removed memberships are not included
	_, err = db.ExecContext(ctx,
		`UPDATE group_memberships SET status = ? WHERE group_id = ? AND user_id = ?`,
		domain.MembershipStatusRemoved, group1.ID, 100,
	)
	if err != nil {
		t.Fatalf("Failed to update membership status: %v", err)
	}

	activeGroups, err := repo.GetUserGroups(ctx, 100)
	if err != nil {
		t.Fatalf("Failed to get user groups: %v", err)
	}

	if len(activeGroups) != 1 {
		t.Errorf("Expected 1 active group for user, got %d", len(activeGroups))
	}
}

func TestDeleteGroup(t *testing.T) {
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

	// Run migrations to create groups table
	if err := RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewGroupRepository(queue)
	ctx := context.Background()

	// Create a group
	group := &domain.Group{
		TelegramChatID: -1001234567890,
		Name:           "Test Group",
		CreatedAt:      time.Now().Truncate(time.Second),
		CreatedBy:      12345,
	}

	err = repo.CreateGroup(ctx, group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Verify group exists
	retrieved, err := repo.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve group: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected group to exist")
	}

	// Delete the group
	err = repo.DeleteGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to delete group: %v", err)
	}

	// Verify group is deleted
	deleted, err := repo.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to check deleted group: %v", err)
	}
	if deleted != nil {
		t.Error("Expected group to be deleted")
	}

	// Test deleting non-existent group (should not error)
	err = repo.DeleteGroup(ctx, 99999)
	if err != nil {
		t.Errorf("Expected no error when deleting non-existent group, got: %v", err)
	}
}
