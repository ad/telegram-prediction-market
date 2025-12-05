package storage

import (
	"context"
	"database/sql"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
)

// RatingRepository handles rating data operations
type RatingRepository struct {
	queue *DBQueue
}

// NewRatingRepository creates a new RatingRepository
func NewRatingRepository(queue *DBQueue) *RatingRepository {
	return &RatingRepository{queue: queue}
}

// GetRating retrieves a user's rating
func (r *RatingRepository) GetRating(ctx context.Context, userID int64) (*domain.Rating, error) {
	var rating domain.Rating

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT user_id, username, score, correct_count, wrong_count, streak
			 FROM ratings WHERE user_id = ?`,
			userID,
		).Scan(
			&rating.UserID, &rating.Username, &rating.Score, &rating.CorrectCount,
			&rating.WrongCount, &rating.Streak,
		)
	})

	if err == sql.ErrNoRows {
		// Return a new rating with zero values
		return &domain.Rating{
			UserID:       userID,
			Username:     "",
			Score:        0,
			CorrectCount: 0,
			WrongCount:   0,
			Streak:       0,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &rating, nil
}

// UpdateRating updates or inserts a user's rating
func (r *RatingRepository) UpdateRating(ctx context.Context, rating *domain.Rating) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO ratings (user_id, username, score, correct_count, wrong_count, streak)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(user_id) DO UPDATE SET
			   username = excluded.username,
			   score = excluded.score,
			   correct_count = excluded.correct_count,
			   wrong_count = excluded.wrong_count,
			   streak = excluded.streak`,
			rating.UserID, rating.Username, rating.Score, rating.CorrectCount,
			rating.WrongCount, rating.Streak,
		)
		return err
	})
}

// GetTopRatings retrieves the top N users by score
func (r *RatingRepository) GetTopRatings(ctx context.Context, limit int) ([]*domain.Rating, error) {
	var ratings []*domain.Rating

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT user_id, username, score, correct_count, wrong_count, streak
			 FROM ratings ORDER BY score DESC LIMIT ?`,
			limit,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var rating domain.Rating
			if err := rows.Scan(
				&rating.UserID, &rating.Username, &rating.Score, &rating.CorrectCount,
				&rating.WrongCount, &rating.Streak,
			); err != nil {
				return err
			}
			ratings = append(ratings, &rating)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return ratings, nil
}

// UpdateStreak updates a user's streak
func (r *RatingRepository) UpdateStreak(ctx context.Context, userID int64, streak int) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx,
			`UPDATE ratings SET streak = ? WHERE user_id = ?`,
			streak, userID,
		)
		return err
	})
}
