package safety

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestInQuietHours(t *testing.T) {
	cases := []struct {
		h, start, end int
		want          bool
	}{
		{3, 23, 7, true},   // wrap, inside
		{8, 23, 7, false},  // wrap, outside
		{23, 23, 7, true},  // wrap, at start
		{7, 23, 7, false},  // wrap, at end (exclusive)
		{12, 9, 17, true},  // normal, inside
		{8, 9, 17, false},  // normal, before
		{5, 0, 0, false},   // disabled
	}
	for _, c := range cases {
		if got := inQuietHours(c.h, c.start, c.end); got != c.want {
			t.Errorf("inQuietHours(%d,%d,%d)=%v want %v", c.h, c.start, c.end, got, c.want)
		}
	}
}

func TestComputeWait(t *testing.T) {
	g := New(Config{PerMinute: 6, PerContactCooldown: 30 * time.Second}, nil, Options{})
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

	// No history: no wait.
	if w := g.computeWait("a@s", now); w != 0 {
		t.Fatalf("want 0, got %v", w)
	}

	// Per-contact cooldown dominates global interval.
	g.lastByContact["a@s"] = now.Add(-5 * time.Second) // 25s left of 30s cooldown
	g.lastSend = now.Add(-5 * time.Second)             // global min interval = 10s -> 5s left
	if w := g.computeWait("a@s", now); w != 25*time.Second {
		t.Fatalf("want 25s (cooldown), got %v", w)
	}
}

// newTestGate builds a gate with no-op sleep so tests don't actually wait.
func newTestGate(t *testing.T, cfg Config, send SendFunc, opt Options) *Gate {
	t.Helper()
	g := New(cfg, send, opt)
	g.sleep = func(time.Duration) {}
	g.now = func() time.Time { return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC) } // noon, no quiet hours
	g.Start(context.Background())
	t.Cleanup(g.Stop)
	return g
}

func TestSendHappyPath(t *testing.T) {
	sent := make(chan string, 1)
	g := newTestGate(t, Config{}, func(_ context.Context, to, text string) error {
		sent <- to + "|" + text
		return nil
	}, Options{})

	if err := g.Enqueue(Job{ToJID: "a@s", Text: "hello"}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-sent:
		if got != "a@s|hello" {
			t.Fatalf("unexpected send: %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("message was not sent")
	}
}

func TestPauseRejectsEnqueue(t *testing.T) {
	g := newTestGate(t, Config{}, func(context.Context, string, string) error { return nil }, Options{})
	g.Pause()
	if err := g.Enqueue(Job{ToJID: "a@s", Text: "x"}); !errors.Is(err, ErrPaused) {
		t.Fatalf("want ErrPaused, got %v", err)
	}
	g.Resume()
	if err := g.Enqueue(Job{ToJID: "a@s", Text: "x"}); err != nil {
		t.Fatalf("after resume want nil, got %v", err)
	}
}

func TestQuietHoursDrops(t *testing.T) {
	var mu sync.Mutex
	var sent int
	dropped := make(chan string, 1)
	g := New(Config{QuietStart: 23, QuietEnd: 7}, func(context.Context, string, string) error {
		mu.Lock()
		sent++
		mu.Unlock()
		return nil
	}, Options{OnDrop: func(_ Job, reason string) { dropped <- reason }})
	g.sleep = func(time.Duration) {}
	g.now = func() time.Time { return time.Date(2026, 5, 26, 3, 0, 0, 0, time.UTC) } // 3am
	g.Start(context.Background())
	defer g.Stop()

	if err := g.Enqueue(Job{ToJID: "a@s", Text: "x"}); err != nil {
		t.Fatal(err)
	}
	select {
	case reason := <-dropped:
		if reason != "quiet hours" {
			t.Fatalf("want quiet-hours drop, got %q", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a drop during quiet hours")
	}
	mu.Lock()
	defer mu.Unlock()
	if sent != 0 {
		t.Fatalf("nothing should send during quiet hours, sent=%d", sent)
	}
}

func TestQuietHoursBypass(t *testing.T) {
	sent := make(chan struct{}, 1)
	g := New(Config{QuietStart: 23, QuietEnd: 7}, func(context.Context, string, string) error {
		sent <- struct{}{}
		return nil
	}, Options{})
	g.sleep = func(time.Duration) {}
	g.now = func() time.Time { return time.Date(2026, 5, 26, 3, 0, 0, 0, time.UTC) }
	g.Start(context.Background())
	defer g.Stop()

	// A scheduled dua bypasses quiet hours.
	g.Enqueue(Job{ToJID: "dad@s", Text: "Good morning", BypassQuiet: true})
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("bypass-quiet message should still send")
	}
}

func TestDailyCap(t *testing.T) {
	var mu sync.Mutex
	var sent int
	drops := make(chan string, 4)
	g := New(Config{PerDay: 1}, func(context.Context, string, string) error {
		mu.Lock()
		sent++
		mu.Unlock()
		return nil
	}, Options{OnDrop: func(_ Job, r string) { drops <- r }})
	g.sleep = func(time.Duration) {}
	g.now = func() time.Time { return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC) }
	g.Start(context.Background())
	defer g.Stop()

	g.Enqueue(Job{ToJID: "a@s", Text: "1"})
	g.Enqueue(Job{ToJID: "a@s", Text: "2"})

	select {
	case r := <-drops:
		if r != "daily cap reached" {
			t.Fatalf("want daily-cap drop, got %q", r)
		}
	case <-time.After(time.Second):
		t.Fatal("second message should be dropped by daily cap")
	}
	mu.Lock()
	defer mu.Unlock()
	if sent != 1 {
		t.Fatalf("only 1 should send under PerDay=1, sent=%d", sent)
	}
}
