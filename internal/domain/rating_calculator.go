package domain

import (
	"context"
	"time"
)

const (
	// Scoring constants based on requirements
	BinaryCorrectPoints      = 10
	MultiOptionCorrectPoints = 15
	MinorityBonusPoints      = 5
	EarlyVotingBonusPoints   = 3
	ParticipationPoints      = 1
	IncorrectPenalty         = -3
	MinorityThreshold        = 0.4            // 40% threshold for minority bonus
	EarlyVotingWindow        = 12 * time.Hour // 12 hours for early voting bonus
)

// RatingRepository interface for rating operations
type RatingRepository interface {
	GetRating(ctx context.Context, userID int64, groupID int64) (*Rating, error)
	UpdateRating(ctx context.Context, rating *Rating) error
	GetTopRatings(ctx context.Context, groupID int64, limit int) ([]*Rating, error)
	UpdateStreak(ctx context.Context, userID int64, groupID int64, streak int) error
}

// RatingCalculator handles rating calculations and updates
type RatingCalculator struct {
	ratingRepo     RatingRepository
	predictionRepo PredictionRepository
	eventRepo      EventRepository
	logger         Logger
}

// NewRatingCalculator creates a new RatingCalculator
func NewRatingCalculator(
	ratingRepo RatingRepository,
	predictionRepo PredictionRepository,
	eventRepo EventRepository,
	logger Logger,
) *RatingCalculator {
	return &RatingCalculator{
		ratingRepo:     ratingRepo,
		predictionRepo: predictionRepo,
		eventRepo:      eventRepo,
		logger:         logger,
	}
}

// CalculateScores calculates and updates scores for all participants of an event
func (rc *RatingCalculator) CalculateScores(ctx context.Context, eventID int64, correctOption int) error {
	// Get the event
	event, err := rc.eventRepo.GetEvent(ctx, eventID)
	if err != nil {
		rc.logger.Error("failed to get event", "event_id", eventID, "error", err)
		return err
	}

	// Get all predictions for this event
	predictions, err := rc.predictionRepo.GetPredictionsByEvent(ctx, eventID)
	if err != nil {
		rc.logger.Error("failed to get predictions", "event_id", eventID, "error", err)
		return err
	}

	if len(predictions) == 0 {
		rc.logger.Info("no predictions for event", "event_id", eventID)
		return nil
	}

	// Calculate vote distribution for minority bonus
	voteDistribution := make(map[int]int)
	for _, pred := range predictions {
		voteDistribution[pred.Option]++
	}
	totalVotes := len(predictions)

	// Process each prediction
	for _, pred := range predictions {
		isCorrect := pred.Option == correctOption

		// Calculate points for this prediction
		points := rc.calculatePoints(event, pred, isCorrect, voteDistribution, totalVotes)

		// Get current rating
		rating, err := rc.ratingRepo.GetRating(ctx, pred.UserID)
		if err != nil {
			rc.logger.Error("failed to get rating", "user_id", pred.UserID, "error", err)
			continue
		}

		// Update rating
		rating.Score += points

		if isCorrect {
			rating.CorrectCount++
			rating.Streak++
		} else {
			rating.WrongCount++
			rating.Streak = 0
		}

		// Save updated rating
		if err := rc.ratingRepo.UpdateRating(ctx, rating); err != nil {
			rc.logger.Error("failed to update rating", "user_id", pred.UserID, "error", err)
			continue
		}

		rc.logger.Info("updated rating",
			"user_id", pred.UserID,
			"points", points,
			"new_score", rating.Score,
			"streak", rating.Streak,
		)
	}

	return nil
}

// calculatePoints calculates points for a single prediction
func (rc *RatingCalculator) calculatePoints(
	event *Event,
	prediction *Prediction,
	isCorrect bool,
	voteDistribution map[int]int,
	totalVotes int,
) int {
	points := ParticipationPoints // Everyone gets participation point

	if !isCorrect {
		// Incorrect prediction penalty
		points += IncorrectPenalty
		return points
	}

	// Base points for correct prediction
	switch event.EventType {
	case EventTypeBinary:
		points += BinaryCorrectPoints
	case EventTypeMultiOption, EventTypeProbability:
		points += MultiOptionCorrectPoints
	}

	// Minority bonus
	optionVotes := voteDistribution[prediction.Option]
	if totalVotes > 0 {
		percentage := float64(optionVotes) / float64(totalVotes)
		if percentage < MinorityThreshold {
			points += MinorityBonusPoints
			rc.logger.Debug("minority bonus awarded",
				"user_id", prediction.UserID,
				"percentage", percentage,
			)
		}
	}

	// Early voting bonus
	timeSinceCreation := prediction.Timestamp.Sub(event.CreatedAt)
	if timeSinceCreation <= EarlyVotingWindow {
		points += EarlyVotingBonusPoints
		rc.logger.Debug("early voting bonus awarded",
			"user_id", prediction.UserID,
			"time_since_creation", timeSinceCreation,
		)
	}

	return points
}

// GetTopRatings retrieves the top N users by score
func (rc *RatingCalculator) GetTopRatings(ctx context.Context, limit int) ([]*Rating, error) {
	ratings, err := rc.ratingRepo.GetTopRatings(ctx, limit)
	if err != nil {
		rc.logger.Error("failed to get top ratings", "limit", limit, "error", err)
		return nil, err
	}

	return ratings, nil
}

// GetUserRating retrieves a specific user's rating
func (rc *RatingCalculator) GetUserRating(ctx context.Context, userID int64) (*Rating, error) {
	rating, err := rc.ratingRepo.GetRating(ctx, userID)
	if err != nil {
		rc.logger.Error("failed to get user rating", "user_id", userID, "error", err)
		return nil, err
	}

	return rating, nil
}

// UpdateStreak updates a user's streak
func (rc *RatingCalculator) UpdateStreak(ctx context.Context, userID int64, correct bool) error {
	rating, err := rc.ratingRepo.GetRating(ctx, userID)
	if err != nil {
		rc.logger.Error("failed to get rating for streak update", "user_id", userID, "error", err)
		return err
	}

	if correct {
		rating.Streak++
	} else {
		rating.Streak = 0
	}

	if err := rc.ratingRepo.UpdateStreak(ctx, userID, rating.Streak); err != nil {
		rc.logger.Error("failed to update streak", "user_id", userID, "error", err)
		return err
	}

	rc.logger.Info("streak updated", "user_id", userID, "streak", rating.Streak)
	return nil
}

// UpdateRatingUsername updates a user's rating (including username)
func (rc *RatingCalculator) UpdateRatingUsername(ctx context.Context, rating *Rating) error {
	if err := rc.ratingRepo.UpdateRating(ctx, rating); err != nil {
		rc.logger.Error("failed to update rating", "user_id", rating.UserID, "error", err)
		return err
	}

	rc.logger.Info("rating updated", "user_id", rating.UserID, "username", rating.Username)
	return nil
}
