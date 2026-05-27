import { useCallback, useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { WhatsAppService } from "../../bindings/wwbot";

export type WAStatus = "loading" | "unpaired" | "pairing" | "connected";

const DEBUG = import.meta.env.DEV;

function dbg(...args: unknown[]) {
  if (DEBUG) console.log("[wa]", ...args);
}

/**
 * useWhatsApp tracks the WhatsApp connection state by calling the bound
 * WhatsAppService and subscribing to backend "wa" events.
 */
export function useWhatsApp() {
  const [status, setStatus] = useState<WAStatus>("loading");
  const [qr, setQR] = useState("");
  const [jid, setJid] = useState("");

  useEffect(() => {
    dbg("init — checking IsPaired…");
    WhatsAppService.IsPaired()
      .then((paired) => {
        dbg("IsPaired =", paired);
        setStatus(paired ? "connected" : "unpaired");
        // The "paired" event only fires during a fresh QR scan, so on a normal
        // launch with an existing session we must fetch the number ourselves.
        if (paired) {
          WhatsAppService.JID()
            .then((j) => {
              dbg("JID =", j);
              if (j) setJid(j);
            })
            .catch((err) => dbg("JID error", err));
        }
      })
      .catch((err) => {
        dbg("IsPaired error", err);
        setStatus("unpaired");
      });

    const off = Events.On("wa", (e) => {
      const ev = e.data;
      dbg("event", ev.type, ev);
      switch (ev.type) {
        case "qr":
          setQR(ev.qr ?? "");
          setStatus("pairing");
          break;
        case "paired":
          if (ev.jid) setJid(ev.jid);
          break;
        case "connected":
          if (ev.jid) setJid(ev.jid);
          setStatus("connected");
          break;
        case "loggedout":
          setQR("");
          setJid("");
          setStatus("unpaired");
          break;
        case "expired":
          dbg("QR expired — resetting to unpaired");
          setQR("");
          setStatus("unpaired");
          break;
      }
    });

    return () => {
      dbg("cleanup — unsubscribing");
      off();
    };
  }, []);

  const startPairing = useCallback(() => {
    dbg("startPairing called");
    setStatus("pairing");
    WhatsAppService.StartPairing()
      .then(() => dbg("StartPairing OK"))
      .catch((err) => {
        dbg("StartPairing failed", err);
        setStatus("unpaired");
      });
  }, []);

  return { status, qr, jid, startPairing };
}
