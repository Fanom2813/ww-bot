import { Page, ComingSoon } from "@/components/Page";

export function Approvals() {
  return (
    <Page
      title="Approvals"
      description="Draft replies the bot wants to send when it's unsure — approve, edit, or reject."
    >
      <ComingSoon>
        Pending drafts (with the incoming message, proposed reply, and
        confidence) and flagged items will appear here.
      </ComingSoon>
    </Page>
  );
}
