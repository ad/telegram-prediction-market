package bot

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory database with schema for testing
func setupTestDB(t *testing.T) (*storage.DBQueue, func()) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	queue := storage.NewDBQueue(db)

	// Initialize schema
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	cleanup := func() {
		queue.Close()
		_ = db.Close()
	}

	return queue, cleanup
}

// TestCreateGroupCommand tests the /create_group command
func TestCreateGroupCommand(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	dbQueue, cleanup := setupTestDB(t)
	defer cleanup()

	// Create repositories
	groupRepo := storage.NewGroupRepository(dbQueue)

	// Create test config with admin user
	adminUserID := int64(12345)

	// Create deep link service
	deepLinkService := domain.NewDeepLinkService("testbot")

	// Create a group directly (simulating the create_group flow)
	group := &domain.Group{
		TelegramChatID: 100,
		Name:           "Test Group",
		CreatedAt:      time.Now(),
		CreatedBy:      adminUserID,
	}

	err := groupRepo.CreateGroup(ctx, group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Verify group was created
	retrievedGroup, err := groupRepo.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve group: %v", err)
	}

	if retrievedGroup == nil {
		t.Fatal("Group not found after creation")
	}

	if retrievedGroup.Name != "Test Group" {
		t.Errorf("Expected group name 'Test Group', got '%s'", retrievedGroup.Name)
	}

	if retrievedGroup.CreatedBy != adminUserID {
		t.Errorf("Expected creator ID %d, got %d", adminUserID, retrievedGroup.CreatedBy)
	}

	// Verify deep-link can be generated
	deepLink := deepLinkService.GenerateGroupInviteLink(group.ID)
	if deepLink == "" {
		t.Error("Deep-link generation failed")
	}

	t.Logf("Group created successfully with ID %d and deep-link: %s", group.ID, deepLink)
}

// TestListGroupsCommand tests the /list_groups command
func TestListGroupsCommand(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	dbQueue, cleanup := setupTestDB(t)
	defer cleanup()

	// Create repositories
	groupRepo := storage.NewGroupRepository(dbQueue)

	// Create test config with admin user
	adminUserID := int64(12345)

	// Create multiple groups
	group1 := &domain.Group{
		TelegramChatID: 100,
		Name:           "Group 1",
		CreatedAt:      time.Now(),
		CreatedBy:      adminUserID,
	}
	err := groupRepo.CreateGroup(ctx, group1)
	if err != nil {
		t.Fatalf("Failed to create group 1: %v", err)
	}

	group2 := &domain.Group{
		TelegramChatID: 200,
		Name:           "Group 2",
		CreatedAt:      time.Now(),
		CreatedBy:      adminUserID,
	}
	err = groupRepo.CreateGroup(ctx, group2)
	if err != nil {
		t.Fatalf("Failed to create group 2: %v", err)
	}

	// Retrieve all groups
	groups, err := groupRepo.GetAllGroups(ctx)
	if err != nil {
		t.Fatalf("Failed to get all groups: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}

	// Verify groups are returned
	foundGroup1 := false
	foundGroup2 := false
	for _, g := range groups {
		if g.Name == "Group 1" {
			foundGroup1 = true
		}
		if g.Name == "Group 2" {
			foundGroup2 = true
		}
	}

	if !foundGroup1 {
		t.Error("Group 1 not found in list")
	}
	if !foundGroup2 {
		t.Error("Group 2 not found in list")
	}

	t.Logf("Successfully retrieved %d groups", len(groups))
}

// TestGroupMembersCommand tests the /group_members command
func TestGroupMembersCommand(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	dbQueue, cleanup := setupTestDB(t)
	defer cleanup()

	// Create repositories
	groupRepo := storage.NewGroupRepository(dbQueue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(dbQueue)
	ratingRepo := storage.NewRatingRepository(dbQueue)

	// Create test config with admin user
	adminUserID := int64(12345)

	// Create a group
	group := &domain.Group{
		TelegramChatID: 100,
		Name:           "Test Group",
		CreatedAt:      time.Now(),
		CreatedBy:      adminUserID,
	}
	err := groupRepo.CreateGroup(ctx, group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Add members to the group
	user1ID := int64(1001)
	user2ID := int64(1002)

	membership1 := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   user1ID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	err = groupMembershipRepo.CreateMembership(ctx, membership1)
	if err != nil {
		t.Fatalf("Failed to create membership 1: %v", err)
	}

	membership2 := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   user2ID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	err = groupMembershipRepo.CreateMembership(ctx, membership2)
	if err != nil {
		t.Fatalf("Failed to create membership 2: %v", err)
	}

	// Create ratings for members
	rating1 := &domain.Rating{
		UserID:   user1ID,
		GroupID:  group.ID,
		Username: "user1",
		Score:    100,
	}
	err = ratingRepo.UpdateRating(ctx, rating1)
	if err != nil {
		t.Fatalf("Failed to create rating 1: %v", err)
	}

	rating2 := &domain.Rating{
		UserID:   user2ID,
		GroupID:  group.ID,
		Username: "user2",
		Score:    200,
	}
	err = ratingRepo.UpdateRating(ctx, rating2)
	if err != nil {
		t.Fatalf("Failed to create rating 2: %v", err)
	}

	// Retrieve group members
	members, err := groupMembershipRepo.GetGroupMembers(ctx, group.ID)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if len(members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(members))
	}

	// Verify members are returned with correct status
	for _, member := range members {
		if member.Status != domain.MembershipStatusActive {
			t.Errorf("Expected active status, got %s", member.Status)
		}

		// Verify we can get rating for each member
		rating, err := ratingRepo.GetRating(ctx, member.UserID, group.ID)
		if err != nil {
			t.Errorf("Failed to get rating for user %d: %v", member.UserID, err)
		}
		if rating == nil {
			t.Errorf("Rating not found for user %d", member.UserID)
		}
	}

	t.Logf("Successfully retrieved %d members", len(members))
}

// TestRemoveMemberCommand tests the /remove_member command
func TestRemoveMemberCommand(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	dbQueue, cleanup := setupTestDB(t)
	defer cleanup()

	// Create repositories
	groupRepo := storage.NewGroupRepository(dbQueue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(dbQueue)

	// Create test config with admin user
	adminUserID := int64(12345)

	// Create a group
	group := &domain.Group{
		TelegramChatID: 100,
		Name:           "Test Group",
		CreatedAt:      time.Now(),
		CreatedBy:      adminUserID,
	}
	err := groupRepo.CreateGroup(ctx, group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Add a member to the group
	userID := int64(1001)
	membership := &domain.GroupMembership{
		GroupID:  group.ID,
		UserID:   userID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	err = groupMembershipRepo.CreateMembership(ctx, membership)
	if err != nil {
		t.Fatalf("Failed to create membership: %v", err)
	}

	// Verify member is active
	hasActiveMembership, err := groupMembershipRepo.HasActiveMembership(ctx, group.ID, userID)
	if err != nil {
		t.Fatalf("Failed to check active membership: %v", err)
	}
	if !hasActiveMembership {
		t.Fatal("Expected active membership")
	}

	// Remove the member
	err = groupMembershipRepo.UpdateMembershipStatus(ctx, group.ID, userID, domain.MembershipStatusRemoved)
	if err != nil {
		t.Fatalf("Failed to remove member: %v", err)
	}

	// Verify member is no longer active
	hasActiveMembership, err = groupMembershipRepo.HasActiveMembership(ctx, group.ID, userID)
	if err != nil {
		t.Fatalf("Failed to check active membership after removal: %v", err)
	}
	if hasActiveMembership {
		t.Error("Expected no active membership after removal")
	}

	// Verify membership still exists but with removed status
	retrievedMembership, err := groupMembershipRepo.GetMembership(ctx, group.ID, userID)
	if err != nil {
		t.Fatalf("Failed to get membership: %v", err)
	}
	if retrievedMembership == nil {
		t.Fatal("Membership should still exist after removal")
	}
	if retrievedMembership.Status != domain.MembershipStatusRemoved {
		t.Errorf("Expected removed status, got %s", retrievedMembership.Status)
	}

	t.Log("Successfully removed member and verified status")
}

// TestAdminPermissionValidation tests that non-admins cannot execute admin commands
func TestAdminPermissionValidation(t *testing.T) {
	// Create test config with admin user
	adminUserID := int64(12345)
	nonAdminUserID := int64(99999)
	cfg := &config.Config{
		AdminUserIDs: []int64{adminUserID},
	}

	// Create a minimal handler to test isAdmin
	handler := &BotHandler{
		config: cfg,
		logger: &mockLogger{},
	}

	if !handler.isAdmin(adminUserID) {
		t.Error("Expected admin user to be recognized as admin")
	}

	if handler.isAdmin(nonAdminUserID) {
		t.Error("Expected non-admin user to not be recognized as admin")
	}

	t.Log("Admin permission validation working correctly")
}
