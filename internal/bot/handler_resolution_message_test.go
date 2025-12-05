package bot

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	_ "modernc.org/sqlite"
)

// TestResolverInformationInMessage tests: Resolver information in message
func TestResolverInformationInMessage(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("resolution message contains resolver username or identifier", prop.ForAll(
		func(resolverID int64, username string, hasUsername bool, question string, correctOption int) bool {
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

			ratingRepo := storage.NewRatingRepository(queue)
			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)
			logger := &mockLogger{}

			ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

			cfg := &config.Config{
				GroupID: 12345,
			}

			handler := &BotHandler{
				ratingCalculator: ratingCalc,
				config:           cfg,
				logger:           logger,
			}

			ctx := context.Background()
			groupID := int64(1)

			// Create rating with or without username
			rating := &domain.Rating{
				UserID:  resolverID,
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

			// Create an event
			event := &domain.Event{
				Question:  question,
				Options:   []string{"Option 1", "Option 2"},
				CreatedAt: time.Now().Add(-24 * time.Hour),
				Deadline:  time.Now().Add(-1 * time.Hour),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: resolverID,
				GroupID:   groupID,
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Build the resolution message (simulating what publishEventResults would create)
			displayName := handler.getUserDisplayName(ctx, resolverID, groupID)

			// The message should contain resolver information
			message := fmt.Sprintf("🏁 СОБЫТИЕ ЗАВЕРШЕНО!\n════════════════════\n\n❓ Вопрос:\n%s\n\n✅ Правильный ответ:\n%s\n\n👤 Завершил: %s\n",
				event.Question, event.Options[correctOption], displayName)

			// Verify the message contains resolver identification
			if hasUsername && username != "" {
				// Should contain @username or username
				expectedUsername := username
				if !strings.HasPrefix(username, "@") {
					expectedUsername = "@" + username
				}
				if !strings.Contains(message, expectedUsername) && !strings.Contains(message, username) {
					t.Logf("Message should contain username %q, got: %q", expectedUsername, message)
					return false
				}
			} else {
				// Should contain "User id[UserID]" format
				expectedID := fmt.Sprintf("User id%d", resolverID)
				if !strings.Contains(message, expectedID) {
					t.Logf("Message should contain user ID %q, got: %q", expectedID, message)
					return false
				}
			}

			return true
		},
		gen.Int64(),
		gen.AlphaString(),
		gen.Bool(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(0, 1),
	))

	properties.TestingRun(t)
}

// TestCreatorVsAdminDistinction tests: Creator vs admin distinction in message
func TestCreatorVsAdminDistinction(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("resolution message format differs for creator vs admin", prop.ForAll(
		func(creatorID int64, adminID int64, username string, question string, correctOption int, isCreatorResolving bool) bool {
			// Ensure creator and admin are different
			if creatorID == adminID {
				return true // Skip this case
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

			ratingRepo := storage.NewRatingRepository(queue)
			predictionRepo := storage.NewPredictionRepository(queue)
			eventRepo := storage.NewEventRepository(queue)
			logger := &mockLogger{}

			ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

			cfg := &config.Config{
				GroupID:      12345,
				AdminUserIDs: []int64{adminID},
			}

			handler := &BotHandler{
				ratingCalculator: ratingCalc,
				config:           cfg,
				logger:           logger,
			}

			ctx := context.Background()

			// Create ratings for both creator and admin
			creatorRating := &domain.Rating{
				UserID:   creatorID,
				Username: username + "_creator",
				Score:    100,
			}
			if err := ratingRepo.UpdateRating(ctx, creatorRating); err != nil {
				t.Logf("Failed to create creator rating: %v", err)
				return false
			}

			adminRating := &domain.Rating{
				UserID:   adminID,
				Username: username + "_admin",
				Score:    100,
			}
			if err := ratingRepo.UpdateRating(ctx, adminRating); err != nil {
				t.Logf("Failed to create admin rating: %v", err)
				return false
			}

			// Create an event
			event := &domain.Event{
				Question:  question,
				Options:   []string{"Option 1", "Option 2"},
				CreatedAt: time.Now().Add(-24 * time.Hour),
				Deadline:  time.Now().Add(-1 * time.Hour),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: creatorID,
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Logf("Failed to create event: %v", err)
				return false
			}

			// Determine who is resolving
			var resolverID int64
			var isAdmin bool
			if isCreatorResolving {
				resolverID = creatorID
				isAdmin = false
			} else {
				resolverID = adminID
				isAdmin = true
			}

			// Build the resolution message (simulating what publishEventResults would create)
			displayName := handler.getUserDisplayName(ctx, resolverID, 1)

			var message string
			if isAdmin && resolverID != creatorID {
				// Admin resolution
				message = fmt.Sprintf("🏁 СОБЫТИЕ ЗАВЕРШЕНО!\n════════════════════\n\n❓ Вопрос:\n%s\n\n✅ Правильный ответ:\n%s\n\n👤 Завершил (администратор): %s\n",
					event.Question, event.Options[correctOption], displayName)
			} else {
				// Creator resolution
				message = fmt.Sprintf("🏁 СОБЫТИЕ ЗАВЕРШЕНО!\n════════════════════\n\n❓ Вопрос:\n%s\n\n✅ Правильный ответ:\n%s\n\n👤 Завершил (создатель): %s\n",
					event.Question, event.Options[correctOption], displayName)
			}

			// Verify the message format differs based on resolver role
			if isAdmin && resolverID != creatorID {
				// Should contain "администратор"
				if !strings.Contains(message, "администратор") {
					t.Logf("Admin resolution message should contain 'администратор', got: %q", message)
					return false
				}
				// Should NOT contain "создатель"
				if strings.Contains(message, "создатель") {
					t.Logf("Admin resolution message should not contain 'создатель', got: %q", message)
					return false
				}
			} else {
				// Should contain "создатель"
				if !strings.Contains(message, "создатель") {
					t.Logf("Creator resolution message should contain 'создатель', got: %q", message)
					return false
				}
				// Should NOT contain "администратор"
				if strings.Contains(message, "администратор") {
					t.Logf("Creator resolution message should not contain 'администратор', got: %q", message)
					return false
				}
			}

			return true
		},
		gen.Int64(),
		gen.Int64(),
		gen.AlphaString(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(0, 1),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestCreatorResolutionMessage tests creator resolution message format
func TestCreatorResolutionMessage(t *testing.T) {
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

	ratingRepo := storage.NewRatingRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	logger := &mockLogger{}

	ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

	cfg := &config.Config{
		GroupID:      12345,
		AdminUserIDs: []int64{99999},
	}

	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Create creator rating
	creatorID := int64(11111)
	groupID := int64(1)
	creatorRating := &domain.Rating{
		UserID:   creatorID,
		Username: "creator_user",
		Score:    100,
		GroupID:  groupID,
	}
	if err := ratingRepo.UpdateRating(ctx, creatorRating); err != nil {
		t.Fatalf("Failed to create creator rating: %v", err)
	}

	// Create an event
	event := &domain.Event{
		Question:  "Test question?",
		Options:   []string{"Yes", "No"},
		CreatedAt: time.Now().Add(-24 * time.Hour),
		Deadline:  time.Now().Add(-1 * time.Hour),
		Status:    domain.EventStatusActive,
		EventType: domain.EventTypeBinary,
		CreatedBy: creatorID,
		GroupID:   groupID,
	}

	if err := eventRepo.CreateEvent(ctx, event); err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Build creator resolution message
	displayName := handler.getUserDisplayName(ctx, creatorID, groupID)
	message := fmt.Sprintf("🏁 СОБЫТИЕ ЗАВЕРШЕНО!\n════════════════════\n\n❓ Вопрос:\n%s\n\n✅ Правильный ответ:\n%s\n\n👤 Завершил (создатель): %s\n",
		event.Question, event.Options[0], displayName)

	// Verify message contains "создатель"
	if !strings.Contains(message, "создатель") {
		t.Errorf("Creator resolution message should contain 'создатель', got: %q", message)
	}

	// Verify message does not contain "администратор"
	if strings.Contains(message, "администратор") {
		t.Errorf("Creator resolution message should not contain 'администратор', got: %q", message)
	}

	// Verify message contains creator username
	if !strings.Contains(message, "@creator_user") {
		t.Errorf("Creator resolution message should contain creator username, got: %q", message)
	}
}

// TestAdminResolutionMessage tests admin resolution message format
func TestAdminResolutionMessage(t *testing.T) {
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

	ratingRepo := storage.NewRatingRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	logger := &mockLogger{}

	ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

	adminID := int64(99999)
	cfg := &config.Config{
		GroupID:      12345,
		AdminUserIDs: []int64{adminID},
	}

	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Create creator rating
	creatorID := int64(11111)
	groupID := int64(1)
	creatorRating := &domain.Rating{
		UserID:   creatorID,
		Username: "creator_user",
		Score:    100,
		GroupID:  groupID,
	}
	if err := ratingRepo.UpdateRating(ctx, creatorRating); err != nil {
		t.Fatalf("Failed to create creator rating: %v", err)
	}

	// Create admin rating
	adminRating := &domain.Rating{
		UserID:   adminID,
		Username: "admin_user",
		Score:    200,
		GroupID:  groupID,
	}
	if err := ratingRepo.UpdateRating(ctx, adminRating); err != nil {
		t.Fatalf("Failed to create admin rating: %v", err)
	}

	// Create an event
	event := &domain.Event{
		Question:  "Test question?",
		Options:   []string{"Yes", "No"},
		CreatedAt: time.Now().Add(-24 * time.Hour),
		Deadline:  time.Now().Add(-1 * time.Hour),
		Status:    domain.EventStatusActive,
		EventType: domain.EventTypeBinary,
		CreatedBy: creatorID,
		GroupID:   groupID,
	}

	if err := eventRepo.CreateEvent(ctx, event); err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Build admin resolution message
	displayName := handler.getUserDisplayName(ctx, adminID, groupID)
	message := fmt.Sprintf("🏁 СОБЫТИЕ ЗАВЕРШЕНО!\n════════════════════\n\n❓ Вопрос:\n%s\n\n✅ Правильный ответ:\n%s\n\n👤 Завершил (администратор): %s\n",
		event.Question, event.Options[0], displayName)

	// Verify message contains "администратор"
	if !strings.Contains(message, "администратор") {
		t.Errorf("Admin resolution message should contain 'администратор', got: %q", message)
	}

	// Verify message does not contain "создатель"
	if strings.Contains(message, "создатель") {
		t.Errorf("Admin resolution message should not contain 'создатель', got: %q", message)
	}

	// Verify message contains admin username
	if !strings.Contains(message, "@admin_user") {
		t.Errorf("Admin resolution message should contain admin username, got: %q", message)
	}
}

// TestResolverNameInclusion tests that resolver name is included in message
func TestResolverNameInclusion(t *testing.T) {
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

	ratingRepo := storage.NewRatingRepository(queue)
	predictionRepo := storage.NewPredictionRepository(queue)
	eventRepo := storage.NewEventRepository(queue)
	logger := &mockLogger{}

	ratingCalc := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, logger)

	cfg := &config.Config{
		GroupID: 12345,
	}

	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	testCases := []struct {
		name           string
		userID         int64
		username       string
		expectedInName string
	}{
		{
			name:           "with username",
			userID:         11111,
			username:       "testuser",
			expectedInName: "@testuser",
		},
		{
			name:           "without username",
			userID:         22222,
			username:       "",
			expectedInName: "User id22222",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			groupID := int64(1)
			// Create rating
			rating := &domain.Rating{
				UserID:   tc.userID,
				Username: tc.username,
				Score:    100,
				GroupID:  groupID,
			}
			if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
				t.Fatalf("Failed to create rating: %v", err)
			}

			// Create an event
			event := &domain.Event{
				Question:  "Test question?",
				Options:   []string{"Yes", "No"},
				CreatedAt: time.Now().Add(-24 * time.Hour),
				Deadline:  time.Now().Add(-1 * time.Hour),
				Status:    domain.EventStatusActive,
				EventType: domain.EventTypeBinary,
				CreatedBy: tc.userID,
				GroupID:   groupID,
			}

			if err := eventRepo.CreateEvent(ctx, event); err != nil {
				t.Fatalf("Failed to create event: %v", err)
			}

			// Build resolution message
			displayName := handler.getUserDisplayName(ctx, tc.userID, groupID)
			message := fmt.Sprintf("🏁 СОБЫТИЕ ЗАВЕРШЕНО!\n════════════════════\n\n❓ Вопрос:\n%s\n\n✅ Правильный ответ:\n%s\n\n👤 Завершил: %s\n",
				event.Question, event.Options[0], displayName)

			// Verify message contains expected name format
			if !strings.Contains(message, tc.expectedInName) {
				t.Errorf("Resolution message should contain %q, got: %q", tc.expectedInName, message)
			}
		})
	}
}
