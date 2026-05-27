import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarTrigger,
  useSidebar,
} from "@/components/ui/sidebar";
import { cn } from "@/lib/utils";
import {
  Activity as ActivityIcon,
  Inbox,
  LayoutDashboard,
  Settings as SettingsIcon,
  Users,
} from "lucide-react";
import type { Route } from "@/components/sidebar-02/nav-main";
import DashboardNavigation from "@/components/sidebar-02/nav-main";
import { NavUser } from "@/components/sidebar-02/nav-user";

const dashboardRoutes: Route[] = [
  { id: "dashboard", title: "Dashboard", icon: <LayoutDashboard className="size-4" />, to: "/", end: true },
  { id: "approvals", title: "Approvals", icon: <Inbox className="size-4" />, to: "/approvals" },
  { id: "contacts", title: "Contacts", icon: <Users className="size-4" />, to: "/contacts" },
  { id: "activity", title: "Activity", icon: <ActivityIcon className="size-4" />, to: "/activity" },
  { id: "settings", title: "Settings", icon: <SettingsIcon className="size-4" />, to: "/settings" },
];

type Props = {
  jid: string;
  paused: boolean;
  isMac: boolean;
  onLogout: () => void;
};

export function DashboardSidebar({ jid, paused, isMac, onLogout }: Props) {
  const { state } = useSidebar();
  const isCollapsed = state === "collapsed";

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarHeader
        className={cn(
          "flex",
          // Collapsed: stack logo over the trigger and center them in the rail.
          // Expanded: logo + name on the left, trigger on the right.
          isCollapsed
            ? "flex-col items-center gap-2"
            : "flex-row items-center justify-between",
          // On macOS the hidden-inset title bar puts traffic lights at the top
          // left, so the logo needs to clear them (md:pt-3.5 would otherwise win
          // at desktop width and leave it tucked under the buttons).
          isMac ? "pt-12" : "md:pt-3.5",
        )}
      >
        <a className="flex items-center gap-2 overflow-hidden">
          <img src="/logo.png" alt="WW Bot" className="size-8 shrink-0 rounded-lg" />
          {!isCollapsed && <span className="font-semibold">WW Bot</span>}
        </a>
        <SidebarTrigger />
      </SidebarHeader>

      <SidebarContent className="gap-4 px-2 py-4">
        <DashboardNavigation routes={dashboardRoutes} />
      </SidebarContent>

      <SidebarFooter className="px-2">
        <NavUser jid={jid} paused={paused} onLogout={onLogout} />
      </SidebarFooter>
    </Sidebar>
  );
}
