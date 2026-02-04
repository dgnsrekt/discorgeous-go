package audio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

const (
	// DiscordSampleRate is the required sample rate for Discord voice.
	DiscordSampleRate = 48000
	// DiscordChannels is the required number of channels for Discord voice.
	DiscordChannels = 2
	// DiscordFrameSize is the number of samples per frame (20ms at 48kHz).
	DiscordFrameSize = 960
	// DiscordFrameBytes is the size of one frame in bytes (stereo 16-bit).
	DiscordFrameBytes = DiscordFrameSize * DiscordChannels * 2
)

var (
	// ErrFFmpegNotFound is returned when ffmpeg is not installed.
	ErrFFmpegNotFound = errors.New("ffmpeg not found in PATH")
	// ErrConversionFailed is returned when ffmpeg conversion fails.
	ErrConversionFailed = errors.New("audio conversion failed")
)

// Converter handles audio format conversion for Discord.
type Converter struct {
	ffmpegPath string
}

// NewConverter creates a new audio converter.
func NewConverter() (*Converter, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, ErrFFmpegNotFound
	}
	return &Converter{ffmpegPath: path}, nil
}

// NewConverterWithPath creates a converter with a specific ffmpeg path.
func NewConverterWithPath(path string) *Converter {
	return &Converter{ffmpegPath: path}
}

// ConvertToDiscordPCM converts WAV audio to Discord-ready 48kHz stereo 16-bit PCM.
// Input: WAV file bytes (any sample rate, mono or stereo)
// Output: Raw PCM bytes (48kHz, stereo, 16-bit signed little-endian)
func (c *Converter) ConvertToDiscordPCM(ctx context.Context, wavData []byte) ([]byte, error) {
	if len(wavData) == 0 {
		return nil, errors.New("empty input data")
	}

	// ffmpeg command to convert any WAV to Discord format:
	// -f wav: Input format is WAV
	// -i pipe:0: Read from stdin
	// -ar 48000: Output sample rate 48kHz
	// -ac 2: Output 2 channels (stereo)
	// -f s16le: Output format raw 16-bit signed little-endian
	// pipe:1: Write to stdout
	args := []string{
		"-f", "wav",
		"-i", "pipe:0",
		"-ar", fmt.Sprintf("%d", DiscordSampleRate),
		"-ac", fmt.Sprintf("%d", DiscordChannels),
		"-f", "s16le",
		"-loglevel", "error",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, c.ffmpegPath, args...)
	cmd.Stdin = bytes.NewReader(wavData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s", ErrConversionFailed, stderr.String())
	}

	return stdout.Bytes(), nil
}

// PCMFrameReader wraps raw PCM data and provides Discord-sized frames.
type PCMFrameReader struct {
	data   []byte
	offset int
}

// NewPCMFrameReader creates a new frame reader from raw PCM data.
func NewPCMFrameReader(pcmData []byte) *PCMFrameReader {
	return &PCMFrameReader{data: pcmData}
}

// ReadFrame reads the next Discord-sized frame (960 samples * 2 channels * 2 bytes).
// Returns io.EOF when no more complete frames are available.
func (r *PCMFrameReader) ReadFrame() ([]byte, error) {
	if r.offset+DiscordFrameBytes > len(r.data) {
		return nil, io.EOF
	}

	frame := r.data[r.offset : r.offset+DiscordFrameBytes]
	r.offset += DiscordFrameBytes
	return frame, nil
}

// Reset resets the reader to the beginning.
func (r *PCMFrameReader) Reset() {
	r.offset = 0
}

// Remaining returns the number of bytes remaining.
func (r *PCMFrameReader) Remaining() int {
	return len(r.data) - r.offset
}
