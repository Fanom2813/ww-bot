import { useEffect, useState } from "react";
import { Outlet, useNavigate } from "react-router";
import { System } from "@wailsio/runtime";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { DashboardSidebar } from "@/components/sidebar-02/app-sidebar";
import { ControlService, WhatsAppService } from "@/lib/api";
import { useBotSound } from "@/lib/useBotSound";

export function AppLayout({ jid, online }: { jid: string; online: boolean }) {
  const [paused, setPaused] = useState(false);
  const [isMac] = useState(() => System.IsMac());
  const navigate = useNavigate();

  // Subtle blip when the bot starts acting on an inbound (not for ignored ones).
  useBotSound();

  useEffect(() => {
    ControlService.Paused().then(setPaused).catch(() => {});
  }, []);

  const handleLogout = () => {
    WhatsAppService.Logout()
      .then(() => navigate("/connect", { replace: true }))
      .catch(() => {});
  };

  return (
    <SidebarProvider className="h-svh overflow-hidden">
      <DashboardSidebar jid={jid} paused={paused} online={online} isMac={isMac} onLogout={handleLogout} />
      {/* The inset itself doesn't scroll; the Page body scrolls internally so
          the page header stays pinned. */}
      <SidebarInset className="min-h-0 overflow-hidden">
        <Outlet />
      </SidebarInset>
    </SidebarProvider>
  );
}
