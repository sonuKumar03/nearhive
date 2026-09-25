package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/stretchr/testify/assert"
)

func TestTechParkScraper(t *testing.T) {
	fixture, err := os.ReadFile("testdata/techpark_itpb.html")
	assert.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(fixture)
	}))
	defer srv.Close()

	cfg := TechParkConfig{
		ID:     "itpb",
		Name:   "ITPB",
		URL:    srv.URL,
		Region: "Bangalore",
		Lat:    12.9854,
		Lng:    77.7366,
		Selectors: TechParkSelectors{
			CompanyList: ".tenant-card",
			Name:        ".company-name",
			Address:     ".company-address",
		},
	}

	tpScraper := NewTechParkScraper([]TechParkConfig{cfg})
	res, err := tpScraper.Scrape(context.Background(), scraper.ScrapeRequest{Region: "Bangalore"})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Sightings, 2)
	assert.Equal(t, "Xerox Business Services", res.Sightings[0].CompanyName)
	assert.Equal(t, 12.9854, res.Sightings[0].Lat)
}
