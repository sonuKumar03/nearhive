package scraper

import (
	"context"
	"strings"
	"sync"
	"time"

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
	taskMgr     *TaskManager
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
		taskMgr:     NewTaskManager(),
		maxWorkers:  maxWorkers,
	}
}

func (o *Orchestrator) Register(s Scraper) {
	o.scrapers = append(o.scrapers, s)
}

func (o *Orchestrator) TaskManager() *TaskManager {
	return o.taskMgr
}

func (o *Orchestrator) finishTask(task *model.ScrapeTask, status string, sightings int, errMsg string, durationMs int64) {
	if o.store == nil || task == nil {
		return
	}
	now := time.Now()
	task.Status = status
	task.Sightings = sightings
	task.FinishedAt = &now
	task.DurationMS = durationMs
	if errMsg != "" {
		task.Error = &errMsg
	}
	// Use background context with short timeout to ensure status write succeeds even if job context cancelled
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = o.store.UpdateTask(saveCtx, task)
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
	} else if req.Lat != 0 && req.Lng != 0 && (req.Region == "" || strings.HasPrefix(req.Region, "Loc(")) {
		// Reverse-geocode coordinates to resolve friendly region/city name
		if o.geocoder != nil {
			if geo, err := o.geocoder.ReverseGeocode(ctx, req.Lat, req.Lng); err == nil && geo != nil && geo.Address != "" {
				parts := strings.Split(geo.Address, ",")
				if len(parts) > 0 {
					req.Region = strings.TrimSpace(parts[0])
				}
			}
		}
	}

	// Pre-create ScrapeTask records for each active scraper if JobID is provided
	taskMap := make(map[string]*model.ScrapeTask)
	if req.JobID != uuid.Nil && o.store != nil {
		for _, s := range o.scrapers {
			if !s.Supports(req) {
				continue
			}
			t := &model.ScrapeTask{
				ID:        uuid.New(),
				JobID:     req.JobID,
				Source:    s.Name(),
				Status:    "pending",
				CreatedAt: time.Now(),
			}
			if err := o.store.CreateTask(ctx, t); err == nil {
				taskMap[s.Name()] = t
			}
		}
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		sightings []model.Sighting
		sem       = make(chan struct{}, o.maxWorkers)
	)

	for _, s := range o.scrapers {
		if !s.Supports(req) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}

		go func(scraper Scraper) {
			defer wg.Done()
			defer func() { <-sem }()

			task := taskMap[scraper.Name()]
			start := time.Now()
			if task != nil {
				task.Status = "running"
				task.StartedAt = &start
				saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = o.store.UpdateTask(saveCtx, task)
				cancel()
			}

			if ctx.Err() != nil {
				if task != nil {
					o.finishTask(task, "cancelled", 0, "job cancelled", time.Since(start).Milliseconds())
				}
				return
			}

			_ = o.rateLimiter.Wait(ctx, scraper.Name())
			if ctx.Err() != nil {
				if task != nil {
					o.finishTask(task, "cancelled", 0, "job cancelled", time.Since(start).Milliseconds())
				}
				return
			}

			res, err := scraper.Scrape(ctx, req)
			if ctx.Err() != nil {
				if task != nil {
					o.finishTask(task, "cancelled", 0, "job cancelled", time.Since(start).Milliseconds())
				}
				return
			}

			if err != nil || res == nil {
				if task != nil {
					errStr := "scraper failed"
					if err != nil {
						errStr = err.Error()
					}
					o.finishTask(task, "failed", 0, errStr, time.Since(start).Milliseconds())
				}
				return
			}

			for i := range res.Sightings {
				if ctx.Err() != nil {
					if task != nil {
						o.finishTask(task, "cancelled", 0, "job cancelled", time.Since(start).Milliseconds())
					}
					return
				}
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
					if ctx.Err() != nil {
						break
					}
					_ = o.verifier.ProcessSighting(ctx, res.Sightings[i])
				}
			}

			if task != nil {
				if ctx.Err() != nil {
					o.finishTask(task, "cancelled", 0, "job cancelled", time.Since(start).Milliseconds())
				} else {
					o.finishTask(task, "done", len(res.Sightings), "", time.Since(start).Milliseconds())
				}
			}

			mu.Lock()
			sightings = append(sightings, res.Sightings...)
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	return sightings, ctx.Err()
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
