package page

import (
	"testing"
)

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name    string
		m       Manifest
		wantErr bool
	}{
		{
			name: "valid minimal manifest",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp", Caption: "Test"},
				},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: false,
		},
		{
			name: "no photos",
			m: Manifest{
				Photos: []Photo{},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "too many photos",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp"},
					{Ord: 1, Filename: "1.webp"},
					{Ord: 2, Filename: "2.webp"},
					{Ord: 3, Filename: "3.webp"},
					{Ord: 4, Filename: "4.webp"},
					{Ord: 5, Filename: "5.webp"},
					{Ord: 6, Filename: "6.webp"},
				},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "photo caption too long",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp", Caption: "This caption is way too long and exceeds the 120 character limit that is enforced by the validation rules in the page package"},
				},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "about too short",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp"},
				},
				About: "Too short",
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "about too long",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp"},
				},
				About: "This is a test about section that needs to be at least 200 characters long to pass validation. We need to make sure this is long enough. And now we keep adding more text to make it exceed the 900 character limit. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. More text here to make it even longer and exceed the limit significantly by adding even more words and phrases to ensure we go well beyond the 900 character threshold that is enforced by the validation logic in the page package. Adding more content to push it over the edge.",
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "missing postmaster name",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp"},
				},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Postmaster: Postmaster{
					Name: "",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid accent color",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp"},
				},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "not-a-color",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "too many facts",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp"},
				},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Facts: []Fact{
					{Key: "Fact 1", Value: "Value 1"},
					{Key: "Fact 2", Value: "Value 2"},
					{Key: "Fact 3", Value: "Value 3"},
					{Key: "Fact 4", Value: "Value 4"},
					{Key: "Fact 5", Value: "Value 5"},
					{Key: "Fact 6", Value: "Value 6"},
				},
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "fact key too long",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp"},
				},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Facts: []Fact{
					{Key: "This fact key is way too long and exceeds the 24 character limit", Value: "Value"},
				},
				Postmaster: Postmaster{
					Name: "Test Postmaster",
				},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
			},
			wantErr: true,
		},
		{
			name: "valid with all optional fields",
			m: Manifest{
				Photos: []Photo{
					{Ord: 0, Filename: "0.webp", Caption: "Photo 1"},
					{Ord: 1, Filename: "1.webp", Caption: "Photo 2"},
				},
				Logo: &Logo{
					Filename: "logo.svg",
					Size:     1024,
				},
				About: "This is a comprehensive description of our charming post office that has been serving the community for generations. We take pride in handling every piece of mail with care and dedication, ensuring that letters reach their destinations safely and on time. Our dedicated staff works tirelessly to maintain the highest standards of postal service.",
				Facts: []Fact{
					{Key: "Population", Value: "100,000"},
					{Key: "Elevation", Value: "500m"},
				},
				Postmaster: Postmaster{
					Name: "Jane Doe",
					Role: "Chief Letter Sorter",
					Bio:  "Jane has been working at the post office for 20 years.",
				},
				DispatchTimes: []string{"9:00 AM", "2:00 PM", "5:00 PM"},
				Classes:       []string{"surface", "first", "air"},
				Accent: Accent{
					Primary:   "#333333",
					Secondary: "#666666",
					Tertiary:  "#999999",
					Highlight: "#f0f0f0",
					Muted:     "#cccccc",
				},
				Masthead:      "classic",
				TypePairing:   "serif-sans",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.m)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHexColor(t *testing.T) {
	tests := []struct {
		color   string
		wantErr bool
	}{
		{"#333333", false},
		{"#ABCDEF", false},
		{"#abcdef", false},
		{"333333", true},      // missing #
		{"#333", true},        // too short
		{"#3333333", true},    // too long
		{"#GGGGGG", true},     // invalid hex
		{"not-a-color", true},
	}

	for _, tt := range tests {
		t.Run(tt.color, func(t *testing.T) {
			err := validateHexColor(tt.color)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateHexColor(%q) error = %v, wantErr %v", tt.color, err, tt.wantErr)
			}
		})
	}
}
