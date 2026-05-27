import { useEffect, useMemo, useState } from "react";
import { Page } from "@/components/Page";
import { Badge } from "@/components/ui/badge";
import { DataTable, type Column } from "@/components/DataTable";
import { ActivityService, Activity as ActivityItem } from "@/lib/api";
import {
  MessageSquare,
  Phone,
  AlertTriangle,
  Send,
  Zap,
  Settings2,
  Terminal,
  EyeOff,
} from "lucide-react";
import { cn } from "@/lib/utils";

const KIND_META: Record<string, { label: string; icon: typeof Send; color: string }> = {
  sent: { label: "Sent", icon: Send, color: "text-green-600" },
  draft: { label: "Draft", icon: MessageSquare, color: "text-blue-600" },
  call: { label: "Call", icon: Phone, color: "text-purple-600" },
  flag: { label: "Flag", icon: AlertTriangle, color: "text-amber-600" },
  proactive: { label: "Proactive", icon: Zap, color: "text-cyan-600" },
  command: { label: "Command", icon: Terminal, color: "text-indigo-600" },
  silent: { label: "Silent", icon: EyeOff, color: "text-muted-foreground" },
  system: { label: "System", icon: Settings2, color: "text-muted-foreground" },
};

const KINDS = Object.keys(KIND_META);

function relTime(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "";
  const diff = Date.now() - d.getTime();
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

function fullTime(ts: string): string {
  const d = new Date(ts);
  return isNaN(d.getTime()) ? "" : d.toLocaleString();
}

function shortJid(jid: string): string {
  const num = jid.split("@")[0].split(":")[0];
  return num ? `+${num}` : jid;
}

export function Activity() {
  const [items, setItems] = useState<ActivityItem[]>([]);
  const [filter, setFilter] = useState("all");
  const [search, setSearch] = useState("");

  useEffect(() => {
    ActivityService.List(200)
      .then((a) => setItems(a ?? []))
      .catch(() => {});
  }, []);

  const counts = useMemo(() => {
    const m: Record<string, number> = {};
    for (const a of items) m[a.kind] = (m[a.kind] ?? 0) + 1;
    return m;
  }, [items]);

  const filterOptions = useMemo(
    () => [
      { value: "all", label: "All types" },
      ...KINDS.filter((k) => counts[k]).map((k) => ({
        value: k,
        label: `${KIND_META[k].label} (${counts[k]})`,
      })),
    ],
    [counts],
  );

  const rows = useMemo(() => {
    let out = items;
    if (filter !== "all") out = out.filter((a) => a.kind === filter);
    const q = search.trim().toLowerCase();
    if (q) {
      out = out.filter(
        (a) => a.summary.toLowerCase().includes(q) || shortJid(a.chatJid).includes(q),
      );
    }
    return out;
  }, [items, filter, search]);

  const columns = useMemo<Column<ActivityItem>[]>(
    () => [
      {
        id: "type",
        header: "Type",
        className: "w-32",
        cell: (a) => {
          const meta = KIND_META[a.kind] ?? KIND_META.system;
          const Icon = meta.icon;
          return (
            <Badge variant="outline" className="gap-1.5">
              <Icon className={cn("size-3", meta.color)} />
              {meta.label}
            </Badge>
          );
        },
      },
      {
        id: "summary",
        header: "Activity",
        className: "max-w-0 truncate",
        cell: (a) => a.summary,
      },
      {
        id: "contact",
        header: "Contact",
        className: "whitespace-nowrap text-muted-foreground",
        cell: (a) => shortJid(a.chatJid),
      },
      {
        id: "time",
        header: "Time",
        headClassName: "text-right",
        className: "whitespace-nowrap text-right text-muted-foreground",
        cell: (a) => <span title={fullTime(a.ts)}>{relTime(a.ts)}</span>,
      },
    ],
    [],
  );

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
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(a) => String(a.id)}
        search={{ value: search, onChange: setSearch, placeholder: "Search activity…" }}
        filter={{ value: filter, onChange: setFilter, label: "All types", options: filterOptions }}
        empty="No matching activity."
      />
    </Page>
  );
}
