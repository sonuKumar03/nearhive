package api

import (
	"context"
	"encoding/json"
	"errors"
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
	taskMgr      *scraper.TaskManager
}

func NewJobHandler(s store.JobStore, o *scraper.Orchestrator, tm *scraper.TaskManager) *JobHandler {
	if tm == nil && o != nil {
		tm = o.TaskManager()
	}
	if tm == nil {
		tm = scraper.NewTaskManager()
	}
	return &JobHandler{
		store:        s,
		orchestrator: o,
		taskMgr:      tm,
	}
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
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		if h.taskMgr != nil {
			h.taskMgr.Register(job.ID, cancel)
		}

		go func() {
			if h.taskMgr != nil {
				defer h.taskMgr.Unregister(job.ID)
			}
			defer cancel()

			sightings, err := h.orchestrator.ScrapeRegion(bgCtx, scraper.ScrapeRequest{
				JobID:    job.ID,
				Region:   req.Region,
				Lat:      req.Lat,
				Lng:      req.Lng,
				RadiusKM: req.RadiusKM,
			})

			// Check if cancelled in DB before marking done or failed
			currentJob, getErr := h.store.GetJobByID(context.Background(), job.ID)
			if getErr == nil && currentJob != nil && currentJob.Status == "cancelled" {
				return
			}

			now := time.Now()
			job.FinishedAt = &now
			if errors.Is(bgCtx.Err(), context.Canceled) {
				job.Status = "cancelled"
				errMsg := "job cancelled by user"
				job.Error = &errMsg
			} else if err != nil {
				job.Status = "failed"
				errStr := err.Error()
				job.Error = &errStr
			} else {
				job.Status = "done"
				job.Sightings = len(sightings)
			}

			saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer saveCancel()
			_ = h.store.UpdateJob(saveCtx, job)
		}()
	}

	JSON(w, http.StatusAccepted, job)
}

func (h *JobHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
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

	if job.Status != "running" && job.Status != "pending" {
		JSON(w, http.StatusOK, map[string]any{
			"message": "job is already " + job.Status,
			"job":     job,
		})
		return
	}

	// Terminate active background context
	if h.taskMgr != nil {
		h.taskMgr.Cancel(id)
	}

	// Update DB record
	now := time.Now()
	job.Status = "cancelled"
	job.FinishedAt = &now
	errMsg := "job cancelled by user"
	job.Error = &errMsg
	_ = h.store.UpdateJob(r.Context(), job)

	JSON(w, http.StatusOK, map[string]any{
		"message": "job cancelled successfully",
		"job":     job,
	})
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
