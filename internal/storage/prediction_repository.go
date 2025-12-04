package storage

import (
	"context"
	"database/sql"

	"telegram-prediction-bot/internal/domain"
)

// PredictionRepository handles prediction data operations
type PredictionRepository struct {
	queue *DBQueue
}

// NewPredictionRepository creates a new PredictionRepository
func NewPredictionRepository(queue *DBQueue) *PredictionRepository {
	return &PredictionRepository{queue: queue}
}

// SavePrediction saves a new prediction to the database
func (r *PredictionRepository) SavePrediction(ctx context.Context, prediction *domain.Prediction) error {
	return r.queue.Execute(func(db *sql.DB) error {
		result, err := db.ExecContext(ctx,
			`INSERT INTO predictions (event_id, user_id, option, timestamp)
			 VALUES (?, ?, ?, ?)`,
			prediction.EventID, prediction.UserID, prediction.Option, prediction.Timestamp,
		)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		prediction.ID = id
		return nil
	})
}

// UpdatePrediction updates an existing prediction
func (r *PredictionRepository) UpdatePrediction(ctx context.Context, prediction *domain.Prediction) error {
	return r.queue.Execute(func(db *sql.DB) error {
		_, err := db.ExecContext(ctx,
			`UPDATE predictions SET option = ?, timestamp = ? WHERE event_id = ? AND user_id = ?`,
			prediction.Option, prediction.Timestamp, prediction.EventID, prediction.UserID,
		)
		return err
	})
}

// GetPredictionsByEvent retrieves all predictions for a specific event
func (r *PredictionRepository) GetPredictionsByEvent(ctx context.Context, eventID int64) ([]*domain.Prediction, error) {
	var predictions []*domain.Prediction

	err := r.queue.Execute(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx,
			`SELECT id, event_id, user_id, option, timestamp
			 FROM predictions WHERE event_id = ? ORDER BY timestamp ASC`,
			eventID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var prediction domain.Prediction
			if err := rows.Scan(
				&prediction.ID, &prediction.EventID, &prediction.UserID,
				&prediction.Option, &prediction.Timestamp,
			); err != nil {
				return err
			}
			predictions = append(predictions, &prediction)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, err
	}

	return predictions, nil
}

// GetPredictionByUserAndEvent retrieves a specific prediction by user and event
func (r *PredictionRepository) GetPredictionByUserAndEvent(ctx context.Context, userID, eventID int64) (*domain.Prediction, error) {
	var prediction domain.Prediction

	err := r.queue.Execute(func(db *sql.DB) error {
		return db.QueryRowContext(ctx,
			`SELECT id, event_id, user_id, option, timestamp
			 FROM predictions WHERE user_id = ? AND event_id = ?`,
			userID, eventID,
		).Scan(
			&prediction.ID, &prediction.EventID, &prediction.UserID,
			&prediction.Option, &prediction.Timestamp,
		)
	})

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &prediction, nil
}
