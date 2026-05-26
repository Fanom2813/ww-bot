// Convenience re-exports of the generated Wails bindings so screens can import
// from "@/lib/api" instead of deep relative paths.
export {
  ActivityService,
  ApprovalsService,
  ContactsService,
  ControlService,
  SettingsService,
  WhatsAppService,
} from "../../bindings/wwbot";

export { Contact, Draft, Activity } from "../../bindings/wwbot/internal/store/models";
export {
  Settings,
  ProviderSetting,
  STTSetting,
  SafetySetting,
} from "../../bindings/wwbot/internal/core/models";
