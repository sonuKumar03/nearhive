package store

import (
	"context"
	"math"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

type MockStore struct {
	mu            sync.RWMutex
	Users         map[uuid.UUID]*model.User
	UsersByEmail  map[string]*model.User
	Companies     map[uuid.UUID]*model.Company
	Locations     map[uuid.UUID]*model.Location
	Sightings     map[uuid.UUID]*model.Sighting
	Jobs          map[uuid.UUID]*model.ScrapeJob
	SearchHistory []model.SearchHistory
}

func NewMockStore() *MockStore {
	return &MockStore{
		Users:        make(map[uuid.UUID]*model.User),
		UsersByEmail: make(map[string]*model.User),
		Companies:    make(map[uuid.UUID]*model.Company),
		Locations:    make(map[uuid.UUID]*model.Location),
		Sightings:    make(map[uuid.UUID]*model.Sighting),
		Jobs:         make(map[uuid.UUID]*model.ScrapeJob),
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
			results = append(results, model.CompanySearchResult{
				CompanyID:      c.ID,
				Name:           c.Name,
				Domain:         c.Domain,
				Industry:       c.Industry,
				EmployeeCount:  c.EmployeeCount,
				LocationID:     l.ID,
				Label:          l.Label,
				Address:        l.Address,
				City:           l.City,
				Lat:            l.Lat,
				Lng:            l.Lng,
				Confidence:     l.Confidence,
				DistanceMeters: dist,
				Verified:       l.Verified,
			})
		}
	}
	return results, nil
}

func (m *MockStore) CountSearch(ctx context.Context, lat, lng, radiusMeters float64, opts SearchOpts) (int, error) {
	res, err := m.Search(ctx, lat, lng, radiusMeters, opts)
	return len(res), err
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
	return j, nil
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
