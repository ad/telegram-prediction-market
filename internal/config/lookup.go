package config

import (
	"os"
	"strconv"
	"time"
)

func (c *Config) LookupEnvOrString(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return defaultVal
}

func (c *Config) LookupEnvOrInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		if x, err := strconv.Atoi(val); err == nil {
			return x
		}
	}

	return defaultVal
}

func (c *Config) LookupEnvOrInt64(key string, defaultVal int64) int64 {
	if val, ok := os.LookupEnv(key); ok {
		if x, err := strconv.ParseInt(val, 10, 64); err == nil {
			return x
		}
	}

	return defaultVal
}

func (c *Config) LookupEnvOrBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if x, err := strconv.ParseBool(val); err == nil {
			return x
		}
	}

	return defaultVal
}

func (c *Config) LookupEnvOrDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if x, err := time.ParseDuration(val); err == nil {
			return x
		}
	}

	return defaultVal
}
