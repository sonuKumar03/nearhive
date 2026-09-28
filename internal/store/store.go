package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record already exists")
	ErrInvalidJobState = errors.New("job cannot be cancelled in its current state")
)

type UserStore interface {
	CreateUser(context.Context, *model.User) error
	GetUserByEmail(context.Context, string) (*model.User, error)
	GetUserByID(context.Context, uuid.UUID) (*model.User, error)
}

type SearchOpts struct {
	MinConfidence *float64
	Industry      *string
	Query         *string
	Limit         int
	Offset        int
}

type TechnicalJobSearchOpts struct {
	Query           *string
	WorkArrangement *model.WorkArrangement
	Limit           int
	Offset          int
}

// Store exposes account writes and discovery reads to the Go API.
type Store interface {
	UserStore
	GetCompanyByID(context.Context, uuid.UUID) (*model.Company, error)
	GetLocationsByCompany(context.Context, uuid.UUID) ([]model.Location, error)
	Search(context.Context, float64, float64, float64, SearchOpts) ([]model.CompanySearchResult, error)
	CountSearch(context.Context, float64, float64, float64, SearchOpts) (int, error)
	ClusterSearch(context.Context, float64, float64, float64, int) ([]model.SpatialCluster, error)
	GetSightingsByCompany(context.Context, uuid.UUID) ([]model.Sighting, error)
	GetTechnicalJobsByCompany(context.Context, uuid.UUID, time.Time) ([]model.TechnicalJobPosting, error)
	SearchTechnicalJobs(context.Context, float64, float64, float64, TechnicalJobSearchOpts) ([]model.TechnicalJobSearchResult, error)
	CountTechnicalJobSearch(context.Context, float64, float64, float64, TechnicalJobSearchOpts) (int, error)
	Close() error
}
