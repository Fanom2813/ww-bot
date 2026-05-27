import { useEffect, useState } from "react";
import { toast } from "sonner";
import { ArrowDown, ArrowUp } from "lucide-react";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Settings as SettingsModel, SettingsService } from "@/lib/api";

export function Settings() {
  const [s, setS] = useState<SettingsModel | null>(null);

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
  const moveProvider = (i: number, dir: number) => {
    const j = i + dir;
    if (j < 0 || j >= s.providers.length) return;
    const ps = [...s.providers];
    [ps[i], ps[j]] = [ps[j], ps[i]];
    set({ providers: ps });
  };

  const save = () => {
    SettingsService.Save(s)
      .then(() => toast("Settings saved"))
      .catch((e) => toast.error("Save failed", { description: String(e) }));
  };

  const num = (v: string) => parseInt(v || "0", 10) || 0;

  return (
    <Page title="Settings" description="AI backends, voice, and anti-ban safety.">
      <div className="grid gap-6">
        {/* Reply mode */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Reply mode</CardTitle>
            <CardDescription>
              Saved contacts are always handled by their trust tier. Guest mode also lets
              the bot reply to people who aren&apos;t in your contacts — 1-1 chats only,
              never groups.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-3">
              <Switch checked={s.guestMode} onCheckedChange={(v) => set({ guestMode: v })} />
              <div>
                <p className="text-sm font-medium">Guest mode</p>
                <p className="text-xs text-muted-foreground">
                  When off, a new number only triggers a “save contact?” prompt — no reply.
                </p>
              </div>
            </div>
            {s.guestMode && (
              <div className="max-w-xs space-y-1">
                <Label>How to handle guests</Label>
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
              </div>
            )}
          </CardContent>
        </Card>

        {/* AI backends */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">AI backends</CardTitle>
            <CardDescription>
              Tried top-to-bottom; first available answers. CLI agents use your own
              subscription (no key) and only activate if installed.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {s.providers.map((p, i) => (
              <div key={`${p.name}-${i}`} className="rounded-md border p-3">
                <div className="flex items-center gap-3">
                  <Switch
                    checked={p.enabled}
                    onCheckedChange={(v) => updateProvider(i, { enabled: v })}
                  />
                  <span className="font-medium">{p.name}</span>
                  <span className="text-xs text-muted-foreground">({p.kind})</span>
                  <div className="ml-auto flex gap-1">
                    <Button size="icon" variant="ghost" onClick={() => moveProvider(i, -1)}>
                      <ArrowUp className="h-4 w-4" />
                    </Button>
                    <Button size="icon" variant="ghost" onClick={() => moveProvider(i, 1)}>
                      <ArrowDown className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
                {p.kind === "openai" && (
                  <div className="mt-3 grid grid-cols-2 gap-3">
                    <div className="space-y-1">
                      <Label>Model</Label>
                      <Input
                        value={p.model ?? ""}
                        onChange={(e) => updateProvider(i, { model: e.target.value })}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label>API key {p.requiresKey ? "" : "(optional)"}</Label>
                      <Input
                        type="password"
                        placeholder={p.requiresKey ? "required" : "not needed"}
                        value={p.apiKey ?? ""}
                        onChange={(e) => updateProvider(i, { apiKey: e.target.value })}
                      />
                    </div>
                  </div>
                )}
              </div>
            ))}
          </CardContent>
        </Card>

        {/* Voice */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Voice transcription</CardTitle>
            <CardDescription>Free-tier Groq Whisper by default.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-3">
              <Switch
                checked={s.stt.enabled}
                onCheckedChange={(v) => set({ stt: { ...s.stt, enabled: v } })}
              />
              <span className="text-sm">Enable voice-note transcription</span>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Model</Label>
                <Input
                  value={s.stt.model}
                  onChange={(e) => set({ stt: { ...s.stt, model: e.target.value } })}
                />
              </div>
              <div className="space-y-1">
                <Label>API key</Label>
                <Input
                  type="password"
                  value={s.stt.apiKey}
                  onChange={(e) => set({ stt: { ...s.stt, apiKey: e.target.value } })}
                />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Safety */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Safety &amp; anti-ban</CardTitle>
            <CardDescription>Pacing, caps, and quiet hours protect the number.</CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <Field label="Min delay (s)" value={s.safety.minDelaySec} onChange={(v) => set({ safety: { ...s.safety, minDelaySec: v } })} />
            <Field label="Max delay (s)" value={s.safety.maxDelaySec} onChange={(v) => set({ safety: { ...s.safety, maxDelaySec: v } })} />
            <Field label="Per minute" value={s.safety.perMinute} onChange={(v) => set({ safety: { ...s.safety, perMinute: v } })} />
            <Field label="Per day" value={s.safety.perDay} onChange={(v) => set({ safety: { ...s.safety, perDay: v } })} />
            <Field label="Cooldown (s)" value={s.safety.perContactCooldownSec} onChange={(v) => set({ safety: { ...s.safety, perContactCooldownSec: v } })} />
            <Field label="Quiet start (h)" value={s.safety.quietStart} onChange={(v) => set({ safety: { ...s.safety, quietStart: v } })} />
            <Field label="Quiet end (h)" value={s.safety.quietEnd} onChange={(v) => set({ safety: { ...s.safety, quietEnd: v } })} />
          </CardContent>
        </Card>

        <Separator />
        <div>
          <Button onClick={save}>Save settings</Button>
          <span className="ml-3 text-xs text-muted-foreground">
            Provider/voice changes apply immediately; safety pacing applies on next launch.
          </span>
        </div>
      </div>
    </Page>
  );

  function Field({
    label,
    value,
    onChange,
  }: {
    label: string;
    value: number;
    onChange: (v: number) => void;
  }) {
    return (
      <div className="space-y-1">
        <Label>{label}</Label>
        <Input type="number" value={value} onChange={(e) => onChange(num(e.target.value))} />
      </div>
    );
  }
}
