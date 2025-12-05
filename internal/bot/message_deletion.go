package bot

import (
	"context"
	"errors"
	"time"

	"telegram-prediction-bot/internal/domain"

	"github.com/go-telegram/bot"
)

// MessageDeleter is an interface for deleting messages (for testing)
type MessageDeleter interface {
	DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error)
}

// deleteMessages attempts to delete multiple messages from a chat.
// It handles various error conditions gracefully:
// - "message not found" errors are logged and ignored
// - "message too old" errors are logged and ignored
// - Rate limit errors trigger one retry after 1 second
// - Other errors are logged and ignored
//
// This function never returns an error to avoid interrupting the event creation flow.
func deleteMessages(ctx context.Context, b MessageDeleter, logger domain.Logger, chatID int64, messageIDs ...int) {
	for _, messageID := range messageIDs {
		err := deleteMessageWithRetry(ctx, b, logger, chatID, messageID)
		if err != nil {
			// Log the error but continue with other deletions
			logger.Warn("message deletion failed",
				"chat_id", chatID,
				"message_id", messageID,
				"error", err.Error())
		} else {
			logger.Debug("message deleted successfully",
				"chat_id", chatID,
				"message_id", messageID)
		}
	}
}

// deleteMessageWithRetry attempts to delete a single message with retry logic for rate limits
func deleteMessageWithRetry(ctx context.Context, b MessageDeleter, logger domain.Logger, chatID int64, messageID int) error {
	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})

	if err == nil {
		return nil
	}

	// Check if this is a rate limit error
	if isRateLimitError(err) {
		logger.Info("rate limit hit, retrying after 1 second",
			"chat_id", chatID,
			"message_id", messageID)

		// Wait 1 second and retry once
		time.Sleep(1 * time.Second)

		_, retryErr := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: messageID,
		})

		if retryErr == nil {
			logger.Info("message deleted successfully after retry",
				"chat_id", chatID,
				"message_id", messageID)
			return nil
		}

		// Log the retry failure
		logger.Warn("message deletion failed after retry",
			"chat_id", chatID,
			"message_id", messageID,
			"error", retryErr.Error())
		return retryErr
	}

	// Check for non-retryable errors that should be handled gracefully
	if isMessageNotFoundError(err) {
		logger.Info("message not found (may have been manually deleted)",
			"chat_id", chatID,
			"message_id", messageID)
		return err
	}

	if isMessageTooOldError(err) {
		logger.Info("message too old to delete (Telegram limitation)",
			"chat_id", chatID,
			"message_id", messageID)
		return err
	}

	// For any other error, log and return
	logger.Warn("message deletion failed with unexpected error",
		"chat_id", chatID,
		"message_id", messageID,
		"error", err.Error())
	return err
}

// isRateLimitError checks if the error is a rate limit error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// Telegram rate limit errors typically contain "Too Many Requests" or "retry after"
	errStr := err.Error()
	return contains(errStr, "Too Many Requests") || contains(errStr, "retry after")
}

// isMessageNotFoundError checks if the error is a "message not found" error
func isMessageNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Telegram returns "message to delete not found" or similar
	errStr := err.Error()
	return contains(errStr, "message to delete not found") ||
		contains(errStr, "message not found") ||
		contains(errStr, "MESSAGE_ID_INVALID")
}

// isMessageTooOldError checks if the error is a "message too old" error
func isMessageTooOldError(err error) bool {
	if err == nil {
		return false
	}
	// Telegram returns "message can't be deleted" for old messages
	errStr := err.Error()
	return contains(errStr, "message can't be deleted") ||
		contains(errStr, "message is too old") ||
		contains(errStr, "MESSAGE_DELETE_FORBIDDEN")
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	// Simple case-sensitive check for now
	// Could be enhanced with strings.Contains or case-insensitive matching
	return len(s) >= len(substr) && findSubstring(s, substr)
}

// findSubstring performs a simple substring search
func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ErrMessageDeletionFailed is returned when message deletion fails
var ErrMessageDeletionFailed = errors.New("message deletion failed")
