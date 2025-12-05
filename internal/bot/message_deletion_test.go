package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	"telegram-prediction-bot/internal/logger"

	"github.com/go-telegram/bot"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// mockBot is a mock implementation of the bot for testing
type mockBot struct {
	deleteMessageFunc func(ctx context.Context, params *bot.DeleteMessageParams) (bool, error)
	callCount         int
}

func (m *mockBot) DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
	m.callCount++
	if m.deleteMessageFunc != nil {
		return m.deleteMessageFunc(ctx, params)
	}
	return true, nil
}

func TestDeletionErrorResilience(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("message deletion failures do not interrupt event creation flow", prop.ForAll(
		func(chatID int64, messageIDs []int, errorType string) bool {
			if len(messageIDs) == 0 {
				return true // Skip empty case
			}

			ctx := context.Background()
			log := logger.New(logger.ERROR)

			// Create a mock bot that simulates various error conditions
			var mockError error
			switch errorType {
			case "not_found":
				mockError = errors.New("message to delete not found")
			case "too_old":
				mockError = errors.New("message can't be deleted")
			case "other":
				mockError = errors.New("some other error")
			default:
				mockError = nil // Success case
			}

			mock := &mockBot{
				deleteMessageFunc: func(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
					if mockError != nil {
						return false, mockError
					}
					return true, nil
				},
			}

			// Call deleteMessages - it should never panic or return an error
			// The function should handle all errors gracefully
			deleteMessages(ctx, mock, log, chatID, messageIDs...)

			// Verify that deletion was attempted for all messages
			expectedCalls := len(messageIDs)
			if mock.callCount != expectedCalls {
				t.Logf("Expected %d deletion attempts, got %d", expectedCalls, mock.callCount)
				return false
			}

			// The key property: the function completed without panicking
			// and attempted to delete all messages despite errors
			return true
		},
		gen.Int64Range(1, 1000000),
		gen.SliceOfN(5, gen.IntRange(1, 100000)),
		gen.OneConstOf("not_found", "too_old", "other", "success"),
	))

	properties.TestingRun(t)
}

func TestRateLimitRetryBehavior(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("rate limit errors trigger exactly one retry after 1 second", prop.ForAll(
		func(chatID int64, messageID int) bool {
			ctx := context.Background()
			log := logger.New(logger.ERROR)

			callCount := 0
			mock := &mockBot{
				deleteMessageFunc: func(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
					callCount++
					if callCount == 1 {
						// First call: return rate limit error
						return false, errors.New("Too Many Requests: retry after 1")
					}
					// Second call: succeed
					return true, nil
				},
			}

			// Mock sleep function that doesn't actually sleep
			sleepCalled := false
			var sleepDuration time.Duration
			mockSleep := func(d time.Duration) {
				sleepCalled = true
				sleepDuration = d
			}

			// Call deleteMessagesWithSleep with mock sleep
			deleteMessagesWithSleep(ctx, mock, log, chatID, mockSleep, messageID)

			// Verify exactly 2 calls were made (original + 1 retry)
			if callCount != 2 {
				t.Logf("Expected exactly 2 calls (1 original + 1 retry), got %d", callCount)
				return false
			}

			// Verify sleep was called with 1 second duration
			if !sleepCalled {
				t.Logf("Expected sleep to be called")
				return false
			}

			if sleepDuration != 1*time.Second {
				t.Logf("Expected sleep duration of 1 second, got %v", sleepDuration)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.IntRange(1, 100000),
	))

	properties.TestingRun(t)
}

func TestNonRetryableErrorHandling(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("non-retryable errors do not trigger retry", prop.ForAll(
		func(chatID int64, messageID int, errorType string) bool {
			ctx := context.Background()
			log := logger.New(logger.ERROR)

			// Create errors that should NOT trigger retry
			var mockError error
			switch errorType {
			case "not_found":
				mockError = errors.New("message to delete not found")
			case "too_old":
				mockError = errors.New("message can't be deleted")
			case "permission":
				mockError = errors.New("permission denied")
			case "network":
				mockError = errors.New("network error")
			default:
				mockError = errors.New("generic error")
			}

			callCount := 0
			mock := &mockBot{
				deleteMessageFunc: func(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
					callCount++
					return false, mockError
				},
			}

			// Call deleteMessages with a single message
			deleteMessages(ctx, mock, log, chatID, messageID)

			// Verify exactly 1 call was made (no retry for non-rate-limit errors)
			if callCount != 1 {
				t.Logf("Expected exactly 1 call (no retry), got %d", callCount)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.IntRange(1, 100000),
		gen.OneConstOf("not_found", "too_old", "permission", "network", "generic"),
	))

	properties.TestingRun(t)
}
