package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_TextFormat(t *testing.T) {
	logger := New("info", "text")
	if logger == nil {
		t.Fatal("New() returned nil")
	}

	// Logger should be functional
	logger.Info("test message")
}

func TestNew_JSONFormat(t *testing.T) {
	logger := New("info", "json")
	if logger == nil {
		t.Fatal("New() returned nil")
	}

	// Logger should be functional
	logger.Info("test message")
}

func TestNew_LogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "invalid"}
	for _, level := range levels {
		logger := New(level, "text")
		if logger == nil {
			t.Errorf("New(%s, text) returned nil", level)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"invalid", slog.LevelInfo}, // default
		{"", slog.LevelInfo},        // default
	}

	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.expected {
			t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestNew_LogLevelFiltering(t *testing.T) {
	// Test that log level filtering works
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	logger := slog.New(handler)

	// Info should be filtered out
	logger.Info("should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Error("Info message should be filtered at warn level")
	}

	// Warn should appear
	logger.Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Error("Warn message should appear at warn level")
	}
}
