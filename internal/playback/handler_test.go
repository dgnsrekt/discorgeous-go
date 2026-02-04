package playback

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dgnsrekt/discorgeous-go/internal/queue"
	"github.com/dgnsrekt/discorgeous-go/internal/tts"
)

// testLogger returns a no-op logger for tests
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestErrors(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{ErrNoTTSEngine, "no TTS engine available"},
		{ErrSynthesisFailed, "TTS synthesis failed"},
		{ErrConversionFailed, "audio conversion failed"},
	}

	for _, tt := range tests {
		if tt.err.Error() != tt.want {
			t.Errorf("%v = %q, want %q", tt.err, tt.err.Error(), tt.want)
		}
	}
}

func TestNewHandler(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)
	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestHandler_Handle_NoTTSEngine(t *testing.T) {
	// Create an empty registry (no engines)
	registry := tts.NewRegistry()

	handler := NewHandler(registry, nil, nil, testLogger())

	job := &queue.SpeakJob{
		ID:        "test-job",
		Text:      "Hello",
		Voice:     "default",
		CreatedAt: time.Now(),
	}

	err := handler.Handle(context.Background(), job)
	if !errors.Is(err, ErrNoTTSEngine) {
		t.Errorf("Handle() error = %v, want ErrNoTTSEngine", err)
	}
}

// mockEngine is a test TTS engine
type mockEngine struct {
	name      string
	result    *tts.AudioResult
	err       error
	callCount int
}

func (m *mockEngine) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (*tts.AudioResult, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockEngine) Name() string {
	return m.name
}

func TestHandler_Handle_SynthesisFails(t *testing.T) {
	registry := tts.NewRegistry()
	engine := &mockEngine{
		name: "mock",
		err:  errors.New("synthesis error"),
	}
	_ = registry.Register(engine)

	handler := NewHandler(registry, nil, nil, testLogger())

	job := &queue.SpeakJob{
		ID:        "test-job",
		Text:      "Hello",
		Voice:     "default",
		CreatedAt: time.Now(),
	}

	err := handler.Handle(context.Background(), job)
	if !errors.Is(err, ErrSynthesisFailed) {
		t.Errorf("Handle() error = %v, want ErrSynthesisFailed", err)
	}
	if engine.callCount != 1 {
		t.Errorf("Synthesize called %d times, want 1", engine.callCount)
	}
}
