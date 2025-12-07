package storage

import (
	"context"
	"database/sql"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
)

// GroupRepository handles group data operations
type GroupRepository struct {
	queue *DBQueue
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(queue *DBQueue) *GroupRepository {
	return &GroupRepository{queue: queue}
}

// CreateGroup creates a new group in the database
func (r *GroupRepository) CreateGroup(ctx context.Context, group *domain.Group) error {
	return r.queue.Execute(func(db *sql.DB) error {
		// Set default status if not provided
		if group.Status == "" {
			group.Status = domain.GroupStatusActive
		}

		result, err := db.ExecContext(ctx,
			`INSERT INTO groups (telegram_chat_id, name, created_at, created_by, is_forum, status) VALUES (?, ?, ?, ?, ?, ?)`,
			group.TelegramChatID, group.Name, group.CreatedAt, group.CreatedBy, group.IsForum, group.Status,
		)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		group.ID = id
		return nil
	})
}

// GetGroup retrieves a group by ID
func (r *GroupRepository) GetGroup(ctx context.Context, groupID int64) (*domain.Group, error) {
	var group domain.Group
	var status sql.NullString

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT id, telegram_chat_id, name, created_at, created_by, is_forum, COALESCE(status, 'active') FROM groups WHERE id = ?`,
			groupID,
		).Scan(&group.ID, &group.TelegramChatID, &group.Name, &group.CreatedAt, &group.CreatedBy, &group.IsForum, &status)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if status.Valid {
		group.Status = domain.GroupStatus(status.String)
	} else {
		group.Status = domain.GroupStatusActive
	}

	return &group, nil
}

// GetGroupByTelegramChatID retrieves a group by Telegram chat ID
func (r *GroupRepository) GetGroupByTelegramChatID(ctx context.Context, telegramChatID int64) (*domain.Group, error) {
	var group domain.Group
	var status sql.NullString

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT id, telegram_chat_id, name, created_at, created_by, is_forum, COALESCE(status, 'active') FROM groups WHERE telegram_chat_id = ?`,
			telegramChatID,
		).Scan(&group.ID, &group.TelegramChatID, &group.Name, &group.CreatedAt, &group.CreatedBy, &group.IsForum, &status)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if status.Valid {
		group.Status = domain.GroupStatus(status.String)
	} else {
		group.Status = domain.GroupStatusActive
	}

	return &group, nil
}

// GetAllGroups retrieves all groups
func (r *GroupRepository) GetAllGroups(ctx context.Context) ([]*domain.Group, error) {
	var groups []*domain.Group

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT id, telegram_chat_id, name, created_at, created_by, is_forum, COALESCE(status, 'active') FROM groups ORDER BY created_at DESC`,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var group domain.Group
			var status sql.NullString
			if err := rows.Scan(&group.ID, &group.TelegramChatID, &group.Name, &group.CreatedAt, &group.CreatedBy, &group.IsForum, &status); err != nil {
				return err
			}
			if status.Valid {
				group.Status = domain.GroupStatus(status.String)
			} else {
				group.Status = domain.GroupStatusActive
			}
			groups = append(groups, &group)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return groups, nil
}

// GetUserGroups retrieves all active groups where the user has active membership
func (r *GroupRepository) GetUserGroups(ctx context.Context, userID int64) ([]*domain.Group, error) {
	var groups []*domain.Group

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT g.id, g.telegram_chat_id, g.name, g.created_at, g.created_by, g.is_forum, COALESCE(g.status, 'active')
			 FROM groups g
			 INNER JOIN group_memberships gm ON g.id = gm.group_id
			 WHERE gm.user_id = ? AND gm.status = ? AND COALESCE(g.status, 'active') = ?
			 ORDER BY gm.joined_at DESC`,
			userID, domain.MembershipStatusActive, domain.GroupStatusActive,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var group domain.Group
			var status sql.NullString
			if err := rows.Scan(&group.ID, &group.TelegramChatID, &group.Name, &group.CreatedAt, &group.CreatedBy, &group.IsForum, &status); err != nil {
				return err
			}
			if status.Valid {
				group.Status = domain.GroupStatus(status.String)
			} else {
				group.Status = domain.GroupStatusActive
			}
			groups = append(groups, &group)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return groups, nil
}

// DeleteGroup deletes a group by ID (hard delete)
func (r *GroupRepository) DeleteGroup(ctx context.Context, groupID int64) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, groupID)
		return err
	})
}

// UpdateGroupStatus updates the status of a group (soft delete/restore)
func (r *GroupRepository) UpdateGroupStatus(ctx context.Context, groupID int64, status domain.GroupStatus) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `UPDATE groups SET status = ? WHERE id = ?`, status, groupID)
		return err
	})
}

// UpdateGroupName updates the name of a group
func (r *GroupRepository) UpdateGroupName(ctx context.Context, groupID int64, name string) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `UPDATE groups SET name = ? WHERE id = ?`, name, groupID)
		return err
	})
}
