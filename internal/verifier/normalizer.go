package verifier

import (
	"regexp"
	"strings"
)

var (
	nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9\s]`)
	multipleSpacesRegex  = regexp.MustCompile(`\s+`)
	suffixes             = []string{
		" limited", " ltd", " pvt", " private",
		" inc", " incorporated", " corp", " corporation",
		" llp", " llc", " technologies", " tech",
		" solutions", " software", " systems",
		" india", " labs", " group",
	}
)

func Normalize(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))

	// First replace non-alphanumeric to simplify suffix matching
	n = nonAlphanumericRegex.ReplaceAllString(n, " ")
	n = multipleSpacesRegex.ReplaceAllString(n, " ")
	n = strings.TrimSpace(n)

	// Remove common corporate suffixes repeatedly until clean
	cleaned := true
	for cleaned {
		cleaned = false
		for _, s := range suffixes {
			cleanSuffix := strings.TrimSpace(s)
			if strings.HasSuffix(n, " "+cleanSuffix) {
				n = strings.TrimSpace(strings.TrimSuffix(n, " "+cleanSuffix))
				cleaned = true
			} else if n == cleanSuffix {
				n = ""
				cleaned = true
			}
		}
	}

	n = multipleSpacesRegex.ReplaceAllString(n, " ")
	return strings.TrimSpace(n)
}
