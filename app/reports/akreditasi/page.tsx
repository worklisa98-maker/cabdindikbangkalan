"use client";

import { Suspense, useMemo } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { apiFetch } from "@/lib/api/client";
import { ReportChartBox } from "@/components/reports/ReportChartBox";
import { ReportShell } from "@/components/reports/report-shell";
import { useReportExportMode } from "@/components/reports/use-report-export-mode";

type DashboardPayload = {
  kpi?: {
    totalSchools?: number;
    totalGuru?: number;
    totalTendik?: number;
    percentAkreditasiAB?: number;
  };
  statusByJenjang?: { jenjang: string; Negeri: number; Swasta: number }[];
  akreditasiComposition?: { label: string; total: number }[];
};

const chartColors: Record<string, string> = {
  A: "#0f766e",
  B: "#2563eb",
  C: "#f59e0b",
  BELUM: "#9ca3af",
};

const formatNumber = (value: number) => new Intl.NumberFormat("id-ID").format(value);

const fetchDashboard = async (search: URLSearchParams): Promise<DashboardPayload> => {
  const query = new URLSearchParams();
  const jenjang = search.get("jenjang");
  const status = search.get("status");
  const schoolId = search.get("school_id");
  if (jenjang) query.set("jenjang", jenjang);
  if (status) query.set("status", status);
  if (schoolId) query.set("school_id", schoolId);

  const response = await apiFetch(
    `/api/dashboard${query.toString() ? `?${query.toString()}` : ""}`
  );
  if (!response.ok) throw new Error("Failed to load dashboard report.");
  const payload = (await response.json()) as { data: DashboardPayload };
  return payload.data ?? {};
};

function AkreditasiReportContent() {
  const searchParams = useSearchParams();
  const queryKey = searchParams.toString();

  const { data, isLoading, isError } = useQuery({
    queryKey: ["report-akreditasi", queryKey],
    queryFn: () => fetchDashboard(new URLSearchParams(queryKey)),
    retry: 0,
  });

  const kpi = data?.kpi ?? {};
  const statusByJenjang = data?.statusByJenjang ?? [];
  const akreditasiComposition = data?.akreditasiComposition ?? [];
  const ready = !isLoading;
  const { isExportMode, showWatermark } = useReportExportMode(searchParams, ready);

  const kpiCards = useMemo(
    () => [
      { label: "Total Sekolah", value: formatNumber(kpi.totalSchools ?? 0) },
      { label: "Total Guru", value: formatNumber(kpi.totalGuru ?? 0) },
      { label: "Total Tendik", value: formatNumber(kpi.totalTendik ?? 0) },
      {
        label: "% Akreditasi A+B",
        value: `${(kpi.percentAkreditasiAB ?? 0).toFixed(1)}%`,
      },
    ],
    [kpi.percentAkreditasiAB, kpi.totalGuru, kpi.totalSchools, kpi.totalTendik]
  );

  return (
    <ReportShell
      title="Laporan Akreditasi"
      subtitle="Ringkasan akreditasi, komposisi status sekolah, dan indikator utama."
      ready={ready}
      exportMode={isExportMode}
      showWatermark={showWatermark}
    >
      <section id="akreditasi-kpi" className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {kpiCards.map((item) => (
          <div key={item.label} className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">{item.label}</p>
            <p className="text-xl font-semibold text-slate-900">{item.value}</p>
          </div>
        ))}
      </section>

      {isError ? (
        <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
          Gagal memuat data laporan akreditasi.
        </p>
      ) : null}

      <section id="akreditasi-charts" className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Status Sekolah per Jenjang</p>
          <ReportChartBox className="mt-3" height={288}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={statusByJenjang}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="jenjang" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="Negeri" fill="#0f766e" isAnimationActive={!isExportMode} />
                <Bar dataKey="Swasta" fill="#94a3b8" isAnimationActive={!isExportMode} />
              </BarChart>
            </ResponsiveContainer>
          </ReportChartBox>
        </div>

        <div className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Komposisi Akreditasi</p>
          <ReportChartBox className="mt-3" height={288}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={akreditasiComposition}
                  dataKey="total"
                  nameKey="label"
                  outerRadius={95}
                  isAnimationActive={!isExportMode}
                >
                  {akreditasiComposition.map((item) => (
                    <Cell
                      key={item.label}
                      fill={chartColors[item.label?.toUpperCase()] ?? "#64748b"}
                    />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </ReportChartBox>
        </div>
      </section>
    </ReportShell>
  );
}

function AkreditasiReportFallback() {
  return (
    <ReportShell
      title="Laporan Akreditasi"
      subtitle="Ringkasan akreditasi, komposisi status sekolah, dan indikator utama."
      ready={false}
    >
      <></>
    </ReportShell>
  );
}

export default function AkreditasiReportPage() {
  return (
    <Suspense fallback={<AkreditasiReportFallback />}>
      <AkreditasiReportContent />
    </Suspense>
  );
}
