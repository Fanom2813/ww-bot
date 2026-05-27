import { Navigate } from "react-router";
import { QRCodeSVG } from "qrcode.react";
import { Loader2, Smartphone } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useWA } from "@/lib/useWA";

export function ConnectScreen() {
  const { status, qr, startPairing } = useWA();

  // Already connected — redirect into the guarded app
  if (status === "connected") return <Navigate to="/" replace />;

  const showQR = status === "pairing" && qr !== "";
  const generating = status === "pairing" && qr === "";

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-8 text-foreground">
      <Card className="w-full max-w-md">
        <CardHeader className="items-center text-center">
          <img src="/logo.png" alt="WW Bot" className="mx-auto mb-2 h-12 w-12 rounded-xl" />
          <CardTitle>Link your WhatsApp</CardTitle>
          <CardDescription>
            Connect the bot to your WhatsApp account to get started.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-5">
          {showQR && (
            <>
              <div className="rounded-lg bg-white p-4">
                <QRCodeSVG value={qr} size={232} level="L" />
              </div>
              <ol className="space-y-1 text-sm text-muted-foreground">
                <li>1. Open WhatsApp on your phone</li>
                <li>2. Settings → Linked Devices → Link a Device</li>
                <li>3. Scan this code</li>
              </ol>
            </>
          )}

          {generating && (
            <div className="flex items-center gap-2 py-10 text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" /> Generating QR code…
            </div>
          )}

          {status === "unpaired" && (
            <Button onClick={startPairing} className="gap-2">
              <Smartphone className="h-4 w-4" /> Show QR code
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
