package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record already exists")
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
	UpdateLocationCoords(ctx context.Context, id uuid.UUID, lat, lng float64) error
	Search(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) ([]model.CompanySearchResult, error)
	CountSearch(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) (int, error)
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
}

type SearchHistoryStore interface {
	RecordSearch(ctx context.Context, h *model.SearchHistory) error
	GetHistoryByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.SearchHistory, error)
}

type Store interface {
	UserStore
	CompanyStore
	LocationStore
	SightingStore
	JobStore
	SearchHistoryStore
	Close() error
}
