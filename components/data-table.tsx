"use client";

import type { ReactNode } from "react";

import { EmptyState } from "@/components/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

type ColumnHeader = {
  key: string;
  label: string;
  align?: "left" | "right" | "center";
  className?: string;
};

type DataTableProps<T> = {
  columns: ColumnHeader[];
  rows: T[];
  renderRow: (row: T) => ReactNode;
  isLoading: boolean;
  isError: boolean;
  emptyMessage?: string;
  tableWrapperClassName?: string;
  tableHeaderClassName?: string;
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
};

export function DataTable<T>({
  columns,
  rows,
  renderRow,
  isLoading,
  isError,
  emptyMessage,
  tableWrapperClassName,
  tableHeaderClassName,
  page,
  pageSize,
  total,
  onPageChange,
}: DataTableProps<T>) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const canPrev = page > 1;
  const canNext = page < totalPages;

  return (
    <div className="space-y-3">
      <Table
        wrapperClassName={cn("overflow-x-auto", tableWrapperClassName)}
        className="min-w-[720px] md:min-w-full"
      >
        <TableHeader className={tableHeaderClassName}>
          <TableRow>
            {columns.map((column) => (
              <TableHead
                key={column.key}
                className={
                  column.align === "right"
                    ? `text-right ${column.className ?? ""}`
                    : column.align === "center"
                      ? `text-center ${column.className ?? ""}`
                      : column.className
                }
              >
                {column.label}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell colSpan={columns.length} className="py-4">
                <div className="space-y-2">
                  <Skeleton className="h-4 w-full rounded-md border-0" />
                  <Skeleton className="h-4 w-5/6 rounded-md border-0" />
                  <Skeleton className="h-4 w-2/3 rounded-md border-0" />
                </div>
              </TableCell>
            </TableRow>
          ) : isError ? (
            <TableRow>
              <TableCell
                colSpan={columns.length}
                className="py-4"
              >
                <EmptyState
                  title="Gagal memuat data"
                  description="Silakan coba lagi atau periksa filter yang digunakan."
                  className="min-h-24 border-red-200 bg-red-50/30"
                />
              </TableCell>
            </TableRow>
          ) : rows.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={columns.length}
                className="py-4"
              >
                <EmptyState
                  title={emptyMessage ?? "Belum ada data."}
                  description="Tidak ada baris yang cocok dengan filter saat ini."
                  className="min-h-24"
                />
              </TableCell>
            </TableRow>
          ) : (
            rows.map((row) => renderRow(row))
          )}
        </TableBody>
      </Table>

      <div className="flex flex-col gap-2 rounded-xl border border-slate-200/60 bg-white px-4 py-3 text-sm text-slate-600 sm:flex-row sm:items-center sm:justify-between">
        <p>
          Halaman {page} dari {totalPages} (total {total} data)
        </p>
        <div className="flex flex-col gap-2 sm:flex-row">
          <button
            type="button"
            disabled={!canPrev}
            onClick={() => onPageChange(page - 1)}
            className="btn-secondary w-full px-3 py-1.5 text-sm font-semibold sm:w-auto"
          >
            Sebelumnya
          </button>
          <button
            type="button"
            disabled={!canNext}
            onClick={() => onPageChange(page + 1)}
            className="btn-secondary w-full px-3 py-1.5 text-sm font-semibold sm:w-auto"
          >
            Berikutnya
          </button>
        </div>
      </div>
    </div>
  );
}
