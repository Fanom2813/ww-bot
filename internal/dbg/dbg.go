// Package dbg is a tiny conditional logger for chatty, informational traces —
// inbound message flow, brain decisions, send tracking, scheduled task firing,
// etc. These lines are useful during development but become noise in a
// packaged build, so they're gated behind the WWBOT_DEBUG environment
// variable.
//
// Real errors and warnings should keep using the standard `log` package so
// they're always visible.
//
// Usage:
//
//	dbg.Printf("[core] decided action=%s", action)  // visible only if WWBOT_DEBUG is set
//
// Enable at runtime by exporting WWBOT_DEBUG=1 (any non-empty value) before
// launching the app.
package dbg

import (
	"log"
	"os"
	"runtime/debug"
	"strings"
)

// Recover should be deferred at the top of any goroutine. If the goroutine
// panics, the panic value and stack trace are written to the standard log
// (which the app tees to ww-bot.log) instead of crashing the process
// silently. Always-on: unrelated to WWBOT_DEBUG.
func Recover(where string) {
	if r := recover(); r != nil {
		log.Printf("[panic] %s: %v\n%s", where, r, debug.Stack())
	}
}

// enabled is resolved once at startup from WWBOT_DEBUG. Values "0", "false",
// "off", and "" disable; anything else enables.
var enabled = func() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("WWBOT_DEBUG")))
	switch v {
	case "", "0", "false", "off", "no":
		return false
	}
	return true
}()

// Enabled reports whether verbose debug logging is on. Useful when building a
// debug-only message is itself expensive (e.g. JSON-marshalling).
func Enabled() bool { return enabled }

// Printf logs to the standard logger only when WWBOT_DEBUG is set.
func Printf(format string, args ...any) {
	if enabled {
		log.Printf(format, args...)
	}
}
