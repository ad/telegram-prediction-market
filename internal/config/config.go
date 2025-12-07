package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const ConfigFileName = "/data/options.json"

// Config holds application configuration
type Config struct {
	TelegramToken         string `json:"TELEGRAM_TOKEN"`
	AdminUserIDs          []int64
	AdminIDsStr           string `json:"ADMIN_USER_IDS"`
	DatabasePath          string `json:"DATABASE"`
	LogLevel              string `json:"LOG_LEVEL"`
	Timezone              *time.Location
	TimezoneStr           string `json:"TIMEZONE"`
	MinEventsToCreate     int    `json:"MIN_EVENTS_TO_CREATE"`
	MaxGroupsPerAdmin     int    `json:"MAX_GROUPS_PER_ADMIN"`
	MaxMembershipsPerUser int    `json:"MAX_MEMBERSHIPS_PER_USER"`
	IDEncodingAlphabet    string `json:"ID_ENCODING_ALPHABET"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		TelegramToken:         os.Getenv("TELEGRAM_TOKEN"),
		AdminIDsStr:           os.Getenv("ADMIN_USER_IDS"),
		DatabasePath:          os.Getenv("DATABASE_PATH"),
		LogLevel:              os.Getenv("LOG_LEVEL"),
		TimezoneStr:           os.Getenv("TIMEZONE"),
		MinEventsToCreate:     lookupEnvOrInt("MIN_EVENTS_TO_CREATE", 0),
		MaxGroupsPerAdmin:     lookupEnvOrInt("MAX_GROUPS_PER_ADMIN", 0),
		MaxMembershipsPerUser: lookupEnvOrInt("MAX_MEMBERSHIPS_PER_USER", 0),
		IDEncodingAlphabet:    os.Getenv("ID_ENCODING_ALPHABET"),
	}

	if _, err := os.Stat(ConfigFileName); err == nil {
		jsonFile, err := os.Open(ConfigFileName)
		if err == nil {
			byteValue, _ := io.ReadAll(jsonFile)
			if err = json.Unmarshal(byteValue, &config); err != nil {
				fmt.Printf("error on unmarshal config from file %s\n", err.Error())
			}
		}
	}

	if config.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN environment variable is required")
	}

	if config.AdminIDsStr == "" {
		return nil, fmt.Errorf("ADMIN_USER_IDS environment variable is required")
	}

	adminIDs, err := parseAdminIDs(config.AdminIDsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ADMIN_USER_IDS: %w", err)
	}

	if config.DatabasePath == "" {
		config.DatabasePath = "/config/telegram-prediction-market.db" // default value
	}

	if config.LogLevel == "" {
		config.LogLevel = "INFO" // default value
	}

	// Load timezone (default to UTC)
	if config.TimezoneStr == "" {
		config.TimezoneStr = "UTC" // default value
	}
	timezone, err := time.LoadLocation(config.TimezoneStr)
	if err != nil {
		return nil, fmt.Errorf("invalid TIMEZONE '%s': %w", config.TimezoneStr, err)
	}

	// Load minimum events to create (default to 3)
	if config.MinEventsToCreate < 0 {
		config.MinEventsToCreate = 3
	}

	// Load max groups per admin (default to 10)
	if config.MaxGroupsPerAdmin <= 0 {
		config.MaxGroupsPerAdmin = 10
	}

	// Load max memberships per user (default to 20)
	if config.MaxMembershipsPerUser <= 0 {
		config.MaxMembershipsPerUser = 20
	}

	// Load ID encoding alphabet (default to base62)
	if config.IDEncodingAlphabet == "" {
		config.IDEncodingAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	}

	return &Config{
		TelegramToken:         config.TelegramToken,
		AdminUserIDs:          adminIDs,
		DatabasePath:          config.DatabasePath,
		LogLevel:              config.LogLevel,
		Timezone:              timezone,
		MinEventsToCreate:     config.MinEventsToCreate,
		MaxGroupsPerAdmin:     config.MaxGroupsPerAdmin,
		MaxMembershipsPerUser: config.MaxMembershipsPerUser,
		IDEncodingAlphabet:    config.IDEncodingAlphabet,
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
