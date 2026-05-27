import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar";
import { cn } from "@/lib/utils";
import { ChevronsUpDown, LogOut } from "lucide-react";

type Props = {
  jid: string;
  paused: boolean;
  onLogout: () => void;
};

/** NavUser is the sidebar footer: the account row opens a dropdown with Log out. */
export function NavUser({ jid, paused, onLogout }: Props) {
  const { state } = useSidebar();
  const isCollapsed = state === "collapsed";
  const number = jid ? shortJid(jid) : "Unknown";

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <SidebarMenuButton
                size="lg"
                tooltip={number}
                className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
              />
            }
          >
            <span className="flex aspect-square size-8 shrink-0 items-center justify-center">
              <span
                className={cn(
                  "size-2.5 rounded-full",
                  paused ? "bg-yellow-500" : "bg-green-500",
                )}
              />
            </span>
            {!isCollapsed && (
              <>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-medium">{number}</span>
                  <span className="truncate text-xs text-muted-foreground">
                    {paused ? "Paused" : "Connected"}
                  </span>
                </div>
                <ChevronsUpDown className="ml-auto size-4" />
              </>
            )}
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
            align="start"
            side="top"
            sideOffset={4}
          >
            <DropdownMenuGroup>
              <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">
                {number}
              </DropdownMenuLabel>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={onLogout}
              className="gap-2 text-destructive focus:text-destructive"
            >
              <LogOut className="size-4" />
              Log out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}

/** shortJid turns "1234567890:12@s.whatsapp.net" into "+1234567890". */
function shortJid(jid: string): string {
  const num = jid.split("@")[0].split(":")[0];
  return num ? `+${num}` : jid;
}
