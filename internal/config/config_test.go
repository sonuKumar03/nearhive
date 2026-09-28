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
