package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("DATABASE_URL", "postgres://localhost/test")
	_ = os.Setenv("JWT_SECRET", "test-secret")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, 5, cfg.MaxScraperWorkers)
	assert.Equal(t, "0 3 * * *", cfg.ScrapeSchedule)
	assert.Equal(t, "https://nominatim.openstreetmap.org", cfg.NominatimURL)
}

func TestLoad_MissingRequired(t *testing.T) {
	os.Clearenv()
	_, err := Load()
	assert.Error(t, err)
}
