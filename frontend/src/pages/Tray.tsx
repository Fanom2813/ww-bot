import { useCallback, useEffect, useMemo, useState } from "react";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";
import { Check, Pause, Play, Search, Send, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  ApprovalsService,
  ContactsService,
  ControlService,
  Contact,
  Draft,
} from "@/lib/api";
import { ProactiveDialog } from "@/components/dialogs";

/**
 * Tray panel — the compact UI shown in the tray popover.
 * Three sections: pause toggle, pending approvals, and a reach-out picker.
 * Anything that needs longer-form editing (rewriting a draft, settings…)
 * opens the full app.
 */
export function Tray() {
  const [paused, setPaused] = useState(false);
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [query, setQuery] = useState("");
  const [reachTarget, setReachTarget] = useState<{ jid: string; name: string } | null>(null);

  const refreshPaused = useCallback(() => {
    ControlService.Paused().then(setPaused).catch(() => {});
  }, []);
  const refreshDrafts = useCallback(() => {
    ApprovalsService.List().then((d) => setDrafts(d ?? [])).catch(() => {});
  }, []);
  const refreshContacts = useCallback(() => {
    ContactsService.List().then((c) => setContacts(c ?? [])).catch(() => {});
  }, []);

  useEffect(() => {
    refreshPaused();
    refreshDrafts();
    refreshContacts();
    const offDrafts = Events.On("drafts", () => refreshDrafts());
    return () => offDrafts?.();
  }, [refreshPaused, refreshDrafts, refreshContacts]);

  const togglePause = () => {
    const action = paused ? ControlService.Resume() : ControlService.Pause();
    action
      .then(() => {
        setPaused(!paused);
        toast.success(paused ? "Bot resumed" : "Bot paused");
      })
      .catch((e) => toast.error("Couldn't toggle", { description: String(e) }));
  };

  const approve = (d: Draft) => {
    ApprovalsService.Approve(d.id, "")
      .then(() => toast.success("Reply sent"))
      .catch((e) => toast.error("Send failed", { description: String(e) }));
  };
  const reject = (d: Draft) => {
    ApprovalsService.Reject(d.id)
      .then(() => toast("Draft rejected"))
      .catch((e) => toast.error("Reject failed", { description: String(e) }));
  };

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const named = contacts.filter((c) => (c.name ?? "").trim() !== "");
    if (!q) return named.slice(0, 50);
    return named
      .filter((c) => c.name.toLowerCase().includes(q) || c.jid.includes(q))
      .slice(0, 50);
  }, [contacts, query]);

  return (
    <div
      className="flex h-screen w-screen flex-col bg-background text-foreground"
      style={{ WebkitUserSelect: "none" } as React.CSSProperties}
    >
      {/* Drag handle / title bar */}
      <div
        className="flex shrink-0 items-center justify-between border-b px-3 py-2 text-sm font-medium"
        style={{ ["--wails-draggable" as never]: "drag" } as React.CSSProperties}
      >
        <span>WW Quick</span>
        <Badge variant={paused ? "destructive" : "secondary"} className="text-[10px]">
          {paused ? "Paused" : "Active"}
        </Badge>
      </div>

      {/* Pause / resume */}
      <div className="shrink-0 border-b p-3">
        <Button
          variant={paused ? "default" : "outline"}
          className="w-full justify-center gap-2"
          onClick={togglePause}
        >
          {paused ? <Play className="size-4" /> : <Pause className="size-4" />}
          {paused ? "Resume bot" : "Pause bot"}
        </Button>
      </div>

      {/* Approvals */}
      <div className="flex max-h-[40%] shrink-0 flex-col overflow-hidden border-b">
        <div className="flex items-center justify-between px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          <span>Pending approvals</span>
          {drafts.length > 0 && <span>{drafts.length}</span>}
        </div>
        <div className="flex-1 overflow-y-auto px-3 pb-3">
          {drafts.length === 0 ? (
            <div className="py-4 text-center text-xs text-muted-foreground">
              No drafts waiting.
            </div>
          ) : (
            <ul className="space-y-2">
              {drafts.map((d) => (
                <li key={d.id} className="rounded-md border bg-card p-2 text-xs">
                  <div className="mb-1 truncate font-medium">{d.senderName || d.senderJid}</div>
                  <div className="mb-2 line-clamp-2 text-muted-foreground">{d.reply}</div>
                  <div className="flex gap-1">
                    <Button
                      size="sm"
                      className="h-7 flex-1 gap-1 text-xs"
                      onClick={() => approve(d)}
                    >
                      <Check className="size-3" /> Approve
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 flex-1 gap-1 text-xs"
                      onClick={() => reject(d)}
                    >
                      <X className="size-3" /> Reject
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>

      {/* Reach out */}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="flex items-center gap-2 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          <Send className="size-3" /> Reach out
        </div>
        <div className="px-3 pb-2">
          <div className="relative">
            <Search className="absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search contacts…"
              className="h-8 pl-7 text-xs"
            />
          </div>
        </div>
        <ul className="flex-1 overflow-y-auto px-2 pb-2">
          {filtered.length === 0 ? (
            <li className="px-2 py-3 text-center text-xs text-muted-foreground">
              {contacts.length === 0 ? "No contacts yet." : "No matches."}
            </li>
          ) : (
            filtered.map((c) => (
              <li key={c.jid}>
                <button
                  type="button"
                  className={cn(
                    "w-full truncate rounded-md px-2 py-1.5 text-left text-xs hover:bg-accent",
                  )}
                  onClick={() => setReachTarget({ jid: c.jid, name: c.name })}
                >
                  {c.name}
                </button>
              </li>
            ))
          )}
        </ul>
      </div>

      <ProactiveDialog target={reachTarget} onClose={() => setReachTarget(null)} />
    </div>
  );
}
