package verifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifier_Classify(t *testing.T) {
	c := NewClassifier()

	tests := []struct {
		name          string
		companyName   string
		metadata      map[string]any
		expectTech    bool
		minConfidence float64
	}{
		// False positives that we want to filter out
		{
			name:        "School",
			companyName: "Brilliant Grammar High School",
			expectTech:  false,
		},
		{
			name:        "Coaching Center",
			companyName: "Race Institute Coaching",
			expectTech:  false,
		},
		{
			name:        "Test Center",
			companyName: "Pearson Prometric Testing Center",
			expectTech:  false,
		},
		{
			name:        "Training Institute",
			companyName: "Naresh IT Training Institute",
			expectTech:  false,
		},
		{
			name:        "Hospital",
			companyName: "Apollo Hospital & Clinic",
			expectTech:  false,
		},
		{
			name:        "Hostel",
			companyName: "Sri Sai Luxury PG Hostel",
			expectTech:  false,
		},
		{
			name:        "Restaurant",
			companyName: "Paradise Biryani & Restaurant",
			expectTech:  false,
		},

		// Legitimate Tech Companies
		{
			name:          "Software Company",
			companyName:   "Persistent Systems",
			expectTech:    true,
			minConfidence: 0.8,
		},
		{
			name:          "Cloud & Technologies",
			companyName:   "Google India Technologies",
			expectTech:    true,
			minConfidence: 0.8,
		},
		{
			name:          "Labs",
			companyName:   "SAP Labs India",
			expectTech:    true,
			minConfidence: 0.8,
		},
		{
			name:          "Solutions",
			companyName:   "Geosys Enterprise Solutions Pvt. Ltd.",
			expectTech:    true,
			minConfidence: 0.8,
		},
		{
			name:          "Neutral corporate name without tech keyword",
			companyName:   "Mu Sigma",
			metadata:      map[string]any{"office_type": "it"},
			expectTech:    true,
			minConfidence: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict := c.Classify(tt.companyName, tt.metadata)
			assert.Equal(t, tt.expectTech, verdict.IsTechCompany, "Entity: %s", tt.companyName)
			if tt.expectTech && tt.minConfidence > 0 {
				assert.GreaterOrEqual(t, verdict.Confidence, tt.minConfidence)
			}
		})
	}
}
