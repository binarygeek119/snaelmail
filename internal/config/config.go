package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	MailDomain string `toml:"mail_domain"`
	SiteKey    string `toml:"site_key"`

	SpeedKmPerDay SpeedKmPerDay `toml:"speed_km_per_day"`
	Handling      Handling      `toml:"handling"`
	Route         Route         `toml:"route"`
	Calendar      Calendar      `toml:"calendar"`
	Customs       Customs       `toml:"customs"`
	Stamp         Stamp         `toml:"stamp"`
	Limits        Limits        `toml:"limits"`
}

type SpeedKmPerDay struct {
	Surface int `toml:"surface"`
	First   int `toml:"first"`
	Air     int `toml:"air"`
}

type Handling struct {
	BaseHours       int `toml:"base_hours"`
	Per100QueuedH   int `toml:"per_100_queued_h"`
	GapThresholdKm  int `toml:"gap_threshold_km"`
	GapHoursPer600  int `toml:"gap_hours_per_600"`
}

type Route struct {
	MaxHops        int     `toml:"max_hops"`
	LocalRadiusKm  int     `toml:"local_radius_km"`
	ConeDeg        int     `toml:"cone_deg"`
	MinProgress    float64 `toml:"min_progress"`
}

type Calendar struct {
	NoMovementWeekdays  []int `toml:"no_movement_weekdays"`
	SaturdaySpeedFactor float64 `toml:"saturday_speed_factor"`
	Holidays            string  `toml:"holidays"`
}

type Customs struct {
	HoldMinH int `toml:"hold_min_h"`
	HoldMaxH int `toml:"hold_max_h"`
}

type Stamp struct {
	Canvas    []int   `toml:"canvas"`
	Positions [][]int `toml:"positions"`
}

type Limits struct {
	MaxSurfaceDays int `toml:"max_surface_days"`
	MaxAirDays     int `toml:"max_air_days"`
}

// Holidays represents the holiday calendar loaded from holidays.toml.
type Holidays struct {
	Recurring []string `toml:"recurring"`
	Specific  []string `toml:"specific"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	// Resolve holidays path relative to config dir
	if cfg.Calendar.Holidays != "" && !filepath.IsAbs(cfg.Calendar.Holidays) {
		cfg.Calendar.Holidays = filepath.Join(filepath.Dir(path), cfg.Calendar.Holidays)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.MailDomain == "" {
		return fmt.Errorf("config: mail_domain is required")
	}
	if c.SiteKey == "" {
		return fmt.Errorf("config: site_key is required")
	}
	if c.SpeedKmPerDay.Surface <= 0 || c.SpeedKmPerDay.First <= 0 || c.SpeedKmPerDay.Air <= 0 {
		return fmt.Errorf("config: speed_km_per_day values must be positive")
	}
	if c.Route.MaxHops < 1 {
		return fmt.Errorf("config: route.max_hops must be >= 1")
	}
	if c.Route.LocalRadiusKm < 0 {
		return fmt.Errorf("config: route.local_radius_km must be >= 0")
	}
	if c.Route.ConeDeg < 1 || c.Route.ConeDeg > 180 {
		return fmt.Errorf("config: route.cone_deg must be 1..180")
	}
	if c.Calendar.SaturdaySpeedFactor <= 0 || c.Calendar.SaturdaySpeedFactor > 1 {
		return fmt.Errorf("config: calendar.saturday_speed_factor must be (0, 1]")
	}
	if len(c.Stamp.Canvas) != 2 {
		return fmt.Errorf("config: stamp.canvas must be [width, height]")
	}
	if len(c.Stamp.Positions) == 0 {
		return fmt.Errorf("config: stamp.positions must have at least one entry")
	}
	return nil
}

// LoadHolidays reads the holidays TOML file referenced by the calendar config.
func LoadHolidays(path string) (*Holidays, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("holidays: read %s: %w", path, err)
	}
	var h Holidays
	if err := toml.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("holidays: parse %s: %w", path, err)
	}
	return &h, nil
}
