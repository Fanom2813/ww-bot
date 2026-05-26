import { useEffect } from "react";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";

// useNotices surfaces backend "notice" events as toasts (draft-needs-review,
// scam/sensitive flags, calls, errors).
export function useNotices() {
  useEffect(() => {
    const off = Events.On("notice", (e) => {
      const n = e.data as { level?: string; title?: string; body?: string };
      const title = n.title ?? "Notice";
      const opts = n.body ? { description: n.body } : undefined;
      if (n.level === "error") toast.error(title, opts);
      else toast(title, opts);
    });
    return () => off();
  }, []);
}
