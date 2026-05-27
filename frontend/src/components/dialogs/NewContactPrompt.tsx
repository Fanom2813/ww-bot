import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";
import { Contact, ContactsService } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type Pending = { jid: string; name: string; preview: string };

/** numberOf turns "123456:7@s.whatsapp.net" into "+123456". */
function numberOf(jid: string): string {
  const n = jid.split("@")[0].split(":")[0];
  return n ? `+${n}` : jid;
}

/**
 * NewContactPrompt listens for the backend "unknown" event (an unsaved number
 * messaged the user) and offers to save them: a toast with a Save action that
 * opens a name-input dialog. Saved contacts default to the auto-reply tier and
 * can be reconfigured on the Contacts page.
 */
export function NewContactPrompt() {
  const [pending, setPending] = useState<Pending | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const off = Events.On("unknown", (e) => {
      const u = e.data as Pending;
      if (!u?.jid) return;
      toast(`New message from ${u.name || numberOf(u.jid)}`, {
        description: u.preview || "Unknown number — not in your contacts.",
        duration: 20000,
        action: {
          label: "Save",
          onClick: () => {
            setName(u.name || "");
            setPending(u);
          },
        },
      });
    });
    return () => off();
  }, []);

  const close = () => {
    if (saving) return;
    setPending(null);
    setName("");
  };

  const save = () => {
    if (!pending) return;
    setSaving(true);
    ContactsService.Upsert(
      new Contact({ jid: pending.jid, name: name.trim(), tier: "auto" as Contact["tier"] }),
    )
      .then(() => {
        toast.success("Contact saved", { description: name.trim() || numberOf(pending.jid) });
        setPending(null);
        setName("");
      })
      .catch((err) => toast.error("Couldn't save contact", { description: String(err) }))
      .finally(() => setSaving(false));
  };

  return (
    <Dialog open={pending !== null} onOpenChange={(open) => !open && close()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Save new contact</DialogTitle>
          <DialogDescription>
            {pending ? numberOf(pending.jid) : ""} messaged you. Give them a name so the bot
            can handle the chat.
          </DialogDescription>
        </DialogHeader>

        {pending?.preview && (
          <p className="line-clamp-3 rounded-md bg-muted/50 p-2 text-sm text-muted-foreground">
            “{pending.preview}”
          </p>
        )}

        <div className="space-y-1.5">
          <Label htmlFor="new-contact-name">Name</Label>
          <Input
            id="new-contact-name"
            autoFocus
            value={name}
            placeholder="e.g. Jane from the gym"
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && save()}
          />
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={close} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving ? "Saving…" : "Save contact"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
