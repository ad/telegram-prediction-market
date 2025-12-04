package domain

import (
	"context"
	"time"
)

const (
	// Achievement thresholds based on requirements
	SharpshooterStreak = 3  // Requirement 5.1
	ProphetStreak      = 10 // Requirement 5.3
	RiskTakerStreak    = 3  // Requirement 5.4
	VeteranCount       = 50 // Requirement 5.5
)

// AchievementRepository interface for achievement operations
type AchievementRepository interface {
	SaveAchievement(ctx context.Context, achievement *Achievement) error
	GetUserAchievements(ctx context.Context, userID int64) ([]*Achievement, error)
	CheckAchievementExists(ctx context.Context, userID int64, code AchievementCode) (bool, error)
}

// AchievementTracker tracks and awards achievements
type AchievementTracker struct {
	achievementRepo AchievementRepository
	ratingRepo      RatingRepository
	predictionRepo  PredictionRepository
	eventRepo       EventRepository
	logger          Logger
}

// NewAchievementTracker creates a new AchievementTracker
func NewAchievementTracker(
	achievementRepo AchievementRepository,
	ratingRepo RatingRepository,
	predictionRepo PredictionRepository,
	eventRepo EventRepository,
	logger Logger,
) *AchievementTracker {
	return &AchievementTracker{
		achievementRepo: achievementRepo,
		ratingRepo:      ratingRepo,
		predictionRepo:  predictionRepo,
		eventRepo:       eventRepo,
		logger:          logger,
	}
}

// CheckAndAwardAchievements checks and awards achievements for a user
func (at *AchievementTracker) CheckAndAwardAchievements(ctx context.Context, userID int64) ([]*Achievement, error) {
	var newAchievements []*Achievement

	// Get user's rating
	rating, err := at.ratingRepo.GetRating(ctx, userID)
	if err != nil {
		at.logger.Error("failed to get rating", "user_id", userID, "error", err)
		return nil, err
	}

	// Check Sharpshooter (3 correct in a row) - Requirement 5.1
	if rating.Streak >= SharpshooterStreak {
		achievement, err := at.awardAchievementIfNew(ctx, userID, AchievementSharpshooter)
		if err != nil {
			at.logger.Error("failed to award sharpshooter", "user_id", userID, "error", err)
		} else if achievement != nil {
			newAchievements = append(newAchievements, achievement)
		}
	}

	// Check Prophet (10 correct in a row) - Requirement 5.3
	if rating.Streak >= ProphetStreak {
		achievement, err := at.awardAchievementIfNew(ctx, userID, AchievementProphet)
		if err != nil {
			at.logger.Error("failed to award prophet", "user_id", userID, "error", err)
		} else if achievement != nil {
			newAchievements = append(newAchievements, achievement)
		}
	}

	// Check Veteran (50 participations) - Requirement 5.5
	totalParticipations := rating.CorrectCount + rating.WrongCount
	if totalParticipations >= VeteranCount {
		achievement, err := at.awardAchievementIfNew(ctx, userID, AchievementVeteran)
		if err != nil {
			at.logger.Error("failed to award veteran", "user_id", userID, "error", err)
		} else if achievement != nil {
			newAchievements = append(newAchievements, achievement)
		}
	}

	// Check Risk Taker (3 minority correct in a row) - Requirement 5.4
	// This requires checking recent predictions
	isRiskTaker, err := at.checkRiskTakerAchievement(ctx, userID)
	if err != nil {
		at.logger.Error("failed to check risk taker", "user_id", userID, "error", err)
	} else if isRiskTaker {
		achievement, err := at.awardAchievementIfNew(ctx, userID, AchievementRiskTaker)
		if err != nil {
			at.logger.Error("failed to award risk taker", "user_id", userID, "error", err)
		} else if achievement != nil {
			newAchievements = append(newAchievements, achievement)
		}
	}

	// Note: Weekly Analyst (Requirement 5.2) would be checked by a separate scheduled job
	// that runs weekly and compares all users' scores for the week

	return newAchievements, nil
}

// awardAchievementIfNew awards an achievement if the user doesn't already have it
func (at *AchievementTracker) awardAchievementIfNew(ctx context.Context, userID int64, code AchievementCode) (*Achievement, error) {
	// Check if achievement already exists
	exists, err := at.achievementRepo.CheckAchievementExists(ctx, userID, code)
	if err != nil {
		return nil, err
	}

	if exists {
		at.logger.Debug("achievement already exists", "user_id", userID, "code", code)
		return nil, nil
	}

	// Create new achievement
	achievement := &Achievement{
		UserID:    userID,
		Code:      code,
		Timestamp: time.Now(),
	}

	if err := at.achievementRepo.SaveAchievement(ctx, achievement); err != nil {
		return nil, err
	}

	at.logger.Info("achievement awarded", "user_id", userID, "code", code)
	return achievement, nil
}

// checkRiskTakerAchievement checks if user has 3 minority correct predictions in a row
func (at *AchievementTracker) checkRiskTakerAchievement(ctx context.Context, userID int64) (bool, error) {
	// This is a simplified implementation
	// In a full implementation, we would need to:
	// 1. Get the user's recent resolved event predictions
	// 2. For each prediction, check if it was:
	//    a) Correct
	//    b) In the minority (<40% of votes)
	// 3. Check if there are 3 consecutive such predictions

	// For now, we'll return false as this requires more complex logic
	// that would need additional repository methods to efficiently query
	// the necessary data (resolved events with vote distributions)

	// TODO: Implement full risk taker check with:
	// - GetRecentResolvedPredictions(userID, limit)
	// - For each prediction, calculate if it was minority
	// - Check for streak of 3

	return false, nil
}

// GetUserAchievements retrieves all achievements for a user
func (at *AchievementTracker) GetUserAchievements(ctx context.Context, userID int64) ([]*Achievement, error) {
	achievements, err := at.achievementRepo.GetUserAchievements(ctx, userID)
	if err != nil {
		at.logger.Error("failed to get user achievements", "user_id", userID, "error", err)
		return nil, err
	}

	return achievements, nil
}

// AwardWeeklyAnalyst awards the Weekly Analyst achievement to the user with most points in a week
// This should be called by a scheduled job at the end of each week
func (at *AchievementTracker) AwardWeeklyAnalyst(ctx context.Context, userID int64) error {
	achievement, err := at.awardAchievementIfNew(ctx, userID, AchievementWeeklyAnalyst)
	if err != nil {
		at.logger.Error("failed to award weekly analyst", "user_id", userID, "error", err)
		return err
	}

	if achievement != nil {
		at.logger.Info("weekly analyst awarded", "user_id", userID)
	}

	return nil
}
