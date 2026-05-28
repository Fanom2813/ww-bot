import { useState } from "react";
import { toast } from "sonner";
import { ContactsService } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  const [mode, setMode] = useState<"auto" | "topic">("auto");
  const [topic, setTopic] = useState("");
  const [sending, setSending] = useState(false);

  if (!target) return null;

  const send = () => {
    const t = mode === "topic" ? topic.trim() : "";
    if (mode === "topic" && !t) return;
    setSending(true);
    ContactsService.Proactive(target.jid, t)
      .then(() => {
        toast.success("Reaching out…", { description: `The bot will message ${target.name}.` });
        onClose();
      })
      .catch((e) => toast.error("Couldn't start", { description: String(e) }))
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

        <RadioGroup value={mode} onValueChange={(v) => setMode((v as "auto" | "topic") ?? "auto")}>
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

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={sending}>
            Cancel
          </Button>
          <Button onClick={send} disabled={sending || (mode === "topic" && !topic.trim())}>
            {sending ? "Sending…" : "Reach out"}
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
