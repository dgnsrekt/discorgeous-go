package relay

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Save and restore environment
	envVars := []string{
		"NTFY_SERVER", "NTFY_TOPICS", "DISCORGEOUS_API_URL", "DISCORGEOUS_BEARER_TOKEN",
		"NTFY_PREFIX", "NTFY_INTERRUPT", "NTFY_DEDUPE_WINDOW", "NTFY_MAX_TEXT_LENGTH",
		"LOG_LEVEL", "LOG_FORMAT",
	}
	saved := make(map[string]string)
	for _, k := range envVars {
		saved[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Clear all env vars
	for _, k := range envVars {
		os.Unsetenv(k)
	}

	tests := []struct {
		name      string
		envSetup  map[string]string
		wantErr   bool
		checkFunc func(*Config) bool
	}{
		{
			name:     "missing NTFY_TOPICS",
			envSetup: map[string]string{},
			wantErr:  true,
		},
		{
			name: "minimal valid config",
			envSetup: map[string]string{
				"NTFY_TOPICS": "test-topic",
			},
			wantErr: false,
			checkFunc: func(c *Config) bool {
				return c.NtfyServer == "https://ntfy.sh" &&
					len(c.NtfyTopics) == 1 &&
					c.NtfyTopics[0] == "test-topic" &&
					c.DiscorgeousAPIURL == "http://discorgeous:8080" &&
					c.MaxTextLength == 1000
			},
		},
		{
			name: "multiple topics",
			envSetup: map[string]string{
				"NTFY_TOPICS": "topic1, topic2, topic3",
			},
			wantErr: false,
			checkFunc: func(c *Config) bool {
				return len(c.NtfyTopics) == 3 &&
					c.NtfyTopics[0] == "topic1" &&
					c.NtfyTopics[1] == "topic2" &&
					c.NtfyTopics[2] == "topic3"
			},
		},
		{
			name: "full config",
			envSetup: map[string]string{
				"NTFY_SERVER":              "https://custom.ntfy.server",
				"NTFY_TOPICS":              "topic1",
				"DISCORGEOUS_API_URL":      "http://localhost:9090",
				"DISCORGEOUS_BEARER_TOKEN": "secret-token",
				"NTFY_PREFIX":              "Alert",
				"NTFY_INTERRUPT":           "true",
				"NTFY_DEDUPE_WINDOW":       "5m",
				"NTFY_MAX_TEXT_LENGTH":     "500",
				"LOG_LEVEL":                "debug",
				"LOG_FORMAT":               "json",
			},
			wantErr: false,
			checkFunc: func(c *Config) bool {
				return c.NtfyServer == "https://custom.ntfy.server" &&
					c.DiscorgeousAPIURL == "http://localhost:9090" &&
					c.DiscorgeousBearerToken == "secret-token" &&
					c.Prefix == "Alert" &&
					c.Interrupt == true &&
					c.DedupeWindow == 5*time.Minute &&
					c.MaxTextLength == 500 &&
					c.LogLevel == "debug" &&
					c.LogFormat == "json"
			},
		},
		{
			name: "invalid log level",
			envSetup: map[string]string{
				"NTFY_TOPICS": "topic1",
				"LOG_LEVEL":   "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid log format",
			envSetup: map[string]string{
				"NTFY_TOPICS": "topic1",
				"LOG_FORMAT":  "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid max text length",
			envSetup: map[string]string{
				"NTFY_TOPICS":          "topic1",
				"NTFY_MAX_TEXT_LENGTH": "0",
			},
			wantErr: true,
		},
		{
			name: "empty topics after trimming",
			envSetup: map[string]string{
				"NTFY_TOPICS": "  ,  ,  ",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			for _, k := range envVars {
				os.Unsetenv(k)
			}

			// Set test env vars
			for k, v := range tt.envSetup {
				os.Setenv(k, v)
			}

			cfg, err := Load()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Load() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Load() unexpected error: %v", err)
				return
			}

			if tt.checkFunc != nil && !tt.checkFunc(cfg) {
				t.Errorf("Load() config check failed, got: %+v", cfg)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				NtfyServer:        "https://ntfy.sh",
				NtfyTopics:        []string{"topic1"},
				DiscorgeousAPIURL: "http://localhost:8080",
				MaxTextLength:     1000,
				LogLevel:          "info",
				LogFormat:         "text",
			},
			wantErr: false,
		},
		{
			name: "empty topics",
			cfg: Config{
				NtfyServer:        "https://ntfy.sh",
				NtfyTopics:        []string{},
				DiscorgeousAPIURL: "http://localhost:8080",
				MaxTextLength:     1000,
				LogLevel:          "info",
				LogFormat:         "text",
			},
			wantErr: true,
		},
		{
			name: "empty server",
			cfg: Config{
				NtfyServer:        "",
				NtfyTopics:        []string{"topic1"},
				DiscorgeousAPIURL: "http://localhost:8080",
				MaxTextLength:     1000,
				LogLevel:          "info",
				LogFormat:         "text",
			},
			wantErr: true,
		},
		{
			name: "empty API URL",
			cfg: Config{
				NtfyServer:        "https://ntfy.sh",
				NtfyTopics:        []string{"topic1"},
				DiscorgeousAPIURL: "",
				MaxTextLength:     1000,
				LogLevel:          "info",
				LogFormat:         "text",
			},
			wantErr: true,
		},
		{
			name: "negative dedupe window",
			cfg: Config{
				NtfyServer:        "https://ntfy.sh",
				NtfyTopics:        []string{"topic1"},
				DiscorgeousAPIURL: "http://localhost:8080",
				MaxTextLength:     1000,
				DedupeWindow:      -1 * time.Second,
				LogLevel:          "info",
				LogFormat:         "text",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
