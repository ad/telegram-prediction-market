package bot

import (
	"testing"

	"telegram-prediction-bot/internal/config"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestAdminAuthorization(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("users not in admin list are rejected with error", prop.ForAll(
		func(adminIDs []int64, testUserID int64) bool {
			// Create config with admin IDs
			cfg := &config.Config{
				AdminUserIDs: adminIDs,
			}

			// Create handler with config
			handler := &BotHandler{
				config: cfg,
				logger: &mockLogger{},
			}

			// Check if user is admin
			isAdmin := handler.isAdmin(testUserID)

			// Verify authorization logic
			shouldBeAdmin := false
			for _, adminID := range adminIDs {
				if adminID == testUserID {
					shouldBeAdmin = true
					break
				}
			}

			if isAdmin != shouldBeAdmin {
				t.Logf("Authorization mismatch: user %d, expected %v, got %v", testUserID, shouldBeAdmin, isAdmin)
				return false
			}

			// If user is not admin, they should be rejected
			if !shouldBeAdmin && isAdmin {
				t.Logf("Non-admin user %d was incorrectly authorized", testUserID)
				return false
			}

			return true
		},
		gen.SliceOfN(10, gen.Int64Range(1, 1000)),
		gen.Int64Range(1, 2000),
	))

	properties.TestingRun(t)
}

// Additional test: Verify empty admin list rejects all users
func TestAdminAuthorizationEmptyList(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("empty admin list rejects all users", prop.ForAll(
		func(testUserID int64) bool {
			// Create config with empty admin list
			cfg := &config.Config{
				AdminUserIDs: []int64{},
			}

			// Create handler with config
			handler := &BotHandler{
				config: cfg,
				logger: &mockLogger{},
			}

			// Check if user is admin
			isAdmin := handler.isAdmin(testUserID)

			// No user should be admin with empty list
			if isAdmin {
				t.Logf("User %d was incorrectly authorized with empty admin list", testUserID)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

// Additional test: Verify admin list with duplicates works correctly
func TestAdminAuthorizationWithDuplicates(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("admin list with duplicates works correctly", prop.ForAll(
		func(adminID int64) bool {
			// Create config with duplicate admin IDs
			cfg := &config.Config{
				AdminUserIDs: []int64{adminID, adminID, adminID},
			}

			// Create handler with config
			handler := &BotHandler{
				config: cfg,
				logger: &mockLogger{},
			}

			// Check if user is admin
			isAdmin := handler.isAdmin(adminID)

			// User should be admin
			if !isAdmin {
				t.Logf("Admin user %d was incorrectly rejected", adminID)
				return false
			}

			// Check a different user
			otherUserID := adminID + 1
			isOtherAdmin := handler.isAdmin(otherUserID)

			// Other user should not be admin
			if isOtherAdmin {
				t.Logf("Non-admin user %d was incorrectly authorized", otherUserID)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}
