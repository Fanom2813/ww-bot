package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// setupFileLogging mirrors stdlib log output to ww-bot.log in the app data
// dir. With -H windowsgui the OS gives us no console, so log.Fatal failures
// are otherwise invisible. Best-effort: silently skip if the dir is unavailable.
func setupFileLogging() {
	d, err := appDir()
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(d, "ww-bot.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

// appDir returns (and creates) the per-user data directory, e.g.
// ~/Library/Application Support/ww-bot on macOS.
func appDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(dir, "ww-bot")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

// sessionDSN is the SQLite DSN for the encrypted WhatsApp session store.
// Uses modernc.org/sqlite _pragma= syntax (mattn-style params are ignored).
func sessionDSN() string {
	const params = "?_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)"
	d, err := appDir()
	if err != nil {
		return "file:wwbot-session.db" + params
	}
	return "file:" + filepath.Join(d, "session.db") + params
}

// dataDSN is the SQLite DSN for the app data store (contacts, drafts, etc.).
func dataDSN() string {
	const params = "?_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	d, err := appDir()
	if err != nil {
		return "file:wwbot-data.db" + params
	}
	return "file:" + filepath.Join(d, "data.db") + params
}
