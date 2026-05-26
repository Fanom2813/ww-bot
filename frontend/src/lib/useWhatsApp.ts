import { useCallback, useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { WhatsAppService } from "../../bindings/wwbot";

export type WAStatus = "loading" | "unpaired" | "pairing" | "connected";

/**
 * useWhatsApp tracks the WhatsApp connection state by calling the bound
 * WhatsAppService and subscribing to backend "wa" events.
 */
export function useWhatsApp() {
  const [status, setStatus] = useState<WAStatus>("loading");
  const [qr, setQR] = useState("");
  const [jid, setJid] = useState("");

  useEffect(() => {
    // If a session already exists the backend auto-reconnects, so treat as connected.
    WhatsAppService.IsPaired()
      .then((paired) => setStatus(paired ? "connected" : "unpaired"))
      .catch(() => setStatus("unpaired"));

    const off = Events.On("wa", (e) => {
      const ev = e.data;
      switch (ev.type) {
        case "qr":
          setQR(ev.qr ?? "");
          setStatus("pairing");
          break;
        case "paired":
          if (ev.jid) setJid(ev.jid);
          break;
        case "connected":
          setStatus("connected");
          break;
        case "loggedout":
          setQR("");
          setJid("");
          setStatus("unpaired");
          break;
      }
    });

    return () => off();
  }, []);

  const startPairing = useCallback(() => {
    setStatus("pairing");
    WhatsAppService.StartPairing().catch((err) => {
      console.error("StartPairing failed:", err);
      setStatus("unpaired");
    });
  }, []);

  return { status, qr, jid, startPairing };
}
