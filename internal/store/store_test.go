package store

import (
	"context"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	// In-memory DB; MaxOpenConns(1) keeps it alive on the single connection.
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestContacts(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	if err := db.UpsertContact(ctx, Contact{JID: "1@s", Name: "Dad", Language: "Swahili", Tier: TierAuto}); err != nil {
		t.Fatal(err)
	}
	c, err := db.GetContact(ctx, "1@s")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "Dad" || c.Tier != TierAuto || c.Language != "Swahili" {
		t.Fatalf("unexpected contact: %+v", c)
	}

	// Default tier when unset.
	if err := db.UpsertContact(ctx, Contact{JID: "2@s", Name: "Stranger"}); err != nil {
		t.Fatal(err)
	}
	c2, _ := db.GetContact(ctx, "2@s")
	if c2.Tier != TierNotify {
		t.Fatalf("want default tier notify, got %q", c2.Tier)
	}

	// Summary update preserves profile.
	if err := db.UpdateSummary(ctx, "1@s", "talked about the car"); err != nil {
		t.Fatal(err)
	}
	c, _ = db.GetContact(ctx, "1@s")
	if c.Summary != "talked about the car" || c.Name != "Dad" {
		t.Fatalf("summary update clobbered profile: %+v", c)
	}

	list, err := db.ListContacts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 contacts, got %d", len(list))
	}

	if _, err := db.GetContact(ctx, "missing@s"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	if v, err := db.GetSetting(ctx, "missing"); err != nil || v != "" {
		t.Fatalf("want empty, got %q err %v", v, err)
	}
	if err := db.SetSetting(ctx, "llm.order", `["ollama","groq"]`); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting(ctx, "llm.order", `["groq"]`); err != nil { // overwrite
		t.Fatal(err)
	}
	v, _ := db.GetSetting(ctx, "llm.order")
	if v != `["groq"]` {
		t.Fatalf("want overwritten value, got %q", v)
	}
}

func TestDrafts(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	id, err := db.CreateDraft(ctx, Draft{ChatJID: "1@s", SenderJID: "1@s", Incoming: "hi", Reply: "hello", Confidence: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	pending, _ := db.ListDrafts(ctx, DraftPending)
	if len(pending) != 1 || pending[0].ID != id {
		t.Fatalf("want 1 pending draft, got %+v", pending)
	}

	// Approve with an edited reply.
	if err := db.SetDraftStatus(ctx, id, DraftApproved, "hello there"); err != nil {
		t.Fatal(err)
	}
	dr, _ := db.GetDraft(ctx, id)
	if dr.Status != DraftApproved || dr.Reply != "hello there" {
		t.Fatalf("approve/edit failed: %+v", dr)
	}
	if pend, _ := db.ListDrafts(ctx, DraftPending); len(pend) != 0 {
		t.Fatalf("want 0 pending after approve, got %d", len(pend))
	}
}

func TestExpireOldPending(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	if _, err := db.CreateDraft(ctx, Draft{ChatJID: "1@s", SenderJID: "1@s"}); err != nil {
		t.Fatal(err)
	}
	// maxAge of -1s makes everything "old".
	n, err := db.ExpireOldPending(ctx, -time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 expired, got %d", n)
	}
}

func TestActivityAndDailyContext(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	for i := 0; i < 3; i++ {
		if err := db.LogActivity(ctx, "sent", "1@s", "sent a reply"); err != nil {
			t.Fatal(err)
		}
	}
	acts, _ := db.ListActivity(ctx, 2)
	if len(acts) != 2 {
		t.Fatalf("want 2 activities (limited), got %d", len(acts))
	}

	if err := db.SetDailyContext(ctx, "2026-05-26", "deep work all day"); err != nil {
		t.Fatal(err)
	}
	txt, _ := db.GetDailyContext(ctx, "2026-05-26")
	if txt != "deep work all day" {
		t.Fatalf("daily context mismatch: %q", txt)
	}
	if txt, _ := db.GetDailyContext(ctx, "1999-01-01"); txt != "" {
		t.Fatalf("want empty for missing day, got %q", txt)
	}
}

func TestMessagesRollingHistory(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	// Interleave the contact and the bot/owner, more than the cap.
	for i := 0; i < 6; i++ {
		if err := db.AddMessage(ctx, Message{ChatJID: "1@s", Name: "Dad", Text: "them-" + itoa(i)}, 4); err != nil {
			t.Fatal(err)
		}
		if err := db.AddMessage(ctx, Message{ChatJID: "1@s", FromMe: true, Text: "me-" + itoa(i)}, 4); err != nil {
			t.Fatal(err)
		}
	}
	// A different chat must stay independent.
	if err := db.AddMessage(ctx, Message{ChatJID: "2@s", Text: "other"}, 4); err != nil {
		t.Fatal(err)
	}

	msgs, err := db.RecentMessages(ctx, "1@s", 10)
	if err != nil {
		t.Fatal(err)
	}
	// Capped at keep=4.
	if len(msgs) != 4 {
		t.Fatalf("want 4 (capped), got %d", len(msgs))
	}
	// Oldest-first ordering: the last 4 inserted were them-5, me-5 ... wait order:
	// inserts ...them-4, me-4, them-5, me-5 → newest 4 = them-4, me-4, them-5, me-5.
	want := []string{"them-4", "me-4", "them-5", "me-5"}
	for i, m := range msgs {
		if m.Text != want[i] {
			t.Fatalf("pos %d: want %q, got %q", i, want[i], m.Text)
		}
	}
	// fromMe preserved.
	if msgs[0].FromMe || !msgs[1].FromMe {
		t.Fatalf("fromMe flags wrong: %+v", msgs)
	}
	// Independent chat untouched.
	if other, _ := db.RecentMessages(ctx, "2@s", 10); len(other) != 1 || other[0].Text != "other" {
		t.Fatalf("chat 2 isolation broken: %+v", other)
	}
}

func itoa(i int) string { return string(rune('0' + i)) }
