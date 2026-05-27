import { useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Settings as SettingsModel, SettingsService } from "@/lib/api";

/** Section is one settings group: a title + subtitle, then its individual fields. */
function Section({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <FieldSet className="border-t pt-8 first:border-t-0 first:pt-0">
      <div>
        <FieldLegend className="mb-0">{title}</FieldLegend>
        <FieldDescription>{description}</FieldDescription>
      </div>
      <FieldGroup>{children}</FieldGroup>
    </FieldSet>
  );
}

export function Settings() {
  const [s, setS] = useState<SettingsModel | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    SettingsService.Get()
      .then(setS)
      .catch((e) => toast.error("Couldn't load settings", { description: String(e) }));
  }, []);

  if (!s) {
    return <Page title="Settings" description="Loading…" />;
  }

  const set = (patch: Partial<SettingsModel>) => setS({ ...s, ...patch } as SettingsModel);

  const updateProvider = (i: number, patch: Record<string, unknown>) => {
    const ps = [...s.providers];
    ps[i] = { ...ps[i], ...patch } as (typeof ps)[number];
    set({ providers: ps });
  };
  const save = () => {
    setSaving(true);
    SettingsService.Save(s)
      .then(() => toast.success("Settings saved"))
      .catch((e) => toast.error("Save failed", { description: String(e) }))
      .finally(() => setSaving(false));
  };

  const num = (v: string) => parseInt(v || "0", 10) || 0;

  return (
    <Page
      title="Settings"
      description="AI backends, voice, reply behavior, and anti-ban safety."
      actions={
        <Button size="sm" onClick={save} disabled={saving}>
          {saving ? "Saving…" : "Save settings"}
        </Button>
      }
    >
      <div className="max-w-3xl space-y-8">
        {/* Reply mode */}
        <Section
          title="Reply mode"
          description="Saved contacts are always handled by their trust tier. Guest mode also lets the bot reply to people who aren’t in your contacts — 1-1 chats only, never groups."
        >
          <Field orientation="horizontal">
            <FieldContent>
              <FieldLabel htmlFor="guest-mode">Guest mode</FieldLabel>
              <FieldDescription>
                When off, a new number only triggers a “save contact?” prompt — no reply.
              </FieldDescription>
            </FieldContent>
            <Switch
              id="guest-mode"
              checked={s.guestMode}
              onCheckedChange={(v) => set({ guestMode: v })}
            />
          </Field>
          {s.guestMode && (
            <Field className="max-w-xs">
              <FieldLabel>How to handle guests</FieldLabel>
              <Select
                value={s.guestTier || "auto"}
                onValueChange={(v) => set({ guestTier: v ?? "auto" })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">Auto-send replies</SelectItem>
                  <SelectItem value="draft">Draft for approval</SelectItem>
                  <SelectItem value="notify">Notify only</SelectItem>
                </SelectContent>
              </Select>
            </Field>
          )}
        </Section>

        {/* AI backends */}
        <Section
          title="AI backends"
          description="Tried top-to-bottom; first available answers. CLI agents use your own subscription (no key) and only activate if installed."
        >
          {s.providers.map((p, i) => (
            <div key={`${p.name}-${i}`} className="space-y-3">
              <Field orientation="horizontal">
                <FieldContent>
                  <FieldLabel>
                    {p.name} <span className="text-muted-foreground">({p.kind})</span>
                  </FieldLabel>
                </FieldContent>
                <Switch
                  checked={p.enabled}
                  onCheckedChange={(v) => updateProvider(i, { enabled: v })}
                />
              </Field>
              {p.kind === "openai" && (
                <div className="grid grid-cols-1 gap-3 pl-1 sm:grid-cols-2">
                  <Field>
                    <FieldLabel>Model</FieldLabel>
                    <Input
                      value={p.model ?? ""}
                      onChange={(e) => updateProvider(i, { model: e.target.value })}
                    />
                  </Field>
                  <Field>
                    <FieldLabel>API key {p.requiresKey ? "" : "(optional)"}</FieldLabel>
                    <Input
                      type="password"
                      placeholder={p.requiresKey ? "required" : "not needed"}
                      value={p.apiKey ?? ""}
                      onChange={(e) => updateProvider(i, { apiKey: e.target.value })}
                    />
                  </Field>
                </div>
              )}
            </div>
          ))}
        </Section>

        {/* Voice */}
        <Section
          title="Voice transcription"
          description="Transcribe incoming voice notes so the bot can understand and reply. Free-tier Groq Whisper by default."
        >
          <Field orientation="horizontal">
            <FieldContent>
              <FieldLabel htmlFor="stt">Enable voice-note transcription</FieldLabel>
            </FieldContent>
            <Switch
              id="stt"
              checked={s.stt.enabled}
              onCheckedChange={(v) => set({ stt: { ...s.stt, enabled: v } })}
            />
          </Field>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field>
              <FieldLabel>Model</FieldLabel>
              <Input
                value={s.stt.model}
                onChange={(e) => set({ stt: { ...s.stt, model: e.target.value } })}
              />
            </Field>
            <Field>
              <FieldLabel>API key</FieldLabel>
              <Input
                type="password"
                value={s.stt.apiKey}
                onChange={(e) => set({ stt: { ...s.stt, apiKey: e.target.value } })}
              />
            </Field>
          </div>
        </Section>

        {/* Safety */}
        <Section
          title="Safety & anti-ban"
          description="Pacing, caps, and quiet hours keep the bot human-like and protect the number. Applies on next launch."
        >
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
            <NumberField label="Min delay (s)" value={s.safety.minDelaySec} onChange={(v) => set({ safety: { ...s.safety, minDelaySec: v } })} />
            <NumberField label="Max delay (s)" value={s.safety.maxDelaySec} onChange={(v) => set({ safety: { ...s.safety, maxDelaySec: v } })} />
            <NumberField label="Per minute" value={s.safety.perMinute} onChange={(v) => set({ safety: { ...s.safety, perMinute: v } })} />
            <NumberField label="Per day" value={s.safety.perDay} onChange={(v) => set({ safety: { ...s.safety, perDay: v } })} />
            <NumberField label="Cooldown (s)" value={s.safety.perContactCooldownSec} onChange={(v) => set({ safety: { ...s.safety, perContactCooldownSec: v } })} />
            <NumberField label="Quiet start (h)" value={s.safety.quietStart} onChange={(v) => set({ safety: { ...s.safety, quietStart: v } })} />
            <NumberField label="Quiet end (h)" value={s.safety.quietEnd} onChange={(v) => set({ safety: { ...s.safety, quietEnd: v } })} />
          </div>
        </Section>
      </div>
    </Page>
  );

  function NumberField({
    label,
    value,
    onChange,
  }: {
    label: string;
    value: number;
    onChange: (v: number) => void;
  }) {
    return (
      <Field>
        <FieldLabel>{label}</FieldLabel>
        <Input type="number" value={value} onChange={(e) => onChange(num(e.target.value))} />
      </Field>
    );
  }
}
