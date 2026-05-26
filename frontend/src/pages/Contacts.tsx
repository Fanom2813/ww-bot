import { Page, ComingSoon } from "@/components/Page";

export function Contacts() {
  return (
    <Page
      title="Contacts"
      description="Per-person rules: who they are, language, reply style, trust tier, and memory."
    >
      <ComingSoon>
        Your contacts and their profiles (Dad, Mom, friends…) with trust tiers
        — auto-send, draft-and-approve, or notify-only — will be managed here.
      </ComingSoon>
    </Page>
  );
}
