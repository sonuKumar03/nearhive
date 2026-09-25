package scraper

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// TaskManager manages in-flight background jobs and their context cancellations.
type TaskManager struct {
	mu      sync.RWMutex
	cancels map[uuid.UUID]context.CancelFunc
}

// NewTaskManager creates a new thread-safe task manager.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		cancels: make(map[uuid.UUID]context.CancelFunc),
	}
}

// Register stores the cancel function for an active job ID.
func (m *TaskManager) Register(jobID uuid.UUID, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancels[jobID] = cancel
}

// Cancel terminates the context of a running job and unregisters it.
// Returns true if the job was found and cancelled, false otherwise.
func (m *TaskManager) Cancel(jobID uuid.UUID) bool {
	m.mu.Lock()
	cancel, ok := m.cancels[jobID]
	if ok {
		delete(m.cancels, jobID)
	}
	m.mu.Unlock()

	if ok && cancel != nil {
		cancel()
		return true
	}
	return false
}

// Unregister removes the cancel function when a job finishes naturally.
func (m *TaskManager) Unregister(jobID uuid.UUID) {
	m.mu.Lock()
	delete(m.cancels, jobID)
	m.mu.Unlock()
}

// IsRunning reports whether a job ID is currently active in the manager.
func (m *TaskManager) IsRunning(jobID uuid.UUID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.cancels[jobID]
	return ok
}
