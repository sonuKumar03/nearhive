package sources

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
	"gopkg.in/yaml.v3"
)

type TechParkSelectors struct {
	CompanyList string `yaml:"company_list"`
	Name        string `yaml:"name"`
	Address     string `yaml:"address"`
}

type TechParkConfig struct {
	ID        string            `yaml:"id"`
	Name      string            `yaml:"name"`
	URL       string            `yaml:"url"`
	Region    string            `yaml:"region"`
	Lat       float64           `yaml:"lat"`
	Lng       float64           `yaml:"lng"`
	Selectors TechParkSelectors `yaml:"selectors"`
}

type techParksYAML struct {
	Parks []TechParkConfig `yaml:"parks"`
}

type TechParkScraper struct {
	parks  []TechParkConfig
	client *http.Client
}

func LoadTechParksFromFile(filePath string) ([]TechParkConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var y techParksYAML
	if err := yaml.Unmarshal(data, &y); err != nil {
		return nil, err
	}
	return y.Parks, nil
}

func NewTechParkScraper(parks []TechParkConfig) *TechParkScraper {
	return &TechParkScraper{
		parks:  parks,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TechParkScraper) Name() string {
	return "techpark"
}

func (t *TechParkScraper) Supports(region string) bool {
	for _, p := range t.parks {
		if strings.EqualFold(p.Region, region) {
			return true
		}
	}
	return false
}

func (t *TechParkScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	var sightings []model.Sighting

	for _, park := range t.parks {
		if !strings.EqualFold(park.Region, req.Region) {
			continue
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, park.URL, nil)
		if err != nil {
			continue
		}
		httpReq.Header.Set("User-Agent", "Mozilla/5.0 (NearHive Company Research Bot)")

		resp, err := t.client.Do(httpReq)
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		doc.Find(park.Selectors.CompanyList).Each(func(i int, s *goquery.Selection) {
			name := strings.TrimSpace(s.Find(park.Selectors.Name).Text())
			if name == "" {
				return
			}
			addr := strings.TrimSpace(s.Find(park.Selectors.Address).Text())
			if addr == "" {
				addr = park.Name + ", " + park.Region
			}

			sightings = append(sightings, model.Sighting{
				Source:      t.Name(),
				SourceURL:   &park.URL,
				CompanyName: name,
				RawAddress:  addr,
				Lat:         park.Lat,
				Lng:         park.Lng,
				Metadata: model.JSONMap{
					"tech_park": park.Name,
				},
				ScrapedAt: time.Now(),
			})
		})
	}

	return &scraper.ScrapeResult{Sightings: sightings}, nil
}
