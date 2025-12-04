package storage

import (
	"context"
	"database/sql"

	"telegram-prediction-bot/internal/domain"
)

// AchievementRepository handles achievement data operations
type AchievementRepository struct {
	queue *DBQueue
}

// NewAchievementRepository creates a new AchievementRepository
func NewAchievementRepository(queue *DBQueue) *AchievementRepository {
	return &AchievementRepository{queue: queue}
}

// SaveAchievement saves a new achievement to the database
func (r *AchievementRepository) SaveAchievement(ctx context.Context, achievement *domain.Achievement) error {
	return r.queue.Execute(func(db *sql.DB) error {
		result, err := db.ExecContext(ctx,
			`INSERT INTO achievements (user_id, code, timestamp)
			 VALUES (?, ?, ?)`,
			achievement.UserID, achievement.Code, achievement.Timestamp,
		)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		achievement.ID = id
		return nil
	})
}

// GetUserAchievements retrieves all achievements for a specific user
func (r *AchievementRepository) GetUserAchievements(ctx context.Context, userID int64) ([]*domain.Achievement, error) {
	var achievements []*domain.Achievement

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT id, user_id, code, timestamp
			 FROM achievements WHERE user_id = ? ORDER BY timestamp DESC`,
			userID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var achievement domain.Achievement
			if err := rows.Scan(
				&achievement.ID, &achievement.UserID,
				&achievement.Code, &achievement.Timestamp,
			); err != nil {
				return err
			}
			achievements = append(achievements, &achievement)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return achievements, nil
}

// CheckAchievementExists checks if a user already has a specific achievement
func (r *AchievementRepository) CheckAchievementExists(ctx context.Context, userID int64, code domain.AchievementCode) (bool, error) {
	var exists bool

	err := r.queue.Execute(func(db *sql.DB) error {
		var count int
		err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM achievements WHERE user_id = ? AND code = ?`,
			userID, code,
		).Scan(&count)
		if err != nil {
			return err
		}
		exists = count > 0
		return nil
	})

	if err != nil {
		return false, err
	}

	return exists, nil
}
