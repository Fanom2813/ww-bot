import { useEffect, useState } from "react";
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
import { ActivityService, ApprovalsService, ControlService } from "@/lib/api";

export function Dashboard() {
  const [status, setStatus] = useState("Loading…");
  const [pending, setPending] = useState(0);
  const [today, setToday] = useState("");
  const [recent, setRecent] = useState<{ kind: string; summary: string }[]>([]);

  useEffect(() => {
    ControlService.Status().then(setStatus).catch(() => setStatus("unavailable"));
    ApprovalsService.List().then((d) => setPending((d ?? []).length)).catch(() => {});
    ActivityService.List(8)
      .then((a) => setRecent((a ?? []).map((x) => ({ kind: x.kind, summary: x.summary }))))
      .catch(() => {});
  }, []);

  const saveToday = () => {
    ControlService.SetToday(today)
      .then(() => toast("Today's context saved"))
      .catch((e) => toast.error("Couldn't save", { description: String(e) }));
  };

  return (
    <Page title="Dashboard" description="Status, today's context, and recent activity.">
      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Status</CardTitle>
          </CardHeader>
          <CardContent className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">{status}</span>
            <Badge variant={pending > 0 ? "default" : "secondary"}>
              {pending} pending
            </Badge>
          </CardContent>
        </Card>

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

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Recent activity</CardTitle>
          </CardHeader>
          <CardContent>
            {recent.length === 0 ? (
              <p className="text-sm text-muted-foreground">Nothing yet.</p>
            ) : (
              <ul className="space-y-2">
                {recent.map((a, i) => (
                  <li key={i} className="flex items-center gap-2 text-sm">
                    <Badge variant="outline" className="shrink-0">
                      {a.kind}
                    </Badge>
                    <span className="truncate text-muted-foreground">{a.summary}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </Page>
  );
}
