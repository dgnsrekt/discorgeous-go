package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear relevant env vars to test defaults
	envVars := []string{
		"DISCORD_TOKEN", "GUILD_ID", "DEFAULT_VOICE_CHANNEL_ID",
		"HTTP_PORT", "BEARER_TOKEN", "PIPER_PATH", "PIPER_MODEL",
		"DEFAULT_VOICE", "AUTO_LEAVE_IDLE", "MAX_TEXT_LENGTH",
		"QUEUE_CAPACITY", "DEFAULT_TTL", "LOG_LEVEL", "LOG_FORMAT",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.PiperPath != "piper" {
		t.Errorf("PiperPath = %s, want piper", cfg.PiperPath)
	}
	if cfg.DefaultVoice != "default" {
		t.Errorf("DefaultVoice = %s, want default", cfg.DefaultVoice)
	}
	if cfg.AutoLeaveIdle != 5*time.Minute {
		t.Errorf("AutoLeaveIdle = %v, want 5m", cfg.AutoLeaveIdle)
	}
	if cfg.MaxTextLength != 1000 {
		t.Errorf("MaxTextLength = %d, want 1000", cfg.MaxTextLength)
	}
	if cfg.QueueCapacity != 100 {
		t.Errorf("QueueCapacity = %d, want 100", cfg.QueueCapacity)
	}
	if cfg.DefaultTTL != 30*time.Second {
		t.Errorf("DefaultTTL = %v, want 30s", cfg.DefaultTTL)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %s, want info", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %s, want text", cfg.LogFormat)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	// Set env vars
	os.Setenv("DISCORD_TOKEN", "test-token")
	os.Setenv("GUILD_ID", "123456")
	os.Setenv("DEFAULT_VOICE_CHANNEL_ID", "789012")
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("BEARER_TOKEN", "secret")
	os.Setenv("AUTO_LEAVE_IDLE", "10m")
	os.Setenv("MAX_TEXT_LENGTH", "500")
	os.Setenv("QUEUE_CAPACITY", "50")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "json")

	defer func() {
		os.Unsetenv("DISCORD_TOKEN")
		os.Unsetenv("GUILD_ID")
		os.Unsetenv("DEFAULT_VOICE_CHANNEL_ID")
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("BEARER_TOKEN")
		os.Unsetenv("AUTO_LEAVE_IDLE")
		os.Unsetenv("MAX_TEXT_LENGTH")
		os.Unsetenv("QUEUE_CAPACITY")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LOG_FORMAT")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DiscordToken != "test-token" {
		t.Errorf("DiscordToken = %s, want test-token", cfg.DiscordToken)
	}
	if cfg.GuildID != "123456" {
		t.Errorf("GuildID = %s, want 123456", cfg.GuildID)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d, want 9090", cfg.HTTPPort)
	}
	if cfg.AutoLeaveIdle != 10*time.Minute {
		t.Errorf("AutoLeaveIdle = %v, want 10m", cfg.AutoLeaveIdle)
	}
	if cfg.MaxTextLength != 500 {
		t.Errorf("MaxTextLength = %d, want 500", cfg.MaxTextLength)
	}
	if cfg.QueueCapacity != 50 {
		t.Errorf("QueueCapacity = %d, want 50", cfg.QueueCapacity)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %s, want debug", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %s, want json", cfg.LogFormat)
	}
}

func TestValidate_InvalidHTTPPort(t *testing.T) {
	cfg := &Config{
		HTTPPort:      0,
		MaxTextLength: 1000,
		QueueCapacity: 100,
		LogLevel:      "info",
		LogFormat:     "text",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid HTTP port")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &Config{
		HTTPPort:      8080,
		MaxTextLength: 1000,
		QueueCapacity: 100,
		LogLevel:      "invalid",
		LogFormat:     "text",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid log level")
	}
}

func TestValidate_InvalidLogFormat(t *testing.T) {
	cfg := &Config{
		HTTPPort:      8080,
		MaxTextLength: 1000,
		QueueCapacity: 100,
		LogLevel:      "info",
		LogFormat:     "invalid",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid log format")
	}
}

func TestValidate_InvalidMaxTextLength(t *testing.T) {
	cfg := &Config{
		HTTPPort:      8080,
		MaxTextLength: 0,
		QueueCapacity: 100,
		LogLevel:      "info",
		LogFormat:     "text",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid max text length")
	}
}

func TestValidate_InvalidQueueCapacity(t *testing.T) {
	cfg := &Config{
		HTTPPort:      8080,
		MaxTextLength: 1000,
		QueueCapacity: 0,
		LogLevel:      "info",
		LogFormat:     "text",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid queue capacity")
	}
}

func TestGetEnvString(t *testing.T) {
	os.Setenv("TEST_STRING", "value")
	defer os.Unsetenv("TEST_STRING")

	if got := getEnvString("TEST_STRING", "default"); got != "value" {
		t.Errorf("getEnvString() = %s, want value", got)
	}

	if got := getEnvString("NONEXISTENT", "default"); got != "default" {
		t.Errorf("getEnvString() = %s, want default", got)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	if got := getEnvInt("TEST_INT", 0); got != 42 {
		t.Errorf("getEnvInt() = %d, want 42", got)
	}

	if got := getEnvInt("NONEXISTENT", 10); got != 10 {
		t.Errorf("getEnvInt() = %d, want 10", got)
	}

	os.Setenv("TEST_INT_INVALID", "not-a-number")
	defer os.Unsetenv("TEST_INT_INVALID")

	if got := getEnvInt("TEST_INT_INVALID", 10); got != 10 {
		t.Errorf("getEnvInt() = %d, want 10 for invalid input", got)
	}
}

func TestGetEnvDuration(t *testing.T) {
	os.Setenv("TEST_DURATION", "5m")
	defer os.Unsetenv("TEST_DURATION")

	if got := getEnvDuration("TEST_DURATION", time.Second); got != 5*time.Minute {
		t.Errorf("getEnvDuration() = %v, want 5m", got)
	}

	if got := getEnvDuration("NONEXISTENT", 10*time.Second); got != 10*time.Second {
		t.Errorf("getEnvDuration() = %v, want 10s", got)
	}

	os.Setenv("TEST_DURATION_INVALID", "not-a-duration")
	defer os.Unsetenv("TEST_DURATION_INVALID")

	if got := getEnvDuration("TEST_DURATION_INVALID", 10*time.Second); got != 10*time.Second {
		t.Errorf("getEnvDuration() = %v, want 10s for invalid input", got)
	}
}
