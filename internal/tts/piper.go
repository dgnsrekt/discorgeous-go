package tts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
)

var (
	// ErrPiperNotFound is returned when the piper binary is not found.
	ErrPiperNotFound = errors.New("piper binary not found")
	// ErrNoModelSpecified is returned when no model is configured.
	ErrNoModelSpecified = errors.New("no piper model specified")
	// ErrSynthesisFailed is returned when TTS synthesis fails.
	ErrSynthesisFailed = errors.New("TTS synthesis failed")
)

// PiperConfig holds configuration for the Piper TTS engine.
type PiperConfig struct {
	// BinaryPath is the path to the piper executable.
	BinaryPath string
	// ModelPath is the path to the ONNX model file.
	ModelPath string
	// DefaultVoice is the default voice/speaker to use.
	DefaultVoice string
}

// PiperEngine implements the Engine interface using local Piper TTS.
type PiperEngine struct {
	config PiperConfig
	logger *slog.Logger
}

// NewPiperEngine creates a new Piper TTS engine.
func NewPiperEngine(cfg PiperConfig, logger *slog.Logger) (*PiperEngine, error) {
	if cfg.BinaryPath == "" {
		cfg.BinaryPath = "piper"
	}

	// Verify piper binary exists
	if _, err := exec.LookPath(cfg.BinaryPath); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrPiperNotFound, cfg.BinaryPath)
	}

	if cfg.ModelPath == "" {
		return nil, ErrNoModelSpecified
	}

	return &PiperEngine{
		config: cfg,
		logger: logger,
	}, nil
}

// Name returns the engine identifier.
func (p *PiperEngine) Name() string {
	return "piper"
}

// Synthesize converts text to audio using Piper.
func (p *PiperEngine) Synthesize(ctx context.Context, req SynthesizeRequest) (*AudioResult, error) {
	if req.Text == "" {
		return nil, errors.New("empty text")
	}

	// Build piper command arguments
	args := []string{
		"--model", p.config.ModelPath,
		"--output-raw",
	}

	// Add voice/speaker if specified
	voice := req.Voice
	if voice == "" || voice == "default" {
		voice = p.config.DefaultVoice
	}
	if voice != "" && voice != "default" {
		args = append(args, "--speaker", voice)
	}

	p.logger.Debug("running piper",
		"binary", p.config.BinaryPath,
		"model", p.config.ModelPath,
		"voice", voice,
		"text_length", len(req.Text),
	)

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, p.config.BinaryPath, args...)

	// Set up stdin with the text
	cmd.Stdin = bytes.NewReader([]byte(req.Text))

	// Capture stdout (raw audio) and stderr (logs/errors)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p.logger.Error("piper failed",
			"error", err,
			"stderr", stderr.String(),
		)
		return nil, fmt.Errorf("%w: %v", ErrSynthesisFailed, err)
	}

	rawAudio := stdout.Bytes()
	if len(rawAudio) == 0 {
		return nil, fmt.Errorf("%w: no audio output", ErrSynthesisFailed)
	}

	p.logger.Debug("piper synthesis complete",
		"output_bytes", len(rawAudio),
	)

	// Piper outputs raw 16-bit PCM at 22050 Hz mono by default
	// We'll wrap it in a WAV header for consistency
	wavData := wrapRawPCMAsWAV(rawAudio, 22050, 1, 16)

	return &AudioResult{
		Data:       wavData,
		Format:     "wav",
		SampleRate: 22050,
		Channels:   1,
	}, nil
}

// wrapRawPCMAsWAV adds a WAV header to raw PCM data.
func wrapRawPCMAsWAV(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	dataSize := len(pcm)
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8

	// WAV header is 44 bytes
	header := make([]byte, 44)

	// RIFF header
	copy(header[0:4], "RIFF")
	putLE32(header[4:8], uint32(36+dataSize))
	copy(header[8:12], "WAVE")

	// fmt subchunk
	copy(header[12:16], "fmt ")
	putLE32(header[16:20], 16) // subchunk size
	putLE16(header[20:22], 1)  // audio format (PCM)
	putLE16(header[22:24], uint16(channels))
	putLE32(header[24:28], uint32(sampleRate))
	putLE32(header[28:32], uint32(byteRate))
	putLE16(header[32:34], uint16(blockAlign))
	putLE16(header[34:36], uint16(bitsPerSample))

	// data subchunk
	copy(header[36:40], "data")
	putLE32(header[40:44], uint32(dataSize))

	return append(header, pcm...)
}

func putLE16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func putLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
