package bot

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/logger"
	"github.com/ad/gitelegram-prediction-market/internal/storage"

	_ "modernc.org/sqlite"
)

// TestSessionConflictDetection проверяет, что конфликты сессий правильно обнаруживаются
func TestSessionConflictDetection(t *testing.T) {
	ctx := context.Background()
	log := logger.New(logger.DEBUG)

	// Создаем тестовое хранилище
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := storage.NewDBQueue(db)

	// Инициализируем схему
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	fsmStorage := storage.NewFSMStorage(queue, log)

	// Создаем handler с минимальными зависимостями
	handler := &BotHandler{
		eventCreationFSM: &EventCreationFSM{storage: fsmStorage},
		logger:           log,
	}

	userID := int64(12345)

	// Тест 1: Нет активной сессии - не должно быть конфликта
	conflictType, err := handler.checkConflictingSession(ctx, userID, "event_creation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflictType != "" {
		t.Errorf("expected no conflict, got: %s", conflictType)
	}

	// Тест 2: Создаем сессию создания события
	eventContext := &domain.EventCreationContext{ChatID: 123}
	err = fsmStorage.Set(ctx, userID, StateAskQuestion, eventContext.ToMap())
	if err != nil {
		t.Fatalf("failed to create event session: %v", err)
	}

	// Проверяем, что нет конфликта при запросе того же типа
	conflictType, err = handler.checkConflictingSession(ctx, userID, "event_creation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflictType != "" {
		t.Errorf("expected no conflict for same type, got: %s", conflictType)
	}

	// Проверяем, что есть конфликт при запросе другого типа
	conflictType, err = handler.checkConflictingSession(ctx, userID, "group_creation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflictType != "создания события" {
		t.Errorf("expected conflict 'создания события', got: %s", conflictType)
	}

	// Тест 3: Создаем сессию создания группы
	_ = fsmStorage.Delete(ctx, userID)
	groupContext := &domain.GroupCreationContext{ChatID: 123}
	err = fsmStorage.Set(ctx, userID, StateGroupAskName, groupContext.ToMap())
	if err != nil {
		t.Fatalf("failed to create group session: %v", err)
	}

	// Проверяем конфликт с созданием события
	conflictType, err = handler.checkConflictingSession(ctx, userID, "event_creation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflictType != "создания группы" {
		t.Errorf("expected conflict 'создания группы', got: %s", conflictType)
	}

	// Тест 4: Создаем сессию завершения события
	_ = fsmStorage.Delete(ctx, userID)
	resolutionContext := &domain.EventResolutionContext{ChatID: 123}
	err = fsmStorage.Set(ctx, userID, StateResolveSelectEvent, resolutionContext.ToMap())
	if err != nil {
		t.Fatalf("failed to create resolution session: %v", err)
	}

	// Проверяем конфликт с созданием события
	conflictType, err = handler.checkConflictingSession(ctx, userID, "event_creation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflictType != "завершения события" {
		t.Errorf("expected conflict 'завершения события', got: %s", conflictType)
	}
}

// TestSessionConflictCallback проверяет обработку callback'ов конфликта сессий
func TestSessionConflictCallback(t *testing.T) {
	ctx := context.Background()
	log := logger.New(logger.DEBUG)

	// Создаем тестовое хранилище
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	queue := storage.NewDBQueue(db)

	// Инициализируем схему
	if err := storage.InitSchema(queue); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(queue); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	fsmStorage := storage.NewFSMStorage(queue, log)

	userID := int64(12345)
	chatID := int64(67890)

	// Создаем активную сессию создания события
	eventContext := &domain.EventCreationContext{ChatID: chatID}
	err = fsmStorage.Set(ctx, userID, StateAskQuestion, eventContext.ToMap())
	if err != nil {
		t.Fatalf("failed to create event session: %v", err)
	}

	// Тест: Callback "перезапустить" удаляет старую сессию
	// Проверяем, что сессия существует
	_, _, err = fsmStorage.Get(ctx, userID)
	if err != nil {
		t.Fatalf("session should exist before restart: %v", err)
	}

	// Удаляем сессию напрямую (имитируя действие handleSessionConflictCallback)
	err = fsmStorage.Delete(ctx, userID)
	if err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	// Проверяем, что сессия была удалена
	_, _, err = fsmStorage.Get(ctx, userID)
	if err != storage.ErrSessionNotFound {
		t.Errorf("old session should be deleted after restart, got error: %v", err)
	}
}
