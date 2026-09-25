package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port              string
	Environment       string
	DatabaseURL       string
	JWTSecret         string
	NominatimURL      string
	GoogleGeoAPIKey   string
	GooglePlacesKey   string
	MaxScraperWorkers int
	ScrapeSchedule    string
	ScrapeRegions     []string
	RateLimitAuth     int
	RateLimitSearch   int
}

func Load() (*Config, error) {
	loadDotEnv()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if !strings.Contains(dbURL, "sslmode=") {
		sslMode := getEnv("DB_SSL_MODE", "disable")
		if strings.Contains(dbURL, "?") {
			dbURL += "&sslmode=" + sslMode
		} else {
			dbURL += "?sslmode=" + sslMode
		}
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		Environment:       getEnv("ENVIRONMENT", "development"),
		DatabaseURL:       dbURL,
		JWTSecret:         jwtSecret,
		NominatimURL:      getEnv("NOMINATIM_URL", "https://nominatim.openstreetmap.org"),
		GoogleGeoAPIKey:   os.Getenv("GOOGLE_GEO_API_KEY"),
		GooglePlacesKey:   os.Getenv("GOOGLE_PLACES_KEY"),
		MaxScraperWorkers: getEnvInt("MAX_SCRAPER_WORKERS", 5),
		ScrapeSchedule:    getEnv("SCRAPE_SCHEDULE", "0 3 * * *"),
		ScrapeRegions:     splitAndTrim(getEnv("SCRAPE_REGIONS", "Bangalore,Pune,Hyderabad")),
		RateLimitAuth:     getEnvInt("RATE_LIMIT_AUTH", 10),
		RateLimitSearch:   getEnvInt("RATE_LIMIT_SEARCH", 60),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
	}
}
