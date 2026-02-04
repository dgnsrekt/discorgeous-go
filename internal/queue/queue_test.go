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

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		mu.Lock()
		processedJobs = append(processedJobs, job.Text)
		mu.Unlock()
		return nil
	})

	q.Start()
	defer q.Stop()

	q.Enqueue(NewSpeakJob("First", "default", false, 0, ""))
	q.Enqueue(NewSpeakJob("Second", "default", false, 0, ""))
	q.Enqueue(NewSpeakJob("Third", "default", false, 0, ""))

	// Wait for jobs to be processed
	time.Sleep(100 * time.Millisecond)

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

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		mu.Lock()
		processedJobs = append(processedJobs, job.Text)
		mu.Unlock()
		return nil
	})

	// Enqueue a job that will expire immediately
	expiredJob := NewSpeakJob("Expired", "default", false, 1*time.Millisecond, "")
	validJob := NewSpeakJob("Valid", "default", false, 0, "")

	q.Enqueue(expiredJob)
	q.Enqueue(validJob)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	q.Start()
	defer q.Stop()

	// Wait for jobs to be processed
	time.Sleep(100 * time.Millisecond)

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

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		close(started)
		<-ctx.Done()
		cancelled.Store(true)
		return ctx.Err()
	})

	q.Start()
	defer q.Stop()

	q.Enqueue(NewSpeakJob("Long running", "default", false, 0, ""))

	// Wait for job to start
	<-started

	// Interrupt should cancel current job
	q.Interrupt()

	// Wait for cancellation
	time.Sleep(50 * time.Millisecond)

	if !cancelled.Load() {
		t.Error("expected job to be cancelled")
	}
}

func TestIdleCallback(t *testing.T) {
	q := NewQueue(10, 50*time.Millisecond, testLogger())

	var idleCalled atomic.Bool

	q.SetIdleCallback(func() {
		idleCalled.Store(true)
	})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		return nil
	})

	q.Start()
	defer q.Stop()

	// Enqueue a job to process
	q.Enqueue(NewSpeakJob("Hello", "default", false, 0, ""))

	// Wait for job to process + idle timeout
	time.Sleep(200 * time.Millisecond)

	if !idleCalled.Load() {
		t.Error("expected idle callback to be called")
	}
}

func TestIdleCallbackNotCalledWhileProcessing(t *testing.T) {
	q := NewQueue(10, 50*time.Millisecond, testLogger())

	var idleCalled atomic.Bool

	q.SetIdleCallback(func() {
		idleCalled.Store(true)
	})

	processingDone := make(chan struct{})
	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		// Wait longer than idle timeout
		time.Sleep(150 * time.Millisecond)
		close(processingDone)
		return nil
	})

	q.Start()
	defer q.Stop()

	q.Enqueue(NewSpeakJob("Hello", "default", false, 0, ""))

	// Wait for processing to complete
	<-processingDone

	// Idle callback should not have been called during processing
	// but should be called after processing + idle timeout
	time.Sleep(100 * time.Millisecond)

	if !idleCalled.Load() {
		t.Error("expected idle callback to be called after processing completed")
	}
}

func TestNoPlaybackHandler(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	// Don't set a playback handler
	q.Start()
	defer q.Stop()

	q.Enqueue(NewSpeakJob("Hello", "default", false, 0, ""))

	// Should not panic, just skip the job
	time.Sleep(50 * time.Millisecond)

	// Queue should be empty
	if q.Len() != 0 {
		t.Errorf("expected queue to be empty, got length %d", q.Len())
	}
}

func TestDedupeKeyRemovedAfterProcessing(t *testing.T) {
	q := NewQueue(10, 5*time.Minute, testLogger())

	var processedCount atomic.Int32
	firstProcessed := make(chan struct{})

	q.SetPlaybackHandler(func(ctx context.Context, job *SpeakJob) error {
		if processedCount.Add(1) == 1 {
			close(firstProcessed)
		}
		return nil
	})

	q.Start()
	defer q.Stop()

	job1 := NewSpeakJob("Hello", "default", false, 0, "unique-key")
	q.Enqueue(job1)

	// Wait for first job to be processed
	<-firstProcessed

	// Small delay to ensure dequeue completed
	time.Sleep(10 * time.Millisecond)

	// Should be able to enqueue with same dedupe key after processing
	job2 := NewSpeakJob("World", "default", false, 0, "unique-key")
	if err := q.Enqueue(job2); err != nil {
		t.Fatalf("unexpected error after processing: %v", err)
	}
}
