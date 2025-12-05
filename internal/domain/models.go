package domain

import (
	"errors"
	"time"
)

// Validation errors
var (
	ErrEmptyQuestion             = errors.New("question cannot be empty")
	ErrInsufficientOptions       = errors.New("must have at least 2 options")
	ErrTooManyOptions            = errors.New("cannot have more than 6 options")
	ErrInvalidDeadline           = errors.New("deadline must be after creation time")
	ErrInvalidCreator            = errors.New("creator ID must be set")
	ErrInvalidBinaryOptions      = errors.New("binary event must have exactly 2 options")
	ErrInvalidMultiOptions       = errors.New("multi-option event must have 2-6 options")
	ErrInvalidProbabilityOptions = errors.New("probability event must have exactly 4 options")
	ErrInvalidEventType          = errors.New("invalid event type")
	ErrInvalidEventID            = errors.New("event ID must be set")
	ErrInvalidUserID             = errors.New("user ID must be set")
	ErrInvalidOption             = errors.New("option must be non-negative")
	ErrInvalidCorrectCount       = errors.New("correct count cannot be negative")
	ErrInvalidWrongCount         = errors.New("wrong count cannot be negative")
	ErrInvalidAchievementCode    = errors.New("invalid achievement code")
)

// EventStatus represents the status of an event
type EventStatus string

const (
	EventStatusActive    EventStatus = "active"
	EventStatusResolved  EventStatus = "resolved"
	EventStatusCancelled EventStatus = "cancelled"
)

// EventType represents the type of an event
type EventType string

const (
	EventTypeBinary      EventType = "binary"
	EventTypeMultiOption EventType = "multi_option"
	EventTypeProbability EventType = "probability"
)

// Event represents a prediction event
type Event struct {
	ID            int64
	Question      string
	Options       []string
	CreatedAt     time.Time
	Deadline      time.Time
	Status        EventStatus
	EventType     EventType
	CorrectOption *int
	CreatedBy     int64
	PollID        string // Telegram poll ID for tracking votes
}

// Prediction represents a user's prediction
type Prediction struct {
	ID        int64
	EventID   int64
	UserID    int64
	Option    int
	Timestamp time.Time
}

// Rating represents a user's rating
type Rating struct {
	UserID       int64
	Username     string
	Score        int
	CorrectCount int
	WrongCount   int
	Streak       int
}

// AchievementCode represents an achievement type
type AchievementCode string

const (
	AchievementSharpshooter    AchievementCode = "sharpshooter"
	AchievementWeeklyAnalyst   AchievementCode = "weekly_analyst"
	AchievementProphet         AchievementCode = "prophet"
	AchievementRiskTaker       AchievementCode = "risk_taker"
	AchievementVeteran         AchievementCode = "veteran"
	AchievementEventOrganizer  AchievementCode = "event_organizer"
	AchievementActiveOrganizer AchievementCode = "active_organizer"
	AchievementMasterOrganizer AchievementCode = "master_organizer"
)

// Achievement represents a user achievement
type Achievement struct {
	ID        int64
	UserID    int64
	Code      AchievementCode
	Timestamp time.Time
}

// Validation methods

// Validate validates an Event
func (e *Event) Validate() error {
	if e.Question == "" {
		return ErrEmptyQuestion
	}
	if len(e.Options) < 2 {
		return ErrInsufficientOptions
	}
	if len(e.Options) > 6 {
		return ErrTooManyOptions
	}
	if e.Deadline.Before(e.CreatedAt) {
		return ErrInvalidDeadline
	}
	if e.CreatedBy == 0 {
		return ErrInvalidCreator
	}

	// Validate event type specific constraints
	switch e.EventType {
	case EventTypeBinary:
		if len(e.Options) != 2 {
			return ErrInvalidBinaryOptions
		}
	case EventTypeMultiOption:
		if len(e.Options) < 2 || len(e.Options) > 6 {
			return ErrInvalidMultiOptions
		}
	case EventTypeProbability:
		if len(e.Options) != 4 {
			return ErrInvalidProbabilityOptions
		}
	default:
		return ErrInvalidEventType
	}

	return nil
}

// Validate validates a Prediction
func (p *Prediction) Validate() error {
	if p.EventID == 0 {
		return ErrInvalidEventID
	}
	if p.UserID == 0 {
		return ErrInvalidUserID
	}
	if p.Option < 0 {
		return ErrInvalidOption
	}
	return nil
}

// Validate validates a Rating
func (r *Rating) Validate() error {
	if r.UserID == 0 {
		return ErrInvalidUserID
	}
	if r.CorrectCount < 0 {
		return ErrInvalidCorrectCount
	}
	if r.WrongCount < 0 {
		return ErrInvalidWrongCount
	}
	return nil
}

// Validate validates an Achievement
func (a *Achievement) Validate() error {
	if a.UserID == 0 {
		return ErrInvalidUserID
	}
	if a.Code == "" {
		return ErrInvalidAchievementCode
	}

	// Validate achievement code is one of the known codes
	switch a.Code {
	case AchievementSharpshooter, AchievementWeeklyAnalyst, AchievementProphet, AchievementRiskTaker, AchievementVeteran,
		AchievementEventOrganizer, AchievementActiveOrganizer, AchievementMasterOrganizer:
		return nil
	default:
		return ErrInvalidAchievementCode
	}
}
