import { Route, Routes } from "react-router";
import { ThemeProvider } from "next-themes";
import { Toaster } from "@/components/ui/sonner";
import { WAProvider } from "@/lib/WAContext";
import { useWA } from "@/lib/useWA";
import { RequireOnboarded, RequireConnected } from "@/components/RouteGuards";
import { Onboarding } from "@/components/Onboarding";
import { ConnectScreen } from "@/components/ConnectScreen";
import { AppLayout } from "@/components/AppLayout";
import { NewContactPrompt } from "@/components/dialogs";
import { Dashboard } from "@/pages/Dashboard";
import { Approvals } from "@/pages/Approvals";
import { Contacts } from "@/pages/Contacts";
import { Activity } from "@/pages/Activity";
import { Settings } from "@/pages/Settings";

function AppRoutes() {
  const { jid, online } = useWA();

  return (
    <>
      <Routes>
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
              <Route path="activity" element={<Activity />} />
              <Route path="settings" element={<Settings />} />
            </Route>
          </Route>
        </Route>
      </Routes>
      <Toaster richColors position="top-right" />
      <NewContactPrompt />
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
