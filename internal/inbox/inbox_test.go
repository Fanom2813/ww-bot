package inbox

import (
	"testing"
	"time"
)

func TestCoalescesBurst(t *testing.T) {
	flushed := make(chan Batch, 1)
	d := New(Config{Quiet: 30 * time.Millisecond, MaxWait: time.Second},
		func(b Batch) { flushed <- b })

	d.Add(Msg{ChatJID: "1@s", Text: "hi"})
	d.Add(Msg{ChatJID: "1@s", Text: "you there?"})
	d.Add(Msg{ChatJID: "1@s", Text: "??"})

	select {
	case b := <-flushed:
		if len(b.Messages) != 3 {
			t.Fatalf("want 3 coalesced messages, got %d", len(b.Messages))
		}
		if b.ChatJID != "1@s" {
			t.Fatalf("chat mismatch: %s", b.ChatJID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("flush did not fire")
	}
}

func TestHumanTakeoverCancels(t *testing.T) {
	flushed := make(chan Batch, 1)
	d := New(Config{Quiet: 40 * time.Millisecond, MaxWait: time.Second},
		func(b Batch) { flushed <- b })

	d.Add(Msg{ChatJID: "1@s", Text: "hello"})
	// User replies manually before the quiet window elapses.
	d.Add(Msg{ChatJID: "1@s", Text: "got it", IsFromMe: true})

	select {
	case <-flushed:
		t.Fatal("flush should have been cancelled by human takeover")
	case <-time.After(120 * time.Millisecond):
		// expected: no flush
	}
}

func TestSeparateChatsIndependent(t *testing.T) {
	flushed := make(chan Batch, 2)
	d := New(Config{Quiet: 25 * time.Millisecond, MaxWait: time.Second},
		func(b Batch) { flushed <- b })

	d.Add(Msg{ChatJID: "a@s", Text: "x"})
	d.Add(Msg{ChatJID: "b@s", Text: "y"})

	got := map[string]int{}
	for i := 0; i < 2; i++ {
		select {
		case b := <-flushed:
			got[b.ChatJID]++
		case <-time.After(500 * time.Millisecond):
			t.Fatal("missing flush")
		}
	}
	if got["a@s"] != 1 || got["b@s"] != 1 {
		t.Fatalf("each chat should flush once: %v", got)
	}
}
