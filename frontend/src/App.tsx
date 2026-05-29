import { useEffect, useState } from "react";
import { Route, Routes, useNavigate } from "react-router";
import { Events } from "@wailsio/runtime";
import { ThemeProvider } from "next-themes";
import { Toaster } from "@/components/ui/sonner";
import { WAProvider } from "@/lib/WAContext";
import { useWA } from "@/lib/useWA";
import { RequireOnboarded, RequireConnected } from "@/components/RouteGuards";
import { Onboarding } from "@/components/Onboarding";
import { ConnectScreen } from "@/components/ConnectScreen";
import { AppLayout } from "@/components/AppLayout";
import { NewContactPrompt, ProactiveDialog } from "@/components/dialogs";
import { Dashboard } from "@/pages/Dashboard";
import { Approvals } from "@/pages/Approvals";
import { Contacts } from "@/pages/Contacts";
import { Groups } from "@/pages/Groups";
import { Schedules } from "@/pages/Schedules";
import { Activity } from "@/pages/Activity";
import { Settings } from "@/pages/Settings";
import { Tray } from "@/pages/Tray";

function AppRoutes() {
  const { jid, online } = useWA();
  const navigate = useNavigate();
  const [reachTarget, setReachTarget] = useState<{ jid: string; name: string } | null>(null);

  useEffect(() => {
    const off = Events.On("tray:nav", (e) => {
      const path = typeof e?.data === "string" ? e.data : Array.isArray(e?.data) ? e.data[0] : "";
      if (path) navigate(path);
    });
    return () => off?.();
  }, [navigate]);

  useEffect(() => {
    const off = Events.On("tray:reach-out", (e) => {
      const raw = Array.isArray(e?.data) ? e.data[0] : e?.data;
      if (raw && typeof raw === "object" && "jid" in raw && "name" in raw) {
        setReachTarget({ jid: String(raw.jid), name: String(raw.name) });
      }
    });
    return () => off?.();
  }, []);

  return (
    <>
      <Routes>
        {/* Tray popover — second window, no auth guard, no full layout */}
        <Route path="/tray" element={<Tray />} />

        {/* Public: onboarding wizard */}
        <Route path="/onboarding" element={<Onboarding />} />

        {/* Public: QR pair screen */}
        <Route path="/connect" element={<ConnectScreen />} />

        {/* Guarded: must be onboarded + connected */}
        <Route element={<RequireOnboarded />}>
          <Route element={<RequireConnected />}>
            <Route element={<AppLayout jid={jid} online={online} />}>
              <Route index element={<Dashboard />} />
              <Route path="approvals" element={<Approvals />} />
              <Route path="contacts" element={<Contacts />} />
              <Route path="groups" element={<Groups />} />
              <Route path="schedules" element={<Schedules />} />
              <Route path="activity" element={<Activity />} />
              <Route path="settings" element={<Settings />} />
            </Route>
          </Route>
        </Route>
      </Routes>
      <Toaster richColors position="top-right" />
      <NewContactPrompt />
      <ProactiveDialog target={reachTarget} onClose={() => setReachTarget(null)} />
    </>
  );
}

function App() {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" storageKey="ww-theme" enableSystem>
      <WAProvider>
        <AppRoutes />
      </WAProvider>
    </ThemeProvider>
  );
}

export default App;
