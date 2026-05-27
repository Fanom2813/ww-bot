import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuItem as SidebarMenuSubItem,
  SidebarMenuSubButton,
  useSidebar,
} from "@/components/ui/sidebar";
import { cn } from "@/lib/utils";
import { ChevronDown, ChevronUp } from "lucide-react";
import { NavLink, useLocation } from "react-router";
import type React from "react";
import { useState } from "react";

export type Route = {
  id: string;
  title: string;
  icon?: React.ReactNode;
  /** Router path, e.g. "/" or "/contacts". */
  to: string;
  /** Match the path exactly (use for the index route). */
  end?: boolean;
  subs?: {
    title: string;
    to: string;
    icon?: React.ReactNode;
  }[];
};

/** isRouteActive mirrors react-router's NavLink active logic. */
function isRouteActive(pathname: string, to: string, end?: boolean): boolean {
  return end ? pathname === to : pathname === to || pathname.startsWith(`${to}/`);
}

export default function DashboardNavigation({ routes }: { routes: Route[] }) {
  const { state } = useSidebar();
  const isCollapsed = state === "collapsed";
  const { pathname } = useLocation();
  const [openCollapsible, setOpenCollapsible] = useState<string | null>(null);

  return (
    <SidebarMenu>
      {routes.map((route) => {
        const isOpen = !isCollapsed && openCollapsible === route.id;
        const hasSubRoutes = !!route.subs?.length;
        const active = isRouteActive(pathname, route.to, route.end);

        return (
          <SidebarMenuItem key={route.id}>
            {hasSubRoutes ? (
              <Collapsible
                open={isOpen}
                onOpenChange={(open) =>
                  setOpenCollapsible(open ? route.id : null)
                }
                className="w-full"
              >
                <CollapsibleTrigger
                  render={
                    <SidebarMenuButton
                      className={cn(
                        "flex w-full items-center rounded-lg px-2 transition-colors",
                        isOpen
                          ? "bg-sidebar-muted text-foreground"
                          : "text-muted-foreground hover:bg-sidebar-muted hover:text-foreground",
                        isCollapsed && "justify-center"
                      )}
                    />
                  }
                >
                  {route.icon}
                  {!isCollapsed && (
                    <span className="ml-2 flex-1 text-sm font-medium">
                      {route.title}
                    </span>
                  )}
                  {!isCollapsed && (
                    <span className="ml-auto">
                      {isOpen ? (
                        <ChevronUp className="size-4" />
                      ) : (
                        <ChevronDown className="size-4" />
                      )}
                    </span>
                  )}
                </CollapsibleTrigger>

                {!isCollapsed && (
                  <CollapsibleContent>
                    <SidebarMenuSub className="my-1 ml-3.5 ">
                      {route.subs?.map((subRoute) => (
                        <SidebarMenuSubItem
                          key={`${route.id}-${subRoute.title}`}
                          className="h-auto"
                        >
                          <SidebarMenuSubButton
                            render={
                              <NavLink
                                to={subRoute.to}
                                className="flex items-center rounded-md px-4 py-1.5 text-sm font-medium text-muted-foreground hover:bg-sidebar-muted hover:text-foreground aria-[current=page]:bg-sidebar-muted aria-[current=page]:text-foreground"
                              />
                            }
                          >
                            {subRoute.title}
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ))}
                    </SidebarMenuSub>
                  </CollapsibleContent>
                )}
              </Collapsible>
            ) : (
              <SidebarMenuButton
                isActive={active}
                tooltip={route.title}
                render={
                  <NavLink
                    to={route.to}
                    end={route.end}
                    className={cn(
                      "flex items-center rounded-lg px-2 transition-colors text-muted-foreground hover:bg-sidebar-muted hover:text-foreground",
                      isCollapsed && "justify-center"
                    )}
                  />
                }
              >
                {route.icon}
                {!isCollapsed && (
                  <span className="ml-2 text-sm font-medium">{route.title}</span>
                )}
              </SidebarMenuButton>
            )}
          </SidebarMenuItem>
        );
      })}
    </SidebarMenu>
  );
}
