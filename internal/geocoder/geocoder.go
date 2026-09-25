package geocoder

import (
	"context"
	"errors"
)

var (
	ErrNoResults = errors.New("no geocoding results found")
)

type GeoResult struct {
	Lat     float64
	Lng     float64
	Address string
}

type Geocoder interface {
	Geocode(ctx context.Context, address string) (*GeoResult, error)
	ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error)
}
