package tts

import (
	"context"
	"io"
)

// SynthesizeRequest contains parameters for TTS synthesis.
type SynthesizeRequest struct {
	Text  string
	Voice string
}

// AudioResult represents synthesized audio output.
type AudioResult struct {
	// Data contains the raw audio bytes (WAV format).
	Data []byte
	// Format describes the audio format (e.g., "wav").
	Format string
	// SampleRate is the audio sample rate in Hz.
	SampleRate int
	// Channels is the number of audio channels.
	Channels int
}

// Reader returns an io.Reader for the audio data.
func (a *AudioResult) Reader() io.Reader {
	return &audioReader{data: a.Data}
}

type audioReader struct {
	data   []byte
	offset int
}

func (r *audioReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

// Engine is the interface for text-to-speech synthesis.
type Engine interface {
	// Synthesize converts text to audio.
	Synthesize(ctx context.Context, req SynthesizeRequest) (*AudioResult, error)
	// Name returns the engine identifier.
	Name() string
}
