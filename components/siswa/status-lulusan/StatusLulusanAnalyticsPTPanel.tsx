"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { DataTable } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { SectionWrapper } from "@/components/section-wrapper";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { TableCell, TableRow } from "@/components/ui/table";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import {
  getErrorMessageFromResponse,
  getFilenameFromContentDisposition,
} from "@/lib/http";

type AnalyticsSeriesItem = {
  tahunLulus: number;
  jalur: string;
  pendaftar: number;
  diterima: number;
  acceptanceRate: number;
  yoyPendaftar: number | null;
  yoyDiterima: number | null;
};

type AnalyticsTotalItem = {
  tahunLulus: number;
  pendaftar: number;
  diterima: number;
  acceptanceRate: number;
  yoyPendaftar: number | null;
  yoyDiterima: number | null;
};

type AnalyticsDistributionItem = {
  label: string;
  total: number;
  sharePercent: number;
};

type AnalyticsDetailItem = {
  schoolName: string;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  jalur: string;
  namaPt: string;
  prodi: string;
  tahunDiterima: number | null;
};

type AnalyticsPayload = {
  startYear: number;
  endYear: number;
  series: AnalyticsSeriesItem[];
  totalsByYear: AnalyticsTotalItem[];
  topKampus: AnalyticsDistributionItem[];
  topProdi: AnalyticsDistributionItem[];
  detailRows: AnalyticsDetailItem[];
  detailMeta: {
    page: number;
    pageSize: number;
    total: number;
  };
};

type StatusLulusanAnalyticsPTPanelProps = {
  isPublic?: boolean;
};

const JALUR_ORDER = ["SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"];
const JALUR_COLORS: Record<string, string> = {
  SNBP: "#0f766e",
  SNBT: "#0284c7",
  MANDIRI: "#d97706",
  KEDINASAN: "#1d4ed8",
  LAINNYA: "#64748b",
};

const formatNumber = (value: number) =>
  new Intl.NumberFormat("id-ID").format(value);

const formatPercent = (value: number | null | undefined) => {
  if (value == null || !Number.isFinite(value)) {
    return "-";
  }
  return `${value.toFixed(1)}%`;
};

const toNumber = (value: unknown) => {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const toNullableNumber = (value: unknown) => {
  if (value == null) return null;
  const parsed = toNumber(value);
  if (!Number.isFinite(parsed)) return null;
  return parsed;
};

const toString = (value: unknown) => {
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return "";
};

const normalizeJalur = (value: string) => {
  const upper = value.trim().toUpperCase();
  if (!upper) return "LAINNYA";
  if (upper.includes("SNBP")) return "SNBP";
  if (upper.includes("SNBT")) return "SNBT";
  if (upper.includes("MANDIRI") || upper.includes("SNBM")) return "MANDIRI";
  if (upper.includes("KEDINAS")) return "KEDINASAN";
  return "LAINNYA";
};

const emptyPayload = (page: number, pageSize: number): AnalyticsPayload => ({
  startYear: 0,
  endYear: 0,
  series: [],
  totalsByYear: [],
  topKampus: [],
  topProdi: [],
  detailRows: [],
  detailMeta: {
    page,
    pageSize,
    total: 0,
  },
});

const fetchAnalytics = async (
  endpoint: string,
  params: {
    startYear: number;
    endYear: number;
    jenjang: string;
    search: string;
    page: number;
    pageSize: number;
  },
  isPublic: boolean
): Promise<AnalyticsPayload> => {
  const query = new URLSearchParams();
  query.set("start_year", String(params.startYear));
  query.set("end_year", String(params.endYear));
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.jenjang !== "ALL") {
    query.set("jenjang", params.jenjang);
  }
  if (params.search.trim()) {
    query.set("q", params.search.trim());
  }
  const response = await apiFetch(`${endpoint}?${query.toString()}`, undefined, {
    skipAuthRedirect: isPublic,
  });
  if (!response.ok) {
    throw new Error("Gagal memuat analytics PT.");
  }
  const raw = await response.json().catch(() => null);
  const data = raw?.data ?? raw ?? {};

  const seriesRaw = Array.isArray(data?.series) ? data.series : [];
  const totalsRaw = Array.isArray(data?.totals_by_year) ? data.totals_by_year : [];
  const topKampusRaw = Array.isArray(data?.top_kampus ?? data?.topKampus)
    ? (data?.top_kampus ?? data?.topKampus)
    : [];
  const topProdiRaw = Array.isArray(data?.top_prodi ?? data?.topProdi)
    ? (data?.top_prodi ?? data?.topProdi)
    : [];
  const detailRowsRaw = Array.isArray(data?.detail_rows ?? data?.detailRows)
    ? (data?.detail_rows ?? data?.detailRows)
    : [];
  const detailMetaRaw =
    typeof data?.detail_meta === "object" && data?.detail_meta != null
      ? data.detail_meta
      : typeof data?.detailMeta === "object" && data?.detailMeta != null
        ? data.detailMeta
        : {};

  return {
    startYear: toNumber(data?.start_year ?? data?.startYear),
    endYear: toNumber(data?.end_year ?? data?.endYear),
    series: seriesRaw.map((item: Record<string, unknown>) => ({
      tahunLulus: toNumber(item?.tahun_lulus ?? item?.tahunLulus),
      jalur: normalizeJalur(toString(item?.jalur)),
      pendaftar: toNumber(item?.pendaftar),
      diterima: toNumber(item?.diterima),
      acceptanceRate: toNumber(item?.acceptance_rate ?? item?.acceptanceRate),
      yoyPendaftar: toNullableNumber(item?.yoy_pendaftar ?? item?.yoyPendaftar),
      yoyDiterima: toNullableNumber(item?.yoy_diterima ?? item?.yoyDiterima),
    })),
    totalsByYear: totalsRaw.map((item: Record<string, unknown>) => ({
      tahunLulus: toNumber(item?.tahun_lulus ?? item?.tahunLulus),
      pendaftar: toNumber(item?.pendaftar),
      diterima: toNumber(item?.diterima),
      acceptanceRate: toNumber(item?.acceptance_rate ?? item?.acceptanceRate),
      yoyPendaftar: toNullableNumber(item?.yoy_pendaftar ?? item?.yoyPendaftar),
      yoyDiterima: toNullableNumber(item?.yoy_diterima ?? item?.yoyDiterima),
    })),
    topKampus: topKampusRaw.map((item: Record<string, unknown>) => ({
      label: toString(item?.label),
      total: toNumber(item?.total),
      sharePercent: toNumber(item?.share_percent ?? item?.sharePercent),
    })),
    topProdi: topProdiRaw.map((item: Record<string, unknown>) => ({
      label: toString(item?.label),
      total: toNumber(item?.total),
      sharePercent: toNumber(item?.share_percent ?? item?.sharePercent),
    })),
    detailRows: isPublic ? [] : detailRowsRaw.map((item: Record<string, unknown>) => {
      const tahunDiterima = toNumber(
        item?.tahun_diterima ?? item?.tahunDiterima
      );
      return {
        schoolName: toString(item?.school_name ?? item?.schoolName),
        nisn: toString(item?.nisn),
        namaSiswa: toString(item?.nama_siswa ?? item?.namaSiswa),
        rombel: toString(item?.rombel),
        tahunLulus: toNumber(item?.tahun_lulus ?? item?.tahunLulus),
        jalur: normalizeJalur(toString(item?.jalur)),
        namaPt: toString(item?.nama_pt ?? item?.namaPt),
        prodi: toString(item?.prodi),
        tahunDiterima: tahunDiterima > 0 ? tahunDiterima : null,
      };
    }),
    detailMeta: {
      page: toNumber((detailMetaRaw as Record<string, unknown>)?.page) || params.page,
      pageSize:
        toNumber(
          (detailMetaRaw as Record<string, unknown>)?.page_size ??
            (detailMetaRaw as Record<string, unknown>)?.pageSize
        ) || params.pageSize,
      total: isPublic ? 0 : toNumber((detailMetaRaw as Record<string, unknown>)?.total),
    },
  };
};

const DistributionList = ({
  title,
  description,
  items,
  emptyTitle,
}: {
  title: string;
  description: string;
  items: AnalyticsDistributionItem[];
  emptyTitle: string;
}) => (
  <Card>
    <CardContent className="space-y-4 p-4 sm:p-5">
      <div className="space-y-1">
        <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
          {title}
        </p>
        <p className="text-sm text-slate-500">{description}</p>
      </div>
      {items.length === 0 ? (
        <EmptyState
          title={emptyTitle}
          description="Section tetap tampil agar pola sebaran mudah dibaca saat data mulai masuk."
          className="min-h-40"
        />
      ) : (
        <div className="h-[320px]">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={[...items].reverse()}
              layout="vertical"
              margin={{ top: 8, right: 16, left: 12, bottom: 8 }}
            >
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
              <XAxis type="number" allowDecimals={false} />
              <YAxis
                type="category"
                dataKey="label"
                width={130}
                tick={{ fontSize: 11 }}
              />
              <Tooltip
                formatter={(value, _name, item) => [
                  `${formatNumber(toNumber(value))} siswa`,
                  `${toNumber(item?.payload?.sharePercent).toFixed(1)}%`,
                ]}
              />
              <Bar dataKey="total" fill="#0f172a" radius={[0, 6, 6, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </CardContent>
  </Card>
);

export default function StatusLulusanAnalyticsPTPanel({
  isPublic = false,
}: StatusLulusanAnalyticsPTPanelProps) {
  const { user } = useAuth();
  const currentYear = new Date().getFullYear();
  const [startYear, setStartYear] = useState(currentYear - 3);
  const [endYear, setEndYear] = useState(currentYear);
  const [jenjang, setJenjang] = useState("ALL");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [isPdfDownloading, setIsPdfDownloading] = useState(false);
  const [pdfError, setPdfError] = useState("");
  const pageSize = 10;

  const endpoint = isPublic
    ? "/api/public/status-lulusan/analytics-pt"
    : "/api/siswa/status-lulusan/analytics-pt";

  const yearOptions = useMemo(() => {
    const years: number[] = [];
    for (let year = currentYear - 7; year <= currentYear + 1; year += 1) {
      years.push(year);
    }
    return years;
  }, [currentYear]);

  const jenjangOptions = useMemo(() => {
    if (user?.role === "SCHOOL_ADMIN") {
      return ["ALL"];
    }
    if (user?.role === "ADMIN") {
      const allowed = (user.allowedLevels ?? [])
        .map((value) => value.toUpperCase())
        .filter((value) => ["SMA", "SMK", "SLB"].includes(value));
      return ["ALL", ...Array.from(new Set(allowed))];
    }
    return ["ALL", "SMA", "SMK", "SLB"];
  }, [user?.allowedLevels, user?.role]);

  const effectiveStartYear = Math.min(startYear, endYear);
  const effectiveEndYear = Math.max(startYear, endYear);

  const {
    data = emptyPayload(page, pageSize),
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: [
      "status-lulusan-analytics-pt",
      endpoint,
      effectiveStartYear,
      effectiveEndYear,
      jenjang,
      search,
      page,
      pageSize,
      isPublic,
    ],
    queryFn: () =>
      fetchAnalytics(
        endpoint,
        {
          startYear: effectiveStartYear,
          endYear: effectiveEndYear,
          jenjang,
          search,
          page,
          pageSize,
        },
        isPublic
      ),
    retry: 0,
  });

  const breakdownData = useMemo(() => {
    const aggregated = new Map<
      string,
      { jalur: string; pendaftar: number; diterima: number; acceptanceRate: number }
    >();
    for (const jalur of JALUR_ORDER) {
      aggregated.set(jalur, {
        jalur,
        pendaftar: 0,
        diterima: 0,
        acceptanceRate: 0,
      });
    }
    for (const item of data.series) {
      const jalur = normalizeJalur(item.jalur);
      const current = aggregated.get(jalur)!;
      current.pendaftar += item.pendaftar;
      current.diterima += item.diterima;
      current.acceptanceRate =
        current.pendaftar > 0
          ? (current.diterima / current.pendaftar) * 100
          : 0;
    }
    return JALUR_ORDER.map((jalur) => aggregated.get(jalur)!);
  }, [data.series]);

  const yearlyTrendData = useMemo(
    () =>
      [...data.totalsByYear].sort((left, right) => left.tahunLulus - right.tahunLulus),
    [data.totalsByYear]
  );

  const totalPendaftarRange = yearlyTrendData.reduce(
    (sum, row) => sum + row.pendaftar,
    0
  );
  const totalDiterimaRange = yearlyTrendData.reduce(
    (sum, row) => sum + row.diterima,
    0
  );
  const acceptanceRateRange =
    totalPendaftarRange > 0
      ? (totalDiterimaRange / totalPendaftarRange) * 100
      : 0;
  const totalYears = yearlyTrendData.length;
  const hasBreakdownData = breakdownData.some(
    (item) => item.pendaftar > 0 || item.diterima > 0
  );
  const hasYearlyTrendData = yearlyTrendData.length > 0;

  const handleDownloadPdf = async () => {
    if (isPublic) return;
    setIsPdfDownloading(true);
    setPdfError("");
    try {
      const query = new URLSearchParams();
      query.set("tab", "analytics");
      query.set("start_year", String(effectiveStartYear));
      query.set("end_year", String(effectiveEndYear));
      if (jenjang !== "ALL") query.set("jenjang", jenjang);

      const response = await apiFetch(`/api/reports/siswa.pdf?${query.toString()}`);
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(response, "Gagal mengunduh PDF analytics.")
        );
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.setAttribute(
        "download",
        getFilenameFromContentDisposition(
          response.headers.get("content-disposition"),
          "siswa-analytics-pt.pdf"
        )
      );
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch (error) {
      setPdfError(error instanceof Error ? error.message : "Gagal mengunduh PDF analytics.");
    } finally {
      setIsPdfDownloading(false);
    }
  };

  return (
    <section className="space-y-8">
      <PageHeader
        eyebrow="Siswa"
        title="Analytics PT"
        description="Tampilan analytics diterima perguruan tinggi yang sama untuk SCHOOL_ADMIN, CABDIN, dan ADMIN_MASTER. Perbedaan hanya pada scope data sesuai role."
      />

      <Card>
        <CardContent className="space-y-4 p-4 sm:p-5">
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Start Year
              </p>
              <Select
                value={String(startYear)}
                onValueChange={(value) => {
                  setStartYear(Number(value));
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {yearOptions.map((year) => (
                    <SelectItem key={year} value={String(year)}>
                      {year}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                End Year
              </p>
              <Select
                value={String(endYear)}
                onValueChange={(value) => {
                  setEndYear(Number(value));
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {yearOptions.map((year) => (
                    <SelectItem key={year} value={String(year)}>
                      {year}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Jenjang
              </p>
              <Select
                value={jenjang}
                onValueChange={(value) => {
                  setJenjang(value);
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {jenjangOptions.map((value) => (
                    <SelectItem key={value} value={value}>
                      {value}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {!isPublic ? (
            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Cari Detail
              </p>
              <input
                type="text"
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                  setPage(1);
                }}
                placeholder="Sekolah, NISN, nama siswa, PT, prodi"
                className="h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm"
              />
            </div>
            ) : null}
          </div>

          {!isPublic ? (
            <div className="flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => void handleDownloadPdf()}
                disabled={isPdfDownloading}
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isPdfDownloading ? "Menyiapkan..." : "Unduh PDF (Tab ini)"}
              </button>
              {pdfError ? <p className="text-xs text-red-600">{pdfError}</p> : null}
            </div>
          ) : null}
        </CardContent>
      </Card>

      {isError ? (
        <EmptyState
          title="Gagal memuat Analytics PT"
          description="Silakan coba lagi. Scope data dan filter yang sedang dipakai tidak berubah."
          action={
            <button
              type="button"
              onClick={() => void refetch()}
              className="rounded-lg border border-red-200 bg-white px-3 py-1.5 text-xs font-semibold text-red-700 transition hover:bg-red-50"
            >
              Coba lagi
            </button>
          }
          className="border-red-200 bg-red-50/30"
        />
      ) : null}

      <SectionWrapper
        title="Ringkasan Utama"
        description="Ringkasan total pendaftar, total diterima, dan acceptance rate pada rentang tahun yang sedang dipantau."
      >
        {isLoading ? (
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            {Array.from({ length: 4 }).map((_, index) => (
              <Skeleton key={`analytics-pt-summary-${index}`} className="h-28 border-0" />
            ))}
          </div>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            {[
              {
                label: "Total Pendaftar PT",
                value: formatNumber(totalPendaftarRange),
                hint: "Akumulasi seluruh jalur pada rentang terpilih",
              },
              {
                label: "Total Diterima",
                value: formatNumber(totalDiterimaRange),
                hint: "Siswa final status PT pada rentang terpilih",
              },
              {
                label: "Acceptance Rate",
                value: formatPercent(acceptanceRateRange),
                hint:
                  totalPendaftarRange > 0
                    ? `${formatNumber(totalDiterimaRange)} dari ${formatNumber(totalPendaftarRange)} pendaftar`
                    : "Belum ada data pendaftar",
              },
              {
                label: "Cakupan Tahun",
                value: `${effectiveStartYear} - ${effectiveEndYear}`,
                hint:
                  totalYears > 0
                    ? `${formatNumber(totalYears)} tahun memiliki data`
                    : "Belum ada tahun terdata",
              },
            ].map((item) => (
              <Card key={item.label}>
                <CardContent className="space-y-2 p-4 sm:p-5">
                  <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    {item.label}
                  </p>
                  <p className="text-3xl font-semibold tracking-tight text-slate-900">
                    {item.value}
                  </p>
                  <p className="text-xs text-slate-500">{item.hint}</p>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </SectionWrapper>

      <SectionWrapper
        title="Breakdown Jalur"
        description="Perbandingan pendaftar, diterima, dan acceptance rate per jalur masuk."
      >
        {isLoading ? (
          <div className="grid gap-4 xl:grid-cols-2">
            <Skeleton className="h-[360px] border-0" />
            <Skeleton className="h-[360px] border-0" />
          </div>
        ) : !hasBreakdownData ? (
          <EmptyState
            title="Belum ada breakdown jalur"
            description="Section ini tetap muncul agar sekolah, cabdin, dan admin master membaca pola jalur yang sama saat data tersedia."
          />
        ) : (
          <div className="grid gap-4 xl:grid-cols-2">
            <Card>
              <CardContent className="space-y-4 p-4 sm:p-5">
                <div className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Visual Jalur
                  </p>
                  <p className="text-sm text-slate-500">
                    Pendaftar dan diterima dibandingkan langsung pada setiap jalur.
                  </p>
                </div>
                <div className="h-[280px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={breakdownData}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                      <XAxis dataKey="jalur" />
                      <YAxis allowDecimals={false} />
                      <Tooltip
                        formatter={(value) => `${formatNumber(toNumber(value))} siswa`}
                      />
                      <Legend />
                      <Bar dataKey="pendaftar" name="Pendaftar" fill="#0284c7" radius={[6, 6, 0, 0]} />
                      <Bar dataKey="diterima" name="Diterima" fill="#0f766e" radius={[6, 6, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="space-y-3 p-4 sm:p-5">
                <div className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Ringkasan Jalur
                  </p>
                  <p className="text-sm text-slate-500">
                    Summary numerik agar pendaftar, diterima, dan rate mudah dibandingkan.
                  </p>
                </div>
                <div className="space-y-3">
                  {breakdownData.map((item) => (
                    <div
                      key={item.jalur}
                      className="rounded-2xl border border-slate-200 bg-slate-50/70 p-4"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <p className="text-sm font-semibold text-slate-900">
                          {item.jalur}
                        </p>
                        <span className="text-xs font-semibold text-slate-600">
                          {formatPercent(item.acceptanceRate)}
                        </span>
                      </div>
                      <div className="mt-3 grid gap-3 text-sm text-slate-600 sm:grid-cols-2">
                        <div className="rounded-xl bg-white px-3 py-2">
                          Pendaftar: {formatNumber(item.pendaftar)}
                        </div>
                        <div className="rounded-xl bg-white px-3 py-2">
                          Diterima: {formatNumber(item.diterima)}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </div>
        )}
      </SectionWrapper>

      <SectionWrapper
        title="Tren Tahunan"
        description="Pendaftar, diterima, dan acceptance rate tahunan pada scope role yang sedang aktif."
      >
        {isLoading ? (
          <div className="grid gap-4 xl:grid-cols-2">
            <Skeleton className="h-[360px] border-0" />
            <Skeleton className="h-[360px] border-0" />
          </div>
        ) : !hasYearlyTrendData ? (
          <EmptyState
            title="Belum ada tren tahunan"
            description="Ubah rentang tahun bila perlu. Section tetap sama untuk semua role meski belum ada datanya."
          />
        ) : (
          <div className="grid gap-4 xl:grid-cols-2">
            <Card>
              <CardContent className="space-y-4 p-4 sm:p-5">
                <div className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Pendaftar vs Diterima
                  </p>
                  <p className="text-sm text-slate-500">
                    Membaca pertumbuhan volume pendaftar dan hasil diterima dari tahun ke tahun.
                  </p>
                </div>
                <div className="h-[280px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={yearlyTrendData}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                      <XAxis dataKey="tahunLulus" />
                      <YAxis allowDecimals={false} />
                      <Tooltip
                        labelFormatter={(value) => `Tahun ${value}`}
                        formatter={(value, name) => [
                          `${formatNumber(toNumber(value))} siswa`,
                          name,
                        ]}
                      />
                      <Legend />
                      <Line
                        type="monotone"
                        dataKey="pendaftar"
                        name="Pendaftar"
                        stroke="#0284c7"
                        strokeWidth={2.5}
                        dot={false}
                      />
                      <Line
                        type="monotone"
                        dataKey="diterima"
                        name="Diterima"
                        stroke="#0f766e"
                        strokeWidth={2.5}
                        dot={false}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="space-y-4 p-4 sm:p-5">
                <div className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Acceptance Rate Tahunan
                  </p>
                  <p className="text-sm text-slate-500">
                    Persentase diterima per tahun untuk membaca kualitas hasil penerimaan.
                  </p>
                </div>
                <div className="h-[280px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={yearlyTrendData}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                      <XAxis dataKey="tahunLulus" />
                      <YAxis domain={[0, 100]} tickFormatter={(value) => `${value}%`} />
                      <Tooltip
                        labelFormatter={(value) => `Tahun ${value}`}
                        formatter={(value) => formatPercent(toNumber(value))}
                      />
                      <Bar
                        dataKey="acceptanceRate"
                        name="Acceptance Rate"
                        fill="#d97706"
                        radius={[6, 6, 0, 0]}
                      />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
          </div>
        )}
      </SectionWrapper>

      <SectionWrapper
        title="Persebaran Perguruan Tinggi dan Prodi"
        description="Sekolah, cabdin, dan admin master melihat section yang sama untuk membaca persebaran siswa diterima di PT dan prodi tujuan."
      >
        {isLoading ? (
          <div className="grid gap-4 xl:grid-cols-2">
            <Skeleton className="h-[380px] border-0" />
            <Skeleton className="h-[380px] border-0" />
          </div>
        ) : (
          <div className="grid gap-4 xl:grid-cols-2">
            <DistributionList
              title="Persebaran PT"
              description="Daftar PT tujuan dengan jumlah siswa diterima terbanyak pada scope aktif."
              items={data.topKampus}
              emptyTitle="Belum ada sebaran PT"
            />
            <DistributionList
              title="Persebaran Prodi"
              description="Daftar prodi tujuan dengan jumlah siswa diterima terbanyak pada scope aktif."
              items={data.topProdi}
              emptyTitle="Belum ada sebaran prodi"
            />
          </div>
        )}
      </SectionWrapper>

      {!isPublic ? (
      <SectionWrapper
        title="Tabel Detail Penerimaan"
        description="Kolom dan layout tabel detail sama untuk SCHOOL_ADMIN, CABDIN, dan ADMIN_MASTER. Scope data tetap dibatasi backend sesuai role."
      >
        <Card>
          <CardContent className="space-y-4 p-4 sm:p-5">
            <DataTable
              columns={[
                { key: "school", label: "Sekolah" },
                { key: "nisn", label: "NISN" },
                { key: "student", label: "Nama Siswa" },
                { key: "rombel", label: "Rombel" },
                { key: "tahun", label: "Tahun Lulus", align: "right" },
                { key: "jalur", label: "Jalur" },
                { key: "pt", label: "Nama PT" },
                { key: "prodi", label: "Jurusan/Prodi" },
                { key: "tahunDiterima", label: "Tahun Diterima", align: "right" },
              ]}
              rows={data.detailRows}
              isLoading={isLoading}
              isError={isError}
              emptyMessage="Belum ada data penerimaan perguruan tinggi pada scope ini."
              tableWrapperClassName="max-h-[520px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
              tableHeaderClassName="sticky top-0 z-10 bg-white"
              page={data.detailMeta.page || page}
              pageSize={data.detailMeta.pageSize || pageSize}
              total={data.detailMeta.total}
              onPageChange={setPage}
              renderRow={(row) => (
                <TableRow
                  key={`${row.schoolName}-${row.nisn}-${row.tahunLulus}-${row.namaPt}-${row.prodi}`}
                >
                  <TableCell>{row.schoolName || "-"}</TableCell>
                  <TableCell>{row.nisn || "-"}</TableCell>
                  <TableCell>{row.namaSiswa || "-"}</TableCell>
                  <TableCell>{row.rombel || "-"}</TableCell>
                  <TableCell className="text-right">
                    {row.tahunLulus || "-"}
                  </TableCell>
                  <TableCell>
                    <span
                      className="inline-flex rounded-full px-2 py-1 text-xs font-semibold"
                      style={{
                        backgroundColor: `${JALUR_COLORS[row.jalur] ?? "#64748b"}1A`,
                        color: JALUR_COLORS[row.jalur] ?? "#475569",
                      }}
                    >
                      {row.jalur || "-"}
                    </span>
                  </TableCell>
                  <TableCell>{row.namaPt || "-"}</TableCell>
                  <TableCell>{row.prodi || "-"}</TableCell>
                  <TableCell className="text-right">
                    {row.tahunDiterima ?? "-"}
                  </TableCell>
                </TableRow>
              )}
            />
          </CardContent>
        </Card>
      </SectionWrapper>
      ) : null}
    </section>
  );
}
