package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestContextSerializationRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("serialize then deserialize preserves data", prop.ForAll(
		func(question string, eventType EventType, options []string, deadlineOffset int64, lastBotMsgID int, lastUserMsgID int, chatID int64) bool {
			// Create original context
			ctx := &EventCreationContext{
				Question:          question,
				EventType:         eventType,
				Options:           options,
				Deadline:          time.Now().Add(time.Duration(deadlineOffset) * time.Second),
				LastBotMessageID:  lastBotMsgID,
				LastUserMessageID: lastUserMsgID,
				ChatID:            chatID,
			}

			// Serialize to map
			data := ctx.ToMap()

			// Serialize to JSON (simulating database storage)
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				t.Logf("Failed to marshal to JSON: %v", err)
				return false
			}

			// Deserialize from JSON
			var newData map[string]interface{}
			err = json.Unmarshal(jsonBytes, &newData)
			if err != nil {
				t.Logf("Failed to unmarshal from JSON: %v", err)
				return false
			}

			// Deserialize from map
			newCtx := &EventCreationContext{}
			err = newCtx.FromMap(newData)
			if err != nil {
				t.Logf("Failed to deserialize from map: %v", err)
				return false
			}

			// Compare all fields
			if ctx.Question != newCtx.Question {
				t.Logf("Question mismatch: expected %s, got %s", ctx.Question, newCtx.Question)
				return false
			}

			if ctx.EventType != newCtx.EventType {
				t.Logf("EventType mismatch: expected %s, got %s", ctx.EventType, newCtx.EventType)
				return false
			}

			// Compare options
			if len(ctx.Options) != len(newCtx.Options) {
				t.Logf("Options length mismatch: expected %d, got %d", len(ctx.Options), len(newCtx.Options))
				return false
			}
			for i := range ctx.Options {
				if ctx.Options[i] != newCtx.Options[i] {
					t.Logf("Options[%d] mismatch: expected %s, got %s", i, ctx.Options[i], newCtx.Options[i])
					return false
				}
			}

			// Compare deadline (with some tolerance for time precision)
			if ctx.Deadline.Unix() != newCtx.Deadline.Unix() {
				t.Logf("Deadline mismatch: expected %v, got %v", ctx.Deadline, newCtx.Deadline)
				return false
			}

			if ctx.LastBotMessageID != newCtx.LastBotMessageID {
				t.Logf("LastBotMessageID mismatch: expected %d, got %d", ctx.LastBotMessageID, newCtx.LastBotMessageID)
				return false
			}

			if ctx.LastUserMessageID != newCtx.LastUserMessageID {
				t.Logf("LastUserMessageID mismatch: expected %d, got %d", ctx.LastUserMessageID, newCtx.LastUserMessageID)
				return false
			}

			if ctx.ChatID != newCtx.ChatID {
				t.Logf("ChatID mismatch: expected %d, got %d", ctx.ChatID, newCtx.ChatID)
				return false
			}

			return true
		},
		gen.AlphaString(),
		gen.OneConstOf(EventTypeBinary, EventTypeMultiOption, EventTypeProbability),
		gen.SliceOf(gen.AlphaString()),
		gen.Int64Range(0, 86400*30), // 0 to 30 days in seconds
		gen.IntRange(0, 1000000),
		gen.IntRange(0, 1000000),
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}
