import { useCallback, useRef, useState } from "react";
import { toast } from "sonner";
import { ControlService } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/**
 * TodayContextDialog edits the bot's "today's context". It loads the current
 * value when opened and persists on save.
 */
export function TodayContextDialog({ open, onOpenChange }: Props) {
  const [text, setText] = useState("");
  const [saving, setSaving] = useState(false);
  const loadedRef = useRef(false);

  const handleOpenChange = useCallback((nextOpen: boolean) => {
    if (nextOpen && !loadedRef.current) {
      loadedRef.current = true;
      ControlService.Today()
        .then(setText)
        .catch(() => {});
    }
    if (!nextOpen) {
      loadedRef.current = false;
    }
    onOpenChange(nextOpen);
  }, [onOpenChange]);

  const save = () => {
    setSaving(true);
    ControlService.SetToday(text)
      .then(() => {
        toast.success("Today's context saved");
        onOpenChange(false);
      })
      .catch((e) => toast.error("Couldn't save", { description: String(e) }))
      .finally(() => setSaving(false));
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !saving && handleOpenChange(o)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Today's context</DialogTitle>
          <DialogDescription>
            Tell the bot what you're doing today so it answers people accurately.
          </DialogDescription>
        </DialogHeader>

        <Textarea
          autoFocus
          rows={4}
          placeholder="e.g. Deep work all morning, free after 3pm, traveling this evening…"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
