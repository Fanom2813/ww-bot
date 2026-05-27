import { createContext, type ReactNode } from "react";
import { useWhatsApp, type WAStatus } from "@/lib/useWhatsApp";
import { useNotices } from "@/lib/useNotices";

interface WAContextValue {
  status: WAStatus;
  qr: string;
  jid: string;
  startPairing: () => void;
}

export const WAContext = createContext<WAContextValue | null>(null);

export function WAProvider({ children }: { children: ReactNode }) {
  const wa = useWhatsApp();
  useNotices();
  return <WAContext.Provider value={wa}>{children}</WAContext.Provider>;
}
