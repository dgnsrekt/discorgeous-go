package audio

import (
	"context"
	"io"
	"os/exec"
	"testing"
	"time"
)

func TestNewConverter(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed, skipping converter tests")
	}

	conv, err := NewConverter()
	if err != nil {
		t.Fatalf("NewConverter() error = %v", err)
	}
	if conv == nil {
		t.Fatal("NewConverter() returned nil")
	}
}

func TestNewConverterWithPath(t *testing.T) {
	conv := NewConverterWithPath("/usr/bin/ffmpeg")
	if conv == nil {
		t.Fatal("NewConverterWithPath() returned nil")
	}
	if conv.ffmpegPath != "/usr/bin/ffmpeg" {
		t.Errorf("ffmpegPath = %q, want %q", conv.ffmpegPath, "/usr/bin/ffmpeg")
	}
}

func TestConverter_ConvertToDiscordPCM_EmptyInput(t *testing.T) {
	conv := NewConverterWithPath("ffmpeg")

	_, err := conv.ConvertToDiscordPCM(context.Background(), nil)
	if err == nil {
		t.Error("ConvertToDiscordPCM(nil) should return error")
	}

	_, err = conv.ConvertToDiscordPCM(context.Background(), []byte{})
	if err == nil {
		t.Error("ConvertToDiscordPCM([]) should return error")
	}
}

func TestConverter_ConvertToDiscordPCM_InvalidWAV(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed, skipping converter tests")
	}

	conv, _ := NewConverter()

	// Pass invalid WAV data
	_, err = conv.ConvertToDiscordPCM(context.Background(), []byte("not a wav file"))
	if err == nil {
		t.Error("ConvertToDiscordPCM(invalid) should return error")
	}
}

func TestConverter_ConvertToDiscordPCM_ContextCancel(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed, skipping converter tests")
	}

	conv, _ := NewConverter()

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create a minimal valid WAV (just header, no data)
	wav := createMinimalWAV(0)

	_, err = conv.ConvertToDiscordPCM(ctx, wav)
	if err != context.Canceled {
		t.Errorf("ConvertToDiscordPCM(cancelled) error = %v, want context.Canceled", err)
	}
}

func TestConverter_ConvertToDiscordPCM_ValidWAV(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed, skipping converter tests")
	}

	conv, _ := NewConverter()

	// Create a minimal valid WAV with some samples
	// 100 samples at 22050 Hz mono = ~4.5ms of audio
	wav := createMinimalWAV(100)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pcm, err := conv.ConvertToDiscordPCM(ctx, wav)
	if err != nil {
		t.Fatalf("ConvertToDiscordPCM() error = %v", err)
	}

	// Output should be resampled to 48kHz stereo
	// 100 samples at 22050Hz = ~4.5ms
	// At 48kHz stereo, that's approximately 48000 * 0.0045 * 2 * 2 bytes = ~864 bytes
	// Allow for some variance due to resampling
	if len(pcm) == 0 {
		t.Error("ConvertToDiscordPCM() returned empty output")
	}
}

// createMinimalWAV creates a minimal valid WAV file with the specified number of samples.
// The WAV is 22050 Hz, mono, 16-bit (matching Piper output).
func createMinimalWAV(numSamples int) []byte {
	sampleRate := 22050
	channels := 1
	bitsPerSample := 16
	bytesPerSample := bitsPerSample / 8
	dataSize := numSamples * channels * bytesPerSample
	fileSize := 36 + dataSize

	wav := make([]byte, 44+dataSize)

	// RIFF header
	copy(wav[0:4], "RIFF")
	writeLE32(wav[4:8], uint32(fileSize))
	copy(wav[8:12], "WAVE")

	// fmt chunk
	copy(wav[12:16], "fmt ")
	writeLE32(wav[16:20], 16)                              // chunk size
	writeLE16(wav[20:22], 1)                               // audio format (PCM)
	writeLE16(wav[22:24], uint16(channels))                // num channels
	writeLE32(wav[24:28], uint32(sampleRate))              // sample rate
	writeLE32(wav[28:32], uint32(sampleRate*channels*2))   // byte rate
	writeLE16(wav[32:34], uint16(channels*bytesPerSample)) // block align
	writeLE16(wav[34:36], uint16(bitsPerSample))           // bits per sample

	// data chunk
	copy(wav[36:40], "data")
	writeLE32(wav[40:44], uint32(dataSize))

	// Fill with silence (zeros)
	// Already zero-initialized

	return wav
}

func writeLE16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func writeLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func TestPCMFrameReader_ReadFrame(t *testing.T) {
	// Create PCM data for exactly 2 frames
	data := make([]byte, DiscordFrameBytes*2)
	for i := range data {
		data[i] = byte(i % 256)
	}

	reader := NewPCMFrameReader(data)

	// Read first frame
	frame1, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame() 1 error = %v", err)
	}
	if len(frame1) != DiscordFrameBytes {
		t.Errorf("frame1 length = %d, want %d", len(frame1), DiscordFrameBytes)
	}

	// Read second frame
	frame2, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame() 2 error = %v", err)
	}
	if len(frame2) != DiscordFrameBytes {
		t.Errorf("frame2 length = %d, want %d", len(frame2), DiscordFrameBytes)
	}

	// Third read should return EOF
	_, err = reader.ReadFrame()
	if err != io.EOF {
		t.Errorf("ReadFrame() 3 error = %v, want io.EOF", err)
	}
}

func TestPCMFrameReader_PartialFrame(t *testing.T) {
	// Create PCM data for 1.5 frames (partial last frame)
	data := make([]byte, DiscordFrameBytes+DiscordFrameBytes/2)

	reader := NewPCMFrameReader(data)

	// First frame should succeed
	_, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame() 1 error = %v", err)
	}

	// Second read should return EOF (not enough data for full frame)
	_, err = reader.ReadFrame()
	if err != io.EOF {
		t.Errorf("ReadFrame() 2 error = %v, want io.EOF", err)
	}
}

func TestPCMFrameReader_Reset(t *testing.T) {
	data := make([]byte, DiscordFrameBytes)
	reader := NewPCMFrameReader(data)

	// Read the frame
	_, _ = reader.ReadFrame()
	if reader.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", reader.Remaining())
	}

	// Reset and read again
	reader.Reset()
	if reader.Remaining() != DiscordFrameBytes {
		t.Errorf("Remaining() after reset = %d, want %d", reader.Remaining(), DiscordFrameBytes)
	}

	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame() after reset error = %v", err)
	}
	if len(frame) != DiscordFrameBytes {
		t.Errorf("frame length = %d, want %d", len(frame), DiscordFrameBytes)
	}
}

func TestDiscordConstants(t *testing.T) {
	// Verify Discord audio constants are correct
	if DiscordSampleRate != 48000 {
		t.Errorf("DiscordSampleRate = %d, want 48000", DiscordSampleRate)
	}
	if DiscordChannels != 2 {
		t.Errorf("DiscordChannels = %d, want 2", DiscordChannels)
	}
	if DiscordFrameSize != 960 {
		t.Errorf("DiscordFrameSize = %d, want 960", DiscordFrameSize)
	}
	// 960 samples * 2 channels * 2 bytes = 3840 bytes
	if DiscordFrameBytes != 3840 {
		t.Errorf("DiscordFrameBytes = %d, want 3840", DiscordFrameBytes)
	}
}
