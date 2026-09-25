package scraper

import (
	"context"

	"github.com/sonukumar/nearhive/internal/model"
)

type ScrapeRequest struct {
	Region   string
	Lat      float64
	Lng      float64
	RadiusKM float64
}

type ScrapeResult struct {
	Sightings []model.Sighting
	Errors    []error
}

type Scraper interface {
	Name() string
	Supports(region string) bool
	Scrape(ctx context.Context, req ScrapeRequest) (*ScrapeResult, error)
}
