import { useEffect, useState } from "react";
import { Outlet, useNavigate } from "react-router";
import { System } from "@wailsio/runtime";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { DashboardSidebar } from "@/components/sidebar-02/app-sidebar";
import { ControlService, WhatsAppService } from "@/lib/api";

export function AppLayout({ jid }: { jid: string }) {
  const [paused, setPaused] = useState(false);
  const [isMac, setIsMac] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    setIsMac(System.IsMac());
  }, []);

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
      <DashboardSidebar jid={jid} paused={paused} isMac={isMac} onLogout={handleLogout} />
      {/* The inset itself doesn't scroll; the Page body scrolls internally so
          the page header stays pinned. */}
      <SidebarInset className="min-h-0 overflow-hidden">
        <Outlet />
      </SidebarInset>
    </SidebarProvider>
  );
}
