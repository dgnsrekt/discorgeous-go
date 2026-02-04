// Package wav provides utilities for WAV audio file handling.
package wav

// WAV format constants.
const (
	// HeaderSize is the size of a standard WAV file header in bytes.
	HeaderSize = 44

	// FormatPCM is the audio format code for uncompressed PCM.
	FormatPCM = 1
)

// Common audio configuration constants.
const (
	// PiperSampleRate is the default sample rate output by Piper TTS (22050 Hz).
	PiperSampleRate = 22050

	// PiperChannels is the default number of channels output by Piper TTS (mono).
	PiperChannels = 1

	// PiperBitsPerSample is the default bit depth output by Piper TTS (16-bit).
	PiperBitsPerSample = 16
)

// WrapRawPCM adds a WAV header to raw PCM data.
// Parameters:
//   - pcm: raw PCM audio data bytes
//   - sampleRate: samples per second (e.g., 22050, 44100, 48000)
//   - channels: number of audio channels (1=mono, 2=stereo)
//   - bitsPerSample: bit depth per sample (typically 16)
//
// Returns a complete WAV file as a byte slice.
func WrapRawPCM(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	dataSize := len(pcm)
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8

	// WAV header is 44 bytes
	header := make([]byte, HeaderSize)

	// RIFF header
	copy(header[0:4], "RIFF")
	PutLE32(header[4:8], uint32(36+dataSize))
	copy(header[8:12], "WAVE")

	// fmt subchunk
	copy(header[12:16], "fmt ")
	PutLE32(header[16:20], 16) // subchunk size
	PutLE16(header[20:22], FormatPCM)
	PutLE16(header[22:24], uint16(channels))
	PutLE32(header[24:28], uint32(sampleRate))
	PutLE32(header[28:32], uint32(byteRate))
	PutLE16(header[32:34], uint16(blockAlign))
	PutLE16(header[34:36], uint16(bitsPerSample))

	// data subchunk
	copy(header[36:40], "data")
	PutLE32(header[40:44], uint32(dataSize))

	return append(header, pcm...)
}

// PutLE16 writes a uint16 value in little-endian format to a byte slice.
func PutLE16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// PutLE32 writes a uint32 value in little-endian format to a byte slice.
func PutLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

// CreateMinimal creates a minimal valid WAV file with the specified number of samples.
// This is useful for testing. The samples are initialized to silence (zero).
func CreateMinimal(numSamples, sampleRate, channels, bitsPerSample int) []byte {
	bytesPerSample := bitsPerSample / 8
	dataSize := numSamples * channels * bytesPerSample

	// Create silent PCM data
	pcm := make([]byte, dataSize)

	return WrapRawPCM(pcm, sampleRate, channels, bitsPerSample)
}

// CreateMinimalPiper creates a minimal valid WAV file matching Piper TTS output format.
// This is a convenience wrapper around CreateMinimal using Piper's default parameters.
func CreateMinimalPiper(numSamples int) []byte {
	return CreateMinimal(numSamples, PiperSampleRate, PiperChannels, PiperBitsPerSample)
}
