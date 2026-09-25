package verifier

import (
	"context"

	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type Engine struct {
	matcher     *Matcher
	merger      *Merger
	geoverifier *GeoVerifier
}

func NewEngine(s store.Store, g geocoder.Geocoder) *Engine {
	return &Engine{
		matcher:     NewMatcher(s),
		merger:      NewMerger(s),
		geoverifier: NewGeoVerifier(s, g),
	}
}

func (e *Engine) ProcessSighting(ctx context.Context, s model.Sighting) error {
	match, err := e.matcher.FindMatch(ctx, s)
	if err != nil {
		return err
	}

	if err := e.merger.MergeSighting(ctx, s, match); err != nil {
		return err
	}

	if match != nil {
		return e.geoverifier.VerifyCompanyLocations(ctx, match.CompanyID)
	}
	return nil
}
