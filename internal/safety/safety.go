// Package safety is the single throttled exit for ALL outbound WhatsApp
// messages — autonomous replies, approved drafts, and proactive greetings. It
// is the main anti-ban mechanism: it serializes sends through one worker and
// applies a human-like reaction delay, per-contact cooldown, global pacing,
// daily caps, quiet hours, and a global pause/kill-switch. Nothing should call
// the WhatsApp send API except through this gate.
package safety

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wwbot/internal/dbg"
)

// Errors returned by Enqueue.
var (
	ErrPaused    = errors.New("safety: gate is paused")
	ErrQueueFull = errors.New("safety: send queue full")
	ErrStopped   = errors.New("safety: gate stopped")
)

// SendFunc actually delivers a message (wraps wa.Client.SendText).
type SendFunc func(ctx context.Context, toJID, text string) error

// TypingFunc optionally signals "composing" presence for a duration.
type TypingFunc func(ctx context.Context, toJID string, d time.Duration)

// Config tunes the gate. Zero values get sensible defaults.
type Config struct {
	MinDelay           time.Duration // reaction delay lower bound
	MaxDelay           time.Duration // reaction delay upper bound
	PerMinute          int           // global pacing: max sends/min (0 = unlimited)
	PerDay             int           // hard daily cap (0 = unlimited)
	PerContactCooldown time.Duration // min gap between two sends to the same contact
	QuietStart         int           // quiet-hours start hour [0,24)
	QuietEnd           int           // quiet-hours end hour [0,24); == start disables
	TypingPerChar      time.Duration // typing simulation per character
	MaxTyping          time.Duration // cap on typing simulation
}

func (c *Config) withDefaults() {
	if c.MinDelay == 0 {
		c.MinDelay = 3 * time.Second
	}
	if c.MaxDelay < c.MinDelay {
		c.MaxDelay = c.MinDelay + 8*time.Second
	}
	if c.PerContactCooldown == 0 {
		c.PerContactCooldown = 10 * time.Second
	}
	if c.TypingPerChar == 0 {
		c.TypingPerChar = 40 * time.Millisecond
	}
	if c.MaxTyping == 0 {
		c.MaxTyping = 6 * time.Second
	}
}

// Job is a queued outbound message. If Parts has more than one entry, they are
// sent as separate WhatsApp messages with short natural pauses between them (the
// bot "texting in a few bursts"); otherwise Text is sent as a single message.
type Job struct {
	ToJID       string
	Text        string
	Parts       []string
	BypassQuiet bool // proactive duas / user-approved sends may bypass quiet hours
}

// Gate serializes and paces outbound messages.
type Gate struct {
	cfg    Config
	send   SendFunc
	typing TypingFunc
	onSent func(toJID, text string)
	onDrop func(j Job, reason string)
	jobs   chan Job
	quit   chan struct{}
	wg     sync.WaitGroup
	paused atomic.Bool
	ctx    context.Context

	mu            sync.Mutex
	lastSend      time.Time
	lastByContact map[string]time.Time
	dayCount      int
	dayKey        string

	// injectables for testing
	now   func() time.Time
	sleep func(time.Duration)
}

// Options bundles optional hooks.
type Options struct {
	Typing TypingFunc
	OnSent func(toJID, text string) // audit hook on successful send
	OnDrop func(j Job, reason string)
}

// New constructs a Gate. Call Start to launch the worker.
func New(cfg Config, send SendFunc, opt Options) *Gate {
	cfg.withDefaults()
	return &Gate{
		cfg:           cfg,
		send:          send,
		typing:        opt.Typing,
		onSent:        opt.OnSent,
		onDrop:        opt.OnDrop,
		jobs:          make(chan Job, 256),
		quit:          make(chan struct{}),
		lastByContact: make(map[string]time.Time),
		now:           time.Now,
		sleep:         time.Sleep,
	}
}

// Start launches the worker goroutine.
func (g *Gate) Start(ctx context.Context) {
	g.ctx = ctx
	g.wg.Add(1)
	go g.loop()
}

// Stop stops the worker and waits for it to finish.
func (g *Gate) Stop() {
	close(g.quit)
	g.wg.Wait()
}

// Pause / Resume toggle the global kill-switch.
func (g *Gate) Pause()       { g.paused.Store(true) }
func (g *Gate) Resume()      { g.paused.Store(false) }
func (g *Gate) Paused() bool { return g.paused.Load() }

// Enqueue queues a message for paced delivery. Returns ErrPaused when the gate
// is paused so the caller can record the suppression instead of sending.
func (g *Gate) Enqueue(j Job) error {
	if g.paused.Load() {
		return ErrPaused
	}
	select {
	case <-g.quit:
		return ErrStopped
	case g.jobs <- j:
		return nil
	default:
		return ErrQueueFull
	}
}

func (g *Gate) loop() {
	defer g.wg.Done()
	defer dbg.Recover("safety.loop")
	for {
		select {
		case <-g.quit:
			return
		case j := <-g.jobs:
			g.process(j)
		}
	}
}

func (g *Gate) process(j Job) {
	if g.paused.Load() {
		g.drop(j, "paused")
		return
	}
	now := g.now()
	if !j.BypassQuiet && inQuietHours(now.Hour(), g.cfg.QuietStart, g.cfg.QuietEnd) {
		g.drop(j, "quiet hours")
		return
	}
	if g.dailyCapReached(now) {
		g.drop(j, "daily cap reached")
		return
	}

	parts := j.Parts
	if len(parts) == 0 {
		parts = []string{j.Text}
	}

	// Pacing once for the whole turn: per-contact cooldown + global min interval.
	if w := g.computeWait(j.ToJID, now); w > 0 {
		g.sleep(w)
	}

	for i, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if i == 0 {
			g.sleep(g.reactionDelay()) // human reaction delay before the first message
		} else {
			g.sleep(g.interPartPause()) // short natural gap before each follow-up
		}
		if g.typing != nil {
			g.typing(g.ctx, j.ToJID, g.typingDuration(part))
		}
		if err := g.send(g.ctx, j.ToJID, part); err != nil {
			g.drop(Job{ToJID: j.ToJID, Text: part}, "send error: "+err.Error())
			return
		}
		if g.onSent != nil {
			g.onSent(j.ToJID, part)
		}
	}
	g.recordSend(j.ToJID)
}

// interPartPause is a short, randomized gap between consecutive messages in the
// same reply (~0.8–2s), so a split reply reads like natural back-to-back texts.
func (g *Gate) interPartPause() time.Duration {
	return 800*time.Millisecond + time.Duration(rand.Int64N(int64(1200*time.Millisecond)))
}

func (g *Gate) drop(j Job, reason string) {
	if g.onDrop != nil {
		g.onDrop(j, reason)
	}
}

// computeWait returns how long to wait before sending to satisfy the
// per-contact cooldown and the global min interval.
func (g *Gate) computeWait(toJID string, now time.Time) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	var wait time.Duration
	if last, ok := g.lastByContact[toJID]; ok {
		if r := g.cfg.PerContactCooldown - now.Sub(last); r > wait {
			wait = r
		}
	}
	if !g.lastSend.IsZero() {
		if r := g.minInterval() - now.Sub(g.lastSend); r > wait {
			wait = r
		}
	}
	return wait
}

func (g *Gate) minInterval() time.Duration {
	if g.cfg.PerMinute <= 0 {
		return 0
	}
	return time.Minute / time.Duration(g.cfg.PerMinute)
}

func (g *Gate) reactionDelay() time.Duration {
	if g.cfg.MaxDelay <= g.cfg.MinDelay {
		return g.cfg.MinDelay
	}
	span := g.cfg.MaxDelay - g.cfg.MinDelay
	return g.cfg.MinDelay + time.Duration(rand.Int64N(int64(span)))
}

func (g *Gate) typingDuration(text string) time.Duration {
	d := time.Duration(len(text)) * g.cfg.TypingPerChar
	if d > g.cfg.MaxTyping {
		d = g.cfg.MaxTyping
	}
	return d
}

func (g *Gate) dailyCapReached(now time.Time) bool {
	if g.cfg.PerDay <= 0 {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	key := now.Format("2006-01-02")
	if key != g.dayKey {
		g.dayKey = key
		g.dayCount = 0
	}
	return g.dayCount >= g.cfg.PerDay
}

func (g *Gate) recordSend(toJID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	g.lastSend = now
	g.lastByContact[toJID] = now
	key := now.Format("2006-01-02")
	if key != g.dayKey {
		g.dayKey = key
		g.dayCount = 0
	}
	g.dayCount++
}

// inQuietHours reports whether hour h falls in [start,end), handling midnight
// wrap (e.g. 23..7). start==end disables quiet hours.
func inQuietHours(h, start, end int) bool {
	if start == end {
		return false
	}
	if start < end {
		return h >= start && h < end
	}
	return h >= start || h < end
}
