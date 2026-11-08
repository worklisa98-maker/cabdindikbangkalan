"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import SiswaModuleActions from "@/components/siswa/SiswaModuleActions";
import AdminPTEntryPanel, {
  type AdminPTEditableOffer,
} from "@/components/siswa/AdminPTEntryPanel";
import { DataTable } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { canUseScopedSchoolFilter } from "@/lib/auth/role-scope";
import { invalidatePTDerivedQueries } from "@/lib/invalidate-pt-queries";
import { jenjangOptions } from "@/lib/shared/jenjangOptions";

type ScopedSchool = {
  id: number;
  namaSekolah: string;
  npsnSekolah?: string | null;
};

type AcceptanceSummary = {
  totalPendaftarPt: number;
  totalDiumumkanDiterima: number;
  acceptanceRate: number;
  belumDiumumkan: number;
};

type AcceptanceJalurItem = {
  jalur: string;
  pendaftar: number;
  diterima: number;
  acceptanceRate: number;
};

type AcceptanceTrendItem = {
  year: number;
  pendaftar: number;
  diterima: number;
  acceptanceRate: number;
  yoyGrowthPendaftar: number | null;
  yoyGrowthDiterima: number | null;
};

type AcceptedStudentItem = {
  id: number;
  schoolId: number;
  sekolah: string;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  jalur: string;
  namaPt: string;
  jurusan: string;
  tahunDiterima: number | null;
  tanggalPengumuman: string | null;
  sourceRole: string;
  schoolConfirmedAt: string | null;
  conflictStatus: string;
  announcementStatus: string;
  canEdit: boolean;
};

type AcceptanceOfferSchoolSummaryItem = {
  schoolId: number;
  sekolah: string;
  jenjang: string;
  statusSekolah: string;
  uploadDataSiswaAkhir: number;
  pendaftaranSnbp: number;
  penerimaanSnbp: number;
  pendaftaranSnbt: number;
  penerimaanSnbt: number;
  pendaftaranMandiri: number;
  penerimaanMandiri: number;
  pendaftaranKedinasan: number;
  penerimaanKedinasan: number;
  pendaftaranLainnya: number;
  penerimaanLainnya: number;
};

type AcceptanceAnalyticsResponse = {
  availableYears: number[];
  summary: AcceptanceSummary;
  breakdownJalur: AcceptanceJalurItem[];
  trendTahunan: AcceptanceTrendItem[];
  acceptedItems: AcceptedStudentItem[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
};

type AcceptanceOfferSchoolSummaryResponse = {
  items: AcceptanceOfferSchoolSummaryItem[];
};

type SupervisorComparisonItem = {
  schoolId: number;
  sekolah: string;
  jenjang: string;
  statusSekolah: string;
  tahunLulus: number;
  eligibleSnbpCount: number;
  sipresSnbpCount: number;
  pengawasSnbpCount: number;
  selisihSnbp: number;
  sipresSnbtCount: number;
  pengawasSnbtCount: number;
  selisihSnbt: number;
  sipresMandiriCount: number;
  pengawasMandiriCount: number;
  selisihMandiri: number;
  sipresKedinasanCount: number;
  pengawasKedinasanCount: number;
  selisihKedinasan: number;
  sipresLainnyaCount: number;
  pengawasLainnyaCount: number;
  selisihLainnya: number;
  comparisonStatus: string;
  note: string;
  updatedAt: string | null;
  eligibleSnbpMissingWarning: boolean;
};

type SupervisorComparisonResponse = {
  items: SupervisorComparisonItem[];
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const readNumber = (value: unknown) => {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return 0;
};

const readString = (value: unknown) => {
  if (typeof value === "string") {
    return value.trim();
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  return "";
};

const formatNumber = (value: number) =>
  new Intl.NumberFormat("id-ID").format(value);

const formatPercent = (value: number) => `${value.toFixed(1)}%`;

const formatDate = (value?: string | null) => {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
  return parsed.toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
};

const formatJalurLabel = (jalur: string) => {
  const normalized = jalur.trim().toUpperCase();
  switch (normalized) {
    case "SNBP":
      return "SNBP";
    case "SNBT":
      return "SNBT";
    case "MANDIRI":
      return "Mandiri";
    case "KEDINASAN":
      return "Kedinasan";
    case "LAINNYA":
      return "Lainnya";
    default:
      return jalur || "-";
  }
};

const fetchScopedSchools = async (): Promise<ScopedSchool[]> => {
  const response = await apiFetch("/api/user/me/schools");
  if (!response.ok) {
    throw new Error("Failed to fetch scoped schools.");
  }
  const payload = (await response.json()) as {
    data?: Array<{
      id: number;
      namaSekolah: string;
      npsnSekolah?: string | null;
    }>;
  };
  return (payload.data ?? []).map((item) => ({
    id: item.id,
    namaSekolah: item.namaSekolah,
    npsnSekolah: item.npsnSekolah ?? null,
  }));
};

const emptyAnalytics = (
  page: number,
  pageSize: number
): AcceptanceAnalyticsResponse => ({
  availableYears: [],
  summary: {
    totalPendaftarPt: 0,
    totalDiumumkanDiterima: 0,
    acceptanceRate: 0,
    belumDiumumkan: 0,
  },
  breakdownJalur: [],
  trendTahunan: [],
  acceptedItems: [],
  pagination: {
    page,
    pageSize,
    total: 0,
  },
});

const emptySchoolSummary = (): AcceptanceOfferSchoolSummaryResponse => ({
  items: [],
});

const emptySupervisorComparison = (): SupervisorComparisonResponse => ({
  items: [],
});

const fetchAcceptanceAnalytics = async (params: {
  page: number;
  pageSize: number;
  search: string;
  year: string;
  jenjang: string;
  statusSekolah: string;
  path: string;
  validationStatus: string;
  schoolId?: number | null;
}): Promise<AcceptanceAnalyticsResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.search.trim()) {
    query.set("q", params.search.trim());
  }
  if (params.year !== "ALL") {
    query.set("year", params.year);
  }
  if (params.jenjang !== "ALL") {
    query.set("jenjang", params.jenjang);
  }
  if (params.statusSekolah !== "ALL") {
    query.set("status_sekolah", params.statusSekolah);
  }
  if (params.path !== "ALL") query.set("jalur", params.path);
  if (params.search.trim()) query.set("q", params.search.trim());
  if (params.validationStatus !== "ALL")
    query.set("validation_status", params.validationStatus);
  if (params.schoolId && params.schoolId > 0) {
    query.set("school_id", String(params.schoolId));
  }

  const response = await apiFetch(
    `/api/siswa/acceptance-offers/analytics?${query.toString()}`,
    { cache: "no-store" }
  );
  if (!response.ok) {
    throw new Error("Failed to fetch acceptance analytics.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const summary = isRecord(data?.summary) ? data.summary : {};
  const pagination = isRecord(data?.pagination) ? data.pagination : {};

  return {
    availableYears: Array.isArray(data?.available_years ?? data?.availableYears)
      ? (data?.available_years ?? data?.availableYears)
          .map((item: unknown) => readNumber(item))
          .filter((item: number) => item > 0)
      : [],
    summary: {
      totalPendaftarPt: readNumber(
        summary.total_pendaftar_pt ?? summary.totalPendaftarPt
      ),
      totalDiumumkanDiterima: readNumber(
        summary.total_diumumkan_diterima ?? summary.totalDiumumkanDiterima
      ),
      acceptanceRate: readNumber(
        summary.acceptance_rate ?? summary.acceptanceRate
      ),
      belumDiumumkan: readNumber(
        summary.belum_diumumkan ?? summary.belumDiumumkan
      ),
    },
    breakdownJalur: Array.isArray(data?.breakdown_jalur ?? data?.breakdownJalur)
      ? (data?.breakdown_jalur ?? data?.breakdownJalur).map((item: unknown) => {
          const row = isRecord(item) ? item : {};
          return {
            jalur: readString(row.jalur),
            pendaftar: readNumber(row.pendaftar),
            diterima: readNumber(row.diterima),
            acceptanceRate: readNumber(
              row.acceptance_rate ?? row.acceptanceRate
            ),
          };
        })
      : [],
    trendTahunan: Array.isArray(data?.trend_tahunan ?? data?.trendTahunan)
      ? (data?.trend_tahunan ?? data?.trendTahunan).map((item: unknown) => {
          const row = isRecord(item) ? item : {};
          const parsedYear = readNumber(row.year);
          const rawYoYPendaftar =
            row.yoy_growth_pendaftar ?? row.yoyGrowthPendaftar;
          const rawYoYDiterima =
            row.yoy_growth_diterima ?? row.yoyGrowthDiterima;
          return {
            year: parsedYear,
            pendaftar: readNumber(row.pendaftar),
            diterima: readNumber(row.diterima),
            acceptanceRate: readNumber(
              row.acceptance_rate ?? row.acceptanceRate
            ),
            yoyGrowthPendaftar:
              rawYoYPendaftar == null ? null : readNumber(rawYoYPendaftar),
            yoyGrowthDiterima:
              rawYoYDiterima == null ? null : readNumber(rawYoYDiterima),
          };
        })
      : [],
    acceptedItems: Array.isArray(data?.accepted_items ?? data?.acceptedItems)
      ? (data?.accepted_items ?? data?.acceptedItems).map((item: unknown) => {
          const row = isRecord(item) ? item : {};
          const parsedTahunDiterima = readNumber(
            row.tahun_diterima ?? row.tahunDiterima
          );
          return {
            id: readNumber(row.id),
            schoolId: readNumber(row.school_id ?? row.schoolId),
            sekolah: readString(row.sekolah),
            nisn: readString(row.nisn),
            namaSiswa: readString(row.nama_siswa ?? row.namaSiswa),
            rombel: readString(row.rombel),
            tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
            jalur: readString(row.jalur),
            namaPt: readString(row.nama_pt ?? row.namaPt),
            jurusan: readString(row.jurusan),
            tahunDiterima: parsedTahunDiterima > 0 ? parsedTahunDiterima : null,
            tanggalPengumuman:
              readString(row.tanggal_pengumuman ?? row.tanggalPengumuman) ||
              null,
            sourceRole: readString(row.source_role ?? row.sourceRole),
            schoolConfirmedAt:
              readString(row.school_confirmed_at ?? row.schoolConfirmedAt) ||
              null,
            conflictStatus:
              readString(row.conflict_status ?? row.conflictStatus) || "NONE",
            announcementStatus:
              readString(row.announcement_status ?? row.announcementStatus) ||
              "-",
            canEdit: Boolean(row.can_edit ?? row.canEdit),
          };
        })
      : [],
    pagination: {
      page: readNumber(pagination.page) || params.page,
      pageSize:
        readNumber(pagination.page_size ?? pagination.pageSize) ||
        params.pageSize,
      total: readNumber(pagination.total),
    },
  };
};

const fetchAcceptanceOfferSchoolSummary = async (params: {
  year: string;
  jenjang: string;
  statusSekolah: string;
  path: string;
  search: string;
  schoolId?: number | null;
}): Promise<AcceptanceOfferSchoolSummaryResponse> => {
  const query = new URLSearchParams();
  if (params.year !== "ALL") {
    query.set("year", params.year);
  }
  if (params.jenjang !== "ALL") {
    query.set("jenjang", params.jenjang);
  }
  if (params.statusSekolah !== "ALL") {
    query.set("status_sekolah", params.statusSekolah);
  }
  if (params.path !== "ALL") query.set("jalur", params.path);
  if (params.search.trim()) query.set("q", params.search.trim());
  if (params.schoolId && params.schoolId > 0) {
    query.set("school_id", String(params.schoolId));
  }

  const response = await apiFetch(
    `/api/siswa/acceptance-offers/school-summary${
      query.toString() ? `?${query.toString()}` : ""
    }`,
    { cache: "no-store" }
  );
  if (!response.ok) {
    throw new Error("Failed to fetch acceptance offer school summary.");
  }

  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const items = Array.isArray(data?.items) ? data.items : [];

  return {
    items: items.map((item: unknown) => {
      const row = isRecord(item) ? item : {};
      return {
        schoolId: readNumber(row.school_id ?? row.schoolId),
        sekolah: readString(row.sekolah),
        jenjang: readString(row.jenjang),
        statusSekolah: readString(row.status_sekolah ?? row.statusSekolah),
        uploadDataSiswaAkhir: readNumber(
          row.upload_data_siswa_akhir ?? row.uploadDataSiswaAkhir
        ),
        pendaftaranSnbp: readNumber(
          row.pendaftaran_snbp ?? row.pendaftaranSnbp
        ),
        penerimaanSnbp: readNumber(row.penerimaan_snbp ?? row.penerimaanSnbp),
        pendaftaranSnbt: readNumber(
          row.pendaftaran_snbt ?? row.pendaftaranSnbt
        ),
        penerimaanSnbt: readNumber(row.penerimaan_snbt ?? row.penerimaanSnbt),
        pendaftaranMandiri: readNumber(
          row.pendaftaran_mandiri ?? row.pendaftaranMandiri
        ),
        penerimaanMandiri: readNumber(
          row.penerimaan_mandiri ?? row.penerimaanMandiri
        ),
        pendaftaranKedinasan: readNumber(
          row.pendaftaran_kedinasan ?? row.pendaftaranKedinasan
        ),
        penerimaanKedinasan: readNumber(
          row.penerimaan_kedinasan ?? row.penerimaanKedinasan
        ),
        pendaftaranLainnya: readNumber(
          row.pendaftaran_lainnya ?? row.pendaftaranLainnya
        ),
        penerimaanLainnya: readNumber(
          row.penerimaan_lainnya ?? row.penerimaanLainnya
        ),
      };
    }),
  };
};

const fetchSupervisorComparison = async (params: {
  year: string;
  jenjang: string;
  statusSekolah: string;
  path: string;
  search: string;
  schoolId?: number | null;
}): Promise<SupervisorComparisonResponse> => {
  const query = new URLSearchParams();
  if (params.year !== "ALL") query.set("year", params.year);
  if (params.jenjang !== "ALL") query.set("jenjang", params.jenjang);
  if (params.statusSekolah !== "ALL")
    query.set("status_sekolah", params.statusSekolah);
  if (params.path !== "ALL") query.set("jalur", params.path);
  if (params.search.trim()) query.set("q", params.search.trim());
  if (params.schoolId && params.schoolId > 0)
    query.set("school_id", String(params.schoolId));
  const response = await apiFetch(
    `/api/siswa/acceptance-offers/supervisor-comparison${query.toString() ? `?${query.toString()}` : ""}`,
    { cache: "no-store" }
  );
  if (!response.ok) throw new Error("Failed to fetch supervisor comparison.");
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const items = Array.isArray(data?.items) ? data.items : [];
  return {
    items: items.map((item: unknown) => {
      const row = isRecord(item) ? item : {};
      return {
        schoolId: readNumber(row.school_id ?? row.schoolId),
        sekolah: readString(row.sekolah),
        jenjang: readString(row.jenjang),
        statusSekolah: readString(row.status_sekolah ?? row.statusSekolah),
        tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
        eligibleSnbpCount: readNumber(
          row.eligible_snbp_count ?? row.eligibleSnbpCount
        ),
        sipresSnbpCount: readNumber(
          row.sipres_snbp_count ?? row.sipresSnbpCount
        ),
        pengawasSnbpCount: readNumber(
          row.pengawas_snbp_count ?? row.pengawasSnbpCount
        ),
        selisihSnbp: readNumber(row.selisih_snbp ?? row.selisihSnbp),
        sipresSnbtCount: readNumber(
          row.sipres_snbt_count ?? row.sipresSnbtCount
        ),
        pengawasSnbtCount: readNumber(
          row.pengawas_snbt_count ?? row.pengawasSnbtCount
        ),
        selisihSnbt: readNumber(row.selisih_snbt ?? row.selisihSnbt),
        sipresMandiriCount: readNumber(
          row.sipres_mandiri_count ?? row.sipresMandiriCount
        ),
        pengawasMandiriCount: readNumber(
          row.pengawas_mandiri_count ?? row.pengawasMandiriCount
        ),
        selisihMandiri: readNumber(row.selisih_mandiri ?? row.selisihMandiri),
        sipresKedinasanCount: readNumber(
          row.sipres_kedinasan_count ?? row.sipresKedinasanCount
        ),
        pengawasKedinasanCount: readNumber(
          row.pengawas_kedinasan_count ?? row.pengawasKedinasanCount
        ),
        selisihKedinasan: readNumber(
          row.selisih_kedinasan ?? row.selisihKedinasan
        ),
        sipresLainnyaCount: readNumber(
          row.sipres_lainnya_count ?? row.sipresLainnyaCount
        ),
        pengawasLainnyaCount: readNumber(
          row.pengawas_lainnya_count ?? row.pengawasLainnyaCount
        ),
        selisihLainnya: readNumber(row.selisih_lainnya ?? row.selisihLainnya),
        comparisonStatus: readString(
          row.comparison_status ?? row.comparisonStatus
        ),
        note: readString(row.note),
        updatedAt: readString(row.updated_at ?? row.updatedAt) || null,
        eligibleSnbpMissingWarning: Boolean(
          row.eligible_snbp_missing_warning ?? row.eligibleSnbpMissingWarning
        ),
      };
    }),
  };
};

export default function SiswaPengumumanPTPage() {
  const queryClient = useQueryClient();
  const { user, status } = useAuth();
  const normalizedRole = (user?.role ?? "").trim().toUpperCase();
  const isSchoolAdmin = normalizedRole === "SCHOOL_ADMIN";
  const canViewSchoolSummary =
    normalizedRole === "ADMIN" ||
    normalizedRole === "ADMIN_CABDIN" ||
    normalizedRole === "ADMIN_MASTER" ||
    normalizedRole === "PENGAWAS";
  const canManageAdminPT =
    normalizedRole === "ADMIN" ||
    normalizedRole === "ADMIN_CABDIN" ||
    normalizedRole === "ADMIN_MASTER";
  const showAdvancedFilters =
    !isSchoolAdmin && canUseScopedSchoolFilter(user?.role);
  const [nisn, setNisn] = useState("");
  const [tahunLulus, setTahunLulus] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isExportingSchoolSummary, setIsExportingSchoolSummary] =
    useState(false);
  const [isExportingAccepted, setIsExportingAccepted] = useState(false);
  const [schoolSummaryExportError, setSchoolSummaryExportError] = useState("");
  const [acceptedExportError, setAcceptedExportError] = useState("");
  const [editingAdminOffer, setEditingAdminOffer] =
    useState<AdminPTEditableOffer | null>(null);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const pageSize = 10;
  const [selectedSchoolId, setSelectedSchoolId] = useState<number | null>(null);
  const [selectedYear, setSelectedYear] = useState("ALL");
  const [selectedPath, setSelectedPath] = useState("ALL");
  const [tableValidationStatus, setTableValidationStatus] = useState("ALL");
  const [selectedJenjang, setSelectedJenjang] = useState("ALL");
  const [selectedStatusSekolah, setSelectedStatusSekolah] = useState("ALL");
  const [editingSnapshot, setEditingSnapshot] =
    useState<SupervisorComparisonItem | null>(null);
  const [snapshotForm, setSnapshotForm] = useState({
    snbp: "",
    snbt: "",
    mandiri: "",
    kedinasan: "",
    lainnya: "",
    note: "",
  });
  const [isSavingSnapshot, setIsSavingSnapshot] = useState(false);

  const { data: scopedSchools = [] } = useQuery({
    queryKey: ["acceptance-analytics-schools", user?.id ?? "anon"],
    queryFn: fetchScopedSchools,
    enabled: status === "authenticated" && showAdvancedFilters,
  });

  const {
    data: analytics = emptyAnalytics(page, pageSize),
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: [
      "acceptance-analytics",
      page,
      pageSize,
      search,
      selectedYear,
      selectedJenjang,
      selectedStatusSekolah,
      selectedPath,
      search,
      tableValidationStatus,
      showAdvancedFilters ? (selectedSchoolId ?? "ALL") : "SCHOOL_ADMIN",
    ],
    queryFn: () =>
      fetchAcceptanceAnalytics({
        page,
        pageSize,
        search,
        year: selectedYear,
        jenjang: showAdvancedFilters ? selectedJenjang : "ALL",
        statusSekolah: showAdvancedFilters ? selectedStatusSekolah : "ALL",
        path: selectedPath,
        validationStatus: tableValidationStatus,
        schoolId: showAdvancedFilters ? selectedSchoolId : null,
      }),
    enabled: status === "authenticated",
  });

  const {
    data: schoolSummary = emptySchoolSummary(),
    isLoading: isSchoolSummaryLoading,
    isError: isSchoolSummaryError,
    refetch: refetchSchoolSummary,
  } = useQuery({
    queryKey: [
      "acceptance-offer-school-summary",
      selectedYear,
      showAdvancedFilters ? selectedJenjang : "ALL",
      showAdvancedFilters ? selectedStatusSekolah : "ALL",
      selectedPath,
      search,
      showAdvancedFilters ? (selectedSchoolId ?? "ALL") : "ALL",
    ],
    queryFn: () =>
      fetchAcceptanceOfferSchoolSummary({
        year: selectedYear,
        jenjang: showAdvancedFilters ? selectedJenjang : "ALL",
        statusSekolah: showAdvancedFilters ? selectedStatusSekolah : "ALL",
        path: selectedPath,
        search,
        schoolId: showAdvancedFilters ? selectedSchoolId : null,
      }),
    enabled: status === "authenticated" && canViewSchoolSummary,
  });

  const {
    data: supervisorComparison = emptySupervisorComparison(),
    isLoading: isSupervisorComparisonLoading,
    isError: isSupervisorComparisonError,
    refetch: refetchSupervisorComparison,
  } = useQuery({
    queryKey: [
      "acceptance-offer-supervisor-comparison",
      selectedYear,
      showAdvancedFilters ? selectedJenjang : "ALL",
      showAdvancedFilters ? selectedStatusSekolah : "ALL",
      selectedPath,
      search,
      showAdvancedFilters ? (selectedSchoolId ?? "ALL") : "ALL",
    ],
    queryFn: () =>
      fetchSupervisorComparison({
        year: selectedYear,
        jenjang: showAdvancedFilters ? selectedJenjang : "ALL",
        statusSekolah: showAdvancedFilters ? selectedStatusSekolah : "ALL",
        path: selectedPath,
        search,
        schoolId: showAdvancedFilters ? selectedSchoolId : null,
      }),
    enabled:
      status === "authenticated" &&
      canViewSchoolSummary &&
      !isSchoolAdmin &&
      selectedYear !== "ALL",
  });

  const yearOptions = useMemo(() => {
    if (analytics.availableYears.length > 0) {
      return analytics.availableYears;
    }
    const fallbackYear = selectedYear !== "ALL" ? Number(selectedYear) : NaN;
    return Number.isFinite(fallbackYear) && fallbackYear > 0
      ? [fallbackYear]
      : [];
  }, [analytics.availableYears, selectedYear]);

  const templateUrl = useMemo(() => {
    if (selectedYear === "ALL") {
      return "/api/siswa/acceptance-offers/template.xlsx";
    }
    return `/api/siswa/acceptance-offers/template.xlsx?year=${encodeURIComponent(
      selectedYear
    )}`;
  }, [selectedYear]);

  const handleDownloadSchoolSummary = async () => {
    setIsExportingSchoolSummary(true);
    setSchoolSummaryExportError("");
    try {
      const query = new URLSearchParams();
      if (selectedYear !== "ALL") {
        query.set("year", selectedYear);
      }
      if (showAdvancedFilters && selectedJenjang !== "ALL") {
        query.set("jenjang", selectedJenjang);
      }
      if (showAdvancedFilters && selectedStatusSekolah !== "ALL") {
        query.set("status_sekolah", selectedStatusSekolah);
      }
      if (selectedPath !== "ALL") query.set("jalur", selectedPath);
      if (search.trim()) query.set("q", search.trim());
      if (showAdvancedFilters && selectedSchoolId && selectedSchoolId > 0) {
        query.set("school_id", String(selectedSchoolId));
      }

      const response = await apiFetch(
        `/api/siswa/acceptance-offers/school-summary.xlsx${
          query.toString() ? `?${query.toString()}` : ""
        }`
      );
      if (!response.ok) {
        throw new Error("school summary export failed");
      }

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.setAttribute("download", "rekap-pengumuman-pt-per-sekolah.xlsx");
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch {
      setSchoolSummaryExportError("Gagal mengunduh Excel rekap sekolah.");
    } finally {
      setIsExportingSchoolSummary(false);
    }
  };

  const handleDownloadAccepted = async () => {
    setIsExportingAccepted(true);
    setAcceptedExportError("");
    try {
      const query = new URLSearchParams();
      if (selectedYear !== "ALL") query.set("year", selectedYear);
      if (showAdvancedFilters && selectedJenjang !== "ALL")
        query.set("jenjang", selectedJenjang);
      if (showAdvancedFilters && selectedStatusSekolah !== "ALL")
        query.set("status_sekolah", selectedStatusSekolah);
      if (selectedPath !== "ALL") query.set("jalur", selectedPath);
      if (showAdvancedFilters && selectedSchoolId && selectedSchoolId > 0)
        query.set("school_id", String(selectedSchoolId));
      if (search.trim()) query.set("q", search.trim());
      const response = await apiFetch(
        `/api/siswa/acceptance-offers/accepted.xlsx${query.toString() ? `?${query.toString()}` : ""}`
      );
      if (!response.ok) throw new Error("accepted export failed");
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.setAttribute("download", "detail-siswa-diumumkan-pt.xlsx");
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch {
      setAcceptedExportError("Gagal mengunduh detail siswa diumumkan.");
    } finally {
      setIsExportingAccepted(false);
    }
  };

  const openSnapshotModal = (row: SupervisorComparisonItem) => {
    setEditingSnapshot(row);
    setSnapshotForm({
      snbp: String(row.pengawasSnbpCount),
      snbt: String(row.pengawasSnbtCount),
      mandiri: String(row.pengawasMandiriCount),
      kedinasan: String(row.pengawasKedinasanCount),
      lainnya: String(row.pengawasLainnyaCount),
      note: row.note,
    });
  };

  const handleSaveSnapshot = async () => {
    if (!editingSnapshot) return;
    const year = Number(selectedYear) || editingSnapshot.tahunLulus;
    const values = {
      snbp_count: Number(snapshotForm.snbp),
      snbt_count: Number(snapshotForm.snbt),
      mandiri_count: Number(snapshotForm.mandiri),
      kedinasan_count: Number(snapshotForm.kedinasan),
      lainnya_count: Number(snapshotForm.lainnya),
    };
    if (
      !Number.isFinite(year) ||
      year <= 0 ||
      Object.values(values).some(
        (value) => !Number.isFinite(value) || value < 0
      )
    ) {
      alert("Tahun dan semua jumlah harus valid.");
      return;
    }
    setIsSavingSnapshot(true);
    try {
      const response = await apiFetch(
        "/api/siswa/acceptance-offers/supervisor-snapshot",
        {
          method: "PUT",
          body: JSON.stringify({
            school_id: editingSnapshot.schoolId,
            tahun_lulus: year,
            ...values,
            note: snapshotForm.note,
          }),
        }
      );
      const payload = await response.json().catch(() => null);
      if (!response.ok)
        throw new Error(payload?.message ?? "Gagal menyimpan rekap pengawas.");
      setEditingSnapshot(null);
      void refetchSupervisorComparison();
      void refetchSchoolSummary();
      await invalidatePTDerivedQueries(queryClient);
    } catch (error) {
      alert(
        error instanceof Error
          ? error.message
          : "Gagal menyimpan rekap pengawas."
      );
    } finally {
      setIsSavingSnapshot(false);
    }
  };

  const handleCancelAnnouncement = async () => {
    const trimmedNISN = nisn.trim();
    const parsedYear = Number(tahunLulus);
    if (!trimmedNISN || !Number.isFinite(parsedYear) || parsedYear <= 0) {
      alert("NISN dan tahun lulus wajib diisi.");
      return;
    }
    if (!window.confirm("Batalkan pengumuman PT untuk siswa ini?")) {
      return;
    }

    setIsSubmitting(true);
    try {
      const response = await apiFetch("/api/siswa/acceptance-offers/cancel", {
        method: "POST",
        body: JSON.stringify({
          nisn: trimmedNISN,
          tahun_lulus: parsedYear,
        }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error(
          payload?.message ?? payload?.error ?? "Gagal membatalkan pengumuman."
        );
      }
      alert("Pengumuman PT berhasil dibatalkan.");
      setNisn("");
      setTahunLulus("");
      void refetch();
      if (canViewSchoolSummary) {
        void refetchSchoolSummary();
        void refetchSupervisorComparison();
      }
      await invalidatePTDerivedQueries(queryClient);
    } catch (error) {
      alert(
        error instanceof Error ? error.message : "Gagal membatalkan pengumuman."
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const summaryCards = useMemo(
    () => [
      {
        label: "Total Pendaftar PT",
        value: formatNumber(analytics.summary.totalPendaftarPt),
      },
      {
        label: "Diumumkan Diterima",
        value: formatNumber(analytics.summary.totalDiumumkanDiterima),
      },
      {
        label: "Persentase Pengumuman",
        value: formatPercent(analytics.summary.acceptanceRate),
      },
      {
        label: "Belum Diumumkan",
        value: formatNumber(analytics.summary.belumDiumumkan),
      },
    ],
    [analytics.summary]
  );

  return (
    <AdminRouteGuard>
      <section className="flex flex-col gap-6">
        <header className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Siswa
          </p>
          <h1 className="text-3xl font-semibold text-zinc-900">
            Pengumuman Diterima PT
          </h1>
          <p className="text-sm text-zinc-600">
            Dashboard analytics pengumuman PT untuk analisis pimpinan dan
            pemantauan daftar siswa yang sudah diumumkan diterima.
          </p>
        </header>

        <SiswaModuleActions
          title="Pengumuman Diterima PT"
          templateUrl={templateUrl}
          importUrl="/api/siswa/acceptance-offers/import"
          accept=".xlsx,.xlsm"
          isSchoolAdmin={isSchoolAdmin}
          onSuccess={() => {
            void refetch();
            if (canViewSchoolSummary) {
              void refetchSchoolSummary();
              void refetchSupervisorComparison();
            }
          }}
        />

        {canManageAdminPT ? (
          <AdminPTEntryPanel
            schools={scopedSchools}
            editingOffer={editingAdminOffer}
            onEditingChange={setEditingAdminOffer}
            onSuccess={() => {
              void refetch();
              if (canViewSchoolSummary) {
                void refetchSchoolSummary();
                void refetchSupervisorComparison();
              }
            }}
          />
        ) : null}

        <Card>
          <CardContent className="space-y-3 p-4">
            <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Filter Global
            </p>
            <div className="grid gap-3 md:grid-cols-3 lg:grid-cols-6">
              {showAdvancedFilters ? (
                <label className="space-y-1 text-xs font-semibold text-zinc-500 uppercase">
                  Sekolah
                  <select
                    value={selectedSchoolId ?? ""}
                    onChange={(event) => {
                      const value = Number(event.target.value);
                      setSelectedSchoolId(
                        Number.isFinite(value) && value > 0 ? value : null
                      );
                      setPage(1);
                    }}
                    className="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm font-normal text-zinc-900 normal-case"
                  >
                    <option value="">Semua sekolah dalam scope</option>
                    {scopedSchools.map((school) => (
                      <option key={school.id} value={school.id}>
                        {school.namaSekolah}
                      </option>
                    ))}
                  </select>
                </label>
              ) : null}
              {showAdvancedFilters ? (
                <label className="space-y-1 text-xs font-semibold text-zinc-500 uppercase">
                  Status sekolah
                  <select
                    value={selectedStatusSekolah}
                    onChange={(event) => {
                      setSelectedStatusSekolah(event.target.value);
                      setPage(1);
                    }}
                    className="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm font-normal text-zinc-900 normal-case"
                  >
                    <option value="ALL">Semua status</option>
                    <option value="NEGERI">Negeri</option>
                    <option value="SWASTA">Swasta</option>
                  </select>
                </label>
              ) : null}
              {showAdvancedFilters ? (
                <label className="space-y-1 text-xs font-semibold text-zinc-500 uppercase">
                  Jenjang
                  <select
                    value={selectedJenjang}
                    onChange={(event) => {
                      setSelectedJenjang(event.target.value);
                      setPage(1);
                    }}
                    className="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm font-normal text-zinc-900 normal-case"
                  >
                    <option value="ALL">Semua jenjang</option>
                    {jenjangOptions.map((jenjang) => (
                      <option key={jenjang} value={jenjang}>
                        {jenjang}
                      </option>
                    ))}
                  </select>
                </label>
              ) : null}
              <label className="space-y-1 text-xs font-semibold text-zinc-500 uppercase">
                Tahun lulus
                <select
                  value={selectedYear}
                  onChange={(event) => {
                    setSelectedYear(event.target.value);
                    setPage(1);
                  }}
                  className="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm font-normal text-zinc-900 normal-case"
                >
                  <option value="ALL">Semua tahun lulus</option>
                  {yearOptions.map((year) => (
                    <option key={year} value={String(year)}>
                      {year}
                    </option>
                  ))}
                </select>
              </label>
              <label className="space-y-1 text-xs font-semibold text-zinc-500 uppercase">
                Jalur
                <select
                  value={selectedPath}
                  onChange={(event) => {
                    setSelectedPath(event.target.value);
                    setPage(1);
                  }}
                  className="mt-1 h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm font-normal text-zinc-900 normal-case"
                >
                  <option value="ALL">Semua jalur</option>
                  {["SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"].map(
                    (path) => (
                      <option key={path} value={path}>
                        {path}
                      </option>
                    )
                  )}
                </select>
              </label>
              <label className="space-y-1 text-xs font-semibold text-zinc-500 uppercase">
                Pencarian
                <input
                  value={search}
                  onChange={(event) => {
                    setSearch(event.target.value);
                    setPage(1);
                  }}
                  placeholder="Siswa, sekolah, PT"
                  className="mt-1 h-10 w-full rounded-md border border-zinc-200 px-3 text-sm font-normal text-zinc-900 normal-case"
                />
              </label>
            </div>
            <button
              type="button"
              onClick={() => {
                setSelectedSchoolId(null);
                setSelectedYear("ALL");
                setSelectedJenjang("ALL");
                setSelectedStatusSekolah("ALL");
                setSelectedPath("ALL");
                setSearch("");
                setPage(1);
              }}
              className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-semibold"
            >
              Reset Filter
            </button>
          </CardContent>
        </Card>

        <section className="grid gap-4 md:grid-cols-4">
          {summaryCards.map((item) => (
            <Card key={item.label}>
              <CardContent className="space-y-2 p-4">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  {item.label}
                </p>
                <p className="text-3xl font-semibold text-zinc-900">
                  {item.value}
                </p>
              </CardContent>
            </Card>
          ))}
        </section>

        <Card>
          <CardContent className="space-y-4 p-4">
            {false ? (
              <>
                <div className="grid gap-3 md:grid-cols-3 lg:grid-cols-6">
                  {showAdvancedFilters ? (
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Sekolah
                      </label>
                      <select
                        value={selectedSchoolId ?? ""}
                        onChange={(event) => {
                          const value = Number(event.target.value);
                          setSelectedSchoolId(
                            Number.isFinite(value) && value > 0 ? value : null
                          );
                          setPage(1);
                        }}
                        className="h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm"
                      >
                        <option value="">Semua sekolah dalam scope</option>
                        {scopedSchools.map((school) => (
                          <option key={school.id} value={school.id}>
                            {school.namaSekolah}
                          </option>
                        ))}
                      </select>
                    </div>
                  ) : null}
                  {showAdvancedFilters ? (
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Status Sekolah
                      </label>
                      <select
                        value={selectedStatusSekolah}
                        onChange={(event) => {
                          setSelectedStatusSekolah(event.target.value);
                          setPage(1);
                        }}
                        className="h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm"
                      >
                        <option value="ALL">Semua status</option>
                        <option value="NEGERI">Negeri</option>
                        <option value="SWASTA">Swasta</option>
                      </select>
                    </div>
                  ) : null}
                  {showAdvancedFilters ? (
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Jenjang
                      </label>
                      <select
                        value={selectedJenjang}
                        onChange={(event) => {
                          setSelectedJenjang(event.target.value);
                          setPage(1);
                        }}
                        className="h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm"
                      >
                        <option value="ALL">Semua jenjang</option>
                        {jenjangOptions.map((jenjang) => (
                          <option key={jenjang} value={jenjang}>
                            {jenjang}
                          </option>
                        ))}
                      </select>
                    </div>
                  ) : null}
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Tahun Lulus
                    </label>
                    <select
                      value={selectedYear}
                      onChange={(event) => {
                        setSelectedYear(event.target.value);
                        setPage(1);
                      }}
                      className="h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm"
                    >
                      <option value="ALL">Semua tahun lulus</option>
                      {yearOptions.map((year) => (
                        <option key={year} value={String(year)}>
                          {year}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Jalur
                    </label>
                    <select
                      value={selectedPath}
                      onChange={(event) => {
                        setSelectedPath(event.target.value);
                        setPage(1);
                      }}
                      className="h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm"
                    >
                      <option value="ALL">Semua jalur</option>
                      {["SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"].map(
                        (path) => (
                          <option key={path} value={path}>
                            {path}
                          </option>
                        )
                      )}
                    </select>
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Cari Siswa/PT
                    </label>
                    <input
                      type="text"
                      value={search}
                      onChange={(event) => {
                        setSearch(event.target.value);
                        setPage(1);
                      }}
                      placeholder="NISN, nama siswa, sekolah, PT, jurusan"
                      className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm"
                    />
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => {
                    setSelectedSchoolId(null);
                    setSelectedYear("ALL");
                    setSelectedJenjang("ALL");
                    setSelectedStatusSekolah("ALL");
                    setSelectedPath("ALL");
                    setSearch("");
                    setPage(1);
                  }}
                  className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-semibold"
                >
                  Reset Filter
                </button>
              </>
            ) : null}

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Breakdown Jalur
                </p>
                <div className="mt-3 space-y-3">
                  {analytics.breakdownJalur.length === 0 ? (
                    <p className="text-sm text-zinc-500">
                      Belum ada data breakdown jalur.
                    </p>
                  ) : (
                    analytics.breakdownJalur.map((item) => (
                      <div
                        key={item.jalur}
                        className="rounded-lg border border-zinc-200 bg-white p-3"
                      >
                        <div className="flex items-center justify-between gap-3">
                          <p className="text-sm font-semibold text-zinc-900">
                            {formatJalurLabel(item.jalur)}
                          </p>
                          <p className="text-xs font-semibold text-zinc-600">
                            {formatPercent(item.acceptanceRate)}
                          </p>
                        </div>
                        <div className="mt-2 grid gap-2 text-sm text-zinc-600 md:grid-cols-2">
                          <span>Pendaftar: {formatNumber(item.pendaftar)}</span>
                          <span>Diterima: {formatNumber(item.diterima)}</span>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>

              <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tren Tahunan
                </p>
                <div className="mt-3 overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className="border-b border-zinc-200 text-left text-xs tracking-[0.16em] text-zinc-500 uppercase">
                        <th className="py-2 pr-3">Tahun</th>
                        <th className="py-2 pr-3">Pendaftar</th>
                        <th className="py-2 pr-3">Diterima</th>
                        <th className="py-2 pr-3">Rate</th>
                        <th className="py-2 pr-3">YoY Pendaftar</th>
                        <th className="py-2">YoY Diterima</th>
                      </tr>
                    </thead>
                    <tbody>
                      {analytics.trendTahunan.length === 0 ? (
                        <tr>
                          <td
                            colSpan={6}
                            className="py-4 text-sm text-zinc-500"
                          >
                            Belum ada data tren tahunan.
                          </td>
                        </tr>
                      ) : (
                        analytics.trendTahunan.map((item) => (
                          <tr
                            key={item.year}
                            className="border-b border-zinc-100 text-zinc-700 last:border-b-0"
                          >
                            <td className="py-2 pr-3 font-semibold">
                              {item.year}
                            </td>
                            <td className="py-2 pr-3">
                              {formatNumber(item.pendaftar)}
                            </td>
                            <td className="py-2 pr-3">
                              {formatNumber(item.diterima)}
                            </td>
                            <td className="py-2 pr-3">
                              {formatPercent(item.acceptanceRate)}
                            </td>
                            <td className="py-2 pr-3">
                              {item.yoyGrowthPendaftar == null
                                ? "-"
                                : formatPercent(item.yoyGrowthPendaftar)}
                            </td>
                            <td className="py-2">
                              {item.yoyGrowthDiterima == null
                                ? "-"
                                : formatPercent(item.yoyGrowthDiterima)}
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {canViewSchoolSummary ? (
          <Card className="order-last">
            <CardContent className="space-y-4 p-4">
              <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Rekap Pengumuman PT per Sekolah
                  </p>
                  <p className="text-sm text-zinc-600">
                    Rekap pendaftaran dan penerimaan per jalur berdasarkan
                    filter aktif.
                  </p>
                </div>
                <div className="space-y-2">
                  <button
                    type="button"
                    onClick={() => void handleDownloadSchoolSummary()}
                    disabled={isExportingSchoolSummary}
                    className="inline-flex rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {isExportingSchoolSummary
                      ? "Menyiapkan..."
                      : "Download Rekap Sekolah"}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleDownloadAccepted()}
                    disabled={isExportingAccepted}
                    className="ml-2 inline-flex rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {isExportingAccepted
                      ? "Menyiapkan..."
                      : "Download Detail Siswa Diumumkan"}
                  </button>
                  {schoolSummaryExportError ? (
                    <p className="text-xs text-red-600">
                      {schoolSummaryExportError}
                    </p>
                  ) : null}
                  {acceptedExportError ? (
                    <p className="text-xs text-red-600">
                      {acceptedExportError}
                    </p>
                  ) : null}
                </div>
              </div>

              <Table
                wrapperClassName="max-h-[520px] overflow-auto"
                className="min-w-[1320px]"
              >
                <TableHeader className="sticky top-0 z-10 bg-slate-50">
                  <TableRow>
                    <TableHead>Nama Sekolah</TableHead>
                    <TableHead className="text-right">
                      Pendaftaran SNBP
                    </TableHead>
                    <TableHead className="text-right">
                      Penerimaan SNBP
                    </TableHead>
                    <TableHead className="text-right">
                      Pendaftaran SNBT
                    </TableHead>
                    <TableHead className="text-right">
                      Penerimaan SNBT
                    </TableHead>
                    <TableHead className="text-right">
                      Pendaftaran Mandiri
                    </TableHead>
                    <TableHead className="text-right">
                      Penerimaan Mandiri
                    </TableHead>
                    <TableHead className="text-right">
                      Pendaftaran Kedinasan
                    </TableHead>
                    <TableHead className="text-right">
                      Penerimaan Kedinasan
                    </TableHead>
                    <TableHead className="text-right">
                      Pendaftaran Lainnya
                    </TableHead>
                    <TableHead className="text-right">
                      Penerimaan Lainnya
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isSchoolSummaryLoading ? (
                    Array.from({ length: 5 }).map((_, index) => (
                      <TableRow key={`summary-loading-${index}`}>
                        <TableCell
                          colSpan={11}
                          className="py-4 text-sm text-zinc-500"
                        >
                          Memuat rekap sekolah...
                        </TableCell>
                      </TableRow>
                    ))
                  ) : isSchoolSummaryError ? (
                    <TableRow>
                      <TableCell colSpan={11} className="py-4">
                        <EmptyState
                          title="Gagal memuat rekap sekolah"
                          description="Silakan coba lagi atau periksa filter yang digunakan."
                          className="min-h-24 border-red-200 bg-red-50/30"
                        />
                      </TableCell>
                    </TableRow>
                  ) : schoolSummary.items.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={11} className="py-4">
                        <EmptyState
                          title="Belum ada rekap sekolah"
                          description="Tidak ada sekolah pada scope dan filter aktif."
                          className="min-h-24"
                        />
                      </TableCell>
                    </TableRow>
                  ) : (
                    schoolSummary.items.map((row) => (
                      <TableRow key={row.schoolId}>
                        <TableCell className="font-medium text-zinc-900">
                          {row.sekolah || "-"}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pendaftaranSnbp)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.penerimaanSnbp)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pendaftaranSnbt)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.penerimaanSnbt)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pendaftaranMandiri)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.penerimaanMandiri)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pendaftaranKedinasan)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.penerimaanKedinasan)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pendaftaranLainnya)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.penerimaanLainnya)}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        ) : null}

        {canViewSchoolSummary && !isSchoolAdmin ? (
          <Card>
            <CardContent className="space-y-4 p-4">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Perbandingan SIPRES vs Pengawas
                </p>
                <p className="text-sm text-zinc-600">
                  Data pengawas bersifat pembanding dan tidak mengubah outcome
                  resmi siswa.
                </p>
              </div>
              {selectedYear === "ALL" ? (
                <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
                  Pilih satu tahun pada filter global untuk melihat atau mengisi
                  data pembanding.
                </div>
              ) : supervisorComparison.items.some(
                  (row) => row.eligibleSnbpMissingWarning
                ) ? (
                <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
                  Data Eligible SNBP tahun {selectedYear} belum diisi. Mohon
                  lengkapi agar analisis SNBP dan pembanding Cabdin valid.
                </div>
              ) : null}
              <Table
                wrapperClassName="max-h-[520px] overflow-auto"
                className="min-w-[1480px]"
              >
                <TableHeader className="sticky top-0 z-10 bg-slate-50">
                  <TableRow>
                    <TableHead>Sekolah</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Eligible SNBP</TableHead>
                    <TableHead className="text-right">SNBP SIPRES</TableHead>
                    <TableHead className="text-right">SNBP Pengawas</TableHead>
                    <TableHead className="text-right">SNBT SIPRES</TableHead>
                    <TableHead className="text-right">SNBT Pengawas</TableHead>
                    <TableHead className="text-right">Mandiri SIPRES</TableHead>
                    <TableHead className="text-right">
                      Mandiri Pengawas
                    </TableHead>
                    <TableHead className="text-right">
                      Kedinasan SIPRES
                    </TableHead>
                    <TableHead className="text-right">
                      Kedinasan Pengawas
                    </TableHead>
                    <TableHead className="text-right">Lainnya SIPRES</TableHead>
                    <TableHead className="text-right">
                      Lainnya Pengawas
                    </TableHead>
                    {normalizedRole === "PENGAWAS" ||
                    normalizedRole === "ADMIN_MASTER" ? (
                      <TableHead>Aksi</TableHead>
                    ) : null}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {selectedYear === "ALL" ? (
                    <TableRow>
                      <TableCell
                        colSpan={14}
                        className="py-4 text-sm text-zinc-500"
                      >
                        Pilih tahun melalui filter global.
                      </TableCell>
                    </TableRow>
                  ) : isSupervisorComparisonLoading ? (
                    <TableRow>
                      <TableCell
                        colSpan={14}
                        className="py-4 text-sm text-zinc-500"
                      >
                        Memuat perbandingan...
                      </TableCell>
                    </TableRow>
                  ) : isSupervisorComparisonError ? (
                    <TableRow>
                      <TableCell colSpan={14}>
                        <EmptyState
                          title="Gagal memuat perbandingan"
                          description="Silakan coba lagi."
                        />
                      </TableCell>
                    </TableRow>
                  ) : supervisorComparison.items.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={14}>
                        <EmptyState
                          title="Belum ada data perbandingan"
                          description="Tidak ada sekolah pada filter aktif."
                        />
                      </TableCell>
                    </TableRow>
                  ) : (
                    supervisorComparison.items.map((row) => (
                      <TableRow key={`${row.schoolId}-${row.tahunLulus}`}>
                        <TableCell className="font-medium">
                          {row.sekolah || "-"}
                        </TableCell>
                        <TableCell>
                          <span className="rounded-full bg-zinc-100 px-2 py-1 text-xs font-semibold text-zinc-700">
                            {row.comparisonStatus === "SESUAI"
                              ? "Sesuai"
                              : row.comparisonStatus === "KURANG_INPUT_SIPRES"
                                ? "Kurang input SIPRES"
                                : row.comparisonStatus === "PERLU_KLARIFIKASI"
                                  ? "Perlu klarifikasi"
                                  : row.comparisonStatus ===
                                      "BELUM_ADA_DATA_SIPRES"
                                    ? "Belum ada data SIPRES"
                                    : "Belum diisi pengawas"}
                          </span>
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.eligibleSnbpCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.sipresSnbpCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pengawasSnbpCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.sipresSnbtCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pengawasSnbtCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.sipresMandiriCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pengawasMandiriCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.sipresKedinasanCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pengawasKedinasanCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.sipresLainnyaCount)}
                        </TableCell>
                        <TableCell className="text-right">
                          {formatNumber(row.pengawasLainnyaCount)}
                        </TableCell>
                        {normalizedRole === "PENGAWAS" ||
                        normalizedRole === "ADMIN_MASTER" ? (
                          <TableCell>
                            <button
                              type="button"
                              onClick={() => openSnapshotModal(row)}
                              className="rounded-md border border-zinc-200 px-3 py-1 text-xs font-semibold text-zinc-700 hover:bg-zinc-50"
                            >
                              Isi/Edit Rekap
                            </button>
                          </TableCell>
                        ) : null}
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        ) : null}

        <Card>
          <CardContent className="space-y-4 p-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Seluruh Data Pengumuman PT
                </p>
                <p className="text-sm text-zinc-600">
                  Seluruh hasil pengumuman dari sekolah, Admin Cabdin, Admin
                  Master, dan migrasi ditampilkan per record.
                </p>
              </div>
              <div
                className="flex flex-wrap gap-2"
                aria-label="Filter status validasi tabel"
              >
                {[
                  ["ALL", "Semua"],
                  ["FINAL", "Final"],
                  ["PENDING", "Pending Validasi"],
                ].map(([value, label]) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => {
                      setTableValidationStatus(value);
                      setPage(1);
                    }}
                    className={`rounded-md border px-3 py-1 text-xs font-semibold ${tableValidationStatus === value ? "border-zinc-900 bg-zinc-900 text-white" : "border-zinc-300 bg-white text-zinc-700"}`}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>

            <DataTable
              columns={[
                { key: "sekolah", label: "Sekolah" },
                { key: "nisn", label: "NISN" },
                { key: "nama", label: "Nama Siswa" },
                { key: "rombel", label: "Rombel" },
                { key: "tahun", label: "Tahun Lulus", align: "right" },
                { key: "jalur", label: "Jalur" },
                { key: "pt", label: "Nama PT" },
                { key: "jurusan", label: "Jurusan" },
                { key: "diterima", label: "Tahun Diterima", align: "right" },
                { key: "tanggal", label: "Tanggal Pengumuman" },
                { key: "sumber", label: "Sumber" },
                { key: "status", label: "Status" },
                { key: "aksi", label: "Aksi" },
              ]}
              rows={analytics.acceptedItems}
              isLoading={isLoading}
              isError={isError}
              emptyMessage="Belum ada data pengumuman PT pada scope ini."
              page={page}
              pageSize={pageSize}
              total={analytics.pagination.total}
              onPageChange={setPage}
              renderRow={(row) => (
                <TableRow key={row.id}>
                  <TableCell>{row.sekolah || "-"}</TableCell>
                  <TableCell>{row.nisn || "-"}</TableCell>
                  <TableCell>{row.namaSiswa || "-"}</TableCell>
                  <TableCell>{row.rombel || "-"}</TableCell>
                  <TableCell className="text-right">
                    {row.tahunLulus || "-"}
                  </TableCell>
                  <TableCell>{row.jalur || "-"}</TableCell>
                  <TableCell>{row.namaPt || "-"}</TableCell>
                  <TableCell>{row.jurusan || "-"}</TableCell>
                  <TableCell className="text-right">
                    {row.tahunDiterima ?? "-"}
                  </TableCell>
                  <TableCell>{formatDate(row.tanggalPengumuman)}</TableCell>
                  <TableCell>
                    {row.sourceRole === "ADMIN_MASTER"
                      ? "Admin Master"
                      : row.sourceRole === "ADMIN"
                        ? "Admin Cabdin"
                        : row.sourceRole === "MIGRATION"
                          ? "Migrasi"
                          : "Sekolah"}
                    {row.schoolConfirmedAt ? " • Dikonfirmasi Sekolah" : ""}
                  </TableCell>
                  <TableCell>
                    {row.conflictStatus === "NONE"
                      ? row.announcementStatus
                      : `Pending Validasi • ${row.conflictStatus}`}
                  </TableCell>
                  <TableCell>
                    {row.canEdit ? (
                      <button
                        type="button"
                        className="rounded-md border border-zinc-300 px-3 py-1 text-xs font-semibold"
                        onClick={() => {
                          setEditingAdminOffer({
                            id: row.id,
                            schoolId: row.schoolId,
                            schoolName: row.sekolah,
                            graduationYear: row.tahunLulus,
                            path: row.jalur,
                            studentName: row.namaSiswa,
                            nisn: row.nisn,
                            campusName: row.namaPt,
                            studyProgram: row.jurusan,
                            sourceRole: row.sourceRole,
                            conflictStatus: row.conflictStatus,
                            announcementStatus: row.announcementStatus,
                          });
                          window.scrollTo({ top: 0, behavior: "smooth" });
                        }}
                      >
                        Edit
                      </button>
                    ) : null}
                    {row.conflictStatus !== "NONE" ? (
                      <a
                        href="/siswa/pending-validasi"
                        className="ml-2 inline-flex rounded-md border border-amber-300 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-900"
                      >
                        Tinjau
                      </a>
                    ) : null}
                    {!row.canEdit && row.conflictStatus === "NONE" ? "-" : null}
                  </TableCell>
                </TableRow>
              )}
            />
          </CardContent>
        </Card>

        <section className="space-y-3 rounded-xl border border-zinc-200 bg-white p-4">
          <p className="text-sm font-semibold text-zinc-900">
            Batalkan Pengumuman PT
          </p>
          <div className="grid gap-3 md:grid-cols-3">
            <input
              type="text"
              value={nisn}
              onChange={(event) => setNisn(event.target.value)}
              placeholder="NISN"
              className="rounded-md border border-zinc-200 px-3 py-2 text-sm"
            />
            <input
              type="number"
              value={tahunLulus}
              onChange={(event) => setTahunLulus(event.target.value)}
              placeholder="Tahun Lulus"
              className="rounded-md border border-zinc-200 px-3 py-2 text-sm"
            />
            <button
              type="button"
              onClick={() => void handleCancelAnnouncement()}
              disabled={isSubmitting}
              className="rounded-md border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 transition hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isSubmitting ? "Memproses..." : "Batalkan Pengumuman"}
            </button>
          </div>
        </section>
        {editingSnapshot ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
            <div className="w-full max-w-2xl rounded-xl bg-white p-5 shadow-xl">
              <div className="mb-4">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Rekap Pengawas
                </p>
                <h2 className="text-lg font-semibold text-zinc-900">
                  {editingSnapshot.sekolah}
                </h2>
                <p className="text-sm text-zinc-600">
                  Tahun lulus{" "}
                  {selectedYear === "ALL"
                    ? editingSnapshot.tahunLulus
                    : selectedYear}
                </p>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                {[
                  ["snbp", "Jumlah diterima SNBP"],
                  ["snbt", "Jumlah diterima SNBT"],
                  ["mandiri", "Jumlah diterima Mandiri"],
                  ["kedinasan", "Jumlah diterima Kedinasan"],
                  ["lainnya", "Jumlah diterima Lainnya"],
                ].map(([key, label]) => (
                  <label key={key} className="space-y-1 text-sm">
                    <span className="font-medium text-zinc-700">{label}</span>
                    <input
                      type="number"
                      min={0}
                      value={snapshotForm[key as keyof typeof snapshotForm]}
                      onChange={(event) =>
                        setSnapshotForm((current) => ({
                          ...current,
                          [key]: event.target.value,
                        }))
                      }
                      className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm"
                    />
                  </label>
                ))}
                <label className="space-y-1 text-sm md:col-span-2">
                  <span className="font-medium text-zinc-700">Catatan</span>
                  <textarea
                    value={snapshotForm.note}
                    onChange={(event) =>
                      setSnapshotForm((current) => ({
                        ...current,
                        note: event.target.value,
                      }))
                    }
                    className="min-h-24 w-full rounded-md border border-zinc-200 px-3 py-2 text-sm"
                  />
                </label>
              </div>
              <div className="mt-5 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setEditingSnapshot(null)}
                  className="rounded-md border border-zinc-200 px-4 py-2 text-sm font-semibold text-zinc-700"
                >
                  Batal
                </button>
                <button
                  type="button"
                  onClick={() => void handleSaveSnapshot()}
                  disabled={isSavingSnapshot}
                  className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                >
                  {isSavingSnapshot ? "Menyimpan..." : "Simpan Rekap"}
                </button>
              </div>
            </div>
          </div>
        ) : null}
      </section>
    </AdminRouteGuard>
  );
}
