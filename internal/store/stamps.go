package store

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type StampVersion struct {
	ID         uuid.UUID  `json:"id"`
	RelayID    uuid.UUID  `json:"relay_id"`
	RelayHostname string  `json:"relay_hostname,omitempty"`
	VersionNum int        `json:"version_num"`
	SHA256     []byte     `json:"-"`
	SHA256Hex  string     `json:"sha256"`
	Format     string     `json:"format"`
	SVGContent string     `json:"svg_content,omitempty"`
	PNGContent []byte     `json:"-"`
	PNGBase64  string     `json:"png_base64,omitempty"`
	Status     string     `json:"status"`
	IsCurrent  bool       `json:"is_current"`
	StampDate  time.Time  `json:"stamp_date"`
	Event      *string    `json:"event,omitempty"`
	Honor      *string    `json:"honor,omitempty"`
	Info       *string    `json:"info,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy *uuid.UUID `json:"reviewed_by,omitempty"`
}

func (s *Store) CreateStampVersion(ctx context.Context, relayID uuid.UUID, format, svg string, png []byte, stampDate time.Time, event, honor, info *string) (*StampVersion, error) {
	var hash []byte
	if format == "svg" {
		h := sha256.Sum256([]byte(svg))
		hash = h[:]
	} else {
		h := sha256.Sum256(png)
		hash = h[:]
	}
	var sv StampVersion
	err := s.pool.QueryRow(ctx, `
		INSERT INTO stamp_versions (relay_id, version_num, sha256, format, svg_content, png_content, status, is_current, stamp_date, event, honor, info)
		VALUES ($1, COALESCE((SELECT MAX(version_num)+1 FROM stamp_versions WHERE relay_id=$1),1), $2, $3, $4, $5, 'pending', false, $6, $7, $8, $9)
		RETURNING id, relay_id, version_num, sha256, format, svg_content, png_content, status, is_current, stamp_date, event, honor, info, created_at, reviewed_at, reviewed_by
	`, relayID, hash, format, nullIfEmpty(svg), nullBytes(png), stampDate, event, honor, info).Scan(
		&sv.ID, &sv.RelayID, &sv.VersionNum, &sv.SHA256, &sv.Format, &sv.SVGContent, &sv.PNGContent, &sv.Status, &sv.IsCurrent, &sv.StampDate, &sv.Event, &sv.Honor, &sv.Info, &sv.CreatedAt, &sv.ReviewedAt, &sv.ReviewedBy,
	)
	if err != nil { return nil, fmt.Errorf("create stamp version: %w", err) }
	sv.SHA256Hex = fmt.Sprintf("%x", sv.SHA256)
	return &sv, nil
}

func nullIfEmpty(s string) *string { if s == "" { return nil }; return &s }
func nullBytes(b []byte) []byte { if len(b) == 0 { return nil }; return b }

func scanStampVersion(scanner interface{ Scan(dest ...any) error }) (*StampVersion, error) {
	var sv StampVersion
	var svg *string
	err := scanner.Scan(&sv.ID, &sv.RelayID, &sv.VersionNum, &sv.SHA256, &sv.Format, &svg, &sv.PNGContent, &sv.Status, &sv.IsCurrent, &sv.StampDate, &sv.Event, &sv.Honor, &sv.Info, &sv.CreatedAt, &sv.ReviewedAt, &sv.ReviewedBy)
	if err != nil { return nil, err }
	if svg != nil { sv.SVGContent = *svg }
	sv.SHA256Hex = fmt.Sprintf("%x", sv.SHA256)
	if len(sv.PNGContent) > 0 { sv.PNGBase64 = base64.StdEncoding.EncodeToString(sv.PNGContent) }
	return &sv, nil
}

const stampVersionCols = `id, relay_id, version_num, sha256, format, svg_content, png_content, status, is_current, stamp_date, event, honor, info, created_at, reviewed_at, reviewed_by`

func (s *Store) ListPendingStampVersions(ctx context.Context) ([]*StampVersion, error) {
	rows, err := s.pool.Query(ctx, `SELECT sv.id, sv.relay_id, sv.version_num, sv.sha256, sv.format, sv.svg_content, sv.png_content, sv.status, sv.is_current, sv.stamp_date, sv.event, sv.honor, sv.info, sv.created_at, sv.reviewed_at, sv.reviewed_by, r.hostname
		FROM stamp_versions sv JOIN relays r ON r.id=sv.relay_id WHERE sv.status='pending' ORDER BY sv.created_at ASC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []*StampVersion
	for rows.Next() {
		var sv StampVersion
		var svg *string
		var hostname string
		if err := rows.Scan(&sv.ID,&sv.RelayID,&sv.VersionNum,&sv.SHA256,&sv.Format,&svg,&sv.PNGContent,&sv.Status,&sv.IsCurrent,&sv.StampDate,&sv.Event,&sv.Honor,&sv.Info,&sv.CreatedAt,&sv.ReviewedAt,&sv.ReviewedBy,&hostname); err != nil { return nil, err }
		if svg != nil { sv.SVGContent = *svg }
		sv.SHA256Hex = fmt.Sprintf("%x", sv.SHA256)
		if len(sv.PNGContent) > 0 { sv.PNGBase64 = base64.StdEncoding.EncodeToString(sv.PNGContent) }
		sv.RelayHostname = hostname
		out = append(out, &sv)
	}
	return out, nil
}

func (s *Store) ListRelayStampVersions(ctx context.Context, relayID uuid.UUID) ([]*StampVersion, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+stampVersionCols+` FROM stamp_versions WHERE relay_id=$1 ORDER BY version_num DESC`, relayID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []*StampVersion
	for rows.Next() {
		sv, err := scanStampVersion(rows)
		if err != nil { return nil, err }
		out = append(out, sv)
	}
	return out, nil
}

func (s *Store) GetStampVersion(ctx context.Context, id uuid.UUID) (*StampVersion, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+stampVersionCols+` FROM stamp_versions WHERE id=$1`, id)
	sv, err := scanStampVersion(row)
	if err != nil { if err.Error() == "no rows in result set" { return nil, nil }; return nil, err }
	return sv, nil
}

func (s *Store) ApproveStampVersion(ctx context.Context, id, reviewer uuid.UUID, makeCurrent bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)
	var relayID uuid.UUID
	var format string
	var svg *string
	var png []byte
	err = tx.QueryRow(ctx, `UPDATE stamp_versions SET status='approved', reviewed_at=now(), reviewed_by=$2 WHERE id=$1 AND status='pending' RETURNING relay_id, format, svg_content, png_content`, id, reviewer).Scan(&relayID, &format, &svg, &png)
	if err != nil { return fmt.Errorf("approve stamp: %w", err) }
	if makeCurrent {
		if _, err := tx.Exec(ctx, `UPDATE stamp_versions SET is_current=false, status=CASE WHEN status='approved' AND id<>$2 THEN 'retired' ELSE status END WHERE relay_id=$1 AND is_current=true`, relayID, id); err != nil { return err }
		if _, err := tx.Exec(ctx, `UPDATE stamp_versions SET is_current=true, status='approved' WHERE id=$1`, id); err != nil { return err }
		// update relay cached stamp
		svgStr := ""; if svg != nil { svgStr = *svg }
		if png == nil { png = []byte{} }
		var hash [32]byte
		if format == "svg" { hash = sha256.Sum256([]byte(svgStr)) } else { hash = sha256.Sum256(png) }
		if _, err := tx.Exec(ctx, `UPDATE relays SET stamp_svg=$2, stamp_png=$3, stamp_sha256=$4 WHERE id=$1`, relayID, svgStr, png, hash[:]); err != nil { return err }
	}
	return tx.Commit(ctx)
}

func (s *Store) RejectStampVersion(ctx context.Context, id, reviewer uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE stamp_versions SET status='rejected', reviewed_at=now(), reviewed_by=$2 WHERE id=$1 AND status='pending'`, id, reviewer)
	return err
}

func (s *Store) SetCurrentStampVersion(ctx context.Context, relayID, stampID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)
	var format string
	var svg *string
	var png []byte
	var status string
	err = tx.QueryRow(ctx, `SELECT format, svg_content, png_content, status FROM stamp_versions WHERE id=$1 AND relay_id=$2`, stampID, relayID).Scan(&format, &svg, &png, &status)
	if err != nil { return fmt.Errorf("stamp not found: %w", err) }
	if status != "approved" && status != "retired" { return fmt.Errorf("stamp must be approved or retired") }
	if _, err := tx.Exec(ctx, `UPDATE stamp_versions SET is_current=false, status=CASE WHEN status='approved' THEN 'retired' ELSE status END WHERE relay_id=$1 AND is_current=true`, relayID); err != nil { return err }
	if _, err := tx.Exec(ctx, `UPDATE stamp_versions SET is_current=true, status='approved' WHERE id=$1`, stampID); err != nil { return err }
	svgStr := ""; if svg != nil { svgStr = *svg }
	if png == nil { png = []byte{} }
	var hash [32]byte
	if format == "svg" { hash = sha256.Sum256([]byte(svgStr)) } else { hash = sha256.Sum256(png) }
	if _, err := tx.Exec(ctx, `UPDATE relays SET stamp_svg=$2, stamp_png=$3, stamp_sha256=$4 WHERE id=$1`, relayID, svgStr, png, hash[:]); err != nil { return err }
	return tx.Commit(ctx)
}

func (s *Store) GetVintageStampVersions(ctx context.Context, relayID uuid.UUID) ([]*StampVersion, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+stampVersionCols+` FROM stamp_versions WHERE relay_id=$1 AND status IN ('approved','retired') AND is_current=false ORDER BY version_num DESC`, relayID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []*StampVersion
	for rows.Next() {
		sv, err := scanStampVersion(rows)
		if err != nil { return nil, err }
		out = append(out, sv)
	}
	return out, nil
}

func (s *Store) GetCurrentStampVersion(ctx context.Context, relayID uuid.UUID) (*StampVersion, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+stampVersionCols+` FROM stamp_versions WHERE relay_id=$1 AND is_current=true LIMIT 1`, relayID)
	sv, err := scanStampVersion(row)
	if err != nil { if err.Error() == "no rows in result set" { return nil, nil }; return nil, err }
	return sv, nil
}
