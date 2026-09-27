package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSearchClusters(t *testing.T) {
	router, mockStore, authMgr := setupTestRouter()
	token, _ := authMgr.GenerateToken(uuid.New())

	// 1. Unauthenticated request should fail
	reqUnauth, _ := http.NewRequest(http.MethodGet, "/api/v1/search/clusters?lat=12.9716&lng=77.5946&radius=15", nil)
	wUnauth := httptest.NewRecorder()
	router.ServeHTTP(wUnauth, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, wUnauth.Code)

	// 2. Missing lat/lng
	reqMissing, _ := http.NewRequest(http.MethodGet, "/api/v1/search/clusters", nil)
	reqMissing.Header.Set("Authorization", "Bearer "+token)
	wMissing := httptest.NewRecorder()
	router.ServeHTTP(wMissing, reqMissing)
	assert.Equal(t, http.StatusBadRequest, wMissing.Code)

	// 3. Seed locations
	cID := uuid.New()
	_ = mockStore.CreateCompany(nil, &model.Company{ID: cID, Name: "Tech Hub Corp"})
	_ = mockStore.CreateLocation(nil, &model.Location{CompanyID: cID, Lat: 12.9716, Lng: 77.5946})
	_ = mockStore.CreateLocation(nil, &model.Location{CompanyID: cID, Lat: 12.9720, Lng: 77.5950})
	_ = mockStore.CreateLocation(nil, &model.Location{CompanyID: cID, Lat: 12.9730, Lng: 77.5960})

	// 4. Query clusters with k=2
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/search/clusters?lat=12.9716&lng=77.5946&radius=15&k=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	meta := resp["meta"].(map[string]any)
	assert.Equal(t, float64(2), meta["k"])
	assert.Equal(t, float64(3), meta["total_points"])

	clusters := resp["clusters"].([]any)
	assert.Len(t, clusters, 2)
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

func TestJobs_TriggerListGetCancel(t *testing.T) {
	router, mockStore, authMgr := setupTestRouter()
	token, _ := authMgr.GenerateToken(uuid.New())

	// 1. Trigger a job
	body, _ := json.Marshal(map[string]any{
		"region":    "Bangalore",
		"lat":       12.9716,
		"lng":       77.5946,
		"radius_km": 15,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs/trigger", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var createdJob model.ScrapeJob
	_ = json.Unmarshal(w.Body.Bytes(), &createdJob)
	assert.NotEmpty(t, createdJob.ID)
	assert.Equal(t, "running", createdJob.Status)

	// 2. List jobs
	reqList, _ := http.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	wList := httptest.NewRecorder()
	router.ServeHTTP(wList, reqList)
	assert.Equal(t, http.StatusOK, wList.Code)

	var listResp map[string]any
	_ = json.Unmarshal(wList.Body.Bytes(), &listResp)
	jobsList := listResp["jobs"].([]any)
	assert.GreaterOrEqual(t, len(jobsList), 1)

	// 3. Get job details
	reqGet, _ := http.NewRequest(http.MethodGet, "/api/v1/jobs/"+createdJob.ID.String(), nil)
	reqGet.Header.Set("Authorization", "Bearer "+token)
	wGet := httptest.NewRecorder()
	router.ServeHTTP(wGet, reqGet)
	assert.Equal(t, http.StatusOK, wGet.Code)

	var fetchedJob model.ScrapeJob
	_ = json.Unmarshal(wGet.Body.Bytes(), &fetchedJob)
	assert.Equal(t, createdJob.ID, fetchedJob.ID)

	// Add a subtask to mockStore to verify GetJob returns tasks
	taskID := uuid.New()
	_ = mockStore.CreateTask(nil, &model.ScrapeTask{
		ID:     taskID,
		JobID:  createdJob.ID,
		Source: "osm",
		Status: "running",
	})
	reqGetWithTasks, _ := http.NewRequest(http.MethodGet, "/api/v1/jobs/"+createdJob.ID.String(), nil)
	reqGetWithTasks.Header.Set("Authorization", "Bearer "+token)
	wGetWithTasks := httptest.NewRecorder()
	router.ServeHTTP(wGetWithTasks, reqGetWithTasks)
	assert.Equal(t, http.StatusOK, wGetWithTasks.Code)
	var fetchedJobWithTasks model.ScrapeJob
	_ = json.Unmarshal(wGetWithTasks.Body.Bytes(), &fetchedJobWithTasks)
	assert.Len(t, fetchedJobWithTasks.Tasks, 1)
	assert.Equal(t, "osm", fetchedJobWithTasks.Tasks[0].Source)

	// 4. Cancel the job
	reqCancel, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs/"+createdJob.ID.String()+"/cancel", nil)
	reqCancel.Header.Set("Authorization", "Bearer "+token)
	wCancel := httptest.NewRecorder()
	router.ServeHTTP(wCancel, reqCancel)
	assert.Equal(t, http.StatusOK, wCancel.Code)

	var cancelResp map[string]any
	_ = json.Unmarshal(wCancel.Body.Bytes(), &cancelResp)
	assert.Equal(t, "job cancelled successfully", cancelResp["message"])

	// Check DB state
	storedJob, err := mockStore.GetJobByID(nil, createdJob.ID)
	assert.NoError(t, err)
	assert.Equal(t, "cancelled", storedJob.Status)
	assert.NotNil(t, storedJob.FinishedAt)
}

func TestSearchDiscovery(t *testing.T) {
	router, mockStore, authMgr := setupTestRouter()
	token, _ := authMgr.GenerateToken(uuid.New())
	ctx := t.Context()

	centerLat := 12.9716
	centerLng := 77.6412
	now := time.Now().Truncate(time.Millisecond)
	fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)
	twentyDaysAgo := now.Add(-20 * 24 * time.Hour)
	twoDaysAgo := now.Add(-2 * 24 * time.Hour)

	// 1. Company with confirmed_office, 2 trustworthy recent jobs (in_office, remote),
	// 1 stale job (>14d), 1 observed_recently (untrusted posted date)
	c1ID := uuid.New()
	require.NoError(t, mockStore.CreateCompany(ctx, &model.Company{ID: c1ID, Name: "Alpha Corp"}))
	require.NoError(t, mockStore.CreateLocation(ctx, &model.Location{
		CompanyID:    c1ID,
		Address:      "Indiranagar 100ft Rd",
		Lat:          centerLat + 0.001,
		Lng:          centerLng + 0.001,
		Confidence:   0.95,
		PresenceType: model.PresenceTypeConfirmedOffice,
		Verified:     true,
	}))
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		CompanyID:               c1ID,
		Source:                  "gh-1",
		SourceFamily:            "job_ats",
		Title:                   "Backend Engineer",
		NormalizedTitle:         "backend engineer",
		ContentHash:             "hash-c1-1",
		WorkArrangement:         model.WorkArrangementInOffice,
		PublicationState:        model.PublicationStatePostedRecently,
		PostedAt:                &fiveDaysAgo,
		PostedAtConfidence:      1.0,
		FirstSeenAt:             fiveDaysAgo,
		LastSeenAt:              now,
		IsActive:                true,
		TechnicalClassification: "software_engineering",
		RuleVersion:             "v1",
	}))
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		CompanyID:               c1ID,
		Source:                  "gh-2",
		SourceFamily:            "job_ats",
		Title:                   "Frontend Engineer",
		NormalizedTitle:         "frontend engineer",
		ContentHash:             "hash-c1-2",
		WorkArrangement:         model.WorkArrangementRemote,
		PublicationState:        model.PublicationStatePostedRecently,
		PostedAt:                &tenDaysAgo,
		PostedAtConfidence:      1.0,
		FirstSeenAt:             tenDaysAgo,
		LastSeenAt:              now,
		IsActive:                true,
		TechnicalClassification: "software_engineering",
		RuleVersion:             "v1",
	}))
	// Stale job (>14 days)
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		CompanyID:               c1ID,
		Source:                  "gh-3",
		SourceFamily:            "job_ats",
		Title:                   "DevOps Engineer",
		NormalizedTitle:         "devops engineer",
		ContentHash:             "hash-c1-3",
		WorkArrangement:         model.WorkArrangementHybrid,
		PublicationState:        model.PublicationStateStale,
		PostedAt:                &twentyDaysAgo,
		PostedAtConfidence:      1.0,
		FirstSeenAt:             twentyDaysAgo,
		LastSeenAt:              twentyDaysAgo,
		IsActive:                true,
		TechnicalClassification: "infrastructure",
		RuleVersion:             "v1",
	}))
	// Untrusted date / observed recently job
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		CompanyID:               c1ID,
		Source:                  "careers-4",
		SourceFamily:            "official_site",
		Title:                   "Data Scientist",
		NormalizedTitle:         "data scientist",
		ContentHash:             "hash-c1-4",
		WorkArrangement:         model.WorkArrangementHybrid,
		PublicationState:        model.PublicationStateObservedRecently,
		PostedAtConfidence:      0.0,
		FirstSeenAt:             twoDaysAgo,
		LastSeenAt:              twoDaysAgo,
		IsActive:                true,
		TechnicalClassification: "data_engineering",
		RuleVersion:             "v1",
	}))

	// 2. Company with probable_office, no jobs
	c2ID := uuid.New()
	require.NoError(t, mockStore.CreateCompany(ctx, &model.Company{ID: c2ID, Name: "Beta Tech"}))
	require.NoError(t, mockStore.CreateLocation(ctx, &model.Location{
		CompanyID:    c2ID,
		Address:      "Indiranagar 12th Main",
		Lat:          centerLat + 0.002,
		Lng:          centerLng + 0.002,
		Confidence:   0.7,
		PresenceType: model.PresenceTypeProbableOffice,
		Verified:     false,
	}))

	// 3. Company with job_location_only, 1 recent hybrid job
	c3ID := uuid.New()
	require.NoError(t, mockStore.CreateCompany(ctx, &model.Company{ID: c3ID, Name: "Gamma Labs"}))
	require.NoError(t, mockStore.CreateLocation(ctx, &model.Location{
		CompanyID:    c3ID,
		Address:      "Indiranagar Double Rd",
		Lat:          centerLat + 0.003,
		Lng:          centerLng + 0.003,
		Confidence:   0.5,
		PresenceType: model.PresenceTypeJobLocationOnly,
		Verified:     false,
	}))
	threeDaysAgo := now.Add(-3 * 24 * time.Hour)
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		CompanyID:               c3ID,
		Source:                  "lever-1",
		SourceFamily:            "job_ats",
		Title:                   "Security Analyst",
		NormalizedTitle:         "security analyst",
		ContentHash:             "hash-c3-1",
		WorkArrangement:         model.WorkArrangementHybrid,
		PublicationState:        model.PublicationStatePostedRecently,
		PostedAt:                &threeDaysAgo,
		PostedAtConfidence:      1.0,
		FirstSeenAt:             threeDaysAgo,
		LastSeenAt:              now,
		IsActive:                true,
		TechnicalClassification: "security",
		RuleVersion:             "v1",
	}))

	// 4. Remote-only company without physical location in radius (location in another city)
	c4ID := uuid.New()
	require.NoError(t, mockStore.CreateCompany(ctx, &model.Company{ID: c4ID, Name: "RemoteOnly Corp"}))
	require.NoError(t, mockStore.CreateLocation(ctx, &model.Location{
		CompanyID:    c4ID,
		Address:      "Remote HQ, Far Away",
		Lat:          18.5204, // Pune
		Lng:          73.8567,
		Confidence:   0.8,
		PresenceType: model.PresenceTypeConfirmedOffice,
	}))
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		CompanyID:               c4ID,
		Source:                  "remote-1",
		SourceFamily:            "job_ats",
		Title:                   "Staff Engineer (Remote)",
		NormalizedTitle:         "staff engineer remote",
		ContentHash:             "hash-c4-1",
		WorkArrangement:         model.WorkArrangementRemote,
		PublicationState:        model.PublicationStatePostedRecently,
		PostedAt:                &fiveDaysAgo,
		PostedAtConfidence:      1.0,
		FirstSeenAt:             fiveDaysAgo,
		LastSeenAt:              now,
		IsActive:                true,
		TechnicalClassification: "software_engineering",
		RuleVersion:             "v1",
	}))

	// Test GET /api/v1/search
	getURL := fmt.Sprintf("/api/v1/search?lat=%f&lng=%f&radius=5", centerLat, centerLng)
	reqGet, _ := http.NewRequest(http.MethodGet, getURL, nil)
	reqGet.Header.Set("Authorization", "Bearer "+token)
	wGet := httptest.NewRecorder()
	router.ServeHTTP(wGet, reqGet)

	require.Equal(t, http.StatusOK, wGet.Code)
	var respGet map[string]any
	require.NoError(t, json.Unmarshal(wGet.Body.Bytes(), &respGet))
	companies := respGet["companies"].([]any)
	require.Len(t, companies, 3, "Only c1, c2, c3 should be in spatial radius; c4 (remote only) must not appear")

	companyMap := make(map[string]map[string]any)
	for _, c := range companies {
		cmap := c.(map[string]any)
		companyMap[cmap["id"].(string)] = cmap
	}

	// Assert c1
	c1Data, ok := companyMap[c1ID.String()]
	require.True(t, ok, "Alpha Corp must be present")
	assert.Equal(t, "confirmed_office", c1Data["presence_type"])
	assert.Equal(t, float64(2), c1Data["recent_technical_job_count"], "Only trustworthy jobs within 14 days counted")
	c1Arr, ok := c1Data["arrangements"].([]any)
	require.True(t, ok, "arrangements should be present")
	assert.ElementsMatch(t, []any{"in_office", "remote"}, c1Arr)

	// Assert c2
	c2Data, ok := companyMap[c2ID.String()]
	require.True(t, ok, "Beta Tech must be present")
	assert.Equal(t, "probable_office", c2Data["presence_type"])
	assert.Equal(t, float64(0), c2Data["recent_technical_job_count"])

	// Assert c3
	c3Data, ok := companyMap[c3ID.String()]
	require.True(t, ok, "Gamma Labs must be present")
	assert.Equal(t, "job_location_only", c3Data["presence_type"])
	assert.Equal(t, float64(1), c3Data["recent_technical_job_count"])
	c3Arr, ok := c3Data["arrangements"].([]any)
	require.True(t, ok, "arrangements should be present")
	assert.ElementsMatch(t, []any{"hybrid"}, c3Arr)

	// Test POST /api/v1/search with JSON body
	postBody, _ := json.Marshal(map[string]any{
		"lat":       centerLat,
		"lng":       centerLng,
		"radius_km": 5,
	})
	reqPost, _ := http.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewReader(postBody))
	reqPost.Header.Set("Authorization", "Bearer "+token)
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	router.ServeHTTP(wPost, reqPost)

	require.Equal(t, http.StatusOK, wPost.Code)
	var respPost map[string]any
	require.NoError(t, json.Unmarshal(wPost.Body.Bytes(), &respPost))
	postCompanies := respPost["companies"].([]any)
	assert.Len(t, postCompanies, 3)
}

func TestCompanyTechnicalJobs(t *testing.T) {
	router, mockStore, authMgr := setupTestRouter()
	token, _ := authMgr.GenerateToken(uuid.New())
	ctx := t.Context()

	cID := uuid.New()
	require.NoError(t, mockStore.CreateCompany(ctx, &model.Company{ID: cID, Name: "Delta Tech"}))

	now := time.Now().Truncate(time.Millisecond)
	fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
	threeDaysAgo := now.Add(-3 * 24 * time.Hour)
	twentyFiveDaysAgo := now.Add(-25 * 24 * time.Hour)

	// Job 1: Posted recently (5 days ago, trusted)
	j1ID := uuid.New()
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		ID:                      j1ID,
		CompanyID:               cID,
		Source:                  "gh-delta-1",
		SourceFamily:            "job_ats",
		Title:                   "Senior Platform Engineer",
		NormalizedTitle:         "senior platform engineer",
		ContentHash:             "hash-d1",
		WorkArrangement:         model.WorkArrangementInOffice,
		PublicationState:        model.PublicationStatePostedRecently,
		PostedAt:                &fiveDaysAgo,
		PostedAtConfidence:      1.0,
		FirstSeenAt:             fiveDaysAgo,
		LastSeenAt:              now,
		IsActive:                true,
		TechnicalClassification: "infrastructure",
		RuleVersion:             "v1",
	}))

	// Job 2: Observed recently (untrusted date / no posted_at, last seen 3 days ago)
	j2ID := uuid.New()
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		ID:                      j2ID,
		CompanyID:               cID,
		Source:                  "careers-delta-2",
		SourceFamily:            "official_site",
		Title:                   "Security Analyst",
		NormalizedTitle:         "security analyst",
		ContentHash:             "hash-d2",
		WorkArrangement:         model.WorkArrangementRemote,
		PublicationState:        model.PublicationStateObservedRecently,
		PostedAtConfidence:      0.0,
		FirstSeenAt:             threeDaysAgo,
		LastSeenAt:              threeDaysAgo,
		IsActive:                true,
		TechnicalClassification: "security",
		RuleVersion:             "v1",
	}))

	// Job 3: Stale (posted 25 days ago)
	j3ID := uuid.New()
	require.NoError(t, mockStore.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
		ID:                      j3ID,
		CompanyID:               cID,
		Source:                  "lever-delta-3",
		SourceFamily:            "job_ats",
		Title:                   "QA Lead",
		NormalizedTitle:         "qa lead",
		ContentHash:             "hash-d3",
		WorkArrangement:         model.WorkArrangementHybrid,
		PublicationState:        model.PublicationStateStale,
		PostedAt:                &twentyFiveDaysAgo,
		PostedAtConfidence:      1.0,
		FirstSeenAt:             twentyFiveDaysAgo,
		LastSeenAt:              twentyFiveDaysAgo,
		IsActive:                true,
		TechnicalClassification: "qa",
		RuleVersion:             "v1",
	}))

	// 1. Unauthenticated request -> 401
	reqUnauth, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/companies/%s/technical-jobs", cID), nil)
	wUnauth := httptest.NewRecorder()
	router.ServeHTTP(wUnauth, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, wUnauth.Code)

	// 2. Non-existent company -> 404
	missingID := uuid.New()
	reqMissing, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/companies/%s/technical-jobs", missingID), nil)
	reqMissing.Header.Set("Authorization", "Bearer "+token)
	wMissing := httptest.NewRecorder()
	router.ServeHTTP(wMissing, reqMissing)
	assert.Equal(t, http.StatusNotFound, wMissing.Code)

	// 3. Invalid UUID -> 400
	reqInvalid, _ := http.NewRequest(http.MethodGet, "/api/v1/companies/invalid-uuid/technical-jobs", nil)
	reqInvalid.Header.Set("Authorization", "Bearer "+token)
	wInvalid := httptest.NewRecorder()
	router.ServeHTTP(wInvalid, reqInvalid)
	assert.Equal(t, http.StatusBadRequest, wInvalid.Code)

	// 4. Default query (?days=14 implicit or explicit)
	reqDefault, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/companies/%s/technical-jobs?days=14", cID), nil)
	reqDefault.Header.Set("Authorization", "Bearer "+token)
	wDefault := httptest.NewRecorder()
	router.ServeHTTP(wDefault, reqDefault)
	require.Equal(t, http.StatusOK, wDefault.Code)

	var respDefault map[string]any
	require.NoError(t, json.Unmarshal(wDefault.Body.Bytes(), &respDefault))
	jobs := respDefault["jobs"].([]any)
	require.Len(t, jobs, 2, "Default query should return 2 recent jobs (j1 posted_recently, j2 observed_recently), omitting stale j3")

	// Verify publication_state is preserved so UI distinguishes posted_recently from observed_recently
	foundPostedRecently := false
	foundObservedRecently := false
	for _, item := range jobs {
		jMap := item.(map[string]any)
		state := jMap["publication_state"].(string)
		if state == "posted_recently" {
			foundPostedRecently = true
			assert.Equal(t, "Senior Platform Engineer", jMap["title"])
		} else if state == "observed_recently" {
			foundObservedRecently = true
			assert.Equal(t, "Security Analyst", jMap["title"])
		}
	}
	assert.True(t, foundPostedRecently, "Should include job marked posted_recently")
	assert.True(t, foundObservedRecently, "Should include job marked observed_recently")

	// 5. Query with days=30 includes stale job 3 as well
	reqAll, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/companies/%s/technical-jobs?days=30", cID), nil)
	reqAll.Header.Set("Authorization", "Bearer "+token)
	wAll := httptest.NewRecorder()
	router.ServeHTTP(wAll, reqAll)
	require.Equal(t, http.StatusOK, wAll.Code)

	var respAll map[string]any
	require.NoError(t, json.Unmarshal(wAll.Body.Bytes(), &respAll))
	jobs30 := respAll["jobs"].([]any)
	assert.Len(t, jobs30, 3, "days=30 query should include all 3 jobs including stale")
}
