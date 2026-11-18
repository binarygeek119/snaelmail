// Package clock implements the postal calendar: walking forward in time while
// respecting Sundays, holidays, and Saturday half-speed.
package clock

import (
	"fmt"
	"time"

	"snailmail/internal/config"
)

// PostalCalendar walks time forward according to postal rules.
type PostalCalendar struct {
	holidays    map[string]bool // "MM-DD" and "YYYY-MM-DD"
	noMove      map[int]bool    // weekday numbers (0=Sunday)
	satFactor   float64
}

// New loads the calendar from config and holidays file.
func New(cfg *config.Calendar, holidaysPath string) (*PostalCalendar, error) {
	hols := make(map[string]bool)
	if holidaysPath != "" {
		h, err := config.LoadHolidays(holidaysPath)
		if err != nil {
			return nil, err
		}
		for _, d := range h.Recurring {
			hols[d] = true
		}
		for _, d := range h.Specific {
			hols[d] = true
		}
	}
	noMove := make(map[int]bool)
	for _, wd := range cfg.NoMovementWeekdays {
		noMove[wd] = true
	}
	return &PostalCalendar{
		holidays:  hols,
		noMove:    noMove,
		satFactor: cfg.SaturdaySpeedFactor,
	}, nil
}

// IsWorkingDay returns true if mail moves on this date.
func (c *PostalCalendar) IsWorkingDay(t time.Time) bool {
	wd := int(t.Weekday())
	if c.noMove[wd] {
		return false
	}
	// Check specific date first (YYYY-MM-DD), then recurring (MM-DD)
	if c.holidays[t.Format("2006-01-02")] {
		return false
	}
	if c.holidays[t.Format("01-02")] {
		return false
	}
	return true
}

// SpeedFactor returns the speed multiplier for a given day.
// Saturday = satFactor, working days = 1.0, non-working = 0.
func (c *PostalCalendar) SpeedFactor(t time.Time) float64 {
	if !c.IsWorkingDay(t) {
		return 0
	}
	if t.Weekday() == time.Saturday {
		return c.satFactor
	}
	return 1.0
}

// Advance moves forward by the time required to cover km at speed km/day,
// starting from t. Returns the new time after consumption.
func (c *PostalCalendar) Advance(t time.Time, km float64, speedKmPerDay float64) (time.Time, error) {
	if speedKmPerDay <= 0 {
		return t, fmt.Errorf("clock: speed must be positive, got %f", speedKmPerDay)
	}
	if km < 0 {
		return t, fmt.Errorf("clock: km must be non-negative, got %f", km)
	}
	if km == 0 {
		return t, nil
	}

	// Total hours needed at full speed
	totalHoursNeeded := (km / speedKmPerDay) * 24.0

	cur := t
	for totalHoursNeeded > 0 {
		factor := c.SpeedFactor(cur)
		if factor == 0 {
			// Skip to next day
			cur = nextDayStart(cur)
			continue
		}

		// Hours available today from current moment
		hoursLeftToday := hoursUntilDayEnd(cur)
		hoursAvailable := hoursLeftToday * factor

		if totalHoursNeeded <= hoursAvailable {
			// Consume remaining hours today
			hoursToConsume := totalHoursNeeded / factor
			cur = cur.Add(time.Duration(hoursToConsume * float64(time.Hour)))
			totalHoursNeeded = 0
		} else {
			// Consume all available hours today, move to next day
			totalHoursNeeded -= hoursAvailable
			cur = nextDayStart(cur)
		}
	}

	return cur, nil
}

// Hold adds wall-clock hours to t, respecting the calendar (skips non-working days).
func (c *PostalCalendar) Hold(t time.Time, hours float64) (time.Time, error) {
	if hours < 0 {
		return t, fmt.Errorf("clock: hold hours must be non-negative, got %f", hours)
	}
	if hours == 0 {
		return t, nil
	}

	cur := t
	remaining := hours
	for remaining > 0 {
		if !c.IsWorkingDay(cur) {
			cur = nextDayStart(cur)
			continue
		}
		hoursLeftToday := hoursUntilDayEnd(cur)
		if remaining <= hoursLeftToday {
			cur = cur.Add(time.Duration(remaining * float64(time.Hour)))
			remaining = 0
		} else {
			remaining -= hoursLeftToday
			cur = nextDayStart(cur)
		}
	}
	return cur, nil
}

// FloorToDispatch returns the next dispatch window (start of next working day
// if t is outside working hours, or t if already on a working day).
func (c *PostalCalendar) FloorToDispatch(t time.Time) time.Time {
	if c.IsWorkingDay(t) {
		return t
	}
	// Advance to next working day
	cur := nextDayStart(t)
	for !c.IsWorkingDay(cur) {
		cur = cur.AddDate(0, 0, 1)
	}
	return cur
}

// nextDayStart returns midnight of the next day.
func nextDayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
}

// hoursUntilDayEnd returns hours from t until midnight.
func hoursUntilDayEnd(t time.Time) float64 {
	midnight := nextDayStart(t)
	return midnight.Sub(t).Hours()
}
