import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Contact, ContactsService } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const TIERS = [
  { value: "auto", label: "Auto-send" },
  { value: "draft", label: "Draft & approve" },
  { value: "notify", label: "Notify only" },
];

type Props = {
  /** The contact to edit, or null when the dialog is closed. */
  contact: Contact | null;
  /** Whether the JID/number field is editable (manual add) or fixed. */
  jidEditable: boolean;
  onClose: () => void;
  onSaved: () => void;
};

/** ContactDialog is the modal editor used to add or configure a managed contact. */
export function ContactDialog({ contact, jidEditable, onClose, onSaved }: Props) {
  const [c, setC] = useState<Contact | null>(contact);
  const [saving, setSaving] = useState(false);

  useEffect(() => setC(contact), [contact]);

  const set = (patch: Partial<Contact>) => c && setC({ ...c, ...patch } as Contact);

  const save = () => {
    if (!c) return;
    const jid = c.jid.includes("@") ? c.jid : `${c.jid}@s.whatsapp.net`;
    setSaving(true);
    ContactsService.Upsert(new Contact({ ...c, jid } as Contact))
      .then(() => {
        toast.success("Contact saved", { description: c.name || jid });
        onSaved();
        onClose();
      })
      .catch((e) => toast.error("Save failed", { description: String(e) }))
      .finally(() => setSaving(false));
  };

  return (
    <Dialog open={contact !== null} onOpenChange={(open) => !open && !saving && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{jidEditable ? "Add contact" : c?.name || c?.jid}</DialogTitle>
          <DialogDescription>Set how the bot should handle this person.</DialogDescription>
        </DialogHeader>

        {c && (
          <div className="grid gap-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="cd-name">Name</Label>
                <Input
                  id="cd-name"
                  value={c.name}
                  placeholder="Dad"
                  onChange={(e) => set({ name: e.target.value })}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="cd-lang">Language</Label>
                <Input
                  id="cd-lang"
                  value={c.language}
                  placeholder="Swahili"
                  onChange={(e) => set({ language: e.target.value })}
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="cd-jid">Number or JID</Label>
              <Input
                id="cd-jid"
                value={c.jid}
                readOnly={!jidEditable}
                placeholder="2557… or 2557…@s.whatsapp.net"
                className={jidEditable ? "" : "text-muted-foreground"}
                onChange={(e) => set({ jid: e.target.value })}
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="cd-style">Reply style</Label>
              <Input
                id="cd-style"
                value={c.style}
                placeholder="e.g. warm and brief, lots of duas"
                onChange={(e) => set({ style: e.target.value })}
              />
            </div>

            <div className="space-y-1.5">
              <Label>Trust tier</Label>
              <Select
                value={c.tier || "notify"}
                onValueChange={(v) => set({ tier: (v ?? "notify") as Contact["tier"] })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TIERS.map((t) => (
                    <SelectItem key={t.value} value={t.value}>
                      {t.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="cd-notes">Notes</Label>
              <Textarea
                id="cd-notes"
                rows={2}
                value={c.notes}
                onChange={(e) => set({ notes: e.target.value })}
              />
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={save} disabled={saving || !c?.jid.trim()}>
            {saving ? "Saving…" : "Save contact"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
