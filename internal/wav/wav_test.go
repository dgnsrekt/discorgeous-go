package wav

import (
	"bytes"
	"testing"
)

func TestConstants(t *testing.T) {
	// Verify WAV constants
	if HeaderSize != 44 {
		t.Errorf("HeaderSize = %d, want 44", HeaderSize)
	}
	if FormatPCM != 1 {
		t.Errorf("FormatPCM = %d, want 1", FormatPCM)
	}

	// Verify Piper constants
	if PiperSampleRate != 22050 {
		t.Errorf("PiperSampleRate = %d, want 22050", PiperSampleRate)
	}
	if PiperChannels != 1 {
		t.Errorf("PiperChannels = %d, want 1", PiperChannels)
	}
	if PiperBitsPerSample != 16 {
		t.Errorf("PiperBitsPerSample = %d, want 16", PiperBitsPerSample)
	}
}

func TestPutLE16(t *testing.T) {
	tests := []struct {
		name   string
		value  uint16
		expect []byte
	}{
		{"zero", 0, []byte{0x00, 0x00}},
		{"one", 1, []byte{0x01, 0x00}},
		{"256", 256, []byte{0x00, 0x01}},
		{"max", 0xFFFF, []byte{0xFF, 0xFF}},
		{"mixed", 0x1234, []byte{0x34, 0x12}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := make([]byte, 2)
			PutLE16(b, tt.value)
			if !bytes.Equal(b, tt.expect) {
				t.Errorf("PutLE16(%d) = %v, want %v", tt.value, b, tt.expect)
			}
		})
	}
}

func TestPutLE32(t *testing.T) {
	tests := []struct {
		name   string
		value  uint32
		expect []byte
	}{
		{"zero", 0, []byte{0x00, 0x00, 0x00, 0x00}},
		{"one", 1, []byte{0x01, 0x00, 0x00, 0x00}},
		{"256", 256, []byte{0x00, 0x01, 0x00, 0x00}},
		{"max", 0xFFFFFFFF, []byte{0xFF, 0xFF, 0xFF, 0xFF}},
		{"mixed", 0x12345678, []byte{0x78, 0x56, 0x34, 0x12}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := make([]byte, 4)
			PutLE32(b, tt.value)
			if !bytes.Equal(b, tt.expect) {
				t.Errorf("PutLE32(%d) = %v, want %v", tt.value, b, tt.expect)
			}
		})
	}
}

func TestWrapRawPCM(t *testing.T) {
	pcmData := []byte{0x01, 0x02, 0x03, 0x04}
	wavData := WrapRawPCM(pcmData, 22050, 1, 16)

	// Check total size
	if len(wavData) != HeaderSize+len(pcmData) {
		t.Errorf("expected %d bytes, got %d", HeaderSize+len(pcmData), len(wavData))
	}

	// Check RIFF header
	if !bytes.Equal(wavData[0:4], []byte("RIFF")) {
		t.Errorf("missing RIFF header")
	}

	// Check WAVE format
	if !bytes.Equal(wavData[8:12], []byte("WAVE")) {
		t.Errorf("missing WAVE format")
	}

	// Check fmt chunk
	if !bytes.Equal(wavData[12:16], []byte("fmt ")) {
		t.Errorf("missing fmt chunk")
	}

	// Check data chunk
	if !bytes.Equal(wavData[36:40], []byte("data")) {
		t.Errorf("missing data chunk")
	}

	// Check file size (should be 36 + data size)
	fileSize := uint32(wavData[4]) | uint32(wavData[5])<<8 | uint32(wavData[6])<<16 | uint32(wavData[7])<<24
	if fileSize != uint32(36+len(pcmData)) {
		t.Errorf("file size = %d, want %d", fileSize, 36+len(pcmData))
	}

	// Check data size
	dataSize := uint32(wavData[40]) | uint32(wavData[41])<<8 | uint32(wavData[42])<<16 | uint32(wavData[43])<<24
	if dataSize != uint32(len(pcmData)) {
		t.Errorf("data size = %d, want %d", dataSize, len(pcmData))
	}

	// Check sample rate
	sampleRate := uint32(wavData[24]) | uint32(wavData[25])<<8 | uint32(wavData[26])<<16 | uint32(wavData[27])<<24
	if sampleRate != 22050 {
		t.Errorf("sample rate = %d, want 22050", sampleRate)
	}

	// Check channels
	channels := uint16(wavData[22]) | uint16(wavData[23])<<8
	if channels != 1 {
		t.Errorf("channels = %d, want 1", channels)
	}

	// Check bits per sample
	bitsPerSample := uint16(wavData[34]) | uint16(wavData[35])<<8
	if bitsPerSample != 16 {
		t.Errorf("bits per sample = %d, want 16", bitsPerSample)
	}

	// Check PCM data is at the end
	if !bytes.Equal(wavData[44:], pcmData) {
		t.Errorf("PCM data mismatch")
	}
}

func TestWrapRawPCM_Stereo(t *testing.T) {
	pcmData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	wavData := WrapRawPCM(pcmData, 44100, 2, 16)

	// Check channels
	channels := uint16(wavData[22]) | uint16(wavData[23])<<8
	if channels != 2 {
		t.Errorf("channels = %d, want 2", channels)
	}

	// Check sample rate
	sampleRate := uint32(wavData[24]) | uint32(wavData[25])<<8 | uint32(wavData[26])<<16 | uint32(wavData[27])<<24
	if sampleRate != 44100 {
		t.Errorf("sample rate = %d, want 44100", sampleRate)
	}

	// Check byte rate (44100 * 2 channels * 2 bytes = 176400)
	byteRate := uint32(wavData[28]) | uint32(wavData[29])<<8 | uint32(wavData[30])<<16 | uint32(wavData[31])<<24
	if byteRate != 176400 {
		t.Errorf("byte rate = %d, want 176400", byteRate)
	}

	// Check block align (2 channels * 2 bytes = 4)
	blockAlign := uint16(wavData[32]) | uint16(wavData[33])<<8
	if blockAlign != 4 {
		t.Errorf("block align = %d, want 4", blockAlign)
	}
}

func TestCreateMinimal(t *testing.T) {
	wav := CreateMinimal(100, 44100, 2, 16)

	// Expected size: 44 header + 100 samples * 2 channels * 2 bytes = 444
	expectedSize := HeaderSize + 100*2*2
	if len(wav) != expectedSize {
		t.Errorf("CreateMinimal(100, 44100, 2, 16) length = %d, want %d", len(wav), expectedSize)
	}

	// Data should be zeros (silence)
	for i := HeaderSize; i < len(wav); i++ {
		if wav[i] != 0 {
			t.Errorf("CreateMinimal should produce silence, got non-zero at byte %d", i)
			break
		}
	}
}

func TestCreateMinimalPiper(t *testing.T) {
	wav := CreateMinimalPiper(100)

	// Expected size: 44 header + 100 samples * 1 channel * 2 bytes = 244
	expectedSize := HeaderSize + 100*1*2
	if len(wav) != expectedSize {
		t.Errorf("CreateMinimalPiper(100) length = %d, want %d", len(wav), expectedSize)
	}

	// Check Piper format parameters
	sampleRate := uint32(wav[24]) | uint32(wav[25])<<8 | uint32(wav[26])<<16 | uint32(wav[27])<<24
	if sampleRate != PiperSampleRate {
		t.Errorf("sample rate = %d, want %d", sampleRate, PiperSampleRate)
	}

	channels := uint16(wav[22]) | uint16(wav[23])<<8
	if channels != PiperChannels {
		t.Errorf("channels = %d, want %d", channels, PiperChannels)
	}

	bitsPerSample := uint16(wav[34]) | uint16(wav[35])<<8
	if bitsPerSample != PiperBitsPerSample {
		t.Errorf("bits per sample = %d, want %d", bitsPerSample, PiperBitsPerSample)
	}
}

func TestWrapRawPCM_EmptyData(t *testing.T) {
	wav := WrapRawPCM(nil, 22050, 1, 16)

	// Should still produce valid header with zero-length data
	if len(wav) != HeaderSize {
		t.Errorf("WrapRawPCM(nil) length = %d, want %d", len(wav), HeaderSize)
	}

	// Data size should be 0
	dataSize := uint32(wav[40]) | uint32(wav[41])<<8 | uint32(wav[42])<<16 | uint32(wav[43])<<24
	if dataSize != 0 {
		t.Errorf("data size = %d, want 0", dataSize)
	}
}
