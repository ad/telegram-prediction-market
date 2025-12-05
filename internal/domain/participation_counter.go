package domain

import (
	"context"
)

// ParticipationCounter counts user participation in completed events
type ParticipationCounter struct {
	predictionRepo PredictionRepository
	eventRepo      EventRepository
	logger         Logger
}

// NewParticipationCounter creates a new ParticipationCounter
func NewParticipationCounter(
	predictionRepo PredictionRepository,
	eventRepo EventRepository,
	logger Logger,
) *ParticipationCounter {
	return &ParticipationCounter{
		predictionRepo: predictionRepo,
		eventRepo:      eventRepo,
		logger:         logger,
	}
}

// CountCompletedEventParticipation counts how many completed events user participated in
func (c *ParticipationCounter) CountCompletedEventParticipation(ctx context.Context, userID int64) (int, error) {
	count, err := c.predictionRepo.GetUserCompletedEventCount(ctx, userID)
	if err != nil {
		c.logger.Error("failed to count completed event participation", "user_id", userID, "error", err)
		return 0, err
	}

	c.logger.Debug("counted completed event participation", "user_id", userID, "count", count)
	return count, nil
}
