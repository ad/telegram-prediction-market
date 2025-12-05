package storage

import (
	"context"
	"database/sql"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
)

// GroupMembershipRepository handles group membership data operations
type GroupMembershipRepository struct {
	queue *DBQueue
}

// NewGroupMembershipRepository creates a new GroupMembershipRepository
func NewGroupMembershipRepository(queue *DBQueue) *GroupMembershipRepository {
	return &GroupMembershipRepository{queue: queue}
}

// CreateMembership creates a new group membership in the database
func (r *GroupMembershipRepository) CreateMembership(ctx context.Context, membership *domain.GroupMembership) error {
	return r.queue.Execute(func(db *sql.DB) error {
		result, err := db.ExecContext(ctx,
			`INSERT INTO group_memberships (group_id, user_id, joined_at, status) VALUES (?, ?, ?, ?)`,
			membership.GroupID, membership.UserID, membership.JoinedAt, membership.Status,
		)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		membership.ID = id
		return nil
	})
}

// GetMembership retrieves a membership by group ID and user ID
func (r *GroupMembershipRepository) GetMembership(ctx context.Context, groupID int64, userID int64) (*domain.GroupMembership, error) {
	var membership domain.GroupMembership

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT id, group_id, user_id, joined_at, status FROM group_memberships WHERE group_id = ? AND user_id = ?`,
			groupID, userID,
		).Scan(&membership.ID, &membership.GroupID, &membership.UserID, &membership.JoinedAt, &membership.Status)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &membership, nil
}

// GetGroupMembers retrieves all members of a group ordered by join date (most recent first)
func (r *GroupMembershipRepository) GetGroupMembers(ctx context.Context, groupID int64) ([]*domain.GroupMembership, error) {
	var memberships []*domain.GroupMembership

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT id, group_id, user_id, joined_at, status FROM group_memberships WHERE group_id = ? ORDER BY joined_at DESC`,
			groupID,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var membership domain.GroupMembership
			if err := rows.Scan(&membership.ID, &membership.GroupID, &membership.UserID, &membership.JoinedAt, &membership.Status); err != nil {
				return err
			}
			memberships = append(memberships, &membership)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return memberships, nil
}

// UpdateMembershipStatus updates the status of a membership
func (r *GroupMembershipRepository) UpdateMembershipStatus(ctx context.Context, groupID int64, userID int64, status domain.MembershipStatus) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx,
			`UPDATE group_memberships SET status = ? WHERE group_id = ? AND user_id = ?`,
			status, groupID, userID,
		)
		return err
	})
}

// HasActiveMembership checks if a user has an active membership in a group
func (r *GroupMembershipRepository) HasActiveMembership(ctx context.Context, groupID int64, userID int64) (bool, error) {
	var count int

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM group_memberships WHERE group_id = ? AND user_id = ? AND status = ?`,
			groupID, userID, domain.MembershipStatusActive,
		).Scan(&count)
	})

	if err != nil {
		return false, err
	}

	return count > 0, nil
}
