package geocoder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Google struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewGoogle(baseURL, apiKey string, client *http.Client) *Google {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://maps.googleapis.com/maps/api/geocode/json"
	}
	return &Google{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  client,
	}
}

type googleGeocodeResponse struct {
	Status  string `json:"status"`
	Results []struct {
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}

func (g *Google) Geocode(ctx context.Context, address string) (*GeoResult, error) {
	if g.apiKey == "" && g.baseURL == "https://maps.googleapis.com/maps/api/geocode/json" {
		return nil, errors.New("google geocoding api key is not configured")
	}

	reqURL := fmt.Sprintf("%s?address=%s&key=%s", g.baseURL, url.QueryEscape(address), g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data googleGeocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Status != "OK" || len(data.Results) == 0 {
		return nil, ErrNoResults
	}

	return &GeoResult{
		Lat:     data.Results[0].Geometry.Location.Lat,
		Lng:     data.Results[0].Geometry.Location.Lng,
		Address: data.Results[0].FormattedAddress,
	}, nil
}

func (g *Google) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error) {
	reqURL := fmt.Sprintf("%s?latlng=%f,%f&key=%s", g.baseURL, lat, lng, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data googleGeocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Status != "OK" || len(data.Results) == 0 {
		return nil, ErrNoResults
	}

	return &GeoResult{
		Lat:     data.Results[0].Geometry.Location.Lat,
		Lng:     data.Results[0].Geometry.Location.Lng,
		Address: data.Results[0].FormattedAddress,
	}, nil
}
