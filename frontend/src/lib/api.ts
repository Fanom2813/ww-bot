// Convenience re-exports of the generated Wails bindings so screens can import
// from "@/lib/api" instead of deep relative paths.
export {
  ActivityService,
  ApprovalsService,
  ContactsService,
  ControlService,
  GroupsService,
  ScheduleService,
  SettingsService,
  UpdaterService,
  WhatsAppService,
} from "../../bindings/wwbot";

export { Contact, Draft, Activity, Group } from "../../bindings/wwbot/internal/store/models";
export { WAContact, WAGroup } from "../../bindings/wwbot/internal/wa/models";
export {
  Settings,
  ProviderSetting,
  STTSetting,
  SafetySetting,
  ScheduledTask,
  PendingContact,
} from "../../bindings/wwbot/internal/core/models";
