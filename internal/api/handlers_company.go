package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/store"
)

type CompanyHandler struct {
	store store.Store
}

func (h *CompanyHandler) GetCompany(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "invalid company id", "VALIDATION_ERROR", nil)
		return
	}

	company, err := h.store.GetCompanyByID(r.Context(), id)
	if err != nil {
		JSONError(w, http.StatusNotFound, "company not found", "NOT_FOUND", nil)
		return
	}

	locations, _ := h.store.GetLocationsByCompany(r.Context(), id)

	JSON(w, http.StatusOK, map[string]any{
		"company":   company,
		"locations": locations,
	})
}

func (h *CompanyHandler) GetSightings(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "invalid company id", "VALIDATION_ERROR", nil)
		return
	}

	sightings, err := h.store.GetSightingsByCompany(r.Context(), id)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to get sightings", "INTERNAL_ERROR", nil)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"sightings": sightings,
	})
}
