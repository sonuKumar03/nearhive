package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sonukumar/nearhive/internal/config"
	"github.com/sonukumar/nearhive/internal/crawler"
	"github.com/sonukumar/nearhive/internal/geocoder"
	"github.com/sonukumar/nearhive/internal/queue"
	"github.com/sonukumar/nearhive/internal/scraper"
	"github.com/sonukumar/nearhive/internal/scraper/sources"
	"github.com/sonukumar/nearhive/internal/store"
	"github.com/sonukumar/nearhive/internal/verifier"
	"github.com/sonukumar/nearhive/migrations"
)

func main() {
	log.Println("🕷️ Starting NearHive Standalone Crawler Service...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	dbStore, err := store.NewPostgresStore(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbStore.Close()

	// Run migrations to ensure latest schema
	if err := migrations.Run(context.Background(), dbStore.DB()); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

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
	orchestrator.Register(sources.NewWikidataScraper())
	orchestrator.Register(sources.NewJustDialScraper())
	if cfg.GooglePlacesKey != "" {
		orchestrator.Register(sources.NewGooglePlacesScraper(cfg.GooglePlacesKey))
	}
	if parks, err := sources.LoadTechParksFromFile("config/techparks.yaml"); err == nil {
		orchestrator.Register(sources.NewTechParkScraper(parks))
	}

	// Postgres Queue
	q := queue.NewPostgresQueue(dbStore.SqlxDB(), cfg.DatabaseURL)

	// Worker Daemon
	worker := crawler.NewWorkerDaemon(q, orchestrator, crawler.WorkerConfig{
		PollInterval: 2 * time.Second,
		JobTimeout:   15 * time.Minute,
		Concurrency:  cfg.CrawlerConcurrency,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)

	log.Println("✅ Crawler service is listening for jobs on PostgreSQL queue...")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("🛑 Shutting down crawler daemon...")
	cancel()
	worker.Stop()
	log.Println("👋 Crawler service shutdown complete")
}
