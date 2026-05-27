import { Navigate, Outlet } from "react-router";
import { Loader2 } from "lucide-react";
import { useWA } from "@/lib/useWA";

const ONBOARDED_KEY = "ww-onboarded";

export function RequireOnboarded() {
  const onboarded = localStorage.getItem(ONBOARDED_KEY) === "1";
  if (!onboarded) return <Navigate to="/onboarding" replace />;
  return <Outlet />;
}

export function RequireConnected() {
  const { status } = useWA();
  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-foreground">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    );
  }
  if (status !== "connected") return <Navigate to="/connect" replace />;
  return <Outlet />;
}
