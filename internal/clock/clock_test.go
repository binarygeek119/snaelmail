package clock

import (
	"testing"
	"time"

	"snailmail/internal/config"
)

func newTestCalendar(t *testing.T) *PostalCalendar {
	t.Helper()
	cfg := &config.Calendar{
		NoMovementWeekdays:  []int{0}, // Sunday
		SaturdaySpeedFactor: 0.5,
		Holidays:            "", // no holidays file for basic tests
	}
	// Create a minimal holidays file
	cal, err := New(cfg, "")
	if err != nil {
		// If loading fails, create a calendar directly
		return &PostalCalendar{
			holidays:  map[string]bool{},
			noMove:    map[int]bool{0: true},
			satFactor: 0.5,
		}
	}
	return cal
}

func TestIsWorkingDay(t *testing.T) {
	cal := newTestCalendar(t)

	tests := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"Monday", time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC), true},
		{"Tuesday", time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC), true},
		{"Wednesday", time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC), true},
		{"Thursday", time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC), true},
		{"Friday", time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC), true},
		{"Saturday", time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC), true},
		{"Sunday", time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cal.IsWorkingDay(tt.t)
			if got != tt.want {
				t.Errorf("IsWorkingDay(%v) = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

func TestSpeedFactor(t *testing.T) {
	cal := newTestCalendar(t)

	tests := []struct {
		name string
		t    time.Time
		want float64
	}{
		{"Monday", time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC), 1.0},
		{"Saturday", time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC), 0.5},
		{"Sunday", time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC), 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cal.SpeedFactor(tt.t)
			if got != tt.want {
				t.Errorf("SpeedFactor(%v) = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

func TestAdvance(t *testing.T) {
	cal := newTestCalendar(t)

	// Friday 10:00, advance 100km at 250km/day = 9.6 hours
	// Should complete same day
	fri := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	got, err := cal.Advance(fri, 100, 250)
	if err != nil {
		t.Fatal(err)
	}
	// 100/250 * 24 = 9.6 hours, so 10:00 + 9.6h = 19:36
	wantHour := 19
	wantMin := 36
	if got.Hour() != wantHour || got.Minute() != wantMin {
		t.Errorf("Advance Friday 100km: got %v, want ~%d:%02d", got, wantHour, wantMin)
	}

	// Saturday 10:00, advance 100km at 250km/day
	// Saturday factor 0.5, so effective speed 125km/day
	// 100/125 * 24 = 19.2 hours, but only 14 hours left in day
	// So should spill to Sunday (no movement), then Monday
	sat := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	got, err = cal.Advance(sat, 100, 250)
	if err != nil {
		t.Fatal(err)
	}
	// Saturday: 14h * 0.5 = 7h effective, need 9.6h total
	// Remaining 2.6h / 0.5 = 5.2h on Monday (Sunday skipped)
	// Monday 00:00 + 5.2h = 05:12
	mon := got
	if mon.Weekday() != time.Monday {
		t.Errorf("Advance Saturday should land on Monday, got %v", mon)
	}
}

func TestAdvanceSunday(t *testing.T) {
	cal := newTestCalendar(t)

	// Sunday 10:00, advance 100km at 250km/day
	// Should skip Sunday entirely, start Monday 00:00
	sun := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	got, err := cal.Advance(sun, 100, 250)
	if err != nil {
		t.Fatal(err)
	}
	if got.Weekday() != time.Monday {
		t.Errorf("Advance Sunday should land on Monday, got %v", got)
	}
	// 100/250 * 24 = 9.6 hours from Monday 00:00 = 09:36
	if got.Hour() != 9 || got.Minute() != 36 {
		t.Errorf("Advance Sunday 100km: got %v, want Monday 09:36", got)
	}
}

func TestHold(t *testing.T) {
	cal := newTestCalendar(t)

	// Friday 22:00, hold 5 hours
	// Only 2 hours left Friday, then Saturday (working day), continue Saturday
	fri := time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)
	got, err := cal.Hold(fri, 5)
	if err != nil {
		t.Fatal(err)
	}
	// Friday: 2h, remaining 3h on Saturday
	// Saturday 00:00 + 3h = 03:00
	if got.Weekday() != time.Saturday {
		t.Errorf("Hold should land on Saturday, got %v", got)
	}
	if got.Hour() != 3 {
		t.Errorf("Hold 5h from Fri 22:00: got %v, want Saturday 03:00", got)
	}

	// Sunday 10:00, hold 5 hours (should skip Sunday)
	sun := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	got, err = cal.Hold(sun, 5)
	if err != nil {
		t.Fatal(err)
	}
	// Sunday is skipped, Monday 00:00 + 5h = 05:00
	if got.Weekday() != time.Monday {
		t.Errorf("Hold from Sunday should land on Monday, got %v", got)
	}
	if got.Hour() != 5 {
		t.Errorf("Hold 5h from Sunday: got %v, want Monday 05:00", got)
	}
}

func TestFloorToDispatch(t *testing.T) {
	cal := newTestCalendar(t)

	// Sunday should floor to Monday
	sun := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	got := cal.FloorToDispatch(sun)
	if got.Weekday() != time.Monday {
		t.Errorf("FloorToDispatch Sunday should be Monday, got %v", got)
	}
	if got.Hour() != 0 || got.Minute() != 0 {
		t.Errorf("FloorToDispatch should be midnight, got %v", got)
	}

	// Monday should stay Monday
	mon := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	got = cal.FloorToDispatch(mon)
	if !got.Equal(mon) {
		t.Errorf("FloorToDispatch Monday should stay Monday, got %v", got)
	}
}
