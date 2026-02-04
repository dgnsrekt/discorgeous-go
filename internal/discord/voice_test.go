package discord

import (
	"context"
	"testing"
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
