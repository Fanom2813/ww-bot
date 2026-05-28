package main

import (
	"log"
	"strconv"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/services/dock"

	"wwbot/internal/core"
)

// wireDockBadge keeps the dock/taskbar badge in sync with how many things in
// the app actually need the user's attention: pending drafts + unsaved-number
// prompts. It subscribes to the same multi-listener notifiers that drive the
// frontend, so the badge updates instantly on every create/approve/reject/save.
// Returns a no-arg syncFn so callers can trigger an initial sync once the run
// loop is up (the dock API needs NSApp running on macOS).
func wireDockBadge(cr *core.Core, dk *dock.DockService) (syncFn func()) {
	var (
		mu   sync.Mutex
		last string // remember the rendered label to skip redundant native calls
	)

	apply := func() {
		drafts, _ := cr.ListDrafts()
		pending := cr.PendingContacts()
		n := len(drafts) + len(pending)

		label := ""
		if n > 0 {
			// Cap at "99+" so the badge stays readable on macOS.
			if n > 99 {
				label = "99+"
			} else {
				label = strconv.Itoa(n)
			}
		}

		mu.Lock()
		if label == last {
			mu.Unlock()
			return
		}
		last = label
		mu.Unlock()

		var err error
		if label == "" {
			err = dk.RemoveBadge()
		} else {
			err = dk.SetBadge(label)
		}
		if err != nil {
			log.Printf("dock badge update failed: %v", err)
		}
	}

	cr.OnDraftQueue(apply)
	cr.OnUnknownContact(func(_, _, _ string) { apply() })

	// Caller schedules apply() from ApplicationStarted — see main.go.
	return apply
}
