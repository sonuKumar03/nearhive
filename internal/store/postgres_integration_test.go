package store

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDatabaseURL returns the isolated test database DSN, or skips when unset.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("NEARHIVE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set NEARHIVE_TEST_DATABASE_URL to a disposable, schema-initialized database")
	}
	return dsn
}

// apiRoleURL derives the read-only Go API role DSN from the test DSN.
func apiRoleURL(t *testing.T, dsn string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.User = url.UserPassword("nearhive_api", "nearhive-api-dev")
	return u.String()
}

// TestAPIRoleCannotWriteDiscoveryData proves the ownership boundary is enforced
// by PostgreSQL privileges, not just by Go conventions: the API role can read
// discovery tables and manage users, but cannot insert into companies.
func TestAPIRoleCannotWriteDiscoveryData(t *testing.T) {
	dsn := testDatabaseURL(t)

	admin, err := NewPostgresStore(dsn)
	require.NoError(t, err)
	defer admin.Close()

	api, err := NewPostgresStore(apiRoleURL(t, dsn))
	if err != nil {
		t.Skipf("nearhive_api role is not provisioned: %v", err)
	}
	defer api.Close()

	ctx := context.Background()

	// Reading discovery data is allowed.
	var count int
	require.NoError(t, api.DB().QueryRowContext(ctx, "SELECT count(*) FROM companies").Scan(&count))

	// Writing scraped data is denied at the database layer.
	_, err = api.DB().ExecContext(ctx,
		"INSERT INTO companies (name, normalized_name) VALUES ($1, $2)",
		"API Role Write Probe", "api role write probe",
	)
	require.Error(t, err, "nearhive_api must not be able to insert into companies")
	assert.Contains(t, strings.ToLower(err.Error()), "permission denied")

	// Positive control: the same role owns user accounts and can write users.
	email := "api-role-probe-" + uuid.NewString() + "@example.com"
	_, err = api.DB().ExecContext(ctx,
		"INSERT INTO users (email, password) VALUES ($1, $2)", email, "hash",
	)
	require.NoError(t, err, "nearhive_api must be able to insert users")
	_, err = admin.DB().ExecContext(ctx, "DELETE FROM users WHERE email = $1", email)
	require.NoError(t, err)
}

// TestSightingsReadHandlesNullCoordinates proves GetSightingsByCompany reads a
// real Python-written row whose source carried no coordinates (NULL lat/lng)
// instead of failing to scan.
func TestSightingsReadHandlesNullCoordinates(t *testing.T) {
	dsn := testDatabaseURL(t)

	admin, err := NewPostgresStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })

	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	domain := suffix + ".example"
	source := "sightings-null-test-" + suffix

	var companyID uuid.UUID
	require.NoError(t, admin.DB().QueryRowContext(ctx,
		"INSERT INTO companies (name, normalized_name, domain) VALUES ($1, $2, $3) RETURNING id",
		"Sightings Null Test", "sightings null test "+suffix, domain,
	).Scan(&companyID))

	_, err = admin.DB().ExecContext(ctx,
		`INSERT INTO sightings (source, source_family, source_record_id, company_id, company_name, raw_address)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		source, "official_site", suffix, companyID, "Sightings Null Test", "",
	)
	require.NoError(t, err)

	// Registered after the close cleanup, so it runs first (LIFO) while admin is open.
	t.Cleanup(func() {
		_, _ = admin.DB().ExecContext(context.Background(), "DELETE FROM sightings WHERE source = $1", source)
		_, _ = admin.DB().ExecContext(context.Background(), "DELETE FROM companies WHERE id = $1", companyID)
	})

	read, err := NewPostgresStore(dsn)
	require.NoError(t, err)
	defer read.Close()

	sightings, err := read.GetSightingsByCompany(ctx, companyID)
	require.NoError(t, err, "NULL lat/lng must not break the scan")
	require.Len(t, sightings, 1)
	assert.Nil(t, sightings[0].Lat)
	assert.Nil(t, sightings[0].Lng)
	assert.Equal(t, source, sightings[0].Source)
}

// TestNearbyCompanySearchUsesLocationEvidence exercises the spatial read against
// real PostGIS data: a company is nearby only through its office location.
func TestNearbyCompanySearchUsesLocationEvidence(t *testing.T) {
	dsn := testDatabaseURL(t)

	admin, err := NewPostgresStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })

	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	domain := suffix + ".example"

	var companyID uuid.UUID
	require.NoError(t, admin.DB().QueryRowContext(ctx,
		"INSERT INTO companies (name, normalized_name, domain) VALUES ($1, $2, $3) RETURNING id",
		"Nearby Search Test", "nearby search test "+suffix, domain,
	).Scan(&companyID))

	const lat, lng = 12.97, 77.59
	_, err = admin.DB().ExecContext(ctx,
		`INSERT INTO locations (company_id, address, coords, presence_type, confidence)
		 VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, 'probable_office', 0.9)`,
		companyID, "Nearby Search Test Office", lng, lat,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = admin.DB().ExecContext(context.Background(), "DELETE FROM companies WHERE id = $1", companyID)
	})

	read, err := NewPostgresStore(dsn)
	require.NoError(t, err)
	defer read.Close()

	near, err := read.Search(ctx, lat, lng, 5000, SearchOpts{})
	require.NoError(t, err)
	assert.Contains(t, companyIDs(near), companyID, "office within radius must be found")

	far, err := read.Search(ctx, 13.50, 78.00, 5000, SearchOpts{})
	require.NoError(t, err)
	assert.NotContains(t, companyIDs(far), companyID, "office outside radius must not be found")
}

func companyIDs(results []model.CompanySearchResult) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.CompanyID)
	}
	return ids
}
