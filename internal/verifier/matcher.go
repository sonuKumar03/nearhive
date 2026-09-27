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

func (m *Matcher) FindDiscoveryMatch(ctx context.Context, s model.Sighting) (*MatchResult, error) {
	// 1. Exact canonical domain match
	domain := extractDomainFromMetadata(s.Metadata)
	if domain != "" {
		if company, err := m.store.FindByDomain(ctx, domain); err == nil && company != nil {
			return &MatchResult{
				CompanyID:  company.ID,
				Confidence: 0.9,
				MatchType:  "domain",
			}, nil
		}
	}

	// 2 & 3. Exact normalized name match near same location or corroborated
	normalized := Normalize(s.CompanyName)
	if normalized != "" {
		if company, err := m.store.FindByNormalizedName(ctx, normalized); err == nil && company != nil {
			locStore, hasLocStore := m.store.(store.LocationStore)
			if hasLocStore {
				locs, err := locStore.GetLocationsByCompany(ctx, company.ID)
				if err == nil {
					if len(locs) == 0 {
						// Company has no locations yet; exact normalized name matches
						return &MatchResult{
							CompanyID:  company.ID,
							Confidence: 0.8,
							MatchType:  "exact_name",
						}, nil
					}

					// Check location proximity (near same location, e.g. within 5000m)
					if s.Lat != 0 && s.Lng != 0 {
						nearby, err := locStore.FindNearbyLocation(ctx, company.ID, s.Lat, s.Lng, 5000)
						if err == nil && nearby != nil {
							return &MatchResult{
								CompanyID:  company.ID,
								Confidence: 0.8,
								MatchType:  "exact_name_location",
							}, nil
						}
					}

					// Check address match
					if s.RawAddress != "" {
						cleanAddr := strings.ToLower(strings.TrimSpace(s.RawAddress))
						for _, loc := range locs {
							existingAddr := strings.ToLower(strings.TrimSpace(loc.Address))
							if cleanAddr == existingAddr || (len(cleanAddr) > 5 && strings.Contains(existingAddr, cleanAddr)) || (len(existingAddr) > 5 && strings.Contains(cleanAddr, existingAddr)) {
								return &MatchResult{
									CompanyID:  company.ID,
									Confidence: 0.8,
									MatchType:  "exact_name_address",
								}, nil
							}
						}
					}

					// Check phone match in metadata
					if s.Metadata != nil {
						if phone, ok := s.Metadata["phone"].(string); ok && phone != "" {
							cleanPhone := strings.TrimSpace(phone)
							if sightStore, ok := m.store.(store.SightingStore); ok {
								companySightings, err := sightStore.GetSightingsByCompany(ctx, company.ID)
								if err == nil {
									for _, cs := range companySightings {
										if cs.Metadata != nil {
											if csPhone, ok := cs.Metadata["phone"].(string); ok && strings.TrimSpace(csPhone) == cleanPhone {
												return &MatchResult{
													CompanyID:  company.ID,
													Confidence: 0.8,
													MatchType:  "exact_name_phone",
												}, nil
											}
										}
									}
								}
							}
						}
					}
				}
			} else {
				// No location store available; exact normalized name matches
				return &MatchResult{
					CompanyID:  company.ID,
					Confidence: 0.7,
					MatchType:  "exact_name",
				}, nil
			}
		}
	}

	// 4. Fuzzy name only marks a possible match; it cannot merge records automatically.
	// Return nil, nil so that ambiguous candidates remain separate rather than risking a destructive merge.
	return nil, nil
}

func extractDomainFromMetadata(meta model.JSONMap) string {
	if meta == nil {
		return ""
	}
	for _, key := range []string{"website", "url", "domain", "company_domain"} {
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

