package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dgnsrekt/discorgeous-go/internal/config"
	"github.com/dgnsrekt/discorgeous-go/internal/logging"
)

func testConfig() *config.Config {
	return &config.Config{
		HTTPPort:      8080,
		BearerToken:   "test-token",
		MaxTextLength: 100,
		QueueCapacity: 10,
		DefaultVoice:  "default",
		LogLevel:      "info",
		LogFormat:     "text",
	}
}

func testServer(cfg *config.Config) *Server {
	logger := logging.New("error", "text") // quiet logger for tests
	return New(cfg, logger)
}

func TestHealthz(t *testing.T) {
	cfg := testConfig()
	srv := testServer(cfg)

	req := httptest.NewRequest("GET", "/v1/healthz", nil)
	w := httptest.NewRecorder()

	srv.handleHealthz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
}

func TestSpeakSuccess(t *testing.T) {
	cfg := testConfig()
	srv := testServer(cfg)

	body := `{"text":"Hello, world!"}`
	req := httptest.NewRequest("POST", "/v1/speak", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	// Manually call withAuth wrapper
	handler := srv.withAuth(srv.handleSpeak)
	handler(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}

	var resp SpeakResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.JobID == "" {
		t.Error("expected non-empty job_id")
	}
}

func TestSpeakMissingText(t *testing.T) {
	cfg := testConfig()
	srv := testServer(cfg)

	body := `{}`
	req := httptest.NewRequest("POST", "/v1/speak", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	handler := srv.withAuth(srv.handleSpeak)
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "text is required" {
		t.Errorf("expected error 'text is required', got '%s'", resp.Error)
	}
}

func TestSpeakTextTooLong(t *testing.T) {
	cfg := testConfig()
	cfg.MaxTextLength = 10
	srv := testServer(cfg)

	body := `{"text":"This text is definitely longer than 10 characters"}`
	req := httptest.NewRequest("POST", "/v1/speak", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	handler := srv.withAuth(srv.handleSpeak)
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "text exceeds maximum length" {
		t.Errorf("expected error 'text exceeds maximum length', got '%s'", resp.Error)
	}
}

func TestSpeakInvalidJSON(t *testing.T) {
	cfg := testConfig()
	srv := testServer(cfg)

	body := `{invalid json}`
	req := httptest.NewRequest("POST", "/v1/speak", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	handler := srv.withAuth(srv.handleSpeak)
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "invalid JSON body" {
		t.Errorf("expected error 'invalid JSON body', got '%s'", resp.Error)
	}
}

func TestSpeakNegativeTTL(t *testing.T) {
	cfg := testConfig()
	srv := testServer(cfg)

	body := `{"text":"Hello","ttl_ms":-100}`
	req := httptest.NewRequest("POST", "/v1/speak", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	handler := srv.withAuth(srv.handleSpeak)
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "ttl_ms must be non-negative" {
		t.Errorf("expected error 'ttl_ms must be non-negative', got '%s'", resp.Error)
	}
}

func TestSpeakWithOptionalFields(t *testing.T) {
	cfg := testConfig()
	srv := testServer(cfg)

	body := `{"text":"Hello","voice":"custom","interrupt":true,"ttl_ms":5000,"dedupe_key":"key123"}`
	req := httptest.NewRequest("POST", "/v1/speak", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	handler := srv.withAuth(srv.handleSpeak)
	handler(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}
}
