import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Page } from "@/components/Page";
import { DataTable, type Column } from "@/components/DataTable";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Group, GroupsService, WAGroup, WhatsAppService } from "@/lib/api";

type Mode = "off" | "mentions" | "always";

const MODE_LABEL: Record<Mode, string> = {
  off: "Silent",
  mentions: "When mentioned",
  always: "Always reply",
};

/* Row merges a WhatsApp group (live, from the address book) with the saved
 * opt-in row (if any). Groups never persisted yet appear in "off" mode. */
type Row = {
  jid: string;
  name: string;
  participants: number;
  mode: Mode;
};

function normalizeMode(m: string | undefined): Mode {
  return m === "always" || m === "mentions" ? m : "off";
}

export function Groups() {
  const [rows, setRows] = useState<Row[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);

  const load = useCallback(() => {
    setLoading(true);
    Promise.all([
      WhatsAppService.Groups().catch(() => [] as WAGroup[]),
      GroupsService.List().catch(() => [] as Group[]),
    ])
      .then(([waGroups, saved]) => {
        const savedByJid = new Map<string, Group>((saved ?? []).map((g) => [g.jid, g]));
        const merged: Row[] = (waGroups ?? []).map((g) => {
          const s = savedByJid.get(g.jid);
          return {
            jid: g.jid,
            name: g.name || g.jid,
            participants: g.participants,
            mode: normalizeMode(s?.mode),
          };
        });
        // Include saved groups the user is no longer in so a stale opt-in can
        // be flipped off without rejoining.
        for (const s of saved ?? []) {
          if (!merged.some((r) => r.jid === s.jid)) {
            merged.push({
              jid: s.jid,
              name: s.name || s.jid,
              participants: 0,
              mode: normalizeMode(s.mode),
            });
          }
        }
        merged.sort((a, b) => a.name.localeCompare(b.name));
        setRows(merged);
      })
      .finally(() => setLoading(false));
  }, []);

  useEffect(load, [load]);

  const setMode = useCallback((r: Row, mode: Mode) => {
    // Optimistic update so the select reflects instantly.
    setRows((prev) => prev.map((x) => (x.jid === r.jid ? { ...x, mode } : x)));
    GroupsService.Save({ jid: r.jid, name: r.name, mode } as Group)
      .then(() => toast.success(`"${r.name}" → ${MODE_LABEL[mode]}`))
      .catch((e) => {
        toast.error("Save failed", { description: String(e) });
        load();
      });
  }, [load]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter((r) => r.name.toLowerCase().includes(q) || r.jid.toLowerCase().includes(q));
  }, [rows, search]);

  const activeCount = rows.filter((r) => r.mode !== "off").length;

  const columns = useMemo<Column<Row>[]>(
    () => [
      { id: "name", header: "Group", className: "font-medium", cell: (r) => r.name },
      {
        id: "participants",
        header: "Members",
        className: "text-muted-foreground tabular-nums",
        cell: (r) => (r.participants > 0 ? r.participants : "—"),
      },
      {
        id: "status",
        header: "Status",
        cell: (r) => {
          if (r.mode === "always") return <Badge>Always replies</Badge>;
          if (r.mode === "mentions")
            return <Badge variant="secondary">Replies when mentioned</Badge>;
          return (
            <Badge variant="outline" className="text-muted-foreground">
              Silent
            </Badge>
          );
        },
      },
      {
        id: "mode",
        header: "Mode",
        headClassName: "text-right",
        className: "text-right",
        cell: (r) => (
          <Select value={r.mode} onValueChange={(v) => setMode(r, v as Mode)}>
            <SelectTrigger size="sm" className="ml-auto w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent align="end">
              <SelectItem value="off">{MODE_LABEL.off}</SelectItem>
              <SelectItem value="mentions">{MODE_LABEL.mentions}</SelectItem>
              <SelectItem value="always">{MODE_LABEL.always}</SelectItem>
            </SelectContent>
          </Select>
        ),
      },
    ],
    [setMode],
  );

  return (
    <Page
      fill
      title="Groups"
      description={`Choose which group chats the bot may participate in. ${activeCount} of ${rows.length} active.`}
    >
      <DataTable
        columns={columns}
        rows={filtered}
        rowKey={(r) => r.jid}
        loading={loading}
        search={{ value: search, onChange: setSearch, placeholder: "Search groups…" }}
        empty="No groups found. Once your phone syncs, the groups you're in will appear here."
      />
    </Page>
  );
}
