package scraper

import (
	"context"
	"sync"

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
				if (res.Sightings[i].Lat == 0 || res.Sightings[i].Lng == 0) && o.geocoder != nil {
					if geo, err := o.geocoder.Geocode(ctx, res.Sightings[i].RawAddress); err == nil && geo != nil {
						res.Sightings[i].Lat = geo.Lat
						res.Sightings[i].Lng = geo.Lng
					}
				}
				if o.verifier != nil {
					_ = o.verifier.ProcessSighting(ctx, res.Sightings[i])
				}
			}

			_ = o.store.SaveSightings(ctx, scraper.Name(), res.Sightings)

			mu.Lock()
			sightings = append(sightings, res.Sightings...)
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	return sightings, nil
}
