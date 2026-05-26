import { Page, ComingSoon } from "@/components/Page";

export function Activity() {
  return (
    <Page
      title="Activity"
      description="A log of what the bot did — messages sent, calls notified, scam flags."
    >
      <ComingSoon>
        An audit trail of the bot&apos;s actions (metadata only, never a raw chat
        archive) will appear here.
      </ComingSoon>
    </Page>
  );
}
