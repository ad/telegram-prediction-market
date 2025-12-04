package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	TelegramToken string
	AdminUserIDs  []int64
	GroupID       int64
	DatabasePath  string
	LogLevel      string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN environment variable is required")
	}

	groupIDStr := os.Getenv("GROUP_ID")
	if groupIDStr == "" {
		return nil, fmt.Errorf("GROUP_ID environment variable is required")
	}
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GROUP_ID: %w", err)
	}

	adminIDsStr := os.Getenv("ADMIN_USER_IDS")
	if adminIDsStr == "" {
		return nil, fmt.Errorf("ADMIN_USER_IDS environment variable is required")
	}
	adminIDs, err := parseAdminIDs(adminIDsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ADMIN_USER_IDS: %w", err)
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/bot.db" // default value
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO" // default value
	}

	return &Config{
		TelegramToken: token,
		AdminUserIDs:  adminIDs,
		GroupID:       groupID,
		DatabasePath:  dbPath,
		LogLevel:      logLevel,
	}, nil
}

// parseAdminIDs parses comma-separated admin user IDs
func parseAdminIDs(s string) ([]int64, error) {
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid admin ID '%s': %w", part, err)
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one admin ID is required")
	}

	return ids, nil
}
