package queue

import (
	"testing"
	"time"
)

func TestNewSpeakJob(t *testing.T) {
	job := NewSpeakJob("Hello, world!", "en-us", false, 5*time.Second, "key123")

	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if job.Text != "Hello, world!" {
		t.Errorf("expected text 'Hello, world!', got '%s'", job.Text)
	}
	if job.Voice != "en-us" {
		t.Errorf("expected voice 'en-us', got '%s'", job.Voice)
	}
	if job.Interrupt {
		t.Error("expected interrupt to be false")
	}
	if job.TTL != 5*time.Second {
		t.Errorf("expected TTL 5s, got %v", job.TTL)
	}
	if job.DedupeKey != "key123" {
		t.Errorf("expected dedupe_key 'key123', got '%s'", job.DedupeKey)
	}
	if job.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
	if job.ExpiresAt.IsZero() {
		t.Error("expected non-zero expires_at when TTL is set")
	}
}

func TestNewSpeakJobNoTTL(t *testing.T) {
	job := NewSpeakJob("Hello", "default", false, 0, "")

	if !job.ExpiresAt.IsZero() {
		t.Error("expected zero expires_at when TTL is zero")
	}
}

func TestIsExpired(t *testing.T) {
	// Job with no TTL never expires
	job := NewSpeakJob("Hello", "default", false, 0, "")
	if job.IsExpired() {
		t.Error("job with no TTL should not be expired")
	}

	// Job with future expiry
	job = NewSpeakJob("Hello", "default", false, 1*time.Hour, "")
	if job.IsExpired() {
		t.Error("job with future expiry should not be expired")
	}

	// Job with past expiry
	job = NewSpeakJob("Hello", "default", false, 1*time.Millisecond, "")
	time.Sleep(5 * time.Millisecond)
	if !job.IsExpired() {
		t.Error("job with past expiry should be expired")
	}
}

func TestJobIDsAreUnique(t *testing.T) {
	job1 := NewSpeakJob("Hello", "default", false, 0, "")
	job2 := NewSpeakJob("Hello", "default", false, 0, "")

	if job1.ID == job2.ID {
		t.Error("expected unique job IDs")
	}
}
