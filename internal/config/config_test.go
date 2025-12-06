package config

import (
	"os"
	"testing"
)

// TestMinEventsToCreateDefault tests that default value is used when not set
func TestMinEventsToCreateDefault(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMinEvents := os.Getenv("MIN_EVENTS_TO_CREATE")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
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
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMinEvents := os.Getenv("MIN_EVENTS_TO_CREATE")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MIN_EVENTS_TO_CREATE", origMinEvents)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
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

// TestMaxGroupsPerAdminDefault tests that default value is used when not set
func TestMaxGroupsPerAdminDefault(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxGroups := os.Getenv("MAX_GROUPS_PER_ADMIN")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_GROUPS_PER_ADMIN", origMaxGroups)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
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
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxGroups := os.Getenv("MAX_GROUPS_PER_ADMIN")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_GROUPS_PER_ADMIN", origMaxGroups)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
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

// TestMaxMembershipsPerUserDefault tests that default value is used when not set
func TestMaxMembershipsPerUserDefault(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("TELEGRAM_TOKEN")
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxMemberships := os.Getenv("MAX_MEMBERSHIPS_PER_USER")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_MEMBERSHIPS_PER_USER", origMaxMemberships)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
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
	origAdminIDs := os.Getenv("ADMIN_USER_IDS")
	origMaxMemberships := os.Getenv("MAX_MEMBERSHIPS_PER_USER")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("TELEGRAM_TOKEN", origToken)
		_ = os.Setenv("ADMIN_USER_IDS", origAdminIDs)
		_ = os.Setenv("MAX_MEMBERSHIPS_PER_USER", origMaxMemberships)
	}()

	// Set required valid env vars
	_ = os.Setenv("TELEGRAM_TOKEN", "test_token")
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
