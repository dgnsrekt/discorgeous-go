package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dgnsrekt/discorgeous-go/internal/api"
	"github.com/dgnsrekt/discorgeous-go/internal/audio"
	"github.com/dgnsrekt/discorgeous-go/internal/config"
	"github.com/dgnsrekt/discorgeous-go/internal/discord"
	"github.com/dgnsrekt/discorgeous-go/internal/logging"
	"github.com/dgnsrekt/discorgeous-go/internal/playback"
	"github.com/dgnsrekt/discorgeous-go/internal/queue"
	"github.com/dgnsrekt/discorgeous-go/internal/tts"
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

	// Warn if bearer token auth is disabled
	if cfg.AuthDisabled() {
		logger.Warn("HTTP bearer authentication is disabled (BEARER_TOKEN is empty)")
	}

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

	// Initialize TTS engine registry with Piper
	ttsRegistry := tts.NewRegistry()
	if cfg.PiperModel != "" {
		piperCfg := tts.PiperConfig{
			BinaryPath:   cfg.PiperPath,
			ModelPath:    cfg.PiperModel,
			DefaultVoice: cfg.DefaultVoice,
		}
		piperEngine, err := tts.NewPiperEngine(piperCfg, logger)
		if err != nil {
			logger.Warn("failed to initialize Piper TTS", "error", err)
		} else {
			if err := ttsRegistry.Register(piperEngine); err != nil {
				logger.Warn("failed to register Piper TTS", "error", err)
			} else {
				logger.Info("Piper TTS engine registered", "model", cfg.PiperModel)
			}
		}
	} else {
		logger.Warn("no Piper model configured, TTS will not work")
	}

	// Initialize audio converter
	audioConv, err := audio.NewConverter()
	if err != nil {
		logger.Warn("ffmpeg not available, audio conversion will fail", "error", err)
	}

	// Initialize Discord voice manager
	var voiceManager *discord.VoiceManager
	if cfg.DiscordToken != "" && cfg.GuildID != "" && cfg.DefaultVoiceChannelID != "" {
		voiceManager, err = discord.NewVoiceManager(
			cfg.DiscordToken,
			cfg.GuildID,
			cfg.DefaultVoiceChannelID,
			logger,
		)
		if err != nil {
			logger.Error("failed to create voice manager", "error", err)
			os.Exit(1)
		}

		if err := voiceManager.Open(); err != nil {
			logger.Error("failed to open Discord session", "error", err)
			os.Exit(1)
		}
		defer voiceManager.Close()
		logger.Info("Discord session opened")
	} else {
		logger.Warn("Discord credentials not configured, voice will not work")
	}

	// Create and start the speech queue
	speechQueue := queue.NewQueue(cfg.QueueCapacity, cfg.AutoLeaveIdle, logger)

	// Set idle callback to disconnect from voice
	speechQueue.SetIdleCallback(func() {
		logger.Info("queue idle, disconnecting from voice channel")
		if voiceManager != nil {
			if err := voiceManager.Disconnect(); err != nil {
				logger.Error("failed to disconnect from voice", "error", err)
			}
		}
	})

	// Set shutdown callback to disconnect from voice during graceful shutdown
	speechQueue.SetShutdownCallback(func() {
		logger.Info("shutdown: disconnecting from voice channel if connected")
		if voiceManager != nil && voiceManager.IsConnected() {
			if err := voiceManager.Disconnect(); err != nil {
				logger.Error("failed to disconnect from voice during shutdown", "error", err)
			} else {
				logger.Info("disconnected from voice channel during shutdown")
			}
		}
	})

	// Set playback handler
	defaultEngine, _ := ttsRegistry.Default()
	if voiceManager != nil && audioConv != nil && defaultEngine != nil {
		handler := playback.NewHandler(ttsRegistry, audioConv, voiceManager, logger)
		speechQueue.SetPlaybackHandler(handler.Handle)
		logger.Info("audio pipeline ready")
	} else {
		// Fallback handler for when not all components are available
		speechQueue.SetPlaybackHandler(func(ctx context.Context, job *queue.SpeakJob) error {
			logger.Info("would play speech (audio pipeline not configured)",
				"job_id", job.ID,
				"text", job.Text,
				"voice", job.Voice,
			)
			return nil
		})
	}

	speechQueue.Start()
	defer speechQueue.Stop()

	// Create and start HTTP server
	server := api.New(cfg, logger, speechQueue)

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
