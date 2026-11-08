"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";

type Coverage = {
  graduationYear: number;
  path: string;
  officialCount: number;
  detailCount: number;
  missingCount: number;
  coveragePercent: number;
  exceedsOfficial: boolean;
};

export default function PTOfficialCoveragePanel() {
  const [year, setYear] = useState(new Date().getFullYear());
  const { data = [] } = useQuery({
    queryKey: ["pt-official-coverage-panel", year],
    queryFn: async (): Promise<Coverage[]> => {
      const response = await apiFetch(
        `/api/siswa/acceptance-offers/official-counts?year=${year}`
      );
      if (!response.ok) return [];
      const payload = await response.json();
      return Array.isArray(payload?.data) ? payload.data : [];
    },
  });
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-xl font-semibold">
            Kontrol Kelengkapan Penerimaan per Jalur
          </h2>
          <p className="text-sm text-zinc-600">
            Angka resmi adalah kontrol detail jalur, bukan total siswa unik PT.
          </p>
        </div>
        <input
          type="number"
          value={year}
          onChange={(e) => setYear(Number(e.target.value))}
          className="rounded-md border px-3 py-2 text-sm"
        />
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        {["SNBP", "SNBT"].map((path) => {
          const item = data.find((row) => row.path === path);
          return (
            <Card key={path}>
              <CardContent className="space-y-2 p-5">
                <h3 className="font-semibold">
                  {path} {year}
                </h3>
                <div className="grid grid-cols-2 gap-3 text-sm">
                  <div>
                    <p className="text-zinc-500">Penerimaan resmi</p>
                    <p className="text-2xl font-semibold">
                      {item?.officialCount ?? 0}
                    </p>
                  </div>
                  <div>
                    <p className="text-zinc-500">Record/detail terinput</p>
                    <p className="text-2xl font-semibold">
                      {item?.detailCount ?? 0}
                    </p>
                  </div>
                  <div>
                    <p className="text-zinc-500">Kekurangan detail</p>
                    <p className="text-xl font-semibold">
                      {item?.missingCount ?? 0}
                    </p>
                  </div>
                  <div>
                    <p className="text-zinc-500">Cakupan jalur</p>
                    <p className="text-xl font-semibold">
                      {(item?.coveragePercent ?? 0).toFixed(1)}%
                    </p>
                  </div>
                </div>
                {item?.exceedsOfficial ? (
                  <p className="rounded-md bg-amber-50 p-2 text-sm font-medium text-amber-800">
                    Detail terinput melebihi jumlah resmi dan perlu diperiksa.
                  </p>
                ) : null}
              </CardContent>
            </Card>
          );
        })}
      </div>
    </section>
  );
}
