package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
)

type JustDialScraper struct {
	client *http.Client
}

func NewJustDialScraper() *JustDialScraper {
	return &JustDialScraper{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (j *JustDialScraper) Name() string {
	return "justdial"
}

func (j *JustDialScraper) Supports(region string) bool {
	return true
}

func (j *JustDialScraper) Scrape(ctx context.Context, req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	targetURL := fmt.Sprintf("https://www.justdial.com/%s/Software-Companies", url.PathEscape(req.Region))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := j.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &scraper.ScrapeResult{}, nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var sightings []model.Sighting
	doc.Find(".jsx-result-card, .resultbox").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find(".resultbox_title, .store-name").Text())
		if name == "" {
			return
		}
		addr := strings.TrimSpace(s.Find(".address-info, .resultbox_address").Text())
		sightings = append(sightings, model.Sighting{
			Source:      j.Name(),
			CompanyName: name,
			RawAddress:  addr + ", " + req.Region,
			ScrapedAt:   time.Now(),
		})
	})

	return &scraper.ScrapeResult{Sightings: sightings}, nil
}
