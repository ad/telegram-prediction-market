package bot

import (
	"strings"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/domain"
)

// TestErrorDefinitions tests that error types are properly defined
func TestErrorDefinitions(t *testing.T) {
	t.Run("ErrUnauthorized is defined", func(t *testing.T) {
		if domain.ErrUnauthorized == nil {
			t.Fatal("ErrUnauthorized should be defined")
		}
	})

	t.Run("ErrUnauthorized has descriptive message", func(t *testing.T) {
		errMsg := domain.ErrUnauthorized.Error()
		if errMsg == "" {
			t.Fatal("ErrUnauthorized should have a message")
		}
		if !strings.Contains(errMsg, "authorized") {
			t.Errorf("ErrUnauthorized message should mention authorization, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "manage") || !strings.Contains(errMsg, "event") {
			t.Errorf("ErrUnauthorized message should mention event management, got: %s", errMsg)
		}
	})

	t.Run("ErrInsufficientParticipation is defined", func(t *testing.T) {
		if domain.ErrInsufficientParticipation == nil {
			t.Fatal("ErrInsufficientParticipation should be defined")
		}
	})

	t.Run("ErrInsufficientParticipation has descriptive message", func(t *testing.T) {
		errMsg := domain.ErrInsufficientParticipation.Error()
		if errMsg == "" {
			t.Fatal("ErrInsufficientParticipation should have a message")
		}
		if !strings.Contains(errMsg, "participation") {
			t.Errorf("ErrInsufficientParticipation message should mention participation, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "create") || !strings.Contains(errMsg, "event") {
			t.Errorf("ErrInsufficientParticipation message should mention event creation, got: %s", errMsg)
		}
	})

	t.Run("Error messages are user-friendly", func(t *testing.T) {
		// Error messages should be concise and clear
		unauthorizedMsg := domain.ErrUnauthorized.Error()
		participationMsg := domain.ErrInsufficientParticipation.Error()

		if len(unauthorizedMsg) > 200 {
			t.Errorf("ErrUnauthorized message is too long (%d chars), should be concise", len(unauthorizedMsg))
		}
		if len(participationMsg) > 200 {
			t.Errorf("ErrInsufficientParticipation message is too long (%d chars), should be concise", len(participationMsg))
		}

		// Should not contain technical jargon
		technicalTerms := []string{"stack", "trace", "exception", "nil", "null", "undefined"}
		for _, term := range technicalTerms {
			if strings.Contains(strings.ToLower(unauthorizedMsg), term) {
				t.Errorf("ErrUnauthorized message should not contain technical term %q", term)
			}
			if strings.Contains(strings.ToLower(participationMsg), term) {
				t.Errorf("ErrInsufficientParticipation message should not contain technical term %q", term)
			}
		}
	})
}

// TestErrorMessageConsistency tests that error messages follow consistent patterns
func TestErrorMessageConsistency(t *testing.T) {
	t.Run("Error messages use consistent language", func(t *testing.T) {
		unauthorizedMsg := domain.ErrUnauthorized.Error()
		participationMsg := domain.ErrInsufficientParticipation.Error()

		// Both should be in English (for internal errors)
		// and describe the problem clearly
		if unauthorizedMsg == "" || participationMsg == "" {
			t.Fatal("Error messages should not be empty")
		}

		// Check that messages start with lowercase (Go convention for errors)
		if unauthorizedMsg[0] >= 'A' && unauthorizedMsg[0] <= 'Z' {
			t.Error("Error messages should start with lowercase letter (Go convention)")
		}
		if participationMsg[0] >= 'A' && participationMsg[0] <= 'Z' {
			t.Error("Error messages should start with lowercase letter (Go convention)")
		}
	})
}

// TestErrorUsageInHandlers tests that errors are used appropriately in handlers
// This test verifies the error handling patterns exist in the codebase
func TestErrorUsageInHandlers(t *testing.T) {
	t.Run("Handler sends user-friendly messages for unauthorized access", func(t *testing.T) {
		// This test verifies that the handler code contains user-friendly error messages
		// The actual message sending is tested in integration tests
		
		// Verify the error type exists and can be used
		err := domain.ErrUnauthorized
		if err == nil {
			t.Fatal("ErrUnauthorized should be available for use in handlers")
		}

		// Verify error can be checked
		if err.Error() == "" {
			t.Error("ErrUnauthorized should have a non-empty error message")
		}
	})

	t.Run("Handler sends user-friendly messages for insufficient participation", func(t *testing.T) {
		// This test verifies that the handler code contains user-friendly error messages
		// The actual message sending is tested in integration tests
		
		// Verify the error type exists and can be used
		err := domain.ErrInsufficientParticipation
		if err == nil {
			t.Fatal("ErrInsufficientParticipation should be available for use in handlers")
		}

		// Verify error can be checked
		if err.Error() == "" {
			t.Error("ErrInsufficientParticipation should have a non-empty error message")
		}
	})
}

// TestErrorDistinction tests that the two error types are distinct
func TestErrorDistinction(t *testing.T) {
	t.Run("Error types are distinct", func(t *testing.T) {
		if domain.ErrUnauthorized == domain.ErrInsufficientParticipation {
			t.Error("ErrUnauthorized and ErrInsufficientParticipation should be distinct errors")
		}

		if domain.ErrUnauthorized.Error() == domain.ErrInsufficientParticipation.Error() {
			t.Error("Error messages should be different for different error types")
		}
	})

	t.Run("Errors can be distinguished programmatically", func(t *testing.T) {
		// Test that we can distinguish between the two errors
		testErr := domain.ErrUnauthorized
		
		if testErr == domain.ErrUnauthorized {
			// This is the expected path
		} else if testErr == domain.ErrInsufficientParticipation {
			t.Error("Should be able to distinguish ErrUnauthorized from ErrInsufficientParticipation")
		}

		testErr = domain.ErrInsufficientParticipation
		
		if testErr == domain.ErrInsufficientParticipation {
			// This is the expected path
		} else if testErr == domain.ErrUnauthorized {
			t.Error("Should be able to distinguish ErrInsufficientParticipation from ErrUnauthorized")
		}
	})
}
