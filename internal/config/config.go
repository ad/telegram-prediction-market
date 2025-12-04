package config

// Config holds application configuration
type Config struct {
	TelegramToken string
	AdminUserIDs  []int64
	GroupID       int64
	DatabasePath  string
	LogLevel      string
}
