import { useEffect, useRef } from "react";
import { Events } from "@wailsio/runtime";

/**
 * useBotSound plays a subtle notification chime whenever the backend fires the
 * "working" event — i.e. the bot took a non-silent action on an inbound message
 * (reply queued, draft created, or notify). Ignored / silent decisions don't
 * fire. Throttled so a burst of decisions stays gentle.
 *
 * Asset: public/sounds/notification.ogg ("Polite" from akx/Notifications, CC0).
 */
const SOUND_URL = "/sounds/notification.ogg";
const VOLUME = 0.35; // subtle
const THROTTLE_MS = 800;

export function useBotSound() {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const lastRef = useRef(0);

  useEffect(() => {
    // Preload once so the first play is instant.
    const a = new Audio(SOUND_URL);
    a.preload = "auto";
    a.volume = VOLUME;
    audioRef.current = a;

    const play = () => {
      const now = performance.now();
      if (now - lastRef.current < THROTTLE_MS) return;
      lastRef.current = now;
      try {
        // Rewind so back-to-back events still chime.
        const el = audioRef.current;
        if (!el) return;
        el.currentTime = 0;
        void el.play().catch(() => {
          // Autoplay can be blocked until first user gesture — silent failure is fine.
        });
      } catch {
        // ignore
      }
    };

    const off = Events.On("working", () => play());
    return () => off();
  }, []);
}
