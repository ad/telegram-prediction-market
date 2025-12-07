package storage

import (
	"context"
	"database/sql"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
)

// ForumTopicRepository handles forum topic data operations
type ForumTopicRepository struct {
	queue *DBQueue
}

// NewForumTopicRepository creates a new ForumTopicRepository
func NewForumTopicRepository(queue *DBQueue) *ForumTopicRepository {
	return &ForumTopicRepository{queue: queue}
}

// CreateForumTopic creates a new forum topic in the database
func (r *ForumTopicRepository) CreateForumTopic(ctx context.Context, topic *domain.ForumTopic) error {
	return r.queue.Execute(func(db *sql.DB) error {
		result, err := db.ExecContext(ctx,
			`INSERT INTO forum_topics (group_id, message_thread_id, name, created_at, created_by) VALUES (?, ?, ?, ?, ?)`,
			topic.GroupID, topic.MessageThreadID, topic.Name, topic.CreatedAt, topic.CreatedBy,
		)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		topic.ID = id
		return nil
	})
}

// GetForumTopic retrieves a forum topic by ID
func (r *ForumTopicRepository) GetForumTopic(ctx context.Context, topicID int64) (*domain.ForumTopic, error) {
	var topic domain.ForumTopic

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT id, group_id, message_thread_id, name, created_at, created_by FROM forum_topics WHERE id = ?`,
			topicID,
		).Scan(&topic.ID, &topic.GroupID, &topic.MessageThreadID, &topic.Name, &topic.CreatedAt, &topic.CreatedBy)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &topic, nil
}

// GetForumTopicByGroupAndThread retrieves a forum topic by group ID and message thread ID
func (r *ForumTopicRepository) GetForumTopicByGroupAndThread(ctx context.Context, groupID int64, messageThreadID int) (*domain.ForumTopic, error) {
	var topic domain.ForumTopic

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT id, group_id, message_thread_id, name, created_at, created_by FROM forum_topics WHERE group_id = ? AND message_thread_id = ?`,
			groupID, messageThreadID,
		).Scan(&topic.ID, &topic.GroupID, &topic.MessageThreadID, &topic.Name, &topic.CreatedAt, &topic.CreatedBy)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &topic, nil
}

// GetForumTopicsByGroup retrieves all forum topics for a group
func (r *ForumTopicRepository) GetForumTopicsByGroup(ctx context.Context, groupID int64) ([]*domain.ForumTopic, error) {
	var topics []*domain.ForumTopic

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT id, group_id, message_thread_id, name, created_at, created_by FROM forum_topics WHERE group_id = ? ORDER BY created_at DESC`,
			groupID,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var topic domain.ForumTopic
			if err := rows.Scan(&topic.ID, &topic.GroupID, &topic.MessageThreadID, &topic.Name, &topic.CreatedAt, &topic.CreatedBy); err != nil {
				return err
			}
			topics = append(topics, &topic)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return topics, nil
}

// DeleteForumTopic deletes a forum topic by ID
func (r *ForumTopicRepository) DeleteForumTopic(ctx context.Context, topicID int64) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `DELETE FROM forum_topics WHERE id = ?`, topicID)
		return err
	})
}
