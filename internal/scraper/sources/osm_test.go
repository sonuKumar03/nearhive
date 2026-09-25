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

func TestOSMScraper_ParseResponse(t *testing.T) {
	fixture, err := os.ReadFile("testdata/osm_bangalore_response.json")
	assert.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer srv.Close()

	osm := NewOSMScraperWithURL(srv.URL, srv.Client())
	res, err := osm.Scrape(context.Background(), scraper.ScrapeRequest{
		Region:   "Bangalore",
		Lat:      12.9716,
		Lng:      77.5946,
		RadiusKM: 15,
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Sightings, 2)
	assert.Equal(t, "Mu Sigma", res.Sightings[0].CompanyName)
	assert.Equal(t, "Flipkart Internet Private Limited", res.Sightings[1].CompanyName)
	assert.Equal(t, 12.9854, res.Sightings[0].Lat)
}
