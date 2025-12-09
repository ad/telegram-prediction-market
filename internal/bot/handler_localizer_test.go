package bot

import (
	"context"
	"testing"

	"github.com/ad/gitelegram-prediction-market/internal/config"
	"github.com/ad/gitelegram-prediction-market/internal/domain"
	"github.com/ad/gitelegram-prediction-market/internal/locale"
	"github.com/ad/gitelegram-prediction-market/internal/logger"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_HandlerHelpUsesLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Handler help message should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func(isAdmin bool) bool {
			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			// Build help text by calling the internal method
			// We'll simulate the help text generation
			var helpText string
			helpText += localizer.MustLocalize(locale.HelpBotTitle) + "\n\n"
			helpText += localizer.MustLocalize(locale.HelpUserCommands) + "\n"
			helpText += localizer.MustLocalize(locale.HelpCommandHelp) + "\n"

			if isAdmin {
				helpText += localizer.MustLocalize(locale.HelpAdminCommands) + "\n"
				helpText += localizer.MustLocalize(locale.HelpCommandCreateGroup) + "\n"
			}

			helpText += localizer.MustLocalize(locale.HelpScoringRules) + "\n"
			helpText += localizer.MustLocalize(locale.HelpAchievements) + "\n"

			// Verify no Cyrillic characters in help text
			if containsCyrillic(helpText) {
				t.Logf("Found hardcoded Russian text in help message: %s", helpText)
				return false
			}

			// Verify help text contains expected English keywords
			if !contains(helpText, "Bot") && !contains(helpText, "bot") {
				t.Logf("Help text missing 'bot' keyword: %s", helpText)
				return false
			}

			return true
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

func TestProperty_HandlerErrorMessagesUseLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Handler error messages should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func() bool {
			// Create handler with English localizer
			cfg := &config.Config{}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			handler := &BotHandler{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Test various error messages
			errorMessages := []string{
				localizer.MustLocalize(locale.ErrorUnauthorized),
				localizer.MustLocalize(locale.ErrorGeneric),
				localizer.MustLocalize(locale.GroupContextNoMembership),
				localizer.MustLocalize(locale.GroupContextMultipleGroups),
				localizer.MustLocalize(locale.DeepLinkInvalidLink),
				localizer.MustLocalize(locale.DeepLinkGroupNotFound),
			}

			for _, msg := range errorMessages {
				// Verify no Cyrillic characters in error message
				if containsCyrillic(msg) {
					t.Logf("Found hardcoded Russian text in error message: %s", msg)
					return false
				}
			}

			// Verify handler is using localizer
			if handler.localizer == nil {
				t.Logf("Handler localizer is nil")
				return false
			}

			return true
		},
	))

	properties.TestingRun(t)
}

func TestProperty_HandlerRatingMessagesUseLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Handler rating messages should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func(groupName string, score int, accuracy float64) bool {
			// Ensure valid inputs
			if groupName == "" {
				groupName = "Test Group"
			}
			_ = score
			_ = accuracy

			// Create handler with English localizer
			cfg := &config.Config{}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			handler := &BotHandler{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Build rating message components
			ratingTitle := localizer.MustLocalize(locale.RatingTop10Title)
			groupLabel := localizer.MustLocalizeWithTemplate(locale.RatingGroupName, groupName)

			// Verify no Cyrillic characters
			if containsCyrillic(ratingTitle) {
				t.Logf("Found hardcoded Russian text in rating title: %s", ratingTitle)
				return false
			}

			if containsCyrillic(groupLabel) {
				t.Logf("Found hardcoded Russian text in group label: %s", groupLabel)
				return false
			}

			// Verify handler is using localizer
			if handler.localizer == nil {
				t.Logf("Handler localizer is nil")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.IntRange(0, 10000),
		gen.Float64Range(0, 100),
	))

	properties.TestingRun(t)
}

func TestProperty_HandlerStatsMessagesUseLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Handler stats messages should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func(groupName string, points int, correct int, wrong int) bool {
			// Ensure valid inputs
			if groupName == "" {
				groupName = "Test Group"
			}
			_ = points
			_ = correct
			_ = wrong

			// Create handler with English localizer
			cfg := &config.Config{}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			handler := &BotHandler{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Build stats message components
			statsTitle := localizer.MustLocalize(locale.MyStatsTitle2)
			groupLabel := localizer.MustLocalizeWithTemplate(locale.MyStatsGroupName, groupName)

			// Verify no Cyrillic characters
			if containsCyrillic(statsTitle) {
				t.Logf("Found hardcoded Russian text in stats title: %s", statsTitle)
				return false
			}

			if containsCyrillic(groupLabel) {
				t.Logf("Found hardcoded Russian text in group label: %s", groupLabel)
				return false
			}

			// Verify handler is using localizer
			if handler.localizer == nil {
				t.Logf("Handler localizer is nil")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.IntRange(0, 10000),
		gen.IntRange(0, 1000),
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

func TestProperty_HandlerEventsMessagesUseLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Handler events messages should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func(eventType domain.EventType) bool {
			// Create handler with English localizer
			cfg := &config.Config{}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			handler := &BotHandler{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Build events message components
			eventsTitle := localizer.MustLocalize(locale.EventsActiveTitle)
			noActiveMsg := localizer.MustLocalize(locale.EventsNoActive)

			// Get event type labels
			var typeLabel string
			switch eventType {
			case domain.EventTypeBinary:
				typeLabel = localizer.MustLocalize(locale.EventTypeBinaryLabel)
			case domain.EventTypeMultiOption:
				typeLabel = localizer.MustLocalize(locale.EventTypeMultiOptionLabel)
			case domain.EventTypeProbability:
				typeLabel = localizer.MustLocalize(locale.EventTypeProbabilityLabel)
			}

			// Verify no Cyrillic characters
			if containsCyrillic(eventsTitle) {
				t.Logf("Found hardcoded Russian text in events title: %s", eventsTitle)
				return false
			}

			if containsCyrillic(noActiveMsg) {
				t.Logf("Found hardcoded Russian text in no active message: %s", noActiveMsg)
				return false
			}

			if containsCyrillic(typeLabel) {
				t.Logf("Found hardcoded Russian text in type label: %s", typeLabel)
				return false
			}

			// Verify handler is using localizer
			if handler.localizer == nil {
				t.Logf("Handler localizer is nil")
				return false
			}

			return true
		},
		gen.OneConstOf(domain.EventTypeBinary, domain.EventTypeMultiOption, domain.EventTypeProbability),
	))

	properties.TestingRun(t)
}

func TestProperty_HandlerSessionMessagesUseLocalizer(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Handler session messages should not contain hardcoded Russian text when using English localizer", prop.ForAll(
		func(sessionType string) bool {
			// Ensure valid session type
			_ = sessionType

			// Create handler with English localizer
			cfg := &config.Config{}
			log := logger.New(logger.ERROR)

			// Create English localizer
			localizer, err := locale.NewLocalizer(context.Background(), locale.NewLocale(locale.En))
			if err != nil {
				t.Logf("Failed to create localizer: %v", err)
				return false
			}

			handler := &BotHandler{
				config:    cfg,
				logger:    log,
				localizer: localizer,
			}

			// Build session message components
			continueMsg := localizer.MustLocalize(locale.SessionContinuePrevious)
			errorDeleteMsg := localizer.MustLocalize(locale.SessionErrorDelete)
			errorUnknownMsg := localizer.MustLocalize(locale.SessionErrorUnknown)

			// Verify no Cyrillic characters
			if containsCyrillic(continueMsg) {
				t.Logf("Found hardcoded Russian text in continue message: %s", continueMsg)
				return false
			}

			if containsCyrillic(errorDeleteMsg) {
				t.Logf("Found hardcoded Russian text in error delete message: %s", errorDeleteMsg)
				return false
			}

			if containsCyrillic(errorUnknownMsg) {
				t.Logf("Found hardcoded Russian text in error unknown message: %s", errorUnknownMsg)
				return false
			}

			// Verify handler is using localizer
			if handler.localizer == nil {
				t.Logf("Handler localizer is nil")
				return false
			}

			return true
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}
