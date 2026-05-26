import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Page, ComingSoon } from "@/components/Page";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { ApprovalsService, Draft } from "@/lib/api";

export function Approvals() {
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [edits, setEdits] = useState<Record<number, string>>({});

  const load = () => {
    ApprovalsService.List()
      .then((d) => setDrafts(d ?? []))
      .catch((e) => toast.error("Couldn't load drafts", { description: String(e) }));
  };
  useEffect(load, []);

  const approve = (d: Draft) => {
    const text = edits[d.id] ?? "";
    ApprovalsService.Approve(d.id, text)
      .then(() => {
        toast("Reply sent");
        load();
      })
      .catch((e) => toast.error("Send failed", { description: String(e) }));
  };

  const reject = (d: Draft) => {
    ApprovalsService.Reject(d.id)
      .then(() => {
        toast("Draft rejected");
        load();
      })
      .catch((e) => toast.error("Reject failed", { description: String(e) }));
  };

  return (
    <Page
      title="Approvals"
      description="Replies the bot wants to send when it's unsure — approve, edit, or reject."
    >
      {drafts.length === 0 ? (
        <ComingSoon>No drafts awaiting review.</ComingSoon>
      ) : (
        <div className="grid gap-4">
          {drafts.map((d) => (
            <Card key={d.id}>
              <CardHeader className="gap-1">
                <div className="flex items-center justify-between">
                  <span className="font-medium">{d.senderName || d.chatJid}</span>
                  <Badge variant="outline">
                    {(d.confidence * 100).toFixed(0)}% conf
                  </Badge>
                </div>
                {d.reason && (
                  <p className="text-xs text-muted-foreground">{d.reason}</p>
                )}
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="rounded-md bg-muted/50 p-3 text-sm text-muted-foreground">
                  {d.incoming || "(no text)"}
                </div>
                <Textarea
                  value={edits[d.id] ?? d.reply}
                  onChange={(e) =>
                    setEdits((m) => ({ ...m, [d.id]: e.target.value }))
                  }
                  rows={3}
                />
                <div className="flex gap-2">
                  <Button onClick={() => approve(d)}>Approve &amp; send</Button>
                  <Button variant="outline" onClick={() => reject(d)}>
                    Reject
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </Page>
  );
}
