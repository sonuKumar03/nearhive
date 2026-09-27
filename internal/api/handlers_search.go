package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type SearchHandler struct {
	store   store.Store
	history store.SearchHistoryStore
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	var lat, lng float64
	var latSet, lngSet bool
	radiusKM := 15.0
	var opts store.SearchOpts
	limit := 50
	page := 1

	if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			Lat           *float64 `json:"lat"`
			Lng           *float64 `json:"lng"`
			Radius        *float64 `json:"radius"`
			RadiusKM      *float64 `json:"radius_km"`
			MinConfidence *float64 `json:"min_confidence"`
			Industry      *string  `json:"industry"`
			Query         *string  `json:"q"`
			Limit         *int     `json:"limit"`
			Page          *int     `json:"page"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.Lat != nil {
				lat = *req.Lat
				latSet = true
			}
			if req.Lng != nil {
				lng = *req.Lng
				lngSet = true
			}
			if req.RadiusKM != nil && *req.RadiusKM > 0 && *req.RadiusKM <= 100 {
				radiusKM = *req.RadiusKM
			} else if req.Radius != nil && *req.Radius > 0 && *req.Radius <= 100 {
				radiusKM = *req.Radius
			}
			if req.MinConfidence != nil {
				opts.MinConfidence = req.MinConfidence
			}
			if req.Industry != nil && *req.Industry != "" {
				opts.Industry = req.Industry
			}
			if req.Query != nil && *req.Query != "" {
				opts.Query = req.Query
			}
			if req.Limit != nil && *req.Limit > 0 && *req.Limit <= 100 {
				limit = *req.Limit
			}
			if req.Page != nil && *req.Page > 0 {
				page = *req.Page
			}
		}
	}

	if !latSet || !lngSet {
		latStr := r.URL.Query().Get("lat")
		lngStr := r.URL.Query().Get("lng")
		if latStr == "" || lngStr == "" {
			JSONError(w, http.StatusBadRequest, "lat and lng query parameters are required", "VALIDATION_ERROR", nil)
			return
		}

		var err1, err2 error
		lat, err1 = strconv.ParseFloat(latStr, 64)
		lng, err2 = strconv.ParseFloat(lngStr, 64)
		if err1 != nil || err2 != nil {
			JSONError(w, http.StatusBadRequest, "lat and lng must be valid numbers", "VALIDATION_ERROR", nil)
			return
		}
	}

	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		JSONError(w, http.StatusBadRequest, "latitude must be between -90 and 90, longitude between -180 and 180", "VALIDATION_ERROR", nil)
		return
	}

	if rStr := r.URL.Query().Get("radius"); rStr != "" {
		if rVal, err := strconv.ParseFloat(rStr, 64); err == nil && rVal > 0 && rVal <= 100 {
			radiusKM = rVal
		}
	}

	if confStr := r.URL.Query().Get("min_confidence"); confStr != "" {
		if confVal, err := strconv.ParseFloat(confStr, 64); err == nil {
			opts.MinConfidence = &confVal
		}
	}
	if ind := r.URL.Query().Get("industry"); ind != "" {
		opts.Industry = &ind
	}
	if q := r.URL.Query().Get("q"); q != "" {
		opts.Query = &q
	}

	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if lVal, err := strconv.Atoi(lStr); err == nil && lVal > 0 && lVal <= 100 {
			limit = lVal
		}
	}
	opts.Limit = limit

	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = pVal
		}
	}
	opts.Offset = (page - 1) * limit

	results, err := h.store.Search(r.Context(), lat, lng, radiusKM*1000, opts)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to query companies", "INTERNAL_ERROR", nil)
		return
	}

	total, _ := h.store.CountSearch(r.Context(), lat, lng, radiusKM*1000, opts)

	userID := GetUserIDFromContext(r.Context())
	if userID != uuid.Nil && h.history != nil {
		_ = h.history.RecordSearch(r.Context(), &model.SearchHistory{
			UserID:      userID,
			QueryLat:    lat,
			QueryLng:    lng,
			RadiusKM:    radiusKM,
			ResultCount: len(results),
		})
	}

	JSON(w, http.StatusOK, map[string]any{
		"meta": map[string]any{
			"total":     total,
			"page":      page,
			"limit":     limit,
			"radius_km": radiusKM,
			"center": map[string]float64{
				"lat": lat,
				"lng": lng,
			},
		},
		"companies": results,
	})
}

func (h *SearchHandler) SearchClusters(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")
	if latStr == "" || lngStr == "" {
		JSONError(w, http.StatusBadRequest, "lat and lng query parameters are required", "VALIDATION_ERROR", nil)
		return
	}

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lng, err2 := strconv.ParseFloat(lngStr, 64)
	if err1 != nil || err2 != nil {
		JSONError(w, http.StatusBadRequest, "lat and lng must be valid numbers", "VALIDATION_ERROR", nil)
		return
	}

	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		JSONError(w, http.StatusBadRequest, "latitude must be between -90 and 90, longitude between -180 and 180", "VALIDATION_ERROR", nil)
		return
	}

	radiusKM := 15.0
	if rStr := r.URL.Query().Get("radius"); rStr != "" {
		if rVal, err := strconv.ParseFloat(rStr, 64); err == nil && rVal > 0 && rVal <= 100 {
			radiusKM = rVal
		}
	}

	k := 20
	if kStr := r.URL.Query().Get("k"); kStr != "" {
		if kVal, err := strconv.Atoi(kStr); err == nil && kVal > 0 && kVal <= 100 {
			k = kVal
		}
	}

	clusters, err := h.store.ClusterSearch(r.Context(), lat, lng, radiusKM*1000, k)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to compute spatial clusters: "+err.Error(), "INTERNAL_ERROR", nil)
		return
	}

	totalPoints := 0
	for _, c := range clusters {
		totalPoints += c.Count
	}

	JSON(w, http.StatusOK, map[string]any{
		"meta": map[string]any{
			"k":             k,
			"cluster_count": len(clusters),
			"total_points":  totalPoints,
			"radius_km":     radiusKM,
			"center": map[string]float64{
				"lat": lat,
				"lng": lng,
			},
		},
		"clusters": clusters,
	})
}

func (h *SearchHandler) SearchJobs(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")
	if latStr == "" || lngStr == "" {
		JSONError(w, http.StatusBadRequest, "lat and lng query parameters are required", "VALIDATION_ERROR", nil)
		return
	}

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lng, err2 := strconv.ParseFloat(lngStr, 64)
	if err1 != nil || err2 != nil {
		JSONError(w, http.StatusBadRequest, "lat and lng must be valid numbers", "VALIDATION_ERROR", nil)
		return
	}

	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		JSONError(w, http.StatusBadRequest, "latitude must be between -90 and 90, longitude between -180 and 180", "VALIDATION_ERROR", nil)
		return
	}

	radiusKM := 15.0
	if rStr := r.URL.Query().Get("radius"); rStr != "" {
		if rVal, err := strconv.ParseFloat(rStr, 64); err == nil && rVal > 0 && rVal <= 100 {
			radiusKM = rVal
		}
	}

	var opts store.TechnicalJobSearchOpts

	if q := r.URL.Query().Get("q"); q != "" {
		opts.Query = &q
	}

	if arrStr := r.URL.Query().Get("work_arrangement"); arrStr != "" {
		switch arrStr {
		case "in_office", "onsite":
			arr := model.WorkArrangementInOffice
			opts.WorkArrangement = &arr
		case "hybrid":
			arr := model.WorkArrangementHybrid
			opts.WorkArrangement = &arr
		case "remote":
			arr := model.WorkArrangementRemote
			opts.WorkArrangement = &arr
		case "unknown":
			arr := model.WorkArrangementUnknown
			opts.WorkArrangement = &arr
		default:
			JSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid work_arrangement '%s'", arrStr), "VALIDATION_ERROR", nil)
			return
		}
	}

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if lVal, err := strconv.Atoi(lStr); err == nil && lVal > 0 && lVal <= 100 {
			limit = lVal
		}
	}
	opts.Limit = limit

	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = pVal
		}
	}
	opts.Offset = (page - 1) * limit

	results, err := h.store.SearchTechnicalJobs(r.Context(), lat, lng, radiusKM*1000, opts)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to query technical jobs", "INTERNAL_ERROR", nil)
		return
	}

	total, _ := h.store.CountTechnicalJobSearch(r.Context(), lat, lng, radiusKM*1000, opts)

	JSON(w, http.StatusOK, map[string]any{
		"meta": map[string]any{
			"total":     total,
			"page":      page,
			"limit":     limit,
			"radius_km": radiusKM,
			"center": map[string]float64{
				"lat": lat,
				"lng": lng,
			},
		},
		"jobs": results,
	})
}
