package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type JSONMap map[string]any

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(src any) error {
	if src == nil {
		*j = make(map[string]any)
		return nil
	}
	switch s := src.(type) {
	case []byte:
		return json.Unmarshal(s, j)
	case string:
		return json.Unmarshal([]byte(s), j)
	default:
		return errors.New("cannot scan type into JSONMap")
	}
}

type User struct {
	ID        uuid.UUID `db:"id" json:"id"`
	Email     string    `db:"email" json:"email"`
	Password  string    `db:"password" json:"-"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Company struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Name           string    `db:"name" json:"name"`
	NormalizedName string    `db:"normalized_name" json:"normalized_name"`
	Domain         *string   `db:"domain" json:"domain,omitempty"`
	Industry       *string   `db:"industry" json:"industry,omitempty"`
	EmployeeCount  *string   `db:"employee_count" json:"employee_count,omitempty"`
	Description    *string   `db:"description" json:"description,omitempty"`
	Verified       bool      `db:"verified" json:"verified"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

type Location struct {
	ID           uuid.UUID    `db:"id" json:"id"`
	CompanyID    uuid.UUID    `db:"company_id" json:"company_id"`
	Label        *string      `db:"label" json:"label,omitempty"`
	Address      string       `db:"address" json:"address"`
	City         *string      `db:"city" json:"city,omitempty"`
	State        *string      `db:"state" json:"state,omitempty"`
	Country      string       `db:"country" json:"country"`
	Pincode      *string      `db:"pincode" json:"pincode,omitempty"`
	Lat          float64      `db:"lat" json:"lat"`
	Lng          float64      `db:"lng" json:"lng"`
	Confidence   float64      `db:"confidence" json:"confidence"`
	PresenceType PresenceType `db:"presence_type" json:"presence_type"`
	Verified     bool         `db:"verified" json:"verified"`
	CreatedAt    time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at" json:"updated_at"`
}

type Sighting struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	Source         string     `db:"source" json:"source"`
	SourceFamily   string     `db:"source_family" json:"source_family,omitempty"`
	SourceRecordID *string    `db:"source_record_id" json:"source_record_id,omitempty"`
	ContentHash    *string    `db:"content_hash" json:"content_hash,omitempty"`
	DiscoveryJobID *uuid.UUID `db:"discovery_job_id" json:"discovery_job_id,omitempty"`
	SourceURL      *string    `db:"source_url" json:"source_url,omitempty"`
	CompanyName    string     `db:"company_name" json:"company_name"`
	RawAddress     string     `db:"raw_address" json:"raw_address"`
	Lat            float64    `db:"lat" json:"lat"`
	Lng            float64    `db:"lng" json:"lng"`
	Metadata       JSONMap    `db:"metadata" json:"metadata"`
	CompanyID      *uuid.UUID `db:"company_id" json:"company_id,omitempty"`
	LocationID     *uuid.UUID `db:"location_id" json:"location_id,omitempty"`
	FirstSeenAt    time.Time  `db:"first_seen_at" json:"first_seen_at,omitempty"`
	LastSeenAt     time.Time  `db:"last_seen_at" json:"last_seen_at,omitempty"`
	ScrapedAt      time.Time  `db:"scraped_at" json:"scraped_at"`
}


type ScrapeTask struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	JobID          uuid.UUID  `db:"job_id" json:"job_id"`
	Source         string     `db:"source" json:"source"`
	Status         string     `db:"status" json:"status"`
	Sightings      int        `db:"sightings" json:"sightings"`
	Error          *string    `db:"error" json:"error,omitempty"`
	DurationMS     int64      `db:"duration_ms" json:"duration_ms"`
	ElapsedSeconds int64      `db:"-" json:"elapsed_seconds"`
	DurationText   string     `db:"-" json:"duration_text"`
	StartedAt      *time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt     *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
}

type ScrapeJob struct {
	ID              uuid.UUID    `db:"id" json:"id"`
	Source          string       `db:"source" json:"source"`
	Status          string       `db:"status" json:"status"`
	Region          *string      `db:"region" json:"region,omitempty"`
	Sightings       int          `db:"sightings" json:"sightings"`
	Error           *string      `db:"error" json:"error,omitempty"`
	Lat             *float64     `db:"lat" json:"lat,omitempty"`
	Lng             *float64     `db:"lng" json:"lng,omitempty"`
	RadiusKM        *float64     `db:"radius_km" json:"radius_km,omitempty"`
	WorkerID        *string      `db:"worker_id" json:"worker_id,omitempty"`
	LastHeartbeatAt *time.Time   `db:"last_heartbeat_at" json:"last_heartbeat_at,omitempty"`
	Attempts        int          `db:"attempts" json:"attempts"`
	StartedAt       *time.Time   `db:"started_at" json:"started_at,omitempty"`
	FinishedAt      *time.Time   `db:"finished_at" json:"finished_at,omitempty"`
	CreatedAt       time.Time    `db:"created_at" json:"created_at"`
	DurationMS      int64        `db:"-" json:"duration_ms"`
	ElapsedSeconds  int64        `db:"-" json:"elapsed_seconds"`
	DurationText    string       `db:"-" json:"duration_text"`
	Tasks           []ScrapeTask `db:"-" json:"tasks,omitempty"`
}

type SearchHistory struct {
	ID          uuid.UUID `db:"id" json:"id"`
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	QueryLat    float64   `db:"query_lat" json:"query_lat"`
	QueryLng    float64   `db:"query_lng" json:"query_lng"`
	RadiusKM    float64   `db:"radius_km" json:"radius_km"`
	ResultCount int       `db:"result_count" json:"result_count"`
	SearchedAt  time.Time `db:"searched_at" json:"searched_at"`
}

type CompanySearchResult struct {
	CompanyID               uuid.UUID         `db:"company_id" json:"id"`
	Name                    string            `db:"name" json:"name"`
	Domain                  *string           `db:"domain" json:"domain,omitempty"`
	Industry                *string           `db:"industry" json:"industry,omitempty"`
	EmployeeCount           *string           `db:"employee_count" json:"employee_count,omitempty"`
	LocationID              uuid.UUID         `db:"location_id" json:"location_id"`
	Label                   *string           `db:"label" json:"label,omitempty"`
	Address                 string            `db:"address" json:"address"`
	City                    *string           `db:"city" json:"city,omitempty"`
	Lat                     float64           `db:"lat" json:"lat"`
	Lng                     float64           `db:"lng" json:"lng"`
	Confidence              float64           `db:"confidence" json:"confidence"`
	DistanceMeters          float64           `db:"distance_m" json:"distance_meters"`
	PresenceType            PresenceType      `db:"presence_type" json:"presence_type"`
	RecentTechnicalJobCount int               `db:"recent_technical_job_count" json:"recent_technical_job_count"`
	Arrangements            []WorkArrangement `db:"arrangements" json:"arrangements,omitempty"`
	Verified                bool              `db:"verified" json:"verified"`
}

type SpatialCluster struct {
	ClusterID int     `db:"cluster_id" json:"cluster_id"`
	Count     int     `db:"count" json:"count"`
	Lat       float64 `db:"lat" json:"lat"`
	Lng       float64 `db:"lng" json:"lng"`
}

// ComputeRuntime populates DurationMS, ElapsedSeconds, and DurationText for ScrapeJob and its Tasks.
func (j *ScrapeJob) ComputeRuntime() {
	now := time.Now()
	var d time.Duration

	switch j.Status {
	case "running":
		if j.StartedAt != nil {
			d = now.Sub(*j.StartedAt)
		} else {
			d = now.Sub(j.CreatedAt)
		}
		j.DurationMS = d.Milliseconds()
		j.ElapsedSeconds = int64(d.Seconds())
		j.DurationText = "Running for " + FormatDuration(d)

	case "pending":
		d = now.Sub(j.CreatedAt)
		j.DurationMS = d.Milliseconds()
		j.ElapsedSeconds = int64(d.Seconds())
		j.DurationText = "Queued for " + FormatDuration(d)

	case "done", "completed":
		if j.FinishedAt != nil && j.StartedAt != nil {
			d = j.FinishedAt.Sub(*j.StartedAt)
		} else if j.FinishedAt != nil {
			d = j.FinishedAt.Sub(j.CreatedAt)
		} else {
			d = time.Since(j.CreatedAt)
		}
		j.DurationMS = d.Milliseconds()
		j.ElapsedSeconds = int64(d.Seconds())
		j.DurationText = "Took " + FormatDuration(d)

	case "failed":
		if j.FinishedAt != nil && j.StartedAt != nil {
			d = j.FinishedAt.Sub(*j.StartedAt)
		} else if j.FinishedAt != nil {
			d = j.FinishedAt.Sub(j.CreatedAt)
		} else {
			d = time.Since(j.CreatedAt)
		}
		j.DurationMS = d.Milliseconds()
		j.ElapsedSeconds = int64(d.Seconds())
		j.DurationText = "Failed after " + FormatDuration(d)

	case "cancelled":
		if j.FinishedAt != nil && j.StartedAt != nil {
			d = j.FinishedAt.Sub(*j.StartedAt)
		} else if j.FinishedAt != nil {
			d = j.FinishedAt.Sub(j.CreatedAt)
		} else {
			d = time.Since(j.CreatedAt)
		}
		j.DurationMS = d.Milliseconds()
		j.ElapsedSeconds = int64(d.Seconds())
		j.DurationText = "Cancelled after " + FormatDuration(d)

	default:
		d = now.Sub(j.CreatedAt)
		j.DurationMS = d.Milliseconds()
		j.ElapsedSeconds = int64(d.Seconds())
		j.DurationText = FormatDuration(d)
	}

	for i := range j.Tasks {
		j.Tasks[i].ComputeRuntime()
	}
}

// ComputeRuntime populates DurationMS, ElapsedSeconds, and DurationText for ScrapeTask.
func (t *ScrapeTask) ComputeRuntime() {
	now := time.Now()
	var d time.Duration
	if t.DurationMS > 0 {
		d = time.Duration(t.DurationMS) * time.Millisecond
	} else if t.FinishedAt != nil && t.StartedAt != nil {
		d = t.FinishedAt.Sub(*t.StartedAt)
	} else if t.StartedAt != nil {
		d = now.Sub(*t.StartedAt)
	} else {
		d = now.Sub(t.CreatedAt)
	}

	t.DurationMS = d.Milliseconds()
	t.ElapsedSeconds = int64(d.Seconds())

	switch t.Status {
	case "running":
		t.DurationText = "Running for " + FormatDuration(d)
	case "pending":
		t.DurationText = "Queued for " + FormatDuration(d)
	case "done", "completed":
		t.DurationText = "Took " + FormatDuration(d)
	case "failed":
		t.DurationText = "Failed after " + FormatDuration(d)
	case "cancelled":
		t.DurationText = "Cancelled after " + FormatDuration(d)
	default:
		t.DurationText = FormatDuration(d)
	}
}

