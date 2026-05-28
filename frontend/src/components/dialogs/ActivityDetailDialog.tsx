import { Activity as ActivityItem } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  MessageSquare,
  Phone,
  AlertTriangle,
  Send,
  Zap,
  Settings2,
  Terminal,
  EyeOff,
} from "lucide-react";
import { cn } from "@/lib/utils";

const KIND_META: Record<string, { label: string; icon: typeof Send; color: string }> = {
  sent: { label: "Sent", icon: Send, color: "text-green-600" },
  draft: { label: "Draft", icon: MessageSquare, color: "text-blue-600" },
  call: { label: "Call", icon: Phone, color: "text-purple-600" },
  flag: { label: "Flag", icon: AlertTriangle, color: "text-amber-600" },
  proactive: { label: "Proactive", icon: Zap, color: "text-cyan-600" },
  command: { label: "Command", icon: Terminal, color: "text-indigo-600" },
  silent: { label: "Silent", icon: EyeOff, color: "text-muted-foreground" },
  system: { label: "System", icon: Settings2, color: "text-muted-foreground" },
};

function fullTime(ts: string): string {
  const d = new Date(ts);
  return isNaN(d.getTime()) ? "" : d.toLocaleString();
}

function shortJid(jid: string): string {
  const num = jid.split("@")[0].split(":")[0];
  return num ? `+${num}` : jid;
}

type Props = {
  activity: ActivityItem | null;
  onClose: () => void;
};

export function ActivityDetailDialog({ activity, onClose }: Props) {
  if (!activity) return null;

  const meta = KIND_META[activity.kind] ?? KIND_META.system;
  const Icon = meta.icon;
  const ts = fullTime(activity.ts);

  return (
    <Dialog open={activity !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Badge variant="outline" className="gap-1.5">
              <Icon className={cn("size-3.5", meta.color)} />
              {meta.label}
            </Badge>
            Activity detail
          </DialogTitle>
          <DialogDescription>Full details of this activity entry.</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div>
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Summary</p>
            <p className="mt-1 text-sm">{activity.summary}</p>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Contact</p>
              <p className="mt-1 text-sm">{shortJid(activity.chatJid)}</p>
              <p className="text-xs text-muted-foreground">{activity.chatJid}</p>
            </div>
            <div>
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Time</p>
              <p className="mt-1 text-sm">{ts}</p>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
