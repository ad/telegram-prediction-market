package domain

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// mockEncoder is a simple encoder for testing that uses base10 string representation
type mockEncoder struct{}

func (m *mockEncoder) Encode(num int64) (string, error) {
	if num < 0 {
		return "", fmt.Errorf("negative numbers not supported")
	}
	return fmt.Sprintf("%d", num), nil
}

func (m *mockEncoder) Decode(encoded string) (int64, error) {
	if encoded == "" {
		return 0, fmt.Errorf("empty string")
	}
	var num int64
	_, err := fmt.Sscanf(encoded, "%d", &num)
	if err != nil {
		return 0, err
	}
	return num, nil
}

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
			// Skip negative IDs as encoder doesn't support them
			if groupID < 0 {
				return true
			}

			service := NewDeepLinkService(botUsername, &mockEncoder{})

			// Generate deep-link
			deepLink, err := service.GenerateGroupInviteLink(groupID)
			if err != nil {
				return false
			}

			// Extract the start parameter from the deep-link
			// Format: https://t.me/{bot_username}?start=group_{encodedGroupID}
			// We need to extract "group_{encodedGroupID}" part
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
			// Skip negative IDs as encoder doesn't support them
			if groupID < 0 {
				return true
			}

			service := NewDeepLinkService(botUsername, &mockEncoder{})

			// Generate deep-link
			deepLink, err := service.GenerateGroupInviteLink(groupID)
			if err != nil {
				return false
			}

			// Verify the format: https://t.me/{bot_username}?start=group_{encodedGroupID}
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
			name:        "zero group ID",
			botUsername: "zerobot",
			groupID:     0,
			expected:    "https://t.me/zerobot?start=group_0",
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
			service := NewDeepLinkService(tt.botUsername, &mockEncoder{})
			result, err := service.GenerateGroupInviteLink(tt.groupID)
			if err != nil {
				t.Errorf("GenerateGroupInviteLink() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("GenerateGroupInviteLink() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseGroupIDFromStart(t *testing.T) {
	service := NewDeepLinkService("testbot", &mockEncoder{})

	tests := []struct {
		name        string
		startParam  string
		expectedID  int64
		expectError bool
	}{
		{
			name:        "valid encoded group ID",
			startParam:  "group_123",
			expectedID:  123,
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
			service := NewDeepLinkService(tt.botUsername, &mockEncoder{})
			deepLink, err := service.GenerateGroupInviteLink(tt.groupID)
			if err != nil {
				t.Errorf("GenerateGroupInviteLink() error = %v", err)
				return
			}

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
