package discord

import (
	"context"
	"testing"
	"time"
)

func TestErrNotConnected(t *testing.T) {
	if ErrNotConnected.Error() != "not connected to voice channel" {
		t.Errorf("ErrNotConnected = %q", ErrNotConnected.Error())
	}
}

func TestErrAlreadyConnected(t *testing.T) {
	if ErrAlreadyConnected.Error() != "already connected to voice channel" {
		t.Errorf("ErrAlreadyConnected = %q", ErrAlreadyConnected.Error())
	}
}

func TestErrConnectionFailed(t *testing.T) {
	if ErrConnectionFailed.Error() != "failed to connect to voice channel" {
		t.Errorf("ErrConnectionFailed = %q", ErrConnectionFailed.Error())
	}
}

func TestErrSpeakingFailed(t *testing.T) {
	if ErrSpeakingFailed.Error() != "failed to set speaking state" {
		t.Errorf("ErrSpeakingFailed = %q", ErrSpeakingFailed.Error())
	}
}

func TestVoiceManager_IsConnected_WhenNotConnected(t *testing.T) {
	// Create a VoiceManager without opening a session
	// (can't test real Discord connection without token)
	vm := &VoiceManager{
		connected: false,
	}

	if vm.IsConnected() {
		t.Error("IsConnected() = true, want false")
	}
}

func TestVoiceManager_SendAudio_WhenNotConnected(t *testing.T) {
	vm := &VoiceManager{
		connected: false,
	}

	err := vm.SendAudio(context.Background(), []byte{1, 2, 3})
	if err != ErrNotConnected {
		t.Errorf("SendAudio() error = %v, want ErrNotConnected", err)
	}
}

func TestVoiceManager_Disconnect_WhenNotConnected(t *testing.T) {
	vm := &VoiceManager{
		connected:       false,
		voiceConnection: nil,
	}

	// Should not error when already disconnected
	err := vm.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() error = %v, want nil", err)
	}
}

// TestConstants verifies named constants have expected values.
func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      time.Duration
		wantSecs float64
	}{
		{"voiceConnectTimeout", voiceConnectTimeout, 10},
		{"voiceConnectPollInterval", voiceConnectPollInterval, 0.1},
		{"frameDuration", frameDuration, 0.02},
		{"connectRetryDelay", connectRetryDelay, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got.Seconds() != tt.wantSecs {
				t.Errorf("%s = %v, want %v seconds", tt.name, tt.got, tt.wantSecs)
			}
		})
	}

	if maxConnectRetries != 3 {
		t.Errorf("maxConnectRetries = %d, want 3", maxConnectRetries)
	}

	if maxOpusDataBytes != 4000 {
		t.Errorf("maxOpusDataBytes = %d, want 4000", maxOpusDataBytes)
	}
}
