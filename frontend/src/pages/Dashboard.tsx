import { useCallback, useEffect, useState } from "react";
import { NavLink } from "react-router";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  ActivityService,
  ApprovalsService,
  ContactsService,
  ControlService,
  Contact,
  Draft,
  PendingContact,
} from "@/lib/api";
import { ScheduleService } from "@/lib/api";
import { ContactDialog, TodayContextDialog } from "@/components/dialogs";
import { StatCards } from "@/components/StatCards";
import {
  Check,
  CheckCircle2,
  MessageSquare,
  NotebookPen,
  Send,
  UserPlus,
  X,
} from "lucide-react";

function numberOf(jid: string): string {
  const n = jid.split("@")[0].split(":")[0];
  return n ? `+${n}` : jid;
}

export function Dashboard() {
  const [botStatus, setBotStatus] = useState(() => {
    ControlService.Status().then((s) => setBotStatus((b) => ({ ...b, status: s }))).catch(() => setBotStatus((b) => ({ ...b, status: "unavailable" })));
    ControlService.Paused().then((p) => setBotStatus((b) => ({ ...b, paused: p }))).catch(() => {});
    return { status: "Loading…", paused: false };
  });
  const [counts, setCounts] = useState({ pending: 0, today: 0 });
  const [data, setData] = useState({
    drafts: [] as Draft[],
    pendingNew: [] as PendingContact[],
    recent: [] as { id: number; kind: string; summary: string; ts: string }[],
  });
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [schedules, setSchedules] = useState<{ id: string; enabled: boolean }[]>([]);
  const [ui, setUi] = useState({ saving: null as Contact | null, todayOpen: false });
  const [todayContext, setTodayContext] = useState("");

  const loadActions = useCallback(() => {
    ApprovalsService.List()
      .then((d) => {
        setData((a) => ({ ...a, drafts: d ?? [] }));
        setCounts((c) => ({ ...c, pending: (d ?? []).length }));
      })
      .catch(() => {});
    ContactsService.PendingNew()
      .then((p) => setData((a) => ({ ...a, pendingNew: p ?? [] })))
      .catch(() => {});
    ContactsService.List()
      .then((c) => setContacts(c ?? []))
      .catch(() => {});
    ScheduleService.List()
      .then((s) => setSchedules((s ?? []).map((t) => ({ id: t.id, enabled: t.enabled }))))
      .catch(() => {});
    ControlService.Today().then(setTodayContext).catch(() => {});
  }, []);

  // Recent activity widget: just the 5 most rows. Also refreshes the "today"
  // count so the stat stays in sync with the same fetch cadence.
  const loadRecent = useCallback(() => {
    ActivityService.List(5)
      .then((a) =>
        setData((d) => ({
          ...d,
          recent: (a ?? []).map((x) => ({ id: x.id, kind: x.kind, summary: x.summary, ts: x.ts })),
        })),
      )
      .catch(() => {});
    ActivityService.CountToday()
      .then((n) => setCounts((c) => ({ ...c, today: n })))
      .catch(() => {});
  }, []);

  useEffect(() => {
    // Live updates: new unknown contact → refresh actions; new activity row →
    // refresh the recent feed and today count.
    const offUnknown = Events.On("unknown", () => loadActions());
    const offActivity = Events.On("activity", () => loadRecent());
    const offDrafts = Events.On("drafts", () => loadActions());
    return () => {
      offUnknown();
      offActivity();
      offDrafts();
    };
  }, [loadActions, loadRecent]);

  const approveDraft = (id: number) =>
    ApprovalsService.Approve(id, "")
      .then(() => {
        toast.success("Reply approved & sent");
        loadActions();
      })
      .catch((e) => toast.error("Couldn't approve", { description: String(e) }));

  const rejectDraft = (id: number) =>
    ApprovalsService.Reject(id)
      .then(loadActions)
      .catch((e) => toast.error("Couldn't reject", { description: String(e) }));

  const ignoreNew = (jid: string) =>
    ContactsService.DismissNew(jid)
      .then(loadActions)
      .catch(() => {});

  const actionsCount = data.drafts.length + data.pendingNew.length;
  const { status, paused } = botStatus;
  const { pending, today } = counts;
  const { drafts, pendingNew, recent } = data;
  const { saving, todayOpen } = ui;

  const autoCount = contacts.filter((c) => c.tier === "auto").length;
  const draftCount = contacts.filter((c) => c.tier === "draft").length;
  const activeSchedules = schedules.filter((s) => s.enabled).length;

  const kindIcon: Record<string, typeof Send> = {
    sent: Send,
    draft: MessageSquare,
    proactive: CheckCircle2,
  };

  return (
    <Page
      title="Dashboard"
      description="Status, today's context, and recent activity."
      actions={
        <Button variant="outline" size="sm" onClick={() => setUi((u) => ({ ...u, todayOpen: true }))}>
          <NotebookPen className="size-4" /> Today's context
        </Button>
      }
    >
      <div className="grid gap-4">
        {/* Status + metrics row */}
        <StatCards
          items={[
            {
              label: "Status",
              value: paused ? "Paused" : "Active",
              hint: status,
              tone: paused ? "negative" : "positive",
            },
            {
              label: "Pending approvals",
              value: pending,
              hint: pending > 0 ? "needs review" : "clear",
              tone: pending > 0 ? "negative" : "positive",
            },
            {
              label: "Contacts",
              value: contacts.length,
              hint: contacts.length > 0 ? `${autoCount} auto, ${draftCount} draft` : undefined,
            },
            {
              label: "Activity today",
              value: today,
              hint: activeSchedules > 0 ? `${activeSchedules} schedule${activeSchedules > 1 ? "s" : ""} active` : undefined,
            },
          ]}
        />

        {/* Today's context */}
        {todayContext && (
          <Card>
            <CardContent className="pt-4">
              <p className="text-sm whitespace-pre-wrap">{todayContext}</p>
            </CardContent>
          </Card>
        )}

        {/* Actions needed + Recent activity, side by side on wide screens */}
        <div className="grid gap-4 lg:grid-cols-2">
        {/* Actions needed */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Actions needed</CardTitle>
          </CardHeader>
          <CardContent>
            {actionsCount === 0 ? (
              <p className="text-sm text-muted-foreground">You're all caught up 🎉</p>
            ) : (
              <div className="divide-y">
                {drafts.map((d) => (
                  <div key={`draft-${d.id}`} className="flex items-center gap-3 py-3 first:pt-0">
                    <MessageSquare className="size-4 shrink-0 text-blue-600" />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">
                        Reply to {d.senderName || numberOf(d.chatJid)}
                      </p>
                      <p className="truncate text-xs text-muted-foreground">{d.reply}</p>
                    </div>
                    <Button size="sm" variant="outline" onClick={() => approveDraft(d.id)}>
                      <Check className="size-4" /> Approve
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => rejectDraft(d.id)}>
                      <X className="size-4" />
                    </Button>
                  </div>
                ))}
                {pendingNew.map((p) => (
                  <div key={`new-${p.jid}`} className="flex items-center gap-3 py-3 first:pt-0">
                    <UserPlus className="size-4 shrink-0 text-amber-600" />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">
                        New number {p.name || numberOf(p.jid)}
                      </p>
                      <p className="truncate text-xs text-muted-foreground">
                        {p.preview || "Not in your contacts"}
                      </p>
                    </div>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        setUi((u) => ({
                          ...u,
                          saving: new Contact({
                            jid: p.jid,
                            name: p.name,
                            tier: "auto" as Contact["tier"],
                          }),
                        }))
                      }
                    >
                      <Check className="size-4" /> Save
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => ignoreNew(p.jid)}>
                      <X className="size-4" />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Recent activity */}
        <Card>
          <CardHeader className="flex-row items-center justify-between">
            <CardTitle className="text-base">Recent activity</CardTitle>
            <NavLink
              to="/activity"
              className="text-xs text-muted-foreground hover:underline"
            >
              View all
            </NavLink>
          </CardHeader>
          <CardContent>
            {recent.length === 0 ? (
              <p className="text-sm text-muted-foreground">Nothing yet.</p>
            ) : (
              <div className="divide-y">
                {recent.map((a) => {
                  const Icon = kindIcon[a.kind] ?? MessageSquare;
                  return (
                    <div
                      key={a.id}
                      className="flex items-center gap-3 py-2.5 first:pt-0 last:pb-0 text-sm"
                    >
                      <Icon className="size-4 shrink-0 text-muted-foreground" />
                      <Badge variant="outline" className="shrink-0">
                        {a.kind}
                      </Badge>
                      <span className="flex-1 truncate text-muted-foreground">
                        {a.summary}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
        </div>
      </div>

      <ContactDialog
        contact={saving}
        jidEditable={false}
        onClose={() => setUi((u) => ({ ...u, saving: null }))}
        onSaved={loadActions}
      />

      <TodayContextDialog open={todayOpen} onOpenChange={(o) => setUi((u) => ({ ...u, todayOpen: o }))} />
    </Page>
  );
}
