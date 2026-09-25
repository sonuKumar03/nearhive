package store

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
)

// SeedInitialData populates verified tech companies in Bangalore/Hyderabad if none exist.
func SeedInitialData(ctx context.Context, s Store) error {
	pg, ok := s.(*PostgresStore)
	if !ok {
		return nil
	}
	var count int
	if err := pg.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM companies"); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	log.Println("Seeding initial verified tech companies for spatial exploration...")

	type seedItem struct {
		name      string
		norm      string
		domain    string
		industry  string
		emp       string
		addr      string
		city      string
		lat       float64
		lng       float64
		conf      float64
		verified  bool
	}

	seeds := []seedItem{
		// Bangalore Tech Corridors
		{name: "Google India", norm: "google", domain: "google.com", industry: "Cloud & AI", emp: "10,000+", addr: "Bagmane World Technology Center, Mahadevapura, Bengaluru, Karnataka 560048", city: "Bangalore", lat: 12.9934, lng: 77.6890, conf: 0.98, verified: true},
		{name: "Microsoft India R&D", norm: "microsoft", domain: "microsoft.com", industry: "Enterprise Software", emp: "10,000+", addr: "Prestige Ferns Galaxy, Bellandur, Bengaluru, Karnataka 560103", city: "Bangalore", lat: 12.9279, lng: 77.6771, conf: 0.97, verified: true},
		{name: "Amazon Development Centre", norm: "amazon", domain: "amazon.com", industry: "E-Commerce & Cloud", emp: "10,000+", addr: "Brigade Gateway, Malleshwaram, Bengaluru, Karnataka 560055", city: "Bangalore", lat: 13.0118, lng: 77.5552, conf: 0.98, verified: true},
		{name: "Flipkart Internet Pvt Ltd", norm: "flipkart", domain: "flipkart.com", industry: "E-Commerce", emp: "10,000+", addr: "Embassy TechVillage, Outer Ring Road, Devarabisanahalli, Bengaluru 560103", city: "Bangalore", lat: 12.9260, lng: 77.6923, conf: 0.96, verified: true},
		{name: "Swiggy (Bundl Technologies)", norm: "swiggy", domain: "swiggy.com", industry: "FoodTech & Logistics", emp: "5,000+", addr: "IBC Knowledge Park, Bannerghatta Main Rd, Bengaluru 560029", city: "Bangalore", lat: 12.9298, lng: 77.5997, conf: 0.95, verified: true},
		{name: "Razorpay Software", norm: "razorpay", domain: "razorpay.com", industry: "Fintech", emp: "2,500+", addr: "SJR Cyber, 22 Laskar Hosur Road, Adugodi, Bengaluru 560030", city: "Bangalore", lat: 12.9385, lng: 77.6111, conf: 0.96, verified: true},
		{name: "Zerodha Broking", norm: "zerodha", domain: "zerodha.com", industry: "Fintech & Trading", emp: "1,500+", addr: "153/154 4th Cross, 4th Phase, Dollars Colony, JP Nagar, Bengaluru 560078", city: "Bangalore", lat: 12.9081, lng: 77.5958, conf: 0.95, verified: true},
		{name: "Infosys Limited", norm: "infosys", domain: "infosys.com", industry: "IT Services", emp: "100,000+", addr: "Electronics City, Hosur Road, Bengaluru 560100", city: "Bangalore", lat: 12.8452, lng: 77.6602, conf: 0.99, verified: true},
		{name: "Wipro Technologies", norm: "wipro", domain: "wipro.com", industry: "IT Consulting", emp: "100,000+", addr: "Doddakannelli, Sarjapur Road, Bengaluru 560035", city: "Bangalore", lat: 12.9099, lng: 77.6872, conf: 0.99, verified: true},
		{name: "PhonePe Private Limited", norm: "phonepe", domain: "phonepe.com", industry: "Digital Payments", emp: "5,000+", addr: "Prestige Tech Park, Marathahalli-Sarjapur Outer Ring Rd, Bengaluru 560103", city: "Bangalore", lat: 12.9352, lng: 77.6946, conf: 0.96, verified: true},
		// Hyderabad Tech Corridors
		{name: "Microsoft Hyderabad Campus", norm: "microsoft", domain: "microsoft.com", industry: "Enterprise Software", emp: "10,000+", addr: "Gachibowli, Hyderabad, Telangana 500032", city: "Hyderabad", lat: 17.4435, lng: 78.3489, conf: 0.98, verified: true},
		{name: "Google Hyderabad", norm: "google", domain: "google.com", industry: "Cloud & Search", emp: "10,000+", addr: "Kondapur, HITEC City, Hyderabad, Telangana 500084", city: "Hyderabad", lat: 17.4589, lng: 78.3728, conf: 0.98, verified: true},
		{name: "Amazon Hyderabad Campus", norm: "amazon", domain: "amazon.com", industry: "Tech & Logistics", emp: "15,000+", addr: "Financial District, Nanakramguda, Hyderabad 500032", city: "Hyderabad", lat: 17.4156, lng: 78.3427, conf: 0.99, verified: true},
		// Pune Tech Corridors
		{name: "Tata Consultancy Services (TCS)", norm: "tcs", domain: "tcs.com", industry: "IT Services", emp: "50,000+", addr: "Sahyadri Park, Rajiv Gandhi Infotech Park, Hinjawadi, Pune 411057", city: "Pune", lat: 18.5913, lng: 73.7389, conf: 0.99, verified: true},
		{name: "Cognizant Technology Solutions", norm: "cognizant", domain: "cognizant.com", industry: "IT Consulting", emp: "20,000+", addr: "Quadron Business Park, Hinjawadi Phase 2, Pune 411057", city: "Pune", lat: 18.5985, lng: 73.7196, conf: 0.97, verified: true},
	}

	for _, item := range seeds {
		cID := uuid.New()
		comp := &model.Company{
			ID:             cID,
			Name:           item.name,
			NormalizedName: item.norm,
			Domain:         &item.domain,
			Industry:       &item.industry,
			EmployeeCount:  &item.emp,
			Verified:       item.verified,
		}
		if err := s.CreateCompany(ctx, comp); err != nil {
			log.Printf("seed company err: %v", err)
			continue
		}

		loc := &model.Location{
			ID:         uuid.New(),
			CompanyID:  cID,
			Label:      &item.name,
			Address:    item.addr,
			City:       &item.city,
			Lat:        item.lat,
			Lng:        item.lng,
			Confidence: item.conf,
			Verified:   item.verified,
		}
		if err := s.CreateLocation(ctx, loc); err != nil {
			log.Printf("seed location err: %v", err)
			continue
		}
	}

	log.Printf("Successfully seeded %d initial verified tech companies", len(seeds))
	return nil
}
