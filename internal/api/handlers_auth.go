package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
)

type AuthHandler struct {
	store   store.UserStore
	authMgr *auth.Manager
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body", "VALIDATION_ERROR", nil)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 6 {
		JSONError(w, http.StatusBadRequest, "email required and password must be >= 6 chars", "VALIDATION_ERROR", nil)
		return
	}

	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to process password", "INTERNAL_ERROR", nil)
		return
	}

	user := &model.User{
		ID:       uuid.New(),
		Email:    req.Email,
		Password: hashed,
	}

	if err := h.store.CreateUser(r.Context(), user); err != nil {
		JSONError(w, http.StatusConflict, "user already exists", "CONFLICT", nil)
		return
	}

	token, err := h.authMgr.GenerateToken(user.ID)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to generate token", "INTERNAL_ERROR", nil)
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"token": token,
		"user": map[string]any{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body", "VALIDATION_ERROR", nil)
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), strings.ToLower(req.Email))
	if err != nil || !auth.CheckPasswordHash(req.Password, user.Password) {
		JSONError(w, http.StatusUnauthorized, "invalid email or password", "UNAUTHORIZED", nil)
		return
	}

	token, err := h.authMgr.GenerateToken(user.ID)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "failed to generate token", "INTERNAL_ERROR", nil)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}
