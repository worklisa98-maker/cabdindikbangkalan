"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";

import { Card, CardContent } from "@/components/ui/card";
import ImportTemplateActions from "@/components/import-template-actions";
import { AnalyticsFilterBar } from "@/components/analytics/AnalyticsFilterBar";
import { DataTable } from "@/components/data-table";
import { TableFilterBar } from "@/components/table-filter-bar";
import { TableCell, TableRow } from "@/components/ui/table";
import { ConfirmPasswordModal } from "@/components/confirm-password-modal";
import { PrestasiDistributionTable } from "@/components/prestasi/PrestasiDistributionTable";
import { PrestasiTopSchoolsChart } from "@/components/prestasi/PrestasiTopSchoolsChart";
import { PrestasiSchoolDetailTable } from "@/components/prestasi/PrestasiSchoolDetailTable";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import {
  achievementCategories,
  achievementChartColors,
  jenjangOptions,
  tingkatOptions,
  type AchievementCategory,
  type JenjangOption,
  type TingkatOption,
} from "@/lib/achievementsConfig";

type AnalyticsKeyTotal = {
  key: string;
  total: number;
};

type AnalyticsTingkatYearPoint = {
  tingkat: string;
  year: number;
  total: number;
};

type AnalyticsJenisYearPoint = {
  jenis: string;
  year: number;
  total: number;
};

type AnalyticsCategoryDonutPoint = {
  kategori: string;
  total: number;
};

type AnalyticsYearPoint = {
  year: number;
  total: number;
};

type PrestasiAnalyticsResponse = {
  availableYears: number[];
  latestYear: number | null;
  kpis: {
    total: number;
    yoyPercent: number | null;
    currentYearTotal: number;
    prevYearTotal: number;
  };
  trendByYear: AnalyticsYearPoint[];
  distribution: {
    byTingkat?: AnalyticsKeyTotal[];
    byJenisLomba?: AnalyticsKeyTotal[];
  };
  distributionByYear?: {
    byTingkat?: AnalyticsTingkatYearPoint[];
    byJenisPeserta?: AnalyticsJenisYearPoint[];
  };
  categoryDonutSiswa?: AnalyticsCategoryDonutPoint[];
  topSchools?: {
    school_id: number;
    school_name: string;
    total: number;
    yoyPercent: number | null;
  }[];
};

type PrestasiLegacyCategoryTrend = {
  category: string;
  year: string;
  total: number;
};

type PrestasiLegacyDistribusi = {
  jenjang: string;
  tingkat: string;
  total: number;
};

type PrestasiLegacyTopSchool = {
  school: string;
  total: number;
};

type PrestasiLegacySchoolDetail = {
  school: string;
  tingkat: string;
  total: number;
};

type PrestasiLegacyResponse = {
  categoryTrends: PrestasiLegacyCategoryTrend[];
  distribusiJenjangTingkat: PrestasiLegacyDistribusi[];
  topSchools: PrestasiLegacyTopSchool[];
  schoolDetails: PrestasiLegacySchoolDetail[];
  availableYears: string[];
};

type PrestasiAdminSummaryResponse = {
  rekapJenjangTingkat: {
    jenjang: string;
    kabupaten: number;
    provinsi: number;
    nasional: number;
    internasional: number;
  }[];
  detailSekolah: {
    nama_sekolah: string;
    kabupaten: number;
    provinsi: number;
    nasional: number;
    internasional: number;
    total: number;
  }[];
};

type AchievementListItem = {
  idPrestasi: string;
  schoolId: number;
  namaSekolah: string;
  namaLomba: string;
  tingkatLomba: string;
  kategoriPrestasi: string;
  peringkatPrestasi: string;
  namaPeserta: string;
  jenisLomba: string;
  tahunLomba: string;
};

type ListMeta = {
  page: number;
  pageSize: number;
  total: number;
};

type AchievementListResponse = {
  data: AchievementListItem[];
  meta: ListMeta;
};

type SchoolSuggestion = {
  id: number;
  nama_sekolah: string;
  npsn?: string | null;
};

const jenisPesertaOptions = ["ALL", "Siswa", "Guru", "Tendik"] as const;
const tingkatFilterOptions = ["ALL", ...tingkatOptions] as const;

const formatNumber = (value: number) =>
  new Intl.NumberFormat("id-ID").format(value);

const formatPercent = (value?: number | null) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "N/A";
  return `${value.toFixed(1)}%`;
};

const maxYearSeries = 5;
const yearSeriesColors = ["#1d4ed8", "#0f766e", "#f59e0b", "#0ea5e9", "#ef4444"];
const donutColors = [
  "#1d4ed8",
  "#0f766e",
  "#f59e0b",
  "#0ea5e9",
  "#ef4444",
  "#22c55e",
  "#6366f1",
  "#ec4899",
];

const normalizeLegacyTingkat = (value: string): TingkatOption | null => {
  const normalized = value.trim().toUpperCase();
  switch (normalized) {
    case "KABUPATEN":
      return "Kabupaten";
    case "PROVINSI":
      return "Provinsi";
    case "NASIONAL":
      return "Nasional";
    case "INTERNASIONAL":
      return "Internasional";
    default:
      return null;
  }
};

const normalizeTingkatLabel = (value: string) => {
  const normalized = value.trim().toUpperCase();
  switch (normalized) {
    case "KABUPATEN":
      return "Kabupaten";
    case "PROVINSI":
      return "Provinsi";
    case "NASIONAL":
      return "Nasional";
    case "INTERNASIONAL":
      return "Internasional";
    default:
      return value;
  }
};

const normalizeJenisLabel = (value: string) => {
  const normalized = value.trim().toUpperCase();
  switch (normalized) {
    case "SISWA":
      return "Siswa";
    case "GURU":
      return "Guru";
    case "TENDIK":
      return "Tendik";
    case "LAINNYA":
      return "Lainnya";
    default:
      return value;
  }
};

const fetchPrestasiAnalytics = async (
  params: {
    year: string;
    jenjang: string;
    status: string;
    schoolId?: number | null;
    jenisLomba?: string;
  },
  isPublic: boolean
): Promise<PrestasiAnalyticsResponse> => {
  const query = new URLSearchParams();
  if (params.year && params.year !== "ALL") query.set("year", params.year);
  if (params.jenjang && params.jenjang !== "ALL") {
    query.set("jenjang", params.jenjang);
  }
  if (params.status && params.status !== "ALL") {
    query.set("status", params.status);
  }
  if (params.schoolId) {
    query.set("school_id", String(params.schoolId));
  }
  if (params.jenisLomba && params.jenisLomba !== "ALL") {
    query.set("jenis_lomba", params.jenisLomba);
  }

  const endpoint = isPublic
    ? "/api/public/prestasi/analytics"
    : "/api/prestasi/analytics";
  const response = await apiFetch(
    query.toString() ? `${endpoint}?${query.toString()}` : endpoint,
    undefined,
    isPublic ? { skipAuthRedirect: true } : undefined
  );
  if (!response.ok) {
    throw new Error("Failed to fetch prestasi analytics.");
  }
  const payload = (await response.json()) as { data: PrestasiAnalyticsResponse };
  return payload.data;
};

const fetchPrestasiLegacy = async (params: {
  year: string;
  jenjang: string;
  kategori: string;
  status: string;
  schoolId?: number | null;
  role: "ADMIN" | "ADMIN_MASTER" | "SCHOOL_ADMIN" | "PUBLIC";
}): Promise<PrestasiLegacyResponse> => {
  const isAdmin = params.role === "ADMIN" || params.role === "ADMIN_MASTER";
  const isSchoolAdmin = params.role === "SCHOOL_ADMIN";

  if (isAdmin || isSchoolAdmin) {
    const query = new URLSearchParams();
    if (params.year && params.year !== "ALL") {
      query.set("year", params.year);
    }
    if (params.jenjang && params.jenjang !== "ALL") {
      query.set("bentuk", params.jenjang);
    }
    if (params.kategori && params.kategori !== "ALL") {
      query.set("kategori", params.kategori);
    }
    if (params.status && params.status !== "ALL") {
      query.set("status", params.status);
    }
    if (isAdmin && params.schoolId) {
      query.set("schoolId", String(params.schoolId));
    }

    const endpoint = "/api/prestasi/admin/summary";
    const response = await apiFetch(
      query.toString() ? `${endpoint}?${query.toString()}` : endpoint
    );
    if (!response.ok) {
      throw new Error("Failed to fetch prestasi admin summary.");
    }
    const payload = (await response.json()) as {
      data: PrestasiAdminSummaryResponse;
    };
    const summary = payload.data ?? {
      rekapJenjangTingkat: [],
      detailSekolah: [],
    };
    const distribusiJenjangTingkat = summary.rekapJenjangTingkat.flatMap(
      (row) => [
        { jenjang: row.jenjang, tingkat: "Kabupaten", total: row.kabupaten ?? 0 },
        { jenjang: row.jenjang, tingkat: "Provinsi", total: row.provinsi ?? 0 },
        { jenjang: row.jenjang, tingkat: "Nasional", total: row.nasional ?? 0 },
        {
          jenjang: row.jenjang,
          tingkat: "Internasional",
          total: row.internasional ?? 0,
        },
      ]
    );
    const schoolDetails = summary.detailSekolah.flatMap((row) => [
      { school: row.nama_sekolah, tingkat: "Kabupaten", total: row.kabupaten ?? 0 },
      { school: row.nama_sekolah, tingkat: "Provinsi", total: row.provinsi ?? 0 },
      { school: row.nama_sekolah, tingkat: "Nasional", total: row.nasional ?? 0 },
      {
        school: row.nama_sekolah,
        tingkat: "Internasional",
        total: row.internasional ?? 0,
      },
    ]);
    const topSchools = summary.detailSekolah
      .map((row) => ({
        school: row.nama_sekolah,
        total: row.total ?? 0,
      }))
      .sort((a, b) => b.total - a.total)
      .slice(0, 10);

    return {
      categoryTrends: [],
      distribusiJenjangTingkat,
      topSchools,
      schoolDetails,
      availableYears: [],
    };
  }

  const query = new URLSearchParams();
  if (params.year && params.year !== "ALL") {
    query.set("tahun", params.year);
  }
  if (params.jenjang && params.jenjang !== "ALL") {
    query.set("bentuk", params.jenjang);
  }
  if (params.kategori && params.kategori !== "ALL") {
    query.set("kategori", params.kategori);
  }
  if (params.status && params.status !== "ALL") {
    query.set("status", params.status);
  }

  const endpoint = "/api/public/prestasi/full";
  const response = await apiFetch(
    query.toString() ? `${endpoint}?${query.toString()}` : endpoint,
    undefined,
    { skipAuthRedirect: true }
  );
  if (!response.ok) {
    throw new Error("Failed to fetch prestasi legacy summary.");
  }
  const payload = (await response.json()) as { data: PrestasiLegacyResponse };
  return payload.data;
};

const fetchAchievementList = async (params: {
  page: number;
  pageSize: number;
  year: string;
  tingkat: string;
  jenisPeserta: string;
  bentuk: string;
  schoolId?: number | null;
  search: string;
}): Promise<AchievementListResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.year && params.year !== "ALL") {
    query.set("year", params.year);
  }
  if (params.tingkat && params.tingkat !== "ALL") {
    query.set("tingkat", params.tingkat);
  }
  if (params.jenisPeserta && params.jenisPeserta !== "ALL") {
    query.set("jenis_peserta", params.jenisPeserta);
  }
  if (params.bentuk && params.bentuk !== "ALL") {
    query.set("bentuk", params.bentuk);
  }
  if (params.schoolId) {
    query.set("sekolahId", String(params.schoolId));
  }
  if (params.search) {
    query.set("q", params.search);
  }

  const response = await apiFetch(`/api/prestasi/list?${query.toString()}`);
  if (!response.ok) {
    throw new Error("Failed to fetch achievement list.");
  }
  const payload = (await response.json()) as { data: AchievementListResponse };
  return payload.data;
};

const fetchSchoolSuggestions = async (
  query: string
): Promise<SchoolSuggestion[]> => {
  const params = new URLSearchParams();
  params.set("q", query);
  const response = await apiFetch(`/api/admin/schools/suggest?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch school suggestions.");
  }
  const payload = (await response.json()) as { data: SchoolSuggestion[] };
  return payload.data ?? [];
};

export default function PrestasiPage() {
  return (
    <Suspense
      fallback={<div className="p-4 text-sm text-zinc-500">Memuat...</div>}
    >
      <PrestasiPageInner />
    </Suspense>
  );
}

function PrestasiPageInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { status, user } = useAuth();
  const isPublic = status === "unauthenticated";
  const role = isPublic ? "PUBLIC" : user?.role ?? "PUBLIC";
  const isAdmin = role === "ADMIN" || role === "ADMIN_MASTER";
  const isSchoolAdmin = role === "SCHOOL_ADMIN";
  const [isHydrated, setIsHydrated] = useState(false);
  useEffect(() => {
    setIsHydrated(true);
  }, []);
  const isHydrating = !isHydrated || status === "loading";
  const showHistoryButton = !isHydrating && (isAdmin || isSchoolAdmin);

  const filters = useMemo(() => {
    const year = searchParams.get("year") ?? "ALL";
    const jenjang = searchParams.get("jenjang") ?? "ALL";
    const statusParam = searchParams.get("status") ?? "ALL";
    const jenisLomba = searchParams.get("jenis_lomba") ?? "ALL";
    const schoolIdRaw = searchParams.get("school_id");
    const schoolId = schoolIdRaw ? Number(schoolIdRaw) : null;
    return {
      year,
      jenjang,
      status: statusParam,
      schoolId: Number.isFinite(schoolId) && schoolId && schoolId > 0 ? schoolId : null,
      jenisLomba,
    };
  }, [searchParams]);

  const selectedYear = useMemo(() => {
    if (!filters.year || filters.year === "ALL") return null;
    const parsed = Number(filters.year);
    return Number.isFinite(parsed) ? parsed : null;
  }, [filters.year]);

  const comparisonYear = useMemo(() => {
    if (selectedYear == null) return null;
    const prev = selectedYear - 1;
    return prev > 0 ? prev : null;
  }, [selectedYear]);

  const [schoolQuery, setSchoolQuery] = useState("");
  useEffect(() => {
    if (filters.schoolId && !schoolQuery) {
      setSchoolQuery(`ID ${filters.schoolId}`);
    }
  }, [filters.schoolId, schoolQuery]);

  const debouncedSchoolQuery = useDebouncedValue(schoolQuery, 300);
  const { data: schoolSuggestions = [], isLoading: isSchoolLoading } = useQuery({
    queryKey: ["school-suggest", debouncedSchoolQuery],
    queryFn: () => fetchSchoolSuggestions(debouncedSchoolQuery),
    enabled: !isHydrating && isAdmin && debouncedSchoolQuery.trim().length >= 2,
  });

  const handleFilterChange = useCallback(
    (next: Partial<{
      year: string;
      jenjang: string;
      status: string;
      schoolId: number | null;
      jenisLomba: string;
    }>) => {
      const params = new URLSearchParams(searchParams.toString());
      const merged = {
        year: filters.year,
        jenjang: filters.jenjang,
        status: filters.status,
        schoolId: filters.schoolId,
        jenisLomba: filters.jenisLomba,
        ...next,
      };
      const setParam = (key: string, value: string) => {
        if (!value || value === "ALL") {
          params.delete(key);
        } else {
          params.set(key, value);
        }
      };

      setParam("year", merged.year);
      setParam("jenjang", merged.jenjang);
      setParam("status", merged.status);
      setParam("jenis_lomba", merged.jenisLomba);

      if (merged.schoolId) {
        params.set("school_id", String(merged.schoolId));
      } else {
        params.delete("school_id");
      }

      const nextQuery = params.toString();
      router.replace(nextQuery ? `?${nextQuery}` : "?", { scroll: false });
    },
    [filters, router, searchParams]
  );

  const {
    data: analytics,
    isLoading: isAnalyticsLoading,
    isError: isAnalyticsError,
    refetch: refetchAnalytics,
  } = useQuery({
    queryKey: [
      "prestasi-analytics",
      role,
      filters.year,
      filters.jenjang,
      filters.status,
      filters.schoolId,
    ],
    queryFn: () =>
      fetchPrestasiAnalytics(
        {
          year: filters.year,
          jenjang: filters.jenjang,
          status: filters.status,
          schoolId: isAdmin ? filters.schoolId : null,
          jenisLomba: "ALL",
        },
        isPublic
      ),
    enabled: !isHydrating,
  });

  const shouldFetchComparison =
    !isHydrating && selectedYear != null && comparisonYear != null;
  const {
    data: comparisonAnalytics,
    isLoading: isComparisonLoading,
  } = useQuery({
    queryKey: [
      "prestasi-analytics-compare",
      role,
      comparisonYear,
      filters.jenjang,
      filters.status,
      filters.schoolId,
    ],
    queryFn: () =>
      fetchPrestasiAnalytics(
        {
          year: comparisonYear ? String(comparisonYear) : "ALL",
          jenjang: filters.jenjang,
          status: filters.status,
          schoolId: isAdmin ? filters.schoolId : null,
          jenisLomba: "ALL",
        },
        isPublic
      ),
    enabled: shouldFetchComparison,
  });

  const {
    data: distributionAnalytics,
    isLoading: isDistributionLoading,
    isError: isDistributionError,
    refetch: refetchDistribution,
  } = useQuery({
    queryKey: [
      "prestasi-analytics-distribution",
      role,
      filters.year,
      filters.jenjang,
      filters.status,
      filters.schoolId,
      filters.jenisLomba,
    ],
    queryFn: () =>
      fetchPrestasiAnalytics(
        {
          year: filters.year,
          jenjang: filters.jenjang,
          status: filters.status,
          schoolId: isAdmin ? filters.schoolId : null,
          jenisLomba: filters.jenisLomba ?? "ALL",
        },
        isPublic
      ),
    enabled: !isHydrating,
  });

  const {
    data: legacyData,
    isLoading: isLegacyLoading,
    isError: isLegacyError,
    refetch: refetchLegacy,
  } = useQuery({
    queryKey: [
      "prestasi-legacy",
      role,
      filters.year,
      filters.jenjang,
      filters.status,
      filters.schoolId,
    ],
    queryFn: () =>
      fetchPrestasiLegacy({
        year: filters.year,
        jenjang: filters.jenjang,
        kategori: "ALL",
        status: filters.status,
        schoolId: isAdmin ? filters.schoolId : null,
        role: role as "ADMIN" | "ADMIN_MASTER" | "SCHOOL_ADMIN" | "PUBLIC",
      }),
    enabled: !isHydrating,
  });

  const {
    data: distributionLegacyData,
    isLoading: isDistributionLegacyLoading,
    isError: isDistributionLegacyError,
    refetch: refetchDistributionLegacy,
  } = useQuery({
    queryKey: [
      "prestasi-legacy-distribution",
      role,
      filters.year,
      filters.jenjang,
      filters.status,
      filters.schoolId,
      filters.jenisLomba,
    ],
    queryFn: () =>
      fetchPrestasiLegacy({
        year: filters.year,
        jenjang: filters.jenjang,
        kategori: filters.jenisLomba,
        status: filters.status,
        schoolId: isAdmin ? filters.schoolId : null,
        role: role as "ADMIN" | "ADMIN_MASTER" | "SCHOOL_ADMIN" | "PUBLIC",
      }),
    enabled: !isHydrating,
  });

  const yearOptions = useMemo(() => {
    if (analytics?.availableYears?.length) {
      return analytics.availableYears;
    }
    const fallbackYear = filters.year !== "ALL" ? Number(filters.year) : null;
    return fallbackYear ? [fallbackYear] : [];
  }, [analytics?.availableYears, filters.year]);

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
      distribution: analytics?.distribution ?? {},
      distributionByYear: analytics?.distributionByYear ?? {},
      categoryDonutSiswa: analytics?.categoryDonutSiswa ?? [],
      topSchools: analytics?.topSchools ?? [],
    }),
    [analytics]
  );

  const distributionAnalyticsSafe = useMemo(
    () => ({
      availableYears: distributionAnalytics?.availableYears ?? [],
      latestYear: distributionAnalytics?.latestYear ?? null,
      distribution: distributionAnalytics?.distribution ?? {},
      distributionByYear: distributionAnalytics?.distributionByYear ?? {},
      categoryDonutSiswa: distributionAnalytics?.categoryDonutSiswa ?? [],
    }),
    [distributionAnalytics]
  );

  const isAllYears = filters.year === "ALL";

  const effectiveYear = selectedYear ?? analyticsSafe.latestYear ?? null;
  const effectivePrevYear =
    effectiveYear != null ? effectiveYear - 1 : null;

  const selectedYearSeries = useMemo(() => {
    if (!isAllYears) return [];
    const years = distributionAnalyticsSafe.availableYears.slice(0, maxYearSeries);
    return years.sort((a, b) => a - b);
  }, [distributionAnalyticsSafe.availableYears, isAllYears]);

  type GroupedYearRow = Record<string, number | string> & { label: string };

  const groupedTingkatData = useMemo(() => {
    if (!isAllYears || selectedYearSeries.length === 0) return [];
    const rows = new Map<string, GroupedYearRow>();
    const ensureRow = (label: string) => {
      if (!rows.has(label)) {
        const entry: GroupedYearRow = { label };
        selectedYearSeries.forEach((year) => {
          entry[String(year)] = 0;
        });
        rows.set(label, entry);
      }
    };

    tingkatOptions.forEach((label) => ensureRow(label));
    (distributionAnalyticsSafe.distributionByYear.byTingkat ?? []).forEach((item) => {
      const label = normalizeTingkatLabel(item.tingkat);
      if (!selectedYearSeries.includes(item.year)) return;
      ensureRow(label);
      const row = rows.get(label);
      if (row) {
        const yearKey = String(item.year);
        const current = typeof row[yearKey] === "number" ? row[yearKey] : 0;
        row[yearKey] = current + item.total;
      }
    });

    return Array.from(rows.values());
  }, [
    distributionAnalyticsSafe.distributionByYear.byTingkat,
    isAllYears,
    selectedYearSeries,
  ]);

  const groupedJenisData = useMemo(() => {
    if (!isAllYears || selectedYearSeries.length === 0) return [];
    const rows = new Map<string, GroupedYearRow>();
    const ensureRow = (label: string) => {
      if (!rows.has(label)) {
        const entry: GroupedYearRow = { label };
        selectedYearSeries.forEach((year) => {
          entry[String(year)] = 0;
        });
        rows.set(label, entry);
      }
    };

    ["Siswa", "Guru", "Tendik", "Lainnya"].forEach((label) => ensureRow(label));
    (distributionAnalyticsSafe.distributionByYear.byJenisPeserta ?? []).forEach((item) => {
      const label = normalizeJenisLabel(item.jenis);
      if (!selectedYearSeries.includes(item.year)) return;
      ensureRow(label);
      const row = rows.get(label);
      if (row) {
        const yearKey = String(item.year);
        const current = typeof row[yearKey] === "number" ? row[yearKey] : 0;
        row[yearKey] = current + item.total;
      }
    });

    return Array.from(rows.values());
  }, [
    distributionAnalyticsSafe.distributionByYear.byJenisPeserta,
    isAllYears,
    selectedYearSeries,
  ]);

  const hasGroupedTingkatData = useMemo(() => {
    if (!isAllYears || selectedYearSeries.length === 0) return false;
    return groupedTingkatData.some((row) =>
      selectedYearSeries.some(
        (year) => Number(row[String(year)] ?? 0) > 0
      )
    );
  }, [groupedTingkatData, isAllYears, selectedYearSeries]);

  const hasGroupedJenisData = useMemo(() => {
    if (!isAllYears || selectedYearSeries.length === 0) return false;
    return groupedJenisData.some((row) =>
      selectedYearSeries.some(
        (year) => Number(row[String(year)] ?? 0) > 0
      )
    );
  }, [groupedJenisData, isAllYears, selectedYearSeries]);

  const hasSimpleTingkatData = useMemo(
    () =>
      (distributionAnalyticsSafe.distribution.byTingkat ?? []).some(
        (item) => item.total > 0
      ),
    [distributionAnalyticsSafe.distribution.byTingkat]
  );

  const hasSimpleJenisData = useMemo(
    () =>
      (distributionAnalyticsSafe.distribution.byJenisLomba ?? []).some(
        (item) => item.total > 0
      ),
    [distributionAnalyticsSafe.distribution.byJenisLomba]
  );

  const showDonut = filters.jenisLomba === "Siswa";
  const hasDonutData = useMemo(
    () => distributionAnalyticsSafe.categoryDonutSiswa.some((item) => item.total > 0),
    [distributionAnalyticsSafe.categoryDonutSiswa]
  );

  const distributionCardCount =
    1 + (role !== "PUBLIC" ? 1 : 0) + (showDonut ? 1 : 0);
  const distributionGridCols =
    distributionCardCount >= 3
      ? "lg:grid-cols-3"
      : distributionCardCount === 1
        ? "lg:grid-cols-1"
        : "lg:grid-cols-2";

  const currentYearLabel =
    filters.year !== "ALL"
      ? filters.year
      : analyticsSafe.latestYear
        ? String(analyticsSafe.latestYear)
        : "-";

  const hasEmptyData =
    !isHydrating && !isAnalyticsLoading && analyticsSafe.kpis.total === 0;

  const legacySafe = useMemo(
    () => ({
      categoryTrends: legacyData?.categoryTrends ?? [],
      distribusiJenjangTingkat: legacyData?.distribusiJenjangTingkat ?? [],
      topSchools: legacyData?.topSchools ?? [],
      schoolDetails: legacyData?.schoolDetails ?? [],
      availableYears: legacyData?.availableYears ?? [],
    }),
    [legacyData]
  );

  const distributionLegacySafe = useMemo(
    () => ({
      distribusiJenjangTingkat:
        distributionLegacyData?.distribusiJenjangTingkat ?? [],
      topSchools: distributionLegacyData?.topSchools ?? [],
    }),
    [distributionLegacyData]
  );

  const categoryTrendSeries = useMemo(() => {
    const yearSet = new Set<number>();
    const byCategory = new Map<string, Map<number, number>>();
    const activeCategories: readonly AchievementCategory[] = achievementCategories;

    const buildTotals = (items?: AnalyticsKeyTotal[]) => {
      const totals: Record<(typeof achievementCategories)[number], number> = {
        Siswa: 0,
        Guru: 0,
        Tendik: 0,
      };
      (items ?? []).forEach((item) => {
        const label = normalizeJenisLabel(item.key);
        if (
          achievementCategories.includes(
            label as (typeof achievementCategories)[number]
          )
        ) {
          totals[label as (typeof achievementCategories)[number]] += item.total;
        }
      });
      return totals;
    };

    const addYearTotals = (
      year: number,
      totals: Record<(typeof achievementCategories)[number], number>
    ) => {
      yearSet.add(year);
      achievementCategories.forEach((category) => {
        if (!activeCategories.includes(category)) return;
        const bucket = byCategory.get(category) ?? new Map<number, number>();
        bucket.set(year, totals[category] ?? 0);
        byCategory.set(category, bucket);
      });
    };

    if (selectedYear != null) {
      const currentTotals = buildTotals(analyticsSafe.distribution.byJenisLomba);
      const comparisonTotals = comparisonAnalytics
        ? buildTotals(comparisonAnalytics.distribution.byJenisLomba)
        : null;
      const hasComparisonData =
        comparisonTotals != null &&
        activeCategories.some((category) => comparisonTotals[category] > 0);

      if (hasComparisonData && comparisonYear != null) {
        addYearTotals(comparisonYear, comparisonTotals);
      }
      addYearTotals(selectedYear, currentTotals);
    } else {
      (analyticsSafe.distributionByYear.byJenisPeserta ?? []).forEach((item) => {
        const label = normalizeJenisLabel(item.jenis);
        if (
          !achievementCategories.includes(
            label as (typeof achievementCategories)[number]
          )
        ) {
          return;
        }
        if (
          !activeCategories.includes(label as (typeof achievementCategories)[number])
        ) {
          return;
        }
        const bucket = byCategory.get(label) ?? new Map<number, number>();
        const current = bucket.get(item.year) ?? 0;
        bucket.set(item.year, current + item.total);
        byCategory.set(label, bucket);
        yearSet.add(item.year);
      });
    }

    const years = Array.from(yearSet).sort((a, b) => a - b);
    const rows = years.map((year) => {
      const row: Record<string, number> & { year: number } = { year };
      achievementCategories.forEach((category) => {
        row[category] = activeCategories.includes(category)
          ? byCategory.get(category)?.get(year) ?? 0
          : 0;
      });
      return row;
    });
    const maxValue = rows.reduce((max, row) => {
      const rowMax = Math.max(
        ...achievementCategories.map((category) => row[category] ?? 0)
      );
      return Math.max(max, rowMax);
    }, 0);

    return {
      rows,
      maxValue: Math.max(1, maxValue),
      byCategory,
      years,
    };
  }, [analyticsSafe.distribution.byJenisLomba, analyticsSafe.distributionByYear.byJenisPeserta, comparisonAnalytics, comparisonYear, selectedYear]);
  const hasTrendData = useMemo(
    () =>
      categoryTrendSeries.rows.some((row) =>
        achievementCategories.some((category) => (row[category] ?? 0) > 0)
      ),
    [categoryTrendSeries.rows]
  );

  const latestYearTotals = useMemo(() => {
    const totals: Record<(typeof achievementCategories)[number], number> = {
      Siswa: 0,
      Guru: 0,
      Tendik: 0,
    };
    if (effectiveYear == null) return totals;
    achievementCategories.forEach((category) => {
      totals[category] =
        categoryTrendSeries.byCategory.get(category)?.get(effectiveYear) ??
        0;
    });
    return totals;
  }, [categoryTrendSeries.byCategory, effectiveYear]);

  const categoryYoYGrowth = useMemo(() => {
    const output: Record<
      (typeof achievementCategories)[number],
      { label: string; tone: string }
    > = {
      Siswa: { label: "N/A", tone: "text-zinc-500" },
      Guru: { label: "N/A", tone: "text-zinc-500" },
      Tendik: { label: "N/A", tone: "text-zinc-500" },
    };

    if (effectiveYear == null || effectivePrevYear == null) return output;

    achievementCategories.forEach((category) => {
      const current =
        categoryTrendSeries.byCategory.get(category)?.get(effectiveYear) ??
        0;
      const previous =
        categoryTrendSeries.byCategory.get(category)?.get(effectivePrevYear) ??
        0;

      if (previous === 0) {
        output[category] = { label: "N/A", tone: "text-zinc-500" };
        return;
      }

      const percent = ((current - previous) / previous) * 100;
      output[category] = {
        label: `${percent >= 0 ? "Naik" : "Turun"} ${formatPercent(Math.abs(percent))}`,
        tone: percent >= 0 ? "text-emerald-600" : "text-rose-600",
      };
    });

    return output;
  }, [categoryTrendSeries.byCategory, effectivePrevYear, effectiveYear]);

  const hasComparisonYear =
    effectivePrevYear != null &&
    categoryTrendSeries.years.includes(effectivePrevYear);

  const yoyComparisonLabel = effectiveYear && hasComparisonYear
    ? `Perbandingan ${effectivePrevYear} ke ${effectiveYear}`
    : "Perbandingan tahun sebelumnya";

  const isTrendLoading =
    isHydrating || isAnalyticsLoading || (selectedYear != null && isComparisonLoading);
  const isTrendError = isAnalyticsError;

  const legacyDistributionCounts = useMemo(() => {
    const base: Record<JenjangOption, Record<TingkatOption, number>> =
      jenjangOptions.reduce(
        (acc, jenjang) => {
          acc[jenjang] = {
            Kabupaten: 0,
            Provinsi: 0,
            Nasional: 0,
            Internasional: 0,
          };
          return acc;
        },
        {} as Record<JenjangOption, Record<TingkatOption, number>>
      );

    distributionLegacySafe.distribusiJenjangTingkat.forEach((row) => {
      const jenjang = row.jenjang?.trim().toUpperCase();
      const tingkat = normalizeLegacyTingkat(row.tingkat);
      if (!jenjang || !tingkat) return;
      if (!jenjangOptions.includes(jenjang as JenjangOption)) return;
      base[jenjang as JenjangOption][tingkat] += row.total;
    });

    return base;
  }, [distributionLegacySafe.distribusiJenjangTingkat]);

  const legacyTopSchools = useMemo(
    () =>
      distributionLegacySafe.topSchools.map((item) => ({
        school: item.school,
        total: item.total,
      })),
    [distributionLegacySafe.topSchools]
  );

  const legacySchoolDetailRows = useMemo(() => {
    const bySchool = new Map<
      string,
      { counts: Record<TingkatOption, number>; total: number }
    >();

    legacySafe.schoolDetails.forEach((item) => {
      const school = item.school?.trim();
      const tingkat = normalizeLegacyTingkat(item.tingkat);
      if (!school || !tingkat) return;

      const entry =
        bySchool.get(school) ??
        ({
          counts: {
            Kabupaten: 0,
            Provinsi: 0,
            Nasional: 0,
            Internasional: 0,
          },
          total: 0,
        } satisfies { counts: Record<TingkatOption, number>; total: number });

      entry.counts[tingkat] += item.total;
      entry.total += item.total;
      bySchool.set(school, entry);
    });

    return Array.from(bySchool.entries()).map(([school, data]) => ({
      school,
      counts: data.counts,
      total: data.total,
    }));
  }, [legacySafe.schoolDetails]);

  const [tablePage, setTablePage] = useState(1);
  const [tableSearch, setTableSearch] = useState("");
  const [tableTingkat, setTableTingkat] = useState("ALL");
  const [tableJenisPeserta, setTableJenisPeserta] = useState("ALL");
  const debouncedSearch = useDebouncedValue(tableSearch, 400);

  const {
    data: listData,
    isLoading: isListLoading,
    isError: isListError,
    refetch: refetchList,
  } = useQuery({
    queryKey: [
      "prestasi-list",
      role,
      tablePage,
      filters.year,
      filters.jenjang,
      filters.schoolId,
      tableTingkat,
      tableJenisPeserta,
      debouncedSearch,
    ],
    queryFn: () =>
      fetchAchievementList({
        page: tablePage,
        pageSize: 10,
        year: filters.year,
        tingkat: tableTingkat,
        jenisPeserta: tableJenisPeserta,
        bentuk: filters.jenjang,
        schoolId: isAdmin ? filters.schoolId : null,
        search: debouncedSearch,
      }),
    enabled: !isHydrating && status === "authenticated" && (isSchoolAdmin || isAdmin),
  });

  const tableRows = listData?.data ?? [];
  const tableMeta = listData?.meta ?? { page: 1, pageSize: 10, total: 0 };

  const tableColumns = useMemo(() => {
    const columns: Array<{
      key: string;
      label: string;
      align?: "left" | "right" | "center";
      className?: string;
    }> = [];
    if (isAdmin) {
      columns.push({ key: "namaSekolah", label: "Sekolah" });
    }
    columns.push(
      { key: "namaLomba", label: "Nama Lomba" },
      { key: "tingkatLomba", label: "Tingkat" },
      { key: "kategoriPrestasi", label: "Kategori" },
      { key: "peringkatPrestasi", label: "Peringkat" },
      { key: "namaPeserta", label: "Peserta" },
      { key: "jenisLomba", label: "Jenis" },
      { key: "tahunLomba", label: "Tahun", align: "right" as const }
    );
    if (isSchoolAdmin) {
      columns.push({ key: "actions", label: "Aksi", align: "center" as const });
    }
    return columns;
  }, [isAdmin, isSchoolAdmin]);

  const [editingRow, setEditingRow] = useState<AchievementListItem | null>(null);
  const [editForm, setEditForm] = useState({
    namaLomba: "",
    tingkatLomba: "",
    kategoriPrestasi: "",
    peringkatPrestasi: "",
    namaPeserta: "",
    jenisLomba: "",
  });
  const [editPassword, setEditPassword] = useState("");
  const [editFormError, setEditFormError] = useState("");
  const [editPasswordError, setEditPasswordError] = useState("");
  const [isEditSaving, setIsEditSaving] = useState(false);
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState("");

  const openEditModal = (row: AchievementListItem) => {
    setEditingRow(row);
    setEditForm({
      namaLomba: row.namaLomba ?? "",
      tingkatLomba: row.tingkatLomba ?? "",
      kategoriPrestasi: row.kategoriPrestasi ?? "",
      peringkatPrestasi: row.peringkatPrestasi ?? "",
      namaPeserta: row.namaPeserta ?? "",
      jenisLomba: row.jenisLomba ?? "",
    });
    setEditPassword("");
    setEditFormError("");
    setEditPasswordError("");
  };

  const closeEditModal = () => {
    setEditingRow(null);
    setIsPasswordModalOpen(false);
    setEditPassword("");
    setEditFormError("");
    setEditPasswordError("");
    setIsEditSaving(false);
  };

  const handleEditConfirm = async () => {
    if (!editingRow) return;
    if (!editPassword.trim()) {
      setEditPasswordError("Password wajib diisi.");
      return;
    }
    setIsEditSaving(true);
    setEditPasswordError("");
    setEditFormError("");

    try {
      const response = await apiFetch(`/api/prestasi/${editingRow.idPrestasi}`, {
        method: "PUT",
        body: JSON.stringify({
          password: editPassword,
          payload: {
            namaLomba: editForm.namaLomba,
            tingkatLomba: editForm.tingkatLomba,
            kategoriPrestasi: editForm.kategoriPrestasi,
            peringkatPrestasi: editForm.peringkatPrestasi,
            namaPeserta: editForm.namaPeserta,
            jenisLomba: editForm.jenisLomba,
          },
        }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        if (payload?.message === "PASSWORD_REQUIRED") {
          setEditPasswordError("Password wajib diisi.");
          return;
        }
        if (payload?.message === "PASSWORD_SALAH") {
          setEditPasswordError("Password salah.");
          return;
        }
        setEditFormError(payload?.error ?? "Gagal memperbarui data.");
        return;
      }

      closeEditModal();
      void refetchList();
      void refetchAnalytics();
      void refetchLegacy();
    } catch {
      setEditFormError("Gagal memperbarui data. Coba lagi.");
    } finally {
      setIsEditSaving(false);
    }
  };

  const handleExport = async () => {
    setIsExporting(true);
    setExportError("");
    try {
      const query = new URLSearchParams();
      if (filters.year && filters.year !== "ALL") {
        query.set("year", filters.year);
      }
      if (tableTingkat && tableTingkat !== "ALL") {
        query.set("tingkat", tableTingkat);
      }
      if (tableJenisPeserta && tableJenisPeserta !== "ALL") {
        query.set("jenis_peserta", tableJenisPeserta);
      }
      if (filters.jenjang && filters.jenjang !== "ALL") {
        query.set("bentuk", filters.jenjang);
      }
      if (filters.schoolId) {
        query.set("sekolahId", String(filters.schoolId));
      }
      if (tableSearch.trim()) {
        query.set("q", tableSearch.trim());
      }

      const response = await apiFetch(
        `/api/prestasi/export.xlsx${query.toString() ? `?${query.toString()}` : ""}`
      );
      if (!response.ok) {
        throw new Error("export failed");
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.setAttribute("download", "prestasi.xlsx");
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch {
      setExportError("Gagal mengunduh Excel prestasi.");
    } finally {
      setIsExporting(false);
    }
  };

  return (
    <section className="space-y-6">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Cabang Dinas Pendidikan Wilayah Bangkalan
          </p>
          <h1 className="text-3xl font-semibold">Dashboard Prestasi</h1>
          <p className="text-sm text-zinc-600">
            Ringkasan prestasi sekolah dengan tren tahunan dan distribusi utama.
          </p>
        </div>
        {showHistoryButton ? (
          <button
            type="button"
            onClick={() => router.push("/prestasi/histori")}
            className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
          >
            Histori Import Data
          </button>
        ) : null}
      </header>

      {isSchoolAdmin ? (
        <ImportTemplateActions
          title="Prestasi"
          templateUrl="/api/templates/achievement"
          uploadType="ACHIEVEMENT"
          accept=".xlsx,.xlsm"
          onSuccess={() => {
            void refetchAnalytics();
            void refetchLegacy();
            void refetchList();
          }}
        />
      ) : null}

      <Card>
        <CardContent className="space-y-3">
          <AnalyticsFilterBar
            role={role as "ADMIN" | "ADMIN_MASTER" | "SCHOOL_ADMIN" | "PUBLIC"}
            module="prestasi"
            values={{
              year: filters.year,
              jenjang: filters.jenjang,
              status: filters.status,
              schoolId: filters.schoolId,
              schoolQuery,
              jenisLomba: filters.jenisLomba,
            }}
            availableYears={yearOptions}
            onChange={handleFilterChange}
            schoolSuggestions={schoolSuggestions}
            isSchoolLoading={isSchoolLoading}
            onSchoolQueryChange={(value) => {
              setSchoolQuery(value);
              if (!value) {
                handleFilterChange({ schoolId: null });
              }
            }}
            onSchoolSelect={(school) => {
              setSchoolQuery(school.nama_sekolah);
              handleFilterChange({ schoolId: school.id });
            }}
            onSchoolClear={() => {
              setSchoolQuery("");
              handleFilterChange({ schoolId: null });
            }}
          />
        </CardContent>
      </Card>

      <section className="space-y-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Ringkasan Utama
          </p>
          <h2 className="text-lg font-semibold text-zinc-900">
            KPI strategis untuk pimpinan
          </h2>
          <p className="text-sm text-zinc-600">
            Ringkas capaian prestasi terbaru sebagai entry point dashboard.
          </p>
        </div>

        {isHydrating || isAnalyticsLoading ? (
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }).map((_, index) => (
              <div
                key={`kpi-skeleton-${index}`}
                className="h-24 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100"
              />
            ))}
          </div>
        ) : isAnalyticsError ? (
        <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span>Unable to load data right now.</span>
            <button
              type="button"
              onClick={() => refetchAnalytics()}
              className="rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600"
            >
              Coba lagi
            </button>
          </div>
        </div>
      ) : (
        <div className="space-y-6">
          {hasEmptyData ? (
            <div className="rounded-xl border border-dashed border-zinc-200 bg-white p-6 text-sm text-zinc-500">
              Belum ada data pada filter ini.
            </div>
          ) : null}

          <section className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <Card>
              <CardContent className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Total Prestasi
                </p>
                <div className="space-y-1 text-sm">
                  {achievementCategories.map((category) => (
                    <div
                      key={category}
                      className="flex items-center justify-between"
                    >
                      <span className="font-semibold text-zinc-600">
                        {category}
                      </span>
                      <span className="text-base font-semibold text-zinc-900">
                        {formatNumber(latestYearTotals[category])}
                      </span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tahun Terakhir
                </p>
                <p className="text-3xl font-semibold text-zinc-900">
                  {effectiveYear ?? "-"}
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Total Tahun {currentYearLabel}
                </p>
                <p className="text-3xl font-semibold text-zinc-900">
                  {formatNumber(analyticsSafe.kpis.currentYearTotal)}
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  YoY Growth
                </p>
                <div className="space-y-1 text-sm">
                  {achievementCategories.map((category) => (
                    <div
                      key={category}
                      className="flex items-center justify-between"
                    >
                      <span className="font-semibold text-zinc-600">
                        YoY {category}
                      </span>
                      <span
                        className={`text-base font-semibold ${categoryYoYGrowth[category].tone}`}
                      >
                        {categoryYoYGrowth[category].label}
                      </span>
                    </div>
                  ))}
                </div>
                <p className="text-xs text-zinc-500">{yoyComparisonLabel}</p>
              </CardContent>
            </Card>
          </section>

        </div>
        )}
      </section>

      <section className="space-y-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Tren Tahunan
          </p>
          <h2 className="text-lg font-semibold text-zinc-900">
            Perkembangan prestasi sekolah dari tahun ke tahun
          </h2>
          <p className="text-sm text-zinc-600">
            Gabungan tren analytics baru dan tren kategori dari data lama.
          </p>
        </div>

        <div className="space-y-6 rounded-2xl border border-zinc-200 bg-zinc-50/60 p-4 sm:p-6">
          {isTrendLoading ? (
            <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72 lg:h-80" />
          ) : isTrendError ? (
            <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <span>Tren analytics belum tersedia.</span>
                <button
                  type="button"
                  onClick={() => refetchAnalytics()}
                  className="rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600"
                >
                  Coba lagi
                </button>
              </div>
            </div>
          ) : (
            <Card>
              <CardContent className="space-y-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Tren Utama
                  </p>
                  <p className="text-sm font-semibold text-zinc-700">
                    Total prestasi per tahun
                  </p>
                </div>
                <div className="h-64 w-full sm:h-72 lg:h-80">
                  {hasTrendData ? (
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={categoryTrendSeries.rows}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="year" />
                        <YAxis domain={[0, categoryTrendSeries.maxValue]} />
                        <Tooltip />
                        <Legend />
                        {achievementCategories.map((category) => (
                          <Line
                            key={category}
                            type="monotone"
                            dataKey={category}
                            name={category}
                            stroke={achievementChartColors[category]}
                            strokeWidth={2}
                            dot={{ r: 3 }}
                            activeDot={{ r: 5 }}
                          />
                        ))}
                      </LineChart>
                    </ResponsiveContainer>
                  ) : (
                    <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                      Data belum tersedia
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </section>

      <section className="space-y-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Distribusi & Komposisi
          </p>
          <h2 className="text-lg font-semibold text-zinc-900">
            Sebaran prestasi menurut tingkat, peserta, dan sekolah
          </h2>
          <p className="text-sm text-zinc-600">
            Membaca pola komposisi prestasi dari data analytics dan data lama.
          </p>
        </div>

        <div className="space-y-6 rounded-2xl border border-zinc-200 bg-zinc-50/60 p-4 sm:p-6">
          <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-end">
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Jenis Lomba
              </label>
              <Select
                value={filters.jenisLomba ?? "ALL"}
                onValueChange={(value) => handleFilterChange({ jenisLomba: value })}
              >
                <SelectTrigger className="w-full md:w-40">
                  <SelectValue placeholder="Semua" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ALL">Semua</SelectItem>
                  {achievementCategories.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {isHydrating || isDistributionLoading ? (
            <div className="grid gap-4 lg:grid-cols-2">
              <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72" />
              {role !== "PUBLIC" ? (
                <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72" />
              ) : null}
            </div>
          ) : isDistributionError ? (
            <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <span>Distribusi analytics belum tersedia.</span>
                <button
                  type="button"
                  onClick={() => refetchDistribution()}
                  className="rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600"
                >
                  Coba lagi
                </button>
              </div>
            </div>
          ) : (
            <section className={`grid gap-6 ${distributionGridCols}`}>
              <Card>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Distribusi Tingkat Lomba
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Sebaran prestasi berdasarkan tingkat
                    </p>
                  </div>
                  <div className="h-64 w-full sm:h-72">
                    {isAllYears ? (
                      hasGroupedTingkatData ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <BarChart data={groupedTingkatData}>
                            <CartesianGrid strokeDasharray="3 3" />
                            <XAxis dataKey="label" />
                            <YAxis />
                            <Tooltip />
                            <Legend />
                            {selectedYearSeries.map((year, index) => (
                              <Bar
                                key={year}
                                dataKey={String(year)}
                                name={String(year)}
                                fill={
                                  yearSeriesColors[
                                    index % yearSeriesColors.length
                                  ]
                                }
                              />
                            ))}
                          </BarChart>
                        </ResponsiveContainer>
                      ) : (
                        <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                          Belum ada data pada filter ini
                        </div>
                      )
                    ) : hasSimpleTingkatData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart
                          data={distributionAnalyticsSafe.distribution.byTingkat ?? []}
                        >
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis dataKey="key" />
                          <YAxis />
                          <Tooltip />
                          <Bar dataKey="total" fill="#1f2937" />
                        </BarChart>
                      </ResponsiveContainer>
                    ) : (
                      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                        Belum ada data pada filter ini
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              {role !== "PUBLIC" ? (
                <Card>
                  <CardContent className="space-y-3">
                    <div>
                      <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Distribusi Jenis Peserta
                      </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Proporsi prestasi menurut jenis peserta
                    </p>
                  </div>
                  <div className="h-64 w-full sm:h-72">
                    {isAllYears ? (
                      hasGroupedJenisData ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <BarChart data={groupedJenisData}>
                            <CartesianGrid strokeDasharray="3 3" />
                            <XAxis dataKey="label" />
                            <YAxis />
                            <Tooltip />
                            <Legend />
                            {selectedYearSeries.map((year, index) => (
                              <Bar
                                key={year}
                                dataKey={String(year)}
                                name={String(year)}
                                fill={
                                  yearSeriesColors[
                                    index % yearSeriesColors.length
                                  ]
                                }
                              />
                            ))}
                          </BarChart>
                        </ResponsiveContainer>
                      ) : (
                        <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                          Belum ada data pada filter ini
                        </div>
                      )
                    ) : hasSimpleJenisData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart
                          data={distributionAnalyticsSafe.distribution.byJenisLomba ?? []}
                        >
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis dataKey="key" />
                          <YAxis />
                          <Tooltip />
                          <Bar dataKey="total" fill="#0f766e" />
                        </BarChart>
                      </ResponsiveContainer>
                    ) : (
                      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                        Belum ada data pada filter ini
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
              ) : null}

              {showDonut ? (
                <Card>
                  <CardContent className="space-y-3">
                    <div>
                      <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Komposisi Kategori
                      </p>
                      <p className="text-sm font-semibold text-zinc-700">
                        Proporsi prestasi per kategori (Siswa)
                      </p>
                    </div>
                    <div className="h-64 w-full sm:h-72">
                      {hasDonutData ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <PieChart>
                            <Pie
                              data={distributionAnalyticsSafe.categoryDonutSiswa}
                              dataKey="total"
                              nameKey="kategori"
                              innerRadius={45}
                              outerRadius={90}
                              paddingAngle={2}
                            >
                              {distributionAnalyticsSafe.categoryDonutSiswa.map(
                                (entry, index) => (
                                  <Cell
                                    key={`${entry.kategori}-${index}`}
                                    fill={
                                      donutColors[index % donutColors.length]
                                    }
                                  />
                                )
                              )}
                            </Pie>
                            <Legend />
                            <Tooltip />
                          </PieChart>
                        </ResponsiveContainer>
                      ) : (
                        <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                          Belum ada data pada filter ini
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ) : null}
            </section>
          )}

          {isDistributionLegacyLoading ? (
            <div className="grid gap-4 lg:grid-cols-2">
              <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
              <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
            </div>
          ) : isDistributionLegacyError ? (
            <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <span>Gagal memuat distribusi data lama.</span>
                <button
                  type="button"
                  onClick={() => refetchDistributionLegacy()}
                  className="rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600"
                >
                  Coba lagi
                </button>
              </div>
            </div>
          ) : (
            <section className="grid gap-6 lg:grid-cols-2">
              <PrestasiDistributionTable tableCounts={legacyDistributionCounts} />
              <PrestasiTopSchoolsChart
                topSchools={legacyTopSchools}
                barColor="#1d4ed8"
              />
            </section>
          )}

          {isAdmin ? (
            isHydrating || isAnalyticsLoading ? (
              <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
            ) : isAnalyticsError ? null : (
              <Card>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Top 10 Sekolah
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Ringkasan prestasi per sekolah
                    </p>
                  </div>
                  <div className="overflow-x-auto">
                    <table className="w-full min-w-[640px] text-sm">
                      <thead>
                        <tr className="text-left text-zinc-500">
                          <th className="py-2">Sekolah</th>
                          <th className="py-2 text-right">Total</th>
                          <th className="py-2 text-right">YoY</th>
                        </tr>
                      </thead>
                      <tbody>
                        {analyticsSafe.topSchools.length === 0 ? (
                          <tr>
                            <td
                              className="py-4 text-center text-zinc-500"
                              colSpan={3}
                            >
                              Belum ada data.
                            </td>
                          </tr>
                        ) : (
                          analyticsSafe.topSchools.map((row) => (
                            <tr key={row.school_id} className="border-t">
                              <td className="py-2 font-semibold text-zinc-900">
                                {row.school_name}
                              </td>
                              <td className="py-2 text-right">
                                {formatNumber(row.total)}
                              </td>
                              <td className="py-2 text-right">
                                {row.yoyPercent == null
                                  ? "N/A"
                                  : `${row.yoyPercent >= 0 ? "Naik" : "Turun"} ${Math.abs(row.yoyPercent).toFixed(1)}%`}
                              </td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </CardContent>
              </Card>
            )
          ) : null}
        </div>
      </section>

      <section className="space-y-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Data Operasional
          </p>
          <h2 className="text-lg font-semibold text-zinc-900">
            Detail data prestasi untuk kebutuhan operasional
          </h2>
          <p className="text-sm text-zinc-600">
            Tabel rekap dan daftar prestasi tetap berada di bagian akhir dashboard.
          </p>
        </div>

        <div className="space-y-6 rounded-2xl border border-zinc-200 bg-zinc-50/60 p-4 sm:p-6">
          {isLegacyLoading ? (
            <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
          ) : isLegacyError ? (
            <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <span>Gagal memuat rekap data lama.</span>
                <button
                  type="button"
                  onClick={() => refetchLegacy()}
                  className="rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600"
                >
                  Coba lagi
                </button>
              </div>
            </div>
          ) : (
            <PrestasiSchoolDetailTable
              rows={legacySchoolDetailRows}
              showJenjangFilter={false}
            />
          )}

          {isAdmin || isSchoolAdmin ? (
            <section className="space-y-4 rounded-2xl border border-zinc-200 bg-white p-4 sm:p-6">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  {isAdmin
                    ? "Daftar Rincian Prestasi Sekolah"
                    : "Daftar Prestasi Sekolah"}
                </p>
                <h2 className="text-lg font-semibold text-zinc-900">
                  {isAdmin
                    ? "Rincian prestasi seluruh sekolah"
                    : "Data prestasi sekolah Anda"}
                </h2>
              </div>

              <TableFilterBar>
                <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-end">
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Jenis Peserta
                    </label>
                    <Select
                      value={tableJenisPeserta}
                      onValueChange={setTableJenisPeserta}
                    >
                      <SelectTrigger className="w-full md:w-36">
                        <SelectValue placeholder="Semua" />
                      </SelectTrigger>
                      <SelectContent>
                        {jenisPesertaOptions.map((option) => (
                          <SelectItem key={option} value={option}>
                            {option}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Tingkat Lomba
                    </label>
                    <Select value={tableTingkat} onValueChange={setTableTingkat}>
                      <SelectTrigger className="w-full md:w-44">
                        <SelectValue placeholder="Semua" />
                      </SelectTrigger>
                      <SelectContent>
                        {tingkatFilterOptions.map((option) => (
                          <SelectItem key={option} value={option}>
                            {option}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Pencarian
                    </label>
                    <input
                      value={tableSearch}
                      onChange={(event) => setTableSearch(event.target.value)}
                      placeholder="Cari nama lomba / peserta"
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-60"
                    />
                  </div>
                </div>
                <div className="flex flex-col items-start gap-2 sm:flex-row sm:items-center">
                  <button
                    type="button"
                    onClick={handleExport}
                    disabled={isExporting}
                    className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {isExporting ? "Menyiapkan..." : "Download Excel"}
                  </button>
                  {exportError ? (
                    <p className="text-xs text-red-600">{exportError}</p>
                  ) : null}
                </div>
              </TableFilterBar>

              <DataTable
                columns={tableColumns}
                rows={tableRows}
                isLoading={isListLoading}
                isError={isListError}
                emptyMessage="Tidak ada data prestasi."
                tableWrapperClassName="max-h-[420px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
                tableHeaderClassName="sticky top-0 z-10 bg-white"
                page={tableMeta.page}
                pageSize={tableMeta.pageSize}
                total={tableMeta.total}
                onPageChange={setTablePage}
                renderRow={(row) => (
                  <TableRow key={row.idPrestasi}>
                    {isAdmin ? <TableCell>{row.namaSekolah}</TableCell> : null}
                    <TableCell>{row.namaLomba}</TableCell>
                    <TableCell>{row.tingkatLomba}</TableCell>
                    <TableCell>{row.kategoriPrestasi}</TableCell>
                    <TableCell>{row.peringkatPrestasi}</TableCell>
                    <TableCell>{row.namaPeserta}</TableCell>
                    <TableCell>{row.jenisLomba}</TableCell>
                    <TableCell className="text-right">{row.tahunLomba}</TableCell>
                    {isSchoolAdmin ? (
                      <TableCell className="text-center">
                        <button
                          type="button"
                          onClick={() => openEditModal(row)}
                          className="rounded-lg border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-zinc-700 hover:bg-slate-50"
                        >
                          Edit
                        </button>
                      </TableCell>
                    ) : null}
                  </TableRow>
                )}
              />
            </section>
          ) : null}
        </div>
      </section>

      {editingRow && isSchoolAdmin ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-2xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Edit Prestasi
              </h2>
              <p className="text-sm text-zinc-600">
                Perbarui detail prestasi yang dipilih.
              </p>
            </div>

            <div className="mt-4 grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama Lomba
                </label>
                <input
                  value={editForm.namaLomba}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      namaLomba: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tingkat Lomba
                </label>
                <input
                  value={editForm.tingkatLomba}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      tingkatLomba: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Kategori Prestasi
                </label>
                <input
                  value={editForm.kategoriPrestasi}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      kategoriPrestasi: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Peringkat
                </label>
                <input
                  value={editForm.peringkatPrestasi}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      peringkatPrestasi: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama Peserta
                </label>
                <input
                  value={editForm.namaPeserta}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      namaPeserta: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Jenis Lomba
                </label>
                <input
                  value={editForm.jenisLomba}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      jenisLomba: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
            </div>

            {editFormError ? (
              <p className="mt-3 text-sm text-red-600">{editFormError}</p>
            ) : null}

            <div className="mt-5 flex flex-col gap-2 sm:flex-row sm:justify-end">
              <button
                type="button"
                onClick={closeEditModal}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={() => setIsPasswordModalOpen(true)}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95"
              >
                Simpan
              </button>
            </div>
          </div>
        </div>
      ) : null}

      <ConfirmPasswordModal
        isOpen={isPasswordModalOpen}
        password={editPassword}
        onPasswordChange={(value) => setEditPassword(value)}
        onConfirm={handleEditConfirm}
        onCancel={() => setIsPasswordModalOpen(false)}
        error={editPasswordError}
        isSubmitting={isEditSaving}
      />
    </section>
  );
}

