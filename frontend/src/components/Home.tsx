import { CheckCircle2 } from "lucide-react";

export function Home({ jid }: { jid: string }) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-background p-8 text-foreground">
      <CheckCircle2 className="h-10 w-10 text-green-500" />
      <h1 className="text-xl font-semibold">WhatsApp connected</h1>
      {jid && <p className="text-sm text-muted-foreground">{jid}</p>}
      <p className="text-xs text-muted-foreground">
        The bot is now listening. Dashboard coming next.
      </p>
    </div>
  );
}
