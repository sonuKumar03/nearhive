package scraper

import (
	"context"
	"testing"
	"time"

	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/stretchr/testify/assert"
)

type mockScraper struct {
	name       string
	region     string
	calledWith ScrapeRequest
	sightings  []model.Sighting
}

func (m *mockScraper) Name() string {
	return m.name
}

func (m *mockScraper) Supports(region string) bool {
	return m.region == "" || m.region == region
}

func (m *mockScraper) Scrape(_ context.Context, req ScrapeRequest) (*ScrapeResult, error) {
	m.calledWith = req
	return &ScrapeResult{Sightings: m.sightings}, nil
}

func TestOrchestrator_FallbackCentroid(t *testing.T) {
	s := store.NewMockStore()
	orch := NewOrchestrator(s, nil, nil, 2)

	ms := &mockScraper{
		name:   "test_scraper",
		region: "Bangalore",
		sightings: []model.Sighting{
			{
				CompanyName: "Test Tech Corp",
				RawAddress:  "Outer Ring Road, Bangalore",
				Lat:         12.9716,
				Lng:         77.5946,
				ScrapedAt:   time.Now(),
			},
		},
	}
	orch.Register(ms)

	// Trigger scrape without lat/lng
	sightings, err := orch.ScrapeRegion(context.Background(), ScrapeRequest{
		Region: "Bangalore",
	})

	assert.NoError(t, err)
	assert.Len(t, sightings, 1)

	// Verify fallback city coordinates were populated in the request to scrapers
	assert.Equal(t, 12.9716, ms.calledWith.Lat)
	assert.Equal(t, 77.5946, ms.calledWith.Lng)
}

func TestOrchestrator_SaveSightings(t *testing.T) {
	s := store.NewMockStore()
	orch := NewOrchestrator(s, nil, nil, 2)

	ms := &mockScraper{
		name:   "test_scraper",
		region: "Hyderabad",
		sightings: []model.Sighting{
			{
				CompanyName: "Cyber Solutions",
				RawAddress:  "HITEC City, Hyderabad",
				Lat:         17.4504,
				Lng:         78.3811,
				ScrapedAt:   time.Now(),
			},
		},
	}
	orch.Register(ms)

	sightings, err := orch.ScrapeRegion(context.Background(), ScrapeRequest{
		Region: "Hyderabad",
	})

	assert.NoError(t, err)
	assert.Len(t, sightings, 1)

	// Ensure sightings were saved to store
	assert.NotEmpty(t, s.Sightings)
}
