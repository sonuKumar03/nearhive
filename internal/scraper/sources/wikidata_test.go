package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/stretchr/testify/assert"
)

func TestWikidataScraper_ParseResponse(t *testing.T) {
	jsonResponse := `{
		"results": {
			"bindings": [
				{
					"company": {
						"type": "uri",
						"value": "http://www.wikidata.org/entity/Q19059388"
					},
					"companyLabel": {
						"type": "literal",
						"value": "Ola Cabs"
					},
					"website": {
						"type": "uri",
						"value": "https://www.olacabs.com"
					},
					"coord": {
						"type": "literal",
						"value": "Point(77.6200 12.9352)"
					}
				},
				{
					"company": {
						"type": "uri",
						"value": "http://www.wikidata.org/entity/Q18125153"
					},
					"companyLabel": {
						"type": "literal",
						"value": "Goibibo"
					},
					"website": {
						"type": "uri",
						"value": "https://www.goibibo.com"
					}
				}
			]
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "NearHive/1.0 (https://github.com/sonuKumar03/nearhive)", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonResponse))
	}))
	defer srv.Close()

	scraperInstance := NewWikidataScraperWithURL(srv.URL, srv.Client())
	assert.True(t, scraperInstance.Supports("Bangalore"))
	assert.True(t, scraperInstance.Supports("Hyderabad"))
	assert.False(t, scraperInstance.Supports("Atlantis"))

	res, err := scraperInstance.Scrape(context.Background(), scraper.ScrapeRequest{
		Region: "Bangalore",
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Sightings, 2)

	assert.Equal(t, "Ola Cabs", res.Sightings[0].CompanyName)
	assert.Equal(t, 12.9352, res.Sightings[0].Lat)
	assert.Equal(t, 77.6200, res.Sightings[0].Lng)
	assert.Equal(t, "https://www.olacabs.com", res.Sightings[0].Metadata["website"])
	assert.Equal(t, "wikidata", res.Sightings[0].Source)

	assert.Equal(t, "Goibibo", res.Sightings[1].CompanyName)
	assert.Equal(t, "https://www.goibibo.com", res.Sightings[1].Metadata["website"])
}
