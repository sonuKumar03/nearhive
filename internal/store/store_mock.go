package store

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

type MockStore struct {
	mu                  sync.RWMutex
	Users               map[uuid.UUID]*model.User
	UsersByEmail        map[string]*model.User
	Companies           map[uuid.UUID]*model.Company
	Locations           map[uuid.UUID]*model.Location
	Sightings           map[uuid.UUID]*model.Sighting
	Jobs                map[uuid.UUID]*model.ScrapeJob
	Tasks               map[uuid.UUID]*model.ScrapeTask
	SearchHistory       []model.SearchHistory
	DiscoveryJobs       map[uuid.UUID]*model.DiscoveryJob
	DiscoverySourceRuns map[uuid.UUID][]*model.DiscoverySourceRun
	TechnicalJobs       map[uuid.UUID]*model.TechnicalJobPosting
}

func NewMockStore() *MockStore {
	return &MockStore{
		Users:               make(map[uuid.UUID]*model.User),
		UsersByEmail:        make(map[string]*model.User),
		Companies:           make(map[uuid.UUID]*model.Company),
		Locations:           make(map[uuid.UUID]*model.Location),
		Sightings:           make(map[uuid.UUID]*model.Sighting),
		Jobs:                make(map[uuid.UUID]*model.ScrapeJob),
		Tasks:               make(map[uuid.UUID]*model.ScrapeTask),
		DiscoveryJobs:       make(map[uuid.UUID]*model.DiscoveryJob),
		DiscoverySourceRuns: make(map[uuid.UUID][]*model.DiscoverySourceRun),
		TechnicalJobs:       make(map[uuid.UUID]*model.TechnicalJobPosting),
	}
}

func (m *MockStore) Close() error { return nil }

func (m *MockStore) CreateUser(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	m.Users[u.ID] = u
	m.UsersByEmail[u.Email] = u
	return nil
}

func (m *MockStore) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.UsersByEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MockStore) GetUserByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.Users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MockStore) CreateCompany(_ context.Context, c *model.Company) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	m.Companies[c.ID] = c
	return nil
}

func (m *MockStore) GetCompanyByID(_ context.Context, id uuid.UUID) (*model.Company, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.Companies[id]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}

func (m *MockStore) FindByDomain(_ context.Context, domain string) (*model.Company, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.Companies {
		if c.Domain != nil && *c.Domain == domain {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStore) FindByNormalizedName(_ context.Context, name string) (*model.Company, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.Companies {
		if c.NormalizedName == name {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStore) FindByFuzzyName(_ context.Context, name string, _ float64) (*model.Company, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.Companies {
		if strings.Contains(c.NormalizedName, name) || strings.Contains(name, c.NormalizedName) {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStore) UpdateCompany(_ context.Context, c *model.Company) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Companies[c.ID] = c
	return nil
}

func (m *MockStore) CreateLocation(_ context.Context, l *model.Location) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	if l.PresenceType == "" {
		l.PresenceType = model.PresenceTypeProbableOffice
	}
	m.Locations[l.ID] = l
	return nil
}

func (m *MockStore) GetLocationsByCompany(_ context.Context, companyID uuid.UUID) ([]model.Location, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.Location
	for _, l := range m.Locations {
		if l.CompanyID == companyID {
			res = append(res, *l)
		}
	}
	return res, nil
}

func (m *MockStore) FindNearbyLocation(_ context.Context, companyID uuid.UUID, lat, lng float64, radiusMeters float64) (*model.Location, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, l := range m.Locations {
		if l.CompanyID == companyID {
			dist := haversineDistance(lat, lng, l.Lat, l.Lng)
			if dist <= radiusMeters {
				return l, nil
			}
		}
	}
	return nil, nil
}

func (m *MockStore) UpdateLocationConfidence(_ context.Context, id uuid.UUID, confidence float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l, ok := m.Locations[id]; ok {
		l.Confidence = confidence
	}
	return nil
}

func (m *MockStore) UpdateLocationCoords(_ context.Context, id uuid.UUID, lat, lng float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l, ok := m.Locations[id]; ok {
		l.Lat = lat
		l.Lng = lng
	}
	return nil
}

func (m *MockStore) Search(_ context.Context, lat, lng, radiusMeters float64, opts SearchOpts) ([]model.CompanySearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []model.CompanySearchResult

	for _, l := range m.Locations {
		c, ok := m.Companies[l.CompanyID]
		if !ok {
			continue
		}
		dist := haversineDistance(lat, lng, l.Lat, l.Lng)
		if dist <= radiusMeters {
			if opts.MinConfidence != nil && l.Confidence < *opts.MinConfidence {
				continue
			}
			pType := l.PresenceType
			if pType == "" {
				pType = model.PresenceTypeProbableOffice
			}
			results = append(results, model.CompanySearchResult{
				CompanyID:               c.ID,
				Name:                    c.Name,
				Domain:                  c.Domain,
				Industry:                c.Industry,
				EmployeeCount:           c.EmployeeCount,
				LocationID:              l.ID,
				Label:                   l.Label,
				Address:                 l.Address,
				City:                    l.City,
				Lat:                     l.Lat,
				Lng:                     l.Lng,
				Confidence:              l.Confidence,
				DistanceMeters:          dist,
				PresenceType:            pType,
				RecentTechnicalJobCount: 0,
				Verified:                l.Verified,
			})
		}
	}
	return results, nil
}

func (m *MockStore) CountSearch(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) (int, error) {
	res, err := m.Search(ctx, lat, lng, radiusMeters, opts)
	return len(res), err
}

func (m *MockStore) ClusterSearch(_ context.Context, lat, lng, radiusMeters float64, k int) ([]model.SpatialCluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if k <= 0 {
		k = 20
	}

	var matched []model.Location
	for _, l := range m.Locations {
		dist := haversineDistance(lat, lng, l.Lat, l.Lng)
		if dist <= radiusMeters {
			matched = append(matched, *l)
		}
	}

	if len(matched) == 0 {
		return []model.SpatialCluster{}, nil
	}

	if k > len(matched) {
		k = len(matched)
	}

	counts := make([]int, k)
	sumLat := make([]float64, k)
	sumLng := make([]float64, k)

	for i, l := range matched {
		cID := i % k
		counts[cID]++
		sumLat[cID] += l.Lat
		sumLng[cID] += l.Lng
	}

	var res []model.SpatialCluster
	for i := 0; i < k; i++ {
		if counts[i] > 0 {
			res = append(res, model.SpatialCluster{
				ClusterID: i,
				Count:     counts[i],
				Lat:       sumLat[i] / float64(counts[i]),
				Lng:       sumLng[i] / float64(counts[i]),
			})
		}
	}
	return res, nil
}

func (m *MockStore) SaveSightings(_ context.Context, source string, sightings []model.Sighting) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range sightings {
		if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		s.Source = source
		sCopy := s
		m.Sightings[s.ID] = &sCopy
	}
	return nil
}

func (m *MockStore) GetSightingsByCompany(_ context.Context, companyID uuid.UUID) ([]model.Sighting, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.Sighting
	for _, s := range m.Sightings {
		if s.CompanyID != nil && *s.CompanyID == companyID {
			res = append(res, *s)
		}
	}
	return res, nil
}

func (m *MockStore) LinkSighting(_ context.Context, sightingID, companyID, locationID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.Sightings[sightingID]; ok {
		s.CompanyID = &companyID
		s.LocationID = &locationID
	}
	return nil
}

func (m *MockStore) CreateJob(_ context.Context, job *model.ScrapeJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	m.Jobs[job.ID] = job
	return nil
}

func (m *MockStore) UpdateJob(_ context.Context, job *model.ScrapeJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Jobs[job.ID] = job
	return nil
}

func (m *MockStore) GetJobByID(_ context.Context, id uuid.UUID) (*model.ScrapeJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.Jobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	jobCopy := *j
	jobCopy.Tasks = []model.ScrapeTask{}
	for _, t := range m.Tasks {
		if t.JobID == id {
			jobCopy.Tasks = append(jobCopy.Tasks, *t)
		}
	}
	return &jobCopy, nil
}

func (m *MockStore) CreateTask(_ context.Context, task *model.ScrapeTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	taskCopy := *task
	m.Tasks[task.ID] = &taskCopy
	return nil
}

func (m *MockStore) UpdateTask(_ context.Context, task *model.ScrapeTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	taskCopy := *task
	m.Tasks[task.ID] = &taskCopy
	return nil
}

func (m *MockStore) GetTasksByJobID(_ context.Context, jobID uuid.UUID) ([]model.ScrapeTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.ScrapeTask
	for _, t := range m.Tasks {
		if t.JobID == jobID {
			res = append(res, *t)
		}
	}
	return res, nil
}

func (m *MockStore) ListJobs(_ context.Context, _, _ int) ([]model.ScrapeJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.ScrapeJob
	for _, j := range m.Jobs {
		res = append(res, *j)
	}
	return res, nil
}

func (m *MockStore) RecordSearch(_ context.Context, h *model.SearchHistory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SearchHistory = append(m.SearchHistory, *h)
	return nil
}

func (m *MockStore) GetHistoryByUser(_ context.Context, userID uuid.UUID, limit int) ([]model.SearchHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []model.SearchHistory
	for _, h := range m.SearchHistory {
		if h.UserID == userID {
			res = append(res, h)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in meters
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// DiscoveryStore implementation

func (m *MockStore) CreateDiscoveryJob(_ context.Context, job *model.DiscoveryJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.Status == "" {
		job.Status = model.DiscoveryStatusPending
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 3
	}
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	jobCopy := *job
	m.DiscoveryJobs[job.ID] = &jobCopy
	return nil
}

func (m *MockStore) GetDiscoveryJob(_ context.Context, id uuid.UUID, userID uuid.UUID) (*model.DiscoveryJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	j, ok := m.DiscoveryJobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	if userID != uuid.Nil && j.UserID != userID {
		return nil, ErrNotFound
	}

	jobCopy := *j
	jobCopy.SourceRuns = []model.DiscoverySourceRun{}
	if runs, found := m.DiscoverySourceRuns[id]; found {
		for _, r := range runs {
			jobCopy.SourceRuns = append(jobCopy.SourceRuns, *r)
		}
	}
	return &jobCopy, nil
}

func (m *MockStore) ListDiscoveryJobs(_ context.Context, userID uuid.UUID, limit, offset int) ([]model.DiscoveryJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var all []model.DiscoveryJob
	for _, j := range m.DiscoveryJobs {
		if userID == uuid.Nil || j.UserID == userID {
			jobCopy := *j
			jobCopy.SourceRuns = []model.DiscoverySourceRun{}
			if runs, found := m.DiscoverySourceRuns[j.ID]; found {
				for _, r := range runs {
					jobCopy.SourceRuns = append(jobCopy.SourceRuns, *r)
				}
			}
			all = append(all, jobCopy)
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	if offset >= len(all) {
		return []model.DiscoveryJob{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (m *MockStore) CancelDiscoveryJob(_ context.Context, id uuid.UUID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.DiscoveryJobs[id]
	if !ok {
		return ErrNotFound
	}
	if userID != uuid.Nil && j.UserID != userID {
		return ErrNotFound
	}

	if j.Status == model.DiscoveryStatusCancelled {
		return nil
	}
	if j.Status != model.DiscoveryStatusPending && j.Status != model.DiscoveryStatusRunning {
		return ErrInvalidJobState
	}

	j.Status = model.DiscoveryStatusCancelled
	j.UpdatedAt = time.Now()
	return nil
}

func (m *MockStore) UpsertDiscoverySourceRun(_ context.Context, run *model.DiscoverySourceRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	now := time.Now()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	run.UpdatedAt = now

	runs := m.DiscoverySourceRuns[run.DiscoveryJobID]
	for i, existing := range runs {
		if existing.Source == run.Source {
			runCopy := *run
			runCopy.ID = existing.ID
			runCopy.CreatedAt = existing.CreatedAt
			runs[i] = &runCopy
			m.DiscoverySourceRuns[run.DiscoveryJobID] = runs
			return nil
		}
	}

	runCopy := *run
	m.DiscoverySourceRuns[run.DiscoveryJobID] = append(m.DiscoverySourceRuns[run.DiscoveryJobID], &runCopy)
	return nil
}

func (m *MockStore) GetDiscoverySourceRuns(_ context.Context, discoveryJobID uuid.UUID) ([]model.DiscoverySourceRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runs, ok := m.DiscoverySourceRuns[discoveryJobID]
	if !ok {
		return []model.DiscoverySourceRun{}, nil
	}

	res := make([]model.DiscoverySourceRun, len(runs))
	for i, r := range runs {
		res[i] = *r
	}
	return res, nil
}

// TechnicalJobStore implementation

func (m *MockStore) GetTechnicalJobsByCompany(_ context.Context, companyID uuid.UUID, since time.Time) ([]model.TechnicalJobPosting, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var res []model.TechnicalJobPosting
	for _, job := range m.TechnicalJobs {
		if job.CompanyID != companyID || !job.IsActive {
			continue
		}
		if !since.IsZero() {
			if job.PostedAt != nil {
				if job.PostedAt.Before(since) {
					continue
				}
			} else {
				if job.LastSeenAt.Before(since) {
					continue
				}
			}
		}
		res = append(res, *job)
	}

	sort.Slice(res, func(i, j int) bool {
		tI := res[i].LastSeenAt
		if res[i].PostedAt != nil {
			tI = *res[i].PostedAt
		}
		tJ := res[j].LastSeenAt
		if res[j].PostedAt != nil {
			tJ = *res[j].PostedAt
		}
		return tI.After(tJ)
	})

	return res, nil
}

func (m *MockStore) UpsertTechnicalJob(_ context.Context, job *model.TechnicalJobPosting) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.NormalizedTitle == "" {
		job.NormalizedTitle = strings.ToLower(strings.TrimSpace(job.Title))
	}
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	if job.FirstSeenAt.IsZero() {
		job.FirstSeenAt = now
	}
	if job.LastSeenAt.IsZero() {
		job.LastSeenAt = now
	}

	// Match existing by (source, source_job_id) if source_job_id is set, or (source, content_hash)
	for _, existing := range m.TechnicalJobs {
		matched := false
		if job.SourceJobID != nil && *job.SourceJobID != "" && existing.SourceJobID != nil && *existing.SourceJobID == *job.SourceJobID && existing.Source == job.Source {
			matched = true
		} else if (job.SourceJobID == nil || *job.SourceJobID == "") && existing.ContentHash == job.ContentHash && existing.Source == job.Source {
			matched = true
		}

		if matched {
			existing.Title = job.Title
			existing.NormalizedTitle = job.NormalizedTitle
			existing.DescriptionExcerpt = job.DescriptionExcerpt
			existing.LocationRaw = job.LocationRaw
			existing.Lat = job.Lat
			existing.Lng = job.Lng
			existing.WorkArrangement = job.WorkArrangement
			existing.PublicationState = job.PublicationState
			if job.PostedAt != nil {
				existing.PostedAt = job.PostedAt
				existing.PostedAtConfidence = job.PostedAtConfidence
			}
			existing.LastSeenAt = job.LastSeenAt
			existing.IsActive = job.IsActive
			existing.TechnicalClassification = job.TechnicalClassification
			existing.RuleVersion = job.RuleVersion
			existing.ClassificationReasons = job.ClassificationReasons
			existing.Metadata = job.Metadata
			existing.UpdatedAt = now
			return nil
		}
	}

	jobCopy := *job
	m.TechnicalJobs[job.ID] = &jobCopy
	return nil
}
