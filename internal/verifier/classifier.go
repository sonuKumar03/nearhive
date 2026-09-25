package verifier

import (
	"strings"
	"unicode"
)

type EntityVerdict struct {
	IsTechCompany bool
	Confidence    float64
	Reason        string
}

type Classifier struct {
	negativeKeywords []string
	positiveKeywords []string
}

func NewClassifier() *Classifier {
	return &Classifier{
		negativeKeywords: []string{
			// Education & Coaching
			"school", "vidya", "vidyalaya", "college", "academy", "coaching",
			"institute", "tuition", "prometric", "classes", "tutorial",
			"polytechnic", "university", "kindergarten", "nursery", "montessori",
			"shiksha", "gurukul", "vidyapeeth",
			// Healthcare & Medical
			"hospital", "clinic", "dental", "pharmacy", "diagnostic", "pathology",
			"opticals", "medicos", "chemist", "nursing", "ayurveda", "homeo",
			// Hospitality & Food
			"hostel", "paying guest", "pg for", "hotel", "restaurant", "cafe",
			"baker", "bakery", "sweets", "dhaba", "caterers", "bar &",
			// Retail & Personal Services
			"salon", "spa", "parlour", "gym", "fitness", "boutique", "jewel",
			"jewellers", "jewellery", "textile", "tailor", "supermarket",
			"grocer", "grocery", "stationery", "laundry", "dry cleaner",
			"realtor", "real estate", "packers and movers", "packers & movers",
		},
		positiveKeywords: []string{
			"software", "technology", "technologies", "tech", "labs",
			"laboratory", "systems", "solutions", "infotech", "digital",
			"networks", "data", "cloud", "analytics", "computing",
			"cyber", "ai", "robotics", "interactive", "studios",
			"consulting", "telecom", "telecommunication", "informatics",
		},
	}
}

func (c *Classifier) Classify(name string, metadata map[string]any) EntityVerdict {
	normalized := strings.ToLower(strings.TrimSpace(name))
	tokens := extractWords(normalized)

	// 1. Check for negative keywords/phrases
	for _, neg := range c.negativeKeywords {
		if strings.Contains(neg, " ") {
			if strings.Contains(normalized, neg) {
				return EntityVerdict{
					IsTechCompany: false,
					Confidence:    0.0,
					Reason:        "matches negative phrase: " + neg,
				}
			}
		} else {
			for _, tok := range tokens {
				if tok == neg {
					return EntityVerdict{
						IsTechCompany: false,
						Confidence:    0.0,
						Reason:        "matches negative keyword: " + neg,
					}
				}
			}
		}
	}

	// 2. Check metadata indicators
	if metadata != nil {
		if amenity, ok := metadata["amenity"].(string); ok {
			amenity = strings.ToLower(amenity)
			if amenity != "" && amenity != "coworking_space" {
				return EntityVerdict{
					IsTechCompany: false,
					Confidence:    0.0,
					Reason:        "non-tech amenity tag: " + amenity,
				}
			}
		}
	}

	// 3. Check for positive tech keywords in name
	var matchedPositive []string
	for _, pos := range c.positiveKeywords {
		for _, tok := range tokens {
			if tok == pos {
				matchedPositive = append(matchedPositive, pos)
				break
			}
		}
	}

	if len(matchedPositive) > 0 {
		return EntityVerdict{
			IsTechCompany: true,
			Confidence:    0.85,
			Reason:        "contains tech keywords: " + strings.Join(matchedPositive, ", "),
		}
	}

	// 4. Check if metadata indicates tech office or verified domain
	if metadata != nil {
		if off, ok := metadata["office_type"].(string); ok && (off == "it" || off == "company" || off == "telecommunication" || off == "research") {
			return EntityVerdict{
				IsTechCompany: true,
				Confidence:    0.65,
				Reason:        "tagged with tech office type: " + off,
			}
		}
		if site, ok := metadata["website"].(string); ok && site != "" {
			return EntityVerdict{
				IsTechCompany: true,
				Confidence:    0.60,
				Reason:        "has corporate website: " + site,
			}
		}
	}

	// Default fallback: accept with moderate baseline if no negative signal
	return EntityVerdict{
		IsTechCompany: true,
		Confidence:    0.50,
		Reason:        "no negative signals detected",
	}
}

func extractWords(s string) []string {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	return strings.FieldsFunc(s, f)
}
