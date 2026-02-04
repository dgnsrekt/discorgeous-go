package tts

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"testing"

	"github.com/dgnsrekt/discorgeous-go/internal/wav"
)

func TestPiperEngine_Name(t *testing.T) {
	// Skip actual creation if piper isn't available
	engine := &PiperEngine{
		config: PiperConfig{
			BinaryPath: "piper",
			ModelPath:  "/fake/model.onnx",
		},
	}

	if engine.Name() != "piper" {
		t.Errorf("expected name 'piper', got '%s'", engine.Name())
	}
}

func TestNewPiperEngine_NoModel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a fake piper binary for the test
	_, err := NewPiperEngine(PiperConfig{
		BinaryPath: "echo", // Use echo as a stand-in
		ModelPath:  "",     // No model
	}, logger)

	if !errors.Is(err, ErrNoModelSpecified) {
		t.Errorf("expected ErrNoModelSpecified, got %v", err)
	}
}

func TestNewPiperEngine_BinaryNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	_, err := NewPiperEngine(PiperConfig{
		BinaryPath: "/nonexistent/path/to/piper",
		ModelPath:  "/fake/model.onnx",
	}, logger)

	if !errors.Is(err, ErrPiperNotFound) {
		t.Errorf("expected ErrPiperNotFound, got %v", err)
	}
}

func TestPiperEngine_Synthesize_EmptyText(t *testing.T) {
	engine := &PiperEngine{
		config: PiperConfig{
			BinaryPath: "echo",
			ModelPath:  "/fake/model.onnx",
		},
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	_, err := engine.Synthesize(context.Background(), SynthesizeRequest{Text: ""})
	if err == nil || err.Error() != "empty text" {
		t.Errorf("expected 'empty text' error, got %v", err)
	}
}

func TestPiperEngine_Synthesize_Cancelled(t *testing.T) {
	// Skip if piper isn't available for real tests
	if _, err := exec.LookPath("piper"); err != nil {
		t.Skip("piper binary not available")
	}

	engine := &PiperEngine{
		config: PiperConfig{
			BinaryPath: "piper",
			ModelPath:  "/fake/model.onnx",
		},
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := engine.Synthesize(ctx, SynthesizeRequest{Text: "test"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestPiperUsesWavPackage(t *testing.T) {
	// Verify Piper uses the correct wav package constants
	if wav.PiperSampleRate != 22050 {
		t.Errorf("PiperSampleRate = %d, want 22050", wav.PiperSampleRate)
	}
	if wav.PiperChannels != 1 {
		t.Errorf("PiperChannels = %d, want 1", wav.PiperChannels)
	}
	if wav.PiperBitsPerSample != 16 {
		t.Errorf("PiperBitsPerSample = %d, want 16", wav.PiperBitsPerSample)
	}
}
