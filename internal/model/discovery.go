package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type DiscoveryStatus string

const (
	DiscoveryStatusPending   DiscoveryStatus = "pending"
	DiscoveryStatusRunning   DiscoveryStatus = "running"
	DiscoveryStatusCompleted DiscoveryStatus = "completed"
	DiscoveryStatusPartial   DiscoveryStatus = "partial"
	DiscoveryStatusFailed    DiscoveryStatus = "failed"
	DiscoveryStatusCancelled DiscoveryStatus = "cancelled"
)

type PresenceType string

const (
	PresenceTypeConfirmedOffice PresenceType = "confirmed_office"
	PresenceTypeProbableOffice  PresenceType = "probable_office"
	PresenceTypeJobLocationOnly PresenceType = "job_location_only"
)

type WorkArrangement string

const (
	WorkArrangementInOffice WorkArrangement = "in_office"
	WorkArrangementOnsite   WorkArrangement = "in_office"
	WorkArrangementHybrid   WorkArrangement = "hybrid"
	WorkArrangementRemote   WorkArrangement = "remote"
	WorkArrangementUnknown  WorkArrangement = "unknown"
)

type PublicationState string

const (
	PublicationStatePostedRecently   PublicationState = "posted_recently"
	PublicationStateObservedRecently PublicationState = "observed_recently"
	PublicationStateStale            PublicationState = "stale"
)

type DiscoveryJob struct {
	ID              uuid.UUID            `db:"id" json:"id"`
	UserID          uuid.UUID            `db:"user_id" json:"user_id"`
	Status          DiscoveryStatus      `db:"status" json:"status"`
	Lat             float64              `db:"lat" json:"lat"`
	Lng             float64              `db:"lng" json:"lng"`
	RadiusKM        float64              `db:"radius_km" json:"radius_km"`
	WorkerID        *string              `db:"worker_id" json:"worker_id,omitempty"`
	LeaseExpiresAt  *time.Time           `db:"lease_expires_at" json:"lease_expires_at,omitempty"`
	LastHeartbeatAt *time.Time           `db:"last_heartbeat_at" json:"last_heartbeat_at,omitempty"`
	Attempts        int                  `db:"attempts" json:"attempts"`
	MaxAttempts     int                  `db:"max_attempts" json:"max_attempts"`
	Error           *string              `db:"error" json:"error,omitempty"`
	CompanyCount    int                  `db:"company_count" json:"company_count"`
	JobCount        int                  `db:"job_count" json:"job_count"`
	EvidenceCount   int                  `db:"evidence_count" json:"evidence_count"`
	StartedAt       *time.Time           `db:"started_at" json:"started_at,omitempty"`
	FinishedAt      *time.Time           `db:"finished_at" json:"finished_at,omitempty"`
	CreatedAt       time.Time            `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time            `db:"updated_at" json:"updated_at"`
	SourceRuns      []DiscoverySourceRun `db:"-" json:"source_runs,omitempty"`
}

type DiscoverySourceRun struct {
	ID             uuid.UUID       `db:"id" json:"id"`
	DiscoveryJobID uuid.UUID       `db:"discovery_job_id" json:"discovery_job_id"`
	Source         string          `db:"source" json:"source"`
	SourceFamily   string          `db:"source_family" json:"source_family"`
	Status         DiscoveryStatus `db:"status" json:"status"`
	Attempts       int             `db:"attempts" json:"attempts"`
	CompanyCount   int             `db:"company_count" json:"company_count"`
	JobCount       int             `db:"job_count" json:"job_count"`
	EvidenceCount  int             `db:"evidence_count" json:"evidence_count"`
	Error          *string         `db:"error" json:"error,omitempty"`
	DurationMS     int64           `db:"duration_ms" json:"duration_ms"`
	StartedAt      *time.Time      `db:"started_at" json:"started_at,omitempty"`
	FinishedAt     *time.Time      `db:"finished_at" json:"finished_at,omitempty"`
	CreatedAt      time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at" json:"updated_at"`
}

type CompanyEvidence struct {
	SourceRecordID string   `json:"source_record_id,omitempty"`
	EvidenceURL    string   `json:"evidence_url,omitempty"`
	Name           string   `json:"name"`
	Domain         *string  `json:"domain,omitempty"`
	Address        string   `json:"address,omitempty"`
	Lat            *float64 `json:"lat,omitempty"`
	Lng            *float64 `json:"lng,omitempty"`
	Phone          *string  `json:"phone,omitempty"`
	ContentHash    string   `json:"content_hash,omitempty"`
	Metadata       JSONMap  `json:"metadata,omitempty"`
}

type TechnicalJobEvidence struct {
	SourceJobID             string           `json:"source_job_id,omitempty"`
	CanonicalURL            string           `json:"canonical_url,omitempty"`
	CompanyName             string           `json:"company_name"`
	CompanyDomain           *string          `json:"company_domain,omitempty"`
	Title                   string           `json:"title"`
	DescriptionExcerpt      *string          `json:"description_excerpt,omitempty"`
	ContentHash             string           `json:"content_hash,omitempty"`
	LocationRaw             string           `json:"location_raw,omitempty"`
	Lat                     *float64         `json:"lat,omitempty"`
	Lng                     *float64         `json:"lng,omitempty"`
	WorkArrangement         WorkArrangement  `json:"work_arrangement"`
	PublicationState        PublicationState `json:"publication_state"`
	PostedAt                *time.Time       `json:"posted_at,omitempty"`
	PostedAtConfidence      float64          `json:"posted_at_confidence"`
	FirstSeenAt             time.Time        `json:"first_seen_at"`
	LastSeenAt              time.Time        `json:"last_seen_at"`
	TechnicalClassification string           `json:"technical_classification"`
	RuleVersion             string           `json:"rule_version"`
	ClassificationReasons   []string         `json:"classification_reasons"`
	Metadata                JSONMap          `json:"metadata,omitempty"`
}

type DiscoveryBatch struct {
	ContractVersion int                    `json:"contract_version"`
	DiscoveryJobID  uuid.UUID              `json:"discovery_job_id"`
	Source          string                 `json:"source"`
	SourceFamily    string                 `json:"source_family"`
	ObservedAt      time.Time              `json:"observed_at"`
	Companies       []CompanyEvidence      `json:"companies"`
	Jobs            []TechnicalJobEvidence `json:"jobs"`
}

type TechnicalJobPosting struct {
	ID                      uuid.UUID        `db:"id" json:"id"`
	CompanyID               uuid.UUID        `db:"company_id" json:"company_id"`
	LocationID              *uuid.UUID       `db:"location_id" json:"location_id,omitempty"`
	DiscoveryJobID          *uuid.UUID       `db:"discovery_job_id" json:"discovery_job_id,omitempty"`
	Source                  string           `db:"source" json:"source"`
	SourceFamily            string           `db:"source_family" json:"source_family"`
	SourceJobID             *string          `db:"source_job_id" json:"source_job_id,omitempty"`
	CanonicalURL            *string          `db:"canonical_url" json:"canonical_url,omitempty"`
	Title                   string           `db:"title" json:"title"`
	NormalizedTitle         string           `db:"normalized_title" json:"normalized_title"`
	DescriptionExcerpt      *string          `db:"description_excerpt" json:"description_excerpt,omitempty"`
	ContentHash             string           `db:"content_hash" json:"content_hash"`
	LocationRaw             *string          `db:"location_raw" json:"location_raw,omitempty"`
	Lat                     *float64         `db:"lat" json:"lat,omitempty"`
	Lng                     *float64         `db:"lng" json:"lng,omitempty"`
	WorkArrangement         WorkArrangement  `db:"work_arrangement" json:"work_arrangement"`
	PublicationState        PublicationState `db:"publication_state" json:"publication_state"`
	PostedAt                *time.Time       `db:"posted_at" json:"posted_at,omitempty"`
	PostedAtConfidence      float64          `db:"posted_at_confidence" json:"posted_at_confidence"`
	FirstSeenAt             time.Time        `db:"first_seen_at" json:"first_seen_at"`
	LastSeenAt              time.Time        `db:"last_seen_at" json:"last_seen_at"`
	IsActive                bool             `db:"is_active" json:"is_active"`
	TechnicalClassification string           `db:"technical_classification" json:"technical_classification"`
	RuleVersion             string           `db:"rule_version" json:"rule_version"`
	ClassificationReasons   pq.StringArray   `db:"classification_reasons" json:"classification_reasons"`
	Metadata                JSONMap          `db:"metadata" json:"metadata"`
	CreatedAt               time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt               time.Time        `db:"updated_at" json:"updated_at"`
}

type LocationEvidenceSummary struct {
	LocationID     uuid.UUID      `db:"location_id" json:"location_id"`
	SourceFamilies pq.StringArray `db:"source_families" json:"source_families"`
	EvidenceTypes  pq.StringArray `db:"evidence_types" json:"evidence_types"`
}

