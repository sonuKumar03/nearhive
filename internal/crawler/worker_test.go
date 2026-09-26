package crawler

import (
	"context"
	"sync"
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

func TestWorkerDaemon_ConcurrentExecution(t *testing.T) {
	s := store.NewMockStore()
	orch := scraper.NewOrchestrator(s, nil, nil, 2)

	job1 := &model.ScrapeJob{ID: uuid.New(), Status: "pending"}
	job2 := &model.ScrapeJob{ID: uuid.New(), Status: "pending"}
	job3 := &model.ScrapeJob{ID: uuid.New(), Status: "pending"}

	jobs := []*model.ScrapeJob{job1, job2, job3}
	completedJobs := make(map[uuid.UUID]bool)
	var mu sync.Mutex

	mockQ := &concurrentMockQueue{
		jobs: jobs,
		onComplete: func(id uuid.UUID) {
			mu.Lock()
			completedJobs[id] = true
			mu.Unlock()
		},
	}

	daemon := NewWorkerDaemon(mockQ, orch, WorkerConfig{
		WorkerID:     "test-worker-concurrent",
		PollInterval: 20 * time.Millisecond,
		JobTimeout:   5 * time.Second,
		Concurrency:  3,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	daemon.Start(ctx)
	time.Sleep(300 * time.Millisecond)
	daemon.Stop()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 3, len(completedJobs))
}

type concurrentMockQueue struct {
	jobs       []*model.ScrapeJob
	onComplete func(uuid.UUID)
	mu         sync.Mutex
}

func (m *concurrentMockQueue) Enqueue(_ context.Context, _ *model.ScrapeJob) error { return nil }
func (m *concurrentMockQueue) Dequeue(_ context.Context, _ string) (*model.ScrapeJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.jobs) == 0 {
		return nil, nil
	}
	job := m.jobs[0]
	m.jobs = m.jobs[1:]
	return job, nil
}
func (m *concurrentMockQueue) Heartbeat(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (m *concurrentMockQueue) Complete(_ context.Context, id uuid.UUID, _ int, _ *string) error {
	if m.onComplete != nil {
		m.onComplete(id)
	}
	return nil
}
func (m *concurrentMockQueue) NotifyCancel(_ context.Context, _ uuid.UUID) error { return nil }
func (m *concurrentMockQueue) StartCancelListener(_ context.Context, _ func(jobID uuid.UUID)) error { return nil }
