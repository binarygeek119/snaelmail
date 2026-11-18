package render

import (
	"strings"
	"testing"
	"time"

	"snailmail/internal/page"
)

func TestRenderPage(t *testing.T) {
	// Create a valid manifest
	m := page.Manifest{
		Photos: []page.Photo{
			{Ord: 0, Filename: "0.webp", Caption: "City Hall"},
			{Ord: 1, Filename: "1.webp", Caption: "Main Street"},
		},
		About: "Welcome to our charming post office, nestled in the heart of downtown. We've been serving the community for generations, handling letters with care and delivering smiles to doorsteps across the region. Our dedicated staff ensures every piece of mail reaches its destination safely and on time.",
		Facts: []page.Fact{
			{Key: "Founded", Value: "1892"},
			{Key: "Elevation", Value: "250m"},
		},
		Postmaster: page.Postmaster{
			Name: "Jane Smith",
			Role: "Head Postmaster",
			Bio:  "Jane has been with us for 15 years and loves her job.",
		},
		DispatchTimes: []string{"9:00 AM", "2:00 PM"},
		Classes:       []string{"surface", "first"},
		Accent: page.Accent{
			Primary:   "#333333",
			Secondary: "#666666",
			Tertiary:  "#999999",
			Highlight: "#f0f0f0",
			Muted:     "#cccccc",
		},
	}

	relay := RelayInfo{
		Hostname:     "po-springfield-us",
		City:         "Springfield",
		Region:       "Illinois",
		CC:           "US",
		Lat:          39.7817,
		Lon:          -89.6501,
		OpenedAt:     timePtr(time.Date(2020, 3, 15, 0, 0, 0, 0, time.UTC)),
		LettersCount: 12345,
	}

	html, err := RenderPage(m, relay)
	if err != nil {
		t.Fatalf("RenderPage() error = %v", err)
	}

	htmlStr := string(html)

	// Check that key elements are present
	checks := []string{
		"Springfield Post Office",
		"Illinois, US",
		"15 March 2020",
		"12345 letters handled",
		"City Hall",
		"Main Street",
		"Jane Smith",
		"Head Postmaster",
		"9:00 AM",
		"2:00 PM",
		"surface",
		"first",
		"#333333",
		"#666666",
	}

	for _, check := range checks {
		if !strings.Contains(htmlStr, check) {
			t.Errorf("HTML missing expected content: %q", check)
		}
	}
}

func TestRenderFallback(t *testing.T) {
	relay := RelayInfo{
		Hostname:     "po-testville-us",
		City:         "Testville",
		Region:       "Teststate",
		CC:           "US",
		Lat:          40.0,
		Lon:          -90.0,
		OpenedAt:     nil, // Not yet opened
		LettersCount: 0,
	}

	html, err := RenderFallback(relay)
	if err != nil {
		t.Fatalf("RenderFallback() error = %v", err)
	}

	htmlStr := string(html)

	// Check fallback content
	if !strings.Contains(htmlStr, "Testville Post Office") {
		t.Error("Fallback missing city name")
	}
	if !strings.Contains(htmlStr, "Coming soon") {
		t.Error("Fallback should say 'Coming soon' when not opened")
	}
	if !strings.Contains(htmlStr, "Pending Assignment") {
		t.Error("Fallback should have pending postmaster")
	}
}

func TestRenderDateFormats(t *testing.T) {
	m := page.Manifest{
		Photos: []page.Photo{
			{Ord: 0, Filename: "0.webp"},
		},
		About: "This is a test about section that needs to be at least 200 characters long to pass validation. We need to make sure this is long enough.",
		Postmaster: page.Postmaster{
			Name: "Test",
		},
		Accent: page.Accent{
			Primary:   "#333333",
			Secondary: "#666666",
			Tertiary:  "#999999",
			Highlight: "#f0f0f0",
			Muted:     "#cccccc",
		},
	}

	tests := []struct {
		name     string
		openedAt *time.Time
		want     string
	}{
		{
			name:     "no opened_at",
			openedAt: nil,
			want:     "Coming soon",
		},
		{
			name:     "with opened_at",
			openedAt: timePtr(time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)),
			want:     "18 November 2026",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relay := RelayInfo{
				Hostname: "test",
				City:     "Test",
				OpenedAt: tt.openedAt,
			}

			html, err := RenderPage(m, relay)
			if err != nil {
				t.Fatalf("RenderPage() error = %v", err)
			}

			if !strings.Contains(string(html), tt.want) {
				t.Errorf("HTML missing expected date: %q", tt.want)
			}
		})
	}
}

func TestRenderCSSVariables(t *testing.T) {
	m := page.Manifest{
		Photos: []page.Photo{
			{Ord: 0, Filename: "0.webp"},
		},
		About: "This is a test about section that needs to be at least 200 characters long to pass validation. We need to make sure this is long enough.",
		Postmaster: page.Postmaster{
			Name: "Test",
		},
		Accent: page.Accent{
			Primary:   "#111111",
			Secondary: "#222222",
			Tertiary:  "#333333",
			Highlight: "#444444",
			Muted:     "#555555",
		},
	}

	relay := RelayInfo{
		Hostname: "test",
		City:     "Test",
	}

	html, err := RenderPage(m, relay)
	if err != nil {
		t.Fatalf("RenderPage() error = %v", err)
	}

	htmlStr := string(html)

	// Check CSS variables
	cssChecks := []string{
		"--primary: #111111",
		"--secondary: #222222",
		"--tertiary: #333333",
		"--highlight: #444444",
		"--muted: #555555",
	}

	for _, check := range cssChecks {
		if !strings.Contains(htmlStr, check) {
			t.Errorf("HTML missing CSS variable: %q", check)
		}
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
