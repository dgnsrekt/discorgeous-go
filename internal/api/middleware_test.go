package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	cfg := testConfig()
	cfg.BearerToken = "secret-token"
	srv := testServer(cfg)

	called := false
	handler := srv.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if called {
		t.Error("handler should not have been called without auth")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "missing authorization header" {
		t.Errorf("expected error 'missing authorization header', got '%s'", resp.Error)
	}
}

func TestAuthMiddlewareInvalidFormat(t *testing.T) {
	cfg := testConfig()
	cfg.BearerToken = "secret-token"
	srv := testServer(cfg)

	called := false
	handler := srv.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Basic auth format
	w := httptest.NewRecorder()

	handler(w, req)

	if called {
		t.Error("handler should not have been called with invalid auth format")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	cfg := testConfig()
	cfg.BearerToken = "secret-token"
	srv := testServer(cfg)

	called := false
	handler := srv.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler(w, req)

	if called {
		t.Error("handler should not have been called with invalid token")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "invalid token" {
		t.Errorf("expected error 'invalid token', got '%s'", resp.Error)
	}
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	cfg := testConfig()
	cfg.BearerToken = "secret-token"
	srv := testServer(cfg)

	called := false
	handler := srv.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("handler should have been called with valid token")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthMiddlewareNoBearerConfigured(t *testing.T) {
	cfg := testConfig()
	cfg.BearerToken = "" // No token configured
	srv := testServer(cfg)

	called := false
	handler := srv.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No authorization header
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("handler should have been called when no bearer token is configured")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthMiddlewareCaseInsensitiveBearer(t *testing.T) {
	cfg := testConfig()
	cfg.BearerToken = "secret-token"
	srv := testServer(cfg)

	called := false
	handler := srv.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "bearer secret-token") // lowercase 'bearer'
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("handler should have been called with lowercase 'bearer'")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}
