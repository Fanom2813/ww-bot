// Package store is the app's local persistence layer (SQLite via mattn).
// It holds DERIVED data only — contact profiles + rolling summaries, settings,
// the approvals/draft queue, an activity log, and daily context. Raw chat
// messages are NEVER persisted here (see the privacy model in PLAN.md).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ErrNotFound is returned when a lookup finds no row.
var ErrNotFound = errors.New("store: not found")

// DB wraps the SQLite connection.
type DB struct{ sql *sql.DB }

// Open opens (and migrates) the data store at the given SQLite DSN, e.g.
// "file:.../data.db?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL".
func Open(ctx context.Context, dsn string) (*DB, error) {
	sqldb, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// SQLite handles one writer at a time; serialize to avoid "database is locked".
	sqldb.SetMaxOpenConns(1)
	if err := sqldb.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	d := &DB{sql: sqldb}
	if err := d.migrate(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

// Close closes the underlying database.
func (d *DB) Close() error { return d.sql.Close() }

func (d *DB) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS contacts (
	jid        TEXT PRIMARY KEY,
	name       TEXT NOT NULL DEFAULT '',
	language   TEXT NOT NULL DEFAULT '',
	style      TEXT NOT NULL DEFAULT '',
	tier       TEXT NOT NULL DEFAULT 'notify',
	summary    TEXT NOT NULL DEFAULT '',
	notes      TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS drafts (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	chat_jid    TEXT NOT NULL,
	sender_jid  TEXT NOT NULL,
	sender_name TEXT NOT NULL DEFAULT '',
	incoming    TEXT NOT NULL DEFAULT '',
	reply       TEXT NOT NULL DEFAULT '',
	reason      TEXT NOT NULL DEFAULT '',
	confidence  REAL NOT NULL DEFAULT 0,
	status      TEXT NOT NULL DEFAULT 'pending',
	created_at  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_drafts_status ON drafts(status);

CREATE TABLE IF NOT EXISTS activity (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	ts       INTEGER NOT NULL DEFAULT 0,
	kind     TEXT NOT NULL,
	chat_jid TEXT NOT NULL DEFAULT '',
	summary  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_activity_ts ON activity(ts DESC);

CREATE TABLE IF NOT EXISTS daily_context (
	day  TEXT PRIMARY KEY, -- YYYY-MM-DD
	text TEXT NOT NULL DEFAULT ''
);
`
	if _, err := d.sql.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Contacts
// ---------------------------------------------------------------------------

// TrustTier controls how the bot may respond to a contact.
type TrustTier string

const (
	// TierAuto: the bot may send autonomously (subject to confidence + guardrails).
	TierAuto TrustTier = "auto"
	// TierDraft: the bot drafts replies for the user to approve.
	TierDraft TrustTier = "draft"
	// TierNotify: the bot never replies, only notifies. Default for new/unknown.
	TierNotify TrustTier = "notify"
)

// Contact is a per-person profile plus a rolling, model-maintained summary.
type Contact struct {
	JID       string    `json:"jid"`
	Name      string    `json:"name"`
	Language  string    `json:"language"`
	Style     string    `json:"style"`
	Tier      TrustTier `json:"tier"`
	Summary   string    `json:"summary"`
	Notes     string    `json:"notes"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UpsertContact inserts or updates the editable profile fields (not summary).
func (d *DB) UpsertContact(ctx context.Context, c Contact) error {
	if c.Tier == "" {
		c.Tier = TierNotify
	}
	_, err := d.sql.ExecContext(ctx, `
INSERT INTO contacts (jid, name, language, style, tier, notes, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(jid) DO UPDATE SET
	name=excluded.name, language=excluded.language, style=excluded.style,
	tier=excluded.tier, notes=excluded.notes, updated_at=excluded.updated_at`,
		c.JID, c.Name, c.Language, c.Style, string(c.Tier), c.Notes, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("store: upsert contact: %w", err)
	}
	return nil
}

// GetContact returns the contact or ErrNotFound.
func (d *DB) GetContact(ctx context.Context, jid string) (Contact, error) {
	var c Contact
	var tier string
	var updated int64
	err := d.sql.QueryRowContext(ctx,
		`SELECT jid, name, language, style, tier, summary, notes, updated_at FROM contacts WHERE jid=?`, jid).
		Scan(&c.JID, &c.Name, &c.Language, &c.Style, &tier, &c.Summary, &c.Notes, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Contact{}, ErrNotFound
	}
	if err != nil {
		return Contact{}, fmt.Errorf("store: get contact: %w", err)
	}
	c.Tier = TrustTier(tier)
	c.UpdatedAt = time.Unix(updated, 0)
	return c, nil
}

// ListContacts returns all contacts ordered by name.
func (d *DB) ListContacts(ctx context.Context) ([]Contact, error) {
	rows, err := d.sql.QueryContext(ctx,
		`SELECT jid, name, language, style, tier, summary, notes, updated_at FROM contacts ORDER BY name, jid`)
	if err != nil {
		return nil, fmt.Errorf("store: list contacts: %w", err)
	}
	defer rows.Close()
	var out []Contact
	for rows.Next() {
		var c Contact
		var tier string
		var updated int64
		if err := rows.Scan(&c.JID, &c.Name, &c.Language, &c.Style, &tier, &c.Summary, &c.Notes, &updated); err != nil {
			return nil, err
		}
		c.Tier = TrustTier(tier)
		c.UpdatedAt = time.Unix(updated, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateSummary replaces a contact's rolling summary (creating the row if absent).
func (d *DB) UpdateSummary(ctx context.Context, jid, summary string) error {
	_, err := d.sql.ExecContext(ctx, `
INSERT INTO contacts (jid, summary, updated_at) VALUES (?, ?, ?)
ON CONFLICT(jid) DO UPDATE SET summary=excluded.summary, updated_at=excluded.updated_at`,
		jid, summary, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("store: update summary: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Settings (key/value; values are opaque strings, often JSON)
// ---------------------------------------------------------------------------

// GetSetting returns the value for key, or ("", nil) if unset.
func (d *DB) GetSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := d.sql.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("store: get setting: %w", err)
	}
	return v, nil
}

// SetSetting upserts a setting value.
func (d *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := d.sql.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value)
	if err != nil {
		return fmt.Errorf("store: set setting: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Drafts / approvals queue
// ---------------------------------------------------------------------------

// DraftStatus is the lifecycle of a proposed reply.
type DraftStatus string

const (
	DraftPending  DraftStatus = "pending"
	DraftApproved DraftStatus = "approved"
	DraftRejected DraftStatus = "rejected"
	DraftSent     DraftStatus = "sent"
	DraftExpired  DraftStatus = "expired"
)

// Draft is a proposed reply awaiting the user's decision.
type Draft struct {
	ID         int64       `json:"id"`
	ChatJID    string      `json:"chatJid"`
	SenderJID  string      `json:"senderJid"`
	SenderName string      `json:"senderName"`
	Incoming   string      `json:"incoming"`
	Reply      string      `json:"reply"`
	Reason     string      `json:"reason"`
	Confidence float64     `json:"confidence"`
	Status     DraftStatus `json:"status"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// CreateDraft inserts a pending draft and returns its id.
func (d *DB) CreateDraft(ctx context.Context, dr Draft) (int64, error) {
	res, err := d.sql.ExecContext(ctx, `
INSERT INTO drafts (chat_jid, sender_jid, sender_name, incoming, reply, reason, confidence, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
		dr.ChatJID, dr.SenderJID, dr.SenderName, dr.Incoming, dr.Reply, dr.Reason, dr.Confidence, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("store: create draft: %w", err)
	}
	return res.LastInsertId()
}

// ListDrafts returns drafts with the given status (newest first).
func (d *DB) ListDrafts(ctx context.Context, status DraftStatus) ([]Draft, error) {
	rows, err := d.sql.QueryContext(ctx, `
SELECT id, chat_jid, sender_jid, sender_name, incoming, reply, reason, confidence, status, created_at
FROM drafts WHERE status=? ORDER BY created_at DESC`, string(status))
	if err != nil {
		return nil, fmt.Errorf("store: list drafts: %w", err)
	}
	defer rows.Close()
	var out []Draft
	for rows.Next() {
		dr, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, dr)
	}
	return out, rows.Err()
}

// GetDraft returns a draft by id or ErrNotFound.
func (d *DB) GetDraft(ctx context.Context, id int64) (Draft, error) {
	row := d.sql.QueryRowContext(ctx, `
SELECT id, chat_jid, sender_jid, sender_name, incoming, reply, reason, confidence, status, created_at
FROM drafts WHERE id=?`, id)
	dr, err := scanDraft(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, ErrNotFound
	}
	return dr, err
}

// SetDraftStatus updates a draft's status, optionally replacing the reply text
// (used when the user edits before approving). Pass reply="" to keep existing.
func (d *DB) SetDraftStatus(ctx context.Context, id int64, status DraftStatus, reply string) error {
	if reply != "" {
		_, err := d.sql.ExecContext(ctx, `UPDATE drafts SET status=?, reply=? WHERE id=?`, string(status), reply, id)
		return err
	}
	_, err := d.sql.ExecContext(ctx, `UPDATE drafts SET status=? WHERE id=?`, string(status), id)
	return err
}

// ExpireOldPending marks pending drafts older than maxAge as expired.
func (d *DB) ExpireOldPending(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge).Unix()
	res, err := d.sql.ExecContext(ctx,
		`UPDATE drafts SET status='expired' WHERE status='pending' AND created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

type scanner interface{ Scan(dest ...any) error }

func scanDraft(s scanner) (Draft, error) {
	var dr Draft
	var status string
	var created int64
	if err := s.Scan(&dr.ID, &dr.ChatJID, &dr.SenderJID, &dr.SenderName, &dr.Incoming,
		&dr.Reply, &dr.Reason, &dr.Confidence, &status, &created); err != nil {
		return Draft{}, err
	}
	dr.Status = DraftStatus(status)
	dr.CreatedAt = time.Unix(created, 0)
	return dr, nil
}

// ---------------------------------------------------------------------------
// Activity log (audit trail of the bot's own actions)
// ---------------------------------------------------------------------------

// Activity is one audit entry.
type Activity struct {
	ID      int64     `json:"id"`
	Ts      time.Time `json:"ts"`
	Kind    string    `json:"kind"` // sent | draft | call | flag | proactive | system
	ChatJID string    `json:"chatJid"`
	Summary string    `json:"summary"`
}

// LogActivity appends an audit entry.
func (d *DB) LogActivity(ctx context.Context, kind, chatJID, summary string) error {
	_, err := d.sql.ExecContext(ctx,
		`INSERT INTO activity (ts, kind, chat_jid, summary) VALUES (?, ?, ?, ?)`,
		time.Now().Unix(), kind, chatJID, summary)
	if err != nil {
		return fmt.Errorf("store: log activity: %w", err)
	}
	return nil
}

// ListActivity returns the most recent entries up to limit.
func (d *DB) ListActivity(ctx context.Context, limit int) ([]Activity, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.sql.QueryContext(ctx,
		`SELECT id, ts, kind, chat_jid, summary FROM activity ORDER BY ts DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list activity: %w", err)
	}
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		var a Activity
		var ts int64
		if err := rows.Scan(&a.ID, &ts, &a.Kind, &a.ChatJID, &a.Summary); err != nil {
			return nil, err
		}
		a.Ts = time.Unix(ts, 0)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Daily context ("today I'm doing X")
// ---------------------------------------------------------------------------

// SetDailyContext sets the free-text plan for a day (YYYY-MM-DD).
func (d *DB) SetDailyContext(ctx context.Context, day, text string) error {
	_, err := d.sql.ExecContext(ctx,
		`INSERT INTO daily_context (day, text) VALUES (?, ?) ON CONFLICT(day) DO UPDATE SET text=excluded.text`,
		day, text)
	if err != nil {
		return fmt.Errorf("store: set daily context: %w", err)
	}
	return nil
}

// GetDailyContext returns the plan text for a day, or "" if none.
func (d *DB) GetDailyContext(ctx context.Context, day string) (string, error) {
	var text string
	err := d.sql.QueryRowContext(ctx, `SELECT text FROM daily_context WHERE day=?`, day).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("store: get daily context: %w", err)
	}
	return text, nil
}
