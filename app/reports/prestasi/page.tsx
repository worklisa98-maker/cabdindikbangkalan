"use client";

import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { apiFetch } from "@/lib/api/client";
import { ReportChartBox } from "@/components/reports/ReportChartBox";
import { ReportShell } from "@/components/reports/report-shell";
import { useReportExportMode } from "@/components/reports/use-report-export-mode";

type PrestasiAnalyticsPayload = {
  kpis?: {
    total?: number;
    currentYearTotal?: number;
    prevYearTotal?: number;
    yoyPercent?: number | null;
  };
  trendByYear?: { year: number; total: number }[];
  distribution?: { byTingkat?: { key: string; total: number }[] };
  topSchools?: { school_name: string; total: number }[];
};

const formatNumber = (value: number) => new Intl.NumberFormat("id-ID").format(value);

const fetchPrestasiReport = async (
  search: URLSearchParams
): Promise<PrestasiAnalyticsPayload> => {
  const query = new URLSearchParams();
  const year = search.get("year");
  const jenjang = search.get("jenjang");
  const statusSekolah = search.get("status_sekolah");
  const schoolId = search.get("school_id");
  const jenisLomba = search.get("jenis_lomba");

  if (year) query.set("year", year);
  if (jenjang) query.set("jenjang", jenjang);
  if (statusSekolah) query.set("status", statusSekolah);
  if (schoolId) query.set("school_id", schoolId);
  if (jenisLomba) query.set("jenis_lomba", jenisLomba);

  const response = await apiFetch(
    `/api/prestasi/analytics${query.toString() ? `?${query.toString()}` : ""}`
  );
  if (!response.ok) throw new Error("Failed to load prestasi report.");
  const payload = (await response.json()) as { data: PrestasiAnalyticsPayload };
  return payload.data ?? {};
};

export default function PrestasiReportPage() {
  const searchParams = useSearchParams();
  const queryKey = searchParams.toString();

  const { data, isLoading, isError } = useQuery({
    queryKey: ["report-prestasi", queryKey],
    queryFn: () => fetchPrestasiReport(new URLSearchParams(queryKey)),
    retry: 0,
  });

  const kpis = data?.kpis ?? {};
  const trendByYear = data?.trendByYear ?? [];
  const distribution = data?.distribution?.byTingkat ?? [];
  const topSchools = data?.topSchools ?? [];
  const ready = !isLoading;
  const { isExportMode, showWatermark } = useReportExportMode(searchParams, ready);

  return (
    <ReportShell
      title="Laporan Prestasi"
      subtitle="Ringkasan prestasi, tren tahunan, dan distribusi tingkat lomba."
      ready={ready}
      exportMode={isExportMode}
      showWatermark={showWatermark}
    >
      <section id="prestasi-kpi" className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs text-slate-500">Total Prestasi</p>
          <p className="text-xl font-semibold text-slate-900">{formatNumber(kpis.total ?? 0)}</p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs text-slate-500">Tahun Berjalan</p>
          <p className="text-xl font-semibold text-slate-900">
            {formatNumber(kpis.currentYearTotal ?? 0)}
          </p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs text-slate-500">Tahun Sebelumnya</p>
          <p className="text-xl font-semibold text-slate-900">
            {formatNumber(kpis.prevYearTotal ?? 0)}
          </p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs text-slate-500">YoY</p>
          <p className="text-xl font-semibold text-slate-900">
            {kpis.yoyPercent == null ? "-" : `${kpis.yoyPercent.toFixed(1)}%`}
          </p>
        </div>
      </section>

      {isError ? (
        <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
          Gagal memuat data laporan prestasi.
        </p>
      ) : null}

      <section id="prestasi-charts" className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Tren Prestasi Tahunan</p>
          <ReportChartBox className="mt-3" height={288}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={trendByYear}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="year" />
                <YAxis />
                <Tooltip />
                <Line
                  type="monotone"
                  dataKey="total"
                  stroke="#1d4ed8"
                  strokeWidth={2}
                  isAnimationActive={!isExportMode}
                />
              </LineChart>
            </ResponsiveContainer>
          </ReportChartBox>
        </div>

        <div className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Distribusi Tingkat Lomba</p>
          <ReportChartBox className="mt-3" height={288}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={distribution}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="key" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="total" fill="#0f766e" isAnimationActive={!isExportMode} />
              </BarChart>
            </ResponsiveContainer>
          </ReportChartBox>
        </div>
      </section>

      <section id="prestasi-top-sekolah" className="rounded-lg border border-slate-200 p-4">
        <p className="text-sm font-semibold text-slate-900">Top Sekolah</p>
        <div className="mt-3 overflow-hidden rounded-md border border-slate-200">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-left text-slate-600">
              <tr>
                <th className="px-3 py-2">Sekolah</th>
                <th className="px-3 py-2 text-right">Total</th>
              </tr>
            </thead>
            <tbody>
              {topSchools.length === 0 ? (
                <tr>
                  <td className="px-3 py-3 text-slate-500" colSpan={2}>
                    Tidak ada data.
                  </td>
                </tr>
              ) : (
                topSchools.map((row, index) => (
                  <tr key={`${row.school_name}-${index}`} className="border-t border-slate-100">
                    <td className="px-3 py-2 font-medium text-slate-900">{row.school_name}</td>
                    <td className="px-3 py-2 text-right text-slate-700">
                      {formatNumber(row.total)}
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
