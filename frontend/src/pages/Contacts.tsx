import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DataTable, type Column } from "@/components/DataTable";
import { ContactDialog } from "@/components/dialogs";
import { Contact, ContactsService, WhatsAppService } from "@/lib/api";
import { Plus } from "lucide-react";

type Row = { jid: string; name: string; managed: Contact | null };

const TIER_LABEL: Record<string, string> = {
  auto: "Auto-send",
  draft: "Draft & approve",
  notify: "Notify only",
};
const TIER_VARIANT: Record<string, "default" | "secondary" | "outline"> = {
  auto: "default",
  draft: "secondary",
  notify: "outline",
};

/** numberOf turns "123456:7@s.whatsapp.net" into "+123456". */
function numberOf(jid: string): string {
  const n = jid.split("@")[0].split(":")[0];
  return n ? `+${n}` : jid;
}

type DialogState = { contact: Contact; jidEditable: boolean } | null;

export function Contacts() {
  const [managed, setManaged] = useState<Contact[]>([]);
  const [waContacts, setWaContacts] = useState<{ jid: string; name: string }[]>([]);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<"all" | "added" | "not">("all");
  const [loading, setLoading] = useState(true);
  const [dialog, setDialog] = useState<DialogState>(null);

  const load = useCallback(() => {
    setLoading(true);
    Promise.all([
      ContactsService.List().catch(() => [] as Contact[]),
      WhatsAppService.Contacts().catch(() => [] as { jid: string; name: string }[]),
    ])
      .then(([m, w]) => {
        setManaged(m ?? []);
        setWaContacts(w ?? []);
      })
      .catch((e) => toast.error("Couldn't load contacts", { description: String(e) }))
      .finally(() => setLoading(false));
  }, []);
  useEffect(load, [load]);

  const rows = useMemo<Row[]>(() => {
    const byJid = new Map<string, Contact>();
    managed.forEach((m) => byJid.set(m.jid, m));
    const seen = new Set<string>();
    const out: Row[] = [];
    waContacts.forEach((w) => {
      const m = byJid.get(w.jid) ?? null;
      out.push({ jid: w.jid, name: m?.name || w.name, managed: m });
      seen.add(w.jid);
    });
    // Managed contacts not in the WhatsApp address book (manual adds / saved
    // unknowns) still belong in the list.
    managed.forEach((m) => {
      if (!seen.has(m.jid)) out.push({ jid: m.jid, name: m.name || numberOf(m.jid), managed: m });
    });
    const q = query.trim().toLowerCase();
    const filtered = out.filter((r) => {
      if (filter === "added" && !r.managed) return false;
      if (filter === "not" && r.managed) return false;
      if (q && !(r.name.toLowerCase().includes(q) || r.jid.toLowerCase().includes(q))) return false;
      return true;
    });
    return filtered.sort((a, b) => a.name.localeCompare(b.name));
  }, [managed, waContacts, query, filter]);

  const addManual = () =>
    setDialog({
      contact: new Contact({ jid: "", name: "", tier: "draft" as Contact["tier"] }),
      jidEditable: true,
    });

  const openRow = (r: Row) =>
    setDialog({
      contact: r.managed ?? new Contact({ jid: r.jid, name: r.name, tier: "draft" as Contact["tier"] }),
      jidEditable: false,
    });

  const managedCount = managed.length;

  const columns = useMemo<Column<Row>[]>(
    () => [
      { id: "name", header: "Name", className: "font-medium", cell: (r) => r.name },
      {
        id: "number",
        header: "Number",
        className: "text-muted-foreground",
        cell: (r) => numberOf(r.jid),
      },
      {
        id: "status",
        header: "Status",
        cell: (r) =>
          r.managed ? (
            <Badge variant={TIER_VARIANT[r.managed.tier] ?? "outline"}>
              {TIER_LABEL[r.managed.tier] ?? r.managed.tier}
            </Badge>
          ) : (
            <span className="text-xs text-muted-foreground">Not added</span>
          ),
      },
      {
        id: "action",
        header: "Action",
        headClassName: "text-right",
        className: "text-right",
        cell: (r) => (
          <Button size="sm" variant={r.managed ? "outline" : "default"} onClick={() => openRow(r)}>
            {r.managed ? "Edit" : "Add"}
          </Button>
        ),
      },
    ],
    [],
  );

  return (
    <Page
      title="Contacts"
      description={`Your WhatsApp contacts — add the ones the bot should manage. ${managedCount} managed.`}
      actions={
        <Button onClick={addManual} size="sm">
          <Plus className="size-4" /> Add contact
        </Button>
      }
    >
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(r) => r.jid}
        loading={loading}
        search={{ value: query, onChange: setQuery, placeholder: "Search name or number…" }}
        filter={{
          value: filter,
          onChange: (v) => setFilter(v as typeof filter),
          label: "All",
          options: [
            { value: "all", label: "All" },
            { value: "added", label: "Added" },
            { value: "not", label: "Not added" },
          ],
        }}
        empty={
          query || filter !== "all"
            ? "No contacts match your filters."
            : "No contacts synced yet. Use “Add contact” to add one manually."
        }
      />

      <ContactDialog
        contact={dialog?.contact ?? null}
        jidEditable={dialog?.jidEditable ?? false}
        onClose={() => setDialog(null)}
        onSaved={load}
      />
    </Page>
  );
}
