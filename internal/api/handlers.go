package api

import (
	"encoding/json"
	"net/http"
)

// SpeakRequest represents the request body for /v1/speak.
type SpeakRequest struct {
	Text      string `json:"text"`
	Voice     string `json:"voice,omitempty"`
	Interrupt bool   `json:"interrupt,omitempty"`
	TTLMS     int    `json:"ttl_ms,omitempty"`
	DedupeKey string `json:"dedupe_key,omitempty"`
}

// SpeakResponse represents the response body for /v1/speak.
type SpeakResponse struct {
	JobID   string `json:"job_id"`
	Message string `json:"message"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthResponse represents the response body for /v1/healthz.
type HealthResponse struct {
	Status string `json:"status"`
}

// handleHealthz handles GET /v1/healthz requests.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

// handleSpeak handles POST /v1/speak requests.
func (s *Server) handleSpeak(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req SpeakRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Warn("failed to decode speak request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid JSON body"})
		return
	}

	// Validate text is present
	if req.Text == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "text is required"})
		return
	}

	// Validate text length
	if len(req.Text) > s.cfg.MaxTextLength {
		s.logger.Warn("text exceeds max length", "length", len(req.Text), "max", s.cfg.MaxTextLength)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "text exceeds maximum length"})
		return
	}

	// Validate TTL if provided
	if req.TTLMS < 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "ttl_ms must be non-negative"})
		return
	}

	// Use default voice if not provided
	voice := req.Voice
	if voice == "" {
		voice = s.cfg.DefaultVoice
	}

	// TODO: In Task 3, this will enqueue to the actual queue.
	// For now, just log and return a placeholder response.
	s.logger.Info("speak request received",
		"text_length", len(req.Text),
		"voice", voice,
		"interrupt", req.Interrupt,
		"ttl_ms", req.TTLMS,
		"dedupe_key", req.DedupeKey,
	)

	// Return success response with placeholder job ID
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(SpeakResponse{
		JobID:   "placeholder-job-id",
		Message: "job enqueued",
	})
}
