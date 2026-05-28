import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export type Stat = {
  label: string;
  value: ReactNode;
  /** Small accent text in the corner (e.g. a status or delta). */
  hint?: string;
  tone?: "default" | "positive" | "negative";
};

// Static class strings so Tailwind keeps them at build time.
const COLS: Record<number, string> = {
  1: "lg:grid-cols-1",
  2: "lg:grid-cols-2",
  3: "lg:grid-cols-3",
  4: "lg:grid-cols-4",
  5: "lg:grid-cols-5",
};

/**
 * StatCards renders a segmented row of KPI cards (adapted from the blocks.so
 * stats-01 block): a label, a large value, and an optional accent hint.
 */
export function StatCards({ items }: { items: Stat[] }) {
  return (
    <div
      className={cn(
        "grid grid-cols-1 gap-2 sm:grid-cols-2",
        COLS[items.length] ?? "lg:grid-cols-3",
      )}
    >
      {items.map((stat) => (
        <Card key={stat.label} className="py-0">
          <CardContent className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2 p-4 sm:p-5">
            <div className="text-sm font-medium text-muted-foreground">{stat.label}</div>
            {stat.hint && (
              <div
                className={cn(
                  "text-xs font-medium tabular-nums",
                  stat.tone === "positive" && "text-green-700 dark:text-green-400",
                  stat.tone === "negative" && "text-red-700 dark:text-red-400",
                  (!stat.tone || stat.tone === "default") && "text-muted-foreground",
                )}
              >
                {stat.hint}
              </div>
            )}
            <div className="w-full flex-none text-2xl font-semibold tracking-tight tabular-nums text-foreground">
              {stat.value}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
