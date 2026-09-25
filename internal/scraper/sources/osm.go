package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
)

type OSMScraper struct {
	apiURL string
	client *http.Client
}

func NewOSMScraper() *OSMScraper {
	return NewOSMScraperWithURL("https://overpass-api.de/api/interpreter", nil)
}

func NewOSMScraperWithURL(apiURL string, client *http.Client) *OSMScraper {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &OSMScraper{
		apiURL: apiURL,
		client: client,
	}
}

func (o *OSMScraper) Name() string {
	return "osm"
}

func (o *OSMScraper) Supports(region string) bool {
	return true
}

type overpassElement struct {
	Type string            `json:"type"`
	Lat  float64           `json:"lat"`
	Lon  float64           `json:"lon"`
	Tags map[string]string `json:"tags"`
}

type overpassResponse struct {
	Elements []overpassElement `json:"elements"`
}

func (o *OSMScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	radiusMeters := int(req.RadiusKM * 1000)
	if radiusMeters <= 0 {
		radiusMeters = 15000
	}

	query := fmt.Sprintf(`[out:json][timeout:25];(node["office"~"company|it|coworking"](around:%d,%f,%f););out center;`,
		radiusMeters, req.Lat, req.Lng)

	data := url.Values{}
	data.Set("data", query)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("User-Agent", "NearHive/1.0")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass returned status %d", resp.StatusCode)
	}

	var opResp overpassResponse
	if err := json.NewDecoder(resp.Body).Decode(&opResp); err != nil {
		return nil, err
	}

	var sightings []model.Sighting
	for _, el := range opResp.Elements {
		name := el.Tags["name"]
		if name == "" {
			continue
		}
		var addressParts []string
		if street := el.Tags["addr:street"]; street != "" {
			addressParts = append(addressParts, street)
		}
		if city := el.Tags["addr:city"]; city != "" {
			addressParts = append(addressParts, city)
		}
		rawAddress := strings.Join(addressParts, ", ")
		if rawAddress == "" {
			rawAddress = req.Region
		}

		meta := make(model.JSONMap)
		if site := el.Tags["website"]; site != "" {
			meta["website"] = site
		}

		sightings = append(sightings, model.Sighting{
			Source:      o.Name(),
			CompanyName: name,
			RawAddress:  rawAddress,
			Lat:         el.Lat,
			Lng:         el.Lon,
			Metadata:    meta,
			ScrapedAt:   time.Now(),
		})
	}

	return &scraper.ScrapeResult{
		Sightings: sightings,
	}, nil
}
