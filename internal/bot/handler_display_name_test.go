package bot

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	_ "modernc.org/sqlite"
)

// TestGetUserDisplayName_WithUsername tests getUserDisplayName with username present
func TestGetUserDisplayName_WithUsername(t *testing.T) {
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

	cfg := &config.Config{}
	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Create rating with username
	userID := int64(12345)
	rating := &domain.Rating{
		UserID:   userID,
		Username: "testuser",
		GroupID:  1,
		Score:    100,
	}

	if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
		t.Fatalf("Failed to create rating: %v", err)
	}

	// Test getUserDisplayName
	displayName := handler.getUserDisplayName(ctx, userID, 1)

	// Should return @username format
	expected := "@testuser"
	if displayName != expected {
		t.Errorf("Expected display name %q, got %q", expected, displayName)
	}
}

// TestGetUserDisplayName_WithUsernameWithAtSign tests getUserDisplayName when username already has @ prefix
func TestGetUserDisplayName_WithUsernameWithAtSign(t *testing.T) {
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

	cfg := &config.Config{}
	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Create rating with username that already has @ prefix
	userID := int64(12346)
	rating := &domain.Rating{
		UserID:   userID,
		GroupID:  1,
		Username: "@testuser2",
		Score:    100,
	}

	if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
		t.Fatalf("Failed to create rating: %v", err)
	}

	// Test getUserDisplayName
	displayName := handler.getUserDisplayName(ctx, userID, 1)

	// Should return username as-is (already has @)
	expected := "@testuser2"
	if displayName != expected {
		t.Errorf("Expected display name %q, got %q", expected, displayName)
	}
}

// TestGetUserDisplayName_WithOnlyUserID tests getUserDisplayName with only user ID (no username)
func TestGetUserDisplayName_WithOnlyUserID(t *testing.T) {
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
		Username: "", // No username
		Score:    50,
	}

	if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
		t.Fatalf("Failed to create rating: %v", err)
	}

	// Test getUserDisplayName
	displayName := handler.getUserDisplayName(ctx, userID, 1)

	// Should return "User [UserID]" format
	expected := "User id67890"
	if displayName != expected {
		t.Errorf("Expected display name %q, got %q", expected, displayName)
	}
}

// TestGetUserDisplayName_WithOnlyFirstName tests getUserDisplayName with only first name (no username)
// In the system, first names are stored in the Username field when no Telegram username is available
func TestGetUserDisplayName_WithOnlyFirstName(t *testing.T) {
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

	cfg := &config.Config{}
	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Create rating with first name stored in Username field (no Telegram username)
	userID := int64(54321)
	rating := &domain.Rating{
		UserID:   userID,
		GroupID:  1,
		Username: "John", // First name stored in Username field
		Score:    75,
	}

	if err := ratingRepo.UpdateRating(ctx, rating); err != nil {
		t.Fatalf("Failed to create rating: %v", err)
	}

	// Test getUserDisplayName
	displayName := handler.getUserDisplayName(ctx, userID, 1)

	// Should return @FirstName format (system treats it as username)
	expected := "@John"
	if displayName != expected {
		t.Errorf("Expected display name %q, got %q", expected, displayName)
	}
}

// TestGetUserDisplayName_UserNotFound tests getUserDisplayName when user doesn't exist in database
func TestGetUserDisplayName_UserNotFound(t *testing.T) {
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

	cfg := &config.Config{}
	handler := &BotHandler{
		ratingCalculator: ratingCalc,
		config:           cfg,
		logger:           logger,
	}

	ctx := context.Background()

	// Test getUserDisplayName with non-existent user
	userID := int64(99999)
	displayName := handler.getUserDisplayName(ctx, userID, 1)

	// Should return "User [UserID]" format as fallback
	expected := "User id99999"
	if displayName != expected {
		t.Errorf("Expected display name %q, got %q", expected, displayName)
	}
}
