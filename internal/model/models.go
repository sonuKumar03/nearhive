package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type JSONMap map[string]any

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(src any) error {
	if src == nil {
		*j = make(map[string]any)
		return nil
	}
	switch s := src.(type) {
	case []byte:
		return json.Unmarshal(s, j)
	case string:
		return json.Unmarshal([]byte(s), j)
	default:
		return errors.New("cannot scan type into JSONMap")
	}
}

type User struct {
	ID        uuid.UUID `db:"id" json:"id"`
	Email     string    `db:"email" json:"email"`
	Password  string    `db:"password" json:"-"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Company struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Name           string    `db:"name" json:"name"`
	NormalizedName string    `db:"normalized_name" json:"normalized_name"`
	Domain         *string   `db:"domain" json:"domain,omitempty"`
	Industry       *string   `db:"industry" json:"industry,omitempty"`
	EmployeeCount  *string   `db:"employee_count" json:"employee_count,omitempty"`
	Description    *string   `db:"description" json:"description,omitempty"`
	Verified       bool      `db:"verified" json:"verified"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

type Location struct {
	ID         uuid.UUID `db:"id" json:"id"`
	CompanyID  uuid.UUID `db:"company_id" json:"company_id"`
	Label      *string   `db:"label" json:"label,omitempty"`
	Address    string    `db:"address" json:"address"`
	City       *string   `db:"city" json:"city,omitempty"`
	State      *string   `db:"state" json:"state,omitempty"`
	Country    string    `db:"country" json:"country"`
	Pincode    *string   `db:"pincode" json:"pincode,omitempty"`
	Lat        float64   `db:"lat" json:"lat"`
	Lng        float64   `db:"lng" json:"lng"`
	Confidence float64   `db:"confidence" json:"confidence"`
	Verified   bool      `db:"verified" json:"verified"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

type Sighting struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	Source      string     `db:"source" json:"source"`
	SourceURL   *string    `db:"source_url" json:"source_url,omitempty"`
	CompanyName string     `db:"company_name" json:"company_name"`
	RawAddress  string     `db:"raw_address" json:"raw_address"`
	Lat         float64    `db:"lat" json:"lat"`
	Lng         float64    `db:"lng" json:"lng"`
	Metadata    JSONMap    `db:"metadata" json:"metadata"`
	CompanyID   *uuid.UUID `db:"company_id" json:"company_id,omitempty"`
	LocationID  *uuid.UUID `db:"location_id" json:"location_id,omitempty"`
	ScrapedAt   time.Time  `db:"scraped_at" json:"scraped_at"`
}

type ScrapeJob struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	Source     string     `db:"source" json:"source"`
	Status     string     `db:"status" json:"status"`
	Region     *string    `db:"region" json:"region,omitempty"`
	Sightings  int        `db:"sightings" json:"sightings"`
	Error      *string    `db:"error" json:"error,omitempty"`
	StartedAt  *time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
}

type SearchHistory struct {
	ID          uuid.UUID `db:"id" json:"id"`
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	QueryLat    float64   `db:"query_lat" json:"query_lat"`
	QueryLng    float64   `db:"query_lng" json:"query_lng"`
	RadiusKM    float64   `db:"radius_km" json:"radius_km"`
	ResultCount int       `db:"result_count" json:"result_count"`
	SearchedAt  time.Time `db:"searched_at" json:"searched_at"`
}

type CompanySearchResult struct {
	CompanyID      uuid.UUID `db:"company_id" json:"id"`
	Name           string    `db:"name" json:"name"`
	Domain         *string   `db:"domain" json:"domain,omitempty"`
	Industry       *string   `db:"industry" json:"industry,omitempty"`
	EmployeeCount  *string   `db:"employee_count" json:"employee_count,omitempty"`
	LocationID     uuid.UUID `db:"location_id" json:"location_id"`
	Label          *string   `db:"label" json:"label,omitempty"`
	Address        string    `db:"address" json:"address"`
	City           *string   `db:"city" json:"city,omitempty"`
	Lat            float64   `db:"lat" json:"lat"`
	Lng            float64   `db:"lng" json:"lng"`
	Confidence     float64   `db:"confidence" json:"confidence"`
	DistanceMeters float64   `db:"distance_m" json:"distance_meters"`
	Verified       bool      `db:"verified" json:"verified"`
}
