package storage

import (
	"context"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
)

// EventRepositoryInterface defines the interface for event operations
type EventRepositoryInterface interface {
	CreateEvent(ctx context.Context, event *domain.Event) error
	GetEvent(ctx context.Context, eventID int64) (*domain.Event, error)
	GetActiveEvents(ctx context.Context) ([]*domain.Event, error)
	UpdateEvent(ctx context.Context, event *domain.Event) error
	ResolveEvent(ctx context.Context, eventID int64, correctOption int) error
	GetEventsByDeadlineRange(ctx context.Context, start, end time.Time) ([]*domain.Event, error)
}

// PredictionRepositoryInterface defines the interface for prediction operations
type PredictionRepositoryInterface interface {
	SavePrediction(ctx context.Context, prediction *domain.Prediction) error
	UpdatePrediction(ctx context.Context, prediction *domain.Prediction) error
	GetPredictionsByEvent(ctx context.Context, eventID int64) ([]*domain.Prediction, error)
	GetPredictionByUserAndEvent(ctx context.Context, userID, eventID int64) (*domain.Prediction, error)
}

// RatingRepositoryInterface defines the interface for rating operations
type RatingRepositoryInterface interface {
	GetRating(ctx context.Context, userID int64) (*domain.Rating, error)
	UpdateRating(ctx context.Context, rating *domain.Rating) error
	GetTopRatings(ctx context.Context, limit int) ([]*domain.Rating, error)
	UpdateStreak(ctx context.Context, userID int64, streak int) error
}

// AchievementRepositoryInterface defines the interface for achievement operations
type AchievementRepositoryInterface interface {
	SaveAchievement(ctx context.Context, achievement *domain.Achievement) error
	GetUserAchievements(ctx context.Context, userID int64) ([]*domain.Achievement, error)
	CheckAchievementExists(ctx context.Context, userID int64, code domain.AchievementCode) (bool, error)
}

// GroupRepositoryInterface defines the interface for group operations
type GroupRepositoryInterface interface {
	CreateGroup(ctx context.Context, group *domain.Group) error
	GetGroup(ctx context.Context, groupID int64) (*domain.Group, error)
	GetAllGroups(ctx context.Context) ([]*domain.Group, error)
	GetUserGroups(ctx context.Context, userID int64) ([]*domain.Group, error)
	DeleteGroup(ctx context.Context, groupID int64) error
}
