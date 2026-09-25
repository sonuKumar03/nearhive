package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/sonukumar/nearhive/internal/store"
)

func NewRouter(s store.Store, authMgr *auth.Manager, orchestrator *scraper.Orchestrator, jwtSecret string) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	authHandler := &AuthHandler{store: s, authMgr: authMgr}
	searchHandler := &SearchHandler{store: s, history: s}
	companyHandler := &CompanyHandler{store: s}
	jobHandler := NewJobHandler(s, orchestrator, nil)

	// API Root Info
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{
			"service": "NearHive API",
			"status":  "running",
			"version": "v1",
		})
	})

	// Public health routes
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Head("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if pg, ok := s.(*store.PostgresStore); ok && pg != nil {
			if err := pg.DB().PingContext(r.Context()); err != nil {
				JSONError(w, http.StatusServiceUnavailable, "database unavailable", "SERVICE_UNAVAILABLE", nil)
				return
			}
		}
		JSON(w, http.StatusOK, map[string]string{"ready": "true", "database": "connected"})
	})
	r.Head("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if pg, ok := s.(*store.PostgresStore); ok && pg != nil {
			if err := pg.DB().PingContext(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	// Public Auth endpoints
	r.Post("/api/v1/auth/register", authHandler.Register)
	r.Post("/api/v1/auth/login", authHandler.Login)

	// Protected routes (strictly requires JWT)
	r.Group(func(protected chi.Router) {
		protected.Use(AuthMiddleware(authMgr))

		protected.Get("/api/v1/search", searchHandler.Search)
		protected.Get("/api/v1/search/clusters", searchHandler.SearchClusters)
		protected.Get("/api/v1/companies/{id}", companyHandler.GetCompany)
		protected.Get("/api/v1/companies/{id}/sightings", companyHandler.GetSightings)
		protected.Get("/api/v1/jobs", jobHandler.ListJobs)
		protected.Post("/api/v1/jobs/trigger", jobHandler.TriggerJob)
		protected.Get("/api/v1/jobs/{id}", jobHandler.GetJob)
		protected.Post("/api/v1/jobs/{id}/cancel", jobHandler.CancelJob)
	})

	return r
}
