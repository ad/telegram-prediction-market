package locale

import (
	"context"
	"testing"
)

// TestLocalizerInitialization tests that Localizer can be initialized properly
func TestLocalizerInitialization(t *testing.T) {
	tests := []struct {
		name        string
		locale      string
		expectError bool
	}{
		{
			name:        "Initialize with Russian locale",
			locale:      Ru,
			expectError: false,
		},
		{
			name:        "Initialize with English locale",
			locale:      En,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			loc := NewLocale(tt.locale)
			localizer, err := NewLocalizer(ctx, loc)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if localizer == nil {
				t.Error("Expected non-nil localizer")
				return
			}

			// Verify localizer has correct locale
			if localizer.GetLocale() != tt.locale {
				t.Errorf("Expected locale %s, got %s", tt.locale, localizer.GetLocale())
			}
		})
	}
}

// TestLocalizerSingleParameter tests message formatting with one parameter
func TestLocalizerSingleParameter(t *testing.T) {
	tests := []struct {
		name           string
		locale         string
		messageKey     string
		param          string
		expectContains string
	}{
		{
			name:           "Format notification question in Russian",
			locale:         Ru,
			messageKey:     NotificationNewEventQuestion,
			param:          "Will it rain tomorrow?",
			expectContains: "Will it rain tomorrow?",
		},
		{
			name:           "Format notification question in English",
			locale:         En,
			messageKey:     NotificationNewEventQuestion,
			param:          "Will it rain tomorrow?",
			expectContains: "Will it rain tomorrow?",
		},
		{
			name:           "Format deadline hours in Russian",
			locale:         Ru,
			messageKey:     DeadlineHoursOnly,
			param:          "24",
			expectContains: "24",
		},
		{
			name:           "Format deadline hours in English",
			locale:         En,
			messageKey:     DeadlineHoursOnly,
			param:          "24",
			expectContains: "24",
		},
		{
			name:           "Format achievement congrats in Russian",
			locale:         Ru,
			messageKey:     NotificationAchievementCongrats,
			param:          "🎯 Меткий стрелок",
			expectContains: "🎯 Меткий стрелок",
		},
		{
			name:           "Format achievement congrats in English",
			locale:         En,
			messageKey:     NotificationAchievementCongrats,
			param:          "🎯 Sharpshooter",
			expectContains: "🎯 Sharpshooter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			loc := NewLocale(tt.locale)
			localizer, err := NewLocalizer(ctx, loc)
			if err != nil {
				t.Fatalf("Failed to create localizer: %v", err)
			}

			result := localizer.MustLocalizeWithTemplate(tt.messageKey, tt.param)

			if result == "" {
				t.Error("Expected non-empty result")
				return
			}

			// Verify the parameter is included in the result
			if !contains(result, tt.expectContains) {
				t.Errorf("Expected result to contain '%s', got: %s", tt.expectContains, result)
			}
		})
	}
}

// TestLocalizerMultipleParameters tests message formatting with multiple parameters
func TestLocalizerMultipleParameters(t *testing.T) {
	tests := []struct {
		name           string
		locale         string
		messageKey     string
		params         []string
		expectContains []string
	}{
		{
			name:           "Format event type with icon and label in Russian",
			locale:         Ru,
			messageKey:     NotificationNewEventType,
			params:         []string{"1️⃣", "Бинарное"},
			expectContains: []string{"1️⃣", "Бинарное"},
		},
		{
			name:           "Format event type with icon and label in English",
			locale:         En,
			messageKey:     NotificationNewEventType,
			params:         []string{"1️⃣", "Binary"},
			expectContains: []string{"1️⃣", "Binary"},
		},
		{
			name:           "Format deadline with days and hours in Russian",
			locale:         Ru,
			messageKey:     DeadlineDaysHours,
			params:         []string{"2", "12"},
			expectContains: []string{"2", "12"},
		},
		{
			name:           "Format deadline with days and hours in English",
			locale:         En,
			messageKey:     DeadlineDaysHours,
			params:         []string{"2", "12"},
			expectContains: []string{"2", "12"},
		},
		{
			name:           "Format option list item in Russian",
			locale:         Ru,
			messageKey:     OptionListItem,
			params:         []string{"1", "Yes"},
			expectContains: []string{"1", "Yes"},
		},
		{
			name:           "Format option list item in English",
			locale:         En,
			messageKey:     OptionListItem,
			params:         []string{"1", "Yes"},
			expectContains: []string{"1", "Yes"},
		},
		{
			name:           "Format results stats in Russian",
			locale:         Ru,
			messageKey:     NotificationResultsStats,
			params:         []string{"5", "10"},
			expectContains: []string{"5", "10"},
		},
		{
			name:           "Format results stats in English",
			locale:         En,
			messageKey:     NotificationResultsStats,
			params:         []string{"5", "10"},
			expectContains: []string{"5", "10"},
		},
		{
			name:           "Format rating entry with three parameters in Russian",
			locale:         Ru,
			messageKey:     RatingTopEntry,
			params:         []string{"1", "John Doe", "100"},
			expectContains: []string{"1", "John Doe", "100"},
		},
		{
			name:           "Format rating entry with three parameters in English",
			locale:         En,
			messageKey:     RatingTopEntry,
			params:         []string{"1", "John Doe", "100"},
			expectContains: []string{"1", "John Doe", "100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			loc := NewLocale(tt.locale)
			localizer, err := NewLocalizer(ctx, loc)
			if err != nil {
				t.Fatalf("Failed to create localizer: %v", err)
			}

			result := localizer.MustLocalizeWithTemplate(tt.messageKey, tt.params...)

			if result == "" {
				t.Error("Expected non-empty result")
				return
			}

			// Verify all parameters are included in the result
			for _, expected := range tt.expectContains {
				if !contains(result, expected) {
					t.Errorf("Expected result to contain '%s', got: %s", expected, result)
				}
			}
		})
	}
}

// TestLocalizerPanicsOnMissingKey tests that MustLocalize panics on missing keys
// This is the expected behavior - the "Must" prefix indicates it will panic on error
func TestLocalizerPanicsOnMissingKey(t *testing.T) {
	tests := []struct {
		name       string
		locale     string
		messageKey string
	}{
		{
			name:       "Missing key in Russian locale should panic",
			locale:     Ru,
			messageKey: "NonExistentKey12345",
		},
		{
			name:       "Missing key in English locale should panic",
			locale:     En,
			messageKey: "NonExistentKey12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			loc := NewLocale(tt.locale)
			localizer, err := NewLocalizer(ctx, loc)
			if err != nil {
				t.Fatalf("Failed to create localizer: %v", err)
			}

			// Expect panic when using missing key
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected panic for missing key '%s', but didn't panic", tt.messageKey)
				}
			}()

			// This should panic
			_ = localizer.MustLocalize(tt.messageKey)
		})
	}
}

// TestLocalizerPanicsOnMissingKeyWithTemplate tests that MustLocalizeWithTemplate panics on missing keys
func TestLocalizerPanicsOnMissingKeyWithTemplate(t *testing.T) {
	tests := []struct {
		name       string
		locale     string
		messageKey string
		params     []string
	}{
		{
			name:       "Missing key with single parameter in Russian should panic",
			locale:     Ru,
			messageKey: "NonExistentKey12345",
			params:     []string{"param1"},
		},
		{
			name:       "Missing key with multiple parameters in English should panic",
			locale:     En,
			messageKey: "NonExistentKey12345",
			params:     []string{"param1", "param2", "param3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			loc := NewLocale(tt.locale)
			localizer, err := NewLocalizer(ctx, loc)
			if err != nil {
				t.Fatalf("Failed to create localizer: %v", err)
			}

			// Expect panic when using missing key
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected panic for missing key '%s', but didn't panic", tt.messageKey)
				}
			}()

			// This should panic
			_ = localizer.MustLocalizeWithTemplate(tt.messageKey, tt.params...)
		})
	}
}

// TestLocalizerNonParameterizedMessages tests messages without parameters
func TestLocalizerNonParameterizedMessages(t *testing.T) {
	tests := []struct {
		name       string
		locale     string
		messageKey string
	}{
		{
			name:       "Get help title in Russian",
			locale:     Ru,
			messageKey: HelpBotTitle,
		},
		{
			name:       "Get help title in English",
			locale:     En,
			messageKey: HelpBotTitle,
		},
		{
			name:       "Get notification title in Russian",
			locale:     Ru,
			messageKey: NotificationNewEventTitle,
		},
		{
			name:       "Get notification title in English",
			locale:     En,
			messageKey: NotificationNewEventTitle,
		},
		{
			name:       "Get achievement name in Russian",
			locale:     Ru,
			messageKey: AchievementSharpshooterName,
		},
		{
			name:       "Get achievement name in English",
			locale:     En,
			messageKey: AchievementSharpshooterName,
		},
		{
			name:       "Get error message in Russian",
			locale:     Ru,
			messageKey: ErrorUnauthorized,
		},
		{
			name:       "Get error message in English",
			locale:     En,
			messageKey: ErrorUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			loc := NewLocale(tt.locale)
			localizer, err := NewLocalizer(ctx, loc)
			if err != nil {
				t.Fatalf("Failed to create localizer: %v", err)
			}

			result := localizer.MustLocalize(tt.messageKey)

			if result == "" {
				t.Error("Expected non-empty result")
				return
			}

			// Verify result doesn't contain template placeholders
			if contains(result, "{{") || contains(result, "}}") {
				t.Errorf("Result should not contain template placeholders: %s", result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
