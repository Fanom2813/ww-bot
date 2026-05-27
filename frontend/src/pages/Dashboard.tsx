import { useEffect, useState } from "react";
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
  const [status, setStatus] = useState("Loading…");
  const [paused, setPaused] = useState(false);
  const [pending, setPending] = useState(0);
  const [todayCount, setTodayCount] = useState(0);
  const [recent, setRecent] = useState<
    { id: number; kind: string; summary: string; ts: string }[]
  >([]);
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [pendingNew, setPendingNew] = useState<PendingContact[]>([]);
  const [saving, setSaving] = useState<Contact | null>(null);

  const loadActions = () => {
    ApprovalsService.List()
      .then((d) => {
        setDrafts(d ?? []);
        setPending((d ?? []).length);
      })
      .catch(() => {});
    ContactsService.PendingNew()
      .then((p) => setPendingNew(p ?? []))
      .catch(() => {});
  };

  useEffect(() => {
    ControlService.Status().then(setStatus).catch(() => setStatus("unavailable"));
    ControlService.Paused().then(setPaused).catch(() => {});
    loadActions();
    ActivityService.List(24)
      .then((a) => {
        setRecent((a ?? []).map((x) => ({ id: x.id, kind: x.kind, summary: x.summary, ts: x.ts })));
        const todayStart = new Date();
        todayStart.setHours(0, 0, 0, 0);
        setTodayCount(
          (a ?? []).filter((x) => new Date(x.ts) >= todayStart).length,
        );
      })
      .catch(() => {});

    // Live-refresh the actions list when a new number comes in.
    const off = Events.On("unknown", () => loadActions());
    return () => off();
  }, []);

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

  const actionsCount = drafts.length + pendingNew.length;

  const [todayOpen, setTodayOpen] = useState(false);

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
        <Button variant="outline" size="sm" onClick={() => setTodayOpen(true)}>
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
            { label: "Actions today", value: todayCount, hint: "since midnight" },
          ]}
        />

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
                        setSaving(
                          new Contact({
                            jid: p.jid,
                            name: p.name,
                            tier: "auto" as Contact["tier"],
                          }),
                        )
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
                      <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
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
        onClose={() => setSaving(null)}
        onSaved={loadActions}
      />

      <TodayContextDialog open={todayOpen} onOpenChange={setTodayOpen} />
    </Page>
  );
}
