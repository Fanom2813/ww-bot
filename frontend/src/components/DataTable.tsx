import type { ReactNode } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { ListFilter, Search } from "lucide-react";

export type Column<T> = {
  /** Stable identifier for the column. */
  id: string;
  header: ReactNode;
  /** Renders the cell for a given row. */
  cell: (row: T) => ReactNode;
  /** Extra classes for the body cell (e.g. alignment, width). */
  className?: string;
  /** Extra classes for the header cell. */
  headClassName?: string;
};

type SearchProps = {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
};

type FilterProps = {
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
  /** Button label when the "all"/first option is selected. Defaults to "Filter". */
  label?: string;
};

type DataTableProps<T> = {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  /** Optional row click handler. */
  onRowClick?: (row: T) => void;
  /** Renders an optional search box above the table. */
  search?: SearchProps;
  /** Renders an optional filter button (with a popover menu) above the table. */
  filter?: FilterProps;
  /** Shown when there are no rows. */
  empty?: ReactNode;
  loading?: boolean;
};

/**
 * DataTable is a generic, presentational table: pass typed columns + rows and
 * it handles the chrome (optional search box, bordered table, empty state).
 */
export function DataTable<T>({
  columns,
  rows,
  rowKey,
  onRowClick,
  search,
  filter,
  empty,
  loading,
}: DataTableProps<T>) {
  const activeFilter =
    filter && (filter.options.find((o) => o.value === filter.value)?.label ?? filter.label ?? "Filter");

  return (
    <div>
      {(search || filter) && (
        <div className="mb-3 flex items-center gap-2">
          {search && (
            <div className="relative max-w-sm flex-1">
              <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-8"
                placeholder={search.placeholder ?? "Search…"}
                value={search.value}
                onChange={(e) => search.onChange(e.target.value)}
              />
            </div>
          )}
          {filter && (
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button variant="outline" size="sm" className="gap-1.5" />}>
                <ListFilter className="size-4" />
                {activeFilter}
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-40">
                <DropdownMenuRadioGroup value={filter.value} onValueChange={filter.onChange}>
                  {filter.options.map((o) => (
                    <DropdownMenuRadioItem key={o.value} value={o.value}>
                      {o.label}
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      )}

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              {columns.map((col) => (
                <TableHead key={col.id} className={col.headClassName}>
                  {col.header}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={rowKey(row)}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
                className={onRowClick ? "cursor-pointer" : undefined}
              >
                {columns.map((col) => (
                  <TableCell key={col.id} className={col.className}>
                    {col.cell(row)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
            {rows.length === 0 && (
              <TableRow className="hover:bg-transparent">
                <TableCell
                  colSpan={columns.length}
                  className={cn("py-10 text-center text-sm text-muted-foreground")}
                >
                  {loading ? "Loading…" : (empty ?? "Nothing here yet.")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
