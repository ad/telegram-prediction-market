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

// CountCompletedEventParticipation counts how many completed events user participated in for a specific group
func (c *ParticipationCounter) CountCompletedEventParticipation(ctx context.Context, userID int64, groupID int64) (int, error) {
	count, err := c.predictionRepo.GetUserCompletedEventCount(ctx, userID, groupID)
	if err != nil {
		c.logger.Error("failed to count completed event participation", "user_id", userID, "group_id", groupID, "error", err)
		return 0, err
	}

	c.logger.Debug("counted completed event participation", "user_id", userID, "group_id", groupID, "count", count)
	return count, nil
}
