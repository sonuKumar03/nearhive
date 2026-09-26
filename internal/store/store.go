package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

var (
	ErrNotFound        = errors.New("record not found")
	ErrConflict        = errors.New("record already exists")
	ErrInvalidJobState = errors.New("job cannot be cancelled in its current state")
)

type SearchOpts struct {
	MinConfidence *float64
	Industry      *string
	Query         *string
	Limit         int
	Offset        int
}

type UserStore interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type CompanyStore interface {
	CreateCompany(ctx context.Context, c *model.Company) error
	GetCompanyByID(ctx context.Context, id uuid.UUID) (*model.Company, error)
	FindByDomain(ctx context.Context, domain string) (*model.Company, error)
	FindByNormalizedName(ctx context.Context, name string) (*model.Company, error)
	FindByFuzzyName(ctx context.Context, name string, threshold float64) (*model.Company, error)
	UpdateCompany(ctx context.Context, c *model.Company) error
}

type LocationStore interface {
	CreateLocation(ctx context.Context, l *model.Location) error
	GetLocationsByCompany(ctx context.Context, companyID uuid.UUID) ([]model.Location, error)
	FindNearbyLocation(ctx context.Context, companyID uuid.UUID, lat, lng float64, radiusMeters float64) (*model.Location, error)
	UpdateLocationConfidence(ctx context.Context, id uuid.UUID, confidence float64) error
	UpdateLocationPresence(ctx context.Context, id uuid.UUID, presence model.PresenceType, confidence float64, verified bool) error
	UpdateLocationCoords(ctx context.Context, id uuid.UUID, lat, lng float64) error
	Search(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) ([]model.CompanySearchResult, error)
	CountSearch(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) (int, error)
	ClusterSearch(ctx context.Context, lat, lng, radiusMeters float64, k int) ([]model.SpatialCluster, error)
}

type SightingStore interface {
	SaveSightings(ctx context.Context, source string, sightings []model.Sighting) error
	GetSightingsByCompany(ctx context.Context, companyID uuid.UUID) ([]model.Sighting, error)
	LinkSighting(ctx context.Context, sightingID uuid.UUID, companyID, locationID uuid.UUID) error
}

type JobStore interface {
	CreateJob(ctx context.Context, job *model.ScrapeJob) error
	UpdateJob(ctx context.Context, job *model.ScrapeJob) error
	GetJobByID(ctx context.Context, id uuid.UUID) (*model.ScrapeJob, error)
	ListJobs(ctx context.Context, limit, offset int) ([]model.ScrapeJob, error)
	CreateTask(ctx context.Context, task *model.ScrapeTask) error
	UpdateTask(ctx context.Context, task *model.ScrapeTask) error
	GetTasksByJobID(ctx context.Context, jobID uuid.UUID) ([]model.ScrapeTask, error)
}

type SearchHistoryStore interface {
	RecordSearch(ctx context.Context, h *model.SearchHistory) error
	GetHistoryByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.SearchHistory, error)
}

type DiscoveryStore interface {
	CreateDiscoveryJob(ctx context.Context, job *model.DiscoveryJob) error
	GetDiscoveryJob(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.DiscoveryJob, error)
	ListDiscoveryJobs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.DiscoveryJob, error)
	CancelDiscoveryJob(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	UpsertDiscoverySourceRun(ctx context.Context, run *model.DiscoverySourceRun) error
	GetDiscoverySourceRuns(ctx context.Context, discoveryJobID uuid.UUID) ([]model.DiscoverySourceRun, error)
	GetLocationEvidenceSummaries(ctx context.Context, companyID uuid.UUID) ([]model.LocationEvidenceSummary, error)
}


type TechnicalJobStore interface {
	GetTechnicalJobsByCompany(ctx context.Context, companyID uuid.UUID, since time.Time) ([]model.TechnicalJobPosting, error)
	UpsertTechnicalJob(ctx context.Context, job *model.TechnicalJobPosting) error
}

type Store interface {
	UserStore
	CompanyStore
	LocationStore
	SightingStore
	JobStore
	SearchHistoryStore
	DiscoveryStore
	TechnicalJobStore
	Close() error
}
