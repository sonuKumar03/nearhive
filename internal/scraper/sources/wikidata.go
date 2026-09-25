package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
)

var defaultWikidataEndpoint = "https://query.wikidata.org/sparql"

var cityWikidataQIDs = map[string]string{
	"bangalore": "wd:Q1355",
	"bengaluru": "wd:Q1355",
	"hyderabad": "wd:Q1361",
	"pune":      "wd:Q1538",
	"chennai":   "wd:Q1352",
	"gurgaon":   "wd:Q11749",
	"gurugram":  "wd:Q11749",
	"noida":     "wd:Q203922",
	"mumbai":    "wd:Q1156",
	"delhi":     "wd:Q1353",
}

type WikidataScraper struct {
	apiURL string
	client *http.Client
}

func NewWikidataScraper() *WikidataScraper {
	return NewWikidataScraperWithURL(defaultWikidataEndpoint, nil)
}

func NewWikidataScraperWithURL(apiURL string, client *http.Client) *WikidataScraper {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &WikidataScraper{
		apiURL: apiURL,
		client: client,
	}
}

func (w *WikidataScraper) Name() string {
	return "wikidata"
}

func (w *WikidataScraper) Supports(region string) bool {
	_, ok := cityWikidataQIDs[strings.ToLower(strings.TrimSpace(region))]
	return ok
}

type sparqlBindingValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type sparqlBinding struct {
	Company      sparqlBindingValue  `json:"company"`
	CompanyLabel sparqlBindingValue  `json:"companyLabel"`
	Website      *sparqlBindingValue `json:"website,omitempty"`
	Coord        *sparqlBindingValue `json:"coord,omitempty"`
}

type sparqlResponse struct {
	Results struct {
		Bindings []sparqlBinding `json:"bindings"`
	} `json:"results"`
}

func (w *WikidataScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	cityKey := strings.ToLower(strings.TrimSpace(req.Region))
	cityQID, ok := cityWikidataQIDs[cityKey]
	if !ok {
		return &scraper.ScrapeResult{}, nil
	}

	query := fmt.Sprintf(`SELECT ?company ?companyLabel ?website ?coord WHERE {
  ?company wdt:P31/wdt:P279* wd:Q4830453 .
  ?company wdt:P159 %s .
  OPTIONAL { ?company wdt:P856 ?website . }
  OPTIONAL { ?company wdt:P625 ?coord . }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
} LIMIT 100`, cityQID)

	reqURL := fmt.Sprintf("%s?query=%s&format=json", w.apiURL, url.QueryEscape(query))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "NearHive/1.0 (https://github.com/sonuKumar03/nearhive)")
	httpReq.Header.Set("Accept", "application/sparql-results+json, application/json")

	resp, err := w.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikidata returned status %d", resp.StatusCode)
	}

	var data sparqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var sightings []model.Sighting
	for _, b := range data.Results.Bindings {
		name := strings.TrimSpace(b.CompanyLabel.Value)
		// Ignore placeholder QIDs if label resolution failed
		if name == "" || strings.HasPrefix(name, "Q") && len(name) > 1 && isNumeric(name[1:]) {
			continue
		}

		lat, lng := 0.0, 0.0
		if b.Coord != nil && b.Coord.Value != "" {
			lat, lng = parseWikidataPoint(b.Coord.Value)
		}

		meta := make(model.JSONMap)
		meta["wikidata_uri"] = b.Company.Value
		if b.Website != nil && b.Website.Value != "" {
			meta["website"] = b.Website.Value
		}

		sightings = append(sightings, model.Sighting{
			Source:      w.Name(),
			CompanyName: name,
			RawAddress:  req.Region + ", India",
			Lat:         lat,
			Lng:         lng,
			Metadata:    meta,
			ScrapedAt:   time.Now(),
		})
	}

	return &scraper.ScrapeResult{Sightings: sightings}, nil
}

func parseWikidataPoint(wkt string) (float64, float64) {
	// e.g. "Point(77.5946 12.9716)" (lng lat)
	wkt = strings.TrimPrefix(wkt, "Point(")
	wkt = strings.TrimSuffix(wkt, ")")
	parts := strings.Fields(wkt)
	if len(parts) == 2 {
		lng, err1 := strconv.ParseFloat(parts[0], 64)
		lat, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			return lat, lng
		}
	}
	return 0, 0
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
