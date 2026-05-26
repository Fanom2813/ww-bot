import { useEffect, useState } from "react";
import { NavLink, Route, Routes } from "react-router";
import {
  Activity as ActivityIcon,
  Inbox,
  LayoutDashboard,
  MessageCircle,
  Power,
  Settings as SettingsIcon,
  Users,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ControlService } from "@/lib/api";
import { Dashboard } from "@/pages/Dashboard";
import { Approvals } from "@/pages/Approvals";
import { Contacts } from "@/pages/Contacts";
import { Activity } from "@/pages/Activity";
import { Settings } from "@/pages/Settings";

const NAV = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard, end: true },
  { to: "/approvals", label: "Approvals", icon: Inbox, end: false },
  { to: "/contacts", label: "Contacts", icon: Users, end: false },
  { to: "/activity", label: "Activity", icon: ActivityIcon, end: false },
  { to: "/settings", label: "Settings", icon: SettingsIcon, end: false },
];

export function AppShell({ jid }: { jid: string }) {
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    ControlService.Paused().then(setPaused).catch(() => {});
  }, []);

  const togglePause = () => {
    const next = !paused;
    const op = next ? ControlService.Pause() : ControlService.Resume();
    op.then(() => setPaused(next)).catch(() => {});
  };

  return (
    <div className="flex h-screen bg-background text-foreground">
      <aside className="flex w-60 shrink-0 flex-col border-r">
        <div className="flex items-center gap-2 px-4 py-4">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <MessageCircle className="h-4 w-4" />
          </div>
          <span className="font-semibold">WW Bot</span>
        </div>

        <nav className="flex-1 space-y-1 px-2">
          {NAV.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                  isActive && "bg-accent text-accent-foreground",
                )
              }
            >
              <Icon className="h-4 w-4" />
              {label}
            </NavLink>
          ))}
        </nav>

        <div className="space-y-2 border-t p-3">
          <div className="flex items-center gap-2 px-1 text-xs text-muted-foreground">
            <span
              className={cn(
                "h-2 w-2 rounded-full",
                paused ? "bg-yellow-500" : "bg-green-500",
              )}
            />
            {paused ? "Paused" : "Active"}
            {jid && ` · ${shortJid(jid)}`}
          </div>
          <button
            onClick={togglePause}
            className="flex w-full items-center justify-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors hover:bg-accent hover:text-accent-foreground"
          >
            <Power className="h-4 w-4" />
            {paused ? "Resume bot" : "Pause bot"}
          </button>
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/approvals" element={<Approvals />} />
          <Route path="/contacts" element={<Contacts />} />
          <Route path="/activity" element={<Activity />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </main>
    </div>
  );
}

/** shortJid turns "1234567890:12@s.whatsapp.net" into "+1234567890". */
function shortJid(jid: string): string {
  const num = jid.split("@")[0].split(":")[0];
  return num ? `+${num}` : jid;
}
