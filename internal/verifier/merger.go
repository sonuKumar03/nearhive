package verifier

import (
	"context"
	"math"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type Merger struct {
	store store.Store
}

func NewMerger(s store.Store) *Merger {
	return &Merger{store: s}
}

func (m *Merger) MergeSighting(ctx context.Context, s model.Sighting, match *MatchResult) error {
	var companyID uuid.UUID

	if match == nil {
		domain := extractDomainFromMetadata(s.Metadata)
		var domainPtr *string
		if domain != "" {
			domainPtr = &domain
		}
		newCompany := &model.Company{
			ID:             uuid.New(),
			Name:           s.CompanyName,
			NormalizedName: Normalize(s.CompanyName),
			Domain:         domainPtr,
		}
		if err := m.store.CreateCompany(ctx, newCompany); err != nil {
			return err
		}
		companyID = newCompany.ID
	} else {
		companyID = match.CompanyID
	}

	sourceWeight := getSourceWeight(s.Source)

	// Check if this location already exists for this company within 500m
	if s.Lat != 0 && s.Lng != 0 {
		existing, err := m.store.FindNearbyLocation(ctx, companyID, s.Lat, s.Lng, 500)
		if err != nil {
			return err
		}
		if existing != nil {
			newConf := math.Min(1.0, existing.Confidence+sourceWeight)
			if err := m.store.UpdateLocationConfidence(ctx, existing.ID, newConf); err != nil {
				return err
			}
			return m.store.LinkSighting(ctx, s.ID, companyID, existing.ID)
		}
	}

	// Create new location record
	newLoc := &model.Location{
		ID:         uuid.New(),
		CompanyID:  companyID,
		Address:    s.RawAddress,
		Lat:        s.Lat,
		Lng:        s.Lng,
		Confidence: sourceWeight,
		Verified:   sourceWeight >= 0.8,
	}
	if err := m.store.CreateLocation(ctx, newLoc); err != nil {
		return err
	}

	return m.store.LinkSighting(ctx, s.ID, companyID, newLoc.ID)
}

func getSourceWeight(source string) float64 {
	weights := map[string]float64{
		"techpark":  0.40,
		"osm":       0.35,
		"google":    0.35,
		"mca":       0.30,
		"linkedin":  0.25,
		"justdial":  0.25,
		"crunchbase": 0.20,
		"jobportal": 0.15,
	}
	if w, ok := weights[source]; ok {
		return w
	}
	return 0.10
}
