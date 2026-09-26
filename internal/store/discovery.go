package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/sonukumar/nearhive/internal/model"
)

// DiscoveryStore implementation on PostgresStore

func (s *PostgresStore) CreateDiscoveryJob(ctx context.Context, job *model.DiscoveryJob) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.Status == "" {
		job.Status = model.DiscoveryStatusPending
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 3
	}
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	query := `
		INSERT INTO discovery_jobs (
			id, user_id, status, lat, lng, radius_km, worker_id,
			lease_expires_at, last_heartbeat_at, attempts, max_attempts,
			error, company_count, job_count, evidence_count,
			started_at, finished_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19
		)
	`
	_, err := s.db.ExecContext(
		ctx, query,
		job.ID, job.UserID, string(job.Status), job.Lat, job.Lng, job.RadiusKM, job.WorkerID,
		job.LeaseExpiresAt, job.LastHeartbeatAt, job.Attempts, job.MaxAttempts,
		job.Error, job.CompanyCount, job.JobCount, job.EvidenceCount,
		job.StartedAt, job.FinishedAt, job.CreatedAt, job.UpdatedAt,
	)
	return err
}

func (s *PostgresStore) GetDiscoveryJob(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.DiscoveryJob, error) {
	var job model.DiscoveryJob
	var query string
	var err error

	if userID != uuid.Nil {
		query = `
			SELECT id, user_id, status, lat, lng, radius_km, worker_id,
			       lease_expires_at, last_heartbeat_at, attempts, max_attempts,
			       error, company_count, job_count, evidence_count,
			       started_at, finished_at, created_at, updated_at
			FROM discovery_jobs
			WHERE id = $1 AND user_id = $2
		`
		err = s.db.GetContext(ctx, &job, query, id, userID)
	} else {
		query = `
			SELECT id, user_id, status, lat, lng, radius_km, worker_id,
			       lease_expires_at, last_heartbeat_at, attempts, max_attempts,
			       error, company_count, job_count, evidence_count,
			       started_at, finished_at, created_at, updated_at
			FROM discovery_jobs
			WHERE id = $1
		`
		err = s.db.GetContext(ctx, &job, query, id)
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	runs, err := s.GetDiscoverySourceRuns(ctx, job.ID)
	if err != nil {
		return nil, err
	}
	job.SourceRuns = runs

	return &job, nil
}

func (s *PostgresStore) ListDiscoveryJobs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.DiscoveryJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var jobs []model.DiscoveryJob
	var err error

	if userID != uuid.Nil {
		query := `
			SELECT id, user_id, status, lat, lng, radius_km, worker_id,
			       lease_expires_at, last_heartbeat_at, attempts, max_attempts,
			       error, company_count, job_count, evidence_count,
			       started_at, finished_at, created_at, updated_at
			FROM discovery_jobs
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		err = s.db.SelectContext(ctx, &jobs, query, userID, limit, offset)
	} else {
		query := `
			SELECT id, user_id, status, lat, lng, radius_km, worker_id,
			       lease_expires_at, last_heartbeat_at, attempts, max_attempts,
			       error, company_count, job_count, evidence_count,
			       started_at, finished_at, created_at, updated_at
			FROM discovery_jobs
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		err = s.db.SelectContext(ctx, &jobs, query, limit, offset)
	}

	if err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = []model.DiscoveryJob{}
	}

	for i := range jobs {
		runs, err := s.GetDiscoverySourceRuns(ctx, jobs[i].ID)
		if err != nil {
			return nil, err
		}
		jobs[i].SourceRuns = runs
	}

	return jobs, nil
}

func (s *PostgresStore) CancelDiscoveryJob(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	var newStatus string
	updateQuery := `
		UPDATE discovery_jobs
		SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1
		  AND (user_id = $2 OR $2 = '00000000-0000-0000-0000-000000000000'::uuid)
		  AND status IN ('pending', 'running')
		RETURNING status
	`
	err := s.db.QueryRowContext(ctx, updateQuery, id, userID).Scan(&newStatus)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// No rows updated. Query to disambiguate ErrNotFound vs idempotent cancel vs ErrInvalidJobState
	var currentStatus string
	var ownerID uuid.UUID
	checkQuery := `SELECT status, user_id FROM discovery_jobs WHERE id = $1`
	err = s.db.QueryRowContext(ctx, checkQuery, id).Scan(&currentStatus, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if userID != uuid.Nil && ownerID != userID {
		return ErrNotFound
	}

	if model.DiscoveryStatus(currentStatus) == model.DiscoveryStatusCancelled {
		return nil
	}

	return ErrInvalidJobState
}

func (s *PostgresStore) UpsertDiscoverySourceRun(ctx context.Context, run *model.DiscoverySourceRun) error {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	now := time.Now()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	run.UpdatedAt = now

	query := `
		INSERT INTO discovery_source_runs (
			id, discovery_job_id, source, source_family, status,
			attempts, company_count, job_count, evidence_count,
			error, duration_ms, started_at, finished_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (discovery_job_id, source) DO UPDATE SET
			status = EXCLUDED.status,
			attempts = EXCLUDED.attempts,
			company_count = EXCLUDED.company_count,
			job_count = EXCLUDED.job_count,
			evidence_count = EXCLUDED.evidence_count,
			error = EXCLUDED.error,
			duration_ms = EXCLUDED.duration_ms,
			started_at = EXCLUDED.started_at,
			finished_at = EXCLUDED.finished_at,
			updated_at = NOW()
	`
	_, err := s.db.ExecContext(
		ctx, query,
		run.ID, run.DiscoveryJobID, run.Source, run.SourceFamily, string(run.Status),
		run.Attempts, run.CompanyCount, run.JobCount, run.EvidenceCount,
		run.Error, run.DurationMS, run.StartedAt, run.FinishedAt, run.CreatedAt, run.UpdatedAt,
	)
	return err
}

func (s *PostgresStore) GetDiscoverySourceRuns(ctx context.Context, discoveryJobID uuid.UUID) ([]model.DiscoverySourceRun, error) {
	var runs []model.DiscoverySourceRun
	query := `
		SELECT id, discovery_job_id, source, source_family, status,
		       attempts, company_count, job_count, evidence_count,
		       error, duration_ms, started_at, finished_at, created_at, updated_at
		FROM discovery_source_runs
		WHERE discovery_job_id = $1
		ORDER BY created_at ASC
	`
	err := s.db.SelectContext(ctx, &runs, query, discoveryJobID)
	if err != nil {
		return nil, err
	}
	if runs == nil {
		runs = []model.DiscoverySourceRun{}
	}
	return runs, nil
}

// TechnicalJobStore implementation on PostgresStore

func (s *PostgresStore) GetTechnicalJobsByCompany(ctx context.Context, companyID uuid.UUID, since time.Time) ([]model.TechnicalJobPosting, error) {
	var jobs []model.TechnicalJobPosting
	var err error

	if since.IsZero() {
		query := `
			SELECT id, company_id, location_id, discovery_job_id, source, source_family, source_job_id,
			       canonical_url, title, normalized_title, description_excerpt, content_hash,
			       location_raw, lat, lng, work_arrangement, publication_state, posted_at,
			       posted_at_confidence, first_seen_at, last_seen_at, is_active,
			       technical_classification, rule_version, classification_reasons, metadata,
			       created_at, updated_at
			FROM technical_job_postings
			WHERE company_id = $1 AND is_active = TRUE
			ORDER BY COALESCE(posted_at, last_seen_at) DESC
		`
		err = s.db.SelectContext(ctx, &jobs, query, companyID)
	} else {
		query := `
			SELECT id, company_id, location_id, discovery_job_id, source, source_family, source_job_id,
			       canonical_url, title, normalized_title, description_excerpt, content_hash,
			       location_raw, lat, lng, work_arrangement, publication_state, posted_at,
			       posted_at_confidence, first_seen_at, last_seen_at, is_active,
			       technical_classification, rule_version, classification_reasons, metadata,
			       created_at, updated_at
			FROM technical_job_postings
			WHERE company_id = $1 AND is_active = TRUE
			  AND ((posted_at IS NOT NULL AND posted_at >= $2) OR (posted_at IS NULL AND last_seen_at >= $2))
			ORDER BY COALESCE(posted_at, last_seen_at) DESC
		`
		err = s.db.SelectContext(ctx, &jobs, query, companyID, since)
	}

	if err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = []model.TechnicalJobPosting{}
	}
	return jobs, nil
}

func (s *PostgresStore) UpsertTechnicalJob(ctx context.Context, job *model.TechnicalJobPosting) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.NormalizedTitle == "" {
		job.NormalizedTitle = strings.ToLower(strings.TrimSpace(job.Title))
	}
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	if job.FirstSeenAt.IsZero() {
		job.FirstSeenAt = now
	}
	if job.LastSeenAt.IsZero() {
		job.LastSeenAt = now
	}

	reasons := pq.StringArray(job.ClassificationReasons)
	if reasons == nil {
		reasons = pq.StringArray{}
	}

	var query string
	if job.SourceJobID != nil && *job.SourceJobID != "" {
		query = `
			INSERT INTO technical_job_postings (
				id, company_id, location_id, discovery_job_id, source, source_family, source_job_id,
				canonical_url, title, normalized_title, description_excerpt, content_hash,
				location_raw, lat, lng, work_arrangement, publication_state, posted_at,
				posted_at_confidence, first_seen_at, last_seen_at, is_active,
				technical_classification, rule_version, classification_reasons, metadata,
				created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7,
				$8, $9, $10, $11, $12,
				$13, $14, $15, $16, $17, $18,
				$19, $20, $21, $22,
				$23, $24, $25, $26,
				$27, $28
			)
			ON CONFLICT (source, source_job_id) WHERE source_job_id IS NOT NULL DO UPDATE SET
				canonical_url = EXCLUDED.canonical_url,
				title = EXCLUDED.title,
				normalized_title = EXCLUDED.normalized_title,
				description_excerpt = EXCLUDED.description_excerpt,
				content_hash = EXCLUDED.content_hash,
				location_raw = EXCLUDED.location_raw,
				lat = EXCLUDED.lat,
				lng = EXCLUDED.lng,
				work_arrangement = EXCLUDED.work_arrangement,
				publication_state = EXCLUDED.publication_state,
				posted_at = EXCLUDED.posted_at,
				posted_at_confidence = EXCLUDED.posted_at_confidence,
				last_seen_at = EXCLUDED.last_seen_at,
				is_active = EXCLUDED.is_active,
				technical_classification = EXCLUDED.technical_classification,
				rule_version = EXCLUDED.rule_version,
				classification_reasons = EXCLUDED.classification_reasons,
				metadata = EXCLUDED.metadata,
				updated_at = NOW()
		`
	} else {
		query = `
			INSERT INTO technical_job_postings (
				id, company_id, location_id, discovery_job_id, source, source_family, source_job_id,
				canonical_url, title, normalized_title, description_excerpt, content_hash,
				location_raw, lat, lng, work_arrangement, publication_state, posted_at,
				posted_at_confidence, first_seen_at, last_seen_at, is_active,
				technical_classification, rule_version, classification_reasons, metadata,
				created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7,
				$8, $9, $10, $11, $12,
				$13, $14, $15, $16, $17, $18,
				$19, $20, $21, $22,
				$23, $24, $25, $26,
				$27, $28
			)
			ON CONFLICT (source, content_hash) WHERE content_hash IS NOT NULL AND source_job_id IS NULL DO UPDATE SET
				canonical_url = EXCLUDED.canonical_url,
				title = EXCLUDED.title,
				normalized_title = EXCLUDED.normalized_title,
				description_excerpt = EXCLUDED.description_excerpt,
				location_raw = EXCLUDED.location_raw,
				lat = EXCLUDED.lat,
				lng = EXCLUDED.lng,
				work_arrangement = EXCLUDED.work_arrangement,
				publication_state = EXCLUDED.publication_state,
				posted_at = EXCLUDED.posted_at,
				posted_at_confidence = EXCLUDED.posted_at_confidence,
				last_seen_at = EXCLUDED.last_seen_at,
				is_active = EXCLUDED.is_active,
				technical_classification = EXCLUDED.technical_classification,
				rule_version = EXCLUDED.rule_version,
				classification_reasons = EXCLUDED.classification_reasons,
				metadata = EXCLUDED.metadata,
				updated_at = NOW()
		`
	}

	_, err := s.db.ExecContext(
		ctx, query,
		job.ID, job.CompanyID, job.LocationID, job.DiscoveryJobID, job.Source, job.SourceFamily, job.SourceJobID,
		job.CanonicalURL, job.Title, job.NormalizedTitle, job.DescriptionExcerpt, job.ContentHash,
		job.LocationRaw, job.Lat, job.Lng, string(job.WorkArrangement), string(job.PublicationState), job.PostedAt,
		job.PostedAtConfidence, job.FirstSeenAt, job.LastSeenAt, job.IsActive,
		job.TechnicalClassification, job.RuleVersion, reasons, job.Metadata,
		job.CreatedAt, job.UpdatedAt,
	)
	return err
}

func (s *PostgresStore) UpdateLocationPresence(ctx context.Context, id uuid.UUID, presence model.PresenceType, confidence float64, verified bool) error {
	query := `UPDATE locations SET presence_type = $1, confidence = $2, verified = $3, updated_at = NOW() WHERE id = $4`
	_, err := s.db.ExecContext(ctx, query, string(presence), confidence, verified, id)
	return err
}

func (s *PostgresStore) GetLocationEvidenceSummaries(ctx context.Context, companyID uuid.UUID) ([]model.LocationEvidenceSummary, error) {
	query := `
		WITH combined_evidence AS (
			SELECT 
				l.id AS location_id,
				COALESCE(NULLIF(s.source_family, ''), s.source) AS source_family,
				CASE 
					WHEN LOWER(COALESCE(NULLIF(s.source_family, ''), s.source)) IN ('job_ats', 'greenhouse', 'lever', 'jobportal') THEN 'job'
					ELSE 'company'
				END AS evidence_type

			FROM locations l
			JOIN sightings s ON (
				s.location_id = l.id 
				OR (s.company_id = l.company_id AND s.lat != 0 AND s.lng != 0 AND ST_DWithin(l.coords, ST_SetSRID(ST_MakePoint(s.lng, s.lat), 4326)::geography, 500))
			)
			WHERE l.company_id = $1

			UNION

			SELECT 
				l.id AS location_id,
				t.source_family AS source_family,
				'job' AS evidence_type
			FROM locations l
			JOIN technical_job_postings t ON (
				t.location_id = l.id 
				OR (t.company_id = l.company_id AND t.lat IS NOT NULL AND t.lng IS NOT NULL AND ST_DWithin(l.coords, ST_SetSRID(ST_MakePoint(t.lng, t.lat), 4326)::geography, 500))
			)
			WHERE l.company_id = $1
		)
		SELECT 
			l.id AS location_id,
			COALESCE(ARRAY_AGG(DISTINCT ce.source_family) FILTER (WHERE ce.source_family IS NOT NULL AND ce.source_family != ''), '{}') AS source_families,
			COALESCE(ARRAY_AGG(DISTINCT ce.evidence_type) FILTER (WHERE ce.evidence_type IS NOT NULL AND ce.evidence_type != ''), '{}') AS evidence_types
		FROM locations l
		LEFT JOIN combined_evidence ce ON ce.location_id = l.id
		WHERE l.company_id = $1
		GROUP BY l.id
	`

	var summaries []model.LocationEvidenceSummary
	err := s.db.SelectContext(ctx, &summaries, query, companyID)
	if err != nil {
		return nil, err
	}
	if summaries == nil {
		summaries = []model.LocationEvidenceSummary{}
	}
	return summaries, nil
}

