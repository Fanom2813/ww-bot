import { useRef, useState } from "react";
import { ScheduledTask } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { Check } from "lucide-react";

type Props = {
  /** The task being added/edited, or null when closed. */
  task: ScheduledTask | null;
  /** Managed contacts to choose targets from. */
  contacts: { jid: string; name: string }[];
  onClose: () => void;
  onSave: (t: ScheduledTask) => void;
};

const pad = (n: number) => String(n).padStart(2, "0");

/** ScheduleDialog edits one recurring proactive task (a daily "cron"). */
export function ScheduleDialog({ task, contacts, onClose, onSave }: Props) {
  const [edits, setEdits] = useState<Partial<ScheduledTask>>({});
  const prevTask = useRef(task);
  if (task !== prevTask.current) {
    prevTask.current = task;
    setEdits({});
  }
  const t = task ? ({ ...task, ...edits } as ScheduledTask) : null;
  if (!t) return null;

  const set = (patch: Partial<ScheduledTask>) => setEdits((prev) => ({ ...prev, ...patch }));
  const selected = new Set(t.contacts ?? []);
  const toggle = (jid: string) => {
    const next = new Set(selected);
    if (next.has(jid)) next.delete(jid);
    else next.add(jid);
    set({ contacts: [...next] });
  };

  const valid = t.prompt.trim() && (t.contacts ?? []).length > 0;

  return (
    <Dialog open={task !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t.label || "Scheduled message"}</DialogTitle>
          <DialogDescription>
            Every day at the set time, the bot reaches out to the chosen contacts following your
            instruction (adapts to the history and date).
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3">
          <div className="grid grid-cols-3 gap-3">
            <div className="col-span-2">
              <Field>
                <FieldLabel htmlFor="sc-label">Name</FieldLabel>
                <Input
                  id="sc-label"
                  value={t.label}
                  placeholder="Morning dua for Dad"
                  onChange={(e) => set({ label: e.target.value })}
                />
              </Field>
            </div>
            <Field>
              <FieldLabel htmlFor="sc-time">Time</FieldLabel>
              <Input
                id="sc-time"
                type="time"
                value={`${pad(t.hour)}:${pad(t.min)}`}
                onChange={(e) => {
                  const [h, m] = e.target.value.split(":").map((v) => parseInt(v, 10) || 0);
                  set({ hour: h, min: m });
                }}
              />
            </Field>
          </div>

          <Field>
            <FieldLabel htmlFor="sc-prompt">Instruction</FieldLabel>
            <Textarea
              id="sc-prompt"
              rows={3}
              value={t.prompt}
              placeholder="e.g. Send a warm morning salaam and a short dua. Or: ask how the project is going."
              onChange={(e) => set({ prompt: e.target.value })}
            />
          </Field>

          <Field>
            <FieldLabel>Contacts ({(t.contacts ?? []).length} selected)</FieldLabel>
            <div className="max-h-48 overflow-auto rounded-md border">
              {contacts.length === 0 ? (
                <p className="p-3 text-sm text-muted-foreground">
                  No saved contacts yet: add some on the Contacts page first.
                </p>
              ) : (
                contacts.map((c) => {
                  const on = selected.has(c.jid);
                  return (
                    <button
                      key={c.jid}
                      type="button"
                      onClick={() => toggle(c.jid)}
                      className={cn(
                        "flex w-full items-center gap-2 border-b px-3 py-2 text-left text-sm last:border-b-0 hover:bg-muted/50",
                        on && "bg-muted/40",
                      )}
                    >
                      <span
                        className={cn(
                          "flex size-4 shrink-0 items-center justify-center rounded border",
                          on ? "border-primary bg-primary text-primary-foreground" : "border-input",
                        )}
                      >
                        {on && <Check className="size-3" />}
                      </span>
                      <span className="truncate">{c.name}</span>
                    </button>
                  );
                })
              )}
            </div>
            <FieldDescription>The bot will message each selected contact.</FieldDescription>
          </Field>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={() => onSave(t)} disabled={!valid}>
            Save schedule
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
