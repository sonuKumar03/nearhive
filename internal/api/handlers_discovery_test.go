package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/sonukumar/nearhive/internal/verifier"
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

func setupDiscoveryBatchRouter(workerToken string) (http.Handler, *store.MockStore) {
	mockStore := store.NewMockStore()
	authMgr := auth.NewManager("test-jwt-secret-very-long-enough-32bytes", 24*time.Hour)
	engine := verifier.NewEngine(mockStore, nil)
	discHandler := NewDiscoveryHandler(mockStore, engine, workerToken)
	router := NewRouterWithQueue(mockStore, authMgr, nil, nil, "test-jwt-secret-very-long-enough-32bytes", discHandler)
	return router, mockStore
}

func TestDiscoveryIngest_Authentication(t *testing.T) {
	workerToken := "correct-secret-worker-token-xyz"
	router, _ := setupDiscoveryBatchRouter(workerToken)

	batchPayload := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "test_source",
		SourceFamily:    "public_directory",
		ObservedAt:      time.Now(),
		Companies:       []model.CompanyEvidence{},
		Jobs:            []model.TechnicalJobEvidence{},
	}
	body, err := json.Marshal(batchPayload)
	require.NoError(t, err)

	// 1. Missing X-NearHive-Worker-Token header -> 401
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusUnauthorized, w1.Code, "missing worker token must be unauthorized")

	// 2. Wrong token -> 401
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-NearHive-Worker-Token", "wrong-worker-token")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code, "wrong worker token must be unauthorized")

	// 3. Unconfigured / empty server token -> 401 even if token passed
	routerNoToken, _ := setupDiscoveryBatchRouter("")
	req3, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-NearHive-Worker-Token", "some-token")
	w3 := httptest.NewRecorder()
	routerNoToken.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusUnauthorized, w3.Code, "endpoint must be disabled / unauthorized when server worker token is empty")

	// 4. Correct token -> passes auth (should not be 401)
	req4, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(body))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("X-NearHive-Worker-Token", workerToken)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)
	assert.NotEqual(t, http.StatusUnauthorized, w4.Code, "valid token must not return 401")
}

func TestDiscoveryIngest_EnvelopeValidation(t *testing.T) {
	workerToken := "test-worker-token"
	router, _ := setupDiscoveryBatchRouter(workerToken)

	// 1. Unknown contract version (must be 1)
	invalidVersionPayload := map[string]any{
		"contract_version": 2,
		"source":           "test_source",
		"source_family":    "public_directory",
		"observed_at":      time.Now().Format(time.RFC3339),
		"companies":        []any{},
		"jobs":             []any{},
	}
	body, _ := json.Marshal(invalidVersionPayload)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-NearHive-Worker-Token", workerToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "unknown contract version must return 400")

	// 2. Payload over 2 MiB limit
	mockStore := store.NewMockStore()
	authMgr := auth.NewManager("test-jwt-secret-very-long-enough-32bytes", 24*time.Hour)
	discHandler := NewDiscoveryHandler(mockStore, nil, workerToken).WithLimits(500, 1024) // 1 KiB limit for testing
	rLimited := NewRouterWithQueue(mockStore, authMgr, nil, nil, "test-jwt-secret-very-long-enough-32bytes", discHandler)

	bigString := strings.Repeat("A", 2048)
	bigPayload := map[string]any{
		"contract_version": 1,
		"source":           "test_source",
		"source_family":    "public_directory",
		"observed_at":      time.Now().Format(time.RFC3339),
		"companies": []map[string]any{
			{"name": bigString},
		},
		"jobs": []any{},
	}
	bigBody, _ := json.Marshal(bigPayload)
	reqBig, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(bigBody))
	reqBig.Header.Set("Content-Type", "application/json")
	reqBig.Header.Set("X-NearHive-Worker-Token", workerToken)
	wBig := httptest.NewRecorder()
	rLimited.ServeHTTP(wBig, reqBig)
	assert.True(t, wBig.Code == http.StatusRequestEntityTooLarge || wBig.Code == http.StatusBadRequest,
		"payload exceeding max body bytes must return 413 or 400, got: %d", wBig.Code)

	// 3. More than 500 records
	manyCompanies := make([]map[string]any, 300)
	for i := range manyCompanies {
		manyCompanies[i] = map[string]any{"name": fmt.Sprintf("Company %d", i)}
	}
	manyJobs := make([]map[string]any, 201)
	for i := range manyJobs {
		manyJobs[i] = map[string]any{"title": fmt.Sprintf("Job %d", i), "company_name": "Acme"}
	}
	overLimitPayload := map[string]any{
		"contract_version": 1,
		"source":           "test_source",
		"source_family":    "public_directory",
		"observed_at":      time.Now().Format(time.RFC3339),
		"companies":        manyCompanies,
		"jobs":             manyJobs,
	}
	overLimitBody, _ := json.Marshal(overLimitPayload)
	reqOver, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(overLimitBody))
	reqOver.Header.Set("Content-Type", "application/json")
	reqOver.Header.Set("X-NearHive-Worker-Token", workerToken)
	wOver := httptest.NewRecorder()
	router.ServeHTTP(wOver, reqOver)
	assert.Equal(t, http.StatusBadRequest, wOver.Code, "payload with > 500 records must return 400")
}

func TestDiscoveryIngest_PerRecordValidation(t *testing.T) {
	workerToken := "test-worker-token"
	router, _ := setupDiscoveryBatchRouter(workerToken)

	validLat := 12.9716
	validLng := 77.5946
	invalidLat := 95.0
	invalidLng := -185.0
	futurePostedAt := time.Now().Add(365 * 24 * time.Hour) // 1 year in future

	batch := map[string]any{
		"contract_version": 1,
		"source":           "mixed_validator",
		"source_family":    "open_dataset",
		"observed_at":      time.Now().Format(time.RFC3339),
		"companies": []map[string]any{
			// 0: Valid company
			{
				"name":         "Valid Company",
				"lat":          validLat,
				"lng":          validLng,
				"evidence_url": "https://example.com/valid",
			},
			// 1: Invalid coordinates (lat > 90)
			{
				"name": "Invalid Coords Company",
				"lat":  invalidLat,
				"lng":  validLng,
			},
			// 2: Unsupported URL scheme (ftp://)
			{
				"name":         "Bad URL Company",
				"evidence_url": "ftp://example.com/bad",
			},
			// 3: Missing company name
			{
				"name": "",
			},
		},
		"jobs": []map[string]any{
			// 0: Valid job
			{
				"company_name":  "Valid Company",
				"title":         "Senior Go Developer",
				"canonical_url": "https://example.com/job/1",
				"lat":           validLat,
				"lng":           validLng,
			},
			// 1: Invalid coordinates (lng < -180)
			{
				"company_name": "Valid Company",
				"title":        "Frontend Engineer",
				"lat":          validLat,
				"lng":          invalidLng,
			},
			// 2: Unsupported URL scheme (file://)
			{
				"company_name":  "Valid Company",
				"title":         "DevOps Engineer",
				"canonical_url": "file:///etc/passwd",
			},
			// 3: Invalid timestamp (posted_at in distant future)
			{
				"company_name": "Valid Company",
				"title":        "ML Engineer",
				"posted_at":    futurePostedAt.Format(time.RFC3339),
			},
		},
	}

	body, err := json.Marshal(batch)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-NearHive-Worker-Token", workerToken)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Partial acceptance must return HTTP 200
	require.Equal(t, http.StatusOK, w.Code, "mixed valid/invalid batch must return HTTP 200")

	var resp struct {
		Companies []struct {
			Index  int    `json:"index"`
			Status string `json:"status"`
			ID     string `json:"id,omitempty"`
			Error  string `json:"error,omitempty"`
		} `json:"companies"`
		Jobs []struct {
			Index  int    `json:"index"`
			Status string `json:"status"`
			ID     string `json:"id,omitempty"`
			Error  string `json:"error,omitempty"`
		} `json:"jobs"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Validate Companies results
	require.Len(t, resp.Companies, 4)
	assert.Equal(t, 0, resp.Companies[0].Index)
	assert.Equal(t, "accepted", resp.Companies[0].Status)
	assert.NotEmpty(t, resp.Companies[0].ID)
	assert.Empty(t, resp.Companies[0].Error)

	assert.Equal(t, 1, resp.Companies[1].Index)
	assert.Equal(t, "rejected", resp.Companies[1].Status)
	assert.NotEmpty(t, resp.Companies[1].Error)

	assert.Equal(t, 2, resp.Companies[2].Index)
	assert.Equal(t, "rejected", resp.Companies[2].Status)
	assert.NotEmpty(t, resp.Companies[2].Error)

	assert.Equal(t, 3, resp.Companies[3].Index)
	assert.Equal(t, "rejected", resp.Companies[3].Status)
	assert.NotEmpty(t, resp.Companies[3].Error)

	// Validate Jobs results
	require.Len(t, resp.Jobs, 4)
	assert.Equal(t, 0, resp.Jobs[0].Index)
	assert.Equal(t, "accepted", resp.Jobs[0].Status)
	assert.NotEmpty(t, resp.Jobs[0].ID)
	assert.Empty(t, resp.Jobs[0].Error)

	assert.Equal(t, 1, resp.Jobs[1].Index)
	assert.Equal(t, "rejected", resp.Jobs[1].Status)
	assert.NotEmpty(t, resp.Jobs[1].Error)

	assert.Equal(t, 2, resp.Jobs[2].Index)
	assert.Equal(t, "rejected", resp.Jobs[2].Status)
	assert.NotEmpty(t, resp.Jobs[2].Error)

	assert.Equal(t, 3, resp.Jobs[3].Index)
	assert.Equal(t, "rejected", resp.Jobs[3].Status)
	assert.NotEmpty(t, resp.Jobs[3].Error)
}

func TestDiscoveryIdempotency_API(t *testing.T) {
	workerToken := "test-worker-token"
	router, mockStore := setupDiscoveryBatchRouter(workerToken)

	t1 := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	t2 := time.Now().Truncate(time.Second)

	// Case 1: Company evidence with (source, source_record_id)
	batchComp1 := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "test_api_src",
		SourceFamily:    "official_site",
		ObservedAt:      t1,
		Companies: []model.CompanyEvidence{
			{
				SourceRecordID: "api-rec-1",
				Name:           "API Test Company",
				Address:        "100 Tech Park",
			},
		},
	}
	body1, _ := json.Marshal(batchComp1)
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-NearHive-Worker-Token", workerToken)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Second submission of same (source, source_record_id) with t2
	batchComp2 := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "test_api_src",
		SourceFamily:    "official_site",
		ObservedAt:      t2,
		Companies: []model.CompanyEvidence{
			{
				SourceRecordID: "api-rec-1",
				Name:           "API Test Company",
				Address:        "100 Tech Park Suite 200",
			},
		},
	}
	body2, _ := json.Marshal(batchComp2)
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-NearHive-Worker-Token", workerToken)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Assert exactly 1 sighting for test_api_src with api-rec-1 and last_seen_at updated
	var foundSightings []*model.Sighting
	for _, s := range mockStore.Sightings {
		if s.Source == "test_api_src" && s.SourceRecordID != nil && *s.SourceRecordID == "api-rec-1" {
			foundSightings = append(foundSightings, s)
		}
	}
	require.Len(t, foundSightings, 1, "Should have exactly 1 sighting row for source and source_record_id")
	assert.WithinDuration(t, t2, foundSightings[0].LastSeenAt, 2*time.Second, "last_seen_at should be updated")

	// Case 2: Company evidence with missing source_record_id and identical content_hash
	hashKey := "content-hash-api-999"
	batchHash1 := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "test_hash_api_src",
		SourceFamily:    "open_dataset",
		ObservedAt:      t1,
		Companies: []model.CompanyEvidence{
			{
				ContentHash: hashKey,
				Name:        "Hash Test Company",
				Address:     "200 Data Lane",
			},
		},
	}
	bHash1, _ := json.Marshal(batchHash1)
	rHash1, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(bHash1))
	rHash1.Header.Set("Content-Type", "application/json")
	rHash1.Header.Set("X-NearHive-Worker-Token", workerToken)
	wHash1 := httptest.NewRecorder()
	router.ServeHTTP(wHash1, rHash1)
	require.Equal(t, http.StatusOK, wHash1.Code)

	batchHash2 := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "test_hash_api_src",
		SourceFamily:    "open_dataset",
		ObservedAt:      t2,
		Companies: []model.CompanyEvidence{
			{
				ContentHash: hashKey,
				Name:        "Hash Test Company",
				Address:     "200 Data Lane",
			},
		},
	}
	bHash2, _ := json.Marshal(batchHash2)
	rHash2, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(bHash2))
	rHash2.Header.Set("Content-Type", "application/json")
	rHash2.Header.Set("X-NearHive-Worker-Token", workerToken)
	wHash2 := httptest.NewRecorder()
	router.ServeHTTP(wHash2, rHash2)
	require.Equal(t, http.StatusOK, wHash2.Code)

	var foundHashSightings []*model.Sighting
	for _, s := range mockStore.Sightings {
		if s.Source == "test_hash_api_src" && s.ContentHash != nil && *s.ContentHash == hashKey {
			foundHashSightings = append(foundHashSightings, s)
		}
	}
	require.Len(t, foundHashSightings, 1, "Should have exactly 1 sighting row for source and content_hash")
	assert.WithinDuration(t, t2, foundHashSightings[0].LastSeenAt, 2*time.Second, "last_seen_at should be updated")

	// Case 3: Technical job with (source, source_job_id)
	batchJob1 := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "job_board_src",
		SourceFamily:    "job_ats",
		ObservedAt:      t1,
		Jobs: []model.TechnicalJobEvidence{
			{
				SourceJobID:             "job-rec-1",
				CompanyName:             "API Test Company",
				Title:                   "Backend Software Engineer",
				ContentHash:             "jhash-1",
				TechnicalClassification: "software_engineering",
				RuleVersion:             "v1",
				FirstSeenAt:             t1,
				LastSeenAt:              t1,
			},
		},
	}
	bJob1, _ := json.Marshal(batchJob1)
	rJob1, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(bJob1))
	rJob1.Header.Set("Content-Type", "application/json")
	rJob1.Header.Set("X-NearHive-Worker-Token", workerToken)
	wJob1 := httptest.NewRecorder()
	router.ServeHTTP(wJob1, rJob1)
	require.Equal(t, http.StatusOK, wJob1.Code)

	batchJob2 := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "job_board_src",
		SourceFamily:    "job_ats",
		ObservedAt:      t2,
		Jobs: []model.TechnicalJobEvidence{
			{
				SourceJobID:             "job-rec-1",
				CompanyName:             "API Test Company",
				Title:                   "Backend Software Engineer (Updated)",
				ContentHash:             "jhash-1-diff",
				TechnicalClassification: "software_engineering",
				RuleVersion:             "v1",
				FirstSeenAt:             t1,
				LastSeenAt:              t2,
			},
		},
	}
	bJob2, _ := json.Marshal(batchJob2)
	rJob2, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(bJob2))
	rJob2.Header.Set("Content-Type", "application/json")
	rJob2.Header.Set("X-NearHive-Worker-Token", workerToken)
	wJob2 := httptest.NewRecorder()
	router.ServeHTTP(wJob2, rJob2)
	require.Equal(t, http.StatusOK, wJob2.Code)

	var foundJobs []*model.TechnicalJobPosting
	for _, j := range mockStore.TechnicalJobs {
		if j.Source == "job_board_src" && j.SourceJobID != nil && *j.SourceJobID == "job-rec-1" {
			foundJobs = append(foundJobs, j)
		}
	}
	require.Len(t, foundJobs, 1, "Should have exactly 1 job row for source and source_job_id")
	assert.WithinDuration(t, t2, foundJobs[0].LastSeenAt, 2*time.Second, "job last_seen_at should be updated")

	// Case 4: Technical job with missing source_job_id and identical content_hash
	jobHashKey := "tech-job-hash-api-555"
	batchJobHash1 := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "job_hash_src",
		SourceFamily:    "official_site",
		ObservedAt:      t1,
		Jobs: []model.TechnicalJobEvidence{
			{
				CompanyName:             "API Test Company",
				Title:                   "Infrastructure Engineer",
				ContentHash:             jobHashKey,
				TechnicalClassification: "infrastructure",
				RuleVersion:             "v1",
				FirstSeenAt:             t1,
				LastSeenAt:              t1,
			},
		},
	}
	bJHash1, _ := json.Marshal(batchJobHash1)
	rJHash1, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(bJHash1))
	rJHash1.Header.Set("Content-Type", "application/json")
	rJHash1.Header.Set("X-NearHive-Worker-Token", workerToken)
	wJHash1 := httptest.NewRecorder()
	router.ServeHTTP(wJHash1, rJHash1)
	require.Equal(t, http.StatusOK, wJHash1.Code)

	batchJobHash2 := model.DiscoveryBatch{
		ContractVersion: 1,
		Source:          "job_hash_src",
		SourceFamily:    "official_site",
		ObservedAt:      t2,
		Jobs: []model.TechnicalJobEvidence{
			{
				CompanyName:             "API Test Company",
				Title:                   "Infrastructure Engineer",
				ContentHash:             jobHashKey,
				TechnicalClassification: "infrastructure",
				RuleVersion:             "v1",
				FirstSeenAt:             t1,
				LastSeenAt:              t2,
			},
		},
	}
	bJHash2, _ := json.Marshal(batchJobHash2)
	rJHash2, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/discovery/batches", bytes.NewReader(bJHash2))
	rJHash2.Header.Set("Content-Type", "application/json")
	rJHash2.Header.Set("X-NearHive-Worker-Token", workerToken)
	wJHash2 := httptest.NewRecorder()
	router.ServeHTTP(wJHash2, rJHash2)
	require.Equal(t, http.StatusOK, wJHash2.Code)

	var foundHashJobs []*model.TechnicalJobPosting
	for _, j := range mockStore.TechnicalJobs {
		if j.Source == "job_hash_src" && j.ContentHash == jobHashKey {
			foundHashJobs = append(foundHashJobs, j)
		}
	}
	require.Len(t, foundHashJobs, 1, "Should have exactly 1 job row for source and content_hash")
	assert.WithinDuration(t, t2, foundHashJobs[0].LastSeenAt, 2*time.Second, "job last_seen_at should be updated")
}

