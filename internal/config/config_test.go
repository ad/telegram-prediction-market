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
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("invalid MIN_EVENTS_TO_CREATE values are rejected", prop.ForAll(
		func(invalidValue string) bool {
			_ = os.Setenv("MIN_EVENTS_TO_CREATE", invalidValue)

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
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	// Unset MIN_EVENTS_TO_CREATE to test default
	_ = os.Unsetenv("MIN_EVENTS_TO_CREATE")

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
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

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
			_ = os.Setenv("MIN_EVENTS_TO_CREATE", tc.value)

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
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

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
			_ = os.Setenv("MIN_EVENTS_TO_CREATE", tc.value)

			config, err := Load()
			if err == nil {
				t.Errorf("Expected error for invalid value '%s', but got valid config: %+v", tc.value, config)
			}
		})
	}
}

// TestDefaultGroupNameDefault tests that default value is used when not set
func TestDefaultGroupNameDefault(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origDefaultGroupName := os.Getenv("DEFAULT_GROUP_NAME")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("DEFAULT_GROUP_NAME", origDefaultGroupName)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	// Unset DEFAULT_GROUP_NAME to test default
	_ = os.Unsetenv("DEFAULT_GROUP_NAME")

	config, err := Load()
	if err != nil {
		t.Fatalf("Expected no error when DEFAULT_GROUP_NAME not set, got: %v", err)
	}

	if config.DefaultGroupName != "Default Group" {
		t.Errorf("Expected default DefaultGroupName to be 'Default Group', got: %s", config.DefaultGroupName)
	}
}

// TestDefaultGroupNameCustomValue tests that custom values are accepted
func TestDefaultGroupNameCustomValue(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origDefaultGroupName := os.Getenv("DEFAULT_GROUP_NAME")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("DEFAULT_GROUP_NAME", origDefaultGroupName)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	testCases := []struct {
		name     string
		value    string
		expected string
	}{
		{"simple name", "My Group", "My Group"},
		{"with special chars", "Test Group #1", "Test Group #1"},
		{"with unicode", "Группа 测试", "Группа 测试"},
		{"empty string uses default", "", "Default Group"},
		{"single char", "A", "A"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Setenv("DEFAULT_GROUP_NAME", tc.value)

			config, err := Load()
			if err != nil {
				t.Fatalf("Expected no error for value '%s', got: %v", tc.value, err)
			}

			if config.DefaultGroupName != tc.expected {
				t.Errorf("Expected DefaultGroupName to be '%s', got: '%s'", tc.expected, config.DefaultGroupName)
			}
		})
	}
}

// TestMaxGroupsPerAdminDefault tests that default value is used when not set
func TestMaxGroupsPerAdminDefault(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxGroups := os.Getenv("MAX_GROUPS_PER_ADMIN")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_GROUPS_PER_ADMIN", origMaxGroups)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	// Unset MAX_GROUPS_PER_ADMIN to test default
	_ = os.Unsetenv("MAX_GROUPS_PER_ADMIN")

	config, err := Load()
	if err != nil {
		t.Fatalf("Expected no error when MAX_GROUPS_PER_ADMIN not set, got: %v", err)
	}

	if config.MaxGroupsPerAdmin != 10 {
		t.Errorf("Expected default MaxGroupsPerAdmin to be 10, got: %d", config.MaxGroupsPerAdmin)
	}
}

// TestMaxGroupsPerAdminValidValues tests that valid positive integer values are accepted
func TestMaxGroupsPerAdminValidValues(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxGroups := os.Getenv("MAX_GROUPS_PER_ADMIN")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_GROUPS_PER_ADMIN", origMaxGroups)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	testCases := []struct {
		name     string
		value    string
		expected int
	}{
		{"one", "1", 1},
		{"five", "5", 5},
		{"ten", "10", 10},
		{"fifty", "50", 50},
		{"large value", "1000", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Setenv("MAX_GROUPS_PER_ADMIN", tc.value)

			config, err := Load()
			if err != nil {
				t.Fatalf("Expected no error for valid value '%s', got: %v", tc.value, err)
			}

			if config.MaxGroupsPerAdmin != tc.expected {
				t.Errorf("Expected MaxGroupsPerAdmin to be %d, got: %d", tc.expected, config.MaxGroupsPerAdmin)
			}
		})
	}
}

// TestMaxGroupsPerAdminInvalidValues tests that invalid values are rejected
func TestMaxGroupsPerAdminInvalidValues(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxGroups := os.Getenv("MAX_GROUPS_PER_ADMIN")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_GROUPS_PER_ADMIN", origMaxGroups)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	testCases := []struct {
		name  string
		value string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"negative large", "-100"},
		{"non-integer", "abc"},
		{"float", "3.14"},
		{"whitespace", "  "},
		{"special chars", "!@#"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Setenv("MAX_GROUPS_PER_ADMIN", tc.value)

			config, err := Load()
			if err == nil {
				t.Errorf("Expected error for invalid value '%s', but got valid config: %+v", tc.value, config)
			}
		})
	}
}

// TestMaxMembershipsPerUserDefault tests that default value is used when not set
func TestMaxMembershipsPerUserDefault(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxMemberships := os.Getenv("MAX_MEMBERSHIPS_PER_USER")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_MEMBERSHIPS_PER_USER", origMaxMemberships)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	// Unset MAX_MEMBERSHIPS_PER_USER to test default
	_ = os.Unsetenv("MAX_MEMBERSHIPS_PER_USER")

	config, err := Load()
	if err != nil {
		t.Fatalf("Expected no error when MAX_MEMBERSHIPS_PER_USER not set, got: %v", err)
	}

	if config.MaxMembershipsPerUser != 20 {
		t.Errorf("Expected default MaxMembershipsPerUser to be 20, got: %d", config.MaxMembershipsPerUser)
	}
}

// TestMaxMembershipsPerUserValidValues tests that valid positive integer values are accepted
func TestMaxMembershipsPerUserValidValues(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxMemberships := os.Getenv("MAX_MEMBERSHIPS_PER_USER")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_MEMBERSHIPS_PER_USER", origMaxMemberships)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	testCases := []struct {
		name     string
		value    string
		expected int
	}{
		{"one", "1", 1},
		{"ten", "10", 10},
		{"twenty", "20", 20},
		{"fifty", "50", 50},
		{"large value", "1000", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Setenv("MAX_MEMBERSHIPS_PER_USER", tc.value)

			config, err := Load()
			if err != nil {
				t.Fatalf("Expected no error for valid value '%s', got: %v", tc.value, err)
			}

			if config.MaxMembershipsPerUser != tc.expected {
				t.Errorf("Expected MaxMembershipsPerUser to be %d, got: %d", tc.expected, config.MaxMembershipsPerUser)
			}
		})
	}
}

// TestMaxMembershipsPerUserInvalidValues tests that invalid values are rejected
func TestMaxMembershipsPerUserInvalidValues(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origGroupID := os.Getenv("GROUP_ID")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxMemberships := os.Getenv("MAX_MEMBERSHIPS_PER_USER")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("GROUP_ID", origGroupID)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_MEMBERSHIPS_PER_USER", origMaxMemberships)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
	_ = os.Setenv("GROUP_ID", "123456")
	_ = os.Setenv("ADMIN_USER_IDS", "111,222")

	testCases := []struct {
		name  string
		value string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"negative large", "-100"},
		{"non-integer", "abc"},
		{"float", "3.14"},
		{"whitespace", "  "},
		{"special chars", "!@#"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Setenv("MAX_MEMBERSHIPS_PER_USER", tc.value)

			config, err := Load()
			if err == nil {
				t.Errorf("Expected error for invalid value '%s', but got valid config: %+v", tc.value, config)
			}
		})
	}
}
