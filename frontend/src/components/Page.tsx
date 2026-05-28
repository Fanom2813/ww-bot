import type { ReactNode } from "react";
import { PageHeader } from "@/components/PageHeader";

interface PageProps {
  title: string;
  description?: string;
  actions?: ReactNode;
  children?: ReactNode;
  fill?: boolean;
}

export function Page({ title, description, actions, fill, children }: PageProps) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="shrink-0 px-4 pt-4 sm:px-6 sm:pt-6">
        <PageHeader title={title} description={description} actions={actions} />
      </div>
      <div
        className={
          fill
            ? "min-h-0 flex-1 flex flex-col overflow-hidden px-4 pt-1 pb-4 sm:px-6 sm:pb-6"
            : "min-h-0 flex-1 overflow-auto px-4 pt-1 pb-4 sm:px-6 sm:pb-6"
        }
      >
        {children}
      </div>
    </div>
  );
}
