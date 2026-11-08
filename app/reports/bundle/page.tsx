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

type AkreditasiPayload = {
  kpi?: {
    totalSchools?: number;
    totalGuru?: number;
    totalTendik?: number;
    percentAkreditasiAB?: number;
  };
  statusByJenjang?: { jenjang: string; Negeri: number; Swasta: number }[];
};

type PrestasiPayload = {
  kpis?: {
    total?: number;
    currentYearTotal?: number;
    prevYearTotal?: number;
    yoyPercent?: number | null;
  };
  trendByYear?: { year: number; total: number }[];
};

type AnalyticsPTPayload = {
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
  breakdownJalur?: { label: string; total: number }[];
};

type PTRankingItem = {
  ptn_name: string;
  count: number;
};

const formatNumber = (value: number) => new Intl.NumberFormat("id-ID").format(value);

const fetchAkreditasi = async (search: URLSearchParams): Promise<AkreditasiPayload> => {
  const query = new URLSearchParams();
  const jenjang = search.get("jenjang");
  const schoolId = search.get("school_id");
  const status = search.get("status") ?? search.get("status_sekolah");
  if (jenjang) query.set("jenjang", jenjang);
  if (schoolId) query.set("school_id", schoolId);
  if (status) query.set("status", status);

  const response = await apiFetch(`/api/dashboard${query.toString() ? `?${query.toString()}` : ""}`);
  if (!response.ok) throw new Error("akreditasi_failed");
  const payload = (await response.json()) as { data: AkreditasiPayload };
  return payload.data ?? {};
};

const fetchPrestasi = async (search: URLSearchParams): Promise<PrestasiPayload> => {
  const query = new URLSearchParams();
  const year = search.get("year");
  const jenjang = search.get("jenjang");
  const statusSekolah = search.get("status_sekolah") ?? search.get("status");
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
  if (!response.ok) throw new Error("prestasi_failed");
  const payload = (await response.json()) as { data: PrestasiPayload };
  return payload.data ?? {};
};

const fetchSiswaAnalytics = async (search: URLSearchParams): Promise<AnalyticsPTPayload> => {
  const query = new URLSearchParams();
  const now = new Date().getFullYear();
  query.set("start_year", search.get("start_year") ?? String(now - 4));
  query.set("end_year", search.get("end_year") ?? String(now));
  const jenjang = search.get("jenjang");
  const schoolId = search.get("school_id");
  if (jenjang) query.set("jenjang", jenjang);
  if (schoolId) query.set("school_id", schoolId);

  const response = await apiFetch(`/api/siswa/status-lulusan/analytics-pt?${query.toString()}`);
  if (!response.ok) throw new Error("analytics_failed");
  const payload = (await response.json()) as { data: AnalyticsPTPayload };
  return payload.data ?? {};
};

const fetchOutcomeDashboard = async (
  search: URLSearchParams,
  status: "PT" | "KERJA" | "WIRAUSAHA" | "BELUM"
): Promise<OutcomeDashboardPayload> => {
  const query = new URLSearchParams();
  query.set("status", status);
  const year = search.get("year");
  const jenjang = search.get("jenjang");
  const schoolId = search.get("school_id");
  const statusSekolah = search.get("status_sekolah");
  if (year) query.set("year", year);
  if (jenjang) query.set("jenjang", jenjang);
  if (schoolId) query.set("school_id", schoolId);
  if (statusSekolah) query.set("status_sekolah", statusSekolah);
  if (status === "PT") {
    const jalur = search.get("jalur");
    if (jalur) query.set("jalur", jalur);
  }

  const response = await apiFetch(`/api/siswa/outcomes/dashboard?${query.toString()}`);
  if (!response.ok) throw new Error(`outcome_${status.toLowerCase()}_failed`);
  const payload = (await response.json()) as { data: OutcomeDashboardPayload };
  return payload.data ?? {};
};

const fetchPTRanking = async (search: URLSearchParams): Promise<PTRankingItem[]> => {
  const query = new URLSearchParams();
  ["year", "jenjang", "status_sekolah", "school_id"].forEach((key) => {
    const value = search.get(key);
    if (value) query.set(key, value);
  });

  const response = await apiFetch(
    `/api/siswa/pt/ranking${query.toString() ? `?${query.toString()}` : ""}`
  );
  if (!response.ok) throw new Error("pt_ranking_failed");
  const payload = (await response.json()) as { data: PTRankingItem[] };
  return payload.data ?? [];
};

function OutcomeSection({
  id,
  title,
  className,
  data,
  isError,
  isExportMode,
}: {
  id: string;
  title: string;
  className?: string;
  data?: OutcomeDashboardPayload;
  isError: boolean;
  isExportMode: boolean;
}) {
  return (
    <section id={id} className={className ? `${className} space-y-4` : "space-y-4"}>
      <div>
        <h2 className="text-xl font-semibold text-slate-900">{title}</h2>
        <p className="text-sm text-slate-600">Ringkasan data status akhir siswa.</p>
      </div>
      {isError ? (
        <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
          Gagal memuat data {title.toLowerCase()}.
        </p>
      ) : null}
      <section className="grid gap-3 sm:grid-cols-3">
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs text-slate-500">Tahun Berjalan</p>
          <p className="text-xl font-semibold text-slate-900">{data?.kpis?.currentYear ?? "-"}</p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs text-slate-500">Total Final</p>
          <p className="text-xl font-semibold text-slate-900">
            {formatNumber(data?.kpis?.currentYearTotal ?? 0)}
          </p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs text-slate-500">YoY</p>
          <p className="text-xl font-semibold text-slate-900">
            {data?.kpis?.yoyPercent == null ? "-" : `${data.kpis.yoyPercent.toFixed(1)}%`}
          </p>
        </div>
      </section>
      <section className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Tren Final 5 Tahun</p>
          <ReportChartBox className="mt-3" height={256}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={data?.trendByYear ?? []}>
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
          <p className="text-sm font-semibold text-slate-900">Breakdown Jalur</p>
          <ReportChartBox className="mt-3" height={256}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={data?.breakdownJalur ?? []}>
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
    </section>
  );
}

export default function BundleReportPage() {
  const searchParams = useSearchParams();
  const queryKey = searchParams.toString();

  const { data: akreditasi, isLoading: isAkreditasiLoading, isError: isAkreditasiError } = useQuery({
    queryKey: ["report-bundle-akreditasi", queryKey],
    queryFn: () => fetchAkreditasi(new URLSearchParams(queryKey)),
    retry: 0,
  });
  const { data: prestasi, isLoading: isPrestasiLoading, isError: isPrestasiError } = useQuery({
    queryKey: ["report-bundle-prestasi", queryKey],
    queryFn: () => fetchPrestasi(new URLSearchParams(queryKey)),
    retry: 0,
  });
  const {
    data: analyticsPT,
    isLoading: isAnalyticsLoading,
    isError: isAnalyticsError,
  } = useQuery({
    queryKey: ["report-bundle-siswa-analytics", queryKey],
    queryFn: () => fetchSiswaAnalytics(new URLSearchParams(queryKey)),
    retry: 0,
  });
  const { data: outcomePT, isLoading: isOutcomePTLoading, isError: isOutcomePTError } = useQuery({
    queryKey: ["report-bundle-siswa-pt", queryKey],
    queryFn: () => fetchOutcomeDashboard(new URLSearchParams(queryKey), "PT"),
    retry: 0,
  });
  const {
    data: outcomeKerja,
    isLoading: isOutcomeKerjaLoading,
    isError: isOutcomeKerjaError,
  } = useQuery({
    queryKey: ["report-bundle-siswa-kerja", queryKey],
    queryFn: () => fetchOutcomeDashboard(new URLSearchParams(queryKey), "KERJA"),
    retry: 0,
  });
  const {
    data: outcomeWirausaha,
    isLoading: isOutcomeWirausahaLoading,
    isError: isOutcomeWirausahaError,
  } = useQuery({
    queryKey: ["report-bundle-siswa-wirausaha", queryKey],
    queryFn: () => fetchOutcomeDashboard(new URLSearchParams(queryKey), "WIRAUSAHA"),
    retry: 0,
  });
  const { data: outcomeBelum, isLoading: isOutcomeBelumLoading, isError: isOutcomeBelumError } =
    useQuery({
      queryKey: ["report-bundle-siswa-belum", queryKey],
      queryFn: () => fetchOutcomeDashboard(new URLSearchParams(queryKey), "BELUM"),
      retry: 0,
    });
  const { data: ptRanking = [], isLoading: isPTRankingLoading, isError: isPTRankingError } =
    useQuery({
      queryKey: ["report-bundle-pt-ranking", queryKey],
      queryFn: () => fetchPTRanking(new URLSearchParams(queryKey)),
      retry: 0,
    });

  const ready =
    !isAkreditasiLoading &&
    !isPrestasiLoading &&
    !isAnalyticsLoading &&
    !isOutcomePTLoading &&
    !isOutcomeKerjaLoading &&
    !isOutcomeWirausahaLoading &&
    !isOutcomeBelumLoading &&
    !isPTRankingLoading;

  const akreditasiKpi = akreditasi?.kpi ?? {};
  const analyticsChart = useMemo(
    () =>
      (analyticsPT?.totals_by_year ?? []).map((row) => ({
        tahun: row.tahun_lulus,
        pendaftar: row.pendaftar,
        diterima: row.diterima,
      })),
    [analyticsPT?.totals_by_year]
  );
  const rankingTop20 = useMemo(() => ptRanking.slice(0, 20), [ptRanking]);
  const { isExportMode, showWatermark } = useReportExportMode(searchParams, ready);

  return (
    <ReportShell
      title="Laporan Lengkap SIPRES"
      subtitle="Gabungan laporan akreditasi, prestasi, siswa, dan peringkat PT."
      ready={ready}
      exportMode={isExportMode}
      showWatermark={showWatermark}
    >
      <section id="bundle-akreditasi" className="space-y-4">
        <div>
          <h2 className="text-xl font-semibold text-slate-900">Akreditasi</h2>
          <p className="text-sm text-slate-600">Ringkasan indikator akreditasi wilayah.</p>
        </div>
        {isAkreditasiError ? (
          <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
            Gagal memuat data akreditasi.
          </p>
        ) : null}
        <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">Total Sekolah</p>
            <p className="text-xl font-semibold text-slate-900">
              {formatNumber(akreditasiKpi.totalSchools ?? 0)}
            </p>
          </div>
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">Total Guru</p>
            <p className="text-xl font-semibold text-slate-900">
              {formatNumber(akreditasiKpi.totalGuru ?? 0)}
            </p>
          </div>
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">Total Tendik</p>
            <p className="text-xl font-semibold text-slate-900">
              {formatNumber(akreditasiKpi.totalTendik ?? 0)}
            </p>
          </div>
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">% Akreditasi A+B</p>
            <p className="text-xl font-semibold text-slate-900">
              {(akreditasiKpi.percentAkreditasiAB ?? 0).toFixed(1)}%
            </p>
          </div>
        </section>
        <section className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Status Sekolah per Jenjang</p>
          <ReportChartBox className="mt-3" height={288}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={akreditasi?.statusByJenjang ?? []}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="jenjang" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="Negeri" fill="#0f766e" isAnimationActive={!isExportMode} />
                <Bar dataKey="Swasta" fill="#94a3b8" isAnimationActive={!isExportMode} />
              </BarChart>
            </ResponsiveContainer>
          </ReportChartBox>
        </section>
      </section>

      <section id="bundle-prestasi" className="pageBreak space-y-4">
        <div>
          <h2 className="text-xl font-semibold text-slate-900">Prestasi</h2>
          <p className="text-sm text-slate-600">Ringkasan prestasi dan tren tahunan.</p>
        </div>
        {isPrestasiError ? (
          <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
            Gagal memuat data prestasi.
          </p>
        ) : null}
        <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">Total Prestasi</p>
            <p className="text-xl font-semibold text-slate-900">
              {formatNumber(prestasi?.kpis?.total ?? 0)}
            </p>
          </div>
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">Tahun Berjalan</p>
            <p className="text-xl font-semibold text-slate-900">
              {formatNumber(prestasi?.kpis?.currentYearTotal ?? 0)}
            </p>
          </div>
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">Tahun Sebelumnya</p>
            <p className="text-xl font-semibold text-slate-900">
              {formatNumber(prestasi?.kpis?.prevYearTotal ?? 0)}
            </p>
          </div>
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs text-slate-500">YoY</p>
            <p className="text-xl font-semibold text-slate-900">
              {prestasi?.kpis?.yoyPercent == null ? "-" : `${prestasi.kpis.yoyPercent.toFixed(1)}%`}
            </p>
          </div>
        </section>
        <section className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Tren Prestasi Tahunan</p>
          <ReportChartBox className="mt-3" height={288}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={prestasi?.trendByYear ?? []}>
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
        </section>
      </section>

      <section id="bundle-siswa-analytics" className="pageBreak space-y-4">
        <div>
          <h2 className="text-xl font-semibold text-slate-900">Siswa - Analytics PT</h2>
          <p className="text-sm text-slate-600">Tren pendaftar dan diterima perguruan tinggi.</p>
        </div>
        {isAnalyticsError ? (
          <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
            Gagal memuat analytics siswa.
          </p>
        ) : null}
        <section className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Tren Pendaftar vs Diterima</p>
          <ReportChartBox className="mt-3" height={320}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={analyticsChart}>
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
      </section>

      <OutcomeSection
        id="bundle-siswa-pt"
        className="pageBreak"
        title="Siswa - Perguruan Tinggi"
        data={outcomePT}
        isError={isOutcomePTError}
        isExportMode={isExportMode}
      />
      <OutcomeSection
        id="bundle-siswa-kerja"
        className="pageBreak"
        title="Siswa - Dunia Kerja"
        data={outcomeKerja}
        isError={isOutcomeKerjaError}
        isExportMode={isExportMode}
      />
      <OutcomeSection
        id="bundle-siswa-wirausaha"
        className="pageBreak"
        title="Siswa - Wirausaha"
        data={outcomeWirausaha}
        isError={isOutcomeWirausahaError}
        isExportMode={isExportMode}
      />
      <OutcomeSection
        id="bundle-siswa-belum"
        className="pageBreak"
        title="Siswa - Belum Melanjutkan"
        data={outcomeBelum}
        isError={isOutcomeBelumError}
        isExportMode={isExportMode}
      />

      <section id="bundle-pt-ranking" className="pageBreak space-y-4">
        <div>
          <h2 className="text-xl font-semibold text-slate-900">Peringkat PT</h2>
          <p className="text-sm text-slate-600">
            Peringkat keseluruhan perguruan tinggi berdasarkan total diterima.
          </p>
        </div>
        {isPTRankingError ? (
          <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
            Gagal memuat peringkat PT.
          </p>
        ) : null}
        <section className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Top 20 PTN</p>
          <ReportChartBox className="mt-3" height={384}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={rankingTop20} layout="vertical" margin={{ left: 120, right: 16 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis type="number" />
                <YAxis dataKey="ptn_name" type="category" width={240} />
                <Tooltip />
                <Bar dataKey="count" fill="#1d4ed8" isAnimationActive={!isExportMode} />
              </BarChart>
            </ResponsiveContainer>
          </ReportChartBox>
        </section>
        <section className="rounded-lg border border-slate-200 p-4">
          <p className="text-sm font-semibold text-slate-900">Top 50 Ranking</p>
          <div className="mt-3 overflow-hidden rounded-md border border-slate-200">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 text-left text-slate-600">
                <tr>
                  <th className="px-3 py-2">No</th>
                  <th className="px-3 py-2">PTN</th>
                  <th className="px-3 py-2 text-right">Total</th>
                </tr>
              </thead>
              <tbody>
                {(ptRanking ?? []).slice(0, 50).map((row, index) => (
                  <tr key={`${row.ptn_name}-${index}`} className="border-t border-slate-100">
                    <td className="px-3 py-2 text-slate-600">{index + 1}</td>
                    <td className="px-3 py-2 font-medium text-slate-900">{row.ptn_name}</td>
                    <td className="px-3 py-2 text-right text-slate-700">{formatNumber(row.count)}</td>
                  </tr>
                ))}
                {ptRanking.length === 0 ? (
                  <tr>
                    <td className="px-3 py-3 text-slate-500" colSpan={3}>
                      Tidak ada data.
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </section>
      </section>
    </ReportShell>
  );
}
