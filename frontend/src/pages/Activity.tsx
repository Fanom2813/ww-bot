import { useEffect, useMemo, useState } from "react";
import { Page } from "@/components/Page";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ActivityService, Activity as ActivityItem } from "@/lib/api";
import {
  MessageSquare,
  Phone,
  AlertTriangle,
  Send,
  Zap,
  Settings2,
  Search,
} from "lucide-react";
import { cn } from "@/lib/utils";

const KIND_META: Record<string, { label: string; icon: typeof Send; color: string }> = {
  sent: { label: "Sent", icon: Send, color: "text-green-600" },
  draft: { label: "Draft", icon: MessageSquare, color: "text-blue-600" },
  call: { label: "Call", icon: Phone, color: "text-purple-600" },
  flag: { label: "Flag", icon: AlertTriangle, color: "text-amber-600" },
  proactive: { label: "Proactive", icon: Zap, color: "text-cyan-600" },
  system: { label: "System", icon: Settings2, color: "text-muted-foreground" },
};

const KINDS = Object.keys(KIND_META);

function relTime(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "";
  const now = Date.now();
  const diff = now - d.getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days === 1) return "yesterday";
  if (days < 7) return `${days}d ago`;
  return d.toLocaleDateString();
}

function dateGroup(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "Earlier";
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 86400000);
  const itemDay = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  if (+itemDay === +today) return "Today";
  if (+itemDay === +yesterday) return "Yesterday";
  return "Earlier";
}

function shortJid(jid: string): string {
  const num = jid.split("@")[0].split(":")[0];
  return num ? `+${num}` : jid;
}

export function Activity() {
  const [items, setItems] = useState<ActivityItem[]>([]);
  const [filter, setFilter] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  useEffect(() => {
    ActivityService.List(200)
      .then((a) => setItems(a ?? []))
      .catch(() => {});
  }, []);

  const filtered = useMemo(() => {
    let out = items;
    if (filter) out = out.filter((a) => a.kind === filter);
    if (search.trim()) {
      const q = search.toLowerCase();
      out = out.filter(
        (a) =>
          a.summary.toLowerCase().includes(q) ||
          shortJid(a.chatJid).includes(q),
      );
    }
    return out;
  }, [items, filter, search]);

  const grouped = useMemo(() => {
    const map = new Map<string, ActivityItem[]>();
    for (const item of filtered) {
      const g = dateGroup(item.ts);
      const arr = map.get(g) ?? [];
      arr.push(item);
      map.set(g, arr);
    }
    const order = ["Today", "Yesterday", "Earlier"];
    return order
      .filter((g) => map.has(g))
      .map((g) => ({ label: g, items: map.get(g)! }));
  }, [filtered]);

  const counts = useMemo(() => {
    const m: Record<string, number> = {};
    for (const a of items) m[a.kind] = (m[a.kind] ?? 0) + 1;
    return m;
  }, [items]);

  if (items.length === 0) {
    return (
      <Page title="Activity" description="What the bot did — replies, drafts, calls, and flags.">
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <Zap className="h-6 w-6 text-muted-foreground" />
          </div>
          <p className="font-medium">No activity yet</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Once the bot starts handling messages, actions will appear here.
          </p>
        </div>
      </Page>
    );
  }

  return (
    <Page title="Activity" description="What the bot did — replies, drafts, calls, and flags.">
      {/* Filter bar */}
      <Card className="mb-4">
        <CardContent className="flex flex-wrap items-center gap-2 py-3">
          <div className="relative flex-1 min-w-[180px]">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search activity…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-8"
            />
          </div>
          <div className="flex flex-wrap gap-1">
            <Button
              size="sm"
              variant={filter === null ? "default" : "outline"}
              onClick={() => setFilter(null)}
            >
              All
            </Button>
            {KINDS.filter((k) => counts[k]).map((k) => {
              const meta = KIND_META[k];
              return (
                <Button
                  key={k}
                  size="sm"
                  variant={filter === k ? "default" : "outline"}
                  onClick={() => setFilter(filter === k ? null : k)}
                  className="gap-1"
                >
                  <meta.icon className="h-3 w-3" />
                  {meta.label}
                  <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-[10px]">
                    {counts[k]}
                  </Badge>
                </Button>
              );
            })}
          </div>
        </CardContent>
      </Card>

      {/* Grouped timeline */}
      {grouped.length === 0 ? (
        <div className="rounded-lg border border-dashed py-10 text-center text-sm text-muted-foreground">
          No matching activity.
        </div>
      ) : (
        <div className="space-y-6">
          {grouped.map(({ label, items: groupItems }) => (
            <section key={label}>
              <h2 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {label}
              </h2>
              <Card>
                <div className="divide-y">
                  {groupItems.map((a) => {
                    const meta = KIND_META[a.kind] ?? KIND_META.system;
                    const Icon = meta.icon;
                    return (
                      <div
                        key={a.id}
                        className="flex items-center gap-3 px-4 py-3 text-sm"
                      >
                        <Icon className={cn("h-4 w-4 shrink-0", meta.color)} />
                        <Badge variant="outline" className="shrink-0">
                          {meta.label}
                        </Badge>
                        <span className="flex-1 truncate">{a.summary}</span>
                        <span className="shrink-0 text-xs text-muted-foreground">
                          {shortJid(a.chatJid)}
                        </span>
                        <span className="shrink-0 text-xs text-muted-foreground">
                          {relTime(a.ts)}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </Card>
            </section>
          ))}
        </div>
      )}
    </Page>
  );
}
