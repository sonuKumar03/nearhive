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

func TestLoad_CustomSSL(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("DATABASE_URL", "postgres://user:pass@ep-test.db.com/nearhive")
	_ = os.Setenv("JWT_SECRET", "test-secret")
	_ = os.Setenv("DB_SSL_MODE", "require")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Contains(t, cfg.DatabaseURL, "sslmode=require")
}

func TestConfig_DiscoveryDefaults(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("DATABASE_URL", "postgres://localhost/test")
	_ = os.Setenv("JWT_SECRET", "test-secret")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "", cfg.DiscoveryWorkerToken)
	assert.Equal(t, 500, cfg.DiscoveryMaxBatchRecords)
	assert.Equal(t, int64(2097152), cfg.DiscoveryMaxBodyBytes)
}

func TestConfig_DiscoveryCustom(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("DATABASE_URL", "postgres://localhost/test")
	_ = os.Setenv("JWT_SECRET", "test-secret")
	_ = os.Setenv("DISCOVERY_WORKER_TOKEN", "custom-secret-token")
	_ = os.Setenv("DISCOVERY_MAX_BATCH_RECORDS", "150")
	_ = os.Setenv("DISCOVERY_MAX_BODY_BYTES", "1048576")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "custom-secret-token", cfg.DiscoveryWorkerToken)
	assert.Equal(t, 150, cfg.DiscoveryMaxBatchRecords)
	assert.Equal(t, int64(1048576), cfg.DiscoveryMaxBodyBytes)
}

func TestConfig_ProductionRequiresWorkerToken(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("DATABASE_URL", "postgres://localhost/test")
	_ = os.Setenv("JWT_SECRET", "test-secret")
	_ = os.Setenv("ENVIRONMENT", "production")

	// Missing worker token in production -> error
	_, err := Load()
	assert.Error(t, err, "production startup must reject empty worker token")

	// Provided worker token in production -> ok
	_ = os.Setenv("DISCOVERY_WORKER_TOKEN", "prod-worker-token-xyz")
	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "prod-worker-token-xyz", cfg.DiscoveryWorkerToken)
}
