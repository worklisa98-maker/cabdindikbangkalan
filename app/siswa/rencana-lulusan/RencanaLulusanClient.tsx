"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { Card, CardContent } from "@/components/ui/card";
import { TypeaheadInput } from "@/components/typeahead-input";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import {
  canUseScopedSchoolFilter,
  isSchoolAdminRole,
} from "@/lib/auth/role-scope";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import {
  SYSTEM_MAX_PT_CHOICES,
  defaultPtChoiceRuleMap,
} from "@/lib/pt-choice-rules";
import SiswaModuleActions from "@/components/siswa/SiswaModuleActions";
import UkkPanel from "@/components/siswa/UkkPanel";
import RencanaTabs, {
  type RencanaTabKey,
} from "@/components/siswa/RencanaTabs";
import RowActionDropdown, {
  type RowActionItem,
} from "@/components/siswa/pending-validasi/RowActionDropdown";

type PipelineRow = {
  id: number;
  school_id?: number;
  schoolId?: number;
  school_name?: string;
  schoolName?: string;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  status: string;
  filled_paths?: string[];
  final_status?: string;
  finalStatus?: string;
  finalized_at?: string | null;
  finalizedAt?: string | null;
  year_closed?: boolean;
  yearClosed?: boolean;
};

type PipelineMeta = {
  page: number;
  pageSize: number;
  total: number;
};

type PipelineListResponse = {
  data: PipelineRow[];
  meta: PipelineMeta;
};

type ScopedSchool = {
  id: number;
  namaSekolah: string;
  npsnSekolah?: string | null;
  bentukPendidikan?: string | null;
};

type EligibleSNBPItem = {
  schoolId: number;
  sekolah: string;
  tahunLulus: number;
  eligibleCount: number;
  note: string;
  updatedAt: string | null;
  statusLabel: string;
  needsWarning: boolean;
};

type PTRegistrationSummary = {
  schoolId: number;
  schoolName: string;
  tahunLulus: number;
  totalSiswaAkhir: number;
  eligibleSNBP: number | null;
  eligibleRequired: boolean;
  eligibleStartYear: number;
  counts: Record<
    string,
    { registeredCount: number; source: string; sourceLabel: string }
  >;
  autoSummaryNotes: string;
  validationMessage: string;
};

type EntrepreneurPlanSummary = {
  schoolId: number;
  schoolName: string;
  tahunLulus: number;
  plannedCount: number;
  source: string;
  sourceLabel: string;
  autoSummaryNotes: string;
};

type PtChoicePayload = {
  pilihan_ke: number;
  jalur: string;
  kampus_id?: number | null;
  kampus: string;
  prodi: string;
  jenjang_prodi?: string;
};

type KedinasanJenis = "SEKOLAH_KEDINASAN" | "TNI" | "POLRI";
type KedinasanMatra = "AD" | "AL" | "AU";
type KedinasanPendidikan = "AKADEMI" | "BINTARA" | "TAMTAMA";
type PtJalurTab = "SNBP" | "SNBT" | "MANDIRI" | "KEDINASAN" | "LAINNYA";

type PtKedinasanPayload = {
  jenis: string;
  matra?: string;
  jenis_pendidikan?: string;
};

type PtDetailResponse = {
  student: {
    id: number;
    nisn: string;
    namaSiswa: string;
    rombel: string;
    tahunLulus: number;
    finalized_at?: string | null;
  };
  status: string;
  choices: PtChoicePayload[];
  kedinasan?: PtKedinasanPayload | null;
};

type PtChoiceRulesResponse = {
  system_max_choices: number;
  tahun_lulus?: number;
  rules: Record<PtJalurTab, number>;
};

type PtDraftChoice = {
  kampusId: number | null;
  kampusQuery: string;
  legacyKampus: string;
  prodi: string;
  jenjangProdi: string;
};

type PtKedinasanDraft = {
  jenis: KedinasanJenis;
  matra: KedinasanMatra | "";
  jenisPendidikan: KedinasanPendidikan | "";
};

type PtDraft = {
  byJalur: Record<PtJalurTab, PtDraftChoice[]>;
  kedinasan: PtKedinasanDraft;
};

type KampusRefItem = {
  id: number;
  nama: string;
};

const ptJalurTabs = [
  { value: "SNBP", label: "SNBP" },
  { value: "SNBT", label: "SNBT" },
  { value: "MANDIRI", label: "MANDIRI" },
  { value: "KEDINASAN", label: "KEDINASAN" },
  { value: "LAINNYA", label: "LAINNYA" },
] as const satisfies ReadonlyArray<{ value: PtJalurTab; label: string }>;

const jenjangProdiOptions = [
  "D1",
  "D2",
  "D3",
  "D4",
  "S1",
  "S2",
  "S3",
  "UNKNOWN",
];
const kedinasanJenisOptions: ReadonlyArray<KedinasanJenis> = [
  "SEKOLAH_KEDINASAN",
  "TNI",
  "POLRI",
];
const kedinasanMatraOptions: ReadonlyArray<KedinasanMatra> = ["AD", "AL", "AU"];
const kedinasanPendidikanOptions: ReadonlyArray<KedinasanPendidikan> = [
  "AKADEMI",
  "BINTARA",
  "TAMTAMA",
];

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

const readBoolean = (value: unknown) => {
  if (typeof value === "boolean") return value;
  if (typeof value === "number") return value !== 0;
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return normalized === "true" || normalized === "1" || normalized === "yes";
  }
  return false;
};

const parseApiErrorMessage = (payload: unknown, fallback: string) => {
  if (!isRecord(payload)) {
    return fallback;
  }

  const details = payload.details;
  if (typeof details === "string" && details.trim()) {
    return details.trim();
  }
  if (isRecord(details)) {
    const detailMessage = readString(details.message ?? details.error);
    if (detailMessage) {
      return detailMessage;
    }
  }

  const message = readString(payload.message);
  if (message) {
    return message;
  }
  const error = readString(payload.error);
  if (error && error !== "BAD_REQUEST") {
    return error;
  }
  return fallback;
};

const normalizeYearClosedMessage = (message: string, tahunLulus: number) => {
  const normalized = message.trim();
  const lower = normalized.toLowerCase();
  if (
    lower.includes("tahun") &&
    (lower.includes("ditutup") || lower.includes("tutup buku"))
  ) {
    return normalized;
  }
  if (lower.includes("year") && lower.includes("closed")) {
    return `Tahun lulusan ${tahunLulus} sudah ditutup. Tidak dapat melakukan perubahan.`;
  }
  return normalized;
};

const normalizeJalurTab = (value: string): PtJalurTab | null => {
  const upper = value.trim().toUpperCase();
  if (upper === "SNBM") {
    return "MANDIRI";
  }
  if (
    upper === "SNBP" ||
    upper === "SNBT" ||
    upper === "MANDIRI" ||
    upper === "KEDINASAN" ||
    upper === "LAINNYA"
  ) {
    return upper;
  }
  return null;
};

const hasSavedDataForJalur = (
  jalur: PtJalurTab,
  detail: PtDetailResponse | null
): boolean => {
  if (!detail) return false;
  const choices = Array.isArray(detail.choices) ? detail.choices : [];
  if (jalur === "KEDINASAN") {
    const jenis = readString(detail.kedinasan?.jenis).toUpperCase();
    const matra = readString(detail.kedinasan?.matra).toUpperCase();
    const jenisPendidikan = readString(
      detail.kedinasan?.jenis_pendidikan
    ).toUpperCase();
    const hasKedinasanChoices = choices.some(
      (choice) => normalizeJalurTab(readString(choice.jalur)) === "KEDINASAN"
    );
    return (
      hasKedinasanChoices ||
      jenis === "TNI" ||
      jenis === "POLRI" ||
      matra !== "" ||
      jenisPendidikan !== ""
    );
  }
  return (
    choices.some(
      (choice) => normalizeJalurTab(readString(choice.jalur)) === jalur
    ) === true
  );
};

const createEmptyPtChoices = (): PtDraftChoice[] =>
  Array.from({ length: SYSTEM_MAX_PT_CHOICES }, () => ({
    kampusId: null,
    kampusQuery: "",
    legacyKampus: "",
    prodi: "",
    jenjangProdi: "UNKNOWN",
  }));

const emptyPtDraft = (): PtDraft => ({
  byJalur: {
    SNBP: createEmptyPtChoices(),
    SNBT: createEmptyPtChoices(),
    MANDIRI: createEmptyPtChoices(),
    KEDINASAN: createEmptyPtChoices(),
    LAINNYA: createEmptyPtChoices(),
  },
  kedinasan: {
    jenis: "SEKOLAH_KEDINASAN",
    matra: "",
    jenisPendidikan: "",
  },
});

const withoutJalurInFilledPaths = (
  row: PipelineRow,
  jalur: PtJalurTab
): PipelineRow => {
  if (!Array.isArray(row.filled_paths)) {
    return row;
  }
  return {
    ...row,
    filled_paths: row.filled_paths.filter(
      (value) => normalizeJalurTab(readString(value)) !== jalur
    ),
  };
};

const fetchPtPipeline = async (params: {
  page: number;
  pageSize: number;
  year?: number;
  schoolId?: number | null;
}): Promise<PipelineListResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.year && params.year > 0) {
    query.set("year", String(params.year));
  }
  if (params.schoolId && params.schoolId > 0) {
    query.set("school_id", String(params.schoolId));
  }

  const response = await apiFetch(
    `/api/siswa/applications/pipeline?${query.toString()}`,
    { cache: "no-store" }
  );
  if (!response.ok) {
    throw new Error("Failed to fetch PT pipeline.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const rows = Array.isArray(data?.data)
    ? data.data.map((item: unknown) => {
        const row = isRecord(item) ? item : {};
        return {
          ...(row as PipelineRow),
          schoolId: readNumber(row.school_id ?? row.schoolId),
          schoolName: readString(row.school_name ?? row.schoolName),
        };
      })
    : [];
  return {
    data: rows,
    meta: isRecord(data?.meta)
      ? (data.meta as PipelineMeta)
      : {
          page: params.page,
          pageSize: params.pageSize,
          total: 0,
        },
  };
};

const fetchEligibleSNBP = async (params: {
  year: number;
  schoolId?: number | null;
}): Promise<EligibleSNBPItem | null> => {
  const query = new URLSearchParams();
  query.set("year", String(params.year));
  if (params.schoolId && params.schoolId > 0) {
    query.set("school_id", String(params.schoolId));
  }
  const response = await apiFetch(
    `/api/siswa/eligible-snbp?${query.toString()}`,
    { cache: "no-store" }
  );
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(
      parseApiErrorMessage(payload, "Gagal memuat Data Eligible SNBP.")
    );
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const rows = Array.isArray(data?.items) ? data.items : [];
  const first = rows.find((row: unknown) => {
    const item = isRecord(row) ? row : {};
    const schoolID = readNumber(item.school_id ?? item.schoolId);
    return (
      !params.schoolId || params.schoolId <= 0 || schoolID === params.schoolId
    );
  });
  if (!first) {
    return null;
  }
  const item = isRecord(first) ? first : {};
  return {
    schoolId: readNumber(item.school_id ?? item.schoolId),
    sekolah: readString(item.sekolah),
    tahunLulus: readNumber(item.tahun_lulus ?? item.tahunLulus),
    eligibleCount: readNumber(item.eligible_count ?? item.eligibleCount),
    note: readString(item.note),
    updatedAt: readString(item.updated_at ?? item.updatedAt) || null,
    statusLabel: readString(item.status_label ?? item.statusLabel),
    needsWarning: Boolean(item.needs_warning ?? item.needsWarning),
  };
};

const normalizePTRegistrationSummary = (
  item: unknown
): PTRegistrationSummary => {
  const row = isRecord(item) ? item : {};
  const rawCounts = isRecord(row.counts) ? row.counts : {};
  const counts: PTRegistrationSummary["counts"] = {};
  for (const key of ["SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"]) {
    const countRow = isRecord(rawCounts[key]) ? rawCounts[key] : {};
    counts[key] = {
      registeredCount: readNumber(
        countRow.registered_count ?? countRow.registeredCount
      ),
      source: readString(countRow.source),
      sourceLabel: readString(countRow.source_label ?? countRow.sourceLabel),
    };
  }
  const eligibleValue = row.eligible_snbp ?? row.eligibleSNBP;
  return {
    schoolId: readNumber(row.school_id ?? row.schoolId),
    schoolName: readString(row.school_name ?? row.schoolName),
    tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
    totalSiswaAkhir: readNumber(row.total_siswa_akhir ?? row.totalSiswaAkhir),
    eligibleSNBP:
      eligibleValue === null || eligibleValue === undefined
        ? null
        : readNumber(eligibleValue),
    eligibleRequired: readBoolean(
      row.eligible_required ?? row.eligibleRequired
    ),
    eligibleStartYear: readNumber(
      row.eligible_start_year ?? row.eligibleStartYear
    ),
    counts,
    autoSummaryNotes: readString(
      row.auto_summary_notes ?? row.autoSummaryNotes
    ),
    validationMessage: readString(
      row.validation_message ?? row.validationMessage
    ),
  };
};

const fetchPTRegistrationSummaries = async (params: {
  year: number;
  schoolId?: number | null;
}): Promise<PTRegistrationSummary[]> => {
  const query = new URLSearchParams();
  query.set("year", String(params.year));
  if (params.schoolId && params.schoolId > 0)
    query.set("school_id", String(params.schoolId));
  const response = await apiFetch(
    `/api/siswa/pt-registration-counts?${query.toString()}`,
    { cache: "no-store" }
  );
  if (!response.ok) throw new Error("Gagal memuat rekap pendaftar PT.");
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return Array.isArray(data?.items)
    ? data.items.map(normalizePTRegistrationSummary)
    : [];
};

const normalizeEntrepreneurPlanSummary = (
  item: unknown
): EntrepreneurPlanSummary => {
  const row = isRecord(item) ? item : {};
  return {
    schoolId: readNumber(row.school_id ?? row.schoolId),
    schoolName: readString(row.school_name ?? row.schoolName),
    tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
    plannedCount: readNumber(row.planned_count ?? row.plannedCount),
    source: readString(row.source),
    sourceLabel: readString(row.source_label ?? row.sourceLabel),
    autoSummaryNotes: readString(
      row.auto_summary_notes ?? row.autoSummaryNotes
    ),
  };
};

const fetchEntrepreneurPlanSummaries = async (params: {
  year: number;
  schoolId?: number | null;
}): Promise<EntrepreneurPlanSummary[]> => {
  const query = new URLSearchParams();
  query.set("year", String(params.year));
  if (params.schoolId && params.schoolId > 0)
    query.set("school_id", String(params.schoolId));
  const response = await apiFetch(
    `/api/siswa/wirausaha-plan-counts?${query.toString()}`,
    { cache: "no-store" }
  );
  if (!response.ok) throw new Error("Gagal memuat rekap rencana wirausaha.");
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return Array.isArray(data?.items)
    ? data.items.map(normalizeEntrepreneurPlanSummary)
    : [];
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
      bentukPendidikan?: string | null;
    }>;
  };
  return (payload.data ?? []).map((item) => ({
    id: item.id,
    namaSekolah: item.namaSekolah,
    npsnSekolah: item.npsnSekolah ?? null,
    bentukPendidikan: item.bentukPendidikan ?? null,
  }));
};

const fetchKampusReferensi = async (
  query: string,
  limit = 20
): Promise<KampusRefItem[]> => {
  const params = new URLSearchParams();
  if (query.trim()) {
    params.set("q", query.trim());
  }
  params.set("limit", String(limit));
  const response = await apiFetch(`/api/referensi/kampus?${params.toString()}`);
  if (!response.ok) {
    throw new Error("Gagal memuat referensi kampus.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const rawItems = Array.isArray(data?.items) ? data.items : [];
  return rawItems
    .map((item: unknown) => ({
      id: readNumber(isRecord(item) ? item.id : 0),
      nama: readString(isRecord(item) ? item.nama : ""),
    }))
    .filter((item: KampusRefItem) => item.id > 0 && item.nama);
};

const fetchPtChoiceRules = async (
  tahunLulus: number
): Promise<PtChoiceRulesResponse> => {
  const response = await apiFetch(
    `/api/siswa/pt-choice-rules?tahun_lulus=${tahunLulus}`,
    { cache: "no-store" }
  );
  if (!response.ok) {
    throw new Error("Gagal memuat batas pilihan PT.");
  }
  const payload = (await response.json().catch(() => null)) as {
    data?: Partial<PtChoiceRulesResponse>;
  } | null;
  const data = payload?.data;
  const rules = defaultPtChoiceRuleMap();
  for (const jalur of ptJalurTabs.map((item) => item.value)) {
    const value = Number(data?.rules?.[jalur] ?? SYSTEM_MAX_PT_CHOICES);
    rules[jalur] =
      Number.isFinite(value) && value >= 1 && value <= SYSTEM_MAX_PT_CHOICES
        ? value
        : SYSTEM_MAX_PT_CHOICES;
  }
  return {
    system_max_choices:
      Number(data?.system_max_choices) || SYSTEM_MAX_PT_CHOICES,
    tahun_lulus: tahunLulus,
    rules,
  };
};

export default function RencanaLulusanClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { status, user } = useAuth();
  const isSchoolAdmin = isSchoolAdminRole(user?.role);
  const showSchoolFilter = canUseScopedSchoolFilter(user?.role);
  const showSchoolColumn = showSchoolFilter;
  const isReadOnly = !isSchoolAdmin;
  const schoolLevel = (user?.schoolLevel ?? "").toUpperCase();
  const isSMK = schoolLevel === "SMK";
  const allowedLevels = useMemo(
    () => (user?.allowedLevels ?? []).map((item) => item.toUpperCase().trim()),
    [user?.allowedLevels]
  );
  const queryClient = useQueryClient();
  const currentSchoolId =
    typeof user?.schoolId === "number" && user.schoolId > 0 ? user.schoolId : 0;
  const [ptShortcutCancellingKey, setPtShortcutCancellingKey] = useState("");
  const [schoolQuery, setSchoolQuery] = useState("");
  const [selectedSchoolId, setSelectedSchoolId] = useState<number | null>(null);

  const { data: scopedSchools = [] } = useQuery({
    queryKey: ["rencana-scoped-schools", user?.id ?? "anon"],
    queryFn: fetchScopedSchools,
    enabled: status === "authenticated" && showSchoolFilter,
  });

  useEffect(() => {
    if (!showSchoolFilter || selectedSchoolId == null) {
      return;
    }
    const selected = scopedSchools.find(
      (school) => school.id === selectedSchoolId
    );
    if (!selected) {
      setSelectedSchoolId(null);
      setSchoolQuery("");
      return;
    }
    if (!schoolQuery) {
      setSchoolQuery(selected.namaSekolah);
    }
  }, [schoolQuery, scopedSchools, selectedSchoolId, showSchoolFilter]);

  const schoolNameById = useMemo(
    () =>
      new Map(scopedSchools.map((school) => [school.id, school.namaSekolah])),
    [scopedSchools]
  );

  const schoolSuggestionItems = useMemo(() => {
    if (!showSchoolFilter) {
      return [];
    }
    const term = schoolQuery.trim().toLowerCase();
    const filtered = term
      ? scopedSchools.filter((school) =>
          school.namaSekolah.toLowerCase().includes(term)
        )
      : scopedSchools;
    return [
      { key: "ALL", label: "Semua sekolah", value: "" },
      ...filtered.map((school) => ({
        key: school.id,
        label: school.namaSekolah,
        value: school.namaSekolah,
        subLabel: school.npsnSekolah ? `NPSN ${school.npsnSekolah}` : undefined,
      })),
    ];
  }, [schoolQuery, scopedSchools, showSchoolFilter]);

  const selectedScopedSchool = useMemo(
    () =>
      selectedSchoolId == null
        ? null
        : (scopedSchools.find((school) => school.id === selectedSchoolId) ??
          null),
    [scopedSchools, selectedSchoolId]
  );
  const hasAnyScopedSMKSchool = useMemo(
    () =>
      scopedSchools.some(
        (school) =>
          (school.bentukPendidikan ?? "").trim().toUpperCase() === "SMK"
      ),
    [scopedSchools]
  );
  const hasUkkAccess = useMemo(() => {
    if (isSchoolAdmin) {
      return isSMK;
    }
    if (user?.role === "PENGAWAS" || user?.role === "ADMIN_MASTER") {
      return true;
    }
    if (user?.role === "ADMIN") {
      return allowedLevels.includes("SMK");
    }
    return false;
  }, [allowedLevels, isSMK, isSchoolAdmin, user?.role]);
  const requiresScopedSMKSelection =
    showSchoolFilter && !isSchoolAdmin && selectedSchoolId == null;
  const hasResolvedSMKScope = useMemo(() => {
    if (!hasUkkAccess) {
      return false;
    }
    if (isSchoolAdmin) {
      return isSMK;
    }
    if (selectedScopedSchool) {
      return (
        (selectedScopedSchool.bentukPendidikan ?? "").trim().toUpperCase() ===
        "SMK"
      );
    }
    return !requiresScopedSMKSelection && hasAnyScopedSMKSchool;
  }, [
    hasAnyScopedSMKSchool,
    hasUkkAccess,
    isSMK,
    isSchoolAdmin,
    requiresScopedSMKSelection,
    selectedScopedSchool,
  ]);
  const ukkEmptyStateMessage = useMemo(() => {
    if (!hasUkkAccess) {
      return "UKK hanya tersedia untuk sekolah SMK.";
    }
    if (isSchoolAdmin && !isSMK) {
      return "UKK hanya tersedia untuk sekolah SMK.";
    }
    if (requiresScopedSMKSelection) {
      return hasAnyScopedSMKSchool
        ? "Pilih sekolah SMK untuk melihat data UKK."
        : "Scope saat ini tidak memiliki sekolah SMK.";
    }
    if (selectedScopedSchool && !hasResolvedSMKScope) {
      return "UKK hanya tersedia untuk sekolah SMK. Pilih sekolah SMK pada filter.";
    }
    if (!hasResolvedSMKScope) {
      return "Scope saat ini tidak memiliki sekolah SMK.";
    }
    return "";
  }, [
    hasAnyScopedSMKSchool,
    hasResolvedSMKScope,
    hasUkkAccess,
    isSMK,
    isSchoolAdmin,
    requiresScopedSMKSelection,
    selectedScopedSchool,
  ]);

  const activeTab = useMemo<RencanaTabKey>(() => {
    const raw = (searchParams.get("tab") ?? "PT").toUpperCase();
    if (raw === "UKK" && !hasUkkAccess) {
      return "PT";
    }
    if (raw === "PT" || raw === "WIRAUSAHA" || raw === "UKK") {
      return raw as RencanaTabKey;
    }
    return "PT";
  }, [hasUkkAccess, searchParams]);

  const tabItems = useMemo(
    () =>
      [
        { key: "PT" as const, label: "Pendaftaran PT" },
        { key: "WIRAUSAHA" as const, label: "Rencana Wirausaha" },
        hasUkkAccess ? { key: "UKK" as const, label: "UKK (SMK)" } : null,
      ].filter(Boolean) as { key: RencanaTabKey; label: string }[],
    [hasUkkAccess]
  );

  const handleTabChange = useCallback((nextTab: RencanaTabKey) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("tab", nextTab);
    router.replace(params.toString() ? `?${params.toString()}` : "?", {
      scroll: false,
    });
  }, [router, searchParams]);

  useEffect(() => {
    if (!hasUkkAccess && activeTab === "UKK") {
      handleTabChange("PT");
    }
  }, [activeTab, handleTabChange, hasUkkAccess]);

  const [ptPage, setPtPage] = useState(1);
  const [ptYear, setPtYear] = useState(() => new Date().getFullYear());
  const [eligibleModalOpen, setEligibleModalOpen] = useState(false);
  const [eligibleCountInput, setEligibleCountInput] = useState("");
  const [eligibleNoteInput, setEligibleNoteInput] = useState("");
  const [eligibleMessage, setEligibleMessage] = useState("");
  const [eligibleError, setEligibleError] = useState("");
  const [isEligibleSaving, setIsEligibleSaving] = useState(false);
  const [ptCountInputs, setPtCountInputs] = useState<Record<string, string>>({
    SNBP: "",
    SNBT: "",
    MANDIRI: "",
    KEDINASAN: "",
    LAINNYA: "",
  });
  const [ptCountMessage, setPtCountMessage] = useState("");
  const ptPageSize = 10;

  const {
    data: ptData,
    isLoading: isPtLoading,
    isError: isPtError,
    refetch: refetchPt,
  } = useQuery({
    queryKey: [
      "pt-pipeline",
      ptPage,
      ptPageSize,
      ptYear,
      showSchoolFilter ? (selectedSchoolId ?? "ALL") : "SCHOOL_ADMIN",
    ],
    queryFn: () =>
      fetchPtPipeline({
        page: ptPage,
        pageSize: ptPageSize,
        year: ptYear,
        schoolId: showSchoolFilter ? selectedSchoolId : null,
      }),
    enabled: status === "authenticated" && activeTab === "PT",
  });

  const eligibleSchoolId = showSchoolFilter ? selectedSchoolId : null;
  const shouldFetchEligible =
    status === "authenticated" &&
    activeTab === "PT" &&
    (isSchoolAdmin || eligibleSchoolId != null);
  const {
    data: eligibleSNBP,
    isLoading: isEligibleLoading,
    isError: isEligibleError,
    refetch: refetchEligibleSNBP,
  } = useQuery({
    queryKey: [
      "rencana-eligible-snbp",
      ptYear,
      isSchoolAdmin ? "SCHOOL_ADMIN" : (eligibleSchoolId ?? "NO_SCHOOL"),
    ],
    queryFn: () =>
      fetchEligibleSNBP({
        year: ptYear,
        schoolId: eligibleSchoolId,
      }),
    enabled: shouldFetchEligible,
  });
  const hasEligibleSNBP = Boolean(eligibleSNBP?.updatedAt);
  const canEditEligibleSNBP =
    isSchoolAdmin ||
    (user?.role === "ADMIN_MASTER" && eligibleSchoolId != null);
  const eligibleSNBPWarning =
    !hasEligibleSNBP && shouldFetchEligible
      ? `Data Eligible SNBP tahun ${ptYear} belum diisi. Mohon lengkapi agar analisis SNBP valid.`
      : "";

  const ptRegistrationQuery = useQuery({
    queryKey: [
      "rencana-pt-registration-counts",
      ptYear,
      showSchoolFilter ? (selectedSchoolId ?? "ALL") : "SCHOOL_ADMIN",
    ],
    queryFn: () =>
      fetchPTRegistrationSummaries({
        year: ptYear,
        schoolId: showSchoolFilter ? selectedSchoolId : null,
      }),
    enabled: status === "authenticated" && activeTab === "PT",
  });
  const ptRegistrationFirst = ptRegistrationQuery.data?.[0] ?? null;

  useEffect(() => {
    if (!isSchoolAdmin || !ptRegistrationFirst) return;
    setPtCountInputs({
      SNBP: String(ptRegistrationFirst.counts.SNBP?.registeredCount ?? 0),
      SNBT: String(ptRegistrationFirst.counts.SNBT?.registeredCount ?? 0),
      MANDIRI: String(ptRegistrationFirst.counts.MANDIRI?.registeredCount ?? 0),
      KEDINASAN: String(
        ptRegistrationFirst.counts.KEDINASAN?.registeredCount ?? 0
      ),
      LAINNYA: String(ptRegistrationFirst.counts.LAINNYA?.registeredCount ?? 0),
    });
  }, [isSchoolAdmin, ptRegistrationFirst]);

  const ptCountMutation = useMutation({
    mutationFn: async () => {
      const counts = Object.fromEntries(
        Object.entries(ptCountInputs).map(([key, value]) => {
          const parsed = Number(value);
          if (!Number.isFinite(parsed) || parsed < 0) {
            throw new Error(
              "Jumlah pendaftar PT wajib berupa angka 0 atau lebih."
            );
          }
          return [key, parsed];
        })
      );
      const response = await apiFetch("/api/siswa/pt-registration-counts", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tahun_lulus: ptYear, counts }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(
          parseApiErrorMessage(payload, "Gagal menyimpan rekap pendaftar PT.")
        );
      }
    },
    onSuccess: () => {
      setPtCountMessage("Rekap pendaftar PT tersimpan.");
      void ptRegistrationQuery.refetch();
    },
  });

  const rawPtRows = useMemo(() => ptData?.data ?? [], [ptData?.data]);
  const { visibleRows: ptRows, filteredCount: ptFilteredCount } =
    useMemo(() => {
      const visibleRows = rawPtRows.filter((row) => {
        const finalStatusValue = readString(
          row.final_status ?? row.finalStatus
        );
        const finalizedAtValue = readString(
          row.finalized_at ?? row.finalizedAt
        );
        return !finalStatusValue && !finalizedAtValue;
      });
      return {
        visibleRows,
        filteredCount: rawPtRows.length - visibleRows.length,
      };
    }, [rawPtRows]);
  const ptMeta = ptData?.meta ?? {
    page: ptPage,
    pageSize: ptPageSize,
    total: 0,
  };

  const ptColumns = useMemo(() => {
    const columns: Array<{
      key: string;
      label: string;
      align?: "left" | "right" | "center";
    }> = [];
    if (showSchoolColumn) {
      columns.push({ key: "school", label: "Sekolah" });
    }
    columns.push(
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "tahunLulus", label: "Tahun Lulus", align: "right" },
      { key: "status", label: "Status" },
      { key: "actions", label: "AKSI", align: "center" }
    );
    return columns;
  }, [showSchoolColumn]);

  const [ptTarget, setPtTarget] = useState<PipelineRow | null>(null);
  const [ptModalKey, setPtModalKey] = useState(0);
  const [ptDetail, setPtDetail] = useState<PtDetailResponse | null>(null);
  const [activePtJalur, setActivePtJalur] = useState<PtJalurTab>("SNBP");
  const [ptDraft, setPtDraft] = useState<PtDraft>(emptyPtDraft());
  const [ptDetailError, setPtDetailError] = useState("");
  const [ptSaveMessage, setPtSaveMessage] = useState("");
  const [ptSaveError, setPtSaveError] = useState("");
  const [isPtLoadingDetail, setIsPtLoadingDetail] = useState(false);
  const [isPtSaving, setIsPtSaving] = useState(false);
  const [isPtCancellingPath, setIsPtCancellingPath] = useState(false);
  const activePtChoices =
    ptDraft.byJalur[activePtJalur] ?? createEmptyPtChoices();
  const { data: ptChoiceRules } = useQuery({
    queryKey: ["pt-choice-rules", ptTarget?.tahunLulus ?? 0],
    queryFn: () => fetchPtChoiceRules(ptTarget?.tahunLulus ?? 0),
    enabled:
      status === "authenticated" &&
      activeTab === "PT" &&
      Boolean(ptTarget?.tahunLulus),
    staleTime: 60_000,
  });
  const ptChoiceRuleMap = ptChoiceRules?.rules ?? defaultPtChoiceRuleMap();
  const activePtLimit = Math.max(
    1,
    Math.min(
      SYSTEM_MAX_PT_CHOICES,
      Number(ptChoiceRuleMap[activePtJalur] ?? SYSTEM_MAX_PT_CHOICES)
    )
  );
  const activePtLimitMessage =
    ptTarget && activePtLimit < SYSTEM_MAX_PT_CHOICES
      ? `Jalur ${activePtJalur} tahun lulus ${ptTarget.tahunLulus} maksimal ${activePtLimit} pilihan. Pilihan ke-${activePtLimit + 1} sampai ke-${SYSTEM_MAX_PT_CHOICES} tidak boleh diisi.`
      : "";
  const isKedinasanAcademicMode =
    activePtJalur !== "KEDINASAN" ||
    ptDraft.kedinasan.jenis === "SEKOLAH_KEDINASAN";

  const debouncedKampusQuery0 = useDebouncedValue(
    activePtChoices[0]?.kampusQuery ?? "",
    250
  );
  const debouncedKampusQuery1 = useDebouncedValue(
    activePtChoices[1]?.kampusQuery ?? "",
    250
  );
  const debouncedKampusQuery2 = useDebouncedValue(
    activePtChoices[2]?.kampusQuery ?? "",
    250
  );
  const debouncedKampusQuery3 = useDebouncedValue(
    activePtChoices[3]?.kampusQuery ?? "",
    250
  );

  const { data: ptKampusOptions0 = [], isFetching: isPtKampusLoading0 } =
    useQuery({
      queryKey: ["ref-kampus", activePtJalur, 0, debouncedKampusQuery0],
      queryFn: () => fetchKampusReferensi(debouncedKampusQuery0),
      enabled:
        status === "authenticated" &&
        activeTab === "PT" &&
        Boolean(ptTarget) &&
        isKedinasanAcademicMode &&
        debouncedKampusQuery0.trim().length > 0,
      staleTime: 30_000,
    });
  const { data: ptKampusOptions1 = [], isFetching: isPtKampusLoading1 } =
    useQuery({
      queryKey: ["ref-kampus", activePtJalur, 1, debouncedKampusQuery1],
      queryFn: () => fetchKampusReferensi(debouncedKampusQuery1),
      enabled:
        status === "authenticated" &&
        activeTab === "PT" &&
        Boolean(ptTarget) &&
        isKedinasanAcademicMode &&
        debouncedKampusQuery1.trim().length > 0,
      staleTime: 30_000,
    });
  const { data: ptKampusOptions2 = [], isFetching: isPtKampusLoading2 } =
    useQuery({
      queryKey: ["ref-kampus", activePtJalur, 2, debouncedKampusQuery2],
      queryFn: () => fetchKampusReferensi(debouncedKampusQuery2),
      enabled:
        status === "authenticated" &&
        activeTab === "PT" &&
        Boolean(ptTarget) &&
        isKedinasanAcademicMode &&
        debouncedKampusQuery2.trim().length > 0,
      staleTime: 30_000,
    });
  const { data: ptKampusOptions3 = [], isFetching: isPtKampusLoading3 } =
    useQuery({
      queryKey: ["ref-kampus", activePtJalur, 3, debouncedKampusQuery3],
      queryFn: () => fetchKampusReferensi(debouncedKampusQuery3),
      enabled:
        status === "authenticated" &&
        activeTab === "PT" &&
        Boolean(ptTarget) &&
        isKedinasanAcademicMode &&
        debouncedKampusQuery3.trim().length > 0,
      staleTime: 30_000,
    });

  const ptKampusOptionsByIndex = useMemo(
    () => [
      ptKampusOptions0,
      ptKampusOptions1,
      ptKampusOptions2,
      ptKampusOptions3,
    ],
    [ptKampusOptions0, ptKampusOptions1, ptKampusOptions2, ptKampusOptions3]
  );
  const ptKampusLoadingByIndex = useMemo(
    () => [
      isPtKampusLoading0,
      isPtKampusLoading1,
      isPtKampusLoading2,
      isPtKampusLoading3,
    ],
    [
      isPtKampusLoading0,
      isPtKampusLoading1,
      isPtKampusLoading2,
      isPtKampusLoading3,
    ]
  );

  const ptIsYearClosed = readBoolean(
    ptTarget?.year_closed ?? ptTarget?.yearClosed
  );
  const ptIsFinalized = Boolean(
    readString(ptTarget?.final_status ?? ptTarget?.finalStatus) ||
    readString(ptTarget?.finalized_at ?? ptTarget?.finalizedAt)
  );
  const ptLocked = ptIsYearClosed || ptIsFinalized;
  const canEditPt = !isReadOnly && !ptLocked;
  const showCancelPtPathButton =
    canEditPt &&
    !isPtLoadingDetail &&
    hasSavedDataForJalur(activePtJalur, ptDetail);

  const resetPtModalData = () => {
    setPtDetail(null);
    setPtDetailError("");
    setPtSaveError("");
    setActivePtJalur("SNBP");
    setPtDraft(emptyPtDraft());
  };

  const openPtDetail = async (
    row: PipelineRow,
    options?: { keepDefaultJalur?: boolean }
  ) => {
    setPtTarget(row);
    resetPtModalData();
    setPtSaveMessage("");
    setIsPtLoadingDetail(true);

    try {
      const params = new URLSearchParams();
      params.set("nisn", row.nisn);
      params.set("tahun_lulus", String(row.tahunLulus));
      const response = await apiFetch(
        `/api/siswa/applications/detail?${params.toString()}`,
        { cache: "no-store" }
      );
      if (!response.ok) {
        throw new Error("Gagal memuat detail.");
      }
      const payload = await response.json().catch(() => null);
      const rawData = (payload?.data ?? payload) as PtDetailResponse;
      const hasFilledPaths = Array.isArray(row.filled_paths);
      const filledJalur = new Set<PtJalurTab>(
        (row.filled_paths ?? [])
          .map((value) => normalizeJalurTab(readString(value)))
          .filter((value): value is PtJalurTab => value !== null)
      );
      const filteredChoices = Array.isArray(rawData?.choices)
        ? rawData.choices.filter((choice) => {
            const jalur = normalizeJalurTab(readString(choice.jalur));
            if (!jalur) return false;
            if (!hasFilledPaths) return true;
            return filledJalur.has(jalur);
          })
        : [];
      const filteredKedinasan =
        hasFilledPaths && !filledJalur.has("KEDINASAN")
          ? null
          : (rawData?.kedinasan ?? null);
      const data: PtDetailResponse = {
        ...rawData,
        choices: filteredChoices,
        kedinasan: filteredKedinasan,
      };
      setPtDetail(data);
      const choices = data.choices;

      const nextDraft = emptyPtDraft();
      const jalurWithData = new Set<PtJalurTab>();
      choices.forEach((choice) => {
        const jalurTab = normalizeJalurTab(readString(choice.jalur));
        if (!jalurTab) return;
        const index = readNumber(choice.pilihan_ke) - 1;
        if (index >= 0 && index < nextDraft.byJalur[jalurTab].length) {
          const kampusValue = readString(choice.kampus);
          const kampusID = readNumber(choice.kampus_id);
          const jenjang = readString(choice.jenjang_prodi).toUpperCase();
          nextDraft.byJalur[jalurTab][index] = {
            kampusId: kampusID > 0 ? kampusID : null,
            kampusQuery: kampusValue,
            legacyKampus: kampusID > 0 ? "" : kampusValue,
            prodi: readString(choice.prodi),
            jenjangProdi: jenjangProdiOptions.includes(jenjang)
              ? jenjang
              : "UNKNOWN",
          };
          jalurWithData.add(jalurTab);
        }
      });
      if (isRecord(data?.kedinasan)) {
        const jenisRaw = readString(data.kedinasan?.jenis).toUpperCase();
        const jenis = kedinasanJenisOptions.includes(jenisRaw as KedinasanJenis)
          ? (jenisRaw as KedinasanJenis)
          : "SEKOLAH_KEDINASAN";
        const matraRaw = readString(data.kedinasan?.matra).toUpperCase();
        const pendidikanRaw = readString(
          data.kedinasan?.jenis_pendidikan
        ).toUpperCase();
        nextDraft.kedinasan = {
          jenis,
          matra: kedinasanMatraOptions.includes(matraRaw as KedinasanMatra)
            ? (matraRaw as KedinasanMatra)
            : "",
          jenisPendidikan: kedinasanPendidikanOptions.includes(
            pendidikanRaw as KedinasanPendidikan
          )
            ? (pendidikanRaw as KedinasanPendidikan)
            : "",
        };
        if (jenis === "TNI" || jenis === "POLRI") {
          jalurWithData.add("KEDINASAN");
        }
      }
      setPtDraft(nextDraft);
      if (options?.keepDefaultJalur) {
        setActivePtJalur("SNBP");
      } else if (jalurWithData.size > 0) {
        setActivePtJalur(jalurWithData.values().next().value ?? "SNBP");
      } else {
        setActivePtJalur("SNBP");
      }
    } catch (error) {
      setPtDetail(null);
      setPtDetailError(
        error instanceof Error ? error.message : "Gagal memuat detail."
      );
    } finally {
      setIsPtLoadingDetail(false);
    }
  };

  const closePtDetail = () => {
    setPtTarget(null);
    setPtSaveMessage("");
    setIsPtCancellingPath(false);
    resetPtModalData();
  };

  const updatePtChoice = <K extends keyof PtDraftChoice>(
    jalur: PtJalurTab,
    index: number,
    field: K,
    value: PtDraftChoice[K]
  ) => {
    setPtDraft((prev) => {
      const nextByJalur = { ...prev.byJalur };
      const nextChoices = [...nextByJalur[jalur]];
      nextChoices[index] = { ...nextChoices[index], [field]: value };
      nextByJalur[jalur] = nextChoices;
      return { ...prev, byJalur: nextByJalur };
    });
  };

  const updateKedinasanDraft = <K extends keyof PtKedinasanDraft>(
    field: K,
    value: PtKedinasanDraft[K]
  ) => {
    setPtDraft((prev) => ({
      ...prev,
      kedinasan: { ...prev.kedinasan, [field]: value },
    }));
  };

  const handlePtSave = async () => {
    if (!ptTarget || !canEditPt) return;
    setIsPtSaving(true);
    setPtSaveMessage("");
    setPtSaveError("");

    const normalized: PtChoicePayload[] = [];
    const currentChoices =
      ptDraft.byJalur[activePtJalur] ?? createEmptyPtChoices();
    const isKedinasanSpecial =
      activePtJalur === "KEDINASAN" &&
      (ptDraft.kedinasan.jenis === "TNI" ||
        ptDraft.kedinasan.jenis === "POLRI");

    for (let i = 0; i < currentChoices.length; i += 1) {
      const choice = currentChoices[i];
      const kampus = choice.kampusQuery.trim();
      const prodi = choice.prodi.trim();
      const kampusId = choice.kampusId ?? 0;
      const hasAny = kampus !== "" || prodi !== "" || kampusId > 0;
      if (!hasAny) {
        continue;
      }
      if (isKedinasanSpecial) {
        setPtSaveError(
          "Mode Kedinasan TNI/POLRI tidak menggunakan kampus/prodi."
        );
        setIsPtSaving(false);
        return;
      }
      if (prodi && kampusId <= 0) {
        setPtSaveError("Pilih kampus dari daftar");
        setIsPtSaving(false);
        return;
      }
      if (!prodi || kampusId <= 0) {
        setPtSaveError("Kampus dan prodi wajib diisi berpasangan.");
        setIsPtSaving(false);
        return;
      }
      const jenjang = choice.jenjangProdi.trim().toUpperCase() || "UNKNOWN";
      if (!jenjangProdiOptions.includes(jenjang)) {
        setPtSaveError("Jenjang prodi tidak valid.");
        setIsPtSaving(false);
        return;
      }
      normalized.push({
        pilihan_ke: i + 1,
        jalur: activePtJalur,
        kampus_id: kampusId,
        kampus,
        prodi,
        jenjang_prodi: jenjang,
      });
    }

    const outOfRuleChoices = normalized.filter(
      (choice) => choice.pilihan_ke > activePtLimit
    );
    if (outOfRuleChoices.length > 0) {
      setPtSaveError(
        activePtLimitMessage ||
          `Jalur ${activePtJalur} maksimal ${activePtLimit} pilihan untuk tahun lulus ${ptTarget.tahunLulus}.`
      );
      setIsPtSaving(false);
      return;
    }
    if (normalized.length > SYSTEM_MAX_PT_CHOICES) {
      setPtSaveError(`Maksimal ${SYSTEM_MAX_PT_CHOICES} pilihan PT per siswa.`);
      setIsPtSaving(false);
      return;
    }
    if (activePtJalur === "KEDINASAN" && ptDraft.kedinasan.jenis === "TNI") {
      if (!ptDraft.kedinasan.matra || !ptDraft.kedinasan.jenisPendidikan) {
        setPtSaveError("Matra dan jenis pendidikan wajib diisi untuk TNI.");
        setIsPtSaving(false);
        return;
      }
    }
    if (activePtJalur === "KEDINASAN" && ptDraft.kedinasan.jenis === "POLRI") {
      if (!ptDraft.kedinasan.jenisPendidikan) {
        setPtSaveError("Jenis pendidikan wajib diisi untuk POLRI.");
        setIsPtSaving(false);
        return;
      }
    }

    try {
      const payload: Record<string, unknown> = {
        nisn: ptTarget.nisn,
        tahun_lulus: ptTarget.tahunLulus,
        jalur: activePtJalur,
        choices: normalized,
      };
      if (activePtJalur === "KEDINASAN") {
        payload.kedinasan = {
          jenis: ptDraft.kedinasan.jenis,
          matra: ptDraft.kedinasan.matra,
          jenis_pendidikan: ptDraft.kedinasan.jenisPendidikan,
        };
      }
      const response = await apiFetch("/api/siswa/applications", {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(
          parseApiErrorMessage(payload, "Gagal menyimpan pendaftaran.")
        );
      }
      setPtSaveMessage("Pendaftaran PT berhasil disimpan.");
      void refetchPt();
      void queryClient.invalidateQueries({
        queryKey: ["siswa-cohorts-list"],
      });
    } catch (error) {
      setPtSaveError(
        error instanceof Error ? error.message : "Gagal menyimpan pendaftaran."
      );
    } finally {
      setIsPtSaving(false);
    }
  };

  const applyCancelledPtPathToPipelineCache = (
    nisn: string,
    tahunLulus: number,
    jalurToCancel: PtJalurTab
  ) => {
    queryClient.setQueriesData<PipelineListResponse>(
      { queryKey: ["pt-pipeline"] },
      (current) => {
        if (!current) return current;
        let hasChanges = false;
        const nextRows = current.data.map((row) => {
          if (row.nisn !== nisn || row.tahunLulus !== tahunLulus) {
            return row;
          }
          const currentFilledPaths = Array.isArray(row.filled_paths)
            ? row.filled_paths
            : [];
          const nextFilledPaths = currentFilledPaths.filter((path) => {
            const normalized = normalizeJalurTab(readString(path));
            return normalized !== jalurToCancel;
          });
          hasChanges = true;
          return {
            ...row,
            filled_paths: nextFilledPaths,
            status: nextFilledPaths.length === 0 ? "Belum Diisi" : row.status,
          };
        });
        if (!hasChanges) {
          return current;
        }
        return {
          ...current,
          data: nextRows,
        };
      }
    );
  };

  const refreshPtPipelineAndSummary = async (options?: {
    skipRouterRefresh?: boolean;
  }) => {
    await refetchPt();
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: ["pt-pipeline"],
        refetchType: "active",
      }),
      queryClient.invalidateQueries({
        queryKey: ["siswa-cohorts-list"],
      }),
      queryClient.invalidateQueries({
        queryKey: ["siswa-status-pending-school"],
      }),
      queryClient.invalidateQueries({
        queryKey: ["siswa-status-pending-summary"],
      }),
      queryClient.invalidateQueries({
        queryKey: ["admission-validation-summary"],
      }),
    ]);
    if (!options?.skipRouterRefresh) {
      router.refresh();
    }
  };

  const handleCancelPtByJalur = async () => {
    if (!ptTarget || !showCancelPtPathButton) return;
    const target = ptTarget;
    const jalurToCancel = activePtJalur;
    if (
      !window.confirm(
        `Batalkan pendaftaran jalur ${jalurToCancel} untuk siswa ini?`
      )
    ) {
      return;
    }
    if (currentSchoolId <= 0) {
      setPtSaveError("school_id tidak ditemukan. Silakan login ulang.");
      return;
    }

    setIsPtCancellingPath(true);
    setPtSaveMessage("");
    setPtSaveError("");
    try {
      const response = await apiFetch("/api/siswa/applications/cancel-path", {
        method: "POST",
        body: JSON.stringify({
          school_id: currentSchoolId,
          nisn: target.nisn,
          tahun_lulus: target.tahunLulus,
          jalur: jalurToCancel,
        }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        const errorMessage = parseApiErrorMessage(
          payload,
          "Gagal membatalkan pendaftaran."
        );
        throw new Error(
          normalizeYearClosedMessage(errorMessage, target.tahunLulus)
        );
      }

      const refreshedTarget = withoutJalurInFilledPaths(target, jalurToCancel);
      applyCancelledPtPathToPipelineCache(
        refreshedTarget.nisn,
        refreshedTarget.tahunLulus,
        jalurToCancel
      );
      resetPtModalData();
      setPtModalKey((prev) => prev + 1);
      await openPtDetail(refreshedTarget, { keepDefaultJalur: true });
      setPtSaveMessage(
        `Pendaftaran jalur ${jalurToCancel} berhasil dibatalkan.`
      );
      await refreshPtPipelineAndSummary({ skipRouterRefresh: true });
    } catch (error) {
      setPtSaveError(
        error instanceof Error
          ? error.message
          : "Gagal membatalkan pendaftaran."
      );
    } finally {
      setIsPtCancellingPath(false);
    }
  };

  const handleCancelPtPathShortcut = async (
    row: PipelineRow,
    jalur: PtJalurTab
  ) => {
    if (isReadOnly) return;
    if (!row.nisn || !row.tahunLulus) return;
    if (currentSchoolId <= 0) {
      alert("school_id tidak ditemukan. Silakan login ulang.");
      return;
    }
    if (
      !window.confirm(`Batalkan pendaftaran jalur ${jalur} untuk siswa ini?`)
    ) {
      return;
    }

    const key = `${row.nisn}-${row.tahunLulus}-${jalur}`;
    setPtShortcutCancellingKey(key);
    try {
      const response = await apiFetch("/api/siswa/applications/cancel-path", {
        method: "POST",
        body: JSON.stringify({
          school_id: currentSchoolId,
          nisn: row.nisn,
          tahun_lulus: row.tahunLulus,
          jalur,
        }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        const errorMessage = parseApiErrorMessage(
          payload,
          "Gagal membatalkan pendaftaran."
        );
        throw new Error(
          normalizeYearClosedMessage(errorMessage, row.tahunLulus)
        );
      }

      applyCancelledPtPathToPipelineCache(row.nisn, row.tahunLulus, jalur);
      alert(`Pendaftaran jalur ${jalur} berhasil dibatalkan.`);
      await refreshPtPipelineAndSummary();
      if (
        ptTarget &&
        ptTarget.nisn === row.nisn &&
        ptTarget.tahunLulus === row.tahunLulus
      ) {
        closePtDetail();
      }
    } catch (error) {
      alert(
        error instanceof Error
          ? error.message
          : "Gagal membatalkan pendaftaran."
      );
    } finally {
      setPtShortcutCancellingKey("");
    }
  };

  const [wirausahaYear, setWirausahaYear] = useState(() =>
    new Date().getFullYear()
  );
  const [wirausahaCountInput, setWirausahaCountInput] = useState("");
  const [wirausahaCountMessage, setWirausahaCountMessage] = useState("");
  const currentYear = new Date().getFullYear();
  const isWirausahaYearLocked = wirausahaYear < currentYear;

  const wirausahaCountQuery = useQuery({
    queryKey: [
      "rencana-wirausaha-plan-counts",
      wirausahaYear,
      showSchoolFilter ? (selectedSchoolId ?? "ALL") : "SCHOOL_ADMIN",
    ],
    queryFn: () =>
      fetchEntrepreneurPlanSummaries({
        year: wirausahaYear,
        schoolId: showSchoolFilter ? selectedSchoolId : null,
      }),
    enabled: status === "authenticated" && activeTab === "WIRAUSAHA",
  });
  const wirausahaCountFirst = wirausahaCountQuery.data?.[0] ?? null;

  useEffect(() => {
    if (!isSchoolAdmin || wirausahaCountQuery.isLoading) return;
    setWirausahaCountInput(
      wirausahaCountFirst ? String(wirausahaCountFirst.plannedCount ?? 0) : ""
    );
  }, [
    isSchoolAdmin,
    wirausahaCountFirst,
    wirausahaCountQuery.isLoading,
    wirausahaYear,
  ]);

  const wirausahaCountMutation = useMutation({
    mutationFn: async () => {
      const plannedCount = Number(wirausahaCountInput);
      if (
        !Number.isInteger(plannedCount) ||
        plannedCount < 0 ||
        wirausahaCountInput.trim() === ""
      ) {
        throw new Error(
          "Jumlah rencana wirausaha wajib berupa bilangan bulat 0 atau lebih."
        );
      }
      if (isWirausahaYearLocked) {
        throw new Error(
          `Tahun lulus ${wirausahaYear} sudah terkunci dan tidak dapat diubah.`
        );
      }
      const response = await apiFetch("/api/siswa/wirausaha-plan-counts", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          tahun_lulus: wirausahaYear,
          planned_count: plannedCount,
        }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(
          parseApiErrorMessage(
            payload,
            "Gagal menyimpan rekap rencana wirausaha."
          )
        );
      }
    },
    onSuccess: () => {
      setWirausahaCountMessage("Rekap rencana wirausaha tersimpan.");
      void wirausahaCountQuery.refetch();
    },
  });

  const openEligibleModal = () => {
    if (!canEditEligibleSNBP) return;
    setEligibleCountInput(
      hasEligibleSNBP ? String(eligibleSNBP?.eligibleCount ?? 0) : ""
    );
    setEligibleNoteInput(eligibleSNBP?.note ?? "");
    setEligibleError("");
    setEligibleMessage("");
    setEligibleModalOpen(true);
  };

  const closeEligibleModal = () => {
    if (isEligibleSaving) return;
    setEligibleModalOpen(false);
    setEligibleError("");
  };

  const handleEligibleSave = async () => {
    if (!canEditEligibleSNBP) return;
    const parsedCount = Number(eligibleCountInput);
    if (!Number.isFinite(parsedCount) || parsedCount < 0) {
      setEligibleError("Jumlah Eligible SNBP wajib angka minimal 0.");
      return;
    }
    setIsEligibleSaving(true);
    setEligibleError("");
    setEligibleMessage("");
    try {
      const payload: Record<string, unknown> = {
        tahun_lulus: ptYear,
        eligible_count: parsedCount,
        note: eligibleNoteInput,
      };
      if (!isSchoolAdmin && eligibleSchoolId != null) {
        payload.school_id = eligibleSchoolId;
      }
      const response = await apiFetch("/api/siswa/eligible-snbp", {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      const responsePayload = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error(
          parseApiErrorMessage(
            responsePayload,
            "Gagal menyimpan Data Eligible SNBP."
          )
        );
      }
      setEligibleModalOpen(false);
      setEligibleMessage("Data Eligible SNBP berhasil disimpan.");
      await refetchEligibleSNBP();
    } catch (error) {
      setEligibleError(
        error instanceof Error
          ? error.message
          : "Gagal menyimpan Data Eligible SNBP."
      );
    } finally {
      setIsEligibleSaving(false);
    }
  };

  return (
    <AdminRouteGuard>
      <section className="space-y-6">
        <header className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Siswa
          </p>
          <h1 className="text-3xl font-semibold text-zinc-900">
            Rencana Lulusan
          </h1>
          <p className="text-sm text-zinc-600">
            Pantau dan lengkapi rencana lulusan siswa kelas akhir.
          </p>
        </header>

        <RencanaTabs
          value={activeTab}
          onChange={handleTabChange}
          tabs={tabItems}
        />

        {activeTab === "PT" ? (
          <>
            <Card>
              <CardContent className="flex flex-col gap-3 md:flex-row md:flex-wrap md:items-end">
                {showSchoolFilter ? (
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Sekolah
                    </label>
                    <TypeaheadInput
                      value={schoolQuery}
                      onValueChange={(value) => {
                        setSchoolQuery(value);
                        setSelectedSchoolId(null);
                        setPtPage(1);
                      }}
                      items={schoolSuggestionItems}
                      onSelect={(item) => {
                        if (item.key === "ALL") {
                          setSelectedSchoolId(null);
                          setSchoolQuery("");
                          setPtPage(1);
                          return;
                        }
                        const selected = scopedSchools.find(
                          (school) => school.id === item.key
                        );
                        if (!selected) return;
                        setSelectedSchoolId(selected.id);
                        setSchoolQuery(selected.namaSekolah);
                        setPtPage(1);
                      }}
                      placeholder="Cari sekolah..."
                      emptyText="Sekolah tidak ditemukan"
                      minChars={0}
                      showClear={Boolean(selectedSchoolId || schoolQuery)}
                      onClear={() => {
                        setSelectedSchoolId(null);
                        setSchoolQuery("");
                        setPtPage(1);
                      }}
                    />
                  </div>
                ) : null}
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Tahun Lulus
                  </label>
                  <input
                    type="number"
                    min={2000}
                    value={ptYear}
                    onChange={(event) => {
                      const nextYear = Number(event.target.value);
                      if (Number.isFinite(nextYear) && nextYear > 0) {
                        setPtYear(nextYear);
                        setPtPage(1);
                        setEligibleMessage("");
                        setEligibleError("");
                      }
                    }}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-[140px]"
                  />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="space-y-4 p-4">
                <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                  <div>
                    <h2 className="text-lg font-semibold text-zinc-900">
                      Rekap Pendaftar PT
                    </h2>
                    <p className="text-sm text-zinc-600">
                      Isi jumlah pendaftar per jalur. Label SNBM/Mandiri tetap
                      disimpan sebagai MANDIRI secara internal.
                    </p>
                  </div>
                  {ptRegistrationFirst ? (
                    <div className="text-sm text-zinc-600 md:text-right">
                      <p>
                        Jumlah siswa akhir:{" "}
                        <span className="font-semibold text-zinc-900">
                          {ptRegistrationFirst.totalSiswaAkhir.toLocaleString(
                            "id-ID"
                          )}
                        </span>
                      </p>
                      <p>
                        Eligible SNBP:{" "}
                        <span className="font-semibold text-zinc-900">
                          {ptRegistrationFirst.eligibleSNBP ?? "-"}
                        </span>
                      </p>
                    </div>
                  ) : null}
                </div>
                {ptRegistrationFirst?.autoSummaryNotes ? (
                  <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                    {ptRegistrationFirst.autoSummaryNotes}
                  </p>
                ) : null}
                {ptRegistrationFirst?.validationMessage ? (
                  <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                    {ptRegistrationFirst.validationMessage}
                  </p>
                ) : null}
                {isSchoolAdmin ? (
                  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
                    {[
                      ["SNBP", "Pendaftar SNBP"],
                      ["SNBT", "Pendaftar SNBT"],
                      ["MANDIRI", "Pendaftar SNBM/Mandiri"],
                      ["KEDINASAN", "Pendaftar Kedinasan"],
                      ["LAINNYA", "Pendaftar Lainnya"],
                    ].map(([key, label]) => (
                      <label
                        key={key}
                        className="space-y-1 text-sm font-medium text-zinc-700"
                      >
                        <span>{label}</span>
                        <input
                          type="number"
                          min={0}
                          value={ptCountInputs[key] ?? ""}
                          onChange={(event) => {
                            setPtCountInputs((prev) => ({
                              ...prev,
                              [key]: event.target.value,
                            }));
                            setPtCountMessage("");
                          }}
                          className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm"
                        />
                      </label>
                    ))}
                  </div>
                ) : (
                  <DataTable<PTRegistrationSummary>
                    columns={[
                      ...(showSchoolColumn
                        ? [{ key: "school", label: "Sekolah" }]
                        : []),
                      { key: "tahun", label: "Tahun", align: "right" as const },
                      { key: "snbp", label: "SNBP", align: "right" as const },
                      { key: "snbt", label: "SNBT", align: "right" as const },
                      {
                        key: "mandiri",
                        label: "SNBM/Mandiri",
                        align: "right" as const,
                      },
                      {
                        key: "kedinasan",
                        label: "Kedinasan",
                        align: "right" as const,
                      },
                      {
                        key: "lainnya",
                        label: "Lainnya",
                        align: "right" as const,
                      },
                    ]}
                    rows={ptRegistrationQuery.data ?? []}
                    isLoading={ptRegistrationQuery.isLoading}
                    isError={ptRegistrationQuery.isError}
                    emptyMessage="Belum ada rekap pendaftar PT."
                    page={1}
                    pageSize={ptRegistrationQuery.data?.length || 1}
                    total={ptRegistrationQuery.data?.length ?? 0}
                    onPageChange={() => undefined}
                    renderRow={(row) => (
                      <TableRow key={`${row.schoolId}-${row.tahunLulus}`}>
                        {showSchoolColumn ? (
                          <TableCell>{row.schoolName || "-"}</TableCell>
                        ) : null}
                        <TableCell className="text-right">
                          {row.tahunLulus}
                        </TableCell>
                        <TableCell className="text-right">
                          {row.counts.SNBP.registeredCount}
                        </TableCell>
                        <TableCell className="text-right">
                          {row.counts.SNBT.registeredCount}
                        </TableCell>
                        <TableCell className="text-right">
                          {row.counts.MANDIRI.registeredCount}
                        </TableCell>
                        <TableCell className="text-right">
                          {row.counts.KEDINASAN.registeredCount}
                        </TableCell>
                        <TableCell className="text-right">
                          {row.counts.LAINNYA.registeredCount}
                        </TableCell>
                      </TableRow>
                    )}
                  />
                )}
                {ptCountMutation.error ? (
                  <p className="text-sm text-red-600">
                    {ptCountMutation.error.message}
                  </p>
                ) : null}
                {ptCountMessage ? (
                  <p className="text-sm text-emerald-700">{ptCountMessage}</p>
                ) : null}
                {isSchoolAdmin ? (
                  <div className="flex justify-end">
                    <button
                      type="button"
                      onClick={() => ptCountMutation.mutate()}
                      disabled={ptCountMutation.isPending}
                      className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                    >
                      {ptCountMutation.isPending
                        ? "Menyimpan..."
                        : "Simpan Rekap PT"}
                    </button>
                  </div>
                ) : null}
              </CardContent>
            </Card>

            <Card>
              <CardContent className="space-y-4 p-4">
                <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                  <div className="space-y-2">
                    <div>
                      <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Pendukung Analisis SNBP
                      </p>
                      <h2 className="text-lg font-semibold text-zinc-900">
                        Data Eligible SNBP
                      </h2>
                    </div>
                    {showSchoolFilter && selectedSchoolId == null ? (
                      <p className="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-700">
                        Pilih sekolah terlebih dahulu untuk melihat Data
                        Eligible SNBP.
                      </p>
                    ) : isEligibleLoading ? (
                      <p className="text-sm text-zinc-500">
                        Memuat Data Eligible SNBP...
                      </p>
                    ) : isEligibleError ? (
                      <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                        Gagal memuat Data Eligible SNBP.
                      </p>
                    ) : hasEligibleSNBP ? (
                      <div className="grid gap-3 text-sm text-zinc-700 sm:grid-cols-2 lg:grid-cols-4">
                        <div>
                          <p className="text-xs font-semibold text-zinc-500">
                            Tahun lulus
                          </p>
                          <p className="font-semibold text-zinc-900">
                            {eligibleSNBP?.tahunLulus || ptYear}
                          </p>
                        </div>
                        <div>
                          <p className="text-xs font-semibold text-zinc-500">
                            Jumlah Eligible SNBP
                          </p>
                          <p className="font-semibold text-zinc-900">
                            {new Intl.NumberFormat("id-ID").format(
                              eligibleSNBP?.eligibleCount ?? 0
                            )}
                          </p>
                        </div>
                        <div>
                          <p className="text-xs font-semibold text-zinc-500">
                            Status
                          </p>
                          <p className="font-semibold text-emerald-700">
                            Sudah diisi
                          </p>
                        </div>
                        <div>
                          <p className="text-xs font-semibold text-zinc-500">
                            Catatan
                          </p>
                          <p className="font-semibold text-zinc-900">
                            {eligibleSNBP?.note || "-"}
                          </p>
                        </div>
                      </div>
                    ) : (
                      <div className="space-y-2">
                        <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                          Data Eligible SNBP tahun {ptYear} belum diisi.
                        </p>
                        <p className="text-sm text-zinc-600">
                          Mohon lengkapi data eligible agar analisis SNBP valid.
                        </p>
                      </div>
                    )}
                    {eligibleMessage ? (
                      <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                        {eligibleMessage}
                      </p>
                    ) : null}
                    {eligibleSNBPWarning ? (
                      <p className="text-xs text-amber-700">
                        {eligibleSNBPWarning}
                      </p>
                    ) : null}
                  </div>
                  {canEditEligibleSNBP ? (
                    <button
                      type="button"
                      onClick={openEligibleModal}
                      className="w-full rounded-lg bg-zinc-900 px-4 py-2 text-sm font-semibold text-white transition hover:bg-zinc-800 md:w-auto"
                    >
                      {hasEligibleSNBP
                        ? "Ubah Data Eligible SNBP"
                        : "Isi Data Eligible SNBP"}
                    </button>
                  ) : null}
                </div>
              </CardContent>
            </Card>

            {false ? (
              <>
                <SiswaModuleActions
                  title="Data Rinci Pendaftaran PT Lama"
                  templateUrl="/api/siswa/applications/template.xlsx"
                  importUrl="/api/siswa/applications/import"
                  accept=".xlsx,.xlsm"
                  isSchoolAdmin={isSchoolAdmin}
                  onSuccess={() => void refetchPt()}
                />

                <Card>
                  <CardContent className="space-y-4">
                    <div>
                      <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Daftar Pendaftaran PT
                      </p>
                      <h2 className="text-lg font-semibold text-zinc-900">
                        Status pendaftaran perguruan tinggi
                      </h2>
                    </div>

                    <DataTable
                      columns={ptColumns}
                      rows={ptRows}
                      isLoading={isPtLoading}
                      isError={isPtError}
                      emptyMessage="Belum ada data siswa kelas akhir."
                      tableWrapperClassName="max-h-[420px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
                      tableHeaderClassName="sticky top-0 z-10 bg-white"
                      page={ptMeta.page}
                      pageSize={ptMeta.pageSize}
                      total={ptMeta.total}
                      onPageChange={setPtPage}
                      renderRow={(row) => {
                        const rowYearClosed = readBoolean(
                          row.year_closed ?? row.yearClosed
                        );
                        const rowFinalized = Boolean(
                          readString(row.final_status ?? row.finalStatus) ||
                          readString(row.finalized_at ?? row.finalizedAt)
                        );
                        const rowLocked = rowYearClosed || rowFinalized;
                        const canOpen = isReadOnly || !rowLocked;
                        const manageLabel = isReadOnly ? "Detail" : "Kelola";
                        const filled = row.filled_paths || [];
                        const filledJalur = filled
                          .map((jalur) => normalizeJalurTab(readString(jalur)))
                          .filter(
                            (jalur): jalur is PtJalurTab => jalur !== null
                          );
                        const canShowPathShortcuts =
                          !isReadOnly &&
                          !rowFinalized &&
                          filledJalur.length > 0;
                        const rowActionItems: RowActionItem[] = [
                          {
                            key: "manage",
                            label: manageLabel,
                            onSelect: () => openPtDetail(row),
                            disabled: !canOpen,
                          },
                        ];
                        if (canShowPathShortcuts) {
                          rowActionItems.push({
                            key: "divider-cancel",
                            label: "----------------",
                            onSelect: () => undefined,
                            disabled: true,
                          });
                          filledJalur.forEach((jalur) => {
                            const isCancellingThisJalur =
                              ptShortcutCancellingKey ===
                              `${row.nisn}-${row.tahunLulus}-${jalur}`;
                            rowActionItems.push({
                              key: `cancel-${jalur}`,
                              label: isCancellingThisJalur
                                ? `Membatalkan ${jalur}...`
                                : `Batalkan ${jalur}`,
                              onSelect: () =>
                                handleCancelPtPathShortcut(row, jalur),
                              destructive: true,
                              disabled: isCancellingThisJalur,
                            });
                          });
                        }

                        return (
                          <TableRow key={row.id}>
                            {showSchoolColumn ? (
                              <TableCell>
                                {readString(
                                  row.schoolName ?? row.school_name
                                ) ||
                                  schoolNameById.get(
                                    readNumber(row.schoolId ?? row.school_id)
                                  ) ||
                                  "-"}
                              </TableCell>
                            ) : null}
                            <TableCell>{row.nisn || "-"}</TableCell>
                            <TableCell>{row.namaSiswa || "-"}</TableCell>
                            <TableCell>{row.rombel || "-"}</TableCell>
                            <TableCell className="text-right">
                              {row.tahunLulus || "-"}
                            </TableCell>
                            <TableCell>{row.status || "-"}</TableCell>
                            <TableCell className="text-center">
                              <div className="flex items-center justify-center">
                                <RowActionDropdown
                                  label="Aksi"
                                  items={rowActionItems}
                                />
                              </div>
                            </TableCell>
                          </TableRow>
                        );
                      }}
                    />
                    {ptFilteredCount > 0 ? (
                      <p className="text-xs text-zinc-500">
                        Data final disembunyikan dari pipeline.
                      </p>
                    ) : null}
                  </CardContent>
                </Card>
              </>
            ) : null}
          </>
        ) : null}

        {activeTab === "WIRAUSAHA" ? (
          <>
            <Card>
              <CardContent className="flex flex-col gap-3 md:flex-row md:flex-wrap md:items-end">
                {showSchoolFilter ? (
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Sekolah
                    </label>
                    <TypeaheadInput
                      value={schoolQuery}
                      onValueChange={(value) => {
                        setSchoolQuery(value);
                        setSelectedSchoolId(null);
                        setWirausahaCountMessage("");
                      }}
                      items={schoolSuggestionItems}
                      onSelect={(item) => {
                        if (item.key === "ALL") {
                          setSelectedSchoolId(null);
                          setSchoolQuery("");
                          setWirausahaCountMessage("");
                          return;
                        }
                        const selected = scopedSchools.find(
                          (school) => school.id === item.key
                        );
                        if (!selected) return;
                        setSelectedSchoolId(selected.id);
                        setSchoolQuery(selected.namaSekolah);
                        setWirausahaCountMessage("");
                      }}
                      placeholder="Cari sekolah..."
                      emptyText="Sekolah tidak ditemukan"
                      minChars={0}
                      showClear={Boolean(selectedSchoolId || schoolQuery)}
                      onClear={() => {
                        setSelectedSchoolId(null);
                        setSchoolQuery("");
                        setWirausahaCountMessage("");
                      }}
                    />
                  </div>
                ) : null}
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Tahun Lulus
                  </label>
                  <input
                    type="number"
                    min={2000}
                    value={wirausahaYear}
                    onChange={(event) => {
                      const nextYear = Number(event.target.value);
                      if (Number.isFinite(nextYear) && nextYear > 0) {
                        setWirausahaYear(nextYear);
                        setWirausahaCountMessage("");
                      }
                    }}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-[140px]"
                  />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="space-y-4 p-4">
                <div>
                  <h2 className="text-lg font-semibold text-zinc-900">
                    Rekap Rencana Wirausaha
                  </h2>
                  <p className="text-sm text-zinc-600">
                    Tahap rencana cukup isi jumlah siswa yang berencana
                    wirausaha.
                  </p>
                </div>
                {wirausahaCountFirst?.autoSummaryNotes ? (
                  <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                    {wirausahaCountFirst.autoSummaryNotes}
                  </p>
                ) : null}
                {isSchoolAdmin && isWirausahaYearLocked ? (
                  <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                    Tahun lulus {wirausahaYear} sudah terkunci. Rekap hanya
                    dapat diubah selama tahun lulus masih berjalan.
                  </p>
                ) : null}
                {isSchoolAdmin ? (
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
                    <label className="space-y-1 text-sm font-medium text-zinc-700">
                      <span>Jumlah Rencana Wirausaha</span>
                      <input
                        type="number"
                        min={0}
                        step={1}
                        value={wirausahaCountInput}
                        onChange={(event) => {
                          setWirausahaCountInput(event.target.value);
                          setWirausahaCountMessage("");
                        }}
                        disabled={isWirausahaYearLocked}
                        className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm disabled:bg-zinc-100 sm:w-56"
                      />
                    </label>
                    <button
                      type="button"
                      onClick={() => wirausahaCountMutation.mutate()}
                      disabled={
                        wirausahaCountMutation.isPending ||
                        isWirausahaYearLocked
                      }
                      className="h-10 rounded-md bg-zinc-900 px-4 text-sm font-semibold text-white disabled:opacity-60"
                    >
                      {wirausahaCountMutation.isPending
                        ? "Menyimpan..."
                        : "Simpan Rekap"}
                    </button>
                  </div>
                ) : (
                  <DataTable<EntrepreneurPlanSummary>
                    columns={[
                      ...(showSchoolColumn
                        ? [{ key: "school", label: "Sekolah" }]
                        : []),
                      { key: "tahun", label: "Tahun", align: "right" as const },
                      {
                        key: "jumlah",
                        label: "Jumlah Rencana",
                        align: "right" as const,
                      },
                      { key: "source", label: "Sumber" },
                    ]}
                    rows={wirausahaCountQuery.data ?? []}
                    isLoading={wirausahaCountQuery.isLoading}
                    isError={wirausahaCountQuery.isError}
                    emptyMessage="Belum ada rekap rencana wirausaha."
                    page={1}
                    pageSize={wirausahaCountQuery.data?.length || 1}
                    total={wirausahaCountQuery.data?.length ?? 0}
                    onPageChange={() => undefined}
                    renderRow={(row) => (
                      <TableRow key={`${row.schoolId}-${row.tahunLulus}`}>
                        {showSchoolColumn ? (
                          <TableCell>{row.schoolName || "-"}</TableCell>
                        ) : null}
                        <TableCell className="text-right">
                          {row.tahunLulus}
                        </TableCell>
                        <TableCell className="text-right">
                          {row.plannedCount}
                        </TableCell>
                        <TableCell>
                          {row.sourceLabel || row.source || "Manual"}
                        </TableCell>
                      </TableRow>
                    )}
                  />
                )}
                {wirausahaCountMutation.error ? (
                  <p className="text-sm text-red-600">
                    {wirausahaCountMutation.error.message}
                  </p>
                ) : null}
                {wirausahaCountMessage ? (
                  <p className="text-sm text-emerald-700">
                    {wirausahaCountMessage}
                  </p>
                ) : null}
              </CardContent>
            </Card>
          </>
        ) : null}

        {activeTab === "UKK" && hasUkkAccess ? (
          <UkkPanel
            schoolId={showSchoolFilter ? selectedSchoolId : null}
            hasSMKScope={hasResolvedSMKScope}
            emptyStateMessage={ukkEmptyStateMessage}
          />
        ) : null}
      </section>

      {eligibleModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-lg rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                {hasEligibleSNBP
                  ? "Ubah Data Eligible SNBP"
                  : "Isi Data Eligible SNBP"}
              </h2>
              <p className="text-sm text-zinc-600">
                Data mengikuti tahun lulus aktif pada tab Pendaftaran PT.
              </p>
            </div>
            <div className="mt-4 space-y-4">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tahun lulus
                </label>
                <input
                  type="number"
                  value={ptYear}
                  readOnly
                  className="w-full rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Jumlah Eligible SNBP
                </label>
                <input
                  type="number"
                  min={0}
                  value={eligibleCountInput}
                  onChange={(event) =>
                    setEligibleCountInput(event.target.value)
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Catatan
                </label>
                <textarea
                  value={eligibleNoteInput}
                  onChange={(event) => setEligibleNoteInput(event.target.value)}
                  rows={3}
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              {eligibleError ? (
                <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                  {eligibleError}
                </p>
              ) : null}
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={closeEligibleModal}
                disabled={isEligibleSaving}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleEligibleSave}
                disabled={isEligibleSaving}
                className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-semibold text-white transition hover:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isEligibleSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {ptTarget ? (
        <div
          key={`pt-modal-${ptModalKey}`}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
        >
          <div className="w-full max-w-4xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Pendaftaran Perguruan Tinggi
              </h2>
              <p className="text-sm text-zinc-600">
                {ptTarget.namaSiswa || "-"} - {ptTarget.nisn || "-"} -{" "}
                {ptTarget.rombel || "-"}
              </p>
            </div>

            {isPtLoadingDetail ? (
              <div className="mt-4 rounded-md border border-dashed border-zinc-200 p-4 text-sm text-zinc-500">
                Memuat detail...
              </div>
            ) : ptDetailError ? (
              <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-600">
                {ptDetailError}
              </div>
            ) : (
              <div className="mt-4 space-y-6">
                {ptLocked ? (
                  <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
                    {ptIsFinalized
                      ? "Data sudah final dan terkunci."
                      : "Tahun sudah tutup buku. Hanya finalisasi yang diperbolehkan."}
                  </div>
                ) : !isReadOnly ? null : (
                  <div className="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-700">
                    Mode baca saja. Hanya admin sekolah yang dapat memperbarui
                    pendaftaran PT.
                  </div>
                )}

                <div className="space-y-4 rounded-lg border border-zinc-200 p-4">
                  <div className="space-y-2">
                    <h3 className="text-sm font-semibold text-zinc-900">
                      Jalur Pendaftaran PT
                    </h3>
                    <div className="flex flex-wrap gap-2">
                      {ptJalurTabs.map((tab) => {
                        const isActive = activePtJalur === tab.value;
                        return (
                          <button
                            key={tab.value}
                            type="button"
                            onClick={() => {
                              setActivePtJalur(tab.value);
                              setPtSaveMessage("");
                              setPtSaveError("");
                            }}
                            className={`rounded-md border px-3 py-1.5 text-xs font-semibold transition ${
                              isActive
                                ? "border-zinc-900 bg-zinc-900 text-white"
                                : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50"
                            }`}
                          >
                            {tab.label}
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  {activePtJalur === "KEDINASAN" ? (
                    <div className="space-y-3 rounded-md border border-zinc-200 p-3">
                      <p className="text-xs text-zinc-600">
                        Kedinasan tidak selalu menggunakan prodi. Pilih jenis
                        kedinasan terlebih dahulu.
                      </p>
                      <div className="grid gap-3 md:grid-cols-3">
                        <div className="space-y-1">
                          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                            Jenis Kedinasan
                          </label>
                          <select
                            value={ptDraft.kedinasan.jenis}
                            onChange={(event) =>
                              updateKedinasanDraft(
                                "jenis",
                                event.target.value as KedinasanJenis
                              )
                            }
                            disabled={!canEditPt}
                            className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                          >
                            {kedinasanJenisOptions.map((option) => (
                              <option key={option} value={option}>
                                {option}
                              </option>
                            ))}
                          </select>
                        </div>

                        {ptDraft.kedinasan.jenis === "TNI" ? (
                          <div className="space-y-1">
                            <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                              Matra
                            </label>
                            <select
                              value={ptDraft.kedinasan.matra}
                              onChange={(event) =>
                                updateKedinasanDraft(
                                  "matra",
                                  event.target.value as KedinasanMatra
                                )
                              }
                              disabled={!canEditPt}
                              className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                            >
                              <option value="">Pilih matra</option>
                              {kedinasanMatraOptions.map((option) => (
                                <option key={option} value={option}>
                                  {option}
                                </option>
                              ))}
                            </select>
                          </div>
                        ) : null}

                        {ptDraft.kedinasan.jenis === "TNI" ||
                        ptDraft.kedinasan.jenis === "POLRI" ? (
                          <div className="space-y-1">
                            <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                              Jenis Pendidikan
                            </label>
                            <select
                              value={ptDraft.kedinasan.jenisPendidikan}
                              onChange={(event) =>
                                updateKedinasanDraft(
                                  "jenisPendidikan",
                                  event.target.value as KedinasanPendidikan
                                )
                              }
                              disabled={!canEditPt}
                              className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                            >
                              <option value="">Pilih jenis pendidikan</option>
                              {kedinasanPendidikanOptions.map((option) => (
                                <option key={option} value={option}>
                                  {option}
                                </option>
                              ))}
                            </select>
                          </div>
                        ) : null}
                      </div>
                    </div>
                  ) : null}

                  {isKedinasanAcademicMode ? (
                    <>
                      {activePtLimitMessage ? (
                        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                          {activePtLimitMessage}
                        </div>
                      ) : null}
                      <div className="space-y-3">
                        <h4 className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Kampus
                        </h4>
                        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                          {activePtChoices.map((choice, index) => {
                            const kampusOptions =
                              ptKampusOptionsByIndex[index] ?? [];
                            const isKampusLoading =
                              ptKampusLoadingByIndex[index] ?? false;
                            const slotHasValue = Boolean(
                              choice.kampusId ||
                              choice.kampusQuery.trim() ||
                              choice.legacyKampus.trim() ||
                              choice.prodi.trim()
                            );
                            const slotDisabledByRule =
                              index + 1 > activePtLimit && !slotHasValue;
                            return (
                              <div
                                key={`pt-choice-kampus-${index}`}
                                className="space-y-1"
                              >
                                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                                  Pilihan {index + 1} - Kampus
                                </label>
                                <TypeaheadInput
                                  value={choice.kampusQuery}
                                  onValueChange={(value) => {
                                    updatePtChoice(
                                      activePtJalur,
                                      index,
                                      "kampusQuery",
                                      value
                                    );
                                    updatePtChoice(
                                      activePtJalur,
                                      index,
                                      "kampusId",
                                      null
                                    );
                                  }}
                                  items={kampusOptions.map((item) => ({
                                    key: item.id,
                                    label: item.nama,
                                    value: item.nama,
                                  }))}
                                  onSelect={(item) => {
                                    const id = readNumber(item.key);
                                    updatePtChoice(
                                      activePtJalur,
                                      index,
                                      "kampusId",
                                      id > 0 ? id : null
                                    );
                                    updatePtChoice(
                                      activePtJalur,
                                      index,
                                      "kampusQuery",
                                      item.value
                                    );
                                    updatePtChoice(
                                      activePtJalur,
                                      index,
                                      "legacyKampus",
                                      ""
                                    );
                                  }}
                                  placeholder="Ketik nama kampus..."
                                  emptyText="Kampus tidak ditemukan"
                                  isLoading={isKampusLoading}
                                  disabled={!canEditPt || slotDisabledByRule}
                                  minChars={1}
                                  showClear={Boolean(
                                    choice.kampusQuery || choice.kampusId
                                  )}
                                  onClear={() => {
                                    updatePtChoice(
                                      activePtJalur,
                                      index,
                                      "kampusId",
                                      null
                                    );
                                    updatePtChoice(
                                      activePtJalur,
                                      index,
                                      "kampusQuery",
                                      ""
                                    );
                                  }}
                                />
                                {choice.legacyKampus && !choice.kampusId ? (
                                  <p className="text-xs text-amber-700">
                                    Legacy: {choice.legacyKampus}. Pilih kampus
                                    dari daftar sebelum simpan.
                                  </p>
                                ) : null}
                                {index + 1 > activePtLimit ? (
                                  <p className="text-xs text-amber-700">
                                    Slot ini di luar batas aktif. Kosongkan
                                    sebelum simpan ulang jika tidak diizinkan.
                                  </p>
                                ) : null}
                              </div>
                            );
                          })}
                        </div>
                      </div>

                      <div className="space-y-3">
                        <h4 className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Prodi & Jenjang
                        </h4>
                        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                          {activePtChoices.map((choice, index) => {
                            const slotHasValue = Boolean(
                              choice.kampusId ||
                              choice.kampusQuery.trim() ||
                              choice.legacyKampus.trim() ||
                              choice.prodi.trim()
                            );
                            const slotDisabledByRule =
                              index + 1 > activePtLimit && !slotHasValue;
                            return (
                              <div
                                key={`pt-choice-prodi-${index}`}
                                className="space-y-1"
                              >
                                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                                  Pilihan {index + 1} - Prodi & Jenjang
                                </label>
                                <div className="flex items-center gap-2">
                                  <select
                                    value={choice.jenjangProdi}
                                    onChange={(event) =>
                                      updatePtChoice(
                                        activePtJalur,
                                        index,
                                        "jenjangProdi",
                                        event.target.value
                                      )
                                    }
                                    disabled={!canEditPt || slotDisabledByRule}
                                    className="w-24 shrink-0 rounded-md border border-zinc-200 bg-white px-2 py-2 text-sm"
                                  >
                                    {jenjangProdiOptions.map((option) => (
                                      <option key={option} value={option}>
                                        {option}
                                      </option>
                                    ))}
                                  </select>
                                  <input
                                    type="text"
                                    value={choice.prodi}
                                    onChange={(event) =>
                                      updatePtChoice(
                                        activePtJalur,
                                        index,
                                        "prodi",
                                        event.target.value
                                      )
                                    }
                                    disabled={!canEditPt || slotDisabledByRule}
                                    className="min-w-0 flex-1 rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                                  />
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      </div>
                    </>
                  ) : (
                    <div className="rounded-md border border-zinc-200 bg-zinc-50 p-3 text-sm text-zinc-600">
                      Mode Kedinasan TNI/POLRI tidak memerlukan input
                      kampus/prodi.
                    </div>
                  )}
                </div>

                {ptSaveMessage ? (
                  <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                    {ptSaveMessage}
                  </p>
                ) : null}
                {ptSaveError ? (
                  <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                    {ptSaveError}
                  </p>
                ) : null}
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={closePtDetail}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Tutup
              </button>
              {showCancelPtPathButton ? (
                <button
                  type="button"
                  onClick={handleCancelPtByJalur}
                  disabled={isPtSaving || isPtCancellingPath}
                  className="rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm font-semibold text-red-700 transition hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isPtCancellingPath
                    ? "Membatalkan..."
                    : `Batalkan ${activePtJalur}`}
                </button>
              ) : null}
              {canEditPt ? (
                <button
                  type="button"
                  onClick={handlePtSave}
                  disabled={isPtSaving}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isPtSaving ? "Menyimpan..." : "Simpan"}
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}

      {/*
      {wirausahaTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-5xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Rencana Wirausaha Siswa
              </h2>
              <p className="text-sm text-zinc-600">
                {wirausahaTarget.namaSiswa || "-"} -{" "}
                {wirausahaTarget.nisn || "-"} - {wirausahaTarget.rombel || "-"}
              </p>
            </div>

            {isWirausahaLoadingDetail ? (
              <div className="mt-4 rounded-md border border-dashed border-zinc-200 p-4 text-sm text-zinc-500">
                Memuat detail...
              </div>
            ) : wirausahaDetailError ? (
              <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-600">
                {wirausahaDetailError}
              </div>
            ) : (
              <div className="mt-4 space-y-4">
                {wirausahaLocked ? (
                  <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
                    {wirausahaIsFinalized
                      ? "Data sudah final dan terkunci."
                      : "Tahun sudah tutup buku. Hanya finalisasi yang diperbolehkan."}
                  </div>
                ) : !isReadOnly ? null : (
                  <div className="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-700">
                    Mode baca saja. Hanya admin sekolah yang dapat memperbarui
                    rencana wirausaha.
                  </div>
                )}

                <div className="grid gap-4 md:grid-cols-2">
                  {wirausahaDraft.plans.map((plan, index) => (
                    <div
                      key={`wirausaha-plan-${index}`}
                      className="space-y-3 rounded-lg border border-zinc-200 p-4"
                    >
                      <h3 className="text-sm font-semibold text-zinc-900">
                        Rencana {index + 1}
                      </h3>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Nama Usaha
                        </label>
                        <input
                          type="text"
                          value={plan.nama_usaha}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "nama_usaha",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Kategori Usaha
                        </label>
                        <input
                          type="text"
                          value={plan.kategori_usaha}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "kategori_usaha",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Status Kesiapan
                        </label>
                        <input
                          type="text"
                          value={plan.status_kesiapan}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "status_kesiapan",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Rencana Mulai
                        </label>
                        <input
                          type="text"
                          value={plan.rencana_mulai}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "rencana_mulai",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Modal Awal
                        </label>
                        <input
                          type="text"
                          value={plan.modal_awal}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "modal_awal",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Sumber Modal
                        </label>
                        <input
                          type="text"
                          value={plan.sumber_modal}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "sumber_modal",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Lokasi (Opsional)
                        </label>
                        <input
                          type="text"
                          value={plan.lokasi ?? ""}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "lokasi",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Platform (Opsional)
                        </label>
                        <input
                          type="text"
                          value={plan.platform ?? ""}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "platform",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Catatan (Opsional)
                        </label>
                        <textarea
                          value={plan.catatan ?? ""}
                          onChange={(event) =>
                            updateWirausahaPlan(
                              index,
                              "catatan",
                              event.target.value
                            )
                          }
                          disabled={!canEditWirausaha}
                          className="min-h-[80px] w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                    </div>
                  ))}
                </div>

                {wirausahaSaveMessage ? (
                  <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                    {wirausahaSaveMessage}
                  </p>
                ) : null}
                {wirausahaSaveError ? (
                  <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                    {wirausahaSaveError}
                  </p>
                ) : null}
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={closeWirausahaDetail}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Tutup
              </button>
              {canEditWirausaha ? (
                <button
                  type="button"
                  onClick={handleWirausahaSave}
                  disabled={isWirausahaSaving}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isWirausahaSaving ? "Menyimpan..." : "Simpan"}
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
      */}
    </AdminRouteGuard>
  );
}
