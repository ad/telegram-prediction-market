package bot

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/config"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// capturingLogger implements domain.Logger and captures log entries
type capturingLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

type logEntry struct {
	level   string
	message string
	fields  map[string]interface{}
}

func (l *capturingLogger) Debug(msg string, args ...interface{}) {
	l.capture("DEBUG", msg, args...)
}

func (l *capturingLogger) Info(msg string, args ...interface{}) {
	l.capture("INFO", msg, args...)
}

func (l *capturingLogger) Warn(msg string, args ...interface{}) {
	l.capture("WARN", msg, args...)
}

func (l *capturingLogger) Error(msg string, args ...interface{}) {
	l.capture("ERROR", msg, args...)
}

func (l *capturingLogger) capture(level, msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fields := make(map[string]interface{})
	for i := 0; i < len(args)-1; i += 2 {
		key := fmt.Sprintf("%v", args[i])
		value := args[i+1]
		fields[key] = value
	}

	l.entries = append(l.entries, logEntry{
		level:   level,
		message: msg,
		fields:  fields,
	})
}

func (l *capturingLogger) getEntries() []logEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]logEntry{}, l.entries...)
}

func TestAdminActionLogging(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("admin actions are logged with timestamp, user_id, and details", prop.ForAll(
		func(adminUserID int64, eventID int64, action string, details string) bool {
			// Create capturing logger
			logger := &capturingLogger{}

			// Create config
			cfg := &config.Config{
				AdminUserIDs: []int64{adminUserID},
			}

			// Create handler
			handler := &BotHandler{
				config: cfg,
				logger: logger,
			}

			// Log an admin action
			handler.logAdminAction(adminUserID, action, eventID, details)

			// Get log entries
			entries := logger.getEntries()

			// Verify log entry was created
			if len(entries) == 0 {
				t.Logf("No log entry created for admin action")
				return false
			}

			// Find the admin action log entry
			var adminLogEntry *logEntry
			for i := range entries {
				if entries[i].message == "admin action" {
					adminLogEntry = &entries[i]
					break
				}
			}

			if adminLogEntry == nil {
				t.Logf("Admin action log entry not found")
				return false
			}

			// Verify required fields are present
			if _, ok := adminLogEntry.fields["admin_user_id"]; !ok || adminLogEntry.fields["admin_user_id"] != adminUserID {
				t.Logf("admin_user_id mismatch: expected %d, got %v", adminUserID, adminLogEntry.fields["admin_user_id"])
				return false
			}

			if _, ok := adminLogEntry.fields["action"]; !ok || adminLogEntry.fields["action"] != action {
				t.Logf("action mismatch: expected %s, got %v", action, adminLogEntry.fields["action"])
				return false
			}

			if adminLogEntry.fields["event_id"] != eventID {
				t.Logf("event_id mismatch: expected %d, got %v", eventID, adminLogEntry.fields["event_id"])
				return false
			}

			if adminLogEntry.fields["details"] != details {
				t.Logf("details mismatch: expected %s, got %v", details, adminLogEntry.fields["details"])
				return false
			}

			// Verify timestamp is present
			if adminLogEntry.fields["timestamp"] == nil {
				t.Logf("timestamp is missing")
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.Int64Range(1, 1000000),
		gen.OneConstOf("create_event", "resolve_event", "edit_event"),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// Additional test: Verify all admin action types are logged
func TestAdminActionLoggingAllTypes(t *testing.T) {
	adminActions := []string{"create_event", "resolve_event", "edit_event"}

	for _, action := range adminActions {
		t.Run(action, func(t *testing.T) {
			// Create capturing logger
			logger := &capturingLogger{}

			// Create config
			cfg := &config.Config{
				AdminUserIDs: []int64{123},
			}

			// Create handler
			handler := &BotHandler{
				config: cfg,
				logger: logger,
			}

			// Log the action
			handler.logAdminAction(123, action, 456, "test details")

			// Get log entries
			entries := logger.getEntries()

			// Verify log entry was created
			if len(entries) == 0 {
				t.Fatalf("No log entry created for action %s", action)
			}

			// Find the admin action log entry
			var adminLogEntry *logEntry
			for i := range entries {
				if entries[i].message == "admin action" {
					adminLogEntry = &entries[i]
					break
				}
			}

			if adminLogEntry == nil {
				t.Fatalf("Admin action log entry not found for action %s", action)
				return
			}

			// Verify action is logged correctly
			if _, ok := adminLogEntry.fields["action"]; !ok || adminLogEntry.fields["action"] != action {
				t.Errorf("action mismatch: expected %s, got %v", action, adminLogEntry.fields["action"])
			}
		})
	}
}

// Additional test: Verify multiple admin actions are all logged
func TestAdminActionLoggingMultiple(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("multiple admin actions are all logged", prop.ForAll(
		func(actionCount int) bool {
			// Limit action count to reasonable number
			count := 1 + (actionCount % 10)

			// Create capturing logger
			logger := &capturingLogger{}

			// Create config
			cfg := &config.Config{
				AdminUserIDs: []int64{123},
			}

			// Create handler
			handler := &BotHandler{
				config: cfg,
				logger: logger,
			}

			// Log multiple actions
			for i := 0; i < count; i++ {
				handler.logAdminAction(123, "test_action", int64(i), fmt.Sprintf("details %d", i))
			}

			// Get log entries
			entries := logger.getEntries()

			// Count admin action entries
			adminActionCount := 0
			for _, entry := range entries {
				if entry.message == "admin action" {
					adminActionCount++
				}
			}

			// Verify all actions were logged
			if adminActionCount != count {
				t.Logf("Expected %d admin action log entries, got %d", count, adminActionCount)
				return false
			}

			return true
		},
		gen.IntRange(0, 20),
	))

	properties.TestingRun(t)
}
