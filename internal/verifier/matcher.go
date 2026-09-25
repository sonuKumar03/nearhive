package verifier

import (
	"context"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type MatchResult struct {
	CompanyID  uuid.UUID
	Confidence float64
	MatchType  string
}

type Matcher struct {
	store store.CompanyStore
}

func NewMatcher(s store.CompanyStore) *Matcher {
	return &Matcher{store: s}
}

func (m *Matcher) FindMatch(ctx context.Context, s model.Sighting) (*MatchResult, error) {
	// 1. Check domain match from metadata
	if domain := extractDomainFromMetadata(s.Metadata); domain != "" {
		if company, err := m.store.FindByDomain(ctx, domain); err == nil && company != nil {
			return &MatchResult{
				CompanyID:  company.ID,
				Confidence: 0.5,
				MatchType:  "domain",
			}, nil
		}
	}

	// 2. Check exact normalized name match
	normalized := Normalize(s.CompanyName)
	if normalized != "" {
		if company, err := m.store.FindByNormalizedName(ctx, normalized); err == nil && company != nil {
			return &MatchResult{
				CompanyID:  company.ID,
				Confidence: 0.4,
				MatchType:  "exact_name",
			}, nil
		}

		// 3. Check fuzzy match
		if company, err := m.store.FindByFuzzyName(ctx, normalized, 0.6); err == nil && company != nil {
			return &MatchResult{
				CompanyID:  company.ID,
				Confidence: 0.2,
				MatchType:  "fuzzy_name",
			}, nil
		}
	}

	return nil, nil
}

func extractDomainFromMetadata(meta model.JSONMap) string {
	if meta == nil {
		return ""
	}
	for _, key := range []string{"website", "url", "domain"} {
		if val, ok := meta[key].(string); ok && val != "" {
			u, err := url.Parse(val)
			if err == nil && u.Host != "" {
				host := strings.ToLower(u.Host)
				return strings.TrimPrefix(host, "www.")
			}
			return strings.ToLower(strings.TrimPrefix(val, "www."))
		}
	}
	return ""
}
