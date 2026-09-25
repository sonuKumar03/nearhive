package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/sonukumar/nearhive/internal/store"
)

type JobHandler struct {
	store        store.JobStore
	orchestrator *scraper.Orchestrator
}

type TriggerJobRequest struct {
	Region   string  `json:"region"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	RadiusKM float64 `json:"radius_km"`
}

func (h *JobHandler) TriggerJob(w http.ResponseWriter, r *http.Request) {
	var req TriggerJobRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Region == "" {
		req.Region = "Bangalore"
	}
	if req.RadiusKM <= 0 {
		req.RadiusKM = 15
	}

	job := &model.ScrapeJob{
		ID:        uuid.New(),
		Source:    "manual_trigger",
		Status:    "running",
		Region:    &req.Region,
		StartedAt: func() *time.Time { t := time.Now(); return &t }(),
	}
	_ = h.store.CreateJob(r.Context(), job)

	if h.orchestrator != nil {
		go func() {
			sightings, err := h.orchestrator.ScrapeRegion(r.Context(), scraper.ScrapeRequest{
				Region:   req.Region,
				Lat:      req.Lat,
				Lng:      req.Lng,
				RadiusKM: req.RadiusKM,
			})
			now := time.Now()
			job.FinishedAt = &now
			if err != nil {
				job.Status = "failed"
				errStr := err.Error()
				job.Error = &errStr
			} else {
				job.Status = "done"
				job.Sightings = len(sightings)
			}
			_ = h.store.UpdateJob(r.Context(), job)
		}()
	}

	JSON(w, http.StatusAccepted, job)
}

func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.store.ListJobs(r.Context(), 20, 0)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to list jobs", "INTERNAL_ERROR", nil)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "invalid job id", "VALIDATION_ERROR", nil)
		return
	}
	job, err := h.store.GetJobByID(r.Context(), id)
	if err != nil {
		JSONError(w, http.StatusNotFound, "job not found", "NOT_FOUND", nil)
		return
	}
	JSON(w, http.StatusOK, job)
}
