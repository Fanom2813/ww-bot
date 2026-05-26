import { Loader2 } from "lucide-react";
import { useWhatsApp } from "@/lib/useWhatsApp";
import { ConnectScreen } from "@/components/ConnectScreen";
import { AppShell } from "@/components/AppShell";

function App() {
  const { status, qr, jid, startPairing } = useWhatsApp();

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-foreground">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (status === "connected") {
    return <AppShell jid={jid} />;
  }

  return <ConnectScreen status={status} qr={qr} onLink={startPairing} />;
}

export default App;
