package schedule

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	loc := time.UTC
	// Before the time today -> today.
	now := time.Date(2026, 5, 26, 6, 0, 0, 0, loc)
	got := nextRun(now, 8, 30)
	want := time.Date(2026, 5, 26, 8, 30, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("want %v, got %v", want, got)
	}

	// After the time today -> tomorrow.
	now = time.Date(2026, 5, 26, 9, 0, 0, 0, loc)
	got = nextRun(now, 8, 30)
	want = time.Date(2026, 5, 27, 8, 30, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("want %v, got %v", want, got)
	}

	// Exactly at the time -> next day (strictly after).
	now = time.Date(2026, 5, 26, 8, 30, 0, 0, loc)
	got = nextRun(now, 8, 30)
	if !got.Equal(time.Date(2026, 5, 27, 8, 30, 0, 0, loc)) {
		t.Fatalf("at-time should roll to tomorrow, got %v", got)
	}
}
