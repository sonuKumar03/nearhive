package geocoder

import "context"

type FallbackGeocoder struct {
	primary  Geocoder
	fallback Geocoder
}

func NewFallbackGeocoder(primary, fallback Geocoder) *FallbackGeocoder {
	return &FallbackGeocoder{
		primary:  primary,
		fallback: fallback,
	}
}

func (f *FallbackGeocoder) Geocode(ctx context.Context, address string) (*GeoResult, error) {
	if f.primary != nil {
		if res, err := f.primary.Geocode(ctx, address); err == nil && res != nil {
			return res, nil
		}
	}
	if f.fallback != nil {
		return f.fallback.Geocode(ctx, address)
	}
	return nil, ErrNoResults
}

func (f *FallbackGeocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error) {
	if f.primary != nil {
		if res, err := f.primary.ReverseGeocode(ctx, lat, lng); err == nil && res != nil {
			return res, nil
		}
	}
	if f.fallback != nil {
		return f.fallback.ReverseGeocode(ctx, lat, lng)
	}
	return nil, ErrNoResults
}
