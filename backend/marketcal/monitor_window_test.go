package marketcal

import (
	"testing"
	"time"
)

func TestMergeMonitorWindows(t *testing.T) {
	base := time.Date(2026, 11, 2, 9, 0, 0, 0, shanghai)
	in := []MonitorWindow{
		{Start: base, End: base.Add(2 * time.Hour)},
		{Start: base.Add(time.Hour), End: base.Add(3 * time.Hour)},
		{Start: base.Add(3 * time.Hour), End: base.Add(4 * time.Hour)},
		{Start: base.Add(5 * time.Hour), End: base.Add(6 * time.Hour)},
	}
	got := mergeMonitorWindows(in)
	if len(got) != 2 {
		t.Fatalf("mergeMonitorWindows() len = %d, want 2: %#v", len(got), got)
	}
	if !got[0].Start.Equal(base) || !got[0].End.Equal(base.Add(4*time.Hour)) {
		t.Fatalf("first window = %#v", got[0])
	}
	if !got[1].Start.Equal(base.Add(5*time.Hour)) || !got[1].End.Equal(base.Add(6*time.Hour)) {
		t.Fatalf("second window = %#v", got[1])
	}
}

func TestNextSessionSkipsNonTradingDays(t *testing.T) {
	loc := shanghai
	now := time.Date(2026, 11, 6, 16, 0, 0, 0, loc) // Friday 16:00
	isTradingDay := func(d time.Time) bool {
		return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday
	}
	start, end, ok := nextSession(now, loc, 9, 15, 15, 0, isTradingDay)
	if !ok {
		t.Fatal("nextSession() ok = false")
	}
	wantStart := time.Date(2026, 11, 9, 9, 15, 0, 0, loc)
	wantEnd := time.Date(2026, 11, 9, 15, 0, 0, 0, loc)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("nextSession() = %s ~ %s, want %s ~ %s", start, end, wantStart, wantEnd)
	}
}

func TestNextSessionInsideWindowKeepsStart(t *testing.T) {
	loc := shanghai
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, loc)
	isTradingDay := func(time.Time) bool { return true }
	start, end, ok := nextSession(now, loc, 9, 15, 15, 0, isTradingDay)
	if !ok {
		t.Fatal("nextSession() ok = false")
	}
	if !start.Before(now) || !end.After(now) {
		t.Fatalf("expected window containing now, got %s ~ %s", start, end)
	}
}
