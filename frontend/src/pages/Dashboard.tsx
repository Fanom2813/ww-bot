import { Page, ComingSoon } from "@/components/Page";

export function Dashboard() {
  return (
    <Page
      title="Dashboard"
      description="Connection, active AI backend, free-quota, and recent activity at a glance."
    >
      <ComingSoon>
        Status overview, today&apos;s context, and the pending-approvals count
        will live here.
      </ComingSoon>
    </Page>
  );
}
