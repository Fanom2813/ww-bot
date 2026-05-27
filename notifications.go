package main

import (
	"log"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"wwbot/internal/core"
	"wwbot/internal/store"
)

// UnknownContact is pushed to the frontend on the "unknown" event when an
// unsaved number messages the user, so the UI can prompt to save them.
type UnknownContact struct {
	JID     string `json:"jid"`
	Name    string `json:"name"`
	Preview string `json:"preview"`
}

const newContactCategory = "new-contact"

// wireNotifications connects the core's unknown-contact prompt to both an in-app
// event (toast + save dialog) and a native OS notification with an inline "Save"
// action that captures the contact's name. Native notifications require a
// signed, bundled app; they no-op (with a log line) in unsigned dev builds.
func wireNotifications(cr *core.Core, ns *notifications.NotificationService) {
	// A "Save" reply field (for the name) plus an "Ignore" button.
	if err := ns.RegisterNotificationCategory(notifications.NotificationCategory{
		ID:               newContactCategory,
		Actions:          []notifications.NotificationAction{{ID: "IGNORE", Title: "Ignore"}},
		HasReplyField:    true,
		ReplyPlaceholder: "Name (optional)",
		ReplyButtonTitle: "Save",
	}); err != nil {
		log.Printf("register notification category: %v", err)
	}

	ns.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Error != nil {
			return
		}
		resp := result.Response
		if resp.ActionIdentifier == "IGNORE" {
			return
		}
		jid, _ := resp.UserInfo["jid"].(string)
		if jid == "" {
			return
		}
		name := resp.UserText
		if name == "" {
			name, _ = resp.UserInfo["name"].(string)
		}
		saveNewContact(cr, jid, name)
	})

	cr.OnUnknownContact(func(jid, name, preview string) {
		// In-app prompt (toast + dialog) for when the window is open.
		if a := application.Get(); a != nil {
			a.Event.Emit("unknown", UnknownContact{JID: jid, Name: name, Preview: preview})
		}
		// Native OS notification so the user is alerted while backgrounded.
		display := name
		if display == "" {
			display = "+" + numberFromJID(jid)
		}
		if err := ns.SendNotificationWithActions(notifications.NotificationOptions{
			ID:         "newcontact-" + jid,
			Title:      "New message from " + display,
			Body:       preview,
			CategoryID: newContactCategory,
			Data:       map[string]any{"jid": jid, "name": name},
		}); err != nil {
			log.Printf("native notification failed: %v", err)
		}
	})
}

// saveNewContact upserts a freshly-discovered number at the auto-reply tier
// (the user's chosen default) and surfaces a confirmation. Used by the native
// notification's Save action; the in-app dialog saves via ContactsService.
func saveNewContact(cr *core.Core, jid, name string) {
	if err := cr.UpsertContact(store.Contact{JID: jid, Name: name, Tier: store.TierAuto}); err != nil {
		log.Printf("save contact %s: %v", jid, err)
		return
	}
	display := name
	if display == "" {
		display = "+" + numberFromJID(jid)
	}
	if a := application.Get(); a != nil {
		a.Event.Emit("notice", Notice{Level: "info", Title: "Contact saved", Body: display})
	}
}

// numberFromJID extracts the phone digits from a JID
// ("123456:7@s.whatsapp.net" → "123456").
func numberFromJID(jid string) string {
	if i := strings.IndexAny(jid, "@:"); i >= 0 {
		return jid[:i]
	}
	return jid
}
