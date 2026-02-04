package queue

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dgnsrekt/discorgeous-go/internal/logging"
)

// testTimeout is the maximum time to wait for any test condition.
// This is a failsafe, not primary synchronization.
const testTimeout = 5 * time.Second

func testLogger() *slog.Logger {
	return logging.New("error", "text")
}

func TestQueueEnqueue(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	job := NewSpeakJob("Hello", "default", false, 0, "")
	err := q.Enqueue(job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", q.Len())
	}
}

func TestQueueCapacity(t *testing.T) {
	q := NewQueue(2, 5*time.Minute, testLogger())

	job1 := NewSpeakJob("Hello", "default", false, 0, "")
	job2 := NewSpeakJob("World", "default", false, 0, "")
	job3 := NewSpeakJob("Overflow", "default", false, 0, "")

	if err := q.Enqueue(job1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := q.Enqueue(job2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := q.Enqueue(job3)
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}
}

func TestQueueDeduplication(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	job1 := NewSpeakJob("Hello", "default", false, 0, "same-key")
	job2 := NewSpeakJob("World", "default", false, 0, "same-key")

	if err := q.Enqueue(job1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := q.Enqueue(job2)
	if err != ErrDuplicateJob {
		t.Errorf("expected ErrDuplicateJob, got %v", err)
	}
}

func TestQueueDeduplicationEmptyKey(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	// Jobs with empty dedupe keys should not be deduplicated
	job1 := NewSpeakJob("Hello", "default", false, 0, "")
	job2 := NewSpeakJob("World", "default", false, 0, "")

	if err := q.Enqueue(job1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := q.Enqueue(job2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.Len() != 2 {
		t.Errorf("expected queue length 2, got %d", q.Len())
	}
}

func TestQueueClosed(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())
	q.Start()
	q.Stop()

	job := NewSpeakJob("Hello", "default", false, 0, "")
	err := q.Enqueue(job)
	if err != ErrQueueClosed {
		t.Errorf("expected ErrQueueClosed, got %v", err)
	}
}

func TestQueueInterrupt(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	job1 := NewSpeakJob("Hello", "default", false, 0, "key1")
	job2 := NewSpeakJob("World", "default", false, 0, "key2")

	q.Enqueue(job1)
	q.Enqueue(job2)

	if q.Len() != 2 {
		t.Errorf("expected queue length 2, got %d", q.Len())
	}

	q.Interrupt()

	if q.Len() != 0 {
		t.Errorf("expected queue length 0 after interrupt, got %d", q.Len())
	}

	// Should be able to enqueue with same dedupe keys after interrupt
	job3 := NewSpeakJob("Again", "default", false, 0, "key1")
	if err := q.Enqueue(job3); err != nil {
		t.Fatalf("unexpected error after interrupt: %v", err)
	}
}

func TestWorkerProcessesJobs(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	var processedJobs []string
	var mu sync.Mutex

	// Channel to signal all jobs completed
	allDone := make(chan struct{})
	expectedCount := 3

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		mu.Lock()
		processedJobs = append(processedJobs, job.Text)
		mu.Unlock()
		return nil
	})

	q.SetJobCompletedCallback(func(job *SpeakJob) {
		mu.Lock()
		count := len(processedJobs)
		mu.Unlock()
		if count == expectedCount {
			close(allDone)
		}
	})

	q.Start()
	defer q.Stop()

	q.Enqueue(NewSpeakJob("First", "default", false, 0, ""))
	q.Enqueue(NewSpeakJob("Second", "default", false, 0, ""))
	q.Enqueue(NewSpeakJob("Third", "default", false, 0, ""))

	// Wait for all jobs with timeout failsafe
	select {
	case <-allDone:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for jobs to complete")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(processedJobs) != 3 {
		t.Fatalf("expected 3 processed jobs, got %d", len(processedJobs))
	}

	expected := []string{"First", "Second", "Third"}
	for i, text := range expected {
		if processedJobs[i] != text {
			t.Errorf("expected job %d to be '%s', got '%s'", i, text, processedJobs[i])
		}
	}
}

func TestWorkerSkipsExpiredJobs(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	var processedJobs []string
	var mu sync.Mutex
	validJobDone := make(chan struct{})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		mu.Lock()
		processedJobs = append(processedJobs, job.Text)
		mu.Unlock()
		return nil
	})

	q.SetJobCompletedCallback(func(job *SpeakJob) {
		if job.Text == "Valid" {
			close(validJobDone)
		}
	})

	// Create an already-expired job by setting ExpiresAt in the past
	expiredJob := NewSpeakJob("Expired", "default", false, 1*time.Nanosecond, "")
	// Force expiry by waiting a tiny bit (deterministic: nanosecond TTL guarantees expiry)
	validJob := NewSpeakJob("Valid", "default", false, 0, "")

	q.Enqueue(expiredJob)
	q.Enqueue(validJob)

	q.Start()
	defer q.Stop()

	// Wait for valid job to complete with timeout failsafe
	select {
	case <-validJobDone:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for valid job to complete")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(processedJobs) != 1 {
		t.Fatalf("expected 1 processed job (expired one skipped), got %d", len(processedJobs))
	}

	if processedJobs[0] != "Valid" {
		t.Errorf("expected 'Valid' job to be processed, got '%s'", processedJobs[0])
	}
}

func TestWorkerCancelCurrentJob(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	var cancelled atomic.Bool
	started := make(chan struct{})
	jobDone := make(chan struct{})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		close(started)
		<-ctx.Done()
		cancelled.Store(true)
		return ctx.Err()
	})

	q.SetJobCompletedCallback(func(job *SpeakJob) {
		close(jobDone)
	})

	q.Start()
	defer q.Stop()

	q.Enqueue(NewSpeakJob("Long running", "default", false, 0, ""))

	// Wait for job to start
	select {
	case <-started:
		// Job started
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for job to start")
	}

	// Interrupt should cancel current job
	q.Interrupt()

	// Wait for job completion with timeout failsafe
	select {
	case <-jobDone:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for job cancellation")
	}

	if !cancelled.Load() {
		t.Error("expected job to be cancelled")
	}
}

func TestIdleCallback(t *testing.T) {
	idleTimeout := 50 * time.Millisecond
	q := NewQueue(10, idleTimeout, testLogger())

	idleCalled := make(chan struct{})
	jobDone := make(chan struct{})

	q.SetIdleCallback(func() {
		close(idleCalled)
	})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		return nil
	})

	q.SetJobCompletedCallback(func(job *SpeakJob) {
		close(jobDone)
	})

	q.Start()
	defer q.Stop()

	// Enqueue a job to process
	q.Enqueue(NewSpeakJob("Hello", "default", false, 0, ""))

	// Wait for job to complete
	select {
	case <-jobDone:
		// Job completed
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for job to complete")
	}

	// Wait for idle callback with timeout failsafe
	select {
	case <-idleCalled:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for idle callback")
	}
}

func TestIdleCallbackNotCalledWhileProcessing(t *testing.T) {
	idleTimeout := 20 * time.Millisecond
	q := NewQueue(10, idleTimeout, testLogger())

	var idleCalledDuringProcessing atomic.Bool
	var processingComplete atomic.Bool
	processingDone := make(chan struct{})
	idleCalled := make(chan struct{})
	continueProcessing := make(chan struct{})

	q.SetIdleCallback(func() {
		if !processingComplete.Load() {
			idleCalledDuringProcessing.Store(true)
		}
		select {
		case <-idleCalled:
			// Already closed
		default:
			close(idleCalled)
		}
	})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		// Wait longer than idle timeout, but use a channel for determinism
		<-continueProcessing
		return nil
	})

	q.SetJobCompletedCallback(func(job *SpeakJob) {
		processingComplete.Store(true)
		close(processingDone)
	})

	q.Start()
	defer q.Stop()

	q.Enqueue(NewSpeakJob("Hello", "default", false, 0, ""))

	// Let the idle timeout pass while processing
	time.Sleep(idleTimeout * 3)

	// Now let processing complete
	close(continueProcessing)

	// Wait for processing to complete
	select {
	case <-processingDone:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for processing to complete")
	}

	// Wait for idle callback
	select {
	case <-idleCalled:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for idle callback")
	}

	if idleCalledDuringProcessing.Load() {
		t.Error("idle callback was called during processing")
	}
}

func TestNoPlaybackHandler(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	jobDone := make(chan struct{})

	// Don't set a playback handler, but set completion callback
	q.SetJobCompletedCallback(func(job *SpeakJob) {
		close(jobDone)
	})

	q.Start()
	defer q.Stop()

	q.Enqueue(NewSpeakJob("Hello", "default", false, 0, ""))

	// Wait for job to be processed (skipped) with timeout failsafe
	select {
	case <-jobDone:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for job to be processed")
	}

	// Queue should be empty
	if q.Len() != 0 {
		t.Errorf("expected queue to be empty, got length %d", q.Len())
	}
}

func TestDedupeKeyRemovedAfterProcessing(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	var processedCount atomic.Int32
	firstProcessed := make(chan struct{})
	secondProcessed := make(chan struct{})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		return nil
	})

	q.SetJobCompletedCallback(func(job *SpeakJob) {
		count := processedCount.Add(1)
		if count == 1 {
			close(firstProcessed)
		} else if count == 2 {
			close(secondProcessed)
		}
	})

	q.Start()
	defer q.Stop()

	job1 := NewSpeakJob("Hello", "default", false, 0, "unique-key")
	q.Enqueue(job1)

	// Wait for first job to be processed
	select {
	case <-firstProcessed:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for first job to complete")
	}

	// Should be able to enqueue with same dedupe key after processing
	job2 := NewSpeakJob("World", "default", false, 0, "unique-key")
	if err := q.Enqueue(job2); err != nil {
		t.Fatalf("unexpected error after processing: %v", err)
	}

	// Wait for second job to be processed
	select {
	case <-secondProcessed:
		// Success
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for second job to complete")
	}

	if processedCount.Load() != 2 {
		t.Errorf("expected 2 processed jobs, got %d", processedCount.Load())
	}
}

func TestShutdownCallback(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	var shutdownCalled atomic.Bool

	q.SetShutdownCallback(func() {
		shutdownCalled.Store(true)
	})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		return nil
	})

	q.Start()
	q.Stop()

	if !shutdownCalled.Load() {
		t.Error("shutdown callback was not called")
	}
}

func TestShutdownCallbackCalledAfterWorkerStops(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	var workerStopped atomic.Bool
	var shutdownCalledAfterWorker atomic.Bool
	workerRunning := make(chan struct{})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		// Signal worker is running
		select {
		case <-workerRunning:
		default:
			close(workerRunning)
		}
		// Wait for cancellation
		<-ctx.Done()
		// Add a small sleep to verify shutdown waits for worker
		time.Sleep(10 * time.Millisecond)
		workerStopped.Store(true)
		return ctx.Err()
	})

	q.SetShutdownCallback(func() {
		if workerStopped.Load() {
			shutdownCalledAfterWorker.Store(true)
		}
	})

	q.Start()

	// Enqueue a job
	q.Enqueue(NewSpeakJob("Hello", "default", false, 0, ""))

	// Wait for worker to start processing
	select {
	case <-workerRunning:
		// Worker is running
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for worker to start")
	}

	// Stop should wait for worker and then call shutdown callback
	q.Stop()

	if !shutdownCalledAfterWorker.Load() {
		t.Error("shutdown callback was called before worker stopped")
	}
}
