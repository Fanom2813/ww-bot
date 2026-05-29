import { useEffect, useState, type ReactNode } from "react";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";
import { Download, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
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
import { ProviderDialog } from "@/components/dialogs";
import { ProviderSetting, Settings as SettingsModel, SettingsService, UpdaterService } from "@/lib/api";

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

const num = (v: string) => parseInt(v || "0", 10) || 0;

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

/** UpdatesSection shows the current app version, a manual check button, and
 *  surfaces a one-click install when a newer release is available. Listens for
 *  the wails:updater:update-available event so it lights up automatically when
 *  the startup auto-check finds something. */
function UpdatesSection() {
  const [current, setCurrent] = useState<string>("");
  const [available, setAvailable] = useState<{ version: string; notes?: string } | null>(null);
  const [checking, setChecking] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [progress, setProgress] = useState<{ percent: number; speed?: number } | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    UpdaterService.CurrentVersion().then(setCurrent).catch(() => {});

    const offAvail = Events.On("wails:updater:update-available", (e) => {
      const r = Array.isArray(e?.data) ? e.data[0] : e?.data;
      if (r && typeof r === "object" && "version" in r) {
        setAvailable({ version: String(r.version), notes: r.notes ? String(r.notes) : undefined });
      }
    });
    const offNone = Events.On("wails:updater:no-update", () => setAvailable(null));
    const offProg = Events.On("wails:updater:download-progress", (e) => {
      const p = Array.isArray(e?.data) ? e.data[0] : e?.data;
      if (p && typeof p === "object") {
        const pct = "percent" in p ? Number(p.percent) : 0;
        const sp = "bytesPerSecond" in p ? Number(p.bytesPerSecond) : undefined;
        setProgress({ percent: pct, speed: sp });
      }
    });
    const offReady = Events.On("wails:updater:update-ready", () => {
      setReady(true);
      setInstalling(false);
    });
    const offErr = Events.On("wails:updater:error", (e) => {
      const info = Array.isArray(e?.data) ? e.data[0] : e?.data;
      const msg =
        info && typeof info === "object" && "message" in info ? String(info.message) : "Unknown error";
      toast.error("Updater error", { description: msg });
      setChecking(false);
      setInstalling(false);
    });
    return () => {
      offAvail?.();
      offNone?.();
      offProg?.();
      offReady?.();
      offErr?.();
    };
  }, []);

  const check = () => {
    setChecking(true);
    UpdaterService.Check()
      .then((rel) => {
        if (rel) {
          setAvailable({ version: rel.version, notes: rel.notes });
          toast.success(`Update available: ${rel.version}`);
        } else {
          setAvailable(null);
          toast(`You're on the latest version (${current || "?"}).`);
        }
      })
      .catch((e) => toast.error("Check failed", { description: String(e) }))
      .finally(() => setChecking(false));
  };

  const install = () => {
    setInstalling(true);
    setProgress({ percent: 0 });
    UpdaterService.DownloadAndInstall()
      .catch((e) => {
        toast.error("Install failed", { description: String(e) });
        setInstalling(false);
      });
  };

  return (
    <Section title="Updates" description="Keep WWAssist up to date with the latest release.">
      <Field orientation="horizontal">
        <FieldContent>
          <FieldLabel>Current version</FieldLabel>
          <FieldDescription>
            {current ? `v${current}` : "Loading…"}
            {available && (
              <>
                {" — "}
                <Badge variant="default">Update available: v{available.version}</Badge>
              </>
            )}
          </FieldDescription>
        </FieldContent>
        <Button variant="outline" size="sm" onClick={check} disabled={checking || installing}>
          <RefreshCw className={checking ? "size-4 animate-spin" : "size-4"} />
          {checking ? "Checking…" : "Check for updates"}
        </Button>
      </Field>

      {available && !ready && (
        <Field orientation="horizontal">
          <FieldContent>
            <FieldLabel>Install v{available.version}</FieldLabel>
            <FieldDescription>
              {installing
                ? progress
                  ? `Downloading… ${Math.round(progress.percent)}%`
                  : "Downloading…"
                : available.notes
                  ? available.notes.split("\n")[0]
                  : "Download and stage the new release."}
            </FieldDescription>
          </FieldContent>
          <Button size="sm" onClick={install} disabled={installing}>
            <Download className="size-4" />
            {installing ? "Installing…" : "Install"}
          </Button>
        </Field>
      )}

      {ready && (
        <Field orientation="horizontal">
          <FieldContent>
            <FieldLabel>Ready to apply</FieldLabel>
            <FieldDescription>Restart WWAssist to finish updating.</FieldDescription>
          </FieldContent>
        </Field>
      )}
    </Section>
  );
}

function SafetySection({
  safety,
  set,
}: {
  safety: SettingsModel["safety"];
  set: (patch: Partial<SettingsModel>) => void;
}) {
  return (
    <>
      <Section
        title="Safety & anti-ban"
        description="Pacing, caps, and quiet hours keep the bot human-like and protect the number. Applies on next launch."
      >
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
          <NumberField label="Min delay (s)" value={safety.minDelaySec} onChange={(v) => set({ safety: { ...safety, minDelaySec: v } })} />
          <NumberField label="Max delay (s)" value={safety.maxDelaySec} onChange={(v) => set({ safety: { ...safety, maxDelaySec: v } })} />
          <NumberField label="Per minute" value={safety.perMinute} onChange={(v) => set({ safety: { ...safety, perMinute: v } })} />
          <NumberField label="Per day" value={safety.perDay} onChange={(v) => set({ safety: { ...safety, perDay: v } })} />
          <NumberField label="Cooldown (s)" value={safety.perContactCooldownSec} onChange={(v) => set({ safety: { ...safety, perContactCooldownSec: v } })} />
        </div>
      </Section>

      <Section
        title="Quiet hours"
        description="Pause outgoing replies during these hours so the bot never texts at odd times (scheduled & proactive messages can bypass). Hours are 0-23; e.g. 23 to 7 means 11pm-7am."
      >
        <Field orientation="horizontal">
          <FieldContent>
            <FieldLabel htmlFor="quiet">Enable quiet hours</FieldLabel>
            <FieldDescription>When off, the bot may send at any time of day.</FieldDescription>
          </FieldContent>
          <Switch
            id="quiet"
            checked={!safety.quietHoursOff}
            onCheckedChange={(v) => set({ safety: { ...safety, quietHoursOff: !v } })}
          />
        </Field>
        {!safety.quietHoursOff && (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
            <NumberField label="Quiet start (h)" value={safety.quietStart} onChange={(v) => set({ safety: { ...safety, quietStart: v } })} />
            <NumberField label="Quiet end (h)" value={safety.quietEnd} onChange={(v) => set({ safety: { ...safety, quietEnd: v } })} />
          </div>
        )}
      </Section>
    </>
  );
}

function BackendsSection({
  cli,
  added,
  typeLabel,
  subLabel,
  toggleProvider,
  removeProvider,
  setProviderDialog,
}: {
  cli: ProviderSetting[];
  added: ProviderSetting[];
  typeLabel: (p: ProviderSetting) => string;
  subLabel: (p: ProviderSetting) => string;
  toggleProvider: (name: string, enabled: boolean) => void;
  removeProvider: (name: string) => void;
  setProviderDialog: (p: ProviderSetting | null) => void;
}) {
  return (
    <Section
      title="AI backends"
      description="Tried top-to-bottom; first available answers. CLI agents use your own subscription (no key). Add hosted providers (OpenAI-compatible or Anthropic) with the button below."
    >
      {cli.map((p) => (
        <Field key={p.name} orientation="horizontal">
          <FieldContent>
            <FieldLabel>
              {p.name} <span className="text-muted-foreground">(CLI)</span>
            </FieldLabel>
            <FieldDescription>Uses your local agent; only runs if installed.</FieldDescription>
          </FieldContent>
          <Switch checked={p.enabled} onCheckedChange={(v) => toggleProvider(p.name, v)} />
        </Field>
      ))}

      {added.map((p) => (
        <Field key={p.name} orientation="horizontal">
          <FieldContent>
            <FieldLabel className="flex items-center gap-2">
              {p.name}
              <span className="text-muted-foreground">({typeLabel(p)})</span>
              {p.hasKey && (
                <Badge variant="secondary" className="text-[10px]">
                  key set
                </Badge>
              )}
            </FieldLabel>
            <FieldDescription className="truncate">{subLabel(p)}</FieldDescription>
          </FieldContent>
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setProviderDialog(new ProviderSetting({ ...p } as ProviderSetting))}
          >
            <Pencil className="size-4" />
          </Button>
          <Button size="icon-sm" variant="ghost" onClick={() => removeProvider(p.name)}>
            <Trash2 className="size-4" />
          </Button>
          <Switch checked={p.enabled} onCheckedChange={(v) => toggleProvider(p.name, v)} />
        </Field>
      ))}

      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={() =>
            setProviderDialog(
              new ProviderSetting({ kind: "openai", requiresKey: true } as ProviderSetting),
            )
          }
        >
          <Plus className="size-4" /> Add provider
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setProviderDialog(new ProviderSetting({ kind: "cli" } as ProviderSetting))}
        >
          <Plus className="size-4" /> Add custom CLI
        </Button>
      </div>
    </Section>
  );
}

export function Settings() {
  const [s, setS] = useState<SettingsModel | null>(null);
  const [saving, setSaving] = useState(false);
  const [providerDialog, setProviderDialog] = useState<ProviderSetting | null>(null);
  const [defaults, setDefaults] = useState({ prompt: "", proactive: "" });

  const load = () =>
    SettingsService.Get()
      .then(setS)
      .catch((e) => toast.error("Couldn't load settings", { description: String(e) }));
  useEffect(() => {
    load();
    SettingsService.DefaultSystemPrompt().then((p) => setDefaults((d) => ({ ...d, prompt: p }))).catch(() => {});
    SettingsService.DefaultProactivePrompt().then((p) => setDefaults((d) => ({ ...d, proactive: p }))).catch(() => {});
  }, []);

  if (!s) {
    return <Page title="Settings" description="Loading…" />;
  }

  const set = (patch: Partial<SettingsModel>) => setS({ ...s, ...patch } as SettingsModel);

  // persist saves the whole settings (incl. any provider/key changes) and reloads
  // so HasKey/cleared keys reflect the keychain.
  const persist = (next: SettingsModel, msg = "Settings saved") => {
    setS(next);
    SettingsService.Save(next)
      .then(() => {
        toast.success(msg);
        load();
      })
      .catch((e) => toast.error("Save failed", { description: String(e) }));
  };

  const save = () => {
    setSaving(true);
    SettingsService.Save(s)
      .then(() => {
        toast.success("Settings saved");
        load();
      })
      .catch((e) => toast.error("Save failed", { description: String(e) }))
      .finally(() => setSaving(false));
  };

  const toggleProvider = (name: string, enabled: boolean) =>
    persist(
      new SettingsModel({
        ...s,
        providers: s.providers.map((x) =>
          x.name === name ? new ProviderSetting({ ...x, enabled }) : x,
        ),
      } as SettingsModel),
    );

  const saveProvider = (p: ProviderSetting) => {
    const exists = s.providers.some((x) => x.name === p.name);
    const providers = exists
      ? s.providers.map((x) => (x.name === p.name ? p : x))
      : [...s.providers, new ProviderSetting({ ...p, enabled: true } as ProviderSetting)];
    setProviderDialog(null);
    persist(new SettingsModel({ ...s, providers } as SettingsModel), "Provider saved");
  };

  const removeProvider = (name: string) =>
    persist(
      new SettingsModel({
        ...s,
        providers: s.providers.filter((x) => x.name !== name),
      } as SettingsModel),
      "Provider removed",
    );

  // Built-in CLI agents (auto-discovered, no command set) vs user-added providers
  // (hosted APIs and custom CLIs).
  const cli = s.providers.filter((p) => p.kind === "cli" && !p.bin);
  const added = s.providers.filter((p) => p.kind !== "cli" || !!p.bin);

  const typeLabel = (p: ProviderSetting) =>
    p.kind === "anthropic" ? "Anthropic" : p.kind === "cli" ? "CLI" : "OpenAI";
  const subLabel = (p: ProviderSetting) =>
    p.kind === "cli"
      ? [p.bin, ...(p.args ?? [])].join(" ")
      : `${p.model}${p.kind !== "anthropic" && p.baseUrl ? ` · ${p.baseUrl}` : ""}`;

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
          description="Saved contacts are always handled by their trust tier. Guest mode also lets the bot reply to people who aren’t in your contacts (1-1 chats only, never groups)."
        >
          <Field orientation="horizontal">
            <FieldContent>
              <FieldLabel htmlFor="guest-mode">Guest mode</FieldLabel>
              <FieldDescription>
                When off, a new number only triggers a “save contact?” prompt, no reply.
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

        {/* System prompt / persona */}
        <Section
          title="System prompt (persona)"
          description="How the bot writes replies: its voice, tone, and rules. Per-contact style still applies on top. Leave blank to use the built-in default; the JSON output format is fixed and unaffected by this."
        >
          <Field>
            <Textarea
              rows={10}
              className="font-mono text-xs"
              placeholder={defaults.prompt || "Leave blank to use the built-in default…"}
              value={s.systemPrompt}
              onChange={(e) => set({ systemPrompt: e.target.value })}
            />
            <FieldDescription>
              {s.systemPrompt
                ? "Using your custom prompt."
                : "Blank: using the built-in default (shown above as placeholder)."}
            </FieldDescription>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => set({ systemPrompt: defaults.prompt })}
                disabled={!defaults.prompt}
              >
                Load default to edit
              </Button>
              {s.systemPrompt && (
                <Button variant="ghost" size="sm" onClick={() => set({ systemPrompt: "" })}>
                  Reset to default
                </Button>
              )}
            </div>
          </Field>
        </Section>

        {/* Proactive prompt */}
        <Section
          title="Proactive prompt"
          description="Guides messages the bot starts itself (the “Reach out” button on a contact, and scheduled messages). It reads the history and current date to open naturally. Leave blank for the built-in default."
        >
          <Field>
            <Textarea
              rows={8}
              className="font-mono text-xs"
              placeholder={defaults.proactive || "Leave blank to use the built-in default…"}
              value={s.proactivePrompt}
              onChange={(e) => set({ proactivePrompt: e.target.value })}
            />
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => set({ proactivePrompt: defaults.proactive })}
                disabled={!defaults.proactive}
              >
                Load default to edit
              </Button>
              {s.proactivePrompt && (
                <Button variant="ghost" size="sm" onClick={() => set({ proactivePrompt: "" })}>
                  Reset to default
                </Button>
              )}
            </div>
          </Field>
        </Section>

        {/* AI backends */}
        <BackendsSection
          cli={cli}
          added={added}
          typeLabel={typeLabel}
          subLabel={subLabel}
          toggleProvider={toggleProvider}
          removeProvider={removeProvider}
          setProviderDialog={setProviderDialog}
        />

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

        {/* Safety & Quiet hours */}
        <SafetySection safety={s.safety} set={set} />

        {/* App updates */}
        <UpdatesSection />
      </div>

      <ProviderDialog
        provider={providerDialog}
        onClose={() => setProviderDialog(null)}
        onSave={saveProvider}
      />
    </Page>
  );
}
