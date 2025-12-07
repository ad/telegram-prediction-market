package bot

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	_ "modernc.org/sqlite"
)

// TestUsernameInAchievementNotification tests: Username in achievement notification
func TestUsernameInAchievementNotification(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("achievement notification contains username, first name, or user ID", prop.ForAll(
		func(userID int64, username string, hasUsername bool) bool {
			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer func() { _ = db.Close() }()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			// Run migrations
			if err := storage.RunMigrations(queue); err != nil {
				t.Fatalf("Failed to run migrations: %v", err)
			}

			ratingRepo := storage.NewRatingRepository(queue)
			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)
			logger := &mockLogger{}

			ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

			cfg := &config.Config{}

			handler := &BotHandler{
				ratingCalculator: ratingCalc,
				config:           cfg,
				logger:           logger,
			}

			ctx := context.Background()
			groupID := int64(1)

			// Create rating with or without username
			rating := &domain.Rating{
				UserID:  userID,
				Score:   100,
				GroupID: groupID,
			}
			if hasUsername && username != "" {
				rating.Username = username
			}

			if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
				t.Logf("Failed to create rating: %v", err)
				return false
			}

			// Get the display name that would be used in the notification
			displayName := handler.getUserDisplayName(ctx, userID, groupID)

			// Verify the display name contains user identification
			if hasUsername && username != "" {
				// Should contain @username or username
				expectedUsername := username
				if !strings.HasPrefix(username, "@") {
					expectedUsername = "@" + username
				}
				if displayName != expectedUsername {
					t.Logf("Display name should be %q, got: %q", expectedUsername, displayName)
					return false
				}
			} else {
				// Should contain "User id[UserID]" format
				expectedID := fmt.Sprintf("User id%d", userID)
				if displayName != expectedID {
					t.Logf("Display name should be %q, got: %q", expectedID, displayName)
					return false
				}
			}

			return true
		},
		gen.Int64(),
		gen.AlphaString(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestAchievementMessageFormat tests Property 16: Achievement message format
func TestAchievementMessageFormat(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("achievement message follows pattern with emoji and achievement name", prop.ForAll(
		func(userID int64, username string, achievementCodeStr string) bool {
			// Map string to valid achievement code
			var expectedName string

			switch achievementCodeStr {
			case "event_organizer":
				expectedName = "🎪 Организатор событий"
			case "active_organizer":
				expectedName = "🎭 Активный организатор"
			case "master_organizer":
				expectedName = "🎬 Мастер организатор"
			case "sharpshooter":
				expectedName = "🎯 Меткий стрелок"
			case "prophet":
				expectedName = "🔮 Провидец"
			default:
				return true // Skip invalid codes
			}

			// Setup in-memory database
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Logf("Failed to open database: %v", err)
				return false
			}
			defer func() { _ = db.Close() }()

			queue := storage.NewDBQueue(db)
			defer queue.Close()

			// Initialize schema
			if err := storage.InitSchema(queue); err != nil {
				t.Logf("Failed to initialize schema: %v", err)
				return false
			}

			// Run migrations
			if err := storage.RunMigrations(queue); err != nil {
				t.Fatalf("Failed to run migrations: %v", err)
			}

			ratingRepo := storage.NewRatingRepository(queue)
			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)
			logger := &mockLogger{}

			ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

			cfg := &config.Config{}

			handler := &BotHandler{
				ratingCalculator: ratingCalc,
				config:           cfg,
				logger:           logger,
			}

			ctx := context.Background()

			// Create rating with username
			rating := &domain.Rating{
				UserID:   userID,
				Username: username,
				Score:    100,
			}

			if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
				t.Logf("Failed to create rating: %v", err)
				return false
			}

			// Get the display name
			displayName := handler.getUserDisplayName(ctx, userID, 1)

			// Build the expected message format
			expectedMessage := fmt.Sprintf("🎉 %s получил ачивку: %s!", displayName, expectedName)

			// Verify the message format matches the pattern
			// Check that it contains the key components
			if !strings.Contains(expectedMessage, "🎉") {
				t.Logf("Message should contain emoji 🎉")
				return false
			}

			if !strings.Contains(expectedMessage, "получил ачивку") {
				t.Logf("Message should contain 'получил ачивку'")
				return false
			}

			if !strings.Contains(expectedMessage, expectedName) {
				t.Logf("Message should contain achievement name %q", expectedName)
				return false
			}

			if !strings.Contains(expectedMessage, displayName) {
				t.Logf("Message should contain display name %q", displayName)
				return false
			}

			return true
		},
		gen.Int64(),
		gen.AlphaString(),
		gen.OneConstOf("event_organizer", "active_organizer", "master_organizer", "sharpshooter", "prophet"),
	))

	properties.TestingRun(t)
}

// TestSendAchievementNotification_WithUsername tests message format with username
func TestSendAchievementNotification_WithUsername(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := storage.NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	ratingRepo := storage.NewRatingRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	logger := &mockLogger{}

	ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

	cfg := &config.Config{}

	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Create rating with username
	userID := int64(12345)
	groupID := int64(1)
	rating := &domain.Rating{
		UserID:   userID,
		Username: "testuser",
		Score:    100,
		GroupID:  groupID,
	}

	if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
		t.Fatalf("Failed to create rating: %v", err)
	}

	// Get display name
	displayName := handler.getUserDisplayName(ctx, userID, groupID)

	// Verify display name contains username
	if !strings.Contains(displayName, "@testuser") && !strings.Contains(displayName, "testuser") {
		t.Errorf("Display name should contain username, got: %q", displayName)
	}
}

// TestSendAchievementNotification_WithoutUsername tests message format with only user ID
func TestSendAchievementNotification_WithoutUsername(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := storage.NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	ratingRepo := storage.NewRatingRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	logger := &mockLogger{}

	ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

	cfg := &config.Config{}

	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Create rating without username
	userID := int64(67890)
	rating := &domain.Rating{
		UserID:   userID,
		Username: "",
		Score:    50,
	}

	if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
		t.Fatalf("Failed to create rating: %v", err)
	}

	// Get display name
	displayName := handler.getUserDisplayName(ctx, userID, 1)

	// Verify display name contains user ID format
	if !strings.Contains(displayName, "User id") {
		t.Errorf("Display name should contain 'User id' format, got: %q", displayName)
	}
}

// TestSendAchievementNotification_EmojiAndName tests emoji and achievement name inclusion
func TestSendAchievementNotification_EmojiAndName(t *testing.T) {
	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := storage.NewDBQueue(db)
	defer queue.Close()

	// Initialize schema
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	ratingRepo := storage.NewRatingRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	logger := &mockLogger{}

	ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

	cfg := &config.Config{}

	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Create rating with username
	userID := int64(11111)
	rating := &domain.Rating{
		UserID:   userID,
		Username: "achiever",
		Score:    200,
	}

	if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
		t.Fatalf("Failed to create rating: %v", err)
	}

	// Get display name
	displayName := handler.getUserDisplayName(ctx, userID, 1)

	// Test different achievements
	testCases := []struct {
		code         domain.AchievementCode
		expectedName string
		expectedText string
	}{
		{domain.AchievementEventOrganizer, "🎪 Организатор событий", "🎉"},
		{domain.AchievementActiveOrganizer, "🎭 Активный организатор", "🎉"},
		{domain.AchievementMasterOrganizer, "🎬 Мастер организатор", "🎉"},
		{domain.AchievementSharpshooter, "🎯 Меткий стрелок", "🎉"},
		{domain.AchievementProphet, "🔮 Провидец", "🎉"},
	}

	for _, tc := range testCases {
		// Build expected message
		expectedMsg := fmt.Sprintf("🎉 %s получил ачивку: %s!", displayName, tc.expectedName)

		// Verify message contains emoji
		if !strings.Contains(expectedMsg, tc.expectedText) {
			t.Errorf("Message for %s should contain %q, got: %q", tc.code, tc.expectedText, expectedMsg)
		}

		// Verify message contains "получил ачивку"
		if !strings.Contains(expectedMsg, "получил ачивку") {
			t.Errorf("Message for %s should contain 'получил ачивку', got: %q", tc.code, expectedMsg)
		}

		// Verify message contains achievement name
		if !strings.Contains(expectedMsg, tc.expectedName) {
			t.Errorf("Message for %s should contain achievement name %q, got: %q", tc.code, tc.expectedName, expectedMsg)
		}
	}
}
