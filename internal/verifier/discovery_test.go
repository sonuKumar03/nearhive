package verifier

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessDiscovery_RetainsNonTechCompanies(t *testing.T) {
	mockStore := store.NewMockStore()
	engine := NewEngine(mockStore, nil)
	ctx := context.Background()

	// 1. Hospital sighting (non-tech entity)
	hospitalSighting := model.Sighting{
		ID:          uuid.New(),
		CompanyName: "Apollo Hospital",
		RawAddress:  "Bannerghatta Road, Bangalore",
		Lat:         12.8912,
		Lng:         77.5978,
		Source:      "public_directory",
		Metadata:    model.JSONMap{"source_family": "public_directory"},
	}
	err := engine.ProcessDiscoverySighting(ctx, hospitalSighting)
	require.NoError(t, err)

	// Hospital must be retained (not filtered by admission classifier)
	assert.Len(t, mockStore.Companies, 1, "hospital should be retained by discovery processing")

	// 2. Retailer sighting (non-tech entity)
	retailerSighting := model.Sighting{
		ID:          uuid.New(),
		CompanyName: "Reliance Fresh Retail",
		RawAddress:  "Indiranagar 100ft Road, Bangalore",
		Lat:         12.9716,
		Lng:         77.6412,
		Source:      "public_directory",
		Metadata:    model.JSONMap{"source_family": "public_directory"},
	}
	err = engine.ProcessDiscoverySighting(ctx, retailerSighting)
	require.NoError(t, err)

	// Retailer must also be retained
	assert.Len(t, mockStore.Companies, 2, "retailer should also be retained by discovery processing")
}

func TestMatcher_FindDiscoveryMatch_FuzzyNameDoesNotMerge(t *testing.T) {
	mockStore := store.NewMockStore()
	engine := NewEngine(mockStore, nil)
	matcher := NewMatcher(mockStore)
	ctx := context.Background()

	// Seed existing company
	existingID := uuid.New()
	existingName := "Manipal Hospital"
	err := mockStore.CreateCompany(ctx, &model.Company{
		ID:             existingID,
		Name:           existingName,
		NormalizedName: Normalize(existingName),
	})
	require.NoError(t, err)

	// Candidate with fuzzy name that would match under crawler FindMatch (threshold 0.6)
	candidateSighting := model.Sighting{
		ID:          uuid.New(),
		CompanyName: "Manipal Cure Clinic", // Shares prefix "manipal"
		RawAddress:  "Whitefield, Bangalore",
		Lat:         12.9698,
		Lng:         77.7499,
		Source:      "public_directory",
		Metadata:    model.JSONMap{"source_family": "public_directory"},
	}

	// 1. Crawler FindMatch would merge on fuzzy match
	crawlerMatch, err := matcher.FindMatch(ctx, candidateSighting)
	require.NoError(t, err)
	if crawlerMatch != nil {
		assert.Equal(t, "fuzzy_name", crawlerMatch.MatchType, "crawler matcher uses fuzzy matching")
	}

	// 2. Discovery matcher must NOT return a merge match on fuzzy name alone
	discMatch, err := matcher.FindDiscoveryMatch(ctx, candidateSighting)
	require.NoError(t, err)
	assert.Nil(t, discMatch, "FindDiscoveryMatch must return nil for fuzzy-only candidate")

	// 3. Processing the sighting must create a new company instead of merging into existing
	err = engine.ProcessDiscoverySighting(ctx, candidateSighting)
	require.NoError(t, err)
	assert.Len(t, mockStore.Companies, 2, "fuzzy name candidate should create a new company, not merge")
}

func TestMatcher_FindDiscoveryMatch_ExactDomainAndLocationMatch(t *testing.T) {
	mockStore := store.NewMockStore()
	matcher := NewMatcher(mockStore)
	ctx := context.Background()

	companyID := uuid.New()
	domain := "acmehealth.org"
	err := mockStore.CreateCompany(ctx, &model.Company{
		ID:             companyID,
		Name:           "Acme Healthcare",
		NormalizedName: "acme healthcare",
		Domain:         &domain,
	})
	require.NoError(t, err)

	err = mockStore.CreateLocation(ctx, &model.Location{
		ID:        uuid.New(),
		CompanyID: companyID,
		Address:   "Koramangala, Bangalore",
		Lat:       12.9352,
		Lng:       77.6245,
	})
	require.NoError(t, err)

	// 1. Exact canonical domain match
	domainMatch, err := matcher.FindDiscoveryMatch(ctx, model.Sighting{
		CompanyName: "Acme Medical Services",
		Metadata:    model.JSONMap{"website": "https://www.acmehealth.org/locations"},
	})
	require.NoError(t, err)
	require.NotNil(t, domainMatch)
	assert.Equal(t, companyID, domainMatch.CompanyID)
	assert.Equal(t, "domain", domainMatch.MatchType)

	// 2. Exact normalized name near same location
	nameLocationMatch, err := matcher.FindDiscoveryMatch(ctx, model.Sighting{
		CompanyName: "ACME HEALTHCARE",
		Lat:         12.9355, // ~35m from existing location
		Lng:         77.6247,
		Metadata:    model.JSONMap{},
	})
	require.NoError(t, err)
	require.NotNil(t, nameLocationMatch)
	assert.Equal(t, companyID, nameLocationMatch.CompanyID)
}

func TestIndependentEvidence_PresenceRecalculation(t *testing.T) {
	ctx := context.Background()

	t.Run("ten job_ats sightings remain job_location_only with confidence 0.4 and verified false", func(t *testing.T) {
		mockStore := store.NewMockStore()
		engine := NewEngine(mockStore, nil)

		// Submit 10 sightings from the same job_ats family at the same location
		for i := 0; i < 10; i++ {
			sighting := model.Sighting{
				ID:          uuid.New(),
				CompanyName: "CloudTech Systems",
				RawAddress:  "Outer Ring Road, Bellandur, Bangalore",
				Lat:         12.9279,
				Lng:         77.6811,
				Source:      "greenhouse",
				Metadata: model.JSONMap{
					"source_family": "job_ats",
					"job_id":        fmt.Sprintf("job-%d", i),
				},
			}
			err := engine.ProcessDiscoverySighting(ctx, sighting)
			require.NoError(t, err)
		}

		companies := mockStore.Companies
		require.Len(t, companies, 1)
		var companyID uuid.UUID
		for id := range companies {
			companyID = id
		}

		locs, err := mockStore.GetLocationsByCompany(ctx, companyID)
		require.NoError(t, err)
		require.Len(t, locs, 1, "all sightings within 500m should merge into 1 location")

		loc := locs[0]
		assert.Equal(t, model.PresenceTypeJobLocationOnly, loc.PresenceType)
		assert.InDelta(t, 0.4, loc.Confidence, 0.01)
		assert.False(t, loc.Verified, "ten job_ats sightings must not produce verified=true")
	})

	t.Run("single public_directory sighting yields probable_office with confidence 0.6 and verified false", func(t *testing.T) {
		mockStore := store.NewMockStore()
		engine := NewEngine(mockStore, nil)

		sighting := model.Sighting{
			ID:          uuid.New(),
			CompanyName: "Apex Diagnostics",
			RawAddress:  "Jayanagar 4th Block, Bangalore",
			Lat:         12.9298,
			Lng:         77.5841,
			Source:      "justdial",
			Metadata:    model.JSONMap{"source_family": "public_directory"},
		}
		err := engine.ProcessDiscoverySighting(ctx, sighting)
		require.NoError(t, err)

		companies := mockStore.Companies
		require.Len(t, companies, 1)
		var companyID uuid.UUID
		for id := range companies {
			companyID = id
		}

		locs, err := mockStore.GetLocationsByCompany(ctx, companyID)
		require.NoError(t, err)
		require.Len(t, locs, 1)

		loc := locs[0]
		assert.Equal(t, model.PresenceTypeProbableOffice, loc.PresenceType)
		assert.InDelta(t, 0.6, loc.Confidence, 0.01)
		assert.False(t, loc.Verified)
	})

	t.Run("multiple sightings in same family do not inflate to confirmed_office", func(t *testing.T) {
		mockStore := store.NewMockStore()
		engine := NewEngine(mockStore, nil)

		// 5 sightings all from public_directory
		for i := 0; i < 5; i++ {
			sighting := model.Sighting{
				ID:          uuid.New(),
				CompanyName: "Apex Diagnostics",
				RawAddress:  "Jayanagar 4th Block, Bangalore",
				Lat:         12.9298,
				Lng:         77.5841,
				Source:      fmt.Sprintf("directory_%d", i),
				Metadata:    model.JSONMap{"source_family": "public_directory"},
			}
			err := engine.ProcessDiscoverySighting(ctx, sighting)
			require.NoError(t, err)
		}

		var companyID uuid.UUID
		for id := range mockStore.Companies {
			companyID = id
		}
		locs, err := mockStore.GetLocationsByCompany(ctx, companyID)
		require.NoError(t, err)
		require.Len(t, locs, 1)

		loc := locs[0]
		assert.Equal(t, model.PresenceTypeProbableOffice, loc.PresenceType)
		assert.InDelta(t, 0.6, loc.Confidence, 0.01)
		assert.False(t, loc.Verified, "same family sightings must not count as independent confirmation")
	})

	t.Run("official_site plus public_directory yields confirmed_office with confidence 0.9 and verified true", func(t *testing.T) {
		mockStore := store.NewMockStore()
		engine := NewEngine(mockStore, nil)

		// Sighting 1: official company site
		sighting1 := model.Sighting{
			ID:          uuid.New(),
			CompanyName: "Zenith Labs",
			RawAddress:  "Koramangala 5th Block, Bangalore",
			Lat:         12.9344,
			Lng:         77.6190,
			Source:      "official_site",
			Metadata: model.JSONMap{
				"source_family": "official_site",
				"website":       "https://zenithlabs.io",
			},
		}
		err := engine.ProcessDiscoverySighting(ctx, sighting1)
		require.NoError(t, err)

		// Sighting 2: independent public directory at same spatial location
		sighting2 := model.Sighting{
			ID:          uuid.New(),
			CompanyName: "Zenith Labs",
			RawAddress:  "Koramangala 5th Block, Bangalore",
			Lat:         12.9345, // ~15m away
			Lng:         77.6191,
			Source:      "public_directory",
			Metadata: model.JSONMap{
				"source_family": "public_directory",
			},
		}
		err = engine.ProcessDiscoverySighting(ctx, sighting2)
		require.NoError(t, err)

		var companyID uuid.UUID
		for id := range mockStore.Companies {
			companyID = id
		}
		locs, err := mockStore.GetLocationsByCompany(ctx, companyID)
		require.NoError(t, err)
		require.Len(t, locs, 1)

		loc := locs[0]
		assert.Equal(t, model.PresenceTypeConfirmedOffice, loc.PresenceType)
		assert.InDelta(t, 0.9, loc.Confidence, 0.01)
		assert.True(t, loc.Verified, "official_site + public_directory must produce confirmed_office with verified=true")
	})
}
