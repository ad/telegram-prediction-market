package config

import (
	"os"
	"strconv"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestInvalidConfigRejection tests: Invalid config rejection
func TestInvalidConfigRejection(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMinEvents := os.Getenv("MIN_EVENTS_TO_CREATE")

	defer func() {
		// Restore original env vars
		os.Setenv("TELEGRAM_TOKEN", origToken)
		os.Setenv("GROUP_ID", origGroupID)
		os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	os.Setenv("TELEGRAM_TOKEN", "test_token")
	os.Setenv("GROUP_ID", "123456")
	os.Setenv("ADMIN_USER_IDS", "111,222")

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("invalid MIN_EVENTS_TO_CREATE values are rejected", prop.ForAll(
		func(invalidValue string) bool {
			os.Setenv("MIN_EVENTS_TO_CREATE", invalidValue)

			config, err := Load()

			// Should return an error for invalid values
			if err == nil {
				t.Logf("Expected error for invalid value '%s', but got valid config: %+v", invalidValue, config)
				return false
			}

			return true
		},
		// Generate invalid values: negative integers and non-integer strings
		// Note: empty string is NOT invalid - it uses the default value
		gen.OneGenOf(
			// Negative integers
			gen.IntRange(-1000, -1).Map(func(n int) string {
				return strconv.Itoa(n)
			}),
			// Non-integer strings (excluding empty string which is valid)
			gen.OneConstOf("abc", "12.5", "1e5", "  ", "NaN", "null", "true", "-", "+", "not_a_number"),
		),
	))

	properties.TestingRun(t)
}

// TestMinEventsToCreateDefault tests that default value is used when not set
func TestMinEventsToCreateDefault(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMinEvents := os.Getenv("MIN_EVENTS_TO_CREATE")

	defer func() {
		// Restore original env vars
		os.Setenv("TELEGRAM_TOKEN", origToken)
		os.Setenv("GROUP_ID", origGroupID)
		os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	os.Setenv("TELEGRAM_TOKEN", "test_token")
	os.Setenv("GROUP_ID", "123456")
	os.Setenv("ADMIN_USER_IDS", "111,222")

	// Unset MIN_EVENTS_TO_CREATE to test default
	os.Unsetenv("MIN_EVENTS_TO_CREATE")

	config, err := Load()
	if err != nil {
		t.Fatalf("Expected no error when MIN_EVENTS_TO_CREATE not set, got: %v", err)
	}

	if config.MinEventsToCreate != 3 {
		t.Errorf("Expected default MinEventsToCreate to be 3, got: %d", config.MinEventsToCreate)
	}
}

// TestMinEventsToCreateValidValues tests that valid integer values are accepted
func TestMinEventsToCreateValidValues(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMinEvents := os.Getenv("MIN_EVENTS_TO_CREATE")

	defer func() {
		// Restore original env vars
		os.Setenv("TELEGRAM_TOKEN", origToken)
		os.Setenv("GROUP_ID", origGroupID)
		os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	os.Setenv("TELEGRAM_TOKEN", "test_token")
	os.Setenv("GROUP_ID", "123456")
	os.Setenv("ADMIN_USER_IDS", "111,222")

	testCases := []struct {
		name     string
		value    string
		expected int
	}{
		{"zero", "0", 0},
		{"one", "1", 1},
		{"five", "5", 5},
		{"ten", "10", 10},
		{"large value", "1000", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("MIN_EVENTS_TO_CREATE", tc.value)

			config, err := Load()
			if err != nil {
				t.Fatalf("Expected no error for valid value '%s', got: %v", tc.value, err)
			}

			if config.MinEventsToCreate != tc.expected {
				t.Errorf("Expected MinEventsToCreate to be %d, got: %d", tc.expected, config.MinEventsToCreate)
			}
		})
	}
}

// TestMinEventsToCreateInvalidValues tests that invalid values are rejected
func TestMinEventsToCreateInvalidValues(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMinEvents := os.Getenv("MIN_EVENTS_TO_CREATE")

	defer func() {
		// Restore original env vars
		os.Setenv("TELEGRAM_TOKEN", origToken)
		os.Setenv("GROUP_ID", origGroupID)
		os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	os.Setenv("TELEGRAM_TOKEN", "test_token")
	os.Setenv("GROUP_ID", "123456")
	os.Setenv("ADMIN_USER_IDS", "111,222")

	testCases := []struct {
		name  string
		value string
	}{
		{"negative", "-1"},
		{"negative large", "-100"},
		{"non-integer", "abc"},
		{"float", "3.14"},
		{"whitespace", "  "},
		{"special chars", "!@#"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("MIN_EVENTS_TO_CREATE", tc.value)

			config, err := Load()
			if err == nil {
				t.Errorf("Expected error for invalid value '%s', but got valid config: %+v", tc.value, config)
			}
		})
	}
}
