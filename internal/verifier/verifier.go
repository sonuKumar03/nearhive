package verifier

import (
	"context"

	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type Engine struct {
	classifier  *Classifier
	matcher     *Matcher
	merger      *Merger
	geoverifier *GeoVerifier
}

func NewEngine(s store.Store, g geocoder.Geocoder) *Engine {
	return &Engine{
		classifier:  NewClassifier(),
		matcher:     NewMatcher(s),
		merger:      NewMerger(s),
		geoverifier: NewGeoVerifier(s, g),
	}
}

func (e *Engine) ProcessSighting(ctx context.Context, s model.Sighting) error {
	// 1. Entity classification: drop non-tech entities (schools, coaching centers, hospitals, etc.)
	verdict := e.classifier.Classify(s.CompanyName, s.Metadata)
	if !verdict.IsTechCompany {
		return nil
	}

	// 2. Find matching company by domain or normalized name
	match, err := e.matcher.FindMatch(ctx, s)
	if err != nil {
		return err
	}

	// 3. Merge sighting into company & spatial clusters
	if err := e.merger.MergeSighting(ctx, s, match); err != nil {
		return err
	}

	// 4. Run geo-verification if company matched
	if match != nil {
		return e.geoverifier.VerifyCompanyLocations(ctx, match.CompanyID)
	}
	return nil
}
