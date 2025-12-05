package domain

import (
	"testing"
)

// TestEventCreationContext_GroupIDSerialization tests that EventCreationContext correctly preserves group_id
// This validates Property 14: Event-Group Association from the design document
func TestEventCreationContext_GroupIDSerialization(t *testing.T) {
	testCases := []struct {
		name     string
		groupID  int64
		question string
		chatID   int64
	}{
		{"basic case", 1, "Test question", 12345},
		{"large group ID", 999999, "Another question", 67890},
		{"empty question", 42, "", 11111},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create context with group ID
			ctx := &EventCreationContext{
				GroupID:  tc.groupID,
				Question: tc.question,
				ChatID:   tc.chatID,
			}

			// Serialize to map
			data := ctx.ToMap()

			// Deserialize from map
			ctx2 := &EventCreationContext{}
			if err := ctx2.FromMap(data); err != nil {
				t.Fatalf("Failed to deserialize context: %v", err)
			}

			// Verify group ID is preserved
			if ctx2.GroupID != tc.groupID {
				t.Errorf("Expected group_id %d, got %d", tc.groupID, ctx2.GroupID)
			}

			// Verify other fields are also preserved
			if ctx2.Question != tc.question {
				t.Errorf("Question not preserved: expected %q, got %q", tc.question, ctx2.Question)
			}

			if ctx2.ChatID != tc.chatID {
				t.Errorf("ChatID not preserved: expected %d, got %d", tc.chatID, ctx2.ChatID)
			}
		})
	}
}
