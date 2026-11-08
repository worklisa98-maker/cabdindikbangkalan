"use client";

import { useMemo } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { apiFetch } from "@/lib/api/client";
import { ReportChartBox } from "@/components/reports/ReportChartBox";
import { ReportShell } from "@/components/reports/report-shell";
import { useReportExportMode } from "@/components/reports/use-report-export-mode";

type PTRankingItem = {
  ptn_name: string;
  count: number;
};

const formatNumber = (value: number) => new Intl.NumberFormat("id-ID").format(value);

const fetchPTRanking = async (search: URLSearchParams): Promise<PTRankingItem[]> => {
  const query = new URLSearchParams();
  ["year", "jenjang", "status_sekolah", "school_id"].forEach((key) => {
    const value = search.get(key);
    if (value) query.set(key, value);
  });

  const response = await apiFetch(
    `/api/siswa/pt/ranking${query.toString() ? `?${query.toString()}` : ""}`
  );
  if (!response.ok) throw new Error("Failed to load PT ranking report.");
  const payload = (await response.json()) as { data: PTRankingItem[] };
  return payload.data ?? [];
};

export default function PTRankingReportPage() {
  const searchParams = useSearchParams();
  const queryKey = searchParams.toString();

  const { data = [], isLoading, isError } = useQuery({
    queryKey: ["report-pt-ranking", queryKey],
    queryFn: () => fetchPTRanking(new URLSearchParams(queryKey)),
    retry: 0,
  });

  const top20 = useMemo(() => data.slice(0, 20), [data]);
  const ready = !isLoading;
  const { isExportMode, showWatermark } = useReportExportMode(searchParams, ready);

  return (
    <ReportShell
      title="Laporan Peringkat PT"
      subtitle="Peringkat keseluruhan perguruan tinggi berdasarkan total siswa diterima."
      ready={ready}
      exportMode={isExportMode}
      showWatermark={showWatermark}
    >
      {isError ? (
        <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
          Gagal memuat data peringkat PT.
        </p>
      ) : null}

      <section id="pt-ranking-chart" className="rounded-lg border border-slate-200 p-4">
        <p className="text-sm font-semibold text-slate-900">Top 20 PTN</p>
        <ReportChartBox className="mt-3" height={384}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={top20} layout="vertical" margin={{ left: 120, right: 16 }}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis type="number" />
              <YAxis dataKey="ptn_name" type="category" width={240} />
              <Tooltip />
              <Bar dataKey="count" fill="#1d4ed8" isAnimationActive={!isExportMode} />
            </BarChart>
          </ResponsiveContainer>
        </ReportChartBox>
      </section>

      <section id="pt-ranking-table" className="rounded-lg border border-slate-200 p-4">
        <p className="text-sm font-semibold text-slate-900">Ranking Lengkap</p>
        <div className="mt-3 overflow-hidden rounded-md border border-slate-200">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-left text-slate-600">
              <tr>
                <th className="px-3 py-2">No</th>
                <th className="px-3 py-2">PTN</th>
                <th className="px-3 py-2 text-right">Total Diterima</th>
              </tr>
            </thead>
            <tbody>
              {data.length === 0 ? (
                <tr>
                  <td className="px-3 py-3 text-slate-500" colSpan={3}>
                    Tidak ada data.
                  </td>
                </tr>
              ) : (
                data.map((row, index) => (
                  <tr key={`${row.ptn_name}-${index}`} className="border-t border-slate-100">
                    <td className="px-3 py-2 text-slate-600">{index + 1}</td>
                    <td className="px-3 py-2 font-medium text-slate-900">{row.ptn_name}</td>
                    <td className="px-3 py-2 text-right text-slate-700">
                      {formatNumber(row.count)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </ReportShell>
  );
}
