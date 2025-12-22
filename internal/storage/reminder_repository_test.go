package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestReminderRepository_OrganizerNotifications(t *testing.T) {
	// Create in-memory database for testing
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	queue := NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	repo := NewReminderRepository(queue)

	ctx := context.Background()
	eventID := int64(1)

	// Initially, no organizer notification should be sent
	sent, err := repo.WasOrganizerNotificationSent(ctx, eventID)
	if err != nil {
		t.Fatalf("WasOrganizerNotificationSent failed: %v", err)
	}
	if sent {
		t.Error("Expected organizer notification not to be sent initially")
	}

	// Mark organizer notification as sent
	err = repo.MarkOrganizerNotificationSent(ctx, eventID)
	if err != nil {
		t.Fatalf("MarkOrganizerNotificationSent failed: %v", err)
	}

	// Now it should be marked as sent
	sent, err = repo.WasOrganizerNotificationSent(ctx, eventID)
	if err != nil {
		t.Fatalf("WasOrganizerNotificationSent failed: %v", err)
	}
	if !sent {
		t.Error("Expected organizer notification to be marked as sent")
	}

	// Test that regular reminders and organizer notifications are independent
	reminderSent, err := repo.WasReminderSent(ctx, eventID)
	if err != nil {
		t.Fatalf("WasReminderSent failed: %v", err)
	}
	if reminderSent {
		t.Error("Expected regular reminder not to be sent")
	}
}
