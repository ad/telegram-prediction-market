package domain

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestDeepLinkRoundTrip tests Property 6: Deep-link Round Trip
func TestDeepLinkRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("generating and parsing deep-link preserves group ID", prop.ForAll(
		func(groupID int64, botUsername string) bool {
			// Skip empty bot usernames as they're invalid
			if botUsername == "" {
				return true
			}

			service := NewDeepLinkService(botUsername)

			// Generate deep-link
			deepLink := service.GenerateGroupInviteLink(groupID)

			// Extract the start parameter from the deep-link
			// Format: https://t.me/{bot_username}?start=group_{groupID}
			// We need to extract "group_{groupID}" part
			startParam := ""
			if len(deepLink) > 0 {
				// Find the "start=" part
				startIndex := -1
				for i := 0; i < len(deepLink)-6; i++ {
					if deepLink[i:i+6] == "start=" {
						startIndex = i + 6
						break
					}
				}
				if startIndex != -1 {
					startParam = deepLink[startIndex:]
				}
			}

			// Parse the group ID back
			parsedGroupID, err := service.ParseGroupIDFromStart(startParam)
			if err != nil {
				return false
			}

			// Verify the round trip preserves the group ID
			return parsedGroupID == groupID
		},
		gen.Int64(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

// TestDeepLinkFormatValidity tests Property 5: Deep-link Format Validity
func TestDeepLinkFormatValidity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("generated deep-link is a valid Telegram bot URL", prop.ForAll(
		func(groupID int64, botUsername string) bool {
			// Skip empty bot usernames as they're invalid
			if botUsername == "" {
				return true
			}

			service := NewDeepLinkService(botUsername)

			// Generate deep-link
			deepLink := service.GenerateGroupInviteLink(groupID)

			// Verify the format: https://t.me/{bot_username}?start=group_{groupID}
			// Check that it starts with https://t.me/
			if len(deepLink) < 15 || deepLink[:13] != "https://t.me/" {
				return false
			}

			// Check that it contains ?start=
			hasStart := false
			for i := 0; i < len(deepLink)-7; i++ {
				if deepLink[i:i+7] == "?start=" {
					hasStart = true
					break
				}
			}
			if !hasStart {
				return false
			}

			// Check that it contains group_
			hasGroupPrefix := false
			for i := 0; i < len(deepLink)-6; i++ {
				if deepLink[i:i+6] == "group_" {
					hasGroupPrefix = true
					break
				}
			}
			if !hasGroupPrefix {
				return false
			}

			// Check that the bot username is in the URL
			hasUsername := false
			for i := 0; i < len(deepLink)-len(botUsername); i++ {
				if deepLink[i:i+len(botUsername)] == botUsername {
					hasUsername = true
					break
				}
			}
			return hasUsername
		},
		gen.Int64(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

// Unit Tests

func TestGenerateGroupInviteLink(t *testing.T) {
	tests := []struct {
		name        string
		botUsername string
		groupID     int64
		expected    string
	}{
		{
			name:        "positive group ID",
			botUsername: "testbot",
			groupID:     123,
			expected:    "https://t.me/testbot?start=group_123",
		},
		{
			name:        "negative group ID",
			botUsername: "mybot",
			groupID:     -456,
			expected:    "https://t.me/mybot?start=group_-456",
		},
		{
			name:        "zero group ID",
			botUsername: "zerobot",
			groupID:     0,
			expected:    "https://t.me/zerobot?start=group_0",
		},
		{
			name:        "large group ID",
			botUsername: "bigbot",
			groupID:     9223372036854775807, // max int64
			expected:    "https://t.me/bigbot?start=group_9223372036854775807",
		},
		{
			name:        "bot username with underscore",
			botUsername: "test_bot",
			groupID:     100,
			expected:    "https://t.me/test_bot?start=group_100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDeepLinkService(tt.botUsername)
			result := service.GenerateGroupInviteLink(tt.groupID)
			if result != tt.expected {
				t.Errorf("GenerateGroupInviteLink() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseGroupIDFromStart(t *testing.T) {
	service := NewDeepLinkService("testbot")

	tests := []struct {
		name        string
		startParam  string
		expectedID  int64
		expectError bool
	}{
		{
			name:        "valid positive group ID",
			startParam:  "group_123",
			expectedID:  123,
			expectError: false,
		},
		{
			name:        "valid negative group ID",
			startParam:  "group_-456",
			expectedID:  -456,
			expectError: false,
		},
		{
			name:        "valid zero group ID",
			startParam:  "group_0",
			expectedID:  0,
			expectError: false,
		},
		{
			name:        "valid large group ID",
			startParam:  "group_9223372036854775807",
			expectedID:  9223372036854775807,
			expectError: false,
		},
		{
			name:        "invalid format - missing group_ prefix",
			startParam:  "123",
			expectedID:  0,
			expectError: true,
		},
		{
			name:        "invalid format - wrong prefix",
			startParam:  "user_123",
			expectedID:  0,
			expectError: true,
		},
		{
			name:        "invalid format - empty string",
			startParam:  "",
			expectedID:  0,
			expectError: true,
		},
		{
			name:        "invalid format - group_ only",
			startParam:  "group_",
			expectedID:  0,
			expectError: true,
		},
		{
			name:        "invalid format - non-numeric ID",
			startParam:  "group_abc",
			expectedID:  0,
			expectError: true,
		},
		{
			name:        "invalid format - group_ with spaces",
			startParam:  "group_ 123",
			expectedID:  0,
			expectError: true,
		},
		{
			name:        "invalid format - multiple underscores",
			startParam:  "group__123",
			expectedID:  0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ParseGroupIDFromStart(tt.startParam)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseGroupIDFromStart() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ParseGroupIDFromStart() unexpected error: %v", err)
				}
				if result != tt.expectedID {
					t.Errorf("ParseGroupIDFromStart() = %v, want %v", result, tt.expectedID)
				}
			}
		})
	}
}

func TestURLFormatValidation(t *testing.T) {
	tests := []struct {
		name        string
		botUsername string
		groupID     int64
	}{
		{
			name:        "standard bot username",
			botUsername: "prediction_bot",
			groupID:     42,
		},
		{
			name:        "short bot username",
			botUsername: "bot",
			groupID:     1,
		},
		{
			name:        "long bot username",
			botUsername: "very_long_bot_username_test",
			groupID:     999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDeepLinkService(tt.botUsername)
			deepLink := service.GenerateGroupInviteLink(tt.groupID)

			// Verify URL starts with https://t.me/
			if len(deepLink) < 13 || deepLink[:13] != "https://t.me/" {
				t.Errorf("URL does not start with https://t.me/: %s", deepLink)
			}

			// Verify URL contains bot username
			found := false
			for i := 0; i <= len(deepLink)-len(tt.botUsername); i++ {
				if deepLink[i:i+len(tt.botUsername)] == tt.botUsername {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("URL does not contain bot username %s: %s", tt.botUsername, deepLink)
			}

			// Verify URL contains ?start=
			found = false
			for i := 0; i <= len(deepLink)-7; i++ {
				if deepLink[i:i+7] == "?start=" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("URL does not contain ?start=: %s", deepLink)
			}

			// Verify URL contains group_ prefix
			found = false
			for i := 0; i <= len(deepLink)-6; i++ {
				if deepLink[i:i+6] == "group_" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("URL does not contain group_ prefix: %s", deepLink)
			}
		})
	}
}
