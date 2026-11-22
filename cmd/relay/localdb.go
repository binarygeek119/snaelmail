package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func relayDBPath() string { return filepath.Join(relayDataDir(), "relay.db") }

func openRelayDB() (*sql.DB, error) {
	if err := os.MkdirAll(relayDataDir(), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", relayDBPath())
	if err != nil {
		return nil, err
	}
	if err := initRelayDB(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func initRelayDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS relay_info (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			main_site TEXT NOT NULL DEFAULT 'https://snaelmail.org',
			hostname TEXT NOT NULL DEFAULT '',
			city TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			cc TEXT NOT NULL DEFAULT '',
			lat REAL NOT NULL DEFAULT 0,
			lon REAL NOT NULL DEFAULT 0,
			tz TEXT NOT NULL DEFAULT 'UTC',
			pubkey TEXT NOT NULL DEFAULT '',
			fingerprint TEXT NOT NULL DEFAULT '',
			relay_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			owner_username TEXT NOT NULL DEFAULT '',
			owner_api_key TEXT NOT NULL DEFAULT ''
		);
		INSERT OR IGNORE INTO relay_info(id) VALUES(1);
	`)
	if err != nil { return err }
	_, _ = db.Exec(`ALTER TABLE relay_info ADD COLUMN owner_username TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE relay_info ADD COLUMN owner_api_key TEXT NOT NULL DEFAULT ''`)
	return nil
	return err
}

func loadSetupStateDB() setupState {
	s := setupState{MainSite: "https://snaelmail.org", TZ: "UTC"}
	db, err := openRelayDB()
	if err != nil {
		return s
	}
	defer db.Close()
	row := db.QueryRow(`SELECT main_site,hostname,city,region,cc,lat,lon,tz,pubkey,fingerprint,relay_id,status,last_error,owner_username,owner_api_key FROM relay_info WHERE id=1`)
	var lat, lon float64
	if err := row.Scan(&s.MainSite, &s.Hostname, &s.City, &s.Region, &s.CC, &lat, &lon, &s.TZ, &s.Pubkey, &s.Fingerprint, &s.RelayID, &s.Status, &s.LastError, &s.OwnerUsername, &s.OwnerAPIKey); err != nil {
		return setupState{MainSite: "https://snaelmail.org", TZ: "UTC"}
	}
	s.Lat, s.Lon = lat, lon
	if s.MainSite == "" { s.MainSite = "https://snaelmail.org" }
	if s.TZ == "" { s.TZ = "UTC" }
	return s
}

func saveSetupStateDB(s setupState) error {
	db, err := openRelayDB()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`UPDATE relay_info SET main_site=?,hostname=?,city=?,region=?,cc=?,lat=?,lon=?,tz=?,pubkey=?,fingerprint=?,relay_id=?,status=?,last_error=?,owner_username=?,owner_api_key=? WHERE id=1`,
		s.MainSite, s.Hostname, s.City, s.Region, s.CC, s.Lat, s.Lon, s.TZ, s.Pubkey, s.Fingerprint, s.RelayID, s.Status, s.LastError, s.OwnerUsername, s.OwnerAPIKey)
	if err != nil {
		return fmt.Errorf("save relay db: %w", err)
	}
	return nil
}
