"use client";

import type { ReactNode } from "react";

import { EmptyState } from "@/components/empty-state";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { tingkatOptions, type TingkatOption } from "@/lib/achievementsConfig";

type SchoolDetailRow = {
  school: string;
  counts: Record<TingkatOption, number>;
  total: number;
};

type PrestasiSchoolDetailTableProps = {
  rows: SchoolDetailRow[];
  showJenjangFilter: boolean;
  children?: ReactNode;
};

export function PrestasiSchoolDetailTable({
  rows,
  showJenjangFilter,
  children,
}: PrestasiSchoolDetailTableProps) {
  return (
    <Card>
      <CardContent className="space-y-4">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Rekapitulasi
            </p>
            <p className="text-sm font-semibold text-zinc-700">
              Detail prestasi berdasarkan sekolah
            </p>
          </div>
          {showJenjangFilter ? children : null}
        </div>

        <Table className="min-w-[720px] md:min-w-full">
          <TableHeader>
            <TableRow>
              <TableHead>Nama Sekolah</TableHead>
              {tingkatOptions.map((tingkat) => (
                <TableHead key={tingkat} className="text-right">
                  {tingkat}
                </TableHead>
              ))}
              <TableHead className="text-right">Total</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={tingkatOptions.length + 2}
                  className="py-4"
                >
                  <EmptyState
                    title="Tidak ada data prestasi"
                    description="Rekap per sekolah akan muncul setelah data tersedia."
                    className="min-h-24"
                  />
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row) => (
                <TableRow key={row.school}>
                  <TableCell className="font-semibold text-zinc-900">
                    {row.school}
                  </TableCell>
                  {tingkatOptions.map((tingkat) => {
                    const value = row.counts[tingkat];
                    return (
                      <TableCell key={tingkat} className="text-right">
                        {value > 0 ? value : "-"}
                      </TableCell>
                    );
                  })}
                  <TableCell className="text-right font-semibold text-zinc-900">
                    {row.total}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
