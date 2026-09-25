package scraper

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTaskManager_RegisterAndCancel(t *testing.T) {
	tm := NewTaskManager()
	jobID := uuid.New()

	ctx, cancel := context.WithCancel(context.Background())
	tm.Register(jobID, cancel)

	if !tm.IsRunning(jobID) {
		t.Fatalf("expected job %s to be running", jobID)
	}

	cancelled := tm.Cancel(jobID)
	if !cancelled {
		t.Fatalf("expected Cancel to return true")
	}

	select {
	case <-ctx.Done():
		// Context was successfully cancelled!
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected context to be cancelled")
	}

	if tm.IsRunning(jobID) {
		t.Fatalf("expected job %s to no longer be running", jobID)
	}

	// Cancelling again should return false
	if tm.Cancel(jobID) {
		t.Fatalf("expected second Cancel to return false")
	}
}

func TestTaskManager_Unregister(t *testing.T) {
	tm := NewTaskManager()
	jobID := uuid.New()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	tm.Register(jobID, cancel)
	if !tm.IsRunning(jobID) {
		t.Fatalf("expected job to be running")
	}

	tm.Unregister(jobID)
	if tm.IsRunning(jobID) {
		t.Fatalf("expected job to not be running after unregister")
	}
}
