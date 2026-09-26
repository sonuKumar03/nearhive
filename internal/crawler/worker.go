package crawler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/queue"
	"github.com/sonukumar/nearhive/internal/scraper"
)

type WorkerConfig struct {
	WorkerID     string
	PollInterval time.Duration
	JobTimeout   time.Duration
	Concurrency  int
}

type WorkerDaemon struct {
	queue        queue.JobQueue
	orchestrator *scraper.Orchestrator
	taskMgr      *scraper.TaskManager
	cfg          WorkerConfig
	sem          chan struct{}
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewWorkerDaemon(q queue.JobQueue, orch *scraper.Orchestrator, cfg WorkerConfig) *WorkerDaemon {
	if cfg.WorkerID == "" {
		hostname, _ := os.Hostname()
		cfg.WorkerID = fmt.Sprintf("crawler-%s-%d", hostname, time.Now().UnixNano()%10000)
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = 10 * time.Minute
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 3
	}

	tm := orch.TaskManager()
	if tm == nil {
		tm = scraper.NewTaskManager()
	}

	return &WorkerDaemon{
		queue:        q,
		orchestrator: orch,
		taskMgr:      tm,
		cfg:          cfg,
		sem:          make(chan struct{}, cfg.Concurrency),
		stopCh:       make(chan struct{}),
	}
}

func (w *WorkerDaemon) Start(ctx context.Context) {
	log.Printf("🚀 Starting NearHive standalone crawler worker [%s] (concurrency: %d)", w.cfg.WorkerID, w.cfg.Concurrency)

	// Start real-time Postgres cancellation listener in background
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		err := w.queue.StartCancelListener(ctx, func(jobID uuid.UUID) {
			log.Printf("🛑 Received cancellation signal for job: %s", jobID)
			w.taskMgr.Cancel(jobID)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("⚠️ Cancellation listener exited: %v", err)
		}
	}()

	// Start main worker polling loop
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.pollLoop(ctx)
	}()
}

func (w *WorkerDaemon) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.drainAvailableSlots(ctx)
		}
	}
}

func (w *WorkerDaemon) drainAvailableSlots(ctx context.Context) {
	for {
		select {
		case w.sem <- struct{}{}:
			job, err := w.queue.Dequeue(ctx, w.cfg.WorkerID)
			if err != nil {
				<-w.sem
				if !errors.Is(ctx.Err(), context.Canceled) {
					log.Printf("⚠️ Failed to dequeue job: %v", err)
				}
				return
			}
			if job == nil {
				// No pending jobs available right now
				<-w.sem
				return
			}

			// Launch concurrent execution in separate goroutine
			w.wg.Add(1)
			go func(j *model.ScrapeJob) {
				defer func() {
					<-w.sem
					w.wg.Done()
				}()
				w.executeJob(ctx, j)
			}(job)
		default:
			// All concurrency slots occupied
			return
		}
	}
}

func (w *WorkerDaemon) executeJob(ctx context.Context, job *model.ScrapeJob) {
	log.Printf("⚡ Worker [%s] processing job %s (region: %v, coords: %v, %v)",
		w.cfg.WorkerID, job.ID, val(job.Region), valFloat(job.Lat), valFloat(job.Lng))

	jobCtx, cancel := context.WithTimeout(ctx, w.cfg.JobTimeout)
	w.taskMgr.Register(job.ID, cancel)
	defer func() {
		w.taskMgr.Unregister(job.ID)
		cancel()
	}()

	// Start periodic heartbeat goroutine
	stopHeartbeat := make(chan struct{})
	go func() {
		heartbeatTicker := time.NewTicker(10 * time.Second)
		defer heartbeatTicker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-stopHeartbeat:
				return
			case <-heartbeatTicker.C:
				_ = w.queue.Heartbeat(context.Background(), job.ID, w.cfg.WorkerID)
			}
		}
	}()

	lat := 0.0
	lng := 0.0
	radius := 15.0
	region := ""
	if job.Lat != nil {
		lat = *job.Lat
	}
	if job.Lng != nil {
		lng = *job.Lng
	}
	if job.RadiusKM != nil && *job.RadiusKM > 0 {
		radius = *job.RadiusKM
	}
	if job.Region != nil {
		region = *job.Region
	}

	sightings, err := w.orchestrator.ScrapeRegion(jobCtx, scraper.ScrapeRequest{
		JobID:    job.ID,
		Region:   region,
		Lat:      lat,
		Lng:      lng,
		RadiusKM: radius,
	})
	close(stopHeartbeat)

	var errMsg *string
	if errors.Is(jobCtx.Err(), context.Canceled) {
		msg := "job cancelled"
		errMsg = &msg
		log.Printf("🛑 Job %s was cancelled", job.ID)
	} else if err != nil {
		msg := err.Error()
		errMsg = &msg
		log.Printf("❌ Job %s failed: %v", job.ID, err)
	} else {
		log.Printf("✅ Job %s completed successfully with %d sightings", job.ID, len(sightings))
	}

	_ = w.queue.Complete(context.Background(), job.ID, len(sightings), errMsg)
}

func (w *WorkerDaemon) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	log.Printf("💤 Worker [%s] stopped gracefully", w.cfg.WorkerID)
}

func val(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func valFloat(f *float64) string {
	if f == nil {
		return "nil"
	}
	return fmt.Sprintf("%.4f", *f)
}
