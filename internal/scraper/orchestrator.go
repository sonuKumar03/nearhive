package scraper

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/sonukumar/nearhive/internal/verifier"
)

type Orchestrator struct {
	scrapers    []Scraper
	store       store.Store
	geocoder    geocoder.Geocoder
	verifier    *verifier.Engine
	rateLimiter *RateLimiterRegistry
	maxWorkers  int
}

func NewOrchestrator(s store.Store, g geocoder.Geocoder, v *verifier.Engine, maxWorkers int) *Orchestrator {
	if maxWorkers <= 0 {
		maxWorkers = 5
	}
	return &Orchestrator{
		store:       s,
		geocoder:    g,
		verifier:    v,
		rateLimiter: NewRateLimiterRegistry(),
		maxWorkers:  maxWorkers,
	}
}

func (o *Orchestrator) Register(s Scraper) {
	o.scrapers = append(o.scrapers, s)
}

func (o *Orchestrator) ScrapeRegion(ctx context.Context, req ScrapeRequest) ([]model.Sighting, error) {
	// Dynamically geocode region centroid if not explicitly provided
	if req.Lat == 0 && req.Lng == 0 && req.Region != "" {
		if o.geocoder != nil {
			if geo, err := o.geocoder.Geocode(ctx, req.Region); err == nil && geo != nil && geo.Lat != 0 {
				req.Lat = geo.Lat
				req.Lng = geo.Lng
			}
		}
		// Graceful fallback to known major tech hubs if geocoding is unavailable or rate-limited
		if req.Lat == 0 && req.Lng == 0 {
			req.Lat, req.Lng = getFallbackCityCoordinates(req.Region)
		}
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		sightings []model.Sighting
		sem       = make(chan struct{}, o.maxWorkers)
	)

	for _, s := range o.scrapers {
		if !s.Supports(req.Region) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}

		go func(scraper Scraper) {
			defer wg.Done()
			defer func() { <-sem }()

			_ = o.rateLimiter.Wait(ctx, scraper.Name())
			res, err := scraper.Scrape(ctx, req)
			if err != nil || res == nil {
				return
			}

			for i := range res.Sightings {
				if res.Sightings[i].ID == uuid.Nil {
					res.Sightings[i].ID = uuid.New()
				}
				if (res.Sightings[i].Lat == 0 || res.Sightings[i].Lng == 0) && o.geocoder != nil {
					if geo, err := o.geocoder.Geocode(ctx, res.Sightings[i].RawAddress); err == nil && geo != nil {
						res.Sightings[i].Lat = geo.Lat
						res.Sightings[i].Lng = geo.Lng
					}
				}
			}

			if o.store != nil {
				_ = o.store.SaveSightings(ctx, scraper.Name(), res.Sightings)
			}

			if o.verifier != nil {
				for i := range res.Sightings {
					_ = o.verifier.ProcessSighting(ctx, res.Sightings[i])
				}
			}

			mu.Lock()
			sightings = append(sightings, res.Sightings...)
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	return sightings, nil
}

func getFallbackCityCoordinates(region string) (float64, float64) {
	switch strings.ToLower(strings.TrimSpace(region)) {
	case "bangalore", "bengaluru":
		return 12.9716, 77.5946
	case "hyderabad", "cyberabad":
		return 17.3850, 78.4867
	case "pune":
		return 18.5204, 73.8567
	case "chennai", "madras":
		return 13.0827, 80.2707
	case "gurgaon", "gurugram":
		return 28.4595, 77.0266
	case "noida":
		return 28.5355, 77.3910
	case "mumbai", "bombay":
		return 19.0760, 72.8777
	case "delhi", "new delhi":
		return 28.6139, 77.2090
	default:
		return 0, 0
	}
}
