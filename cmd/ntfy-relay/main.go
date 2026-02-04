package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/dgnsrekt/discorgeous-go/internal/logging"
	"github.com/dgnsrekt/discorgeous-go/internal/relay"
)

func main() {
	// Load configuration from environment
	cfg, err := relay.Load()
	if err != nil {
		os.Stderr.WriteString("failed to load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	// Initialize structured logger
	logger := logging.New(cfg.LogLevel, cfg.LogFormat)
	logger.Info("starting ntfy-relay", "version", "0.1.0")

	// Warn if bearer token is not set
	if cfg.DiscorgeousBearerToken == "" {
		logger.Warn("DISCORGEOUS_BEARER_TOKEN is not set, requests may fail if Discorgeous requires auth")
	}

	// Log loaded configuration (without sensitive values)
	logger.Info("configuration loaded",
		"ntfy_server", cfg.NtfyServer,
		"ntfy_topics", cfg.NtfyTopics,
		"discorgeous_api_url", cfg.DiscorgeousAPIURL,
		"prefix", cfg.Prefix,
		"interrupt", cfg.Interrupt,
		"dedupe_window", cfg.DedupeWindow,
		"max_text_length", cfg.MaxTextLength,
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

	// Create and run the relay client
	client := relay.NewClient(cfg, logger)

	logger.Info("starting relay client")
	if err := client.Run(ctx); err != nil {
		logger.Error("relay client error", "error", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
