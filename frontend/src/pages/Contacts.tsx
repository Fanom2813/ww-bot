import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Contact, ContactsService } from "@/lib/api";

const TIERS = [
  { value: "auto", label: "Auto-send" },
  { value: "draft", label: "Draft & approve" },
  { value: "notify", label: "Notify only" },
];

function TierSelect({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <Select value={value || "notify"} onValueChange={(v) => onChange(v ?? "notify")}>
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
  );
}

function ContactCard({ initial, onSaved }: { initial: Contact; onSaved: () => void }) {
  const [c, setC] = useState<Contact>(initial);

  const save = () => {
    ContactsService.Upsert(c)
      .then(() => {
        toast("Contact saved");
        onSaved();
      })
      .catch((e) => toast.error("Save failed", { description: String(e) }));
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{c.name || c.jid}</CardTitle>
        <p className="text-xs text-muted-foreground">{c.jid}</p>
      </CardHeader>
      <CardContent className="grid gap-3">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1">
            <Label>Name</Label>
            <Input value={c.name} onChange={(e) => setC({ ...c, name: e.target.value } as Contact)} />
          </div>
          <div className="space-y-1">
            <Label>Language</Label>
            <Input value={c.language} onChange={(e) => setC({ ...c, language: e.target.value } as Contact)} />
          </div>
        </div>
        <div className="space-y-1">
          <Label>Reply style</Label>
          <Input
            placeholder="e.g. warm and brief, lots of duas"
            value={c.style}
            onChange={(e) => setC({ ...c, style: e.target.value } as Contact)}
          />
        </div>
        <div className="space-y-1">
          <Label>Trust tier</Label>
          <TierSelect value={c.tier} onChange={(v) => setC({ ...c, tier: v } as Contact)} />
        </div>
        <div className="space-y-1">
          <Label>Notes</Label>
          <Textarea
            rows={2}
            value={c.notes}
            onChange={(e) => setC({ ...c, notes: e.target.value } as Contact)}
          />
        </div>
        <div>
          <Button onClick={save}>Save</Button>
        </div>
      </CardContent>
    </Card>
  );
}

export function Contacts() {
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [newJid, setNewJid] = useState("");
  const [newName, setNewName] = useState("");

  const load = () => {
    ContactsService.List()
      .then((c) => setContacts(c ?? []))
      .catch((e) => toast.error("Couldn't load contacts", { description: String(e) }));
  };
  useEffect(load, []);

  const add = () => {
    const jid = newJid.includes("@") ? newJid : `${newJid}@s.whatsapp.net`;
    ContactsService.Upsert(new Contact({ jid, name: newName, tier: "draft" as Contact["tier"] }))
      .then(() => {
        setNewJid("");
        setNewName("");
        toast("Contact added");
        load();
      })
      .catch((e) => toast.error("Add failed", { description: String(e) }));
  };

  return (
    <Page
      title="Contacts"
      description="Per-person rules: language, reply style, and trust tier."
    >
      <Card className="mb-4">
        <CardHeader>
          <CardTitle className="text-base">Add contact</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label>Number or JID</Label>
            <Input
              placeholder="2557… or 2557…@s.whatsapp.net"
              value={newJid}
              onChange={(e) => setNewJid(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label>Name</Label>
            <Input placeholder="Dad" value={newName} onChange={(e) => setNewName(e.target.value)} />
          </div>
          <Button onClick={add} disabled={!newJid.trim()}>
            Add
          </Button>
        </CardContent>
      </Card>

      {contacts.length === 0 ? (
        <p className="text-sm text-muted-foreground">No contacts yet.</p>
      ) : (
        <div className="grid gap-4">
          {contacts.map((c) => (
            <ContactCard key={c.jid} initial={c} onSaved={load} />
          ))}
        </div>
      )}
    </Page>
  );
}
