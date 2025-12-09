package bot

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/locale"
	"github.com/ad/gitelegram-prediction-market/internal/logger"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// containsCyrillic checks if a string contains Cyrillic characters
func containsCyrillic(s string) bool {
	for _, r := range s {
		if (r >= 0x0400 && r <= 0x04FF) || (r >= 0x0500 && r <= 0x052F) {
			return true
		}
	}
	return false
}

func TestProperty_FSMSummaryUsesLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("FSM summary messages should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func(question string, eventType domain.EventType, options []string) bool {
			// Ensure valid inputs
			if question == "" {
				question = "Test question?"
			}
			if len(options) < 2 {
				options = []string{"Option 1", "Option 2"}
			}

			// Create FSM with English localizer
			cfg := &config.Config{
				Timezone: time.UTC,
			}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			fsm := &EventCreationFSM{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Create deadline (always in future)
			deadline := time.Now().Add(24 * time.Hour)

			// Create context
			ctx := &domain.EventCreationContext{
				Question:  question,
				EventType: eventType,
				Options:   options,
				Deadline:  deadline,
			}

			// Build summary
			summary := fsm.buildEventSummary(ctx)

			// Verify no Cyrillic characters in summary
			if containsCyrillic(summary) {
				t.Logf("Found hardcoded Russian text in summary: %s", summary)
				return false
			}

			// Verify summary contains the question
			if !contains(summary, question) {
				t.Logf("Summary missing question: %s", question)
				return false
			}

			// Verify summary contains options
			for _, opt := range options {
				if !contains(summary, opt) {
					t.Logf("Summary missing option: %s", opt)
					return false
				}
			}

			return true
		},
		gen.Identifier(),
		gen.OneConstOf(domain.EventTypeBinary, domain.EventTypeMultiOption, domain.EventTypeProbability),
		gen.SliceOfN(2, gen.Identifier()),
	))

	properties.TestingRun(t)
}

func TestProperty_FSMFinalSummaryUsesLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("FSM final summary messages should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func(eventID int64, question string, eventType domain.EventType, options []string) bool {
			// Ensure valid inputs
			if question == "" {
				question = "Test question?"
			}
			if len(options) < 2 {
				options = []string{"Option 1", "Option 2"}
			}
			if eventID < 1 {
				eventID = 1
			}

			// Create FSM with English localizer
			cfg := &config.Config{
				Timezone: time.UTC,
			}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			fsm := &EventCreationFSM{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Create deadline (always in future)
			deadline := time.Now().Add(24 * time.Hour)

			// Create event
			event := &domain.Event{
				ID:        eventID,
				Question:  question,
				EventType: eventType,
				Options:   options,
				Deadline:  deadline,
			}

			// Build final summary
			pollReference := "Poll published in group"
			summary := fsm.buildFinalEventSummary(event, pollReference)

			// Verify no Cyrillic characters in summary
			if containsCyrillic(summary) {
				t.Logf("Found hardcoded Russian text in final summary: %s", summary)
				return false
			}

			// Verify summary contains the question
			if !contains(summary, question) {
				t.Logf("Final summary missing question: %s", question)
				return false
			}

			// Verify summary contains event ID
			eventIDStr := fmt.Sprintf("%d", eventID)
			if !contains(summary, eventIDStr) {
				t.Logf("Final summary missing event ID: %d", eventID)
				return false
			}

			// Verify summary contains options
			for _, opt := range options {
				if !contains(summary, opt) {
					t.Logf("Final summary missing option: %s", opt)
					return false
				}
			}

			return true
		},
		gen.Int64Range(1, 1000000),
		gen.Identifier(),
		gen.OneConstOf(domain.EventTypeBinary, domain.EventTypeMultiOption, domain.EventTypeProbability),
		gen.SliceOfN(2, gen.Identifier()),
	))

	properties.TestingRun(t)
}

func TestProperty_FSMDeadlinePromptUsesLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("FSM deadline prompt should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func() bool {
			// Create FSM with English localizer
			cfg := &config.Config{
				Timezone: time.UTC,
			}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			fsm := &EventCreationFSM{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Get deadline prompt message
			prompt := fsm.getDeadlinePromptMessage()

			// Verify no Cyrillic characters in prompt
			if containsCyrillic(prompt) {
				t.Logf("Found hardcoded Russian text in deadline prompt: %s", prompt)
				return false
			}

			// Verify prompt contains expected English text
			if !contains(prompt, "deadline") && !contains(prompt, "Deadline") {
				t.Logf("Deadline prompt missing 'deadline' keyword: %s", prompt)
				return false
			}

			return true
		},
	))

	properties.TestingRun(t)
}

func TestProperty_FSMEventTypeLabelsUseLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("FSM event type labels should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func(eventType domain.EventType) bool {
			// Create FSM with English localizer
			cfg := &config.Config{
				Timezone: time.UTC,
			}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			fsm := &EventCreationFSM{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Create context with event type
			ctx := &domain.EventCreationContext{
				Question:  "Test question?",
				EventType: eventType,
				Options:   []string{"Option 1", "Option 2"},
				Deadline:  time.Now().Add(24 * time.Hour),
			}

			// Build summary which includes event type label
			summary := fsm.buildEventSummary(ctx)

			// Verify no Cyrillic characters in summary
			if containsCyrillic(summary) {
				t.Logf("Found hardcoded Russian text in event type label: %s", summary)
				return false
			}

			return true
		},
		gen.OneConstOf(domain.EventTypeBinary, domain.EventTypeMultiOption, domain.EventTypeProbability),
	))

	properties.TestingRun(t)
}
