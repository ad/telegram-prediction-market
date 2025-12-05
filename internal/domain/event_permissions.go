package domain

import (
	"context"
	"errors"
)

var (
	ErrUnauthorized              = errors.New("user is not authorized to manage this event")
	ErrInsufficientParticipation = errors.New("insufficient participation to create events")
)

// EventPermissionValidator validates user permissions for event operations
type EventPermissionValidator struct {
	eventRepo         EventRepository
	predictionRepo    PredictionRepository
	minEventsToCreate int
	logger            Logger
}

// NewEventPermissionValidator creates a new EventPermissionValidator
func NewEventPermissionValidator(
	eventRepo EventRepository,
	predictionRepo PredictionRepository,
	minEventsToCreate int,
	logger Logger,
) *EventPermissionValidator {
	return &EventPermissionValidator{
		eventRepo:         eventRepo,
		predictionRepo:    predictionRepo,
		minEventsToCreate: minEventsToCreate,
		logger:            logger,
	}
}

// CanManageEvent checks if user can resolve/cancel event
// Returns true if user is the creator or an administrator
func (v *EventPermissionValidator) CanManageEvent(ctx context.Context, userID int64, eventID int64, adminIDs []int64) (bool, error) {
	// Check if user is admin
	if v.IsAdmin(userID, adminIDs) {
		v.logger.Debug("user is admin, can manage event", "user_id", userID, "event_id", eventID)
		return true, nil
	}

	// Check if user is the creator
	isCreator, err := v.IsEventCreator(ctx, userID, eventID)
	if err != nil {
		v.logger.Error("failed to check if user is event creator", "user_id", userID, "event_id", eventID, "error", err)
		return false, err
	}

	if isCreator {
		v.logger.Debug("user is event creator, can manage event", "user_id", userID, "event_id", eventID)
		return true, nil
	}

	v.logger.Debug("user cannot manage event", "user_id", userID, "event_id", eventID)
	return false, nil
}

// CanCreateEvent checks if user has participated in enough completed events in a specific group
// Returns true if user meets the participation requirement or is an admin
// Also returns the current participation count
func (v *EventPermissionValidator) CanCreateEvent(ctx context.Context, userID int64, groupID int64, adminIDs []int64) (bool, int, error) {
	// Admins are exempt from participation requirement
	if v.IsAdmin(userID, adminIDs) {
		v.logger.Debug("user is admin, can create event", "user_id", userID, "group_id", groupID)
		return true, 0, nil
	}

	// Count user's participation in completed events for this group
	count, err := v.predictionRepo.GetUserCompletedEventCount(ctx, userID, groupID)
	if err != nil {
		v.logger.Error("failed to count user completed event participation", "user_id", userID, "group_id", groupID, "error", err)
		return false, 0, err
	}

	canCreate := count >= v.minEventsToCreate
	v.logger.Debug("checked if user can create event", "user_id", userID, "participation_count", count, "required", v.minEventsToCreate, "can_create", canCreate)

	return canCreate, count, nil
}

// IsEventCreator checks if user is the creator of the event
func (v *EventPermissionValidator) IsEventCreator(ctx context.Context, userID int64, eventID int64) (bool, error) {
	event, err := v.eventRepo.GetEvent(ctx, eventID)
	if err != nil {
		return false, err
	}

	if event == nil {
		return false, ErrEventNotFound
	}

	return event.CreatedBy == userID, nil
}

// IsAdmin checks if user is in admin list
func (v *EventPermissionValidator) IsAdmin(userID int64, adminIDs []int64) bool {
	for _, adminID := range adminIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}
