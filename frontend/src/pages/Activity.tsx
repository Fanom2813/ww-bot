import { useEffect, useState } from "react";
import { Page, ComingSoon } from "@/components/Page";
import { Badge } from "@/components/ui/badge";
import { ActivityService, Activity as ActivityItem } from "@/lib/api";

function when(ts: unknown): string {
  const d = new Date(ts as string);
  return isNaN(d.getTime()) ? "" : d.toLocaleString();
}

export function Activity() {
  const [items, setItems] = useState<ActivityItem[]>([]);

  useEffect(() => {
    ActivityService.List(100)
      .then((a) => setItems(a ?? []))
      .catch(() => {});
  }, []);

  return (
    <Page
      title="Activity"
      description="What the bot did — replies sent, drafts, calls, and flags."
    >
      {items.length === 0 ? (
        <ComingSoon>No activity yet.</ComingSoon>
      ) : (
        <div className="divide-y rounded-lg border">
          {items.map((a) => (
            <div key={a.id} className="flex items-center gap-3 p-3 text-sm">
              <Badge variant="outline" className="shrink-0">
                {a.kind}
              </Badge>
              <span className="flex-1 truncate">{a.summary}</span>
              <span className="shrink-0 text-xs text-muted-foreground">{when(a.ts)}</span>
            </div>
          ))}
        </div>
      )}
    </Page>
  );
}
