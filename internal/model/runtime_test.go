package model

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{0, "0s"},
		{500 * time.Millisecond, "0s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{65 * time.Second, "1m 5s"},
		{120 * time.Second, "2m"},
		{125 * time.Second, "2m 5s"},
		{3600 * time.Second, "1h"},
		{3665 * time.Second, "1h 1m 5s"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.d)
		if got != tt.expected {
			t.Errorf("FormatDuration(%v) = %q; want %q", tt.d, got, tt.expected)
		}
	}
}

func TestDiscoveryJobComputeRuntime(t *testing.T) {
	now := time.Now()
	started := now.Add(-30 * time.Second)
	finished := now.Add(-10 * time.Second)

	job := &DiscoveryJob{
		ID:        uuid.New(),
		Status:    DiscoveryStatusRunning,
		StartedAt: &started,
		CreatedAt: now.Add(-40 * time.Second),
		SourceRuns: []DiscoverySourceRun{
			{
				ID:         uuid.New(),
				Status:     DiscoveryStatusCompleted,
				DurationMS: 5000,
			},
		},
	}

	job.ComputeRuntime()

	if job.DurationMS < 29000 || job.DurationMS > 32000 {
		t.Errorf("expected ~30000ms, got %d", job.DurationMS)
	}
	if !strings.HasPrefix(job.DurationText, "Running for ") {
		t.Errorf("expected prefix 'Running for ', got %q", job.DurationText)
	}
	if len(job.SourceRuns) != 1 || job.SourceRuns[0].DurationText != "Took 5s" {
		t.Errorf("expected SourceRun duration 'Took 5s', got %q", job.SourceRuns[0].DurationText)
	}

	// Test completed status
	job.Status = DiscoveryStatusCompleted
	job.FinishedAt = &finished
	job.ComputeRuntime()

	if job.DurationMS != 20000 {
		t.Errorf("expected 20000ms, got %d", job.DurationMS)
	}
	if job.DurationText != "Took 20s" {
		t.Errorf("expected 'Took 20s', got %q", job.DurationText)
	}
}

func TestScrapeJobComputeRuntime(t *testing.T) {
	now := time.Now()
	started := now.Add(-45 * time.Second)

	job := &ScrapeJob{
		ID:        uuid.New(),
		Status:    "running",
		StartedAt: &started,
		CreatedAt: now.Add(-50 * time.Second),
		Tasks: []ScrapeTask{
			{
				ID:         uuid.New(),
				Status:     "running",
				StartedAt:  &started,
				CreatedAt:  now.Add(-45 * time.Second),
				DurationMS: 0,
			},
		},
	}

	job.ComputeRuntime()

	if !strings.HasPrefix(job.DurationText, "Running for ") {
		t.Errorf("expected 'Running for ...', got %q", job.DurationText)
	}
	if !strings.HasPrefix(job.Tasks[0].DurationText, "Running for ") {
		t.Errorf("expected 'Running for ...' in task, got %q", job.Tasks[0].DurationText)
	}

	// Test pending status
	job.Status = "pending"
	job.StartedAt = nil
	job.CreatedAt = now.Add(-12 * time.Second)
	job.ComputeRuntime()

	if !strings.HasPrefix(job.DurationText, "Queued for ") {
		t.Errorf("expected 'Queued for ...', got %q", job.DurationText)
	}
}
