import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";
import { Check, Pencil, Users, X, Zap } from "lucide-react";
import { Page } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { ApprovalsService, Draft } from "@/lib/api";

/* ------------------------------------------------------------------------- */
/* Helpers                                                                   */
/* ------------------------------------------------------------------------- */

function relTime(d: Date): string {
  const s = Math.round((Date.now() - d.getTime()) / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h ago`;
  const days = Math.round(h / 24);
  return `${days}d ago`;
}

function confColor(c: number): string {
  if (c < 0.5) return "bg-red-500";
  if (c < 0.75) return "bg-amber-500";
  return "bg-emerald-500";
}

/** Split a reply by "---" lines into separate message parts (the gateway does
 *  this on send). Shown in the detail view so the user sees what will go out. */
function splitParts(text: string): string[] {
  return text.split(/\n\s*---\s*\n/).flatMap((p) => {
    const t = p.trim();
    return t ? [t] : [];
  });
}

const isMac =
  typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);
const cmd = isMac ? "⌘" : "Ctrl";

/* ------------------------------------------------------------------------- */
/* Page                                                                      */
/* ------------------------------------------------------------------------- */

export function Approvals() {
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [edits, setEdits] = useState<Record<number, string>>({});
  const editorRef = useRef<HTMLTextAreaElement | null>(null);

  const load = useCallback(() => {
    ApprovalsService.List()
      .then((d) => setDrafts(d ?? []))
      .catch((e) => toast.error("Couldn't load drafts", { description: String(e) }));
  }, []);

  // Initial load + live refresh on backend "drafts" events.
  useEffect(() => {
    load();
    const off = Events.On("drafts", () => load());
    return () => off();
  }, [load]);

  // Keep a sensible selection: pick the first draft when empty, drop when gone.
  if (drafts.length === 0 && selectedId !== null) {
    setSelectedId(null);
  } else if (drafts.length > 0 && (selectedId == null || !drafts.some((d) => d.id === selectedId))) {
    setSelectedId(drafts[0].id);
  }

  const selected = useMemo(
    () => drafts.find((d) => d.id === selectedId) ?? null,
    [drafts, selectedId],
  );
  const selectedText = selected ? edits[selected.id] ?? selected.reply : "";

  /* ----- actions --------------------------------------------------------- */

  const approve = useCallback(
    (d: Draft) => {
      const text = edits[d.id] ?? "";
      ApprovalsService.Approve(d.id, text)
        .then(() => {
          toast.success("Reply sent");
          setEdits(({ [d.id]: _, ...rest }) => rest);
        })
        .catch((e) => toast.error("Send failed", { description: String(e) }));
    },
    [edits],
  );

  const reject = useCallback((d: Draft) => {
    ApprovalsService.Reject(d.id)
      .then(() => toast("Draft rejected"))
      .catch((e) => toast.error("Reject failed", { description: String(e) }));
  }, []);

  // Stable refs so the keyboard listener doesn't re-subscribe on every edits change.
  const approveRef = useRef(approve);
  approveRef.current = approve;
  const rejectRef = useRef(reject);
  rejectRef.current = reject;

  const approveAllFromContact = useCallback((d: Draft) => {
    ApprovalsService.ApproveAllFromContact(d.chatJid)
      .then((n) => toast.success(`Approved ${n} draft${n === 1 ? "" : "s"}`))
      .catch((e) => toast.error("Batch approve failed", { description: String(e) }));
  }, []);

  /* ----- keyboard shortcuts --------------------------------------------- */

  // Move selection up/down (J/K or ↓/↑), approve (⌘↩), reject (X), focus editor (E).
  // Skip when the user is typing in an input/textarea.
  useEffect(() => {
    const isTyping = () => {
      const el = document.activeElement;
      if (!el) return false;
      const tag = el.tagName;
      return tag === "INPUT" || tag === "TEXTAREA" || (el as HTMLElement).isContentEditable;
    };
    const handler = (e: KeyboardEvent) => {
      // ⌘/Ctrl+Enter approves even while typing in the editor.
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter" && selected) {
        e.preventDefault();
        approveRef.current(selected);
        return;
      }
      if (isTyping()) return;
      if (drafts.length === 0) return;
      const i = drafts.findIndex((d) => d.id === selectedId);
      if (e.key === "j" || e.key === "ArrowDown") {
        e.preventDefault();
        setSelectedId(drafts[Math.min(drafts.length - 1, i + 1)].id);
      } else if (e.key === "k" || e.key === "ArrowUp") {
        e.preventDefault();
        setSelectedId(drafts[Math.max(0, i - 1)].id);
      } else if (e.key === "x" && selected) {
        e.preventDefault();
        rejectRef.current(selected);
      } else if (e.key === "e" && selected) {
        e.preventDefault();
        editorRef.current?.focus();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [drafts, selectedId, selected]);

  /* ----- render --------------------------------------------------------- */

  const sameContactCount = selected
    ? drafts.filter((d) => d.chatJid === selected.chatJid).length
    : 0;

  return (
    <Page
      fill
      title="Approvals"
      description="Replies the bot drafted when it wasn’t sure — pick one, edit, then send."
      actions={
        <Badge variant="outline" className="font-mono">
          {drafts.length} pending
        </Badge>
      }
    >
      {drafts.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center">
          <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-muted">
            <Zap className="size-6 text-muted-foreground" />
          </div>
          <p className="font-medium">No drafts awaiting review</p>
          <p className="mt-1 text-sm text-muted-foreground">
            When the bot isn’t sure, replies land here. New ones appear automatically.
          </p>
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 gap-4">
          {/* Left: list */}
          <aside className="flex w-72 shrink-0 flex-col gap-1 overflow-auto rounded-lg border p-1.5">
            {drafts.map((d) => {
              const active = d.id === selectedId;
              const dirty = edits[d.id] != null && edits[d.id] !== d.reply;
              return (
                <button
                  type="button"
                  key={d.id}
                  onClick={() => setSelectedId(d.id)}
                  className={cn(
                    "rounded-md px-3 py-2 text-left transition-colors",
                    active ? "bg-accent" : "hover:bg-muted",
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm font-medium">
                      {d.senderName || d.chatJid}
                    </span>
                    <span
                      className={cn("size-2 shrink-0 rounded-full", confColor(d.confidence))}
                      title={`${(d.confidence * 100).toFixed(0)}% confidence`}
                    />
                  </div>
                  <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">
                    {d.reply || d.incoming || "(empty)"}
                  </p>
                  <div className="mt-0.5 flex items-center gap-2 text-[10px] text-muted-foreground">
                    <span>{relTime(new Date(d.createdAt))}</span>
                    {dirty && <span className="text-amber-500">• edited</span>}
                  </div>
                </button>
              );
            })}
          </aside>

          {/* Right: detail */}
          <section className="flex min-w-0 flex-1 flex-col overflow-auto rounded-lg border p-5">
            {selected ? (
              <DraftDetail
                d={selected}
                value={selectedText}
                onChange={(v) =>
                  setEdits((m) => ({ ...m, [selected.id]: v }))
                }
                editorRef={editorRef}
                onApprove={() => approve(selected)}
                onReject={() => reject(selected)}
                onApproveAll={
                  sameContactCount > 1 ? () => approveAllFromContact(selected) : undefined
                }
                sameContactCount={sameContactCount}
              />
            ) : (
              <p className="m-auto text-sm text-muted-foreground">Select a draft to review.</p>
            )}
          </section>
        </div>
      )}
    </Page>
  );
}

/* ------------------------------------------------------------------------- */
/* Detail pane                                                               */
/* ------------------------------------------------------------------------- */

interface DraftDetailProps {
  d: Draft;
  value: string;
  onChange: (v: string) => void;
  editorRef: React.RefObject<HTMLTextAreaElement | null>;
  onApprove: () => void;
  onReject: () => void;
  onApproveAll?: () => void;
  sameContactCount: number;
}

function DraftDetail({
  d,
  value,
  onChange,
  editorRef,
  onApprove,
  onReject,
  onApproveAll,
  sameContactCount,
}: DraftDetailProps) {
  const parts = useMemo(() => splitParts(value), [value]);
  const confPct = (d.confidence * 100).toFixed(0);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* header */}
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-lg font-semibold">{d.senderName || d.chatJid}</h2>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {relTime(new Date(d.createdAt))} · {d.chatJid}
          </p>
        </div>
        <Badge variant="outline" className="shrink-0 font-mono">
          <span className={cn("mr-1.5 size-2 rounded-full", confColor(d.confidence))} />
          {confPct}%
        </Badge>
      </div>

      {/* reason */}
      {d.reason && (
        <p className="mb-3 rounded-md bg-amber-500/10 px-3 py-1.5 text-xs text-muted-foreground">
          {d.reason}
        </p>
      )}

      {/* incoming */}
      <div className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        Incoming
      </div>
      <div className="mb-5 rounded-md bg-muted/50 p-3 text-sm">
        {d.incoming || <span className="text-muted-foreground">(no text)</span>}
      </div>

      {/* draft editor */}
      <div className="mb-2 flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Draft reply
        </span>
        <span className="text-xs text-muted-foreground">
          Use <code className="rounded bg-muted px-1">---</code> on its own line to split into
          multiple messages.
        </span>
      </div>
      <Textarea
        ref={editorRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        rows={6}
        className="font-mono text-sm"
        placeholder="Type a reply…"
      />

      {/* preview of split parts */}
      {parts.length > 1 && (
        <div className="mt-3 space-y-1.5">
          <div className="text-xs text-muted-foreground">
            Will send as {parts.length} messages with short pauses between:
          </div>
          {parts.map((p) => (
            <div
              key={p}
              className="ml-auto max-w-[80%] rounded-2xl rounded-br-sm bg-emerald-500/15 px-3 py-1.5 text-sm"
            >
              {p}
            </div>
          ))}
        </div>
      )}

      {/* action bar */}
      <div className="mt-5 flex flex-wrap items-center gap-2 border-t pt-4">
        <Button onClick={onApprove}>
          <Check className="size-4" /> Approve &amp; send
          <kbd className="ml-1 rounded bg-foreground/10 px-1 py-px text-[10px]">{cmd}↩</kbd>
        </Button>
        <Button variant="outline" onClick={onReject}>
          <X className="size-4" /> Reject
          <kbd className="ml-1 rounded bg-foreground/10 px-1 py-px text-[10px]">X</kbd>
        </Button>
        <Button variant="ghost" onClick={() => editorRef.current?.focus()}>
          <Pencil className="size-4" /> Edit
          <kbd className="ml-1 rounded bg-foreground/10 px-1 py-px text-[10px]">E</kbd>
        </Button>
        {onApproveAll && (
          <Button variant="outline" className="ml-auto" onClick={onApproveAll}>
            <Users className="size-4" /> Approve all {sameContactCount} from this contact
          </Button>
        )}
      </div>

      {/* shortcuts hint */}
      <p className="mt-3 text-[11px] text-muted-foreground">
        Shortcuts: <kbd className="rounded bg-muted px-1">J</kbd>/<kbd className="rounded bg-muted px-1">K</kbd> move ·{" "}
        <kbd className="rounded bg-muted px-1">{cmd}↩</kbd> approve ·{" "}
        <kbd className="rounded bg-muted px-1">X</kbd> reject ·{" "}
        <kbd className="rounded bg-muted px-1">E</kbd> edit
      </p>
    </div>
  );
}
