package relay

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestFormatText(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		title   string
		message string
		want    string
	}{
		{
			name: "message only",
			cfg: &Config{
				MaxTextLength: 1000,
			},
			title:   "",
			message: "Hello world",
			want:    "Hello world",
		},
		{
			name: "title only",
			cfg: &Config{
				MaxTextLength: 1000,
			},
			title:   "Alert",
			message: "",
			want:    "Alert",
		},
		{
			name: "title and message",
			cfg: &Config{
				MaxTextLength: 1000,
			},
			title:   "Alert",
			message: "Something happened",
			want:    "Alert: Something happened",
		},
		{
			name: "with prefix",
			cfg: &Config{
				Prefix:        "NTFY",
				MaxTextLength: 1000,
			},
			title:   "Alert",
			message: "Something happened",
			want:    "NTFY: Alert: Something happened",
		},
		{
			name: "prefix only, no title or message",
			cfg: &Config{
				Prefix:        "NTFY",
				MaxTextLength: 1000,
			},
			title:   "",
			message: "",
			want:    "NTFY",
		},
		{
			name: "truncate at max length",
			cfg: &Config{
				MaxTextLength: 10,
			},
			title:   "",
			message: "This is a very long message that should be truncated",
			want:    "This is a ",
		},
		{
			name: "empty everything",
			cfg: &Config{
				MaxTextLength: 1000,
			},
			title:   "",
			message: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.cfg, newTestLogger())
			got := client.FormatText(tt.title, tt.message)
			if got != tt.want {
				t.Errorf("FormatText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeduplication(t *testing.T) {
	cfg := &Config{
		NtfyServer:        "https://ntfy.sh",
		NtfyTopics:        []string{"test"},
		DiscorgeousAPIURL: "http://localhost:8080",
		MaxTextLength:     1000,
		DedupeWindow:      100 * time.Millisecond,
	}

	client := NewClient(cfg, newTestLogger())

	// Generate dedupe key
	key := client.generateDedupeKey("test message")
	if key == "" {
		t.Fatal("generateDedupeKey returned empty string")
	}

	// First check should not be duplicate
	if client.isDuplicate(key) {
		t.Error("isDuplicate() should return false for new key")
	}

	// Record the key
	client.recordDedupeKey(key)

	// Now it should be duplicate
	if !client.isDuplicate(key) {
		t.Error("isDuplicate() should return true for recorded key within window")
	}

	// Wait for dedupe window to expire
	time.Sleep(150 * time.Millisecond)

	// Should no longer be duplicate
	if client.isDuplicate(key) {
		t.Error("isDuplicate() should return false after dedupe window expires")
	}
}

func TestDedupeCleanup(t *testing.T) {
	cfg := &Config{
		NtfyServer:        "https://ntfy.sh",
		NtfyTopics:        []string{"test"},
		DiscorgeousAPIURL: "http://localhost:8080",
		MaxTextLength:     1000,
		DedupeWindow:      50 * time.Millisecond,
	}

	client := NewClient(cfg, newTestLogger())

	// Add some keys
	client.recordDedupeKey("key1")
	client.recordDedupeKey("key2")

	if len(client.dedupeMap) != 2 {
		t.Errorf("expected 2 keys in dedupeMap, got %d", len(client.dedupeMap))
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	client.cleanupDedupeMap()

	if len(client.dedupeMap) != 0 {
		t.Errorf("expected 0 keys after cleanup, got %d", len(client.dedupeMap))
	}
}

func TestForwardToDiscorgeous(t *testing.T) {
	var mu sync.Mutex
	var receivedReq SpeakRequest
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.URL.Path != "/v1/speak" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		receivedAuth = r.Header.Get("Authorization")

		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"job_id":  "test-job-123",
			"message": "job enqueued",
		})
	}))
	defer server.Close()

	cfg := &Config{
		NtfyServer:             "https://ntfy.sh",
		NtfyTopics:             []string{"test"},
		DiscorgeousAPIURL:      server.URL,
		DiscorgeousBearerToken: "test-token",
		MaxTextLength:          1000,
		Interrupt:              true,
	}

	client := NewClient(cfg, newTestLogger())

	err := client.forwardToDiscorgeous("Hello world", "dedupe-123")
	if err != nil {
		t.Errorf("forwardToDiscorgeous() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedReq.Text != "Hello world" {
		t.Errorf("expected text 'Hello world', got %q", receivedReq.Text)
	}

	if !receivedReq.Interrupt {
		t.Error("expected interrupt to be true")
	}

	if receivedReq.DedupeKey != "dedupe-123" {
		t.Errorf("expected dedupe_key 'dedupe-123', got %q", receivedReq.DedupeKey)
	}

	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected auth 'Bearer test-token', got %q", receivedAuth)
	}
}

func TestForwardToDiscorgeousNoAuth(t *testing.T) {
	var mu sync.Mutex
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedAuth = r.Header.Get("Authorization")
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"job_id":  "test-job-123",
			"message": "job enqueued",
		})
	}))
	defer server.Close()

	cfg := &Config{
		NtfyServer:             "https://ntfy.sh",
		NtfyTopics:             []string{"test"},
		DiscorgeousAPIURL:      server.URL,
		DiscorgeousBearerToken: "", // No token
		MaxTextLength:          1000,
	}

	client := NewClient(cfg, newTestLogger())

	err := client.forwardToDiscorgeous("Test message", "")
	if err != nil {
		t.Errorf("forwardToDiscorgeous() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedAuth != "" {
		t.Errorf("expected no auth header, got %q", receivedAuth)
	}
}

func TestForwardToDiscorgeousError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "internal server error",
		})
	}))
	defer server.Close()

	cfg := &Config{
		NtfyServer:        "https://ntfy.sh",
		NtfyTopics:        []string{"test"},
		DiscorgeousAPIURL: server.URL,
		MaxTextLength:     1000,
	}

	client := NewClient(cfg, newTestLogger())

	err := client.forwardToDiscorgeous("Test message", "")
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got: %v", err)
	}
}

func TestHandleMessage(t *testing.T) {
	var mu sync.Mutex
	var receivedReqs []SpeakRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		var req SpeakRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedReqs = append(receivedReqs, req)

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"job_id":  "test-job",
			"message": "job enqueued",
		})
	}))
	defer server.Close()

	cfg := &Config{
		NtfyServer:        "https://ntfy.sh",
		NtfyTopics:        []string{"test"},
		DiscorgeousAPIURL: server.URL,
		MaxTextLength:     1000,
		Prefix:            "Alert",
	}

	client := NewClient(cfg, newTestLogger())

	// Test with title and message
	client.handleMessage(NtfyMessage{
		ID:      "msg1",
		Event:   "message",
		Topic:   "test",
		Title:   "Server Down",
		Message: "Database connection lost",
	})

	mu.Lock()
	if len(receivedReqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(receivedReqs))
	}
	if receivedReqs[0].Text != "Alert: Server Down: Database connection lost" {
		t.Errorf("unexpected text: %q", receivedReqs[0].Text)
	}
	mu.Unlock()

	// Test with empty message (should not forward)
	client.handleMessage(NtfyMessage{
		ID:      "msg2",
		Event:   "message",
		Topic:   "test",
		Title:   "",
		Message: "",
	})

	// Wait a bit to ensure no request was made for empty message
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	// Should still only have 2 requests (one from message with prefix "Alert")
	// Wait, the prefix is "Alert" so even with empty title and message, it would send "Alert"
	// Let me check - the FormatText function would return just "Alert" for prefix only
	if len(receivedReqs) != 2 {
		t.Errorf("expected 2 requests (prefix-only also sends), got %d", len(receivedReqs))
	}
	mu.Unlock()
}

func TestRunCancellation(t *testing.T) {
	// Test that Run respects context cancellation
	cfg := &Config{
		NtfyServer:        "https://ntfy.sh",
		NtfyTopics:        []string{"test-topic"},
		DiscorgeousAPIURL: "http://localhost:8080",
		MaxTextLength:     1000,
	}

	client := NewClient(cfg, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		client.Run(ctx)
		close(done)
	}()

	// Cancel immediately
	cancel()

	// Should complete quickly
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Run did not exit after context cancellation")
	}
}
