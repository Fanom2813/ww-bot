package main

import (
	"wwbot/internal/core"
	"wwbot/internal/store"
)

// These thin services expose the core's operations to the frontend with
// type-safe Wails bindings. Each holds the shared *core.Core.

// ContactsService manages per-contact profiles and trust tiers.
type ContactsService struct{ core *core.Core }

func (s *ContactsService) ServiceName() string            { return "Contacts" }
func (s *ContactsService) List() ([]store.Contact, error) { return s.core.ListContacts() }
func (s *ContactsService) Upsert(c store.Contact) error   { return s.core.UpsertContact(c) }

// PendingNew lists unsaved numbers awaiting a save/ignore decision.
func (s *ContactsService) PendingNew() []core.PendingContact { return s.core.PendingContacts() }

// DismissNew drops a pending number without saving it.
func (s *ContactsService) DismissNew(jid string) { s.core.DismissContact(jid) }

// Proactive has the bot reach out to a contact — a free opener (topic == "") or
// one about the given topic.
func (s *ContactsService) Proactive(jid, topic string) error {
	return s.core.StartProactive(jid, topic)
}

// SendDirect sends the given message text to a contact verbatim — no AI. Still
// flows through the safety gate (typing indicator, rate limits, splitting).
func (s *ContactsService) SendDirect(jid, text string) error {
	return s.core.SendDirect(jid, text)
}

// ApprovalsService manages the draft queue.
type ApprovalsService struct{ core *core.Core }

func (s *ApprovalsService) ServiceName() string          { return "Approvals" }
func (s *ApprovalsService) List() ([]store.Draft, error) { return s.core.ListDrafts() }
func (s *ApprovalsService) Approve(id int64, edited string) error {
	return s.core.ApproveDraft(id, edited)
}
func (s *ApprovalsService) Reject(id int64) error { return s.core.RejectDraft(id) }
func (s *ApprovalsService) ApproveAllFromContact(chatJID string) (int, error) {
	return s.core.ApproveDraftsFromContact(chatJID)
}

// SettingsService reads/writes the configuration.
type SettingsService struct{ core *core.Core }

func (s *SettingsService) ServiceName() string         { return "Settings" }
func (s *SettingsService) Get() core.Settings          { return s.core.GetSettings() }
func (s *SettingsService) Save(st core.Settings) error { return s.core.SaveSettings(st) }

// DefaultSystemPrompt returns the built-in persona, for the Settings UI to show
// or restore.
func (s *SettingsService) DefaultSystemPrompt() string { return s.core.DefaultSystemPrompt() }

// DefaultProactivePrompt returns the built-in proactive prompt.
func (s *SettingsService) DefaultProactivePrompt() string { return s.core.DefaultProactivePrompt() }

// ScheduleService manages recurring proactive tasks (scheduled "crons").
type ScheduleService struct{ core *core.Core }

func (s *ScheduleService) ServiceName() string                { return "Schedule" }
func (s *ScheduleService) List() []core.ScheduledTask         { return s.core.Schedules() }
func (s *ScheduleService) Save(ts []core.ScheduledTask) error { return s.core.SaveSchedules(ts) }

// GroupsService manages per-group opt-in. By default the bot is silent in
// every group; the user explicitly enables "always reply" on the groups they
// want the bot to participate in.
type GroupsService struct{ core *core.Core }

func (s *GroupsService) ServiceName() string          { return "Groups" }
func (s *GroupsService) List() ([]store.Group, error) { return s.core.ListGroups() }
func (s *GroupsService) Save(g store.Group) error     { return s.core.SaveGroup(g) }

// ActivityService exposes the audit log.
type ActivityService struct{ core *core.Core }

func (s *ActivityService) ServiceName() string { return "Activity" }
func (s *ActivityService) List(limit int) ([]store.Activity, error) {
	return s.core.ListActivity(limit)
}
func (s *ActivityService) CountToday() (int, error) { return s.core.CountActivityToday() }

// ControlService exposes the kill-switch, status, and daily context.
type ControlService struct{ core *core.Core }

func (s *ControlService) ServiceName() string        { return "Control" }
func (s *ControlService) Pause()                     { s.core.Pause() }
func (s *ControlService) Resume()                    { s.core.Resume() }
func (s *ControlService) Paused() bool               { return s.core.Paused() }
func (s *ControlService) Status() string             { return s.core.Status() }
func (s *ControlService) SetToday(text string) error { return s.core.SetToday(text) }
func (s *ControlService) Today() string              { return s.core.Today() }
