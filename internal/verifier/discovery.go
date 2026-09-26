package verifier

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

// ProcessDiscoverySighting processes a sighting discovered through the Python discovery pipeline.
// Unlike ProcessSighting (used by the crawler), this does NOT apply the tech-company admission filter,
// retaining all discovered entities (such as hospitals, retailers, schools, etc.).
// It uses FindDiscoveryMatch for strict corroboration and recalculates deterministic location evidence.
func (e *Engine) ProcessDiscoverySighting(ctx context.Context, s model.Sighting) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}

	// Infer source family if not populated
	if s.SourceFamily == "" && s.Metadata != nil {
		if sf, ok := s.Metadata["source_family"].(string); ok && sf != "" {
			s.SourceFamily = sf
		}
	}
	if s.SourceFamily == "" {
		s.SourceFamily = InferSourceFamily(s.Source, "")
	}

	// 1. Strict discovery match (domain or corroborated exact identity, never fuzzy name alone)
	match, err := e.matcher.FindDiscoveryMatch(ctx, s)
	if err != nil {
		return err
	}

	// Ensure sighting is stored so spatial evidence queries can find it
	if e.store != nil {
		_ = e.store.SaveSightings(ctx, s.Source, []model.Sighting{s})
	}

	// 2. Merge sighting into company & spatial clusters
	if err := e.merger.MergeSighting(ctx, s, match); err != nil {
		return err
	}

	// 3. Determine company ID
	var companyID uuid.UUID
	if match != nil {
		companyID = match.CompanyID
	} else if e.store != nil {
		normName := Normalize(s.CompanyName)
		c, err := e.store.FindByNormalizedName(ctx, normName)
		if err == nil && c != nil {
			companyID = c.ID
		}
	}

	// 4. Deterministic location evidence presence recalculation
	if companyID != uuid.Nil {
		return e.RecalculateLocationEvidence(ctx, companyID)
	}

	return nil
}

// RecalculateLocationEvidence evaluates the distinct independent evidence families
// across all spatially matched locations for a company and updates each location's presence type,
// confidence, and verified status.
func (e *Engine) RecalculateLocationEvidence(ctx context.Context, companyID uuid.UUID) error {
	if e.store == nil {
		return nil
	}

	summaries, err := e.store.GetLocationEvidenceSummaries(ctx, companyID)
	if err != nil {
		return err
	}

	for _, summary := range summaries {
		presence, conf, verified := EvaluatePresence(summary.SourceFamilies)
		if err := e.store.UpdateLocationPresence(ctx, summary.LocationID, presence, conf, verified); err != nil {
			return err
		}
	}

	return nil
}

// EvaluatePresence derives presence_type, confidence, and verified status from distinct source families.
// - confirmed_office: official company evidence plus another independent family, or two independent non-syndicated families agreeing spatially. Sets confirmed_office, 0.9, verified=true.
// - probable_office: one credible location source. Sets probable_office, 0.6, verified=false.
// - job_location_only: a recent job names the location but no credible office evidence exists. Sets job_location_only, 0.4, verified=false.
// Ten sightings from one source family (e.g. job_ats) must NOT produce confirmed_office.
func EvaluatePresence(sourceFamilies []string) (model.PresenceType, float64, bool) {
	familySet := make(map[string]bool)
	hasOfficial := false
	distinctOfficeFamilies := 0
	distinctJobFamilies := 0

	for _, rawFam := range sourceFamilies {
		fam := strings.TrimSpace(rawFam)
		if fam == "" {
			continue
		}
		if familySet[fam] {
			continue
		}
		familySet[fam] = true

		switch fam {
		case "official_site":
			hasOfficial = true
			distinctOfficeFamilies++
		case "job_ats":
			distinctJobFamilies++
		default:
			// e.g. public_directory, open_dataset, government_registry, etc.
			distinctOfficeFamilies++
		}
	}

	totalDistinctFamilies := len(familySet)

	// 1. confirmed_office: official evidence + another independent family OR 2 independent non-syndicated families agreeing spatially
	if (hasOfficial && totalDistinctFamilies >= 2) || distinctOfficeFamilies >= 2 || (distinctOfficeFamilies >= 1 && distinctJobFamilies >= 1) {
		return model.PresenceTypeConfirmedOffice, 0.9, true
	}

	// 2. probable_office: one credible office location source
	if distinctOfficeFamilies >= 1 {
		return model.PresenceTypeProbableOffice, 0.6, false
	}

	// 3. job_location_only: a recent job names the location but no credible office evidence exists
	if distinctJobFamilies >= 1 {
		return model.PresenceTypeJobLocationOnly, 0.4, false
	}

	// Default fallback
	return model.PresenceTypeProbableOffice, 0.1, false
}

// InferSourceFamily maps legacy or short source names to their canonical source family.
func InferSourceFamily(source, sourceFamily string) string {
	if sourceFamily != "" {
		return sourceFamily
	}
	switch strings.ToLower(source) {
	case "official_site", "company_site", "official":
		return "official_site"
	case "public_directory", "justdial", "google", "techpark", "mca":
		return "public_directory"
	case "open_dataset", "osm", "wikidata":
		return "open_dataset"
	case "job_ats", "greenhouse", "lever", "jobportal":
		return "job_ats"
	default:
		return source
	}
}
