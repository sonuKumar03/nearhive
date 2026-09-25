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

func TestOSMScraper_WaysAndRelationsWithCenter(t *testing.T) {
	jsonBody := `{
		"elements": [
			{
				"type": "way",
				"id": 201,
				"center": {
					"lat": 13.0475,
					"lon": 77.6200
				},
				"tags": {
					"name": "Manyata Tech Park Tower A",
					"office": "it",
					"addr:street": "Hebbal Outer Ring Rd",
					"addr:city": "Bengaluru",
					"website": "https://manyatatechpark.com"
				}
			},
			{
				"type": "relation",
				"id": 301,
				"center": {
					"lat": 17.4399,
					"lon": 78.3807
				},
				"tags": {
					"name": "WeWork Mindspace",
					"amenity": "coworking_space",
					"addr:city": "Hyderabad"
				}
			}
		]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonBody))
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
	assert.Equal(t, "Manyata Tech Park Tower A", res.Sightings[0].CompanyName)
	assert.Equal(t, 13.0475, res.Sightings[0].Lat)
	assert.Equal(t, 77.6200, res.Sightings[0].Lng)
	assert.Equal(t, "https://manyatatechpark.com", res.Sightings[0].Metadata["website"])

	assert.Equal(t, "WeWork Mindspace", res.Sightings[1].CompanyName)
	assert.Equal(t, 17.4399, res.Sightings[1].Lat)
	assert.Equal(t, 78.3807, res.Sightings[1].Lng)
	assert.Equal(t, "coworking_space", res.Sightings[1].Metadata["amenity"])
}

func TestOSMScraper_MirrorFailover(t *testing.T) {
	// First server fails with 504 Gateway Timeout
	failingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer failingSrv.Close()

	// Second server responds with valid JSON
	jsonBody := `{
		"elements": [
			{
				"type": "node",
				"id": 105,
				"lat": 18.5204,
				"lon": 73.8567,
				"tags": {
					"name": "Persistent Systems",
					"office": "it",
					"addr:city": "Pune"
				}
			}
		]
	}`
	backupSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonBody))
	}))
	defer backupSrv.Close()

	osm := NewOSMScraperWithEndpoints([]string{failingSrv.URL, backupSrv.URL}, backupSrv.Client())
	res, err := osm.Scrape(context.Background(), scraper.ScrapeRequest{
		Region:   "Pune",
		Lat:      18.5204,
		Lng:      73.8567,
		RadiusKM: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Sightings, 1)
	assert.Equal(t, "Persistent Systems", res.Sightings[0].CompanyName)
	assert.Equal(t, 18.5204, res.Sightings[0].Lat)
}
