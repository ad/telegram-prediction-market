package bot

import (
	"context"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"
)

// For any user without active membership in an event's group, attempting to vote on that event should be rejected
func TestVotePermissionValidation_Property(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	dbQueue, cleanup := setupTestDB(t)
	defer cleanup()

	groupRepo := storage.NewGroupRepository(dbQueue)
	membershipRepo := storage.NewGroupMembershipRepository(dbQueue)
	eventRepo := storage.NewEventRepository(dbQueue)

	// Create test groups
	group1 := &domain.Group{
		TelegramChatID: 1001,
		Name:           "Test Group 1",
		CreatedAt:      time.Now(),
		CreatedBy:      100,
	}
	group2 := &domain.Group{
		TelegramChatID: 1002,
		Name:           "Test Group 2",
		CreatedAt:      time.Now(),
		CreatedBy:      100,
	}

	// Create test users
	user1ID := int64(200)
	user2ID := int64(201)

	// Create groups
	if err := groupRepo.CreateGroup(ctx, group1); err != nil {
		t.Fatalf("Failed to create group1: %v", err)
	}
	if err := groupRepo.CreateGroup(ctx, group2); err != nil {
		t.Fatalf("Failed to create group2: %v", err)
	}

	// User1 is member of group1 only
	membership1 := &domain.GroupMembership{
		GroupID:  group1.ID,
		UserID:   user1ID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := membershipRepo.CreateMembership(ctx, membership1); err != nil {
		t.Fatalf("Failed to create membership1: %v", err)
	}

	// User2 is member of group2 only
	membership2 := &domain.GroupMembership{
		GroupID:  group2.ID,
		UserID:   user2ID,
		JoinedAt: time.Now(),
		Status:   domain.MembershipStatusActive,
	}
	if err := membershipRepo.CreateMembership(ctx, membership2); err != nil {
		t.Fatalf("Failed to create membership2: %v", err)
	}

	// Create event in group1
	event := &domain.Event{
		GroupID:   group1.ID,
		Question:  "Test Question",
		Options:   []string{"Yes", "No"},
		CreatedAt: time.Now(),
		Deadline:  time.Now().Add(24 * time.Hour),
		Status:    domain.EventStatusActive,
		EventType: domain.EventTypeBinary,
		CreatedBy: user1ID,
	}
	if err := eventRepo.CreateEvent(ctx, event); err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Property Test: User1 (member of group1) should be able to vote
	hasAccess1, err := membershipRepo.HasActiveMembership(ctx, event.GroupID, user1ID)
	if err != nil {
		t.Fatalf("Failed to check membership for user1: %v", err)
	}
	if !hasAccess1 {
		t.Errorf("Property violated: User1 is member of group1 but HasActiveMembership returned false")
	}

	// Property Test: User2 (NOT member of group1) should NOT be able to vote
	hasAccess2, err := membershipRepo.HasActiveMembership(ctx, event.GroupID, user2ID)
	if err != nil {
		t.Fatalf("Failed to check membership for user2: %v", err)
	}
	if hasAccess2 {
		t.Errorf("Property violated: User2 is NOT member of group1 but HasActiveMembership returned true")
	}

	// Property Test: User with removed membership should NOT be able to vote
	if err := membershipRepo.UpdateMembershipStatus(ctx, group1.ID, user1ID, domain.MembershipStatusRemoved); err != nil {
		t.Fatalf("Failed to update membership status: %v", err)
	}
	hasAccessRemoved, err := membershipRepo.HasActiveMembership(ctx, event.GroupID, user1ID)
	if err != nil {
		t.Fatalf("Failed to check membership for removed user: %v", err)
	}
	if hasAccessRemoved {
		t.Errorf("Property violated: User1 has removed membership but HasActiveMembership returned true")
	}
}
