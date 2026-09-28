package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

func (s *PostgresStore) GetTechnicalJobsByCompany(ctx context.Context, companyID uuid.UUID, since time.Time) ([]model.TechnicalJobPosting, error) {
	var jobs []model.TechnicalJobPosting
	query := `
		SELECT j.id, j.company_id, j.discovery_job_id, j.source, j.source_family, j.source_job_id,
		       j.canonical_url, j.title, j.normalized_title, j.description_excerpt, j.content_hash,
		       COALESCE(jl.location_raw, '') AS location_raw, jl.latitude AS lat, jl.longitude AS lng,
		       j.work_arrangement, j.publication_state, j.posted_at,
		       j.posted_at_confidence, j.first_seen_at, j.last_seen_at, j.is_active,
		       j.technical_classification, j.rule_version, j.classification_reasons, j.metadata,
		       j.created_at, j.updated_at
		FROM technical_job_postings j
		LEFT JOIN LATERAL (
		    SELECT location_raw, latitude, longitude FROM job_locations
		    WHERE job_id = j.id AND is_active ORDER BY ordinal LIMIT 1
		) jl ON TRUE
		WHERE j.company_id = $1 AND j.is_active = TRUE
		  AND ($2::timestamptz IS NULL OR COALESCE(j.posted_at, j.last_seen_at) >= $2)
		ORDER BY COALESCE(j.posted_at, j.last_seen_at) DESC`
	var filter any
	if !since.IsZero() {
		filter = since
	}
	err := s.db.SelectContext(ctx, &jobs, query, companyID, filter)

	if err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = []model.TechnicalJobPosting{}
	}
	return jobs, nil
}

func (s *PostgresStore) SearchTechnicalJobs(ctx context.Context, lat, lng, radiusMeters float64, opts TechnicalJobSearchOpts) ([]model.TechnicalJobSearchResult, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	var arrangementStr *string
	if opts.WorkArrangement != nil {
		str := string(*opts.WorkArrangement)
		arrangementStr = &str
	}

	query := `
		SELECT 
			tj.id,
			tj.company_id,
			c.name AS company_name,
			c.domain AS company_domain,
			tj.title,
			tj.normalized_title,
			tj.description_excerpt,
			tj.canonical_url,
			tj.source,
			tj.source_family,
			jl.location_raw,
			jl.latitude AS lat,
			jl.longitude AS lng,
			jl.distance_m,
			tj.work_arrangement,
			tj.publication_state,
			tj.posted_at,
			tj.posted_at_confidence,
			tj.first_seen_at,
			tj.last_seen_at,
			tj.metadata
		FROM technical_job_postings tj
		JOIN companies c ON c.id = tj.company_id
		JOIN LATERAL (
			SELECT location_raw, latitude, longitude,
				ST_Distance(
					ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography,
					ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
				) AS distance_m
			FROM job_locations
			WHERE job_id = tj.id AND is_active = TRUE
			  AND latitude IS NOT NULL AND longitude IS NOT NULL
			  AND ST_DWithin(
				ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
				$3
			  )
			ORDER BY distance_m ASC
			LIMIT 1
		) jl ON TRUE
		WHERE tj.is_active = TRUE
		  AND tj.work_arrangement <> 'remote'
		  AND tj.publication_state = 'posted_recently'
		  AND tj.posted_at IS NOT NULL
		  AND tj.posted_at >= NOW() - INTERVAL '14 days'
		  AND tj.posted_at_confidence > 0
		  AND ($4::text IS NULL OR tj.work_arrangement = $4)
		  AND (
			  $5::text IS NULL
			  OR tj.title ILIKE '%' || $5 || '%'
			  OR tj.normalized_title % $5
			  OR c.name ILIKE '%' || $5 || '%'
		  )
		ORDER BY distance_m ASC, tj.posted_at DESC
		LIMIT $6 OFFSET $7
	`

	var results []model.TechnicalJobSearchResult
	err := s.db.SelectContext(ctx, &results, query, lng, lat, radiusMeters, arrangementStr, opts.Query, limit, offset)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []model.TechnicalJobSearchResult{}
	}
	return results, nil
}

func (s *PostgresStore) CountTechnicalJobSearch(ctx context.Context, lat, lng, radiusMeters float64, opts TechnicalJobSearchOpts) (int, error) {
	var arrangementStr *string
	if opts.WorkArrangement != nil {
		str := string(*opts.WorkArrangement)
		arrangementStr = &str
	}

	query := `
		SELECT COUNT(*)
		FROM technical_job_postings tj
		JOIN companies c ON c.id = tj.company_id
		WHERE tj.is_active = TRUE
		  AND tj.work_arrangement <> 'remote'
		  AND tj.publication_state = 'posted_recently'
		  AND tj.posted_at IS NOT NULL
		  AND tj.posted_at >= NOW() - INTERVAL '14 days'
		  AND tj.posted_at_confidence > 0
		  AND EXISTS (
			  SELECT 1 FROM job_locations jl
			  WHERE jl.job_id = tj.id AND jl.is_active = TRUE
				AND jl.latitude IS NOT NULL AND jl.longitude IS NOT NULL
				AND ST_DWithin(
					ST_SetSRID(ST_MakePoint(jl.longitude, jl.latitude), 4326)::geography,
					ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
					$3
				)
		  )
		  AND ($4::text IS NULL OR tj.work_arrangement = $4)
		  AND (
			  $5::text IS NULL
			  OR tj.title ILIKE '%' || $5 || '%'
			  OR tj.normalized_title % $5
			  OR c.name ILIKE '%' || $5 || '%'
		  )
	`

	var count int
	err := s.db.GetContext(ctx, &count, query, lng, lat, radiusMeters, arrangementStr, opts.Query)
	return count, err
}
