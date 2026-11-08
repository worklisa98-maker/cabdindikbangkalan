"use client";

import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  jenjangOptions,
  tingkatOptions,
  type JenjangOption,
  type TingkatOption,
} from "@/lib/achievementsConfig";

type TableCounts = Record<JenjangOption, Record<TingkatOption, number>>;

type PrestasiDistributionTableProps = {
  tableCounts: TableCounts;
};

export function PrestasiDistributionTable({
  tableCounts,
}: PrestasiDistributionTableProps) {
  return (
    <Card>
      <CardContent className="space-y-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Rekapitulasi
          </p>
          <p className="text-sm font-semibold text-zinc-700">
            Distribusi prestasi berdasarkan jenjang dan tingkat
          </p>
        </div>

        <Table className="min-w-[720px] md:min-w-full">
          <TableHeader>
            <TableRow>
              <TableHead>Jenjang Sekolah</TableHead>
              {tingkatOptions.map((tingkat) => (
                <TableHead key={tingkat} className="text-right">
                  {tingkat}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {jenjangOptions.map((jenjang) => (
              <TableRow key={jenjang}>
                <TableCell className="font-semibold text-zinc-900">
                  {jenjang}
                </TableCell>
                {tingkatOptions.map((tingkat) => {
                  const value = tableCounts[jenjang]?.[tingkat] ?? 0;
                  return (
                    <TableCell key={tingkat} className="text-right">
                      {value > 0 ? value : "-"}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
