package verifier

import (
	"context"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/store"
)

type GeoVerifier struct {
	store    store.Store
	geocoder geocoder.Geocoder
}

func NewGeoVerifier(s store.Store, g geocoder.Geocoder) *GeoVerifier {
	return &GeoVerifier{store: s, geocoder: g}
}

func (gv *GeoVerifier) VerifyCompanyLocations(ctx context.Context, companyID uuid.UUID) error {
	locs, err := gv.store.GetLocationsByCompany(ctx, companyID)
	if err != nil {
		return err
	}

	for _, loc := range locs {
		if loc.Lat == 0 && loc.Lng == 0 && loc.Address != "" && gv.geocoder != nil {
			res, err := gv.geocoder.Geocode(ctx, loc.Address)
			if err == nil && res != nil {
				_ = gv.store.UpdateLocationCoords(ctx, loc.ID, res.Lat, res.Lng)
			}
		}
	}
	return nil
}
