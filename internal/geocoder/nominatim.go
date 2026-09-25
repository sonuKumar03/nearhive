package geocoder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Nominatim struct {
	baseURL string
	client  *http.Client
}

func NewNominatim(baseURL string, client *http.Client) *Nominatim {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://nominatim.openstreetmap.org"
	}
	return &Nominatim{
		baseURL: baseURL,
		client:  client,
	}
}

type nominatimResult struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
}

func (n *Nominatim) Geocode(ctx context.Context, address string) (*GeoResult, error) {
	reqURL := fmt.Sprintf("%s/search?q=%s&format=json&limit=1", n.baseURL, url.QueryEscape(address))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NearHive/1.0 (contact@nearhive.local)")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status: %d", resp.StatusCode)
	}

	var results []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	lat, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return nil, err
	}
	lng, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return nil, err
	}

	return &GeoResult{
		Lat:     lat,
		Lng:     lng,
		Address: results[0].DisplayName,
	}, nil
}

func (n *Nominatim) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeoResult, error) {
	reqURL := fmt.Sprintf("%s/reverse?lat=%f&lon=%f&format=json", n.baseURL, lat, lng)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NearHive/1.0")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status: %d", resp.StatusCode)
	}

	var result nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &GeoResult{
		Lat:     lat,
		Lng:     lng,
		Address: result.DisplayName,
	}, nil
}
