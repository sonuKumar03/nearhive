package scraper

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestOrchestrator_SubtasksCreationAndCancellation(t *testing.T) {
	s := store.NewMockStore()
	orch := NewOrchestrator(s, nil, nil, 2)
	jobID := uuid.New()

	blockingScraper := &blockingMockScraper{
		name:   "slow_scraper",
		region: "Bangalore",
	}
	orch.Register(blockingScraper)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := orch.ScrapeRegion(ctx, ScrapeRequest{
		JobID:    jobID,
		Region:   "Bangalore",
		Lat:      12.9716,
		Lng:      77.5946,
		RadiusKM: 10,
	})

	assert.ErrorIs(t, err, context.Canceled)

	// Check that subtasks were created and cancelled
	tasks, getErr := s.GetTasksByJobID(context.Background(), jobID)
	assert.NoError(t, getErr)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "slow_scraper", tasks[0].Source)
	assert.Equal(t, "cancelled", tasks[0].Status)
	assert.Equal(t, "job cancelled", *tasks[0].Error)
}

type blockingMockScraper struct {
	name   string
	region string
}

func (b *blockingMockScraper) Name() string { return b.name }
func (b *blockingMockScraper) Supports(region string) bool { return true }
func (b *blockingMockScraper) Scrape(ctx context.Context, _ ScrapeRequest) (*ScrapeResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return &ScrapeResult{}, nil
	}
}

