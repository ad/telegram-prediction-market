package storage

import (
	"context"
	"database/sql"
	"time"
)

// ReminderRepository handles reminder log operations
type ReminderRepository struct {
	queue *DBQueue
}

// NewReminderRepository creates a new ReminderRepository
func NewReminderRepository(queue *DBQueue) *ReminderRepository {
	return &ReminderRepository{queue: queue}
}

// WasReminderSent checks if a reminder was already sent for an event
func (r *ReminderRepository) WasReminderSent(ctx context.Context, eventID int64) (bool, error) {
	var exists bool

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM reminder_log WHERE event_id = ?)`,
			eventID,
		).Scan(&exists)
	})

	if err != nil {
		return false, err
	}

	return exists, nil
}

// MarkReminderSent marks a reminder as sent for an event
func (r *ReminderRepository) MarkReminderSent(ctx context.Context, eventID int64) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO reminder_log (event_id, sent_at) VALUES (?, ?)
			 ON CONFLICT(event_id) DO UPDATE SET sent_at = excluded.sent_at`,
			eventID, time.Now(),
		)
		return err
	})
}

// WasOrganizerNotificationSent checks if an organizer notification was already sent for an event
func (r *ReminderRepository) WasOrganizerNotificationSent(ctx context.Context, eventID int64) (bool, error) {
	var exists bool

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM organizer_notifications WHERE event_id = ?)`,
			eventID,
		).Scan(&exists)
	})

	if err != nil {
		return false, err
	}

	return exists, nil
}

// MarkOrganizerNotificationSent marks an organizer notification as sent for an event
func (r *ReminderRepository) MarkOrganizerNotificationSent(ctx context.Context, eventID int64) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO organizer_notifications (event_id, sent_at) VALUES (?, ?)
			 ON CONFLICT(event_id) DO UPDATE SET sent_at = excluded.sent_at`,
			eventID, time.Now(),
		)
		return err
	})
}
