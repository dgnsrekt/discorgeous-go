package relay

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all ntfy relay configuration.
type Config struct {
	// Ntfy settings
	NtfyServer string
	NtfyTopics []string

	// Discorgeous API settings
	DiscorgeousAPIURL      string
	DiscorgeousBearerToken string

	// Formatting settings
	Prefix        string
	Interrupt     bool
	DedupeWindow  time.Duration
	MaxTextLength int

	// Logging settings
	LogLevel  string
	LogFormat string
}

// Load reads relay configuration from environment variables with sane defaults.
func Load() (*Config, error) {
	topicsStr := os.Getenv("NTFY_TOPICS")
	var topics []string
	if topicsStr != "" {
		for _, t := range strings.Split(topicsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				topics = append(topics, t)
			}
		}
	}

	cfg := &Config{
		// Ntfy settings
		NtfyServer: getEnvString("NTFY_SERVER", "https://ntfy.sh"),
		NtfyTopics: topics,

		// Discorgeous API settings
		DiscorgeousAPIURL:      getEnvString("DISCORGEOUS_API_URL", "http://discorgeous:8080"),
		DiscorgeousBearerToken: os.Getenv("DISCORGEOUS_BEARER_TOKEN"),

		// Formatting settings
		Prefix:        os.Getenv("NTFY_PREFIX"),
		Interrupt:     getEnvBool("NTFY_INTERRUPT", false),
		DedupeWindow:  getEnvDuration("NTFY_DEDUPE_WINDOW", 0),
		MaxTextLength: getEnvInt("NTFY_MAX_TEXT_LENGTH", 1000),

		// Logging settings
		LogLevel:  getEnvString("LOG_LEVEL", "info"),
		LogFormat: getEnvString("LOG_FORMAT", "text"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that required configuration values are set.
func (c *Config) Validate() error {
	if len(c.NtfyTopics) == 0 {
		return errors.New("NTFY_TOPICS is required (comma-separated list of topics)")
	}

	if c.NtfyServer == "" {
		return errors.New("NTFY_SERVER cannot be empty")
	}

	if c.DiscorgeousAPIURL == "" {
		return errors.New("DISCORGEOUS_API_URL cannot be empty")
	}

	if c.MaxTextLength < 1 {
		return errors.New("NTFY_MAX_TEXT_LENGTH must be at least 1")
	}

	if c.DedupeWindow < 0 {
		return errors.New("NTFY_DEDUPE_WINDOW must be non-negative")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.LogLevel] {
		return errors.New("LOG_LEVEL must be one of: debug, info, warn, error")
	}

	validLogFormats := map[string]bool{"text": true, "json": true}
	if !validLogFormats[c.LogFormat] {
		return errors.New("LOG_FORMAT must be one of: text, json")
	}

	return nil
}

// getEnvString returns the environment variable value or a default.
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns the environment variable as an int or a default.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBool returns the environment variable as a bool or a default.
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// getEnvDuration returns the environment variable as a duration or a default.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
