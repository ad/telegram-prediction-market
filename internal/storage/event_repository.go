package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
)

// EventRepository handles event data operations
type EventRepository struct {
	queue *DBQueue
}

// NewEventRepository creates a new EventRepository
func NewEventRepository(queue *DBQueue) *EventRepository {
	return &EventRepository{queue: queue}
}

// CreateEvent creates a new event in the database
func (r *EventRepository) CreateEvent(ctx context.Context, event *domain.Event) error {
	return r.queue.Execute(func(db *sql.DB) error {
		optionsJSON, err := json.Marshal(event.Options)
		if err != nil {
			return err
		}

		result, err := db.ExecContext(ctx,
			`INSERT INTO events (group_id, question, options_json, created_at, deadline, status, event_type, created_by, poll_id, poll_message_id, message_thread_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			event.GroupID, event.Question, optionsJSON, event.CreatedAt, event.Deadline,
			event.Status, event.EventType, event.CreatedBy, event.PollID, event.PollMessageID, event.MessageThreadID,
		)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		event.ID = id
		return nil
	})
}

// GetEvent retrieves an event by ID
func (r *EventRepository) GetEvent(ctx context.Context, eventID int64) (*domain.Event, error) {
	var event domain.Event
	var optionsJSON string
	var correctOption sql.NullInt64
	var pollID sql.NullString
	var pollMessageID sql.NullInt64
	var messageThreadID sql.NullInt64

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT id, group_id, question, options_json, created_at, deadline, status, event_type, correct_option, created_by, poll_id, poll_message_id, message_thread_id
			 FROM events WHERE id = ?`,
			eventID,
		).Scan(
			&event.ID, &event.GroupID, &event.Question, &optionsJSON, &event.CreatedAt,
			&event.Deadline, &event.Status, &event.EventType, &correctOption, &event.CreatedBy, &pollID, &pollMessageID, &messageThreadID,
		)
	})

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(optionsJSON), &event.Options); err != nil {
		return nil, err
	}

	if correctOption.Valid {
		val := int(correctOption.Int64)
		event.CorrectOption = &val
	}

	if pollID.Valid {
		event.PollID = pollID.String
	}

	if pollMessageID.Valid {
		event.PollMessageID = int(pollMessageID.Int64)
	}

	if messageThreadID.Valid {
		val := int(messageThreadID.Int64)
		event.MessageThreadID = &val
	}

	return &event, nil
}

// GetActiveEvents retrieves all active events for a specific group
func (r *EventRepository) GetActiveEvents(ctx context.Context, groupID int64) ([]*domain.Event, error) {
	var events []*domain.Event

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT id, group_id, question, options_json, created_at, deadline, status, event_type, correct_option, created_by, poll_id, poll_message_id, message_thread_id
			 FROM events WHERE status = ? AND group_id = ? ORDER BY created_at DESC`,
			domain.EventStatusActive, groupID,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var event domain.Event
			var optionsJSON string
			var correctOption sql.NullInt64
			var pollID sql.NullString
			var pollMessageID sql.NullInt64
			var messageThreadID sql.NullInt64

			if err := rows.Scan(
				&event.ID, &event.GroupID, &event.Question, &optionsJSON, &event.CreatedAt,
				&event.Deadline, &event.Status, &event.EventType, &correctOption, &event.CreatedBy, &pollID, &pollMessageID, &messageThreadID,
			); err != nil {
				return err
			}

			if err := json.Unmarshal([]byte(optionsJSON), &event.Options); err != nil {
				return err
			}

			if correctOption.Valid {
				val := int(correctOption.Int64)
				event.CorrectOption = &val
			}

			if pollID.Valid {
				event.PollID = pollID.String
			}

			if pollMessageID.Valid {
				event.PollMessageID = int(pollMessageID.Int64)
			}

			if messageThreadID.Valid {
				val := int(messageThreadID.Int64)
				event.MessageThreadID = &val
			}

			events = append(events, &event)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return events, nil
}

// UpdateEvent updates an existing event
func (r *EventRepository) UpdateEvent(ctx context.Context, event *domain.Event) error {
	return r.queue.Execute(func(db *sql.DB) error {
		optionsJSON, err := json.Marshal(event.Options)
		if err != nil {
			return err
		}

		var correctOption interface{}
		if event.CorrectOption != nil {
			correctOption = *event.CorrectOption
		}

		_, err = db.ExecContext(ctx,
			`UPDATE events SET group_id = ?, question = ?, options_json = ?, deadline = ?, status = ?, correct_option = ?, poll_id = ?, poll_message_id = ?, message_thread_id = ?
			 WHERE id = ?`,
			event.GroupID, event.Question, optionsJSON, event.Deadline, event.Status, correctOption, event.PollID, event.PollMessageID, event.MessageThreadID, event.ID,
		)
		return err
	})
}

// ResolveEvent marks an event as resolved with the correct option
func (r *EventRepository) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx,
			`UPDATE events SET status = ?, correct_option = ? WHERE id = ?`,
			domain.EventStatusResolved, correctOption, eventID,
		)
		return err
	})
}

// GetEventsByDeadlineRange retrieves events with deadline in the specified range
func (r *EventRepository) GetEventsByDeadlineRange(ctx context.Context, start, end time.Time) ([]*domain.Event, error) {
	var events []*domain.Event

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT id, group_id, question, options_json, created_at, deadline, status, event_type, correct_option, created_by, poll_id, poll_message_id, message_thread_id
			 FROM events WHERE deadline BETWEEN ? AND ? ORDER BY deadline ASC`,
			start, end,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var event domain.Event
			var optionsJSON string
			var correctOption sql.NullInt64
			var pollID sql.NullString
			var pollMessageID sql.NullInt64
			var messageThreadID sql.NullInt64

			if err := rows.Scan(
				&event.ID, &event.GroupID, &event.Question, &optionsJSON, &event.CreatedAt,
				&event.Deadline, &event.Status, &event.EventType, &correctOption, &event.CreatedBy, &pollID, &pollMessageID, &messageThreadID,
			); err != nil {
				return err
			}

			if err := json.Unmarshal([]byte(optionsJSON), &event.Options); err != nil {
				return err
			}

			if correctOption.Valid {
				val := int(correctOption.Int64)
				event.CorrectOption = &val
			}

			if pollID.Valid {
				event.PollID = pollID.String
			}

			if pollMessageID.Valid {
				event.PollMessageID = int(pollMessageID.Int64)
			}

			if messageThreadID.Valid {
				val := int(messageThreadID.Int64)
				event.MessageThreadID = &val
			}

			events = append(events, &event)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return events, nil
}

// GetEventByPollID retrieves an event by its Telegram poll ID
func (r *EventRepository) GetEventByPollID(ctx context.Context, pollID string) (*domain.Event, error) {
	var event domain.Event
	var optionsJSON string
	var correctOption sql.NullInt64
	var pollIDNull sql.NullString
	var pollMessageID sql.NullInt64
	var messageThreadID sql.NullInt64

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT id, group_id, question, options_json, created_at, deadline, status, event_type, correct_option, created_by, poll_id, poll_message_id, message_thread_id
			 FROM events WHERE poll_id = ?`,
			pollID,
		).Scan(
			&event.ID, &event.GroupID, &event.Question, &optionsJSON, &event.CreatedAt,
			&event.Deadline, &event.Status, &event.EventType, &correctOption, &event.CreatedBy, &pollIDNull, &pollMessageID, &messageThreadID,
		)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(optionsJSON), &event.Options); err != nil {
		return nil, err
	}

	if correctOption.Valid {
		val := int(correctOption.Int64)
		event.CorrectOption = &val
	}

	if pollIDNull.Valid {
		event.PollID = pollIDNull.String
	}

	if pollMessageID.Valid {
		event.PollMessageID = int(pollMessageID.Int64)
	}

	if messageThreadID.Valid {
		val := int(messageThreadID.Int64)
		event.MessageThreadID = &val
	}

	return &event, nil
}

// GetResolvedEvents retrieves all resolved events
func (r *EventRepository) GetResolvedEvents(ctx context.Context) ([]*domain.Event, error) {
	var events []*domain.Event

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT id, group_id, question, options_json, created_at, deadline, status, event_type, correct_option, created_by, poll_id, poll_message_id, message_thread_id
			 FROM events WHERE status = ? ORDER BY created_at DESC`,
			domain.EventStatusResolved,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var event domain.Event
			var optionsJSON string
			var correctOption sql.NullInt64
			var pollID sql.NullString
			var pollMessageID sql.NullInt64
			var messageThreadID sql.NullInt64

			if err := rows.Scan(
				&event.ID, &event.GroupID, &event.Question, &optionsJSON, &event.CreatedAt,
				&event.Deadline, &event.Status, &event.EventType, &correctOption, &event.CreatedBy, &pollID, &pollMessageID, &messageThreadID,
			); err != nil {
				return err
			}

			if err := json.Unmarshal([]byte(optionsJSON), &event.Options); err != nil {
				return err
			}

			if correctOption.Valid {
				val := int(correctOption.Int64)
				event.CorrectOption = &val
			}

			if pollID.Valid {
				event.PollID = pollID.String
			}

			if pollMessageID.Valid {
				event.PollMessageID = int(pollMessageID.Int64)
			}

			if messageThreadID.Valid {
				val := int(messageThreadID.Int64)
				event.MessageThreadID = &val
			}

			events = append(events, &event)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return events, nil
}

// GetUserCreatedEventsCount counts events created by user in a specific group
func (r *EventRepository) GetUserCreatedEventsCount(ctx context.Context, userID int64, groupID int64) (int, error) {
	var count int

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM events WHERE created_by = ? AND group_id = ?`,
			userID, groupID,
		).Scan(&count)
	})

	if err != nil {
		return 0, err
	}

	return count, nil
}
