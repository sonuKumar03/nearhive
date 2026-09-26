package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryJobs_AuthRequired(t *testing.T) {
	router, _, _ := setupTestRouter()
	jobID := uuid.New().String()

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/discovery/jobs", `{"lat":12.9716,"lng":77.5946,"radius_km":15}`},
		{http.MethodGet, "/api/v1/discovery/jobs", ""},
		{http.MethodGet, "/api/v1/discovery/jobs/" + jobID, ""},
		{http.MethodPost, "/api/v1/discovery/jobs/" + jobID + "/cancel", ""},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s %s", ep.method, ep.path), func(t *testing.T) {
			var bodyReader *bytes.Reader
			if ep.body != "" {
				bodyReader = bytes.NewReader([]byte(ep.body))
			} else {
				bodyReader = bytes.NewReader(nil)
			}
			req, err := http.NewRequest(ep.method, ep.path, bodyReader)
			require.NoError(t, err)
			if ep.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code, "endpoint %s %s should require auth", ep.method, ep.path)
		})
	}
}

func TestDiscoveryJobs_CoordinateAndRadiusValidation(t *testing.T) {
	router, _, authMgr := setupTestRouter()
	token, err := authMgr.GenerateToken(uuid.New())
	require.NoError(t, err)

	testCases := []struct {
		name       string
		payload    map[string]any
		rawBody    string
		wantStatus int
	}{
		{
			name:       "valid minimum bounds (-90, -180, small radius)",
			payload:    map[string]any{"lat": -90.0, "lng": -180.0, "radius_km": 0.1},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "valid maximum bounds (90, 180, 100km radius)",
			payload:    map[string]any{"lat": 90.0, "lng": 180.0, "radius_km": 100.0},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "valid origin coordinates (0, 0, 15km)",
			payload:    map[string]any{"lat": 0.0, "lng": 0.0, "radius_km": 15.0},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid latitude below -90",
			payload:    map[string]any{"lat": -90.001, "lng": 77.5946, "radius_km": 15.0},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid latitude above 90",
			payload:    map[string]any{"lat": 90.001, "lng": 77.5946, "radius_km": 15.0},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid longitude below -180",
			payload:    map[string]any{"lat": 12.9716, "lng": -180.001, "radius_km": 15.0},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid longitude above 180",
			payload:    map[string]any{"lat": 12.9716, "lng": 180.001, "radius_km": 15.0},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid radius zero (must be > 0)",
			payload:    map[string]any{"lat": 12.9716, "lng": 77.5946, "radius_km": 0.0},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid radius negative",
			payload:    map[string]any{"lat": 12.9716, "lng": 77.5946, "radius_km": -5.0},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid radius above 100km",
			payload:    map[string]any{"lat": 12.9716, "lng": 77.5946, "radius_km": 100.001},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing lat",
			payload:    map[string]any{"lng": 77.5946, "radius_km": 15.0},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing lng",
			payload:    map[string]any{"lat": 12.9716, "radius_km": 15.0},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing radius",
			payload:    map[string]any{"lat": 12.9716, "lng": 77.5946},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed JSON body",
			rawBody:    `{"lat": 12.9716, "lng": }`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			if tc.rawBody != "" {
				body = []byte(tc.rawBody)
			} else {
				var err error
				body, err = json.Marshal(tc.payload)
				require.NoError(t, err)
			}

			req, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs", bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code, "case '%s' status mismatch", tc.name)
		})
	}
}

func TestDiscoveryJobs_CreateAndInitialPendingStatus(t *testing.T) {
	router, mockStore, authMgr := setupTestRouter()
	userID := uuid.New()
	token, err := authMgr.GenerateToken(userID)
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"lat":       12.9716,
		"lng":       77.5946,
		"radius_km": 20.0,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var createdJob model.DiscoveryJob
	err = json.Unmarshal(w.Body.Bytes(), &createdJob)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, createdJob.ID)
	assert.Equal(t, userID, createdJob.UserID)
	assert.Equal(t, model.DiscoveryStatusPending, createdJob.Status)
	assert.Equal(t, 12.9716, createdJob.Lat)
	assert.Equal(t, 77.5946, createdJob.Lng)
	assert.Equal(t, 20.0, createdJob.RadiusKM)

	// Check DB persistence
	stored, err := mockStore.GetDiscoveryJob(nil, createdJob.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, model.DiscoveryStatusPending, stored.Status)
	assert.Equal(t, userID, stored.UserID)
}

func TestDiscoveryJobs_UserIsolation(t *testing.T) {
	router, _, authMgr := setupTestRouter()

	user1ID := uuid.New()
	user1Token, err := authMgr.GenerateToken(user1ID)
	require.NoError(t, err)

	user2ID := uuid.New()
	user2Token, err := authMgr.GenerateToken(user2ID)
	require.NoError(t, err)

	// 1. User 1 creates a discovery job
	body, err := json.Marshal(map[string]any{
		"lat":       12.9716,
		"lng":       77.5946,
		"radius_km": 25.0,
	})
	require.NoError(t, err)

	reqCreate, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	reqCreate.Header.Set("Authorization", "Bearer "+user1Token)
	reqCreate.Header.Set("Content-Type", "application/json")

	wCreate := httptest.NewRecorder()
	router.ServeHTTP(wCreate, reqCreate)
	require.Equal(t, http.StatusCreated, wCreate.Code)

	var createdJob model.DiscoveryJob
	err = json.Unmarshal(wCreate.Body.Bytes(), &createdJob)
	require.NoError(t, err)
	jobID := createdJob.ID.String()

	// 2. User 2 cannot read User 1's job
	reqGet, err := http.NewRequest(http.MethodGet, "/api/v1/discovery/jobs/"+jobID, nil)
	require.NoError(t, err)
	reqGet.Header.Set("Authorization", "Bearer "+user2Token)

	wGet := httptest.NewRecorder()
	router.ServeHTTP(wGet, reqGet)
	assert.Equal(t, http.StatusNotFound, wGet.Code)

	// 3. User 2 cannot cancel User 1's job
	reqCancel, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs/"+jobID+"/cancel", nil)
	require.NoError(t, err)
	reqCancel.Header.Set("Authorization", "Bearer "+user2Token)

	wCancel := httptest.NewRecorder()
	router.ServeHTTP(wCancel, reqCancel)
	assert.Equal(t, http.StatusNotFound, wCancel.Code)

	// 4. User 2's job list does not contain User 1's job
	reqList, err := http.NewRequest(http.MethodGet, "/api/v1/discovery/jobs", nil)
	require.NoError(t, err)
	reqList.Header.Set("Authorization", "Bearer "+user2Token)

	wList := httptest.NewRecorder()
	router.ServeHTTP(wList, reqList)
	assert.Equal(t, http.StatusOK, wList.Code)

	var listResp map[string]any
	err = json.Unmarshal(wList.Body.Bytes(), &listResp)
	require.NoError(t, err)
	jobsList := listResp["jobs"].([]any)
	assert.Empty(t, jobsList)

	// 5. User 1 CAN read the job
	reqGetOwner, err := http.NewRequest(http.MethodGet, "/api/v1/discovery/jobs/"+jobID, nil)
	require.NoError(t, err)
	reqGetOwner.Header.Set("Authorization", "Bearer "+user1Token)

	wGetOwner := httptest.NewRecorder()
	router.ServeHTTP(wGetOwner, reqGetOwner)
	assert.Equal(t, http.StatusOK, wGetOwner.Code)

	var fetchedJob model.DiscoveryJob
	err = json.Unmarshal(wGetOwner.Body.Bytes(), &fetchedJob)
	require.NoError(t, err)
	assert.Equal(t, createdJob.ID, fetchedJob.ID)

	// 6. User 1's job list contains the job
	reqListOwner, err := http.NewRequest(http.MethodGet, "/api/v1/discovery/jobs", nil)
	require.NoError(t, err)
	reqListOwner.Header.Set("Authorization", "Bearer "+user1Token)

	wListOwner := httptest.NewRecorder()
	router.ServeHTTP(wListOwner, reqListOwner)
	assert.Equal(t, http.StatusOK, wListOwner.Code)

	var listOwnerResp map[string]any
	err = json.Unmarshal(wListOwner.Body.Bytes(), &listOwnerResp)
	require.NoError(t, err)
	ownerJobs := listOwnerResp["jobs"].([]any)
	assert.Len(t, ownerJobs, 1)
}

func TestDiscoveryJobs_CancellationIdempotent(t *testing.T) {
	router, mockStore, authMgr := setupTestRouter()
	userID := uuid.New()
	token, err := authMgr.GenerateToken(userID)
	require.NoError(t, err)

	// Create job
	body, err := json.Marshal(map[string]any{
		"lat":       12.9716,
		"lng":       77.5946,
		"radius_km": 15.0,
	})
	require.NoError(t, err)

	reqCreate, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs", bytes.NewReader(body))
	require.NoError(t, err)
	reqCreate.Header.Set("Authorization", "Bearer "+token)
	reqCreate.Header.Set("Content-Type", "application/json")

	wCreate := httptest.NewRecorder()
	router.ServeHTTP(wCreate, reqCreate)
	require.Equal(t, http.StatusCreated, wCreate.Code)

	var createdJob model.DiscoveryJob
	err = json.Unmarshal(wCreate.Body.Bytes(), &createdJob)
	require.NoError(t, err)
	jobID := createdJob.ID.String()

	// First cancellation
	reqCancel1, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs/"+jobID+"/cancel", nil)
	require.NoError(t, err)
	reqCancel1.Header.Set("Authorization", "Bearer "+token)

	wCancel1 := httptest.NewRecorder()
	router.ServeHTTP(wCancel1, reqCancel1)
	assert.Equal(t, http.StatusOK, wCancel1.Code)

	// Second cancellation (idempotent)
	reqCancel2, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs/"+jobID+"/cancel", nil)
	require.NoError(t, err)
	reqCancel2.Header.Set("Authorization", "Bearer "+token)

	wCancel2 := httptest.NewRecorder()
	router.ServeHTTP(wCancel2, reqCancel2)
	assert.Equal(t, http.StatusOK, wCancel2.Code)

	// Third cancellation (idempotent)
	reqCancel3, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs/"+jobID+"/cancel", nil)
	require.NoError(t, err)
	reqCancel3.Header.Set("Authorization", "Bearer "+token)

	wCancel3 := httptest.NewRecorder()
	router.ServeHTTP(wCancel3, reqCancel3)
	assert.Equal(t, http.StatusOK, wCancel3.Code)

	// Verify DB state is cancelled
	stored, err := mockStore.GetDiscoveryJob(nil, createdJob.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, model.DiscoveryStatusCancelled, stored.Status)
}

func TestDiscoveryJobs_InvalidJobIDs(t *testing.T) {
	router, _, authMgr := setupTestRouter()
	token, err := authMgr.GenerateToken(uuid.New())
	require.NoError(t, err)

	invalidID := "not-a-valid-uuid"

	// GET invalid ID
	reqGet, err := http.NewRequest(http.MethodGet, "/api/v1/discovery/jobs/"+invalidID, nil)
	require.NoError(t, err)
	reqGet.Header.Set("Authorization", "Bearer "+token)

	wGet := httptest.NewRecorder()
	router.ServeHTTP(wGet, reqGet)
	assert.Equal(t, http.StatusBadRequest, wGet.Code)

	// POST cancel invalid ID
	reqCancel, err := http.NewRequest(http.MethodPost, "/api/v1/discovery/jobs/"+invalidID+"/cancel", nil)
	require.NoError(t, err)
	reqCancel.Header.Set("Authorization", "Bearer "+token)

	wCancel := httptest.NewRecorder()
	router.ServeHTTP(wCancel, reqCancel)
	assert.Equal(t, http.StatusBadRequest, wCancel.Code)
}
