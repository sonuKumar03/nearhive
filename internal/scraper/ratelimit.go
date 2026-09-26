package scraper

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiterRegistry struct {
	limiters map[string]*rate.Limiter
}

func NewRateLimiterRegistry() *RateLimiterRegistry {
	return &RateLimiterRegistry{
		limiters: map[string]*rate.Limiter{
			"osm":      rate.NewLimiter(rate.Every(2*time.Second), 1),
			"wikidata": rate.NewLimiter(rate.Every(2*time.Second), 1),
			"techpark": rate.NewLimiter(rate.Every(3*time.Second), 1),
			"google":   rate.NewLimiter(rate.Every(200*time.Millisecond), 5),
			"justdial": rate.NewLimiter(rate.Every(5*time.Second), 1),
		},
	}
}

func (r *RateLimiterRegistry) Wait(ctx context.Context, source string) error {
	limiter, ok := r.limiters[source]
	if !ok {
		return nil
	}
	return limiter.Wait(ctx)
}
