package tts

import (
	"bytes"
	"io"
	"testing"
)

func TestAudioResult_Reader(t *testing.T) {
	testData := []byte("test audio data")
	result := &AudioResult{
		Data:       testData,
		Format:     "wav",
		SampleRate: 22050,
		Channels:   1,
	}

	reader := result.Reader()

	// Read all data
	buf := new(bytes.Buffer)
	n, err := io.Copy(buf, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if int(n) != len(testData) {
		t.Errorf("expected %d bytes, got %d", len(testData), n)
	}

	if !bytes.Equal(buf.Bytes(), testData) {
		t.Errorf("data mismatch: got %v, want %v", buf.Bytes(), testData)
	}
}

func TestAudioReader_MultipleReads(t *testing.T) {
	testData := []byte("hello world test data")
	result := &AudioResult{Data: testData}
	reader := result.Reader()

	// Read in chunks
	buf := make([]byte, 5)
	var totalRead int
	var chunks [][]byte

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			totalRead += n
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			chunks = append(chunks, chunk)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if totalRead != len(testData) {
		t.Errorf("expected total read %d, got %d", len(testData), totalRead)
	}

	// Reconstruct and verify
	var reconstructed []byte
	for _, chunk := range chunks {
		reconstructed = append(reconstructed, chunk...)
	}
	if !bytes.Equal(reconstructed, testData) {
		t.Errorf("reconstructed data mismatch")
	}
}

func TestAudioReader_EmptyData(t *testing.T) {
	result := &AudioResult{Data: []byte{}}
	reader := result.Reader()

	buf := make([]byte, 10)
	n, err := reader.Read(buf)

	if n != 0 {
		t.Errorf("expected 0 bytes read, got %d", n)
	}
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}
