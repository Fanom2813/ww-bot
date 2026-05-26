import { Page, ComingSoon } from "@/components/Page";

export function Settings() {
  return (
    <Page
      title="Settings"
      description="AI backends, voice transcription, anti-ban safety, privacy, and general options."
    >
      <ComingSoon>
        Tabs for AI backends (ordered by you), voice/STT keys, safety &amp;
        pacing, privacy, and general options will live here.
      </ComingSoon>
    </Page>
  );
}
