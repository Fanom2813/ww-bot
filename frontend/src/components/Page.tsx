import type { ReactNode } from "react";

interface PageProps {
  title: string;
  description?: string;
  children?: ReactNode;
}

/** Page is the shared header + content wrapper for routed screens. */
export function Page({ title, description, children }: PageProps) {
  return (
    <div className="mx-auto max-w-4xl p-8">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description && (
          <p className="mt-1 text-sm text-muted-foreground">{description}</p>
        )}
      </header>
      {children}
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
