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

func TestTechParkScraper_SeedTenants(t *testing.T) {
	cfg := TechParkConfig{
		ID:      "manyata",
		Name:    "Manyata Tech Park",
		Region:  "Bangalore",
		Lat:     13.0475,
		Lng:     77.6200,
		Tenants: []string{"Cognizant", "IBM India", "Target Corporation"},
	}

	tpScraper := NewTechParkScraper([]TechParkConfig{cfg})
	res, err := tpScraper.Scrape(context.Background(), scraper.ScrapeRequest{Region: "Bangalore"})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Sightings, 3)
	assert.Equal(t, "Cognizant", res.Sightings[0].CompanyName)
	assert.Equal(t, 13.0475, res.Sightings[0].Lat)
	assert.Equal(t, 77.6200, res.Sightings[0].Lng)
	assert.Equal(t, "Manyata Tech Park, Bangalore", res.Sightings[0].RawAddress)
	assert.Equal(t, "Manyata Tech Park", res.Sightings[0].Metadata["tech_park"])
	assert.Equal(t, "IBM India", res.Sightings[1].CompanyName)
}

func TestLoadTechParksFromFile_ProductionYAML(t *testing.T) {
	parks, err := LoadTechParksFromFile("../../../config/techparks.yaml")
	assert.NoError(t, err)
	assert.NotEmpty(t, parks)
	assert.GreaterOrEqual(t, len(parks), 10)

	// Verify Bangalore tech parks are loaded
	var bangaloreParks int
	for _, p := range parks {
		if p.Region == "Bangalore" {
			bangaloreParks++
		}
	}
	assert.GreaterOrEqual(t, bangaloreParks, 4)
}
