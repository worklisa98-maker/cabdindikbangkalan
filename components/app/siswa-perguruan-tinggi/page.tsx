"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
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

import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";

import { Card, CardContent } from "@/components/ui/card";
import ImportTemplateActions from "@/components/import-template-actions";
import LegacyImportActions from "@/components/siswa/LegacyImportActions";
import { AnalyticsFilterBar } from "@/components/analytics/AnalyticsFilterBar";
import { DataTable } from "@/components/data-table";
import { TableFilterBar } from "@/components/table-filter-bar";
import { TableCell, TableRow } from "@/components/ui/table";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { TypeaheadInput } from "@/components/typeahead-input";
import { ConfirmPasswordModal } from "@/components/confirm-password-modal";

type AnalyticsKeyTotal = {
  key: string;
  total: number;
};

type AnalyticsYearPoint = {
  year: number;
  total: number;
};

type AdmissionAnalyticsResponse = {
  availableYears: number[];
  latestYear: number | null;
  kpis: {
    totalDiterima: number;
    yoyPercent: number | null;
    currentYearTotal: number;
    prevYearTotal: number;
  };
  trendByYear: AnalyticsYearPoint[];
  distribution: {
    byJalur?: AnalyticsKeyTotal[];
  };
  topSchools?: {
    school_id: number;
    school_name: string;
    total: number;
    yoyPercent: number | null;
  }[];
  trendByJalur?: AdmissionLegacyTrendPoint[];
  topPerguruanTinggi?: AdmissionLegacyTopUniversity[];
  pendingValidation?: number;
};

type AdmissionLegacyTrendPoint = {
  year: number;
  SNBP: number;
  SNBT: number;
  MANDIRI: number;
  KEDINASAN: number;
  LAINNYA: number;
};

type AdmissionLegacyTopUniversity = {
  name: string;
  total: number;
};

type LegacyImportStatus = {
  enabled: boolean;
  legacyImportMaxYear?: number | null;
};

type AdmissionListItem = {
  id: string;
  schoolId: number;
  namaSekolah: string;
  nisnSiswa: string;
  namaSiswa: string;
  namaPerguruanTinggi: string;
  jurusanPerguruanTinggi: string;
  jalurPenerimaan: string;
  tahunDiterima: number;
};

type ListMeta = {
  page: number;
  pageSize: number;
  total: number;
};

type AdmissionListResponse = {
  data: AdmissionListItem[];
  meta: ListMeta;
};

type SchoolSuggestion = {
  id: number;
  nama_sekolah: string;
  npsn?: string | null;
};

type UniversitySuggestion = {
  nama: string;
};

const formatNumber = (value: number) =>
  new Intl.NumberFormat("id-ID").format(value);

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const readNumber = (value: unknown): number | null => {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return null;
};

const normalizeTrendByJalur = (
  payload: unknown
): AdmissionLegacyTrendPoint[] => {
  if (!Array.isArray(payload)) {
    return [];
  }

  const grouped = new Map<number, AdmissionLegacyTrendPoint>();
  payload.forEach((rawPoint) => {
    if (!isRecord(rawPoint)) {
      return;
    }

    const year = readNumber(rawPoint.year);
    if (!year || year <= 0) {
      return;
    }

    const hasWideShape =
      rawPoint.SNBP !== undefined ||
      rawPoint.SNBT !== undefined ||
      rawPoint.MANDIRI !== undefined ||
      rawPoint.KEDINASAN !== undefined ||
      rawPoint.LAINNYA !== undefined;

    const existing =
      grouped.get(year) ??
      ({
        year,
        SNBP: 0,
        SNBT: 0,
        MANDIRI: 0,
        KEDINASAN: 0,
        LAINNYA: 0,
      } satisfies AdmissionLegacyTrendPoint);

    if (hasWideShape) {
      existing.SNBP = readNumber(rawPoint.SNBP) ?? 0;
      existing.SNBT = readNumber(rawPoint.SNBT) ?? 0;
      existing.MANDIRI = readNumber(rawPoint.MANDIRI) ?? 0;
      existing.KEDINASAN = readNumber(rawPoint.KEDINASAN) ?? 0;
      existing.LAINNYA = readNumber(rawPoint.LAINNYA) ?? 0;
      grouped.set(year, existing);
      return;
    }

    const keyRaw = typeof rawPoint.key === "string" ? rawPoint.key : "";
    const key = keyRaw.trim().toUpperCase();
    const total = readNumber(rawPoint.total) ?? 0;
    if (key === "SNBP") {
      existing.SNBP = total;
    } else if (key === "SNBT") {
      existing.SNBT = total;
    } else if (key === "MANDIRI" || key === "SNBM") {
      existing.MANDIRI = total;
    } else if (key === "KEDINASAN") {
      existing.KEDINASAN = total;
    } else {
      existing.LAINNYA = total;
    }
    grouped.set(year, existing);
  });

  return Array.from(grouped.values()).sort((a, b) => a.year - b.year);
};

const normalizeTopUniversities = (payload: unknown): AdmissionLegacyTopUniversity[] => {
  if (!Array.isArray(payload)) {
    return [];
  }
  return payload
    .map((rawItem) => {
      if (!isRecord(rawItem)) {
        return null;
      }
      const name =
        (typeof rawItem.name === "string" && rawItem.name.trim()) ||
        (typeof rawItem.university_name === "string" &&
          rawItem.university_name.trim()) ||
        (typeof rawItem.pt_name === "string" && rawItem.pt_name.trim()) ||
        "";
      if (!name) {
        return null;
      }
      return {
        name,
        total:
          readNumber(rawItem.total) ??
          readNumber(rawItem.TOTAL) ??
          readNumber(rawItem.count) ??
          0,
      };
    })
    .filter((item): item is AdmissionLegacyTopUniversity => Boolean(item));
};

const extractPendingValidationCount = (payload: unknown): number => {
  if (!isRecord(payload)) {
    return 0;
  }
  const readPending = (source: Record<string, unknown>) =>
    readNumber(source.pending_count) ??
    readNumber(source.pendingCount) ??
    readNumber(source.pendingValidation) ??
    readNumber(source.pending_validasi) ??
    readNumber(source.pending) ??
    0;

  const direct = readPending(payload);
  if (direct > 0 || (payload.pending === 0 && direct === 0)) {
    return direct;
  }

  if (isRecord(payload.data)) {
    const nestedDirect = readPending(payload.data);
    if (nestedDirect > 0 || (payload.data.pending === 0 && nestedDirect === 0)) {
      return nestedDirect;
    }
    if (isRecord(payload.data.validation)) {
      return (
        readNumber(payload.data.validation.pending) ??
        readNumber(payload.data.validation.pendingValidation) ??
        0
      );
    }
  }

  if (isRecord(payload.validation)) {
    return readNumber(payload.validation.pending) ?? 0;
  }
  return 0;
};

const fetchLegacyImportStatus = async (): Promise<LegacyImportStatus> => {
  const response = await apiFetch("/legacy-import/status");
  if (!response.ok) {
    throw new Error("legacy_status_failed");
  }
  const payload = (await response.json().catch(() => null)) as {
    data?: LegacyImportStatus;
  } | null;
  return payload?.data ?? { enabled: false };
};

const fetchAdmissionAnalytics = async (
  params: {
    year: string;
    jenjang: string;
    status: string;
    schoolId?: number | null;
  },
  isPublic: boolean
): Promise<AdmissionAnalyticsResponse> => {
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

  const endpoints = isPublic
    ? ["/api/public/siswa/analytics/final", "/api/public/siswa-perguruan-tinggi/analytics"]
    : ["/api/siswa/analytics/final", "/api/siswa-pt/analytics"];

  const requestInit = isPublic ? { skipAuthRedirect: true } : undefined;
  let response: Response | null = null;
  for (const endpoint of endpoints) {
    const candidateResponse = await apiFetch(
      query.toString() ? `${endpoint}?${query.toString()}` : endpoint,
      undefined,
      requestInit
    );
    if (
      (candidateResponse.status === 404 || candidateResponse.status === 405) &&
      endpoint !== endpoints[endpoints.length - 1]
    ) {
      continue;
    }
    response = candidateResponse;
    break;
  }

  if (!response) {
    throw new Error("Failed to fetch admission analytics.");
  }

  if (!response.ok) {
    throw new Error("Failed to fetch admission analytics.");
  }

  const payload = (await response.json()) as { data: Record<string, unknown> };
  const data = isRecord(payload.data) ? payload.data : {};
  return {
    availableYears: Array.isArray(data.availableYears)
      ? data.availableYears.filter((item) => typeof item === "number")
      : [],
    latestYear:
      typeof data.latestYear === "number" ? data.latestYear : null,
    kpis: isRecord(data.kpis)
      ? {
          totalDiterima: readNumber(data.kpis.totalDiterima) ?? 0,
          yoyPercent: readNumber(data.kpis.yoyPercent),
          currentYearTotal: readNumber(data.kpis.currentYearTotal) ?? 0,
          prevYearTotal: readNumber(data.kpis.prevYearTotal) ?? 0,
        }
      : {
          totalDiterima: 0,
          yoyPercent: null,
          currentYearTotal: 0,
          prevYearTotal: 0,
        },
    trendByYear: Array.isArray(data.trendByYear)
      ? data.trendByYear
          .map((item) => {
            if (!isRecord(item)) {
              return null;
            }
            const year = readNumber(item.year);
            const total = readNumber(item.total);
            if (!year || total == null) {
              return null;
            }
            return { year, total };
          })
          .filter((item): item is AnalyticsYearPoint => Boolean(item))
      : [],
    distribution: {
      byJalur: Array.isArray(data.distribution)
        ? []
        : isRecord(data.distribution) && Array.isArray(data.distribution.byJalur)
          ? data.distribution.byJalur
              .map((item) => {
                if (!isRecord(item)) {
                  return null;
                }
                const key =
                  typeof item.key === "string"
                    ? item.key
                    : typeof item.label === "string"
                      ? item.label
                      : "";
                const total = readNumber(item.total);
                if (!key || total == null) {
                  return null;
                }
                return { key, total };
              })
              .filter((item): item is AnalyticsKeyTotal => Boolean(item))
          : [],
    },
    topSchools:
      Array.isArray(data.topSchools) && data.topSchools.length > 0
        ? data.topSchools
            .map((item) => {
              if (!isRecord(item)) {
                return null;
              }
              const schoolID = readNumber(item.school_id);
              if (!schoolID) {
                return null;
              }
              const schoolName =
                typeof item.school_name === "string" ? item.school_name : "";
              return {
                school_id: schoolID,
                school_name: schoolName,
                total: readNumber(item.total) ?? 0,
                yoyPercent: readNumber(item.yoyPercent),
              };
            })
            .filter(
              (item): item is NonNullable<AdmissionAnalyticsResponse["topSchools"]>[number] =>
                Boolean(item)
            )
        : [],
    trendByJalur: normalizeTrendByJalur(data.trendByJalur),
    topPerguruanTinggi: normalizeTopUniversities(
      data.topPerguruanTinggi ?? data.topUniversities
    ),
    pendingValidation: extractPendingValidationCount(data),
  } as AdmissionAnalyticsResponse;
};

const fetchAdmissionList = async (params: {
  page: number;
  pageSize: number;
  year: string;
  pt: string;
  schoolId?: number | null;
  search: string;
}): Promise<AdmissionListResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.year && params.year !== "ALL") {
    query.set("year", params.year);
  }
  if (params.pt && params.pt !== "ALL") {
    query.set("pt", params.pt);
  }
  if (params.schoolId) {
    query.set("sekolahId", String(params.schoolId));
  }
  if (params.search) {
    query.set("q", params.search);
  }

  const response = await apiFetch(
    `/api/siswa-perguruan-tinggi/list?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch admission list.");
  }
  const payload = (await response.json()) as { data: AdmissionListResponse };
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

const fetchUniversitySuggestions = async (
  query: string
): Promise<UniversitySuggestion[]> => {
  const params = new URLSearchParams();
  params.set("q", query);
  const response = await apiFetch(`/api/universities/suggest?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch university suggestions.");
  }
  const payload = (await response.json()) as { data: UniversitySuggestion[] };
  return payload.data ?? [];
};

export default function SiswaPerguruanTinggiPage() {
  return (
    <Suspense
      fallback={<div className="p-4 text-sm text-zinc-500">Memuat...</div>}
    >
      <SiswaPerguruanTinggiPageInner />
    </Suspense>
  );
}

function SiswaPerguruanTinggiPageInner() {
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
  const showPendingValidationButton = !isHydrating && (isAdmin || isSchoolAdmin);
  const { data: legacyImportStatus } = useQuery({
    queryKey: ["legacy-import-status", status, user?.id ?? 0],
    queryFn: fetchLegacyImportStatus,
    enabled: status === "authenticated" && isSchoolAdmin,
  });

  const filters = useMemo(() => {
    const year = searchParams.get("year") ?? "ALL";
    const jenjang = searchParams.get("jenjang") ?? "ALL";
    const statusParam = searchParams.get("status") ?? "ALL";
    const schoolIdRaw = searchParams.get("school_id");
    const schoolId = schoolIdRaw ? Number(schoolIdRaw) : null;
    return {
      year,
      jenjang,
      status: statusParam,
      schoolId: Number.isFinite(schoolId) && schoolId && schoolId > 0 ? schoolId : null,
    };
  }, [searchParams]);

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
    }>) => {
      const params = new URLSearchParams(searchParams.toString());
      const merged = {
        year: filters.year,
        jenjang: filters.jenjang,
        status: filters.status,
        schoolId: filters.schoolId,
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
      "admission-analytics",
      role,
      filters.year,
      filters.jenjang,
      filters.status,
      filters.schoolId,
    ],
    queryFn: () =>
      fetchAdmissionAnalytics(
        {
          year: filters.year,
          jenjang: filters.jenjang,
          status: filters.status,
          schoolId: isAdmin ? filters.schoolId : null,
        },
        isPublic
      ),
    retry: isPublic ? 0 : 3,
    enabled: !isHydrating,
  });

  const {
    data: validationSummary,
  } = useQuery({
    queryKey: ["admission-validation-summary"],
    queryFn: async () => {
      const response = await apiFetch("/api/siswa/validation/summary");
      if (!response.ok) {
        return { pending: 0 };
      }
      const payload = await response.json().catch(() => null);
      return { pending: extractPendingValidationCount(payload) };
    },
    enabled: !isHydrating && status === "authenticated" && (isSchoolAdmin || isAdmin),
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
        totalDiterima: 0,
        yoyPercent: null,
        currentYearTotal: 0,
        prevYearTotal: 0,
      },
      trendByYear: analytics?.trendByYear ?? [],
      distribution: analytics?.distribution ?? {},
      topSchools: analytics?.topSchools ?? [],
      trendByJalur: analytics?.trendByJalur ?? [],
      topPerguruanTinggi: analytics?.topPerguruanTinggi ?? [],
      pendingValidation: analytics?.pendingValidation ?? 0,
    }),
    [analytics]
  );

  const currentYearLabel =
    filters.year !== "ALL"
      ? filters.year
      : analyticsSafe.latestYear
        ? String(analyticsSafe.latestYear)
        : "-";

  const yoyStatus =
    analyticsSafe.kpis.yoyPercent == null
      ? "N/A"
      : analyticsSafe.kpis.yoyPercent >= 0
        ? "Naik"
        : "Turun";

  const yoyValue =
    analyticsSafe.kpis.yoyPercent == null
      ? "N/A"
      : `${Math.abs(analyticsSafe.kpis.yoyPercent).toFixed(1)}%`;

  const hasEmptyData =
    !isHydrating && !isAnalyticsLoading && analyticsSafe.kpis.totalDiterima === 0;

  const legacySafe = useMemo(
    () => ({
      trenPerTahun: analyticsSafe.trendByJalur,
      topPerguruanTinggi: analyticsSafe.topPerguruanTinggi,
    }),
    [analyticsSafe.topPerguruanTinggi, analyticsSafe.trendByJalur]
  );

  const isLegacyLoading = isAnalyticsLoading;
  const isLegacyError = isAnalyticsError;
  const refetchLegacy = refetchAnalytics;

  const pendingValidationCount = Math.max(
    0,
    validationSummary?.pending ?? analyticsSafe.pendingValidation ?? 0
  );

  const availableJalurCategories = useMemo(
    () =>
      (analyticsSafe.distribution.byJalur ?? [])
        .map((item) => item.key)
        .filter((key) => Boolean(key)),
    [analyticsSafe.distribution.byJalur]
  );

  const legacyTrendByJalur = useMemo(() => {
    if (availableJalurCategories.length === 0) {
      return [];
    }
    const filteredTrend =
      filters.year && filters.year !== "ALL"
        ? legacySafe.trenPerTahun.filter(
            (point) => String(point.year) === filters.year
          )
        : legacySafe.trenPerTahun;
    return filteredTrend.map((point) => {
      const entry: Record<string, number> & { year: number } = {
        year: point.year,
      };
      availableJalurCategories.forEach((category) => {
        const value = (point as Record<string, unknown>)[category];
        entry[category] = typeof value === "number" ? value : 0;
      });
      return entry;
    });
  }, [availableJalurCategories, legacySafe.trenPerTahun, filters.year]);

  const jalurColorMap: Record<string, string> = {
    SNBP: "#0f172a",
    SNBT: "#2563eb",
    MANDIRI: "#0f766e",
    KEDINASAN: "#b45309",
    LAINNYA: "#64748b",
  };
  const jalurColorFallback = ["#0f172a", "#2563eb", "#0f766e", "#b45309", "#64748b"];

  const legacyTopUniversities = useMemo(
    () =>
      legacySafe.topPerguruanTinggi.map((item) => ({
        name: item.name,
        total: item.total,
      })),
    [legacySafe.topPerguruanTinggi]
  );
  const hasTrendByYearData = useMemo(
    () => analyticsSafe.trendByYear.some((point) => point.total > 0),
    [analyticsSafe.trendByYear]
  );
  const hasLegacyTrendData = useMemo(
    () =>
      legacyTrendByJalur.some((row) =>
        availableJalurCategories.some(
          (category) => Number(row[category] ?? 0) > 0
        )
      ),
    [availableJalurCategories, legacyTrendByJalur]
  );
  const hasDistributionByJalurData = useMemo(
    () =>
      (analyticsSafe.distribution.byJalur ?? []).some(
        (item) => item.total > 0
      ),
    [analyticsSafe.distribution.byJalur]
  );
  const hasTopUniversitiesData = useMemo(
    () => legacyTopUniversities.some((item) => item.total > 0),
    [legacyTopUniversities]
  );

  const [tablePage, setTablePage] = useState(1);
  const [tableSearch, setTableSearch] = useState("");
  const [tablePt, setTablePt] = useState("ALL");
  const [ptQuery, setPtQuery] = useState("");
  const debouncedSearch = useDebouncedValue(tableSearch, 400);
  const debouncedPtQuery = useDebouncedValue(ptQuery, 300);

  const { data: ptSuggestions = [], isLoading: isPtLoading } = useQuery({
    queryKey: ["pt-suggest", debouncedPtQuery],
    queryFn: () => fetchUniversitySuggestions(debouncedPtQuery),
    enabled:
      !isHydrating &&
      (isSchoolAdmin || isAdmin) &&
      debouncedPtQuery.trim().length >= 2,
  });

  const {
    data: listData,
    isLoading: isListLoading,
    isError: isListError,
    refetch: refetchList,
  } = useQuery({
    queryKey: [
      "admission-list",
      role,
      tablePage,
      filters.year,
      filters.schoolId,
      tablePt,
      debouncedSearch,
    ],
    queryFn: () =>
      fetchAdmissionList({
        page: tablePage,
        pageSize: 10,
        year: filters.year,
        pt: tablePt,
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
      { key: "nisnSiswa", label: "NISN" },
      { key: "namaSiswa", label: "Nama" },
      { key: "namaPerguruanTinggi", label: "Perguruan Tinggi" },
      { key: "jurusanPerguruanTinggi", label: "Jurusan" },
      { key: "jalurPenerimaan", label: "Jalur" },
      { key: "tahunDiterima", label: "Tahun", align: "right" as const }
    );
    if (isSchoolAdmin) {
      columns.push({ key: "actions", label: "Aksi", align: "center" as const });
    }
    return columns;
  }, [isAdmin, isSchoolAdmin]);

  const [editingRow, setEditingRow] = useState<AdmissionListItem | null>(null);
  const [editForm, setEditForm] = useState({
    nisnSiswa: "",
    namaSiswa: "",
    namaPerguruanTinggi: "",
    jurusanPerguruanTinggi: "",
    jalurPenerimaan: "",
    tahunDiterima: "",
  });
  const [editPassword, setEditPassword] = useState("");
  const [editFormError, setEditFormError] = useState("");
  const [editPasswordError, setEditPasswordError] = useState("");
  const [isEditSaving, setIsEditSaving] = useState(false);
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState("");

  const openEditModal = (row: AdmissionListItem) => {
    setEditingRow(row);
    setEditForm({
      nisnSiswa: row.nisnSiswa ?? "",
      namaSiswa: row.namaSiswa ?? "",
      namaPerguruanTinggi: row.namaPerguruanTinggi ?? "",
      jurusanPerguruanTinggi: row.jurusanPerguruanTinggi ?? "",
      jalurPenerimaan: row.jalurPenerimaan ?? "",
      tahunDiterima: row.tahunDiterima ? String(row.tahunDiterima) : "",
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
      const response = await apiFetch(
        `/api/siswa-perguruan-tinggi/${editingRow.id}`,
        {
          method: "PUT",
          body: JSON.stringify({
            password: editPassword,
            payload: {
              nisnSiswa: editForm.nisnSiswa,
              namaSiswa: editForm.namaSiswa,
              namaPerguruanTinggi: editForm.namaPerguruanTinggi,
              jurusanPerguruanTinggi: editForm.jurusanPerguruanTinggi,
              jalurPenerimaan: editForm.jalurPenerimaan,
              tahunDiterima: Number(editForm.tahunDiterima || 0),
            },
          }),
        }
      );
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
      if (tablePt && tablePt !== "ALL") {
        query.set("pt", tablePt);
      }
      if (filters.schoolId) {
        query.set("sekolahId", String(filters.schoolId));
      }
      if (tableSearch.trim()) {
        query.set("q", tableSearch.trim());
      }

      const response = await apiFetch(
        `/api/siswa-perguruan-tinggi/export.xlsx${query.toString() ? `?${query.toString()}` : ""}`
      );
      if (!response.ok) {
        throw new Error("export failed");
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.setAttribute("download", "siswa-perguruan-tinggi.xlsx");
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch {
      setExportError("Gagal mengunduh Excel siswa perguruan tinggi.");
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
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-3xl font-semibold">
              Dashboard Siswa Perguruan Tinggi
            </h1>
            <span className="rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700">
              Pending validasi: {pendingValidationCount}
            </span>
          </div>
          <p className="text-sm text-zinc-600">
            Ringkasan penerimaan siswa perguruan tinggi dan tren tahunan.
          </p>
        </div>
        {showHistoryButton || showPendingValidationButton ? (
          <div className="flex flex-wrap items-center gap-2">
            {showPendingValidationButton ? (
              <button
                type="button"
                onClick={() => router.push("/siswa/pending-validasi")}
                className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Status Pending Validasi
              </button>
            ) : null}
            {showHistoryButton ? (
              <button
                type="button"
                onClick={() => router.push("/siswa-perguruan-tinggi/histori")}
                className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Histori Import Data
              </button>
            ) : null}
          </div>
        ) : null}
      </header>

      {isSchoolAdmin ? (
        <>
          <ImportTemplateActions
            title="Siswa Perguruan Tinggi"
            templateUrl="/api/templates/acceptance"
            uploadType="ACCEPTANCE"
            accept=".xlsx,.xlsm"
            onSuccess={() => {
              void refetchAnalytics();
              void refetchList();
            }}
          />
          <LegacyImportActions
            title="Diterima Perguruan Tinggi"
            templateUrl="/api/legacy-import/templates/higher-ed"
            importUrl="/api/siswa/higher-ed/final/import"
            accept=".xlsx,.xlsm"
            isSchoolAdmin={isSchoolAdmin}
            enabled={legacyImportStatus?.enabled ?? false}
            maxYear={legacyImportStatus?.legacyImportMaxYear ?? null}
            onSuccess={() => {
              void refetchAnalytics();
              void refetchList();
            }}
          />
        </>
      ) : null}

      <Card>
        <CardContent className="space-y-3">
          <AnalyticsFilterBar
            role={role as "ADMIN" | "ADMIN_MASTER" | "SCHOOL_ADMIN" | "PUBLIC"}
            module="siswa-pt"
            values={{
              year: filters.year,
              jenjang: filters.jenjang,
              status: filters.status,
              schoolId: filters.schoolId,
              schoolQuery,
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
            Ringkas capaian penerimaan siswa sebagai entry point dashboard.
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
                    Total Diterima PT
                  </p>
                  <p className="text-3xl font-semibold text-zinc-900">
                    {formatNumber(analyticsSafe.kpis.totalDiterima)}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Tahun Terakhir
                  </p>
                  <p className="text-3xl font-semibold text-zinc-900">
                    {analyticsSafe.latestYear ?? "-"}
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
                  <p
                    className={`text-2xl font-semibold ${
                      analyticsSafe.kpis.yoyPercent == null
                        ? "text-zinc-500"
                        : analyticsSafe.kpis.yoyPercent >= 0
                          ? "text-emerald-600"
                          : "text-rose-600"
                    }`}
                  >
                    {yoyStatus} {yoyValue}
                  </p>
                  <p className="text-xs text-zinc-500">
                    Perbandingan {analyticsSafe.kpis.prevYearTotal} ke{" "}
                    {analyticsSafe.kpis.currentYearTotal}
                  </p>
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
            Perkembangan penerimaan siswa dari tahun ke tahun
          </h2>
          <p className="text-sm text-zinc-600">
            Tren tahunan berdasarkan analytics final.
          </p>
        </div>

        <div className="space-y-6 rounded-2xl border border-zinc-200 bg-zinc-50/60 p-4 sm:p-6">
          <div className="grid gap-6 lg:grid-cols-3">
            <div className="lg:col-span-2">
              {isHydrating || isAnalyticsLoading ? (
                <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72 lg:h-80" />
              ) : isAnalyticsError ? (
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
                        Total siswa diterima per tahun
                      </p>
                    </div>
                    <div className="h-64 w-full sm:h-72 lg:h-80">
                      {hasTrendByYearData ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <LineChart data={analyticsSafe.trendByYear}>
                            <CartesianGrid strokeDasharray="3 3" />
                            <XAxis dataKey="year" />
                            <YAxis />
                            <Tooltip />
                            <Legend />
                            <Line
                              type="monotone"
                              dataKey="total"
                              name="Total Diterima"
                              stroke="#0f172a"
                              strokeWidth={2}
                            />
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

            <div className="lg:col-span-1">
              {isLegacyLoading ? (
                <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72 lg:h-80" />
              ) : isLegacyError ? (
                  <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <span>Gagal memuat tren jalur masuk.</span>
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
                <Card>
                  <CardContent className="space-y-3">
                    <div>
                      <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Tren Jalur Masuk
                      </p>
                      <p className="text-sm font-semibold text-zinc-700">
                        Rekap penerimaan per jalur setiap tahun
                      </p>
                    </div>
                    <div className="h-64 w-full sm:h-72">
                      {hasLegacyTrendData ? (
                        <ResponsiveContainer width="100%" height="100%">
                          <LineChart data={legacyTrendByJalur}>
                            <CartesianGrid strokeDasharray="3 3" />
                            <XAxis dataKey="year" />
                            <YAxis />
                            <Tooltip />
                            <Legend />
                            {availableJalurCategories.map((category, index) => (
                              <Line
                                key={category}
                                type="monotone"
                                dataKey={category}
                                stroke={
                                  jalurColorMap[category] ??
                                  jalurColorFallback[
                                    index % jalurColorFallback.length
                                  ]
                                }
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
          </div>
        </div>
      </section>

      <section className="space-y-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Distribusi & Komposisi
          </p>
          <h2 className="text-lg font-semibold text-zinc-900">
            Sebaran penerimaan menurut jalur dan sekolah
          </h2>
          <p className="text-sm text-zinc-600">
            Membaca pola komposisi penerimaan dari analytics final.
          </p>
        </div>

        <div className="space-y-6 rounded-2xl border border-zinc-200 bg-zinc-50/60 p-4 sm:p-6">
          {isHydrating || isAnalyticsLoading ? (
            <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72" />
          ) : isAnalyticsError ? (
            <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <span>Distribusi analytics belum tersedia.</span>
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
                    Distribusi Jalur Masuk
                  </p>
                  <p className="text-sm font-semibold text-zinc-700">
                    Sebaran penerimaan per jalur
                  </p>
                </div>
                <div className="h-64 w-full sm:h-72">
                  {hasDistributionByJalurData ? (
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={analyticsSafe.distribution.byJalur ?? []}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="key" />
                        <YAxis />
                        <Tooltip />
                        <Bar dataKey="total" fill="#1f2937" />
                      </BarChart>
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

          {isLegacyLoading ? (
            <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
          ) : isLegacyError ? (
            <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <span>Gagal memuat komposisi penerimaan.</span>
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
            <Card>
              <CardContent className="space-y-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Top 10 Perguruan Tinggi
                  </p>
                  <p className="text-sm font-semibold text-zinc-700">
                    Perguruan tinggi favorit berdasarkan analytics final
                  </p>
                </div>
                <div className="h-64 w-full sm:h-72">
                  {!hasTopUniversitiesData ? (
                    <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                      Tidak ada data.
                    </div>
                  ) : (
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart
                        data={legacyTopUniversities}
                        layout="vertical"
                        margin={{ left: 16 }}
                      >
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis type="number" />
                        <YAxis
                          dataKey="name"
                          type="category"
                          width={140}
                          tick={{ fontSize: 10 }}
                        />
                        <Tooltip />
                        <Bar dataKey="total" fill="#1f2937" />
                      </BarChart>
                    </ResponsiveContainer>
                  )}
                </div>
              </CardContent>
            </Card>
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
                      Ringkasan penerimaan per sekolah
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
            Detail data penerimaan untuk kebutuhan operasional
          </h2>
          <p className="text-sm text-zinc-600">
            Tabel daftar penerimaan tetap berada di bagian akhir dashboard.
          </p>
        </div>

        <div className="rounded-2xl border border-zinc-200 bg-zinc-50/60 p-4 sm:p-6">
          {isAdmin || isSchoolAdmin ? (
            <section className="space-y-4 rounded-2xl border border-zinc-200 bg-white p-4 sm:p-6">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  {isAdmin
                    ? "Daftar Rincian Siswa Perguruan Tinggi"
                    : "Daftar Penerimaan PT"}
                </p>
                <h2 className="text-lg font-semibold text-zinc-900">
                  {isAdmin
                    ? "Rincian siswa diterima perguruan tinggi"
                    : "Data penerimaan perguruan tinggi sekolah Anda"}
                </h2>
              </div>

              <TableFilterBar>
                <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-end">
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Perguruan Tinggi
                    </label>
                    <TypeaheadInput
                      value={ptQuery}
                      onValueChange={(value) => {
                        setPtQuery(value);
                        setTablePt("ALL");
                      }}
                      items={ptSuggestions.map((item) => ({
                        key: item.nama,
                        label: item.nama,
                        value: item.nama,
                      }))}
                      onSelect={(item) => {
                        setTablePt(item.value);
                        setPtQuery(item.value);
                      }}
                      placeholder="Ketik nama PT..."
                      emptyText="PT tidak ditemukan"
                      isLoading={isPtLoading}
                      showClear={Boolean(tablePt !== "ALL" || ptQuery)}
                      onClear={() => {
                        setTablePt("ALL");
                        setPtQuery("");
                      }}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Pencarian
                    </label>
                    <input
                      value={tableSearch}
                      onChange={(event) => setTableSearch(event.target.value)}
                      placeholder="Cari NISN / nama siswa"
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
                emptyMessage="Tidak ada data penerimaan."
                tableWrapperClassName="max-h-[520px] overflow-y-auto"
                page={tableMeta.page}
                pageSize={tableMeta.pageSize}
                total={tableMeta.total}
                onPageChange={setTablePage}
                renderRow={(row) => (
                  <TableRow key={row.id}>
                    {isAdmin ? <TableCell>{row.namaSekolah}</TableCell> : null}
                    <TableCell>{row.nisnSiswa}</TableCell>
                    <TableCell>{row.namaSiswa}</TableCell>
                    <TableCell>{row.namaPerguruanTinggi}</TableCell>
                    <TableCell>{row.jurusanPerguruanTinggi}</TableCell>
                    <TableCell>{row.jalurPenerimaan}</TableCell>
                    <TableCell className="text-right">{row.tahunDiterima}</TableCell>
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
                Edit Penerimaan PT
              </h2>
              <p className="text-sm text-zinc-600">
                Perbarui data penerimaan perguruan tinggi siswa.
              </p>
            </div>

            <div className="mt-4 grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  NISN
                </label>
                <input
                  value={editForm.nisnSiswa}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      nisnSiswa: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama Siswa
                </label>
                <input
                  value={editForm.namaSiswa}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      namaSiswa: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1 md:col-span-2">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Perguruan Tinggi
                </label>
                <input
                  value={editForm.namaPerguruanTinggi}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      namaPerguruanTinggi: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Jurusan
                </label>
                <input
                  value={editForm.jurusanPerguruanTinggi}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      jurusanPerguruanTinggi: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Jalur Penerimaan
                </label>
                <input
                  value={editForm.jalurPenerimaan}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      jalurPenerimaan: event.target.value,
                    }))
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tahun Diterima
                </label>
                <input
                  type="number"
                  value={editForm.tahunDiterima}
                  onChange={(event) =>
                    setEditForm((prev) => ({
                      ...prev,
                      tahunDiterima: event.target.value,
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

