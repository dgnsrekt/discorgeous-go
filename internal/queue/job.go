package queue

import (
	"time"

	"github.com/google/uuid"
)

// SpeakJob represents a speech job to be processed.
type SpeakJob struct {
	ID        string
	Text      string
	Voice     string
	Interrupt bool
	TTL       time.Duration
	DedupeKey string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewSpeakJob creates a new speak job with a unique ID.
func NewSpeakJob(text, voice string, interrupt bool, ttl time.Duration, dedupeKey string) *SpeakJob {
	now := time.Now()
	job := &SpeakJob{
		ID:        uuid.New().String(),
		Text:      text,
		Voice:     voice,
		Interrupt: interrupt,
		TTL:       ttl,
		DedupeKey: dedupeKey,
		CreatedAt: now,
	}

	if ttl > 0 {
		job.ExpiresAt = now.Add(ttl)
	}

	return job
}

// IsExpired returns true if the job has passed its TTL.
func (j *SpeakJob) IsExpired() bool {
	if j.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(j.ExpiresAt)
}
