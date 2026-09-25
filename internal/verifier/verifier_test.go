package verifier

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Infosys Limited", "infosys"},
		{"INFOSYS LTD", "infosys"},
		{"Wipro Technologies Pvt. Ltd.", "wipro"},
		{"Tata Consultancy Services (TCS) India", "tata consultancy services tcs"},
		{"Microsoft Corporation India Pvt Ltd", "microsoft"},
		{"ThoughtWorks Software Solutions", "thoughtworks"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, Normalize(tc.input), "mismatch for %s", tc.input)
	}
}

func TestMatcher_DomainAndNameMatch(t *testing.T) {
	mockStore := store.NewMockStore()
	companyID := uuid.New()
	domain := "infosys.com"

	_ = mockStore.CreateCompany(context.Background(), &model.Company{
		ID:             companyID,
		Name:           "Infosys Limited",
		NormalizedName: "infosys",
		Domain:         &domain,
	})

	matcher := NewMatcher(mockStore)

	// 1. Domain match
	domainMatch, err := matcher.FindMatch(context.Background(), model.Sighting{
		CompanyName: "Infosys Careers",
		Metadata:    model.JSONMap{"website": "https://www.infosys.com/about"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, domainMatch)
	assert.Equal(t, companyID, domainMatch.CompanyID)
	assert.Equal(t, "domain", domainMatch.MatchType)

	// 2. Normalized name match
	nameMatch, err := matcher.FindMatch(context.Background(), model.Sighting{
		CompanyName: "INFOSYS LIMITED",
	})
	assert.NoError(t, err)
	assert.NotNil(t, nameMatch)
	assert.Equal(t, companyID, nameMatch.CompanyID)
	assert.Equal(t, "exact_name", nameMatch.MatchType)
}

func TestMerger_ConfidenceScoring(t *testing.T) {
	mockStore := store.NewMockStore()
	merger := NewMerger(mockStore)

	sighting1 := model.Sighting{
		CompanyName: "Swiggy",
		RawAddress:  "Koramangala, Bangalore",
		Lat:         12.9352,
		Lng:         77.6245,
		Source:      "osm",
	}

	match := &MatchResult{
		CompanyID: uuid.New(),
		MatchType: "new_company",
	}
	_ = mockStore.CreateCompany(context.Background(), &model.Company{
		ID:             match.CompanyID,
		Name:           sighting1.CompanyName,
		NormalizedName: "swiggy",
	})

	// First sighting from OSM (weight 0.35)
	err := merger.MergeSighting(context.Background(), sighting1, match)
	assert.NoError(t, err)

	locs, _ := mockStore.GetLocationsByCompany(context.Background(), match.CompanyID)
	assert.Len(t, locs, 1)
	assert.InDelta(t, 0.35, locs[0].Confidence, 0.01)

	// Second sighting from TechPark at same location (weight 0.40)
	sighting2 := model.Sighting{
		CompanyName: "Swiggy",
		RawAddress:  "Koramangala 4th Block",
		Lat:         12.9354, // ~25m away
		Lng:         77.6246,
		Source:      "techpark",
	}
	err = merger.MergeSighting(context.Background(), sighting2, match)
	assert.NoError(t, err)

	locsAfter, _ := mockStore.GetLocationsByCompany(context.Background(), match.CompanyID)
	assert.Len(t, locsAfter, 1) // Merged into existing location
	assert.InDelta(t, 0.75, locsAfter[0].Confidence, 0.01)
}

func TestEngine_ProcessSighting_FiltersNonTechEntities(t *testing.T) {
	mockStore := store.NewMockStore()
	engine := NewEngine(mockStore, nil)

	// Non-tech sighting (school)
	schoolSighting := model.Sighting{
		CompanyName: "Brilliant Grammar High School",
		RawAddress:  "Ameerpet, Hyderabad",
		Lat:         17.4435,
		Lng:         78.4485,
		Source:      "osm",
	}
	err := engine.ProcessSighting(context.Background(), schoolSighting)
	assert.NoError(t, err)
	// Should NOT be added to company store
	assert.Empty(t, mockStore.Companies)

	// Tech company sighting
	techSighting := model.Sighting{
		CompanyName: "Persistent Systems",
		RawAddress:  "Senapati Bapat Road, Pune",
		Lat:         18.5204,
		Lng:         73.8567,
		Source:      "wikidata",
	}
	err = engine.ProcessSighting(context.Background(), techSighting)
	assert.NoError(t, err)
	// Should be added to company store
	assert.Len(t, mockStore.Companies, 1)
}
