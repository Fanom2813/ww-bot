// Convenience re-exports of the generated Wails bindings so screens can import
// from "@/lib/api" instead of deep relative paths.
export {
  ActivityService,
  ApprovalsService,
  ContactsService,
  ControlService,
  ScheduleService,
  SettingsService,
  WhatsAppService,
} from "../../bindings/wwbot";

export { Contact, Draft, Activity } from "../../bindings/wwbot/internal/store/models";
export { WAContact } from "../../bindings/wwbot/internal/wa/models";
export {
  Settings,
  ProviderSetting,
  STTSetting,
  SafetySetting,
  ScheduledTask,
  PendingContact,
} from "../../bindings/wwbot/internal/core/models";
