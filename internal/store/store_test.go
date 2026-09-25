package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestMockStore_CompanyAndLocationSearch(t *testing.T) {
	ctx := context.Background()
	mock := NewMockStore()

	companyID := uuid.New()
	c := &model.Company{
		ID:             companyID,
		Name:           "Tech Corp",
		NormalizedName: "tech corp",
	}
	err := mock.CreateCompany(ctx, c)
	assert.NoError(t, err)

	loc := &model.Location{
		CompanyID:  companyID,
		Address:    "Whitefield, Bangalore",
		Lat:        12.9854,
		Lng:        77.7366,
		Confidence: 0.8,
	}
	err = mock.CreateLocation(ctx, loc)
	assert.NoError(t, err)

	// Search within 5km from center close to location
	results, err := mock.Search(ctx, 12.9850, 77.7360, 5000, SearchOpts{})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Tech Corp", results[0].Name)

	// Search from far away (50km away) with 1km radius
	farResults, err := mock.Search(ctx, 13.5000, 78.0000, 1000, SearchOpts{})
	assert.NoError(t, err)
	assert.Empty(t, farResults)
}
