package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (http.Handler, *store.MockStore, *auth.Manager) {
	mockStore := store.NewMockStore()
	authMgr := auth.NewManager("test-jwt-secret-very-long-enough-32bytes", 24*time.Hour)
	router := NewRouter(mockStore, authMgr, nil, "test-jwt-secret-very-long-enough-32bytes")
	return router, mockStore, authMgr
}

func TestAuthRegisterAndLogin(t *testing.T) {
	router, _, _ := setupTestRouter()

	// 1. Register
	regBody, _ := json.Marshal(map[string]string{
		"email":    "founder@nearhive.com",
		"password": "Password123!",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var regResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	assert.NotEmpty(t, regResp["token"])

	// 2. Login
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "founder@nearhive.com",
		"password": "Password123!",
	})
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var loginResp map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &loginResp)
	assert.NotEmpty(t, loginResp["token"])
}

func TestSearch_RequiresAuth(t *testing.T) {
	router, mockStore, authMgr := setupTestRouter()

	// Unauthenticated request should fail with 401
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/search?lat=12.9716&lng=77.5946&radius=15", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Authenticated request
	token, _ := authMgr.GenerateToken(uuid.New())
	// Seed one company
	cID := uuid.New()
	_ = mockStore.CreateCompany(nil, &model.Company{ID: cID, Name: "Infosys"})
	_ = mockStore.CreateLocation(nil, &model.Location{CompanyID: cID, Lat: 12.9716, Lng: 77.5946, Confidence: 0.9})

	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/search?lat=12.9716&lng=77.5946&radius=15", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var searchResp map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &searchResp)
	companies := searchResp["companies"].([]any)
	assert.Len(t, companies, 1)
}

func TestHealthEndpoints(t *testing.T) {
	router, _, _ := setupTestRouter()

	// 1. GET /health
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. HEAD /health
	reqHead, _ := http.NewRequest(http.MethodHead, "/health", nil)
	wHead := httptest.NewRecorder()
	router.ServeHTTP(wHead, reqHead)
	assert.Equal(t, http.StatusOK, wHead.Code)

	// 3. GET /health/ready
	reqReady, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	wReady := httptest.NewRecorder()
	router.ServeHTTP(wReady, reqReady)
	assert.Equal(t, http.StatusOK, wReady.Code)
}
