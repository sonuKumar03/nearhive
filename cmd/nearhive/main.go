package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sonukumar/nearhive/internal/api"
	"github.com/sonukumar/nearhive/internal/auth"
	"github.com/sonukumar/nearhive/internal/config"
	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/sonukumar/nearhive/internal/scheduler"
	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/sonukumar/nearhive/internal/scraper/sources"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/sonukumar/nearhive/internal/verifier"
	"github.com/sonukumar/nearhive/migrations"
)

var rootCmd = &cobra.Command{
	Use:   "nearhive",
	Short: "NearHive 🐝 — Scalable tech company locator and scraper",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(scrapeCmd())
	rootCmd.AddCommand(userCmd())
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the NearHive REST API and background worker",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("failed to load config: %v", err)
			}

			dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
			if err != nil {
				log.Fatalf("failed to connect to database: %v", err)
			}
			defer dbStore.Close()

			// Auto-run database migrations on startup
			if err := migrations.Run(context.Background(), dbStore.DB()); err != nil {
				log.Fatalf("failed to run database migrations: %v", err)
			}

			// Seed initial tech hub data if empty
			_ = store.SeedInitialData(context.Background(), dbStore)

			// Geocoders
			nom := geocoder.NewNominatim(cfg.NominatimURL, nil)
			var googleGeo *geocoder.Google
			if cfg.GoogleGeoAPIKey != "" {
				googleGeo = geocoder.NewGoogle("", cfg.GoogleGeoAPIKey, nil)
			}
			geo := geocoder.NewFallbackGeocoder(nom, googleGeo)

			// Verifier
			verifEngine := verifier.NewEngine(dbStore, geo)

			// Scrapers & Orchestrator
			orchestrator := scraper.NewOrchestrator(dbStore, geo, verifEngine, cfg.MaxScraperWorkers)
			orchestrator.Register(sources.NewOSMScraper())
			orchestrator.Register(sources.NewJustDialScraper())
			if cfg.GooglePlacesKey != "" {
				orchestrator.Register(sources.NewGooglePlacesScraper(cfg.GooglePlacesKey))
			}
			if parks, err := sources.LoadTechParksFromFile("config/techparks.yaml"); err == nil {
				orchestrator.Register(sources.NewTechParkScraper(parks))
			}

			// Auth Manager
			authMgr := auth.NewManager(cfg.JWTSecret, 24*time.Hour)

			// Background Scheduler (daily crawl)
			sched := scheduler.NewScheduler(24*time.Hour, func(ctx context.Context) error {
				for _, r := range cfg.ScrapeRegions {
					_, _ = orchestrator.ScrapeRegion(ctx, scraper.ScrapeRequest{Region: r, RadiusKM: 20})
				}
				return nil
			})
			schedCtx, schedCancel := context.WithCancel(context.Background())
			defer schedCancel()
			sched.Start(schedCtx)

			// HTTP Server
			router := api.NewRouter(dbStore, authMgr, orchestrator, cfg.JWTSecret)
			srv := &http.Server{
				Addr:         ":" + cfg.Port,
				Handler:      router,
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 15 * time.Second,
			}

			go func() {
				log.Printf("🐝 NearHive API server listening on :%s", cfg.Port)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("listen failed: %v", err)
				}
			}()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
			<-stop

			log.Println("Shutting down NearHive...")
			sched.Stop()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		},
	}
}

func scrapeCmd() *cobra.Command {
	var region string
	var radius float64

	cmd := &cobra.Command{
		Use:   "scrape",
		Short: "Run a one-off scrape for a specific region",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("config error: %v", err)
			}
			dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
			if err != nil {
				log.Fatalf("database error: %v", err)
			}
			defer dbStore.Close()

			nom := geocoder.NewNominatim(cfg.NominatimURL, nil)
			verifEngine := verifier.NewEngine(dbStore, nom)
			orchestrator := scraper.NewOrchestrator(dbStore, nom, verifEngine, cfg.MaxScraperWorkers)
			orchestrator.Register(sources.NewOSMScraper())
			orchestrator.Register(sources.NewJustDialScraper())

			log.Printf("Starting scrape for region: %s (radius: %.1f km)...", region, radius)
			sightings, err := orchestrator.ScrapeRegion(context.Background(), scraper.ScrapeRequest{
				Region:   region,
				RadiusKM: radius,
			})
			if err != nil {
				log.Fatalf("scrape failed: %v", err)
			}
			log.Printf("Scrape complete! Captured %d sightings.", len(sightings))
		},
	}
	cmd.Flags().StringVarP(&region, "region", "r", "Bangalore", "Region/City name")
	cmd.Flags().Float64VarP(&radius, "radius", "d", 15.0, "Radius in kilometers")
	return cmd
}

func userCmd() *cobra.Command {
	userRoot := &cobra.Command{Use: "user", Short: "User management commands"}
	var email, password string

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user account",
		Run: func(cmd *cobra.Command, args []string) {
			if email == "" || password == "" {
				log.Fatal("both --email and --password are required")
			}
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("config error: %v", err)
			}
			dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
			if err != nil {
				log.Fatalf("database error: %v", err)
			}
			defer dbStore.Close()

			hashed, err := auth.HashPassword(password)
			if err != nil {
				log.Fatalf("hash error: %v", err)
			}

			user := &model.User{
				ID:       uuid.New(),
				Email:    email,
				Password: hashed,
			}
			if err := dbStore.CreateUser(context.Background(), user); err != nil {
				log.Fatalf("create user failed: %v", err)
			}
			fmt.Printf("User created successfully: %s (ID: %s)\n", user.Email, user.ID)
		},
	}
	createCmd.Flags().StringVarP(&email, "email", "e", "", "User email")
	createCmd.Flags().StringVarP(&password, "password", "p", "", "User password")
	userRoot.AddCommand(createCmd)
	return userRoot
}
