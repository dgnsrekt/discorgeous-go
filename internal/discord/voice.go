package discord

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dgnsrekt/discorgeous-go/internal/audio"
	"layeh.com/gopus"
)

const (
	// voiceConnectTimeout is the maximum time to wait for voice connection readiness.
	voiceConnectTimeout = 10 * time.Second
	// voiceConnectPollInterval is the polling interval while waiting for connection.
	voiceConnectPollInterval = 100 * time.Millisecond
	// frameDuration is the duration of one Discord audio frame (20ms).
	frameDuration = 20 * time.Millisecond
	// maxOpusDataBytes is the maximum size of an encoded Opus frame.
	maxOpusDataBytes = 4000
)

var (
	// ErrNotConnected is returned when trying to send audio while not connected.
	ErrNotConnected = errors.New("not connected to voice channel")
	// ErrAlreadyConnected is returned when trying to connect while already connected.
	ErrAlreadyConnected = errors.New("already connected to voice channel")
	// ErrConnectionFailed is returned when voice connection fails.
	ErrConnectionFailed = errors.New("failed to connect to voice channel")
)

// VoiceManager manages Discord voice connections.
type VoiceManager struct {
	mu              sync.Mutex
	session         *discordgo.Session
	voiceConnection *discordgo.VoiceConnection
	guildID         string
	channelID       string
	logger          *slog.Logger
	connected       bool
	opusEncoder     *gopus.Encoder
}

// NewVoiceManager creates a new voice manager.
func NewVoiceManager(token, guildID, channelID string, logger *slog.Logger) (*VoiceManager, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	// Create Opus encoder (48kHz, stereo, voip application)
	encoder, err := gopus.NewEncoder(audio.DiscordSampleRate, audio.DiscordChannels, gopus.Voip)
	if err != nil {
		return nil, err
	}

	return &VoiceManager{
		session:     session,
		guildID:     guildID,
		channelID:   channelID,
		logger:      logger,
		opusEncoder: encoder,
	}, nil
}

// Open opens the Discord session.
func (vm *VoiceManager) Open() error {
	return vm.session.Open()
}

// Close closes the Discord session and voice connection.
func (vm *VoiceManager) Close() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.voiceConnection != nil {
		vm.voiceConnection.Disconnect()
		vm.voiceConnection = nil
	}
	vm.connected = false

	return vm.session.Close()
}

// Connect joins the configured voice channel.
func (vm *VoiceManager) Connect(ctx context.Context) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.connected && vm.voiceConnection != nil {
		return nil // Already connected
	}

	vm.logger.Info("connecting to voice channel", "guild_id", vm.guildID, "channel_id", vm.channelID)

	// Join voice channel (mute=false, deaf=true - we don't need to hear)
	vc, err := vm.session.ChannelVoiceJoin(vm.guildID, vm.channelID, false, true)
	if err != nil {
		return err
	}

	// Wait for the voice connection to be ready
	// discordgo's Ready is a bool, so we poll with timeout
	deadline := time.Now().Add(voiceConnectTimeout)
	for {
		if ctx.Err() != nil {
			vc.Disconnect()
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			vc.Disconnect()
			return ErrConnectionFailed
		}
		if vc.Ready {
			break
		}
		time.Sleep(voiceConnectPollInterval)
	}

	vm.voiceConnection = vc
	vm.connected = true
	vm.logger.Info("connected to voice channel")

	return nil
}

// Disconnect leaves the voice channel.
func (vm *VoiceManager) Disconnect() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.voiceConnection == nil {
		return nil
	}

	vm.logger.Info("disconnecting from voice channel")
	err := vm.voiceConnection.Disconnect()
	vm.voiceConnection = nil
	vm.connected = false

	return err
}

// IsConnected returns whether the bot is connected to voice.
func (vm *VoiceManager) IsConnected() bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.connected && vm.voiceConnection != nil
}

// SendAudio sends PCM audio data to the voice channel.
// The PCM data must be 48kHz, stereo, 16-bit signed little-endian.
func (vm *VoiceManager) SendAudio(ctx context.Context, pcmData []byte) error {
	vm.mu.Lock()
	vc := vm.voiceConnection
	connected := vm.connected
	vm.mu.Unlock()

	if !connected || vc == nil {
		return ErrNotConnected
	}

	frameReader := audio.NewPCMFrameReader(pcmData)

	// Start speaking
	if err := vc.Speaking(true); err != nil {
		vm.logger.Error("failed to set speaking state", "error", err)
	}

	defer func() {
		// Stop speaking
		if err := vc.Speaking(false); err != nil {
			vm.logger.Error("failed to clear speaking state", "error", err)
		}
	}()

	// Send frames with timing control
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			frame, err := frameReader.ReadFrame()
			if err == io.EOF {
				return nil // Done sending
			}
			if err != nil {
				return err
			}

			// Encode PCM frame to Opus
			opusData, err := vm.encodeOpus(frame)
			if err != nil {
				vm.logger.Error("opus encoding failed", "error", err)
				continue
			}

			// Send the frame to Discord
			select {
			case <-ctx.Done():
				return ctx.Err()
			case vc.OpusSend <- opusData:
				// Frame sent
			}
		}
	}
}

// encodeOpus converts raw PCM to Opus.
// Input: 960 samples * 2 channels * 2 bytes = 3840 bytes of PCM
// Output: Opus encoded data
func (vm *VoiceManager) encodeOpus(pcm []byte) ([]byte, error) {
	// Convert bytes to int16 samples
	samples := make([]int16, len(pcm)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(pcm[i*2:]))
	}

	// Encode to Opus
	// frameSize: number of samples per channel (960 for 20ms at 48kHz)
	// maxDataBytes: maximum size of output buffer
	opus, err := vm.opusEncoder.Encode(samples, audio.DiscordFrameSize, maxOpusDataBytes)
	if err != nil {
		return nil, err
	}

	return opus, nil
}
