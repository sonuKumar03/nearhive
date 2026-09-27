package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrations_DiscoveryMigrationEmbedded(t *testing.T) {
	content, err := FS.ReadFile("000011_python_discovery.up.sql")
	require.NoError(t, err)
	sqlStr := string(content)

	assert.Contains(t, sqlStr, "CREATE TABLE IF NOT EXISTS discovery_jobs")
	assert.Contains(t, sqlStr, "CREATE TABLE IF NOT EXISTS discovery_source_runs")
	assert.Contains(t, sqlStr, "CREATE TABLE IF NOT EXISTS technical_job_postings")
	assert.Contains(t, sqlStr, "presence_type")
	assert.Contains(t, sqlStr, "confirmed_office")
	assert.Contains(t, sqlStr, "probable_office")
	assert.Contains(t, sqlStr, "job_location_only")
	assert.Contains(t, sqlStr, "work_arrangement")
	assert.Contains(t, sqlStr, "in_office")
	assert.Contains(t, sqlStr, "hybrid")
	assert.Contains(t, sqlStr, "remote")
	assert.Contains(t, sqlStr, "unknown")
}

func TestMigrations_RunAgainstDB(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/nearhive?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skip("Postgres not available, skipping db migration execution test")
		return
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skip("Postgres ping failed, skipping db migration execution test")
		return
	}

	err = Run(ctx, db)
	require.NoError(t, err)

	// Verify discovery_jobs table exists
	var exists bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'discovery_jobs'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "discovery_jobs table should exist")

	// Verify technical_job_postings table exists
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'technical_job_postings'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "technical_job_postings table should exist")

	// Verify presence_type column exists on locations
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns 
			WHERE table_name = 'locations' AND column_name = 'presence_type'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "presence_type column should exist on locations")
}
