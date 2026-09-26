package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/sonukumar/nearhive/internal/model"
)

const CancelChannel = "scrape_job_events"

type CancelEvent struct {
	JobID  uuid.UUID `json:"job_id"`
	Action string    `json:"action"` // "cancel"
}

type JobQueue interface {
	Enqueue(ctx context.Context, job *model.ScrapeJob) error
	Dequeue(ctx context.Context, workerID string) (*model.ScrapeJob, error)
	Heartbeat(ctx context.Context, jobID uuid.UUID, workerID string) error
	Complete(ctx context.Context, jobID uuid.UUID, sightings int, errMsg *string) error
	NotifyCancel(ctx context.Context, jobID uuid.UUID) error
	StartCancelListener(ctx context.Context, onCancel func(jobID uuid.UUID)) error
}

type PostgresQueue struct {
	db      *sqlx.DB
	connStr string
}

func NewPostgresQueue(db *sqlx.DB, connStr string) *PostgresQueue {
	return &PostgresQueue{
		db:      db,
		connStr: connStr,
	}
}

func (q *PostgresQueue) Enqueue(ctx context.Context, job *model.ScrapeJob) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	job.Status = "pending"

	query := `INSERT INTO scrape_jobs (id, source, status, region, sightings, error, lat, lng, radius_km, worker_id, last_heartbeat_at, attempts, started_at, finished_at, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`
	_, err := q.db.ExecContext(ctx, query, job.ID, job.Source, job.Status, job.Region, job.Sightings, job.Error, job.Lat, job.Lng, job.RadiusKM, job.WorkerID, job.LastHeartbeatAt, job.Attempts, job.StartedAt, job.FinishedAt, job.CreatedAt)
	return err
}

func (q *PostgresQueue) Dequeue(ctx context.Context, workerID string) (*model.ScrapeJob, error) {
	query := `
	WITH next_job AS (
		SELECT id
		FROM scrape_jobs
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	)
	UPDATE scrape_jobs
	SET status = 'running',
		started_at = NOW(),
		worker_id = $1,
		last_heartbeat_at = NOW(),
		attempts = attempts + 1
	FROM next_job
	WHERE scrape_jobs.id = next_job.id
	RETURNING scrape_jobs.id, scrape_jobs.source, scrape_jobs.status, scrape_jobs.region,
	          scrape_jobs.sightings, scrape_jobs.error, scrape_jobs.lat, scrape_jobs.lng,
	          scrape_jobs.radius_km, scrape_jobs.worker_id, scrape_jobs.last_heartbeat_at,
	          scrape_jobs.attempts, scrape_jobs.started_at, scrape_jobs.finished_at,
	          scrape_jobs.created_at;`

	var job model.ScrapeJob
	err := q.db.GetContext(ctx, &job, query, workerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (q *PostgresQueue) Heartbeat(ctx context.Context, jobID uuid.UUID, workerID string) error {
	query := `UPDATE scrape_jobs SET last_heartbeat_at = NOW() WHERE id = $1 AND worker_id = $2 AND status = 'running'`
	_, err := q.db.ExecContext(ctx, query, jobID, workerID)
	return err
}

func (q *PostgresQueue) Complete(ctx context.Context, jobID uuid.UUID, sightings int, errMsg *string) error {
	status := "done"
	if errMsg != nil {
		if *errMsg == "job cancelled" || *errMsg == "context canceled" {
			status = "cancelled"
		} else {
			status = "failed"
		}
	}
	query := `UPDATE scrape_jobs
	          SET status = $1, sightings = $2, error = $3, finished_at = NOW()
	          WHERE id = $4 AND status != 'cancelled'`
	_, err := q.db.ExecContext(ctx, query, status, sightings, errMsg, jobID)
	return err
}

func (q *PostgresQueue) NotifyCancel(ctx context.Context, jobID uuid.UUID) error {
	payload, err := json.Marshal(CancelEvent{
		JobID:  jobID,
		Action: "cancel",
	})
	if err != nil {
		return err
	}
	query := fmt.Sprintf("SELECT pg_notify('%s', $1)", CancelChannel)
	_, err = q.db.ExecContext(ctx, query, string(payload))
	return err
}

func (q *PostgresQueue) StartCancelListener(ctx context.Context, onCancel func(jobID uuid.UUID)) error {
	if q.connStr == "" {
		return errors.New("empty connection string for postgres cancel listener")
	}

	listener := pq.NewListener(q.connStr, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("⚠️ PostgreSQL listener event error: %v", err)
		}
	})

	if err := listener.Listen(CancelChannel); err != nil {
		return fmt.Errorf("failed to listen on channel %s: %w", CancelChannel, err)
	}
	defer listener.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case notification, ok := <-listener.Notify:
			if !ok || notification == nil {
				continue
			}
			var ev CancelEvent
			if err := json.Unmarshal([]byte(notification.Extra), &ev); err == nil {
				if ev.Action == "cancel" && ev.JobID != uuid.Nil {
					onCancel(ev.JobID)
				}
			}
		case <-time.After(30 * time.Second):
			// Ping listener to keep alive and check connection health
			if err := listener.Ping(); err != nil {
				log.Printf("⚠️ PostgreSQL listener ping failed: %v", err)
			}
		}
	}
}
