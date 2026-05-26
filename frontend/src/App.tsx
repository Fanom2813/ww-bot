import { Loader2 } from "lucide-react";
import { Toaster } from "@/components/ui/sonner";
import { useWhatsApp } from "@/lib/useWhatsApp";
import { useNotices } from "@/lib/useNotices";
import { ConnectScreen } from "@/components/ConnectScreen";
import { AppShell } from "@/components/AppShell";

function App() {
  const { status, qr, jid, startPairing } = useWhatsApp();
  useNotices();

  return (
    <>
      {status === "loading" ? (
        <div className="flex min-h-screen items-center justify-center bg-background text-foreground">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      ) : status === "connected" ? (
        <AppShell jid={jid} />
      ) : (
        <ConnectScreen status={status} qr={qr} onLink={startPairing} />
      )}
      <Toaster richColors position="top-right" />
    </>
  );
}

export default App;
