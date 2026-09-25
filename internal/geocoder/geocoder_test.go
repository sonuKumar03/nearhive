package geocoder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNominatim_Geocode(t *testing.T) {
	mockResponse := `[
		{
			"lat": "12.9854",
			"lon": "77.7366",
			"display_name": "ITPB, Whitefield, Bangalore, Karnataka, India"
		}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	}))
	defer srv.Close()

	geo := NewNominatim(srv.URL, srv.Client())
	res, err := geo.Geocode(context.Background(), "ITPB Whitefield")

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.InDelta(t, 12.9854, res.Lat, 0.0001)
	assert.InDelta(t, 77.7366, res.Lng, 0.0001)
	assert.Contains(t, res.Address, "ITPB")
}

func TestFallbackGeocoder_PrimaryFailsThenFallbackSucceeds(t *testing.T) {
	primarySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer primarySrv.Close()

	fallbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"results": [{
				"formatted_address": "Google Office, Bangalore",
				"geometry": {"location": {"lat": 12.9716, "lng": 77.5946}}
			}],
			"status": "OK"
		}`))
	}))
	defer fallbackSrv.Close()

	primary := NewNominatim(primarySrv.URL, primarySrv.Client())
	fallback := NewGoogle(fallbackSrv.URL, "dummy-key", fallbackSrv.Client())

	chained := NewFallbackGeocoder(primary, fallback)
	res, err := chained.Geocode(context.Background(), "Google Bangalore")

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.InDelta(t, 12.9716, res.Lat, 0.0001)
}
