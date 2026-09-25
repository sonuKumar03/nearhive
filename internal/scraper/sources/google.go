package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
)

type GooglePlacesScraper struct {
	apiKey string
	client *http.Client
}

func NewGooglePlacesScraper(apiKey string) *GooglePlacesScraper {
	return &GooglePlacesScraper{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *GooglePlacesScraper) Name() string {
	return "google"
}

func (g *GooglePlacesScraper) Supports(region string) bool {
	return g.apiKey != ""
}

type placesResponse struct {
	Results []struct {
		Name     string `json:"name"`
		Vicinity string `json:"vicinity"`
		Geometry struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
	Status string `json:"status"`
}

func (g *GooglePlacesScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	if g.apiKey == "" {
		return &scraper.ScrapeResult{}, nil
	}

	radiusMeters := int(req.RadiusKM * 1000)
	if radiusMeters <= 0 {
		radiusMeters = 15000
	}

	apiURL := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/nearbysearch/json?location=%f,%f&radius=%d&type=point_of_interest&keyword=%s&key=%s",
		req.Lat, req.Lng, radiusMeters, url.QueryEscape("software technology company"), g.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data placesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var sightings []model.Sighting
	for _, p := range data.Results {
		sightings = append(sightings, model.Sighting{
			Source:      g.Name(),
			CompanyName: p.Name,
			RawAddress:  p.Vicinity,
			Lat:         p.Geometry.Location.Lat,
			Lng:         p.Geometry.Location.Lng,
			ScrapedAt:   time.Now(),
		})
	}

	return &scraper.ScrapeResult{Sightings: sightings}, nil
}
