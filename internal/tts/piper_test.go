package tts

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"testing"
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

func TestWrapRawPCMAsWAV(t *testing.T) {
	// Create fake PCM data
	pcmData := make([]byte, 100)
	for i := range pcmData {
		pcmData[i] = byte(i)
	}

	wavData := wrapRawPCMAsWAV(pcmData, 22050, 1, 16)

	// Check WAV header size
	if len(wavData) != 44+len(pcmData) {
		t.Errorf("expected %d bytes, got %d", 44+len(pcmData), len(wavData))
	}

	// Check RIFF header
	if !bytes.Equal(wavData[0:4], []byte("RIFF")) {
		t.Errorf("missing RIFF header")
	}

	// Check WAVE format
	if !bytes.Equal(wavData[8:12], []byte("WAVE")) {
		t.Errorf("missing WAVE format")
	}

	// Check fmt subchunk
	if !bytes.Equal(wavData[12:16], []byte("fmt ")) {
		t.Errorf("missing fmt subchunk")
	}

	// Check data subchunk
	if !bytes.Equal(wavData[36:40], []byte("data")) {
		t.Errorf("missing data subchunk")
	}

	// Check file size in header (36 + data size)
	fileSize := uint32(wavData[4]) | uint32(wavData[5])<<8 | uint32(wavData[6])<<16 | uint32(wavData[7])<<24
	if fileSize != uint32(36+len(pcmData)) {
		t.Errorf("expected file size %d, got %d", 36+len(pcmData), fileSize)
	}

	// Check data size in header
	dataSize := uint32(wavData[40]) | uint32(wavData[41])<<8 | uint32(wavData[42])<<16 | uint32(wavData[43])<<24
	if dataSize != uint32(len(pcmData)) {
		t.Errorf("expected data size %d, got %d", len(pcmData), dataSize)
	}

	// Check sample rate
	sampleRate := uint32(wavData[24]) | uint32(wavData[25])<<8 | uint32(wavData[26])<<16 | uint32(wavData[27])<<24
	if sampleRate != 22050 {
		t.Errorf("expected sample rate 22050, got %d", sampleRate)
	}

	// Check channels
	channels := uint16(wavData[22]) | uint16(wavData[23])<<8
	if channels != 1 {
		t.Errorf("expected 1 channel, got %d", channels)
	}

	// Check bits per sample
	bitsPerSample := uint16(wavData[34]) | uint16(wavData[35])<<8
	if bitsPerSample != 16 {
		t.Errorf("expected 16 bits per sample, got %d", bitsPerSample)
	}

	// Check PCM data is preserved
	if !bytes.Equal(wavData[44:], pcmData) {
		t.Errorf("PCM data not preserved correctly")
	}
}

func TestWrapRawPCMAsWAV_Stereo(t *testing.T) {
	pcmData := make([]byte, 200)
	wavData := wrapRawPCMAsWAV(pcmData, 44100, 2, 16)

	// Check channels
	channels := uint16(wavData[22]) | uint16(wavData[23])<<8
	if channels != 2 {
		t.Errorf("expected 2 channels, got %d", channels)
	}

	// Check sample rate
	sampleRate := uint32(wavData[24]) | uint32(wavData[25])<<8 | uint32(wavData[26])<<16 | uint32(wavData[27])<<24
	if sampleRate != 44100 {
		t.Errorf("expected sample rate 44100, got %d", sampleRate)
	}

	// Check byte rate (sample_rate * channels * bits_per_sample / 8)
	expectedByteRate := uint32(44100 * 2 * 16 / 8)
	byteRate := uint32(wavData[28]) | uint32(wavData[29])<<8 | uint32(wavData[30])<<16 | uint32(wavData[31])<<24
	if byteRate != expectedByteRate {
		t.Errorf("expected byte rate %d, got %d", expectedByteRate, byteRate)
	}

	// Check block align (channels * bits_per_sample / 8)
	expectedBlockAlign := uint16(2 * 16 / 8)
	blockAlign := uint16(wavData[32]) | uint16(wavData[33])<<8
	if blockAlign != expectedBlockAlign {
		t.Errorf("expected block align %d, got %d", expectedBlockAlign, blockAlign)
	}
}
