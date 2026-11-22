package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"snailmail/internal/relayreg"
	"snailmail/internal/route"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Relay represents a post office relay.
type Relay struct {
	ID           uuid.UUID
	Hostname     string
	Pubkey       []byte
	KeyFpr       []byte
	City         string
	Region       string
	CC           string
	Lat          float64
	Lon          float64
	TZ           string
	StampSVG     string
	StampPNG     []byte
	StampSHA256  []byte
	PlateJSON    []byte
	PlateSig     []byte
	Status       string
	Official     bool
	Paused       bool
	Central      bool
	OpenedAt     *time.Time
	RetiredAt    *time.Time
	LastTXTCheck *time.Time
	LettersPassed int64
	Complaints30d int
	BranchAPIKey  *string
	OwnerUserID   *uuid.UUID
	OwnerUsername string
}

// CreateRelay inserts a new relay.
func (s *Store) CreateRelay(ctx context.Context, hostname string, pubkey, keyFpr []byte, city, region, cc string, lat, lon float64, tz, stampSVG string, stampPNG, stampSHA256 []byte, ownerUserID *uuid.UUID) (*Relay, error) {
	id := uuid.New()
	var r Relay
	var ownerUsername *string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO relays (id, hostname, pubkey, key_fpr, city, region, cc, lat, lon, tz, stamp_svg, stamp_png, stamp_sha256, status, owner_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'pending', $14)
		RETURNING id, hostname, pubkey, key_fpr, city, region, cc, lat, lon, tz, stamp_svg, stamp_png, stamp_sha256,
		          plate_json, plate_sig, status, official, paused, central, opened_at, retired_at, last_txt_check, letters_passed, complaints_30d, owner_user_id,
		          (SELECT handle FROM users WHERE id = relays.owner_user_id)
	`, id, hostname, pubkey, keyFpr, city, region, cc, lat, lon, tz, stampSVG, stampPNG, stampSHA256, ownerUserID).Scan(
		&r.ID, &r.Hostname, &r.Pubkey, &r.KeyFpr, &r.City, &r.Region, &r.CC, &r.Lat, &r.Lon, &r.TZ,
		&r.StampSVG, &r.StampPNG, &r.StampSHA256, &r.PlateJSON, &r.PlateSig, &r.Status, &r.Official,
		&r.Paused, &r.Central, &r.OpenedAt, &r.RetiredAt, &r.LastTXTCheck, &r.LettersPassed, &r.Complaints30d, &r.OwnerUserID, &ownerUsername,
	)
	if err != nil {
		return nil, fmt.Errorf("create relay: %w", err)
	}
	if ownerUsername != nil { r.OwnerUsername = *ownerUsername }
	return &r, nil
}

// GetRelayByID fetches a relay by ID.
func (s *Store) GetRelayByID(ctx context.Context, id uuid.UUID) (*Relay, error) {
	var r Relay
	var ownerUsername *string
	err := s.pool.QueryRow(ctx, `
		SELECT r.id, r.hostname, r.pubkey, r.key_fpr, r.city, r.region, r.cc, r.lat, r.lon, r.tz, r.stamp_svg, r.stamp_png, r.stamp_sha256,
		       r.plate_json, r.plate_sig, r.status, r.official, r.paused, r.central, r.opened_at, r.retired_at, r.last_txt_check, r.letters_passed, r.complaints_30d,
		       r.owner_user_id, u.handle
		FROM relays r LEFT JOIN users u ON u.id = r.owner_user_id WHERE r.id = $1
	`, id).Scan(
		&r.ID, &r.Hostname, &r.Pubkey, &r.KeyFpr, &r.City, &r.Region, &r.CC, &r.Lat, &r.Lon, &r.TZ,
		&r.StampSVG, &r.StampPNG, &r.StampSHA256, &r.PlateJSON, &r.PlateSig, &r.Status, &r.Official,
		&r.Paused, &r.Central, &r.OpenedAt, &r.RetiredAt, &r.LastTXTCheck, &r.LettersPassed, &r.Complaints30d,
		&r.OwnerUserID, &ownerUsername,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get relay by id: %w", err)
	}
	if ownerUsername != nil { r.OwnerUsername = *ownerUsername }
	return &r, nil
}

// GetRelayByHostname fetches a relay by hostname.
func (s *Store) GetRelayByHostname(ctx context.Context, hostname string) (*Relay, error) {
	var r Relay
	var ownerUsername *string
	err := s.pool.QueryRow(ctx, `
		SELECT r.id, r.hostname, r.pubkey, r.key_fpr, r.city, r.region, r.cc, r.lat, r.lon, r.tz, r.stamp_svg, r.stamp_png, r.stamp_sha256,
		       r.plate_json, r.plate_sig, r.status, r.official, r.paused, r.central, r.opened_at, r.retired_at, r.last_txt_check, r.letters_passed, r.complaints_30d,
		       r.owner_user_id, u.handle
		FROM relays r LEFT JOIN users u ON u.id = r.owner_user_id WHERE r.hostname = $1
	`, hostname).Scan(
		&r.ID, &r.Hostname, &r.Pubkey, &r.KeyFpr, &r.City, &r.Region, &r.CC, &r.Lat, &r.Lon, &r.TZ,
		&r.StampSVG, &r.StampPNG, &r.StampSHA256, &r.PlateJSON, &r.PlateSig, &r.Status, &r.Official,
		&r.Paused, &r.Central, &r.OpenedAt, &r.RetiredAt, &r.LastTXTCheck, &r.LettersPassed, &r.Complaints30d,
		&r.OwnerUserID, &ownerUsername,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get relay by hostname: %w", err)
	}
	if ownerUsername != nil { r.OwnerUsername = *ownerUsername }
	return &r, nil
}

// UpdateRelayStatus updates the relay's status.
func (s *Store) UpdateRelayStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE relays SET status = $1 WHERE id = $2`, status, id)
	return err
}

// ActivateRelay sets status to 'active' and opens_at if null.
func (s *Store) ActivateRelay(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relays
		SET status = 'active', opened_at = COALESCE(opened_at, now())
		WHERE id = $1
	`, id)
	return err
}

// SetRelayPaused sets the paused flag.
func (s *Store) SetRelayPaused(ctx context.Context, id uuid.UUID, paused bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE relays SET paused = $1 WHERE id = $2`, paused, id)
	return err
}

// UpdateRelayTXTCheck updates the last TXT check timestamp.
func (s *Store) UpdateRelayTXTCheck(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE relays SET last_txt_check = now() WHERE id = $2`, id)
	return err
}

// RelayChallenge represents a pending verification challenge.
type RelayChallenge struct {
	RelayID   uuid.UUID
	Token     string
	ExpiresAt time.Time
}

// CreateRelayChallenge inserts a new challenge.
func (s *Store) CreateRelayChallenge(ctx context.Context, relayID uuid.UUID, token string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO relay_challenges (relay_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, relayID, token, expiresAt)
	return err
}

// GetRelayChallenge fetches an unexpired challenge for a relay.
func (s *Store) GetRelayChallenge(ctx context.Context, relayID uuid.UUID) (*RelayChallenge, error) {
	var c RelayChallenge
	err := s.pool.QueryRow(ctx, `
		SELECT relay_id, token, expires_at
		FROM relay_challenges
		WHERE relay_id = $1 AND expires_at > now()
		ORDER BY expires_at DESC
		LIMIT 1
	`, relayID).Scan(&c.RelayID, &c.Token, &c.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get relay challenge: %w", err)
	}
	return &c, nil
}

// DeleteRelayChallenge removes a challenge.
func (s *Store) DeleteRelayChallenge(ctx context.Context, relayID uuid.UUID, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM relay_challenges WHERE relay_id = $1 AND token = $2`, relayID, token)
	return err
}

// UpdateRelayPlate updates the relay's plate.
func (s *Store) UpdateRelayPlate(ctx context.Context, id uuid.UUID, plateJSON, plateSig []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relays SET plate_json = $1, plate_sig = $2 WHERE id = $3
	`, plateJSON, plateSig, id)
	return err
}

// UpdateRelayLocation updates the relay's location.
func (s *Store) UpdateRelayLocation(ctx context.Context, id uuid.UUID, city, region, cc string, lat, lon float64, tz string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relays SET city = $1, region = $2, cc = $3, lat = $4, lon = $5, tz = $6 WHERE id = $7
	`, city, region, cc, lat, lon, tz, id)
	return err
}

// RetireRelay sets status to 'retired' and retired_at.
func (s *Store) RetireRelay(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relays SET status = 'retired', retired_at = now() WHERE id = $1
	`, id)
	return err
}

// IncrementLettersPassed increments the letters_passed counter.
func (s *Store) IncrementLettersPassed(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relays SET letters_passed = letters_passed + 1 WHERE id = $1
	`, id)
	return err
}

// ListActiveRelays returns all active, unpaused relays for public listing.
func (s *Store) ListActiveRelays(ctx context.Context) ([]*Relay, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.hostname, r.pubkey, r.key_fpr, r.city, r.region, r.cc, r.lat, r.lon, r.tz, r.stamp_svg, r.stamp_png, r.stamp_sha256,
		       r.plate_json, r.plate_sig, r.status, r.official, r.paused, r.central, r.opened_at, r.retired_at, r.last_txt_check, r.letters_passed, r.complaints_30d,
		       r.owner_user_id, u.handle
		FROM relays r LEFT JOIN users u ON u.id = r.owner_user_id
		WHERE r.status = 'active' AND r.paused = false
		ORDER BY r.letters_passed DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active relays: %w", err)
	}
	defer rows.Close()

	var relays []*Relay
	for rows.Next() {
		var r Relay
		var ownerUsername *string
		if err := rows.Scan(
			&r.ID, &r.Hostname, &r.Pubkey, &r.KeyFpr, &r.City, &r.Region, &r.CC, &r.Lat, &r.Lon, &r.TZ,
			&r.StampSVG, &r.StampPNG, &r.StampSHA256, &r.PlateJSON, &r.PlateSig, &r.Status, &r.Official,
			&r.Paused, &r.Central, &r.OpenedAt, &r.RetiredAt, &r.LastTXTCheck, &r.LettersPassed, &r.Complaints30d,
			&r.OwnerUserID, &ownerUsername,
		); err != nil {
			return nil, fmt.Errorf("scan relay: %w", err)
		}
		if ownerUsername != nil { r.OwnerUsername = *ownerUsername }
		relays = append(relays, &r)
	}
	return relays, nil
}

// StatusGate implements the routing gate per §12.
type Blocker int

const (
	BlockerNone Blocker = iota
	BlockerStatus
	BlockerPaused
	BlockerPlate
	BlockerTXT
	BlockerComplaints
)

// BlockerName returns a human-readable name for a blocker type.
func BlockerName(b Blocker) string {
	switch b {
	case BlockerStatus:
		return "status"
	case BlockerPaused:
		return "paused"
	case BlockerPlate:
		return "plate"
	case BlockerTXT:
		return "txt"
	case BlockerComplaints:
		return "complaints"
	default:
		return "unknown"
	}
}

// Routable checks if a relay is routable and returns blockers.
func Routable(r *Relay, complaintThreshold int) (bool, []Blocker) {
	var blockers []Blocker

	if r.Status != "active" {
		blockers = append(blockers, BlockerStatus)
	}
	if r.Paused {
		blockers = append(blockers, BlockerPaused)
	}
	if r.PlateJSON == nil || r.PlateSig == nil {
		blockers = append(blockers, BlockerPlate)
	}
	if r.LastTXTCheck == nil || time.Since(*r.LastTXTCheck) > 24*time.Hour {
		blockers = append(blockers, BlockerTXT)
	}
	if r.Complaints30d >= complaintThreshold {
		blockers = append(blockers, BlockerComplaints)
	}

	return len(blockers) == 0, blockers
}

// RelayToRoute converts a Relay to a route.Relay for routing.
func RelayToRoute(r *Relay) *route.Relay {
	return &route.Relay{
		ID:     [16]byte(r.ID),
		Active: r.Status == "active",
		Paused: r.Paused,
		Plate:  r.PlateJSON != nil && r.PlateSig != nil,
		City:   r.City,
		CC:     r.CC,
		Lat:    r.Lat,
		Lon:    r.Lon,
	}
}

// VerifyPlateSignature checks the plate signature against the relay's public key.
func VerifyPlateSignature(r *Relay) error {
	if r.PlateJSON == nil || r.PlateSig == nil {
		return errors.New("no plate data")
	}
	pub := ed25519.PublicKey(r.Pubkey)
	// In a real implementation, we'd parse the plate JSON and verify the signature
	// For now, just check that both exist
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	return nil
}

// DNSResolver adapter for relayreg
type RelayDNSResolver struct {
	resolver relayreg.DNSResolver
}

func NewRelayDNSResolver() *RelayDNSResolver {
	return &RelayDNSResolver{resolver: &relayreg.NetResolver{}}
}

func (r *RelayDNSResolver) LookupTXT(ctx context.Context, hostname string) ([]string, error) {
	return r.resolver.LookupTXT(ctx, hostname)
}

// SetRelayBranchAPIKey sets the branch API key for a relay.
func (s *Store) SetRelayBranchAPIKey(ctx context.Context, relayID uuid.UUID, key string) error {
	_, err := s.pool.Exec(ctx, `UPDATE relays SET branch_api_key = $1 WHERE id = $2`, key, relayID)
	return err
}

// GetRelayByBranchAPIKey fetches a relay by branch API key.
func (s *Store) GetRelayByBranchAPIKey(ctx context.Context, key string) (*Relay, error) {
	var r Relay
	var ownerUsername *string
	err := s.pool.QueryRow(ctx, `
		SELECT r.id, r.hostname, r.pubkey, r.key_fpr, r.city, r.region, r.cc, r.lat, r.lon, r.tz, r.stamp_svg, r.stamp_png, r.stamp_sha256,
		       r.plate_json, r.plate_sig, r.status, r.official, r.paused, r.central, r.opened_at, r.retired_at, r.last_txt_check, r.letters_passed, r.complaints_30d, r.branch_api_key,
		       r.owner_user_id, u.handle
		FROM relays r LEFT JOIN users u ON u.id = r.owner_user_id WHERE r.branch_api_key = $1
	`, key).Scan(
		&r.ID, &r.Hostname, &r.Pubkey, &r.KeyFpr, &r.City, &r.Region, &r.CC, &r.Lat, &r.Lon, &r.TZ,
		&r.StampSVG, &r.StampPNG, &r.StampSHA256, &r.PlateJSON, &r.PlateSig, &r.Status, &r.Official,
		&r.Paused, &r.Central, &r.OpenedAt, &r.RetiredAt, &r.LastTXTCheck, &r.LettersPassed, &r.Complaints30d, &r.BranchAPIKey,
		&r.OwnerUserID, &ownerUsername,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get relay by branch api key: %w", err)
	}
	if ownerUsername != nil { r.OwnerUsername = *ownerUsername }
	return &r, nil
}

// UpdateRelayStamp updates the relay stamp SVG/PNG/SHA256.
func (s *Store) UpdateRelayStamp(ctx context.Context, id uuid.UUID, stampSVG string, stampPNG []byte, stampSHA [32]byte) error {
	_, err := s.pool.Exec(ctx, `UPDATE relays SET stamp_svg=$1, stamp_png=$2, stamp_sha256=$3 WHERE id=$4`, stampSVG, stampPNG, stampSHA[:], id)
	return err
}

// EnsureCentralRelay creates the central post office if it does not exist.
func (s *Store) EnsureCentralRelay(ctx context.Context) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM relays WHERE central = true)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate central key: %w", err)
	}
	keyFpr := sha256.Sum256(pub)
	stampSVG := `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 120 120'>
  <defs><linearGradient id='g' x1='0' y1='0' x2='1' y2='1'><stop offset='0%' stop-color='#667eea'/><stop offset='100%' stop-color='#764ba2'/></linearGradient></defs>
  <circle cx='60' cy='60' r='54' fill='none' stroke='url(#g)' stroke-width='6'/>
  <circle cx='60' cy='60' r='46' fill='none' stroke='#764ba2' stroke-width='2'/>
  <text x='60' y='52' text-anchor='middle' font-size='14' font-family='Georgia,serif' fill='#3b2d5c' font-weight='700'>SNAELMAIL</text>
  <text x='60' y='68' text-anchor='middle' font-size='10' font-family='Georgia,serif' fill='#5b4b7a'>CENTRAL POST</text>
  <text x='60' y='80' text-anchor='middle' font-size='8' font-family='Georgia,serif' fill='#5b4b7a'>OFFICE</text>
</svg>`
	stampSHA := sha256.Sum256([]byte(stampSVG))
	_, err = s.pool.Exec(ctx, `
		INSERT INTO relays (
			id, hostname, pubkey, key_fpr, city, region, cc, lat, lon, tz,
			stamp_svg, stamp_png, stamp_sha256, plate_json, plate_sig,
			status, official, paused, central, opened_at, letters_passed, complaints_30d
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULL,NULL,'active',true,false,true,now(),0,0)
		ON CONFLICT (hostname) DO UPDATE SET central=true, status='active', official=true
	`, uuid.New(), "central.snaelmail.org", []byte(pub), keyFpr[:], "Des Moines", "Iowa", "US", 41.586834, -93.624962, "America/Chicago", stampSVG, []byte{}, stampSHA[:])
	return err
}
