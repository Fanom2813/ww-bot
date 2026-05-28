import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DataTable, type Column } from "@/components/DataTable";
import { ContactDialog, ProactiveDialog, ScheduleDialog } from "@/components/dialogs";
import {
  Contact,
  ContactsService,
  ScheduledTask,
  ScheduleService,
  WhatsAppService,
} from "@/lib/api";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { CalendarClock, MessageCirclePlus, MoreHorizontal, Pencil, Plus, UserPlus } from "lucide-react";

function newId(): string {
  return typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : `sch-${Date.now()}`;
}

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
  const [proactive, setProactive] = useState<{ jid: string; name: string } | null>(null);
  const [scheduleTask, setScheduleTask] = useState<ScheduledTask | null>(null);

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

  const openSchedule = (r: Row) =>
    setScheduleTask(
      new ScheduledTask({
        id: newId(),
        label: `Message ${r.name}`,
        hour: 8,
        min: 0,
        contacts: [r.jid],
        prompt: "",
        enabled: true,
      } as ScheduledTask),
    );

  // saveSchedule appends the new task to the existing list and persists it.
  const saveSchedule = (t: ScheduledTask) => {
    setScheduleTask(null);
    ScheduleService.List()
      .then((existing) => ScheduleService.Save([...(existing ?? []), t]))
      .then(() => toast.success("Schedule created", { description: "Manage it on the Schedules page." }))
      .catch((e) => toast.error("Couldn't schedule", { description: String(e) }));
  };

  const pickerContacts = useMemo(() => rows.map((r) => ({ jid: r.jid, name: r.name })), [rows]);

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
          <div className="flex justify-end">
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" />}>
                <MoreHorizontal className="size-4" />
                <span className="sr-only">Actions</span>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-44">
                <DropdownMenuItem onClick={() => setProactive({ jid: r.jid, name: r.name })}>
                  <MessageCirclePlus className="size-4" /> Reach out now
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => openSchedule(r)}>
                  <CalendarClock className="size-4" /> Schedule…
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => openRow(r)}>
                  {r.managed ? (
                    <>
                      <Pencil className="size-4" /> Edit
                    </>
                  ) : (
                    <>
                      <UserPlus className="size-4" /> Add
                    </>
                  )}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
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

      <ProactiveDialog target={proactive} onClose={() => setProactive(null)} />

      <ScheduleDialog
        task={scheduleTask}
        contacts={pickerContacts}
        onClose={() => setScheduleTask(null)}
        onSave={saveSchedule}
      />
    </Page>
  );
}
