package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ad/gitelegram-prediction-market/internal/bot"
	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/logger"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

func main() {
	// Load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logLevel := logger.ParseLevel(cfg.LogLevel)
	log := logger.New(logLevel)
	log.Info("Starting Telegram Prediction Bot", "log_level", cfg.LogLevel)

	// Initialize database
	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		log.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Error("Failed to enable WAL mode", "error", err)
		os.Exit(1)
	}

	log.Info("Database opened", "path", cfg.DatabasePath)

	// Initialize DBQueue for safe concurrent access
	dbQueue := storage.NewDBQueue(db)
	defer dbQueue.Close()

	// Initialize database schema
	if err := storage.InitSchema(dbQueue); err != nil {
		log.Error("Failed to initialize database schema", "error", err)
		os.Exit(1)
	}
	log.Info("Database schema initialized")

	// Run database migrations
	if err := storage.RunMigrations(dbQueue); err != nil {
		log.Error("Failed to run database migrations", "error", err)
		os.Exit(1)
	}
	log.Info("Database migrations completed")

	// Create repositories
	eventRepo := storage.NewEventRepository(dbQueue)
	predictionRepo := storage.NewPredictionRepository(dbQueue)
	ratingRepo := storage.NewRatingRepository(dbQueue)
	achievementRepo := storage.NewAchievementRepository(dbQueue)
	reminderRepo := storage.NewReminderRepository(dbQueue)
	groupRepo := storage.NewGroupRepository(dbQueue)
	groupMembershipRepo := storage.NewGroupMembershipRepository(dbQueue)

	log.Info("Repositories created")

	// Create domain managers
	eventManager := domain.NewEventManager(eventRepo, predictionRepo, log)
	ratingCalculator := domain.NewRatingCalculator(ratingRepo, predictionRepo, eventRepo, log)
	achievementTracker := domain.NewAchievementTracker(achievementRepo, ratingRepo, predictionRepo, eventRepo, log)
	groupContextResolver := domain.NewGroupContextResolver(groupRepo)

	log.Info("Domain managers created")

	// Create FSM storage
	fsmStorage := storage.NewFSMStorage(dbQueue, log)
	log.Info("FSM storage created")

	// Cleanup stale FSM sessions on startup
	cleanupCtx := context.Background()
	if err := fsmStorage.CleanupStale(cleanupCtx); err != nil {
		log.Error("Failed to cleanup stale FSM sessions", "error", err)
		// Don't exit, just log the error
	} else {
		log.Info("Stale FSM sessions cleaned up")
	}

	// Create context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create bot handler first (needed for default handler)
	var handler *bot.BotHandler

	// Initialize Telegram bot
	opts := []tgbot.Option{
		tgbot.WithDefaultHandler(func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			// Handle poll answers
			if update.PollAnswer != nil && handler != nil {
				handler.HandlePollAnswer(ctx, b, update)
				return
			}
			// Default handler for other unhandled updates
		}),
	}

	b, err := tgbot.New(cfg.TelegramToken, opts...)
	if err != nil {
		log.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	log.Info("Telegram bot created")

	// Get bot info for deep-link service
	botInfo, err := b.GetMe(ctx)
	if err != nil {
		log.Error("Failed to get bot info", "error", err)
		os.Exit(1)
	}
	log.Info("Bot info retrieved", "username", botInfo.Username)

	// Create deep-link service
	deepLinkService := domain.NewDeepLinkService(botInfo.Username)
	log.Info("Deep-link service created")

	// Create notification service
	notificationService := domain.NewNotificationService(
		b,
		eventRepo,
		predictionRepo,
		ratingRepo,
		reminderRepo,
		log,
	)

	log.Info("Notification service created")

	// Create event creation FSM
	eventCreationFSM := bot.NewEventCreationFSM(
		fsmStorage,
		b,
		eventManager,
		achievementTracker,
		groupContextResolver,
		groupRepo,
		ratingRepo,
		cfg,
		log,
	)
	log.Info("Event creation FSM created")

	// Create event permission validator
	eventPermissionValidator := domain.NewEventPermissionValidator(
		eventRepo,
		predictionRepo,
		groupMembershipRepo,
		cfg.MinEventsToCreate,
		log,
	)
	log.Info("Event permission validator created")

	// Create bot handler
	handler = bot.NewBotHandler(
		b,
		eventManager,
		ratingCalculator,
		achievementTracker,
		predictionRepo,
		cfg,
		log,
		eventCreationFSM,
		eventPermissionValidator,
		groupRepo,
		groupMembershipRepo,
		deepLinkService,
		groupContextResolver,
		ratingRepo,
	)

	log.Info("Bot handler created")

	// Register command handlers
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypePrefix, handler.HandleStart)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/help", tgbot.MatchTypeExact, handler.HandleHelp)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/rating", tgbot.MatchTypeExact, handler.HandleRating)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/my", tgbot.MatchTypeExact, handler.HandleMy)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/events", tgbot.MatchTypeExact, handler.HandleEvents)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/groups", tgbot.MatchTypeExact, handler.HandleGroups)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/create_event", tgbot.MatchTypeExact, handler.HandleCreateEvent)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/resolve_event", tgbot.MatchTypeExact, handler.HandleResolveEvent)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/edit_event", tgbot.MatchTypeExact, handler.HandleEditEvent)

	// Register admin group management commands
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/create_group", tgbot.MatchTypeExact, handler.HandleCreateGroup)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/list_groups", tgbot.MatchTypeExact, handler.HandleListGroups)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/group_members", tgbot.MatchTypeExact, handler.HandleGroupMembers)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/remove_member", tgbot.MatchTypeExact, handler.HandleRemoveMember)

	// Register callback query handler
	b.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "", tgbot.MatchTypePrefix, handler.HandleCallback)

	// Register message handler for conversation flows
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "", tgbot.MatchTypePrefix, handler.HandleMessage)

	log.Info("Command handlers registered")

	// Start notification scheduler
	if err := notificationService.StartScheduler(ctx); err != nil {
		log.Error("Failed to start notification scheduler", "error", err)
		os.Exit(1)
	}

	log.Info("Notification scheduler started")

	// Start bot polling in a goroutine
	go func() {
		log.Info("Starting bot polling")
		b.Start(ctx)
	}()

	log.Info("Bot is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-ctx.Done()

	log.Info("Shutdown signal received, stopping bot...")

	// Graceful shutdown
	// The context cancellation will stop the bot polling and scheduler
	// DBQueue will be closed by defer

	log.Info("Bot stopped successfully")
}
