"use client";

import { useMemo } from "react";
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

type AnalyticsPTPayload = {
  series?: { tahun_lulus: number; jalur: string; pendaftar: number; diterima: number }[];
  totals_by_year?: { tahun_lulus: number; pendaftar: number; diterima: number }[];
};

type OutcomeDashboardPayload = {
  kpis?: {
    currentYear?: number;
    currentYearTotal?: number;
    prevYear?: number;
    prevYearTotal?: number;
    yoyPercent?: number | null;
  };
  trendByYear?: { year: number; total: number }[];
  breakdownJalur?: { label: string; total: number; sharePercent: number }[];
  topKampus?: { label: string; total: number }[];
};

const formatNumber = (value: number) => new Intl.NumberFormat("id-ID").format(value);

const resolveTabLabel = (tab: string) => {
  switch (tab) {
    case "analytics":
      return "Analytics PT";
    case "pt":
      return "Perguruan Tinggi";
    case "kerja":
      return "Dunia Kerja";
    case "wirausaha":
      return "Wirausaha";
    case "belum":
      return "Belum Melanjutkan";
    default:
      return "Perguruan Tinggi";
  }
};

const fetchAnalyticsPT = async (search: URLSearchParams): Promise<AnalyticsPTPayload> => {
  const query = new URLSearchParams();
  const now = new Date().getFullYear();
  query.set("start_year", search.get("start_year") ?? String(now - 4));
  query.set("end_year", search.get("end_year") ?? String(now));
  const jenjang = search.get("jenjang");
  if (jenjang) query.set("jenjang", jenjang);
  const schoolId = search.get("school_id");
  if (schoolId) query.set("school_id", schoolId);

  const response = await apiFetch(`/api/siswa/status-lulusan/analytics-pt?${query.toString()}`);
  if (!response.ok) throw new Error("Failed to load analytics PT report.");
  const payload = (await response.json()) as { data: AnalyticsPTPayload };
  return payload.data ?? {};
};

const fetchOutcomeDashboard = async (
  tab: string,
  search: URLSearchParams
): Promise<OutcomeDashboardPayload> => {
  const statusMap: Record<string, string> = {
    pt: "PT",
    kerja: "KERJA",
    wirausaha: "WIRAUSAHA",
    belum: "BELUM",
  };

  const query = new URLSearchParams();
  query.set("status", statusMap[tab] ?? "PT");
  const year = search.get("year");
  const jenjang = search.get("jenjang");
  const schoolId = search.get("school_id");
  const jalur = search.get("jalur");
  const statusSekolah = search.get("status_sekolah");
  if (year) query.set("year", year);
  if (jenjang) query.set("jenjang", jenjang);
  if (schoolId) query.set("school_id", schoolId);
  if (jalur) query.set("jalur", jalur);
  if (statusSekolah) query.set("status_sekolah", statusSekolah);

  const response = await apiFetch(`/api/siswa/outcomes/dashboard?${query.toString()}`);
  if (!response.ok) throw new Error("Failed to load outcome dashboard report.");
  const payload = (await response.json()) as { data: OutcomeDashboardPayload };
  return payload.data ?? {};
};

export default function SiswaReportPage() {
  const searchParams = useSearchParams();
  const queryKey = searchParams.toString();
  const tab = (searchParams.get("tab") ?? "analytics").toLowerCase();
  const isAnalyticsTab = tab === "analytics";

  const {
    data: analyticsPT,
    isLoading: isAnalyticsLoading,
    isError: isAnalyticsError,
  } = useQuery({
    queryKey: ["report-siswa-analytics", queryKey],
    queryFn: () => fetchAnalyticsPT(new URLSearchParams(queryKey)),
    enabled: isAnalyticsTab,
    retry: 0,
  });

  const {
    data: dashboard,
    isLoading: isDashboardLoading,
    isError: isDashboardError,
  } = useQuery({
    queryKey: ["report-siswa-dashboard", tab, queryKey],
    queryFn: () => fetchOutcomeDashboard(tab, new URLSearchParams(queryKey)),
    enabled: !isAnalyticsTab,
    retry: 0,
  });

  const analyticsChartData = useMemo(() => {
    const rows = analyticsPT?.totals_by_year ?? [];
    return rows.map((row) => ({
      tahun: row.tahun_lulus,
      pendaftar: row.pendaftar,
      diterima: row.diterima,
    }));
  }, [analyticsPT?.totals_by_year]);

  const dashboardTrend = dashboard?.trendByYear ?? [];
  const breakdown = dashboard?.breakdownJalur ?? [];

  const ready = isAnalyticsTab ? !isAnalyticsLoading : !isDashboardLoading;
  const { isExportMode, showWatermark } = useReportExportMode(searchParams, ready);

  return (
    <ReportShell
      title={`Laporan Siswa - ${resolveTabLabel(tab)}`}
      subtitle="Ringkasan tab siswa sesuai filter yang aktif."
      ready={ready}
      exportMode={isExportMode}
      showWatermark={showWatermark}
    >
      {isAnalyticsTab ? (
        <>
          {isAnalyticsError ? (
            <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
              Gagal memuat analytics PT.
            </p>
          ) : null}

          <section id="siswa-analytics-pt" className="rounded-lg border border-slate-200 p-4">
            <p className="text-sm font-semibold text-slate-900">Tren Pendaftar vs Diterima</p>
            <ReportChartBox className="mt-3" height={320}>
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={analyticsChartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="tahun" />
                  <YAxis />
                  <Tooltip />
                  <Line
                    type="monotone"
                    dataKey="pendaftar"
                    stroke="#0ea5e9"
                    strokeWidth={2}
                    isAnimationActive={!isExportMode}
                  />
                  <Line
                    type="monotone"
                    dataKey="diterima"
                    stroke="#0f766e"
                    strokeWidth={2}
                    isAnimationActive={!isExportMode}
                  />
                </LineChart>
              </ResponsiveContainer>
            </ReportChartBox>
          </section>
        </>
      ) : (
        <>
          {isDashboardError ? (
            <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
              Gagal memuat dashboard status lulusan.
            </p>
          ) : null}

          <section id="siswa-status-kpi" className="grid gap-3 sm:grid-cols-3">
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
              <p className="text-xs text-slate-500">Tahun Berjalan</p>
              <p className="text-xl font-semibold text-slate-900">
                {dashboard?.kpis?.currentYear ?? "-"}
              </p>
            </div>
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
              <p className="text-xs text-slate-500">Total Final</p>
              <p className="text-xl font-semibold text-slate-900">
                {formatNumber(dashboard?.kpis?.currentYearTotal ?? 0)}
              </p>
            </div>
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
              <p className="text-xs text-slate-500">YoY</p>
              <p className="text-xl font-semibold text-slate-900">
                {dashboard?.kpis?.yoyPercent == null
                  ? "-"
                  : `${dashboard.kpis.yoyPercent.toFixed(1)}%`}
              </p>
            </div>
          </section>

          <section id="siswa-status-charts" className="grid gap-4 lg:grid-cols-2">
            <div className="rounded-lg border border-slate-200 p-4">
              <p className="text-sm font-semibold text-slate-900">Tren Final 5 Tahun</p>
              <ReportChartBox className="mt-3" height={288}>
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={dashboardTrend}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="year" />
                    <YAxis />
                    <Tooltip />
                    <Bar dataKey="total" fill="#1f2937" isAnimationActive={!isExportMode} />
                  </BarChart>
                </ResponsiveContainer>
              </ReportChartBox>
            </div>
            <div className="rounded-lg border border-slate-200 p-4">
              <p className="text-sm font-semibold text-slate-900">Breakdown Jalur PT</p>
              <ReportChartBox className="mt-3" height={288}>
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={breakdown}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="label" />
                    <YAxis />
                    <Tooltip />
                    <Bar dataKey="total" fill="#1d4ed8" isAnimationActive={!isExportMode} />
                  </BarChart>
                </ResponsiveContainer>
              </ReportChartBox>
            </div>
          </section>
        </>
      )}
    </ReportShell>
  );
}
