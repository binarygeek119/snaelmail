// Package page implements the public page system for relays per §14.
package page

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Manifest is the structured page data per §14.
type Manifest struct {
	// City photos: 1-6 images, 16:10 ratio, required
	Photos []Photo `json:"photos" validate:"required,min=1,max=6"`

	// City map: optional single image
	Map *Photo `json:"map,omitempty"`

	// City logo: PNG/SVG, 64KB max
	Logo *Logo `json:"logo,omitempty" validate:"omitempty"`

	// About the city: 200-900 chars, required
	About string `json:"about" validate:"required,min=200,max=900"`

	// City facts: ≤5 pairs, 24/60 chars
	Facts []Fact `json:"facts,omitempty" validate:"max=5"`

	// Postmaster info
	Postmaster Postmaster `json:"postmaster" validate:"required"`

	// Dispatch times: ≤3
	DispatchTimes []string `json:"dispatch_times,omitempty" validate:"max=3"`

	// Classes handled: enum
	Classes []string `json:"classes,omitempty"`

	// Accent colors: 5 fixed hex values
	Accent Accent `json:"accent" validate:"required"`

	// Masthead variant
	Masthead string `json:"masthead,omitempty"`

	// Typography pairing
	TypePairing string `json:"type_pairing,omitempty"`
}

// Photo is a city photo with caption.
type Photo struct {
	// Ordinal position (0-5)
	Ord int `json:"ord"`
	// Filename (images/NN.webp)
	Filename string `json:"filename"`
	// Caption: max 120 chars
	Caption string `json:"caption,omitempty" validate:"max=120"`
}

// Logo is the city logo.
type Logo struct {
	// Filename (logo.svg or logo.png)
	Filename string `json:"filename"`
	// Size in bytes
	Size int `json:"size" validate:"max=65536"`
}

// Fact is a city fact key-value pair.
type Fact struct {
	Key   string `json:"key" validate:"max=24"`
	Value string `json:"value" validate:"max=60"`
}

// Postmaster is the postmaster info.
type Postmaster struct {
	// Name: 60 chars, required
	Name string `json:"name" validate:"required,max=60"`
	// Role: 60 chars
	Role string `json:"role,omitempty" validate:"max=60"`
	// Bio: 400 chars
	Bio string `json:"bio,omitempty" validate:"max=400"`
}

// Accent is the 5 accent colors.
type Accent struct {
	Primary   string `json:"primary" validate:"required,hexcolor"`
	Secondary string `json:"secondary" validate:"required,hexcolor"`
	Tertiary  string `json:"tertiary" validate:"required,hexcolor"`
	Highlight string `json:"highlight" validate:"required,hexcolor"`
	Muted     string `json:"muted" validate:"required,hexcolor"`
}

// Page is a relay's public page.
type Page struct {
	RelayID     uuid.UUID
	Revision    int
	Manifest    Manifest
	Signature   []byte
	Status      string // draft, review, live, rejected
	PublishedAt *time.Time
	ReviewNote  *string
}

// Validate checks the manifest against §14 limits.
func Validate(m *Manifest) error {
	// Photos: 0-6 allowed
	if len(m.Photos) > 6 {
		return errors.New("photos: must be 0-6 images")
	}
	for i, p := range m.Photos {
		if len(p.Caption) > 120 {
			return fmt.Errorf("photo[%d].caption: max 120 chars", i)
		}
		if p.Ord != i {
			return fmt.Errorf("photo[%d].ord: must be %d, got %d", i, i, p.Ord)
		}
	}

	// Logo: 64KB max
	if m.Logo != nil && m.Logo.Size > 65536 {
		return errors.New("logo: max 64KB")
	}

	// About: 200-900 chars
	if len(m.About) < 200 || len(m.About) > 900 {
		return fmt.Errorf("about: must be 200-900 chars, got %d", len(m.About))
	}

	// Facts: ≤5
	if len(m.Facts) > 5 {
		return errors.New("facts: max 5")
	}
	for i, f := range m.Facts {
		if len(f.Key) > 24 {
			return fmt.Errorf("fact[%d].key: max 24 chars", i)
		}
		if len(f.Value) > 60 {
			return fmt.Errorf("fact[%d].value: max 60 chars", i)
		}
	}

	// Postmaster: required
	if m.Postmaster.Name == "" {
		return errors.New("postmaster.name: required")
	}
	if len(m.Postmaster.Name) > 60 {
		return errors.New("postmaster.name: max 60 chars")
	}
	if len(m.Postmaster.Role) > 60 {
		return errors.New("postmaster.role: max 60 chars")
	}
	if len(m.Postmaster.Bio) > 400 {
		return errors.New("postmaster.bio: max 400 chars")
	}

	// Dispatch times: ≤3
	if len(m.DispatchTimes) > 3 {
		return errors.New("dispatch_times: max 3")
	}

	// Accent: 5 hex colors
	if err := validateHexColor(m.Accent.Primary); err != nil {
		return fmt.Errorf("accent.primary: %w", err)
	}
	if err := validateHexColor(m.Accent.Secondary); err != nil {
		return fmt.Errorf("accent.secondary: %w", err)
	}
	if err := validateHexColor(m.Accent.Tertiary); err != nil {
		return fmt.Errorf("accent.tertiary: %w", err)
	}
	if err := validateHexColor(m.Accent.Highlight); err != nil {
		return fmt.Errorf("accent.highlight: %w", err)
	}
	if err := validateHexColor(m.Accent.Muted); err != nil {
		return fmt.Errorf("accent.muted: %w", err)
	}

	return nil
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateHexColor(s string) error {
	if !hexColorRe.MatchString(s) {
		return fmt.Errorf("must be #RRGGBB format")
	}
	return nil
}
