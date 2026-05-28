import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { DataTable, type Column } from "@/components/DataTable";
import { ScheduleDialog } from "@/components/dialogs";
import { Contact, ContactsService, ScheduledTask, ScheduleService } from "@/lib/api";
import { Pencil, Plus, Trash2 } from "lucide-react";

const pad = (n: number) => String(n).padStart(2, "0");

function newId(): string {
  return typeof crypto !== "undefined" && crypto.randomUUID
    ? crypto.randomUUID()
    : `sch-${Date.now()}`;
}

export function Schedules() {
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [contacts, setContacts] = useState<{ jid: string; name: string }[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<ScheduledTask | null>(null);

  const load = useCallback(() => {
    setLoading(true);
    Promise.all([
      ScheduleService.List().catch(() => [] as ScheduledTask[]),
      ContactsService.List().catch(() => [] as Contact[]),
    ])
      .then(([ts, cs]) => {
        setTasks(ts ?? []);
        setContacts((cs ?? []).map((c) => ({ jid: c.jid, name: c.name || c.jid })));
      })
      .finally(() => setLoading(false));
  }, []);
  useEffect(load, [load]);

  const nameFor = useMemo(() => {
    const m = new Map(contacts.map((c) => [c.jid, c.name]));
    return (jid: string) => m.get(jid) || jid.split("@")[0].split(":")[0];
  }, [contacts]);

  const saveTask = useCallback((t: ScheduledTask) => {
    setEditing(null);
    setTasks((current) => {
      const exists = current.some((x) => x.id === t.id);
      const next = exists ? current.map((x) => (x.id === t.id ? t : x)) : [...current, t];
      ScheduleService.Save(next)
        .then(() => {
          toast.success("Schedule saved");
          load();
        })
        .catch((e) => toast.error("Save failed", { description: String(e) }));
      return next;
    });
  }, [load]);

  const removeTask = useCallback((id: string) => {
    setTasks((current) => {
      const next = current.filter((x) => x.id !== id);
      ScheduleService.Save(next)
        .then(() => {
          toast.success("Schedule removed");
          load();
        })
        .catch((e) => toast.error("Save failed", { description: String(e) }));
      return next;
    });
  }, [load]);

  const toggleTask = useCallback((id: string, enabled: boolean) => {
    setTasks((current) => {
      const next = current.map((x) => (x.id === id ? new ScheduledTask({ ...x, enabled }) : x));
      ScheduleService.Save(next)
        .then(() => {
          toast.success("Schedule saved");
          load();
        })
        .catch((e) => toast.error("Save failed", { description: String(e) }));
      return next;
    });
  }, [load]);

  const addTask = () =>
    setEditing(
      new ScheduledTask({ id: newId(), label: "", hour: 8, min: 0, contacts: [], prompt: "", enabled: true }),
    );

  const columns = useMemo<Column<ScheduledTask>[]>(
    () => [
      { id: "label", header: "Name", className: "font-medium", cell: (t) => t.label || "Untitled" },
      {
        id: "time",
        header: "Time",
        className: "whitespace-nowrap tabular-nums",
        cell: (t) => `${pad(t.hour)}:${pad(t.min)}`,
      },
      {
        id: "contacts",
        header: "Contacts",
        className: "max-w-0 truncate text-muted-foreground",
        cell: (t) =>
          (t.contacts ?? []).length === 0
            ? "—"
            : (t.contacts ?? []).map(nameFor).join(", "),
      },
      {
        id: "enabled",
        header: "On",
        cell: (t) => (
          <Switch checked={t.enabled} onCheckedChange={(v) => toggleTask(t.id, v)} />
        ),
      },
      {
        id: "action",
        header: "Action",
        headClassName: "text-right",
        className: "text-right",
        cell: (t) => (
          <div className="flex justify-end gap-1">
            <Button size="icon-sm" variant="ghost" onClick={() => setEditing(new ScheduledTask({ ...t }))}>
              <Pencil className="size-4" />
            </Button>
            <Button size="icon-sm" variant="ghost" onClick={() => removeTask(t.id)}>
              <Trash2 className="size-4" />
            </Button>
          </div>
        ),
      },
    ],
    [nameFor, toggleTask, removeTask],
  );

  return (
    <Page
      fill
      title="Schedules"
      description="Recurring proactive messages — the bot reaches out to contacts at a set time each day following your instruction."
      actions={
        <Button size="sm" onClick={addTask}>
          <Plus className="size-4" /> Add schedule
        </Button>
      }
    >
      <DataTable
        columns={columns}
        rows={tasks}
        rowKey={(t) => t.id}
        loading={loading}
        empty="No schedules yet. Click “Add schedule” to have the bot reach out on a timer."
      />

      <ScheduleDialog
        task={editing}
        contacts={contacts}
        onClose={() => setEditing(null)}
        onSave={saveTask}
      />
    </Page>
  );
}
