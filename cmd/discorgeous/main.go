package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dgnsrekt/discorgeous-go/internal/api"
	"github.com/dgnsrekt/discorgeous-go/internal/config"
	"github.com/dgnsrekt/discorgeous-go/internal/logging"
)

func main() {
	// Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		// Use stderr before logger is initialized
		os.Stderr.WriteString("failed to load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	// Initialize structured logger
	logger := logging.New(cfg.LogLevel, cfg.LogFormat)
	logger.Info("starting discorgeous", "version", "0.1.0")

	// Log loaded configuration (without sensitive values)
	logger.Info("configuration loaded",
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
		"http_port", cfg.HTTPPort,
		"auto_leave_idle", cfg.AutoLeaveIdle,
		"max_text_length", cfg.MaxTextLength,
		"queue_capacity", cfg.QueueCapacity,
	)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", "signal", sig.String())
		cancel()
	}()

	// Create and start HTTP server
	server := api.New(cfg, logger)

	go func() {
		if err := server.Start(); err != nil {
			logger.Error("HTTP server error", "error", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown HTTP server", "error", err)
	}

	logger.Info("shutdown complete")
}
