package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dgnsrekt/discorgeous-go/internal/config"
	"github.com/dgnsrekt/discorgeous-go/internal/queue"
)

// Server handles HTTP API requests.
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	server *http.Server
	queue  *queue.Queue
}

// New creates a new API server.
func New(cfg *config.Config, logger *slog.Logger, q *queue.Queue) *Server {
	s := &Server{
		cfg:    cfg,
		logger: logger,
		queue:  q,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", s.handleHealthz)
	mux.HandleFunc("POST /v1/speak", s.withAuth(s.handleSpeak))

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	return s.server.Shutdown(ctx)
}
