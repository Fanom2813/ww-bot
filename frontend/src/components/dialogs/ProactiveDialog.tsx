import { useState } from "react";
import { toast } from "sonner";
import { ContactsService } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type Target = { jid: string; name: string };

type Props = {
  /** The contact to reach out to, or null when closed. */
  target: Target | null;
  onClose: () => void;
};

/**
 * ProactiveDialog lets the user have the bot reach out to a contact: either let
 * the AI decide what to say from the history, or give it a topic to open about.
 */
export function ProactiveDialog({ target, onClose }: Props) {
  const [mode, setMode] = useState<"auto" | "topic" | "manual">("auto");
  const [topic, setTopic] = useState("");
  const [manual, setManual] = useState("");
  const [sending, setSending] = useState(false);

  if (!target) return null;

  const canSend =
    (mode === "auto") ||
    (mode === "topic" && topic.trim().length > 0) ||
    (mode === "manual" && manual.trim().length > 0);

  const send = () => {
    if (!canSend) return;
    setSending(true);
    const op =
      mode === "manual"
        ? ContactsService.SendDirect(target.jid, manual.trim()).then(() => ({
            title: "Sending…",
            desc: `Your message will go to ${target.name}.`,
          }))
        : ContactsService.Proactive(target.jid, mode === "topic" ? topic.trim() : "").then(() => ({
            title: "Reaching out…",
            desc: `The bot will message ${target.name}.`,
          }));
    op
      .then((m) => {
        toast.success(m.title, { description: m.desc });
        onClose();
      })
      .catch((e) => toast.error("Couldn't send", { description: String(e) }))
      .finally(() => setSending(false));
  };

  return (
    <Dialog open={target !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Reach out to {target.name}</DialogTitle>
          <DialogDescription>
            Have the bot start the conversation. It uses your history and the current date so the
            opener feels natural.
          </DialogDescription>
        </DialogHeader>

        <RadioGroup
          value={mode}
          onValueChange={(v) => setMode((v as "auto" | "topic" | "manual") ?? "auto")}
        >
          <Field orientation="horizontal">
            <RadioGroupItem id="pr-auto" value="auto" />
            <FieldContentInline
              htmlFor="pr-auto"
              label="Let the AI decide"
              desc="It looks at your past chat and opens naturally (or just says hi if you've never talked)."
            />
          </Field>
          <Field orientation="horizontal">
            <RadioGroupItem id="pr-topic" value="topic" />
            <FieldContentInline
              htmlFor="pr-topic"
              label="About a topic"
              desc="Give a short topic and the AI opens a conversation around it."
            />
          </Field>
          <Field orientation="horizontal">
            <RadioGroupItem id="pr-manual" value="manual" />
            <FieldContentInline
              htmlFor="pr-manual"
              label="Write it yourself"
              desc="Send exactly what you type — no AI. Still paced with the typing indicator."
            />
          </Field>
        </RadioGroup>

        {mode === "topic" && (
          <Field>
            <FieldLabel htmlFor="pr-topic-input">Topic</FieldLabel>
            <Input
              id="pr-topic-input"
              autoFocus
              value={topic}
              placeholder="e.g. school, the weekend trip, his health"
              onChange={(e) => setTopic(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && send()}
            />
          </Field>
        )}

        {mode === "manual" && (
          <Field>
            <FieldLabel htmlFor="pr-manual-input">Message</FieldLabel>
            <Textarea
              id="pr-manual-input"
              autoFocus
              rows={4}
              value={manual}
              placeholder="Type the message you want to send…"
              onChange={(e) => setManual(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) send();
              }}
            />
            <FieldDescription>Press ⌘/Ctrl+Enter to send.</FieldDescription>
          </Field>
        )}

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={sending}>
            Cancel
          </Button>
          <Button onClick={send} disabled={sending || !canSend}>
            {sending ? "Sending…" : mode === "manual" ? "Send" : "Reach out"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function FieldContentInline({
  htmlFor,
  label,
  desc,
}: {
  htmlFor: string;
  label: string;
  desc: string;
}) {
  return (
    <div className="grid gap-0.5">
      <FieldLabel htmlFor={htmlFor} className="font-normal">
        {label}
      </FieldLabel>
      <FieldDescription>{desc}</FieldDescription>
    </div>
  );
}
