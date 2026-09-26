package crawler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/stretchr/testify/assert"
)

type mockQueue struct {
	dequeuedJob *model.ScrapeJob
	completed   bool
	completeStatus string
}

func (m *mockQueue) Enqueue(_ context.Context, _ *model.ScrapeJob) error { return nil }
func (m *mockQueue) Dequeue(_ context.Context, _ string) (*model.ScrapeJob, error) {
	job := m.dequeuedJob
	m.dequeuedJob = nil
	return job, nil
}
func (m *mockQueue) Heartbeat(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (m *mockQueue) Complete(_ context.Context, _ uuid.UUID, _ int, errMsg *string) error {
	m.completed = true
	if errMsg != nil {
		m.completeStatus = *errMsg
	} else {
		m.completeStatus = "done"
	}
	return nil
}
func (m *mockQueue) NotifyCancel(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQueue) StartCancelListener(_ context.Context, _ func(jobID uuid.UUID)) error { return nil }

func TestWorkerDaemon_ProcessJob(t *testing.T) {
	s := store.NewMockStore()
	orch := scraper.NewOrchestrator(s, nil, nil, 2)

	reg := "Bangalore"
	lat := 12.9716
	lng := 77.5946
	r := 10.0
	job := &model.ScrapeJob{
		ID:        uuid.New(),
		Source:    "manual_trigger",
		Status:    "pending",
		Region:    &reg,
		Lat:       &lat,
		Lng:       &lng,
		RadiusKM:  &r,
		CreatedAt: time.Now(),
	}

	mq := &mockQueue{dequeuedJob: job}
	daemon := NewWorkerDaemon(mq, orch, WorkerConfig{
		WorkerID:     "test-worker-1",
		PollInterval: 50 * time.Millisecond,
		JobTimeout:   5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	daemon.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	daemon.Stop()

	assert.True(t, mq.completed)
	assert.Equal(t, "done", mq.completeStatus)
}
