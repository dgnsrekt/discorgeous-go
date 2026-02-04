package playback

import (
	"context"
	"errors"
	"log/slog"

	"github.com/dgnsrekt/discorgeous-go/internal/audio"
	"github.com/dgnsrekt/discorgeous-go/internal/discord"
	"github.com/dgnsrekt/discorgeous-go/internal/queue"
	"github.com/dgnsrekt/discorgeous-go/internal/tts"
)

var (
	// ErrNoTTSEngine is returned when no TTS engine is available.
	ErrNoTTSEngine = errors.New("no TTS engine available")
	// ErrPlaybackSynthesisFailed is returned when TTS synthesis fails during playback.
	ErrPlaybackSynthesisFailed = errors.New("playback synthesis failed")
	// ErrConversionFailed is returned when audio conversion fails.
	ErrConversionFailed = errors.New("audio conversion failed")
)

// Handler processes speech jobs using TTS and Discord voice.
type Handler struct {
	ttsRegistry  *tts.Registry
	audioConv    *audio.Converter
	voiceManager *discord.VoiceManager
	logger       *slog.Logger
}

// NewHandler creates a new playback handler.
func NewHandler(
	ttsRegistry *tts.Registry,
	audioConv *audio.Converter,
	voiceManager *discord.VoiceManager,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		ttsRegistry:  ttsRegistry,
		audioConv:    audioConv,
		voiceManager: voiceManager,
		logger:       logger,
	}
}

// Handle processes a single speech job.
// This is the function passed to queue.SetPlaybackHandler.
func (h *Handler) Handle(ctx context.Context, job *queue.SpeakJob) error {
	h.logger.Info("processing speech job",
		"job_id", job.ID,
		"text_length", len(job.Text),
		"voice", job.Voice,
	)

	// Step 1: Get TTS engine
	engine, err := h.ttsRegistry.Default()
	if err != nil {
		return ErrNoTTSEngine
	}

	// Step 2: Synthesize text to audio
	h.logger.Debug("synthesizing speech", "job_id", job.ID, "engine", engine.Name())

	audioResult, err := engine.Synthesize(ctx, tts.SynthesizeRequest{
		Text:  job.Text,
		Voice: job.Voice,
	})
	if err != nil {
		h.logger.Error("TTS synthesis failed", "job_id", job.ID, "error", err)
		return errors.Join(ErrPlaybackSynthesisFailed, err)
	}

	h.logger.Debug("synthesis complete",
		"job_id", job.ID,
		"format", audioResult.Format,
		"sample_rate", audioResult.SampleRate,
		"channels", audioResult.Channels,
		"bytes", len(audioResult.Data),
	)

	// Step 3: Convert audio to Discord format (48kHz stereo PCM)
	h.logger.Debug("converting audio", "job_id", job.ID)

	pcmData, err := h.audioConv.ConvertToDiscordPCM(ctx, audioResult.Data)
	if err != nil {
		h.logger.Error("audio conversion failed", "job_id", job.ID, "error", err)
		return errors.Join(ErrConversionFailed, err)
	}

	h.logger.Debug("conversion complete", "job_id", job.ID, "pcm_bytes", len(pcmData))

	// Step 4: Ensure connected to voice channel
	if !h.voiceManager.IsConnected() {
		h.logger.Info("connecting to voice channel", "job_id", job.ID)
		if err := h.voiceManager.Connect(ctx); err != nil {
			h.logger.Error("voice connection failed", "job_id", job.ID, "error", err)
			return err
		}
	}

	// Step 5: Send audio to Discord
	h.logger.Debug("sending audio to voice channel", "job_id", job.ID)

	if err := h.voiceManager.SendAudio(ctx, pcmData); err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Info("playback interrupted", "job_id", job.ID)
		} else {
			h.logger.Error("audio send failed", "job_id", job.ID, "error", err)
		}
		return err
	}

	h.logger.Info("speech playback complete", "job_id", job.ID)
	return nil
}
