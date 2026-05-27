import { useContext } from "react";
import { WAContext } from "@/lib/WAContext";
import type { WAStatus } from "@/lib/useWhatsApp";

export type { WAStatus };

export function useWA(): {
  status: WAStatus;
  qr: string;
  jid: string;
  online: boolean;
  startPairing: () => void;
} {
  const ctx = useContext(WAContext);
  if (!ctx) throw new Error("useWA must be used inside <WAProvider>");
  return ctx;
}
