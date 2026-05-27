import type { ReactNode } from "react";
import { SidebarTrigger } from "@/components/ui/sidebar";

interface PageHeaderProps {
  title: string;
  description?: string;
  /** Right-aligned controls (buttons, filters, …). */
  actions?: ReactNode;
}

/**
 * PageHeader is the reusable title block for routed screens: a title, an
 * optional description, an optional actions slot, and a sidebar trigger that
 * shows on mobile (where the sidebar is an off-canvas sheet).
 */
export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <header className="mb-6 flex items-start gap-3">
      <SidebarTrigger className="-ml-1 mt-1 md:hidden" />
      <div className="min-w-0 flex-1">
        <h1 className="truncate text-2xl font-semibold tracking-tight">{title}</h1>
        {description && (
          <p className="mt-1 text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </header>
  );
}

interface PageProps extends PageHeaderProps {
  children?: ReactNode;
}

/**
 * Page is the shared header + content wrapper for routed screens. It fills the
 * inset height and keeps the header pinned while the content area scrolls.
 */
export function Page({ title, description, actions, children }: PageProps) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="shrink-0 px-4 pt-4 sm:px-6 sm:pt-6">
        <PageHeader title={title} description={description} actions={actions} />
      </div>
      <div className="min-h-0 flex-1 overflow-auto px-4 pt-1 pb-4 sm:px-6 sm:pb-6">
        {children}
      </div>
    </div>
  );
}

/** Placeholder block used by not-yet-built screens. */
export function ComingSoon({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}
