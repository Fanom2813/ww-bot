import { useEffect, useState } from "react";
import { NavLink } from "react-router";
import { toast } from "sonner";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  ActivityService,
  ApprovalsService,
  ControlService,
} from "@/lib/api";
import {
  CheckCircle2,
  Clock,
  Inbox,
  MessageSquare,
  Send,
} from "lucide-react";
import { cn } from "@/lib/utils";

export function Dashboard() {
  const [status, setStatus] = useState("Loading…");
  const [paused, setPaused] = useState(false);
  const [pending, setPending] = useState(0);
  const [todayCount, setTodayCount] = useState(0);
  const [today, setToday] = useState("");
  const [recent, setRecent] = useState<
    { id: number; kind: string; summary: string; ts: string }[]
  >([]);

  useEffect(() => {
    ControlService.Status().then(setStatus).catch(() => setStatus("unavailable"));
    ControlService.Paused().then(setPaused).catch(() => {});
    ApprovalsService.List().then((d) => setPending((d ?? []).length)).catch(() => {});
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
  }, []);

  const saveToday = () => {
    ControlService.SetToday(today)
      .then(() => toast("Today's context saved"))
      .catch((e) => toast.error("Couldn't save", { description: String(e) }));
  };

  const kindIcon: Record<string, typeof Send> = {
    sent: Send,
    draft: MessageSquare,
    proactive: CheckCircle2,
  };

  return (
    <Page title="Dashboard" description="Status, today's context, and recent activity.">
      <div className="grid gap-4">
        {/* Status + metrics row */}
        <div className="grid gap-4 sm:grid-cols-3">
          <Card>
            <CardContent className="flex items-center gap-3 py-4">
              <div
                className={cn(
                  "flex h-10 w-10 items-center justify-center rounded-full",
                  paused
                    ? "bg-amber-500/15 text-amber-600"
                    : "bg-green-500/15 text-green-600",
                )}
              >
                {paused ? (
                  <Clock className="h-5 w-5" />
                ) : (
                  <CheckCircle2 className="h-5 w-5" />
                )}
              </div>
              <div>
                <p className="text-sm font-medium">{paused ? "Paused" : "Active"}</p>
                <p className="text-xs text-muted-foreground">{status}</p>
              </div>
            </CardContent>
          </Card>

          <NavLink to="/approvals" className="group">
            <Card className="transition-colors group-hover:border-primary/40">
              <CardContent className="flex items-center gap-3 py-4">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-blue-500/15 text-blue-600">
                  <Inbox className="h-5 w-5" />
                </div>
                <div>
                  <p className="text-sm font-medium">
                    {pending} pending approval{pending !== 1 ? "s" : ""}
                  </p>
                  <p className="text-xs text-muted-foreground">Review draft replies</p>
                </div>
              </CardContent>
            </Card>
          </NavLink>

          <Card>
            <CardContent className="flex items-center gap-3 py-4">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-purple-500/15 text-purple-600">
                <Send className="h-5 w-5" />
              </div>
              <div>
                <p className="text-sm font-medium">
                  {todayCount} action{todayCount !== 1 ? "s" : ""} today
                </p>
                <p className="text-xs text-muted-foreground">Bot activity since midnight</p>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Today's context */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Today's context</CardTitle>
            <CardDescription>
              Tell the bot what you're doing today so it answers people accurately.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Textarea
              placeholder="e.g. Deep work all morning, free after 3pm, traveling this evening…"
              value={today}
              onChange={(e) => setToday(e.target.value)}
              rows={3}
            />
            <Button onClick={saveToday} disabled={!today.trim()}>
              Save
            </Button>
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
    </Page>
  );
}
