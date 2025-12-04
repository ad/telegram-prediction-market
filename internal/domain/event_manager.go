package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEventNotFound     = errors.New("event not found")
	ErrEventHasVotes     = errors.New("event has votes and cannot be edited")
	ErrEventNotActive    = errors.New("event is not active")
	ErrInvalidCorrectOpt = errors.New("invalid correct option")
)

// Logger interface for logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

// EventRepository interface for event operations
type EventRepository interface {
	CreateEvent(ctx context.Context, event *Event) error
	GetEvent(ctx context.Context, eventID int64) (*Event, error)
	GetActiveEvents(ctx context.Context) ([]*Event, error)
	UpdateEvent(ctx context.Context, event *Event) error
	ResolveEvent(ctx context.Context, eventID int64, correctOption int) error
}

// PredictionRepository interface for prediction operations
type PredictionRepository interface {
	GetPredictionsByEvent(ctx context.Context, eventID int64) ([]*Prediction, error)
	GetPredictionByUserAndEvent(ctx context.Context, userID, eventID int64) (*Prediction, error)
}

// EventManager manages event operations and business logic
type EventManager struct {
	eventRepo      EventRepository
	predictionRepo PredictionRepository
	logger         Logger
}

// NewEventManager creates a new EventManager
func NewEventManager(
	eventRepo EventRepository,
	predictionRepo PredictionRepository,
	logger Logger,
) *EventManager {
	return &EventManager{
		eventRepo:      eventRepo,
		predictionRepo: predictionRepo,
		logger:         logger,
	}
}

// CreateEvent creates a new event after validation
func (em *EventManager) CreateEvent(ctx context.Context, event *Event) error {
	// Validate event
	if err := event.Validate(); err != nil {
		em.logger.Error("event validation failed", "error", err)
		return err
	}

	// Set default status if not set
	if event.Status == "" {
		event.Status = EventStatusActive
	}

	// Set created time if not set
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	// Create event in database
	if err := em.eventRepo.CreateEvent(ctx, event); err != nil {
		em.logger.Error("failed to create event", "error", err)
		return err
	}

	em.logger.Info("event created", "event_id", event.ID, "question", event.Question)
	return nil
}

// GetActiveEvents retrieves all active events
func (em *EventManager) GetActiveEvents(ctx context.Context) ([]*Event, error) {
	events, err := em.eventRepo.GetActiveEvents(ctx)
	if err != nil {
		em.logger.Error("failed to get active events", "error", err)
		return nil, err
	}

	em.logger.Debug("retrieved active events", "count", len(events))
	return events, nil
}

// GetEvent retrieves a specific event by ID
func (em *EventManager) GetEvent(ctx context.Context, eventID int64) (*Event, error) {
	event, err := em.eventRepo.GetEvent(ctx, eventID)
	if err != nil {
		em.logger.Error("failed to get event", "event_id", eventID, "error", err)
		return nil, err
	}

	if event == nil {
		return nil, ErrEventNotFound
	}

	return event, nil
}

// UpdateEvent updates an existing event
func (em *EventManager) UpdateEvent(ctx context.Context, event *Event) error {
	// Validate event
	if err := event.Validate(); err != nil {
		em.logger.Error("event validation failed", "error", err)
		return err
	}

	// Update event in database
	if err := em.eventRepo.UpdateEvent(ctx, event); err != nil {
		em.logger.Error("failed to update event", "event_id", event.ID, "error", err)
		return err
	}

	em.logger.Info("event updated", "event_id", event.ID)
	return nil
}

// ResolveEvent resolves an event with the correct option
func (em *EventManager) ResolveEvent(ctx context.Context, eventID int64, correctOption int) error {
	// Get the event first
	event, err := em.GetEvent(ctx, eventID)
	if err != nil {
		return err
	}

	// Check if event is active
	if event.Status != EventStatusActive {
		em.logger.Warn("attempted to resolve non-active event", "event_id", eventID, "status", event.Status)
		return ErrEventNotActive
	}

	// Validate correct option is within range
	if correctOption < 0 || correctOption >= len(event.Options) {
		em.logger.Error("invalid correct option", "event_id", eventID, "option", correctOption, "max", len(event.Options)-1)
		return ErrInvalidCorrectOpt
	}

	// Resolve the event
	if err := em.eventRepo.ResolveEvent(ctx, eventID, correctOption); err != nil {
		em.logger.Error("failed to resolve event", "event_id", eventID, "error", err)
		return err
	}

	em.logger.Info("event resolved", "event_id", eventID, "correct_option", correctOption)
	return nil
}

// CanEditEvent checks if an event can be edited (no votes exist)
func (em *EventManager) CanEditEvent(ctx context.Context, eventID int64) (bool, error) {
	// Get predictions for this event
	predictions, err := em.predictionRepo.GetPredictionsByEvent(ctx, eventID)
	if err != nil {
		em.logger.Error("failed to get predictions for event", "event_id", eventID, "error", err)
		return false, err
	}

	// Event can be edited only if there are no predictions
	canEdit := len(predictions) == 0
	em.logger.Debug("checked if event can be edited", "event_id", eventID, "can_edit", canEdit, "vote_count", len(predictions))

	return canEdit, nil
}
