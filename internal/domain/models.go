package domain

import "time"

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
	Score        int
	CorrectCount int
	WrongCount   int
	Streak       int
}

// AchievementCode represents an achievement type
type AchievementCode string

const (
	AchievementSharpshooter  AchievementCode = "sharpshooter"
	AchievementWeeklyAnalyst AchievementCode = "weekly_analyst"
	AchievementProphet       AchievementCode = "prophet"
	AchievementRiskTaker     AchievementCode = "risk_taker"
	AchievementVeteran       AchievementCode = "veteran"
)

// Achievement represents a user achievement
type Achievement struct {
	ID        int64
	UserID    int64
	Code      AchievementCode
	Timestamp time.Time
}
