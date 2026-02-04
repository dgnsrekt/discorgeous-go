package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	// Discord settings
	DiscordToken          string
	GuildID               string
	DefaultVoiceChannelID string

	// HTTP settings
	HTTPPort    int
	BearerToken string

	// TTS settings
	PiperPath    string
	PiperModel   string
	DefaultVoice string

	// Behavior settings
	AutoLeaveIdle time.Duration
	MaxTextLength int
	QueueCapacity int
	DefaultTTL    time.Duration

	// Logging settings
	LogLevel  string
	LogFormat string
}

// Load reads configuration from environment variables with sane defaults.
func Load() (*Config, error) {
	cfg := &Config{
		// Discord settings (required)
		DiscordToken:          os.Getenv("DISCORD_TOKEN"),
		GuildID:               os.Getenv("GUILD_ID"),
		DefaultVoiceChannelID: os.Getenv("DEFAULT_VOICE_CHANNEL_ID"),

		// HTTP settings
		HTTPPort:    getEnvInt("HTTP_PORT", 8080),
		BearerToken: os.Getenv("BEARER_TOKEN"),

		// TTS settings
		PiperPath:    getEnvString("PIPER_PATH", "piper"),
		PiperModel:   getEnvString("PIPER_MODEL", ""),
		DefaultVoice: getEnvString("DEFAULT_VOICE", "default"),

		// Behavior settings
		AutoLeaveIdle: getEnvDuration("AUTO_LEAVE_IDLE", 5*time.Minute),
		MaxTextLength: getEnvInt("MAX_TEXT_LENGTH", 1000),
		QueueCapacity: getEnvInt("QUEUE_CAPACITY", 100),
		DefaultTTL:    getEnvDuration("DEFAULT_TTL", 30*time.Second),

		// Logging settings
		LogLevel:  getEnvString("LOG_LEVEL", "info"),
		LogFormat: getEnvString("LOG_FORMAT", "text"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// AuthDisabled returns true if bearer token authentication is disabled.
func (c *Config) AuthDisabled() bool {
	return c.BearerToken == ""
}

// Validate checks that required configuration values are set.
func (c *Config) Validate() error {
	// For initial scaffold, we don't require Discord settings
	// They will be required when Discord integration is added

	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		return errors.New("HTTP_PORT must be between 1 and 65535")
	}

	if c.MaxTextLength < 1 {
		return errors.New("MAX_TEXT_LENGTH must be at least 1")
	}

	if c.QueueCapacity < 1 {
		return errors.New("QUEUE_CAPACITY must be at least 1")
	}

	if c.AutoLeaveIdle < 0 {
		return errors.New("AUTO_LEAVE_IDLE must be non-negative")
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

// getEnvDuration returns the environment variable as a duration or a default.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
