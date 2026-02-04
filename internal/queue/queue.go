package queue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var (
	// ErrQueueFull is returned when the queue is at capacity.
	ErrQueueFull = errors.New("queue is full")
	// ErrQueueClosed is returned when attempting to enqueue to a closed queue.
	ErrQueueClosed = errors.New("queue is closed")
	// ErrDuplicateJob is returned when a job with the same dedupe key exists.
	ErrDuplicateJob = errors.New("duplicate job")
)

// PlaybackHandler is called by the worker to play a job.
// Implementations should handle the actual TTS and voice playback.
type PlaybackHandler func(ctx context.Context, job *SpeakJob) error

// IdleCallback is called when the queue becomes idle.
type IdleCallback func()

// Queue is a bounded queue with a single playback worker.
type Queue struct {
	mu            sync.Mutex
	jobs          []*SpeakJob
	capacity      int
	dedupeKeys    map[string]bool
	logger        *slog.Logger
	closed        bool
	idleTimeout   time.Duration
	idleCallback  IdleCallback
	playbackFunc  PlaybackHandler
	cancelCurrent context.CancelFunc
	wg            sync.WaitGroup
	stopCh        chan struct{}
	enqueueCh     chan struct{}
}

// NewQueue creates a new bounded queue.
func NewQueue(capacity int, idleTimeout time.Duration, logger *slog.Logger) *Queue {
	return &Queue{
		jobs:        make([]*SpeakJob, 0, capacity),
		capacity:    capacity,
		dedupeKeys:  make(map[string]bool),
		logger:      logger,
		idleTimeout: idleTimeout,
		stopCh:      make(chan struct{}),
		enqueueCh:   make(chan struct{}, 1),
	}
}

// SetPlaybackHandler sets the function called to play each job.
func (q *Queue) SetPlaybackHandler(fn PlaybackHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.playbackFunc = fn
}

// SetIdleCallback sets the function called when the queue becomes idle.
func (q *Queue) SetIdleCallback(fn IdleCallback) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.idleCallback = fn
}

// Enqueue adds a job to the queue.
func (q *Queue) Enqueue(job *SpeakJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	if len(q.jobs) >= q.capacity {
		return ErrQueueFull
	}

	// Check for duplicate dedupe key
	if job.DedupeKey != "" && q.dedupeKeys[job.DedupeKey] {
		return ErrDuplicateJob
	}

	q.jobs = append(q.jobs, job)
	if job.DedupeKey != "" {
		q.dedupeKeys[job.DedupeKey] = true
	}

	q.logger.Debug("job enqueued", "job_id", job.ID, "queue_depth", len(q.jobs))

	// Signal the worker
	select {
	case q.enqueueCh <- struct{}{}:
	default:
	}

	return nil
}

// Interrupt cancels the current playback and clears the queue.
func (q *Queue) Interrupt() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Cancel current playback
	if q.cancelCurrent != nil {
		q.cancelCurrent()
		q.cancelCurrent = nil
	}

	// Clear the queue
	cleared := len(q.jobs)
	q.jobs = q.jobs[:0]
	q.dedupeKeys = make(map[string]bool)

	q.logger.Info("queue interrupted", "jobs_cleared", cleared)
}

// Len returns the current queue length.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// Start begins the playback worker goroutine.
func (q *Queue) Start() {
	q.wg.Add(1)
	go q.worker()
}

// Stop gracefully stops the worker.
func (q *Queue) Stop() {
	q.mu.Lock()
	q.closed = true
	if q.cancelCurrent != nil {
		q.cancelCurrent()
	}
	q.mu.Unlock()

	close(q.stopCh)
	q.wg.Wait()
}

// worker is the single playback goroutine.
func (q *Queue) worker() {
	defer q.wg.Done()

	var idleTimer *time.Timer
	var idleTimerCh <-chan time.Time

	resetIdleTimer := func() {
		if idleTimer != nil {
			idleTimer.Stop()
		}
		if q.idleTimeout > 0 {
			idleTimer = time.NewTimer(q.idleTimeout)
			idleTimerCh = idleTimer.C
		}
	}

	stopIdleTimer := func() {
		if idleTimer != nil {
			idleTimer.Stop()
			idleTimerCh = nil
		}
	}

	for {
		// Try to get next job
		job := q.dequeue()

		if job != nil {
			stopIdleTimer()
			q.processJob(job)
			continue
		}

		// Queue is empty, start idle timer if not already running
		if idleTimerCh == nil && q.idleTimeout > 0 {
			resetIdleTimer()
		}

		select {
		case <-q.stopCh:
			stopIdleTimer()
			return
		case <-q.enqueueCh:
			// New job available
			continue
		case <-idleTimerCh:
			// Idle timeout reached
			q.mu.Lock()
			callback := q.idleCallback
			q.mu.Unlock()

			if callback != nil {
				q.logger.Info("idle timeout reached")
				callback()
			}
			idleTimerCh = nil
		}
	}
}

// dequeue removes and returns the next job from the queue.
func (q *Queue) dequeue() *SpeakJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.jobs) > 0 {
		job := q.jobs[0]
		q.jobs = q.jobs[1:]

		// Remove dedupe key
		if job.DedupeKey != "" {
			delete(q.dedupeKeys, job.DedupeKey)
		}

		// Skip expired jobs
		if job.IsExpired() {
			q.logger.Debug("skipping expired job", "job_id", job.ID)
			continue
		}

		return job
	}

	return nil
}

// processJob handles a single job with cancellation support.
func (q *Queue) processJob(job *SpeakJob) {
	q.mu.Lock()
	handler := q.playbackFunc
	ctx, cancel := context.WithCancel(context.Background())
	q.cancelCurrent = cancel
	q.mu.Unlock()

	defer func() {
		cancel()
		q.mu.Lock()
		q.cancelCurrent = nil
		q.mu.Unlock()
	}()

	if handler == nil {
		q.logger.Warn("no playback handler set, skipping job", "job_id", job.ID)
		return
	}

	q.logger.Info("processing job", "job_id", job.ID, "text_length", len(job.Text))

	if err := handler(ctx, job); err != nil {
		if errors.Is(err, context.Canceled) {
			q.logger.Info("job cancelled", "job_id", job.ID)
		} else {
			q.logger.Error("job failed", "job_id", job.ID, "error", err)
		}
	} else {
		q.logger.Info("job completed", "job_id", job.ID)
	}
}
