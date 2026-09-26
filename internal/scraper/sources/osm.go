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

var defaultOverpassEndpoints = []string{
	"https://overpass-api.de/api/interpreter",
	"https://overpass.kumi.systems/api/interpreter",
	"https://maps.mail.ru/osm/tools/overpass/api/interpreter",
}

type OSMScraper struct {
	endpoints []string
	client    *http.Client
}

func NewOSMScraper() *OSMScraper {
	return NewOSMScraperWithEndpoints(defaultOverpassEndpoints, nil)
}

func NewOSMScraperWithURL(apiURL string, client *http.Client) *OSMScraper {
	return NewOSMScraperWithEndpoints([]string{apiURL}, client)
}

func NewOSMScraperWithEndpoints(endpoints []string, client *http.Client) *OSMScraper {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if len(endpoints) == 0 {
		endpoints = defaultOverpassEndpoints
	}
	return &OSMScraper{
		endpoints: endpoints,
		client:    client,
	}
}

func (o *OSMScraper) Name() string {
	return "osm"
}

func (o *OSMScraper) Supports(req scraper.ScrapeRequest) bool {
	return (req.Lat != 0 || req.Lng != 0) || req.Region != ""
}

type overpassCenter struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type overpassElement struct {
	Type   string            `json:"type"`
	ID     int64             `json:"id"`
	Lat    float64           `json:"lat"`
	Lon    float64           `json:"lon"`
	Center *overpassCenter   `json:"center,omitempty"`
	Tags   map[string]string `json:"tags"`
}

type overpassResponse struct {
	Elements []overpassElement `json:"elements"`
}

func (o *OSMScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	if req.Lat == 0 && req.Lng == 0 {
		return &scraper.ScrapeResult{}, fmt.Errorf("osm requires valid coordinates (cannot query 0, 0)")
	}

	radiusMeters := int(req.RadiusKM * 1000)
	if radiusMeters <= 0 {
		radiusMeters = 15000
	}

	query := fmt.Sprintf(`[out:json][timeout:25];(nwr["office"~"company|it|software|telecommunication|coworking|research"](around:%d,%f,%f);nwr["amenity"="coworking_space"](around:%d,%f,%f););out center;`,
		radiusMeters, req.Lat, req.Lng,
		radiusMeters, req.Lat, req.Lng)

	var lastErr error
	var opResp overpassResponse

	for _, endpoint := range o.endpoints {
		data := url.Values{}
		data.Set("data", query)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
		if err != nil {
			lastErr = err
			continue
		}
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		httpReq.Header.Set("User-Agent", "NearHive/1.0")

		resp, err := o.client.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("overpass endpoint %s returned status %d", endpoint, resp.StatusCode)
			continue
		}

		err = json.NewDecoder(resp.Body).Decode(&opResp)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		// Successfully retrieved and parsed response from this mirror
		lastErr = nil
		break
	}

	if lastErr != nil && len(opResp.Elements) == 0 {
		return nil, lastErr
	}

	var sightings []model.Sighting
	for _, el := range opResp.Elements {
		name := strings.TrimSpace(el.Tags["name"])
		if name == "" {
			name = strings.TrimSpace(el.Tags["brand"])
		}
		if name == "" {
			name = strings.TrimSpace(el.Tags["operator"])
		}
		if name == "" {
			continue
		}
		lat := el.Lat
		lng := el.Lon
		if lat == 0 && lng == 0 && el.Center != nil {
			lat = el.Center.Lat
			lng = el.Center.Lon
		}
		if lat == 0 && lng == 0 {
			continue
		}

		// Filter out non-tech amenities (e.g. schools, clinics, places of worship)
		if amenity := el.Tags["amenity"]; amenity != "" && amenity != "coworking_space" {
			continue
		}

		var addressParts []string
		if street := el.Tags["addr:street"]; street != "" {
			addressParts = append(addressParts, street)
		}
		if housenumber := el.Tags["addr:housenumber"]; housenumber != "" {
			addressParts = append(addressParts, housenumber)
		}
		if suburb := el.Tags["addr:suburb"]; suburb != "" {
			addressParts = append(addressParts, suburb)
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
		if phone := el.Tags["phone"]; phone != "" {
			meta["phone"] = phone
		}
		if branch := el.Tags["branch"]; branch != "" {
			meta["branch"] = branch
		}
		if office := el.Tags["office"]; office != "" {
			meta["office_type"] = office
		}
		if amenity := el.Tags["amenity"]; amenity != "" {
			meta["amenity"] = amenity
		}

		sightings = append(sightings, model.Sighting{
			Source:      o.Name(),
			CompanyName: name,
			RawAddress:  rawAddress,
			Lat:         lat,
			Lng:         lng,
			Metadata:    meta,
			ScrapedAt:   time.Now(),
		})
	}

	return &scraper.ScrapeResult{
		Sightings: sightings,
	}, nil
}
