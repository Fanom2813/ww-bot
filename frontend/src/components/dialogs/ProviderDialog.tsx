import { useCallback, useRef, useState } from "react";
import { ProviderSetting } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
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

type Props = {
  /** The provider being added/edited, or null when closed. */
  provider: ProviderSetting | null;
  onClose: () => void;
  onSave: (p: ProviderSetting) => void;
};

type LocalEdits = Partial<ProviderSetting> & {
  /** The raw text for the CLI args input (joined from provider.args or typed by user). */
  _argsText?: string;
};

/** ProviderDialog adds or edits an LLM provider: OpenAI-compatible, Anthropic, or a custom CLI. */
export function ProviderDialog({ provider, onClose, onSave }: Props) {
  const [edits, setEdits] = useState<LocalEdits>({});
  const lastProviderName = useRef<string | undefined>(undefined);

  // When the provider identity changes (dialog opened with a different row),
  // reset all local edits.
  if (provider?.name !== lastProviderName.current) {
    lastProviderName.current = provider?.name;
    setEdits({});
  }

  // Merge the canonical provider with any local edits.
  const p = provider ? { ...provider, ...edits } as ProviderSetting : null;

  // Compute the args text: if the user has edited it, use their text;
  // otherwise derive from the provider's args array.
  const argsText = edits._argsText !== undefined
    ? edits._argsText
    : (provider?.args ?? []).join(" ");

  const set = useCallback((patch: Partial<ProviderSetting>) => {
    setEdits((prev) => ({ ...prev, ...patch }));
  }, []);

  const setArgsText = useCallback((text: string) => {
    setEdits((prev) => ({ ...prev, _argsText: text }));
  }, []);

  const handleOpenChange = useCallback((open: boolean) => {
    if (!open) {
      setEdits({});
      onClose();
    }
  }, [onClose]);

  if (!p) return null;

  const isEdit = !!provider?.name;
  const kind = p.kind || "openai";
  const isAnthropic = kind === "anthropic";
  const isCLI = kind === "cli";

  const valid =
    !!p.name.trim() &&
    (isCLI
      ? !!(p.bin ?? "").trim()
      : !!(p.model ?? "").trim() && (isAnthropic || !!(p.baseUrl ?? "").trim()));

  const save = () => {
    const next = new ProviderSetting({ ...p } as ProviderSetting);
    next.name = next.name.trim();
    if (isCLI) {
      next.bin = (next.bin ?? "").trim();
      next.args = argsText.trim() ? argsText.trim().split(/\s+/) : [];
      next.requiresKey = false;
      next.baseUrl = "";
      next.model = "";
      next.apiKey = "";
    } else {
      next.model = (next.model ?? "").trim();
      if (!isAnthropic) next.baseUrl = (next.baseUrl ?? "").trim();
      next.requiresKey = isAnthropic || !!(next.apiKey && next.apiKey.trim()) || !!p.hasKey;
    }
    onSave(next);
  };

  return (
    <Dialog open={provider !== null} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit provider" : "Add LLM provider"}</DialogTitle>
          <DialogDescription>
            An OpenAI-compatible endpoint, Anthropic, or any local CLI that takes a prompt.
            API keys are stored in your OS keychain, never in the app.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3">
          <Field>
            <FieldLabel>Type</FieldLabel>
            <Select value={kind} onValueChange={(v) => set({ kind: v ?? "openai" })}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="openai">OpenAI-compatible</SelectItem>
                <SelectItem value="anthropic">Anthropic (Claude)</SelectItem>
                <SelectItem value="cli">Custom CLI</SelectItem>
              </SelectContent>
            </Select>
          </Field>

          <Field>
            <FieldLabel htmlFor="pv-name">Name</FieldLabel>
            <Input
              id="pv-name"
              value={p.name}
              placeholder={isCLI ? "my-claude" : isAnthropic ? "claude" : "groq"}
              readOnly={isEdit}
              className={isEdit ? "text-muted-foreground" : ""}
              onChange={(e) => set({ name: e.target.value })}
            />
          </Field>

          {isCLI ? (
            <>
              <Field>
                <FieldLabel htmlFor="pv-bin">Command</FieldLabel>
                <Input
                  id="pv-bin"
                  value={p.bin ?? ""}
                  placeholder="claude"
                  onChange={(e) => set({ bin: e.target.value })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="pv-args">Arguments</FieldLabel>
                <Input
                  id="pv-args"
                  value={argsText}
                  placeholder="--model sonnet -p"
                  onChange={(e) => setArgsText(e.target.value)}
                />
                <FieldDescription>
                  Flags passed before the prompt, e.g. <code>--model sonnet -p</code>.
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel>Prompt delivery</FieldLabel>
                <Select
                  value={p.appendPrompt ? "arg" : "stdin"}
                  onValueChange={(v) => set({ appendPrompt: v === "arg" })}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="stdin">Pipe to stdin (e.g. claude -p)</SelectItem>
                    <SelectItem value="arg">Append as last argument (e.g. gemini -p …)</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
            </>
          ) : (
            <>
              {!isAnthropic && (
                <Field>
                  <FieldLabel htmlFor="pv-url">Base URL</FieldLabel>
                  <Input
                    id="pv-url"
                    value={p.baseUrl ?? ""}
                    placeholder="https://api.groq.com/openai/v1"
                    onChange={(e) => set({ baseUrl: e.target.value })}
                  />
                </Field>
              )}
              <Field>
                <FieldLabel htmlFor="pv-model">Model</FieldLabel>
                <Input
                  id="pv-model"
                  value={p.model ?? ""}
                  placeholder={isAnthropic ? "claude-sonnet-4-5" : "llama-3.3-70b-versatile"}
                  onChange={(e) => set({ model: e.target.value })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="pv-key">
                  API key{" "}
                  {!isAnthropic && <span className="text-muted-foreground">(blank for local)</span>}
                </FieldLabel>
                <Input
                  id="pv-key"
                  type="password"
                  value={p.apiKey ?? ""}
                  placeholder={p.hasKey ? "•••••••• saved — leave blank to keep" : "sk-…"}
                  onChange={(e) => set({ apiKey: e.target.value })}
                />
                <FieldDescription>Stored securely in your OS keychain.</FieldDescription>
              </Field>
            </>
          )}
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => { setEdits({}); onClose(); }}>
            Cancel
          </Button>
          <Button onClick={save} disabled={!valid}>
            {isEdit ? "Save" : "Add provider"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
