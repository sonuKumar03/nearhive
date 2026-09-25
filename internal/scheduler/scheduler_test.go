package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type dummyRunner struct {
	runs int32
}

func (d *dummyRunner) Run(ctx context.Context) error {
	atomic.AddInt32(&d.runs, 1)
	return nil
}

func TestScheduler_ExecutesInterval(t *testing.T) {
	d := &dummyRunner{}
	s := NewScheduler(10*time.Millisecond, d.Run)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(35 * time.Millisecond)
	s.Stop()

	assert.GreaterOrEqual(t, atomic.LoadInt32(&d.runs), int32(2))
}
