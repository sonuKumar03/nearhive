package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/sonukumar/nearhive/internal/verifier"
)

type DiscoveryHandler struct {
	store       store.Store
	engine      *verifier.Engine
	workerToken string
}

func NewDiscoveryHandler(s store.Store, engine *verifier.Engine, workerToken string) *DiscoveryHandler {
	return &DiscoveryHandler{
		store:       s,
		engine:      engine,
		workerToken: workerToken,
	}
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
