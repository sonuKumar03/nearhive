package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/sonukumar/nearhive/internal/verifier"
)

type DiscoveryHandler struct {
	store           store.Store
	engine          *verifier.Engine
	workerToken     string
	maxBatchRecords int
	maxBodyBytes    int64
}

func NewDiscoveryHandler(s store.Store, engine *verifier.Engine, workerToken string) *DiscoveryHandler {
	return &DiscoveryHandler{
		store:           s,
		engine:          engine,
		workerToken:     workerToken,
		maxBatchRecords: 500,
		maxBodyBytes:    2097152, // 2 MiB
	}
}

func (h *DiscoveryHandler) WithLimits(maxRecords int, maxBodyBytes int64) *DiscoveryHandler {
	if maxRecords > 0 {
		h.maxBatchRecords = maxRecords
	}
	if maxBodyBytes > 0 {
		h.maxBodyBytes = maxBodyBytes
	}
	return h
}


type CreateDiscoveryJobRequest struct {
	Lat      *float64 `json:"lat"`
	Lng      *float64 `json:"lng"`
	RadiusKM *float64 `json:"radius_km"`
	Radius   *float64 `json:"radius"`
}

func (h *DiscoveryHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	if userID == uuid.Nil {
		JSONError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED", nil)
		return
	}

	var req CreateDiscoveryJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body", "VALIDATION_ERROR", nil)
		return
	}

	if req.Lat == nil || req.Lng == nil {
		JSONError(w, http.StatusBadRequest, "latitude and longitude are required", "VALIDATION_ERROR", nil)
		return
	}

	if *req.Lat < -90.0 || *req.Lat > 90.0 {
		JSONError(w, http.StatusBadRequest, "latitude must be between -90 and 90", "VALIDATION_ERROR", nil)
		return
	}

	if *req.Lng < -180.0 || *req.Lng > 180.0 {
		JSONError(w, http.StatusBadRequest, "longitude must be between -180 and 180", "VALIDATION_ERROR", nil)
		return
	}

	var radius float64
	if req.RadiusKM != nil {
		radius = *req.RadiusKM
	} else if req.Radius != nil {
		radius = *req.Radius
	} else {
		JSONError(w, http.StatusBadRequest, "radius_km is required", "VALIDATION_ERROR", nil)
		return
	}

	if radius <= 0.0 || radius > 100.0 {
		JSONError(w, http.StatusBadRequest, "radius must be between 0 and 100 km", "VALIDATION_ERROR", nil)
		return
	}

	job := &model.DiscoveryJob{
		ID:          uuid.New(),
		UserID:      userID,
		Status:      model.DiscoveryStatusPending,
		Lat:         *req.Lat,
		Lng:         *req.Lng,
		RadiusKM:    radius,
		MaxAttempts: 3,
		Attempts:    0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.store.CreateDiscoveryJob(r.Context(), job); err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to create discovery job", "INTERNAL_ERROR", nil)
		return
	}

	JSON(w, http.StatusCreated, job)
}

func (h *DiscoveryHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	if userID == uuid.Nil {
		JSONError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED", nil)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	jobs, err := h.store.ListDiscoveryJobs(r.Context(), userID, limit, offset)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to list discovery jobs", "INTERNAL_ERROR", nil)
		return
	}
	if jobs == nil {
		jobs = []model.DiscoveryJob{}
	}

	JSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (h *DiscoveryHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "invalid job id", "VALIDATION_ERROR", nil)
		return
	}

	userID := GetUserIDFromContext(r.Context())
	if userID == uuid.Nil {
		JSONError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED", nil)
		return
	}

	job, err := h.store.GetDiscoveryJob(r.Context(), id, userID)
	if errors.Is(err, store.ErrNotFound) {
		JSONError(w, http.StatusNotFound, "discovery job not found", "NOT_FOUND", nil)
		return
	}
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to get discovery job", "INTERNAL_ERROR", nil)
		return
	}

	JSON(w, http.StatusOK, job)
}

func (h *DiscoveryHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "invalid job id", "VALIDATION_ERROR", nil)
		return
	}

	userID := GetUserIDFromContext(r.Context())
	if userID == uuid.Nil {
		JSONError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED", nil)
		return
	}

	err = h.store.CancelDiscoveryJob(r.Context(), id, userID)
	if errors.Is(err, store.ErrNotFound) {
		JSONError(w, http.StatusNotFound, "discovery job not found", "NOT_FOUND", nil)
		return
	}
	if errors.Is(err, store.ErrInvalidJobState) {
		JSONError(w, http.StatusConflict, "job cannot be cancelled in its current state", "CONFLICT", nil)
		return
	}
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to cancel discovery job", "INTERNAL_ERROR", nil)
		return
	}

	job, err := h.store.GetDiscoveryJob(r.Context(), id, userID)
	if err != nil {
		JSON(w, http.StatusOK, map[string]any{
			"message": "job cancelled successfully",
		})
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"message": "job cancelled successfully",
		"job":     job,
	})
}

func (h *DiscoveryHandler) IngestBatch(w http.ResponseWriter, r *http.Request) {
	// 1. Worker token authentication with constant-time compare
	token := r.Header.Get("X-NearHive-Worker-Token")
	if h.workerToken == "" || token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(h.workerToken)) != 1 {
		JSONError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED", nil)
		return
	}

	// 2. Bound request body size
	maxBytes := h.maxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 2097152 // 2 MiB default
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	// 3. Decode payload
	var batch model.DiscoveryBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			JSONError(w, http.StatusRequestEntityTooLarge, "request body too large", "PAYLOAD_TOO_LARGE", nil)
			return
		}
		JSONError(w, http.StatusBadRequest, "invalid request body", "VALIDATION_ERROR", nil)
		return
	}

	// 4. Validate contract version
	if batch.ContractVersion != 1 {
		JSONError(w, http.StatusBadRequest, "unsupported contract version; must be 1", "VALIDATION_ERROR", nil)
		return
	}

	// 5. Validate batch record limits
	maxRecords := h.maxBatchRecords
	if maxRecords <= 0 {
		maxRecords = 500
	}
	totalRecords := len(batch.Companies) + len(batch.Jobs)
	if totalRecords > maxRecords {
		JSONError(w, http.StatusBadRequest, fmt.Sprintf("batch contains %d records which exceeds max limit of %d", totalRecords, maxRecords), "VALIDATION_ERROR", nil)
		return
	}

	now := time.Now()
	observedAt := batch.ObservedAt
	if observedAt.IsZero() {
		observedAt = now
	} else if observedAt.After(now.Add(5 * time.Minute)) {
		JSONError(w, http.StatusBadRequest, "observed_at cannot be in the future", "VALIDATION_ERROR", nil)
		return
	}

	for i, j := range batch.Jobs {
		if !isValidWorkArrangement(j.WorkArrangement) {
			JSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid work_arrangement '%s' at job index %d", j.WorkArrangement, i), "VALIDATION_ERROR", nil)
			return
		}
		if !isValidPublicationState(j.PublicationState) {
			JSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid publication_state '%s' at job index %d", j.PublicationState, i), "VALIDATION_ERROR", nil)
			return
		}
	}

	companiesResults := make([]BatchRecordResult, 0, len(batch.Companies))
	for i, c := range batch.Companies {
		if strings.TrimSpace(c.Name) == "" {
			companiesResults = append(companiesResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "company name is required",
			})
			continue
		}
		if !isValidCoordinates(c.Lat, c.Lng) {
			companiesResults = append(companiesResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "invalid coordinates",
			})
			continue
		}
		if c.EvidenceURL != "" && !isValidURLScheme(c.EvidenceURL) {
			companiesResults = append(companiesResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "unsupported URL scheme; must be http or https",
			})
			continue
		}

		contentHash := c.ContentHash
		if contentHash == "" {
			contentHash = computeContentHash(batch.Source, c.SourceRecordID, c.Name, c.Address)
		}

		sourceFamily := batch.SourceFamily
		if sourceFamily == "" {
			sourceFamily = verifier.InferSourceFamily(batch.Source, "")
		}

		var sourceRecID *string
		if c.SourceRecordID != "" {
			sourceRecID = &c.SourceRecordID
		}
		var sourceURL *string
		if c.EvidenceURL != "" {
			sourceURL = &c.EvidenceURL
		}
		var discJobID *uuid.UUID
		if batch.DiscoveryJobID != uuid.Nil {
			discJobID = &batch.DiscoveryJobID
		}

		var latVal, lngVal float64
		if c.Lat != nil {
			latVal = *c.Lat
		}
		if c.Lng != nil {
			lngVal = *c.Lng
		}

		meta := make(model.JSONMap)
		for k, v := range c.Metadata {
			meta[k] = v
		}
		if c.Domain != nil && *c.Domain != "" {
			meta["domain"] = *c.Domain
		}
		if c.Phone != nil && *c.Phone != "" {
			meta["phone"] = *c.Phone
		}

		sighting := model.Sighting{
			ID:             uuid.New(),
			Source:         batch.Source,
			SourceFamily:   sourceFamily,
			SourceRecordID: sourceRecID,
			ContentHash:    &contentHash,
			DiscoveryJobID: discJobID,
			SourceURL:      sourceURL,
			CompanyName:    c.Name,
			RawAddress:     c.Address,
			Lat:            latVal,
			Lng:            lngVal,
			Metadata:       meta,
			FirstSeenAt:    observedAt,
			LastSeenAt:     observedAt,
			ScrapedAt:      observedAt,
		}

		if err := h.store.UpsertDiscoverySighting(r.Context(), &sighting); err != nil {
			companiesResults = append(companiesResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "failed to store company evidence",
			})
			continue
		}

		if h.engine != nil {
			if err := h.engine.ProcessDiscoverySighting(r.Context(), sighting); err != nil {
				log.Printf("[Discovery] warning: failed to process discovery sighting %s: %v", sighting.ID, err)
			}
		}

		companiesResults = append(companiesResults, BatchRecordResult{
			Index:  i,
			Status: "accepted",
			ID:     sighting.ID.String(),
		})
	}

	jobsResults := make([]BatchRecordResult, 0, len(batch.Jobs))
	for i, j := range batch.Jobs {
		if strings.TrimSpace(j.CompanyName) == "" {
			jobsResults = append(jobsResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "company name is required",
			})
			continue
		}
		if strings.TrimSpace(j.Title) == "" {
			jobsResults = append(jobsResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "job title is required",
			})
			continue
		}
		if !isValidCoordinates(j.Lat, j.Lng) {
			jobsResults = append(jobsResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "invalid coordinates",
			})
			continue
		}
		if j.CanonicalURL != "" && !isValidURLScheme(j.CanonicalURL) {
			jobsResults = append(jobsResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "unsupported URL scheme; must be http or https",
			})
			continue
		}
		if !isValidPostedAt(j.PostedAt) {
			jobsResults = append(jobsResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "invalid timestamp; posted_at cannot be in the future",
			})
			continue
		}

		contentHash := j.ContentHash
		if contentHash == "" {
			contentHash = computeContentHash(batch.Source, j.SourceJobID, j.CompanyName, j.Title)
		}

		var companyID uuid.UUID
		if j.CompanyDomain != nil && *j.CompanyDomain != "" {
			c, err := h.store.FindByDomain(r.Context(), *j.CompanyDomain)
			if err == nil && c != nil {
				companyID = c.ID
			}
		}
		if companyID == uuid.Nil {
			norm := verifier.Normalize(j.CompanyName)
			c, err := h.store.FindByNormalizedName(r.Context(), norm)
			if err == nil && c != nil {
				companyID = c.ID
			}
		}
		if companyID == uuid.Nil {
			newComp := &model.Company{
				ID:             uuid.New(),
				Name:           j.CompanyName,
				NormalizedName: verifier.Normalize(j.CompanyName),
				Domain:         j.CompanyDomain,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := h.store.CreateCompany(r.Context(), newComp); err == nil {
				companyID = newComp.ID
			} else {
				norm := verifier.Normalize(j.CompanyName)
				if c, findErr := h.store.FindByNormalizedName(r.Context(), norm); findErr == nil && c != nil {
					companyID = c.ID
				} else {
					jobsResults = append(jobsResults, BatchRecordResult{
						Index:  i,
						Status: "rejected",
						Error:  "failed to associate company for technical job",
					})
					continue
				}
			}
		}

		var locationID *uuid.UUID
		if j.Lat != nil && j.Lng != nil {
			loc, err := h.store.FindNearbyLocation(r.Context(), companyID, *j.Lat, *j.Lng, 500)
			if err == nil && loc != nil {
				locationID = &loc.ID
			}
		}

		var sourceJobID *string
		if j.SourceJobID != "" {
			sourceJobID = &j.SourceJobID
		}
		var canonicalURL *string
		if j.CanonicalURL != "" {
			canonicalURL = &j.CanonicalURL
		}
		var locationRaw *string
		if j.LocationRaw != "" {
			locationRaw = &j.LocationRaw
		}
		var discJobID *uuid.UUID
		if batch.DiscoveryJobID != uuid.Nil {
			discJobID = &batch.DiscoveryJobID
		}

		workArr := j.WorkArrangement
		if workArr == "" {
			workArr = model.WorkArrangementUnknown
		} else if workArr == "onsite" {
			workArr = model.WorkArrangementInOffice
		}
		pubState := j.PublicationState
		if pubState == "" {
			pubState = model.PublicationStateObservedRecently
		}
		techClass := j.TechnicalClassification
		if techClass == "" {
			techClass = "software_engineering"
		}
		ruleVer := j.RuleVersion
		if ruleVer == "" {
			ruleVer = "v1"
		}

		firstSeen := j.FirstSeenAt
		if firstSeen.IsZero() {
			firstSeen = observedAt
		}
		lastSeen := j.LastSeenAt
		if lastSeen.IsZero() {
			lastSeen = observedAt
		}

		sourceFamily := batch.SourceFamily
		if sourceFamily == "" {
			sourceFamily = verifier.InferSourceFamily(batch.Source, "job_ats")
		}

		jobPosting := model.TechnicalJobPosting{
			ID:                      uuid.New(),
			CompanyID:               companyID,
			LocationID:              locationID,
			DiscoveryJobID:          discJobID,
			Source:                  batch.Source,
			SourceFamily:            sourceFamily,
			SourceJobID:             sourceJobID,
			CanonicalURL:            canonicalURL,
			Title:                   j.Title,
			NormalizedTitle:         strings.ToLower(strings.TrimSpace(j.Title)),
			DescriptionExcerpt:      j.DescriptionExcerpt,
			ContentHash:             contentHash,
			LocationRaw:             locationRaw,
			Lat:                     j.Lat,
			Lng:                     j.Lng,
			WorkArrangement:         workArr,
			PublicationState:        pubState,
			PostedAt:                j.PostedAt,
			PostedAtConfidence:      j.PostedAtConfidence,
			FirstSeenAt:             firstSeen,
			LastSeenAt:              lastSeen,
			IsActive:                true,
			TechnicalClassification: techClass,
			RuleVersion:             ruleVer,
			ClassificationReasons:   j.ClassificationReasons,
			Metadata:                j.Metadata,
			CreatedAt:               now,
			UpdatedAt:               now,
		}

		if err := h.store.UpsertTechnicalJob(r.Context(), &jobPosting); err != nil {
			jobsResults = append(jobsResults, BatchRecordResult{
				Index:  i,
				Status: "rejected",
				Error:  "failed to store technical job posting",
			})
			continue
		}

		if h.engine != nil && companyID != uuid.Nil {
			if err := h.engine.RecalculateLocationEvidence(r.Context(), companyID); err != nil {
				log.Printf("[Discovery] warning: failed to recalculate location evidence for company %s: %v", companyID, err)
			}
		}

		jobsResults = append(jobsResults, BatchRecordResult{
			Index:  i,
			Status: "accepted",
			ID:     jobPosting.ID.String(),
		})
	}

	JSON(w, http.StatusOK, DiscoveryBatchResponse{
		Companies: companiesResults,
		Jobs:      jobsResults,
	})
}

type BatchRecordResult struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
	Error  string `json:"error,omitempty"`
}

type DiscoveryBatchResponse struct {
	Companies []BatchRecordResult `json:"companies"`
	Jobs      []BatchRecordResult `json:"jobs"`
}

func isValidWorkArrangement(wa model.WorkArrangement) bool {
	switch wa {
	case "", model.WorkArrangementInOffice, model.WorkArrangementHybrid, model.WorkArrangementRemote, model.WorkArrangementUnknown, "onsite":
		return true
	default:
		return false
	}
}

func isValidPublicationState(ps model.PublicationState) bool {
	switch ps {
	case "", model.PublicationStatePostedRecently, model.PublicationStateObservedRecently, model.PublicationStateStale:
		return true
	default:
		return false
	}
}

func isValidURLScheme(rawURL string) bool {
	if rawURL == "" {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

func isValidCoordinates(lat, lng *float64) bool {
	if lat == nil && lng == nil {
		return true
	}
	if lat == nil || lng == nil {
		return false
	}
	if *lat < -90.0 || *lat > 90.0 {
		return false
	}
	if *lng < -180.0 || *lng > 180.0 {
		return false
	}
	return true
}

func isValidPostedAt(t *time.Time) bool {
	if t == nil {
		return true
	}
	if t.After(time.Now().Add(48 * time.Hour)) {
		return false
	}
	return true
}

func computeContentHash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte("|"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

