"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
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
import { jenjangOptions } from "@/lib/shared/jenjangOptions";
import { Card, CardContent } from "@/components/ui/card";
import OutcomeStatusTabs from "@/components/siswa/OutcomeStatusTabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type AnalyticsYearPoint = {
  year: number;
  total: number;
};

type AnalyticsKeyTotal = {
  key: string;
  total: number;
};

type FinalOutcomeTopItem = {
  name: string;
  total: number;
  sharePercent: number;
};

type FinalOutcomeAnalyticsResponse = {
  availableYears: number[];
  latestYear: number | null;
  kpis: {
    total: number;
    yoyPercent: number | null;
    currentYearTotal: number;
    prevYearTotal: number;
  };
  trendByYear: AnalyticsYearPoint[];
  breakdownByJalur?: AnalyticsKeyTotal[];
  topKampus?: FinalOutcomeTopItem[];
  topProdi?: FinalOutcomeTopItem[];
};

const statusOptions = [
  { value: "PT", label: "Perguruan Tinggi" },
  { value: "KERJA", label: "Dunia Kerja" },
  { value: "WIRAUSAHA", label: "Wirausaha" },
  { value: "BELUM", label: "Belum Melanjutkan" },
] as const;

const jalurOrder = ["SNBP", "SNBT", "SNBM (Mandiri)", "Kedinasan", "Lainnya"];

const formatNumber = (value: number) =>
  new Intl.NumberFormat("id-ID").format(value);

const formatPercent = (value: number) =>
  `${value.toFixed(1).replace("-0.0", "0.0")}%`;

const isValidYear = (value: string) => /^\d{4}$/.test(value);

const fetchFinalOutcomeAnalytics = async (filters: {
  year?: string;
  jenjang?: string;
  status?: string;
}): Promise<FinalOutcomeAnalyticsResponse> => {
  const params = new URLSearchParams();
  if (filters.year) params.set("year", filters.year);
  if (filters.jenjang && filters.jenjang !== "ALL")
    params.set("jenjang", filters.jenjang);
  if (filters.status) params.set("status", filters.status);
  const endpoint = params.toString()
    ? `/api/public/siswa/outcomes/analytics?${params.toString()}`
    : "/api/public/siswa/outcomes/analytics";
  const response = await apiFetch(endpoint, undefined, {
    skipAuthRedirect: true,
  });
  if (!response.ok) {
    throw new Error("Failed to fetch final outcome analytics.");
  }
  const payload = (await response.json()) as { data: FinalOutcomeAnalyticsResponse };
  return payload.data;
};

export default function StatusLulusanPublicPage() {
  return (
    <Suspense
      fallback={<div className="p-4 text-sm text-zinc-500">Memuat...</div>}
    >
      <StatusLulusanPublicInner />
    </Suspense>
  );
}

function StatusLulusanPublicInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [activeTab, setActiveTab] = useState<"kampus" | "prodi">("kampus");

  const filters = useMemo(() => {
    const rawYear = searchParams.get("year") ?? "";
    const rawJenjang = searchParams.get("jenjang") ?? "ALL";
    const rawStatus = (searchParams.get("status") ?? "PT").toUpperCase();
    const normalizedYear = isValidYear(rawYear) ? rawYear : "";
    const normalizedJenjang = jenjangOptions.includes(
      rawJenjang as (typeof jenjangOptions)[number]
    )
      ? rawJenjang
      : "ALL";
    const statusValues = statusOptions.map((item) => item.value);
    const normalizedStatus: (typeof statusOptions)[number]["value"] =
      statusValues.includes(
        rawStatus as (typeof statusOptions)[number]["value"]
      )
        ? (rawStatus as (typeof statusOptions)[number]["value"])
        : "PT";
    return {
      year: normalizedYear,
      jenjang: normalizedJenjang,
      status: normalizedStatus,
    };
  }, [searchParams]);

  const [previousStatus, setPreviousStatus] = useState(filters.status);
  if (previousStatus !== filters.status) {
    setPreviousStatus(filters.status);
    if (filters.status !== "PT") setActiveTab("kampus");
  }

  const handleFilterChange = useCallback(
    (next: Partial<{ year: string; jenjang: string; status: string }>) => {
      const params = new URLSearchParams(searchParams.toString());
      const merged = {
        year: filters.year,
        jenjang: filters.jenjang,
        status: filters.status,
        ...next,
      };
      if (!merged.year) {
        params.delete("year");
      } else {
        params.set("year", merged.year);
      }
      if (!merged.jenjang || merged.jenjang === "ALL") {
        params.delete("jenjang");
      } else {
        params.set("jenjang", merged.jenjang);
      }
      if (!merged.status) {
        params.delete("status");
      } else {
        params.set("status", merged.status);
      }
      const nextQuery = params.toString();
      router.replace(nextQuery ? `?${nextQuery}` : "?", { scroll: false });
    },
    [filters, router, searchParams]
  );

  const {
    data: analytics,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: [
      "final-outcome-analytics",
      filters.year,
      filters.jenjang,
      filters.status,
    ],
    queryFn: () =>
      fetchFinalOutcomeAnalytics({
        year: filters.year || undefined,
        jenjang: filters.jenjang === "ALL" ? undefined : filters.jenjang,
        status: filters.status,
      }),
    retry: 0,
  });

  const analyticsSafe = useMemo(
    () => ({
      availableYears: analytics?.availableYears ?? [],
      latestYear: analytics?.latestYear ?? null,
      kpis: analytics?.kpis ?? {
        total: 0,
        yoyPercent: null,
        currentYearTotal: 0,
        prevYearTotal: 0,
      },
      trendByYear: analytics?.trendByYear ?? [],
      breakdownByJalur: analytics?.breakdownByJalur ?? [],
      topKampus: analytics?.topKampus ?? [],
      topProdi: analytics?.topProdi ?? [],
    }),
    [analytics]
  );

  const yearOptions = analyticsSafe.availableYears.map(String);
  const selectedYear =
    filters.year || (analyticsSafe.latestYear ? String(analyticsSafe.latestYear) : "");
  const selectedYearValue = yearOptions.includes(selectedYear) ? selectedYear : "";
  const currentTotal = analyticsSafe.kpis.currentYearTotal ?? analyticsSafe.kpis.total ?? 0;
  const hasYoY = analyticsSafe.kpis.prevYearTotal > 0 && analyticsSafe.kpis.yoyPercent != null;
  const yoyStatus =
    analyticsSafe.kpis.yoyPercent != null && analyticsSafe.kpis.yoyPercent >= 0
      ? "Naik"
      : "Turun";
  const yoyValue =
    analyticsSafe.kpis.yoyPercent == null ? "N/A" : formatPercent(Math.abs(analyticsSafe.kpis.yoyPercent));
  const statusLabel =
    statusOptions.find((status) => status.value === filters.status)?.label ??
    filters.status;
  const prevYearLabel = selectedYear ? String(Number(selectedYear) - 1) : "-";

  const breakdownSorted = useMemo(() => {
    const orderMap = new Map(jalurOrder.map((item, index) => [item, index]));
    return [...analyticsSafe.breakdownByJalur].sort(
      (a, b) => (orderMap.get(a.key) ?? 99) - (orderMap.get(b.key) ?? 99)
    );
  }, [analyticsSafe.breakdownByJalur]);

  const activeTopItems = activeTab === "kampus" ? analyticsSafe.topKampus : analyticsSafe.topProdi;
  const hasTopItems = activeTopItems.length > 0;
  const hasTrendData = analyticsSafe.trendByYear.length > 0;
  const hasBreakdownData = breakdownSorted.length > 0;

  return (
    <section className="space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Siswa
        </p>
        <h1 className="text-3xl font-semibold text-zinc-900">
          Status Lulusan (Final Outcome)
        </h1>
        <p className="text-sm text-zinc-600">
          Ringkasan agregat status akhir lulusan tingkat Cabang Dinas.
        </p>
        <div className="flex flex-wrap items-end gap-3 pt-2">
          <div className="space-y-1">
            <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Jenjang
            </label>
            <Select
              value={filters.jenjang}
              onValueChange={(value) => handleFilterChange({ jenjang: value })}
            >
              <SelectTrigger className="h-10 w-[170px]">
                <SelectValue placeholder="Semua" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ALL">Semua</SelectItem>
                {jenjangOptions.map((option) => (
                  <SelectItem key={option} value={option}>
                    {option}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Tahun Lulus
            </label>
            <Select
              value={selectedYearValue}
              onValueChange={(value) => handleFilterChange({ year: value })}
              disabled={yearOptions.length === 0}
            >
              <SelectTrigger className="h-10 w-[160px]">
                <SelectValue placeholder="Pilih tahun" />
              </SelectTrigger>
              <SelectContent>
                {yearOptions.map((option) => (
                  <SelectItem key={option} value={option}>
                    {option}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      </header>

      <OutcomeStatusTabs
        value={filters.status}
        onChange={(value) => handleFilterChange({ status: value })}
      />

      {isError ? (
        <div className="rounded-xl border border-dashed border-red-200 bg-white p-4 text-sm text-red-600">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span>Gagal memuat data status lulusan.</span>
            <button
              type="button"
              onClick={() => refetch()}
              className="rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600"
            >
              Coba lagi
            </button>
          </div>
        </div>
      ) : null}

      <div className={`grid gap-4 ${hasYoY ? "md:grid-cols-2" : ""}`}>
        {isLoading ? (
          <div className="h-24 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
        ) : (
          <Card>
            <CardContent className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Total {statusLabel} Tahun {selectedYear || "-"}
              </p>
              <p className="text-3xl font-semibold text-zinc-900">
                {formatNumber(currentTotal)}
              </p>
            </CardContent>
          </Card>
        )}
        {isLoading && hasYoY ? (
          <div className="h-24 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
        ) : hasYoY ? (
          <Card>
            <CardContent className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                YoY vs Tahun {prevYearLabel}
              </p>
              <p className="text-3xl font-semibold text-zinc-900">
                {yoyValue}
              </p>
              <p className="text-sm text-zinc-600">{yoyStatus}</p>
            </CardContent>
          </Card>
        ) : null}
      </div>
      {!isLoading && !hasYoY ? (
        <p className="text-sm text-zinc-500">
          Data tahun sebelumnya belum tersedia.
        </p>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-2">
        {isLoading ? (
          <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
        ) : (
          <Card>
            <CardContent className="space-y-3">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tren 5 Tahun
                </p>
                <p className="text-sm text-zinc-600">
                  Window {selectedYear || "-"} (5 tahun terakhir).
                </p>
              </div>
              <div className="h-64">
                {!hasTrendData ? (
                  <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                    Tidak ada data.
                  </div>
                ) : (
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={analyticsSafe.trendByYear}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="year" />
                      <YAxis />
                      <Tooltip />
                      <Line
                        type="monotone"
                        dataKey="total"
                        stroke="#0f172a"
                        strokeWidth={2}
                        dot={{ r: 3 }}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                )}
              </div>
            </CardContent>
          </Card>
        )}

        {filters.status === "PT" ? (
          isLoading ? (
            <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
          ) : (
            <Card>
              <CardContent className="space-y-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Breakdown Jalur PT
                  </p>
                  <p className="text-sm text-zinc-600">
                    Komposisi jalur penerimaan untuk status PT.
                  </p>
                </div>
                <div className="h-64">
                  {!hasBreakdownData ? (
                    <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                      Tidak ada data.
                    </div>
                  ) : (
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={breakdownSorted} margin={{ left: 8 }}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="key" tick={{ fontSize: 11 }} />
                        <YAxis />
                        <Tooltip />
                        <Bar dataKey="total" fill="#2563eb" />
                      </BarChart>
                    </ResponsiveContainer>
                  )}
                </div>
              </CardContent>
            </Card>
          )
        ) : null}
      </div>

      {filters.status === "PT" ? (
        isLoading ? (
          <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
        ) : (
          <Card>
            <CardContent className="space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Top 10
                  </p>
                  <p className="text-sm text-zinc-600">
                    Perguruan tinggi dan prodi teratas untuk tahun terpilih.
                  </p>
                </div>
                <div className="inline-flex rounded-full border border-zinc-200 bg-white p-1 text-xs font-semibold text-zinc-700">
                  <button
                    type="button"
                    onClick={() => setActiveTab("kampus")}
                    className={`rounded-full px-3 py-1 transition ${
                      activeTab === "kampus"
                        ? "bg-zinc-900 text-white"
                        : "text-zinc-600 hover:text-zinc-900"
                    }`}
                  >
                    Kampus
                  </button>
                  <button
                    type="button"
                    onClick={() => setActiveTab("prodi")}
                    className={`rounded-full px-3 py-1 transition ${
                      activeTab === "prodi"
                        ? "bg-zinc-900 text-white"
                        : "text-zinc-600 hover:text-zinc-900"
                    }`}
                  >
                    Prodi
                  </button>
                </div>
              </div>

              {!hasTopItems ? (
                <div className="flex h-40 items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                  Tidak ada data.
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-16">Rank</TableHead>
                      <TableHead>Nama</TableHead>
                      <TableHead className="text-right">Total</TableHead>
                      <TableHead className="text-right">Share</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {activeTopItems.map((item, index) => {
                      const percent = Math.min(
                        100,
                        Math.max(0, item.sharePercent ?? 0)
                      );
                      return (
                        <TableRow key={`${item.name}-${index}`}>
                          <TableCell>
                            <span className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-zinc-900 text-xs font-semibold text-white">
                              {index + 1}
                            </span>
                          </TableCell>
                          <TableCell className="min-w-[240px]">
                            <p className="font-semibold text-zinc-900">
                              {item.name}
                            </p>
                            <div className="mt-2 h-1.5 w-full rounded-full bg-zinc-100">
                              <div
                                className="h-1.5 rounded-full bg-emerald-500"
                                style={{ width: `${percent}%` }}
                              />
                            </div>
                          </TableCell>
                          <TableCell className="text-right font-semibold text-zinc-900">
                            {formatNumber(item.total)}
                          </TableCell>
                          <TableCell className="text-right">
                            {formatPercent(item.sharePercent ?? 0)}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        )
      ) : null}
    </section>
  );
}
