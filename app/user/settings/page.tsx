"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";
import { AUTH_MESSAGE_KEY } from "@/lib/auth";

import { PasswordInput } from "@/components/password-input";
import { SchoolLocationSettings } from "@/components/portal/school-location-settings";
import { MasterPerguruanTinggiCard } from "@/components/app/settings/master-perguruan-tinggi-card";
import { PtChoiceRulesCard } from "@/components/app/settings/pt-choice-rules-card";
import { SystemTimeSettingsCard } from "@/components/system-time/SystemTimeSettingsCard";
import { Card, CardContent } from "@/components/ui/card";
import { TypeaheadInput } from "@/components/typeahead-input";
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
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { formatIndoDateTime } from "@/lib/date";
import { jenjangOptions } from "@/lib/shared/jenjangOptions";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { getFilenameFromContentDisposition } from "@/lib/http";
import { z } from "zod";

type SchoolSettings = {
  id: number;
  npsnSekolah: string | null;
  namaSekolah: string;
  bentukPendidikan: string | null;
  statusSekolah: string | null;
  tmtAkreditasi: string | null;
  akreditasi: string | null;
  jumlahGuruAsn: number;
  jumlahGuruHonorer: number;
  jumlahTendikAsn: number;
  jumlahTendikHonorer: number;
  lastUpdatePrestasi?: string | null;
  lastUpdateSiswaPt?: string | null;
};

type UserProfile = {
  id: number;
  username: string;
};

type SettingsResponse = {
  school: SchoolSettings;
  user: UserProfile;
};

type AccountForm = {
  username: string;
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
};

type SchoolAdminAccount = {
  id: number;
  username: string;
  namaSekolah?: string | null;
  supervisor_name?: string | null;
  supervisor_username?: string | null;
  isOnline: boolean;
  hasInitialPassword: boolean;
  lastUpdatePrestasi?: string | null;
  lastUpdateSiswaPt?: string | null;
};

type CabdinAdminAccount = {
  id: number;
  name: string;
  username: string;
  levels: string[];
  createdAt?: string | null;
};

type LegacyImportLevelSetting = {
  level: string;
  enabled: boolean;
  override?: boolean;
  locked?: boolean;
  source?: string;
  updatedAt?: string | null;
};

type LegacyImportSettingsPayload = {
  globalEnabled: boolean;
  autoDisableYear: number;
  forceEnabled: boolean;
  maxYear: number;
  levels?: LegacyImportLevelSetting[];
};

type LegacyPTMigrationStatus = {
  status: string;
  migratedCount: number;
  skippedCount: number;
  reportPath?: string | null;
  startedAt?: string | null;
  finishedAt?: string | null;
  message?: string;
};

type SchoolOption = {
  id: number;
  npsnSekolah?: string | null;
  namaSekolah: string;
  supervisor?: {
    id: number;
    username: string;
    name: string;
  } | null;
};

type SchoolSuggestion = {
  id: number;
  nama_sekolah: string;
  npsn?: string | null;
};

type PengawasAccount = {
  id: number;
  username: string;
  name: string;
  schoolCount: number;
  createdAt?: string | null;
  schools?: PengawasSchool[];
};

type PengawasSchool = {
  id: number;
  npsn?: string | null;
  nama_sekolah: string;
  bentuk_pendidikan?: string | null;
};

type SupervisedSchool = {
  id: number;
  namaSekolah: string;
  npsnSekolah?: string | null;
  bentukPendidikan?: string | null;
};

type OpsMonitoringSchoolItem = {
  school_id: number;
  school_name: string;
  jenjang?: string | null;
  status_sekolah?: string | null;
  prestasi_updated: boolean;
  prestasi_last_update?: string | null;
  siswa_updated: boolean;
  siswa_last_update?: string | null;
  status_lulusan_final_count: number;
  pending_validasi_count: number;
};

type OpsMonitoringSchoolResponse = {
  items: OpsMonitoringSchoolItem[];
  page: number;
  limit: number;
  total: number;
};

type ResetDeletedSummary = {
  prestasi_siswa: number;
  class12_students: number;
  student_plans: number;
  student_outcomes: number;
  outcome_pt_details: number;
  outcome_work_details: number;
  outcome_business_details: number;
  applications: number;
  acceptance_offers: number;
  wirausaha: number;
  student_cohorts: number;
  student_final_outcomes: number;
};

type ResetSummary = {
  school_id: number;
  tahun_lulus: number | null;
  deleted: ResetDeletedSummary;
};

type BulkGenerateSummary = {
  createdCount?: number;
  generatedCount: number;
  skippedCount: number;
  rejectedCount?: number;
  generated: {
    schoolId: number;
    namaSekolah: string;
    username: string;
    reason?: string;
  }[];
  skipped: { schoolId: number; namaSekolah: string; reason: string }[];
  rejected?: { schoolId: number; namaSekolah: string; reason: string }[];
};

type RevealSchoolAdminPasswordPayload = {
  username: string;
  password: string;
  npsn: string;
  namaSekolah: string;
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

const cabdinAccountEditSchema = z.object({
  name: z.string().trim().min(1, "Nama wajib diisi."),
  username: z
    .string()
    .trim()
    .min(1, "Email wajib diisi.")
    .email("Format email tidak valid."),
});

const cabdinPasswordResetSchema = z
  .object({
    newPassword: z.string().min(8, "Password minimal 8 karakter."),
    confirmPassword: z.string().min(1, "Konfirmasi password wajib diisi."),
  })
  .refine((value) => value.newPassword === value.confirmPassword, {
    message: "Konfirmasi password tidak sama.",
    path: ["confirmPassword"],
  });

const fetchSchoolSettings = async (): Promise<SettingsResponse> => {
  const response = await apiFetch("/user/settings");
  if (!response.ok) {
    throw new Error("Failed to fetch settings");
  }
  const payload = (await response.json()) as { data: SettingsResponse };
  return payload.data;
};

const fetchSupervisedSchools = async (): Promise<SupervisedSchool[]> => {
  const response = await apiFetch("/api/user/me/schools");
  if (!response.ok) {
    throw new Error("Failed to fetch supervised schools");
  }
  const payload = (await response.json()) as { data?: SupervisedSchool[] };
  return payload.data ?? [];
};

const fetchOpsMonitoringSchools = async (params: {
  q?: string;
  page?: number;
  limit?: number;
  schoolId?: number | null;
}): Promise<OpsMonitoringSchoolResponse> => {
  const query = new URLSearchParams();
  if (params.q) {
    query.set("q", params.q);
  }
  if (params.page) {
    query.set("page", String(params.page));
  }
  if (params.limit) {
    query.set("limit", String(params.limit));
  }
  if (params.schoolId) {
    query.set("school_id", String(params.schoolId));
  }
  const response = await apiFetch(
    `/api/user/me/ops-monitoring-schools${
      query.toString() ? `?${query.toString()}` : ""
    }`
  );
  if (!response.ok) {
    throw new Error("Gagal memuat monitoring operasional sekolah.");
  }
  const payload = (await response.json()) as {
    data?: OpsMonitoringSchoolResponse;
  };
  return (
    payload.data ?? {
      items: [],
      page: params.page ?? 1,
      limit: params.limit ?? 10,
      total: 0,
    }
  );
};

const fetchSchoolAdmins = async (params: {
  sortBy?: string;
  order?: string;
  q?: string;
  schoolId?: number | null;
  page?: number;
  pageSize?: number;
}): Promise<{ data: SchoolAdminAccount[]; totalCount: number | null }> => {
  const query = new URLSearchParams();
  if (params.sortBy) {
    query.set("sortBy", params.sortBy);
  }
  if (params.order) {
    query.set("order", params.order);
  }
  if (params.q) {
    query.set("q", params.q);
  }
  if (params.schoolId) {
    query.set("schoolId", String(params.schoolId));
  }
  if (params.page) {
    query.set("page", String(params.page));
  }
  if (params.pageSize) {
    query.set("pageSize", String(params.pageSize));
  }
  const response = await apiFetch(
    `/admin/school-admins${query.toString() ? `?${query.toString()}` : ""}`
  );
  if (!response.ok) {
    const clone = response.clone();
    let errorBody: unknown = "";
    try {
      errorBody = await clone.json();
    } catch {
      errorBody = await clone.text().catch(() => "");
    }
    const preview = (() => {
      if (typeof errorBody === "string") return errorBody.trim();
      if (errorBody && typeof errorBody === "object") {
        try {
          return JSON.stringify(errorBody);
        } catch {
          return String(errorBody);
        }
      }
      return "";
    })();
    console.error(
      `[school-admins] request failed status=${response.status}${
        preview ? ` body=${preview}` : ""
      }`
    );

    const parsedMessage =
      typeof errorBody === "object" &&
      errorBody !== null &&
      ("message" in errorBody || "error" in errorBody)
        ? String(
            (errorBody as { message?: unknown; error?: unknown }).message ??
              (errorBody as { message?: unknown; error?: unknown }).error ??
              ""
          )
        : "";
    throw new Error(parsedMessage || "Failed to fetch school admins");
  }
  const payload = (await response.json()) as {
    data?:
      | SchoolAdminAccount[]
      | { items?: SchoolAdminAccount[]; rows?: SchoolAdminAccount[] };
    meta?: { total_count?: number; total?: number };
    total_count?: number;
    total?: number;
  };
  const data = Array.isArray(payload.data)
    ? payload.data
    : (payload.data?.items ?? payload.data?.rows ?? []);
  let totalCount =
    payload.meta?.total_count ??
    payload.meta?.total ??
    payload.total_count ??
    payload.total ??
    null;
  if (totalCount === 0 && data.length > 0) {
    totalCount = null;
  }
  return {
    data,
    totalCount,
  };
};

const readCabdinAdminErrorMessage = async (response: Response) => {
  const body = (await response.json().catch(() => null)) as {
    message?: string;
    details?: unknown;
    error?: string;
  } | null;
  if (!body) return "";
  if (typeof body.message === "string" && body.message.trim()) {
    return body.message.trim();
  }
  if (typeof body.details === "string" && body.details.trim()) {
    return body.details.trim();
  }
  if (typeof body.error === "string" && body.error.trim()) {
    return body.error.trim();
  }
  return "";
};

const mapCabdinAdminError = (code: string) => {
  switch (code) {
    case "LEVELS_REQUIRED":
      return "Pilih minimal satu jenjang.";
    case "LEVEL_INVALID":
      return "Jenjang tidak valid.";
    case "PASSWORD_MIN_8":
      return "Password minimal 8 karakter.";
    case "EMAIL_REQUIRED":
      return "Email wajib diisi.";
    case "EMAIL_INVALID":
      return "Format email tidak valid.";
    case "NAME_REQUIRED":
      return "Nama wajib diisi.";
    case "EMAIL_EXISTS":
      return "Email sudah digunakan.";
    case "USERNAME_EXISTS":
      return "Email sudah digunakan.";
    case "USER_NOT_FOUND":
      return "Admin cabdin tidak ditemukan.";
    case "USER_NOT_CABDIN":
      return "User bukan admin cabdin.";
    case "INVALID_PAYLOAD":
      return "Lengkapi data terlebih dahulu.";
    default:
      return "";
  }
};

const fetchCabdinAdmins = async (): Promise<CabdinAdminAccount[]> => {
  const response = await apiFetch("/admin/master/cabdin-admins");
  if (!response.ok) {
    const message = mapCabdinAdminError(
      await readCabdinAdminErrorMessage(response)
    );
    throw new Error(message || "Gagal memuat admin cabdin.");
  }
  const payload = (await response.json()) as { data?: CabdinAdminAccount[] };
  return Array.isArray(payload.data) ? payload.data : [];
};

const createCabdinAdmin = async (payload: {
  name: string;
  username: string;
  password: string;
  levels: string[];
}) => {
  const response = await apiFetch("/admin/master/cabdin-admins", {
    method: "POST",
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const message = mapCabdinAdminError(
      await readCabdinAdminErrorMessage(response)
    );
    throw new Error(message || "Gagal membuat admin cabdin.");
  }
  return response.json();
};

const updateCabdinAdminLevels = async (id: number, levels: string[]) => {
  const response = await apiFetch(`/admin/master/cabdin-admins/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ levels }),
  });
  if (!response.ok) {
    const message = mapCabdinAdminError(
      await readCabdinAdminErrorMessage(response)
    );
    throw new Error(message || "Gagal mengubah jenjang admin cabdin.");
  }
  return response.json();
};

const updateCabdinAdminAccount = async (
  id: number,
  payload: { name: string; username: string }
) => {
  const response = await apiFetch(`/admin/master/cabdin-admins/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const message = mapCabdinAdminError(
      await readCabdinAdminErrorMessage(response)
    );
    throw new Error(message || "Gagal memperbarui akun admin cabdin.");
  }
  return response.json();
};

const resetCabdinAdminPassword = async (id: number, newPassword: string) => {
  const response = await apiFetch(
    `/admin/master/cabdin-admins/${id}/password`,
    {
      method: "PATCH",
      body: JSON.stringify({ new_password: newPassword }),
    }
  );
  if (!response.ok) {
    const message = mapCabdinAdminError(
      await readCabdinAdminErrorMessage(response)
    );
    throw new Error(message || "Gagal reset password admin cabdin.");
  }
  return response.json();
};

const readLegacyImportErrorMessage = async (response: Response) => {
  const body = (await response.json().catch(() => null)) as {
    message?: string;
    details?: unknown;
    error?: string;
  } | null;
  if (!body) return "";
  if (typeof body.message === "string" && body.message.trim()) {
    return body.message.trim();
  }
  if (typeof body.details === "string" && body.details.trim()) {
    return body.details.trim();
  }
  if (typeof body.error === "string" && body.error.trim()) {
    return body.error.trim();
  }
  return "";
};

const mapLegacyImportError = (code: string) => {
  switch (code) {
    case "LEVEL_REQUIRED":
      return "Jenjang wajib diisi.";
    case "LEVELS_REQUIRED":
      return "Pilih minimal satu jenjang.";
    case "LEVEL_INVALID":
      return "Jenjang tidak valid.";
    case "LEVEL_ENABLED_REQUIRED":
      return "Status jenjang wajib diisi.";
    case "LEVEL_NOT_ALLOWED":
      return "Anda tidak diizinkan mengubah jenjang ini.";
    case "MASTER_OVERRIDE_ACTIVE":
      return "Jenjang ini sudah di-override admin master.";
    case "AUTO_DISABLE_YEAR_INVALID":
      return "Tahun auto-disable tidak valid.";
    case "MAX_YEAR_INVALID":
      return "Tahun maksimal tidak valid.";
    case "INVALID_PAYLOAD":
      return "Payload tidak valid.";
    default:
      return "";
  }
};

const fetchLegacyImportSettings =
  async (): Promise<LegacyImportSettingsPayload> => {
    const response = await apiFetch("/admin/master/legacy-import/settings");
    if (!response.ok) {
      const message = mapLegacyImportError(
        await readLegacyImportErrorMessage(response)
      );
      throw new Error(message || "Gagal memuat pengaturan legacy import.");
    }
    const payload = (await response.json()) as {
      data?: LegacyImportSettingsPayload;
    };
    if (!payload.data) {
      throw new Error("Gagal memuat pengaturan legacy import.");
    }
    return payload.data;
  };

const updateLegacyImportSettings = async (payload: {
  globalEnabled?: boolean;
  autoDisableYear?: number;
  forceEnabled?: boolean;
  maxYear?: number;
}) => {
  const response = await apiFetch("/admin/master/legacy-import/settings", {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const message = mapLegacyImportError(
      await readLegacyImportErrorMessage(response)
    );
    throw new Error(message || "Gagal menyimpan pengaturan legacy import.");
  }
  return response.json();
};

const updateLegacyImportLevelsMaster = async (
  levels: { level: string; enabled: boolean; override: boolean }[]
) => {
  const response = await apiFetch("/admin/master/legacy-import/levels", {
    method: "PATCH",
    body: JSON.stringify({ levels }),
  });
  if (!response.ok) {
    const message = mapLegacyImportError(
      await readLegacyImportErrorMessage(response)
    );
    throw new Error(message || "Gagal menyimpan pengaturan jenjang legacy.");
  }
  return response.json();
};

const fetchCabdinLegacyLevels = async (): Promise<{
  settings: LegacyImportSettingsPayload;
  levels: LegacyImportLevelSetting[];
}> => {
  const response = await apiFetch("/admin/cabdin/legacy-import/levels");
  if (!response.ok) {
    const message = mapLegacyImportError(
      await readLegacyImportErrorMessage(response)
    );
    throw new Error(message || "Gagal memuat pengaturan legacy import.");
  }
  const payload = (await response.json()) as {
    data?: {
      settings: LegacyImportSettingsPayload;
      levels: LegacyImportLevelSetting[];
    };
  };
  if (!payload.data) {
    throw new Error("Gagal memuat pengaturan legacy import.");
  }
  return payload.data;
};

const updateCabdinLegacyLevels = async (
  levels: { level: string; enabled: boolean }[]
) => {
  const response = await apiFetch("/admin/cabdin/legacy-import/levels", {
    method: "PATCH",
    body: JSON.stringify({ levels }),
  });
  if (!response.ok) {
    const message = mapLegacyImportError(
      await readLegacyImportErrorMessage(response)
    );
    throw new Error(message || "Gagal menyimpan pengaturan legacy import.");
  }
  return response.json();
};

const fetchLegacyPTMigrationStatus = async (
  run = false
): Promise<LegacyPTMigrationStatus> => {
  const response = await apiFetch(
    `/admin/legacy-migrations/pt/status${run ? "?run=1" : ""}`
  );
  if (!response.ok) {
    throw new Error("Gagal memuat status migrasi legacy PT.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return {
    status: readString(data?.status),
    migratedCount: readNumber(data?.migrated_count ?? data?.migratedCount),
    skippedCount: readNumber(data?.skipped_count ?? data?.skippedCount),
    reportPath: readString(data?.report_path ?? data?.reportPath) || null,
    startedAt: readString(data?.started_at ?? data?.startedAt) || null,
    finishedAt: readString(data?.finished_at ?? data?.finishedAt) || null,
    message: readString(data?.message),
  };
};

const downloadLegacyPTMigrationReport = async (): Promise<{
  blob: Blob;
  filename: string;
}> => {
  const response = await apiFetch("/admin/legacy-migrations/pt/report.xlsx");
  if (!response.ok) {
    throw new Error("Laporan migrasi belum tersedia.");
  }
  const blob = await response.blob();
  const filename =
    getFilenameFromContentDisposition(
      response.headers.get("content-disposition")
    ) ?? "legacy-pt-migration-report.xlsx";
  return { blob, filename };
};

const revealSchoolAdminPassword = async (
  id: number,
  adminPassword: string
): Promise<RevealSchoolAdminPasswordPayload> => {
  const response = await apiFetch(
    `/admin/school-admins/${id}/reveal-password`,
    {
      method: "POST",
      body: JSON.stringify({ adminPassword }),
    }
  );

  const body = (await response.json().catch(() => null)) as {
    data?: RevealSchoolAdminPasswordPayload;
    message?: string;
  } | null;

  if (!response.ok) {
    const message = body?.message;
    if (message === "ADMIN_PASSWORD_SALAH") {
      throw new Error("Password admin salah.");
    }
    if (message === "INITIAL_PASSWORD_NOT_AVAILABLE") {
      throw new Error("Password tidak tersedia.");
    }
    if (response.status === 429) {
      throw new Error("Terlalu banyak percobaan. Coba lagi nanti.");
    }
    throw new Error("Gagal memuat password awal.");
  }
  if (!body?.data) {
    throw new Error("Gagal memuat password awal.");
  }
  return body.data;
};

const downloadGeneratedSchoolAdmins = async (payload: {
  jenjang: string;
  status: string;
}) => {
  const response = await apiFetch("/admin/generate-school-admins?download=1", {
    method: "POST",
    body: JSON.stringify(payload),
  });

  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    const body = (await response.json().catch(() => null)) as {
      message?: string;
      error?: string;
      details?: string;
    } | null;
    const message = body?.message ?? body?.details ?? body?.error;
    throw new Error(message || "Gagal generate akun.");
  }

  if (!response.ok) {
    const text = await response.text().catch(() => "");
    throw new Error(text || "Gagal generate akun.");
  }

  const blob = await response.blob();
  const filename =
    getFilenameFromContentDisposition(
      response.headers.get("content-disposition")
    ) ?? "akun-admin-sekolah.xlsx";

  return { blob, filename };
};

const fetchAdminSchools = async (): Promise<SchoolOption[]> => {
  const response = await apiFetch("/admin/schools");
  if (!response.ok) {
    throw new Error("Failed to fetch schools");
  }
  const payload = (await response.json()) as { data: SchoolOption[] };
  return payload.data ?? [];
};

const fetchPengawasAccounts = async (): Promise<PengawasAccount[]> => {
  const response = await apiFetch("/admin/pengawas");
  if (!response.ok) {
    throw new Error("Gagal memuat daftar pengawas.");
  }
  const payload = (await response.json()) as { data?: PengawasAccount[] };
  return payload.data ?? [];
};

const formatSchoolConflictMessage = (details: unknown) => {
  if (!Array.isArray(details)) {
    return "Beberapa sekolah sudah diampu pengawas lain.";
  }
  const names = details
    .map((item) => {
      if (!item || typeof item !== "object") return "";
      const record = item as { nama_sekolah?: unknown };
      return typeof record.nama_sekolah === "string"
        ? record.nama_sekolah.trim()
        : "";
    })
    .filter(Boolean);
  if (names.length === 0) {
    return "Beberapa sekolah sudah diampu pengawas lain.";
  }
  return `Sekolah sudah diampu pengawas lain: ${names.join(", ")}`;
};

const readPengawasError = async (response: Response) => {
  const body = (await response.json().catch(() => null)) as {
    message?: string;
    error?: string;
    details?: unknown;
  } | null;
  return {
    message: body?.message || body?.error || "",
    details: body?.details,
  };
};

const createPengawasAccount = async (payload: {
  username: string;
  password: string;
  name?: string;
  school_ids?: number[];
}) => {
  const response = await apiFetch("/admin/pengawas", {
    method: "POST",
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const { message, details } = await readPengawasError(response);
    if (response.status === 409 && message === "SCHOOL_ASSIGNMENT_CONFLICT") {
      throw new Error(formatSchoolConflictMessage(details));
    }
    if (message === "PASSWORD_MIN_8") {
      throw new Error("Password minimal 8 karakter.");
    }
    if (message === "USERNAME_EXISTS") {
      throw new Error("Username sudah digunakan.");
    }
    if (message === "INVALID_PAYLOAD") {
      throw new Error("Lengkapi data pengawas.");
    }
    throw new Error("Gagal membuat akun pengawas.");
  }
  const body = (await response.json().catch(() => null)) as {
    data?: PengawasAccount;
  } | null;
  if (!body?.data) {
    throw new Error("Gagal membuat akun pengawas.");
  }
  return body.data;
};

const updatePengawasAccount = async (
  id: number,
  payload: { username?: string; name?: string }
) => {
  const response = await apiFetch(`/admin/pengawas/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const { message } = await readPengawasError(response);
    if (message === "USERNAME_EXISTS") {
      throw new Error("Username sudah digunakan.");
    }
    if (message === "USER_NOT_FOUND") {
      throw new Error("Akun pengawas tidak ditemukan.");
    }
    throw new Error("Gagal memperbarui akun pengawas.");
  }
  const body = (await response.json().catch(() => null)) as {
    data?: PengawasAccount;
  } | null;
  if (!body?.data) {
    throw new Error("Gagal memperbarui akun pengawas.");
  }
  return body.data;
};

const resetPengawasPassword = async (id: number, password: string) => {
  const response = await apiFetch(`/admin/pengawas/${id}/reset-password`, {
    method: "POST",
    body: JSON.stringify({ password }),
  });
  if (!response.ok) {
    const { message } = await readPengawasError(response);
    if (message === "PASSWORD_MIN_8") {
      throw new Error("Password minimal 8 karakter.");
    }
    throw new Error("Gagal reset password pengawas.");
  }
};

const updatePengawasSchools = async (id: number, schoolIDs: number[]) => {
  const response = await apiFetch(`/admin/pengawas/${id}/schools`, {
    method: "PUT",
    body: JSON.stringify({ school_ids: schoolIDs }),
  });
  if (!response.ok) {
    const { message, details } = await readPengawasError(response);
    if (response.status === 409 && message === "SCHOOL_ASSIGNMENT_CONFLICT") {
      throw new Error(formatSchoolConflictMessage(details));
    }
    if (message === "SCHOOL_NOT_FOUND") {
      throw new Error("Sekolah yang dipilih tidak ditemukan.");
    }
    throw new Error("Gagal menyimpan sekolah ampu pengawas.");
  }
  const body = (await response.json().catch(() => null)) as {
    data?: PengawasSchool[];
  } | null;
  return body?.data ?? [];
};

const fetchSchoolSuggestions = async (
  query: string
): Promise<SchoolSuggestion[]> => {
  const response = await apiFetch(
    `/api/admin/schools/suggest?q=${encodeURIComponent(query)}&limit=10`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch school suggestions");
  }
  const payload = (await response.json()) as { data?: SchoolSuggestion[] };
  return payload.data ?? [];
};

const updateSchoolSettings = async (payload: {
  school: SchoolSettings;
  user?: {
    username?: string;
    current_password?: string;
    new_password?: string;
  };
}) => {
  const response = await apiFetch("/user/settings", {
    method: "PUT",
    body: JSON.stringify(payload),
  });

  const body = (await response.json().catch(() => null)) as {
    data?: SettingsResponse;
    error?: string;
    message?: string;
  } | null;
  if (!response.ok) {
    if (response.status === 429) {
      throw new Error("Terlalu banyak percobaan. Coba lagi nanti.");
    }
    const message = body?.message;
    if (message === "PASSWORD_LAMA_REQUIRED") {
      throw new Error("Password lama wajib diisi untuk perubahan akun.");
    }
    if (message === "PASSWORD_LAMA_SALAH") {
      throw new Error("Password lama salah.");
    }
    if (message === "USERNAME_TAK_TERSEDIA") {
      throw new Error("Username sudah digunakan.");
    }
    if (message === "PASSWORD_MIN_8") {
      throw new Error("Password baru minimal 8 karakter.");
    }
    throw new Error("Gagal menyimpan data sekolah.");
  }
  if (!body?.data) {
    throw new Error("Gagal menyimpan data sekolah.");
  }
  return body.data;
};

const updateSchoolAdmin = async (id: number, values: { username: string }) => {
  const response = await apiFetch(`/admin/school-admins/${id}`, {
    method: "PUT",
    body: JSON.stringify({
      username: values.username,
    }),
  });
  if (!response.ok) {
    throw new Error("Gagal memperbarui akun.");
  }
  return response.json();
};

const resetSchoolAdminPassword = async (
  id: number,
  values: { adminPassword: string; newPassword: string }
) => {
  const response = await apiFetch(`/admin/school-admins/${id}/password`, {
    method: "PUT",
    body: JSON.stringify({
      admin_password: values.adminPassword,
      new_password: values.newPassword,
    }),
  });
  const body = (await response.json().catch(() => null)) as {
    error?: string;
    message?: string;
    details?: string;
  } | null;
  if (!response.ok) {
    const message = body?.message;
    if (message === "ADMIN_PASSWORD_SALAH") {
      throw new Error("Password admin salah.");
    }
    if (
      message === "PASSWORD_MIN_8" ||
      body?.details === "password too short"
    ) {
      throw new Error("Password baru minimal 8 karakter.");
    }
    throw new Error("Gagal reset password admin sekolah.");
  }
  return body;
};

const createSchoolAdmin = async (
  schoolId: number,
  username: string,
  password: string
) => {
  const response = await apiFetch("/admin/school-admins", {
    method: "POST",
    body: JSON.stringify({
      school_id: schoolId,
      username,
      password,
    }),
  });
  if (!response.ok) {
    throw new Error("Gagal membuat admin sekolah.");
  }
  return response.json();
};

const resetSchoolData = async (payload: {
  school_id: number;
  tahun_lulus: number | null;
  reset_prestasi: boolean;
  reset_siswa: boolean;
  override: boolean;
}) => {
  const response = await apiFetch("/api/admin/reset-data", {
    method: "POST",
    body: JSON.stringify(payload),
  });
  const body = (await response.json().catch(() => null)) as {
    data?: ResetSummary;
    error?: string;
    message?: string;
    details?: unknown;
  } | null;
  if (!response.ok) {
    if (response.status === 400) {
      const detailMessage =
        typeof body?.details === "string" ? body.details : "";
      throw new Error(
        body?.message || detailMessage || body?.error || "Validasi gagal."
      );
    }
    if (response.status === 403) {
      throw new Error("Anda tidak memiliki akses.");
    }
    throw new Error(
      body?.message || body?.error || "Gagal menghapus data sekolah."
    );
  }
  if (!body?.data) {
    throw new Error("Gagal menghapus data sekolah.");
  }
  return body.data;
};

const changeSelfPassword = async (payload: {
  oldPassword: string;
  newPassword: string;
}) => {
  const response = await apiFetch("/api/user/me/password", {
    method: "PATCH",
    body: JSON.stringify({
      old_password: payload.oldPassword,
      new_password: payload.newPassword,
    }),
  });
  const body = (await response.json().catch(() => null)) as {
    message?: string;
    code?: string;
  } | null;
  if (!response.ok) {
    if (
      body?.code === "OLD_PASSWORD_INVALID" ||
      body?.message === "PASSWORD_LAMA_SALAH"
    ) {
      throw new Error("OLD_PASSWORD_INVALID");
    }
    if (body?.message === "PASSWORD_MIN_8") {
      throw new Error("PASSWORD_MIN_8");
    }
    if (response.status === 403) {
      throw new Error("FORBIDDEN");
    }
    if (response.status === 429) {
      throw new Error("TOO_MANY_REQUESTS");
    }
    throw new Error("Gagal mengubah password.");
  }
  return body;
};

export default function UserSettingsPage() {
  const router = useRouter();
  const { user, status, logout } = useAuth();
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const isGaspolSchoolUser = user?.role === "USER_SEKOLAH_GASPOL";
  const isAdminMaster = user?.role === "ADMIN_MASTER";
  const isAdminCabdin = user?.role === "ADMIN";
  const isPengawas = user?.role === "PENGAWAS";
  const isAdmin = isAdminCabdin || isAdminMaster;
  const canUseSelfPassword =
    isAdminMaster || isAdminCabdin || isPengawas || isGaspolSchoolUser;
  const showOpsMonitoring = isAdminMaster || isAdminCabdin || isPengawas;
  const [isMounted, setIsMounted] = useState(false);
  const [supervisedSchools, setSupervisedSchools] = useState<
    SupervisedSchool[]
  >([]);
  const [isSupervisedSchoolsLoading, setIsSupervisedSchoolsLoading] =
    useState(false);
  const [supervisedSchoolsError, setSupervisedSchoolsError] = useState("");
  const [opsMonitoringRows, setOpsMonitoringRows] = useState<
    OpsMonitoringSchoolItem[]
  >([]);
  const [isOpsMonitoringLoading, setIsOpsMonitoringLoading] = useState(false);
  const [opsMonitoringError, setOpsMonitoringError] = useState("");
  const [opsMonitoringQuery, setOpsMonitoringQuery] = useState("");
  const [opsMonitoringPage, setOpsMonitoringPage] = useState(1);
  const [opsMonitoringTotal, setOpsMonitoringTotal] = useState(0);
  const [pengawasAccounts, setPengawasAccounts] = useState<PengawasAccount[]>(
    []
  );
  const [isPengawasLoading, setIsPengawasLoading] = useState(false);
  const [pengawasError, setPengawasError] = useState("");
  const [pengawasMessage, setPengawasMessage] = useState("");
  const [isPengawasCreateOpen, setIsPengawasCreateOpen] = useState(false);
  const [pengawasCreateName, setPengawasCreateName] = useState("");
  const [pengawasCreateUsername, setPengawasCreateUsername] = useState("");
  const [pengawasCreatePassword, setPengawasCreatePassword] = useState("");
  const [pengawasCreateSchoolQuery, setPengawasCreateSchoolQuery] =
    useState("");
  const [pengawasCreateSchoolIDs, setPengawasCreateSchoolIDs] = useState<
    number[]
  >([]);
  const [pengawasCreateSchoolSuggestions, setPengawasCreateSchoolSuggestions] =
    useState<SchoolSuggestion[]>([]);
  const [isPengawasCreateSchoolLoading, setIsPengawasCreateSchoolLoading] =
    useState(false);
  const [pengawasCreateError, setPengawasCreateError] = useState("");
  const [isPengawasCreating, setIsPengawasCreating] = useState(false);
  const [isPengawasEditOpen, setIsPengawasEditOpen] = useState(false);
  const [pengawasEditTarget, setPengawasEditTarget] =
    useState<PengawasAccount | null>(null);
  const [pengawasEditName, setPengawasEditName] = useState("");
  const [pengawasEditUsername, setPengawasEditUsername] = useState("");
  const [pengawasEditError, setPengawasEditError] = useState("");
  const [isPengawasEditSaving, setIsPengawasEditSaving] = useState(false);
  const [isPengawasResetOpen, setIsPengawasResetOpen] = useState(false);
  const [pengawasResetTarget, setPengawasResetTarget] =
    useState<PengawasAccount | null>(null);
  const [pengawasResetPassword, setPengawasResetPassword] = useState("");
  const [pengawasResetConfirm, setPengawasResetConfirm] = useState("");
  const [pengawasResetError, setPengawasResetError] = useState("");
  const [isPengawasResetSaving, setIsPengawasResetSaving] = useState(false);
  const [isPengawasSchoolsOpen, setIsPengawasSchoolsOpen] = useState(false);
  const [pengawasSchoolsTarget, setPengawasSchoolsTarget] =
    useState<PengawasAccount | null>(null);
  const [pengawasSchoolsQuery, setPengawasSchoolsQuery] = useState("");
  const [pengawasSchoolsSelectedIDs, setPengawasSchoolsSelectedIDs] = useState<
    number[]
  >([]);
  const [pengawasSchoolsSuggestions, setPengawasSchoolsSuggestions] = useState<
    SchoolSuggestion[]
  >([]);
  const [isPengawasSchoolsLoading, setIsPengawasSchoolsLoading] =
    useState(false);
  const [pengawasSchoolsError, setPengawasSchoolsError] = useState("");
  const [isPengawasSchoolsSaving, setIsPengawasSchoolsSaving] = useState(false);

  const [initialValues, setInitialValues] = useState<SchoolSettings | null>(
    null
  );
  const [formValues, setFormValues] = useState<SchoolSettings | null>(null);
  const [userProfile, setUserProfile] = useState<UserProfile | null>(null);
  const [accountValues, setAccountValues] = useState<AccountForm>({
    username: "",
    currentPassword: "",
    newPassword: "",
    confirmPassword: "",
  });
  const [isEditing, setIsEditing] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  const [adminAccounts, setAdminAccounts] = useState<SchoolAdminAccount[]>([]);
  const [adminError, setAdminError] = useState("");
  const [adminMessage, setAdminMessage] = useState("");
  const [cabdinAdmins, setCabdinAdmins] = useState<CabdinAdminAccount[]>([]);
  const [cabdinError, setCabdinError] = useState("");
  const [cabdinMessage, setCabdinMessage] = useState("");
  const [isCabdinLoading, setIsCabdinLoading] = useState(false);
  const [isCabdinSaving, setIsCabdinSaving] = useState(false);
  const [isCabdinCreateOpen, setIsCabdinCreateOpen] = useState(false);
  const [cabdinCreateName, setCabdinCreateName] = useState("");
  const [cabdinCreateUsername, setCabdinCreateUsername] = useState("");
  const [cabdinCreatePassword, setCabdinCreatePassword] = useState("");
  const [cabdinCreateLevels, setCabdinCreateLevels] = useState<string[]>([]);
  const [cabdinCreateError, setCabdinCreateError] = useState("");
  const [isCabdinEditOpen, setIsCabdinEditOpen] = useState(false);
  const [cabdinEditTarget, setCabdinEditTarget] =
    useState<CabdinAdminAccount | null>(null);
  const [cabdinEditLevels, setCabdinEditLevels] = useState<string[]>([]);
  const [cabdinEditError, setCabdinEditError] = useState("");
  const [isCabdinAccountEditOpen, setIsCabdinAccountEditOpen] = useState(false);
  const [cabdinAccountEditTarget, setCabdinAccountEditTarget] =
    useState<CabdinAdminAccount | null>(null);
  const [cabdinAccountEditName, setCabdinAccountEditName] = useState("");
  const [cabdinAccountEditUsername, setCabdinAccountEditUsername] =
    useState("");
  const [cabdinAccountEditError, setCabdinAccountEditError] = useState("");
  const [isCabdinAccountSaving, setIsCabdinAccountSaving] = useState(false);
  const [isCabdinPasswordResetOpen, setIsCabdinPasswordResetOpen] =
    useState(false);
  const [cabdinPasswordResetTarget, setCabdinPasswordResetTarget] =
    useState<CabdinAdminAccount | null>(null);
  const [cabdinPasswordResetValue, setCabdinPasswordResetValue] = useState("");
  const [cabdinPasswordResetConfirm, setCabdinPasswordResetConfirm] =
    useState("");
  const [cabdinPasswordResetError, setCabdinPasswordResetError] = useState("");
  const [isCabdinPasswordResetSaving, setIsCabdinPasswordResetSaving] =
    useState(false);
  const [legacySettingsError, setLegacySettingsError] = useState("");
  const [legacySettingsMessage, setLegacySettingsMessage] = useState("");
  const [isLegacySettingsLoading, setIsLegacySettingsLoading] = useState(false);
  const [isLegacySettingsSaving, setIsLegacySettingsSaving] = useState(false);
  const [isLegacyLevelsSaving, setIsLegacyLevelsSaving] = useState(false);
  const [legacyGlobalEnabled, setLegacyGlobalEnabled] = useState(true);
  const [legacyAutoDisableYear, setLegacyAutoDisableYear] = useState("2027");
  const [legacyForceEnabled, setLegacyForceEnabled] = useState(false);
  const [legacyMaxYear, setLegacyMaxYear] = useState("2025");
  const [legacyLevelOverrides, setLegacyLevelOverrides] = useState<
    Record<string, { override: boolean; enabled: boolean }>
  >({});
  const [cabdinLegacyLevels, setCabdinLegacyLevels] = useState<
    LegacyImportLevelSetting[]
  >([]);
  const [cabdinLegacyError, setCabdinLegacyError] = useState("");
  const [cabdinLegacyMessage, setCabdinLegacyMessage] = useState("");
  const [isCabdinLegacyLoading, setIsCabdinLegacyLoading] = useState(false);
  const [isCabdinLegacySaving, setIsCabdinLegacySaving] = useState(false);
  const [cabdinLegacyDraft, setCabdinLegacyDraft] = useState<
    Record<string, boolean>
  >({});
  const [legacyPtStatus, setLegacyPtStatus] =
    useState<LegacyPTMigrationStatus | null>(null);
  const [legacyPtError, setLegacyPtError] = useState("");
  const [legacyPtMessage, setLegacyPtMessage] = useState("");
  const [isLegacyPtLoading, setIsLegacyPtLoading] = useState(false);
  const [isLegacyPtRunning, setIsLegacyPtRunning] = useState(false);
  const [adminSortBy, setAdminSortBy] = useState("status");
  const [adminSortOrder, setAdminSortOrder] = useState<"asc" | "desc">("desc");
  const [adminPage, setAdminPage] = useState(1);
  const [adminTotalCount, setAdminTotalCount] = useState<number | null>(null);
  const [isAdminLoadingMore, setIsAdminLoadingMore] = useState(false);
  const [adminSchoolQuery, setAdminSchoolQuery] = useState("");
  const [adminSelectedSchoolId, setAdminSelectedSchoolId] = useState<
    number | null
  >(null);
  const [bulkJenjang, setBulkJenjang] = useState("SMA");
  const [bulkStatus, setBulkStatus] = useState("NEGERI");
  const [bulkSummary, setBulkSummary] = useState<BulkGenerateSummary | null>(
    null
  );
  const [bulkError, setBulkError] = useState("");
  const [bulkMessage, setBulkMessage] = useState("");
  const [isBulkGenerating, setIsBulkGenerating] = useState(false);
  const [isRevealPasswordOpen, setIsRevealPasswordOpen] = useState(false);
  const [revealTarget, setRevealTarget] = useState<SchoolAdminAccount | null>(
    null
  );
  const [revealAdminPassword, setRevealAdminPassword] = useState("");
  const [revealedPassword, setRevealedPassword] = useState<string | null>(null);
  const [revealError, setRevealError] = useState("");
  const [isRevealing, setIsRevealing] = useState(false);

  const [isExportOpen, setIsExportOpen] = useState(false);
  const [exportAdminPassword, setExportAdminPassword] = useState("");
  const [exportError, setExportError] = useState("");
  const [isExporting, setIsExporting] = useState(false);
  const [adminSchoolSuggestions, setAdminSchoolSuggestions] = useState<
    SchoolSuggestion[]
  >([]);
  const [isAdminSchoolLoading, setIsAdminSchoolLoading] = useState(false);
  const [schoolOptions, setSchoolOptions] = useState<SchoolOption[]>([]);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [selectedSchoolId, setSelectedSchoolId] = useState<number | null>(null);
  const [createUsername, setCreateUsername] = useState("");
  const [createPassword, setCreatePassword] = useState("");
  const [createError, setCreateError] = useState("");
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [selectedAdmin, setSelectedAdmin] = useState<SchoolAdminAccount | null>(
    null
  );
  const [editUsername, setEditUsername] = useState("");
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const [adminPassword, setAdminPassword] = useState("");
  const [newAdminPassword, setNewAdminPassword] = useState("");
  const [confirmAdminPassword, setConfirmAdminPassword] = useState("");
  const [resetError, setResetError] = useState("");
  const [isResetSaving, setIsResetSaving] = useState(false);
  const [resetSchoolId, setResetSchoolId] = useState<number | null>(null);
  const [resetSchoolQuery, setResetSchoolQuery] = useState("");
  const [resetSchoolSuggestions, setResetSchoolSuggestions] = useState<
    SchoolSuggestion[]
  >([]);
  const [isResetSchoolLoading, setIsResetSchoolLoading] = useState(false);
  const [resetSelectedSchool, setResetSelectedSchool] =
    useState<SchoolSuggestion | null>(null);
  const [resetDeleteAchievement, setResetDeleteAchievement] = useState(false);
  const [resetDeleteAdmissions, setResetDeleteAdmissions] = useState(false);
  const [resetTahunLulus, setResetTahunLulus] = useState<number | null>(null);
  const [resetOverride, setResetOverride] = useState(false);
  const [resetModalOpen, setResetModalOpen] = useState(false);
  const [resetDataError, setResetDataError] = useState("");
  const [isResetDataSaving, setIsResetDataSaving] = useState(false);
  const [resetMessage, setResetMessage] = useState("");
  const debouncedResetSchoolQuery = useDebouncedValue(resetSchoolQuery, 300);
  const debouncedAdminSchoolQuery = useDebouncedValue(adminSchoolQuery, 300);
  const debouncedOpsMonitoringQuery = useDebouncedValue(
    opsMonitoringQuery,
    300
  );
  const debouncedPengawasCreateSchoolQuery = useDebouncedValue(
    pengawasCreateSchoolQuery,
    300
  );
  const debouncedPengawasSchoolsQuery = useDebouncedValue(
    pengawasSchoolsQuery,
    300
  );
  const [isChangePasswordOpen, setIsChangePasswordOpen] = useState(false);
  const [oldAdminPassword, setOldAdminPassword] = useState("");
  const [newAdminPasswordValue, setNewAdminPasswordValue] = useState("");
  const [confirmAdminPasswordValue, setConfirmAdminPasswordValue] =
    useState("");
  const [changePasswordError, setChangePasswordError] = useState("");
  const [isChangePasswordSaving, setIsChangePasswordSaving] = useState(false);

  useEffect(() => {
    setIsMounted(true);
  }, []);

  const adminPageSize = 10;
  const opsMonitoringPageSize = 10;

  const loadCabdinAdmins = useCallback(async () => {
    if (!isAdminMaster) return;
    setIsCabdinLoading(true);
    setCabdinError("");
    try {
      const data = await fetchCabdinAdmins();
      setCabdinAdmins(data);
    } catch (error) {
      setCabdinError(
        error instanceof Error ? error.message : "Gagal memuat admin cabdin."
      );
    } finally {
      setIsCabdinLoading(false);
    }
  }, [isAdminMaster]);

  const loadAdminSchools = useCallback(async () => {
    if (!isAdmin) return;
    setAdminError("");
    try {
      const schools = await fetchAdminSchools();
      setSchoolOptions(schools);
      if (schools.length > 0) {
        setSelectedSchoolId((prev) => prev ?? schools[0].id);
      }
    } catch (loadErr) {
      setAdminError(
        loadErr instanceof Error
          ? loadErr.message
          : "Gagal memuat daftar sekolah."
      );
    }
  }, [isAdmin]);

  const loadPengawasAccounts = useCallback(async () => {
    if (!isAdminMaster) return;
    setIsPengawasLoading(true);
    setPengawasError("");
    try {
      const records = await fetchPengawasAccounts();
      setPengawasAccounts(records);
    } catch (loadErr) {
      setPengawasError(
        loadErr instanceof Error ? loadErr.message : "Gagal memuat pengawas."
      );
    } finally {
      setIsPengawasLoading(false);
    }
  }, [isAdminMaster]);

  const loadLegacySettings = useCallback(async () => {
    if (!isAdminMaster) return;
    setIsLegacySettingsLoading(true);
    setLegacySettingsError("");
    try {
      const data = await fetchLegacyImportSettings();
      setLegacyGlobalEnabled(Boolean(data.globalEnabled));
      setLegacyAutoDisableYear(String(data.autoDisableYear ?? ""));
      setLegacyForceEnabled(Boolean(data.forceEnabled));
      setLegacyMaxYear(String(data.maxYear ?? ""));
      const nextOverrides: Record<
        string,
        { override: boolean; enabled: boolean }
      > = {};
      jenjangOptions.forEach((level) => {
        const record = data.levels?.find((item) => item.level === level);
        nextOverrides[level] = {
          override: Boolean(record?.override),
          enabled: record?.enabled ?? true,
        };
      });
      setLegacyLevelOverrides(nextOverrides);
    } catch (error) {
      setLegacySettingsError(
        error instanceof Error
          ? error.message
          : "Gagal memuat pengaturan legacy import."
      );
    } finally {
      setIsLegacySettingsLoading(false);
    }
  }, [isAdminMaster]);

  const loadCabdinLegacySettings = useCallback(async () => {
    if (!isAdminCabdin) return;
    setIsCabdinLegacyLoading(true);
    setCabdinLegacyError("");
    try {
      const data = await fetchCabdinLegacyLevels();
      setCabdinLegacyLevels(data.levels ?? []);
      const nextDraft: Record<string, boolean> = {};
      (data.levels ?? []).forEach((item) => {
        nextDraft[item.level] = item.enabled;
      });
      setCabdinLegacyDraft(nextDraft);
    } catch (error) {
      setCabdinLegacyError(
        error instanceof Error
          ? error.message
          : "Gagal memuat pengaturan legacy import."
      );
    } finally {
      setIsCabdinLegacyLoading(false);
    }
  }, [isAdminCabdin]);

  const loadLegacyPtMigration = useCallback(
    async (run = false) => {
      if (!isAdminMaster) return;
      setIsLegacyPtLoading(true);
      setLegacyPtError("");
      try {
        const status = await fetchLegacyPTMigrationStatus(run);
        setLegacyPtStatus(status);
      } catch {
        setLegacyPtError("Gagal memuat status migrasi legacy PT.");
      } finally {
        setIsLegacyPtLoading(false);
      }
    },
    [isAdminMaster]
  );

  const loadAdminAccounts = useCallback(
    async (options?: { page?: number }) => {
      if (!isAdmin) return;
      const nextPage = options?.page ?? 1;
      const query =
        adminSelectedSchoolId != null ? "" : debouncedAdminSchoolQuery.trim();
      const resolvedQuery = query.length >= 2 ? query : "";

      setIsAdminLoadingMore(true);
      setAdminError("");
      try {
        const result = await fetchSchoolAdmins({
          sortBy: adminSortBy,
          order: adminSortOrder,
          q: resolvedQuery,
          schoolId: adminSelectedSchoolId,
          page: nextPage,
          pageSize: adminPageSize,
        });
        setAdminAccounts(result.data);
        setAdminTotalCount(result.totalCount);
        setAdminPage(nextPage);
      } catch {
        setAdminError("Gagal memuat akun admin sekolah.");
      } finally {
        setIsAdminLoadingMore(false);
      }
    },
    [
      isAdmin,
      adminSelectedSchoolId,
      debouncedAdminSchoolQuery,
      adminSortBy,
      adminSortOrder,
      adminPageSize,
    ]
  );

  const loadOpsMonitoringSchools = useCallback(
    async (options?: { page?: number }) => {
      if (!showOpsMonitoring) return;
      const nextPage = options?.page ?? 1;
      const rawQuery = debouncedOpsMonitoringQuery.trim();
      const resolvedQuery = rawQuery;

      setIsOpsMonitoringLoading(true);
      setOpsMonitoringError("");
      try {
        const result = await fetchOpsMonitoringSchools({
          q: resolvedQuery,
          page: nextPage,
          limit: opsMonitoringPageSize,
        });
        setOpsMonitoringRows(result.items ?? []);
        setOpsMonitoringTotal(result.total ?? 0);
        setOpsMonitoringPage(result.page ?? nextPage);
      } catch (loadErr) {
        setOpsMonitoringError(
          loadErr instanceof Error
            ? loadErr.message
            : "Gagal memuat monitoring operasional sekolah."
        );
      } finally {
        setIsOpsMonitoringLoading(false);
      }
    },
    [showOpsMonitoring, debouncedOpsMonitoringQuery, opsMonitoringPageSize]
  );

  useEffect(() => {
    if (status !== "authenticated") return;
    if (isSchoolAdmin) {
      fetchSchoolSettings()
        .then((data) => {
          setInitialValues(data.school);
          setFormValues({ ...data.school });
          setUserProfile(data.user);
          setAccountValues({
            username: data.user.username,
            currentPassword: "",
            newPassword: "",
            confirmPassword: "",
          });
        })
        .catch(() => {
          setError("Gagal memuat data sekolah.");
        });
    }
    if (isAdmin) {
      void loadAdminSchools();
    }
    if (isAdminMaster) {
      void loadCabdinAdmins();
      void loadLegacySettings();
      void loadLegacyPtMigration(false);
      void loadPengawasAccounts();
    }
    if (isAdminCabdin) {
      void loadCabdinLegacySettings();
    }
    if (isPengawas) {
      setIsSupervisedSchoolsLoading(true);
      setSupervisedSchoolsError("");
      void fetchSupervisedSchools()
        .then((items) => {
          setSupervisedSchools(items);
        })
        .catch((loadErr) => {
          setSupervisedSchoolsError(
            loadErr instanceof Error
              ? loadErr.message
              : "Gagal memuat sekolah yang diampu."
          );
        })
        .finally(() => {
          setIsSupervisedSchoolsLoading(false);
        });
    }
  }, [
    status,
    isSchoolAdmin,
    isAdmin,
    isAdminMaster,
    isAdminCabdin,
    isPengawas,
    loadAdminSchools,
    loadCabdinAdmins,
    loadLegacySettings,
    loadLegacyPtMigration,
    loadCabdinLegacySettings,
    loadPengawasAccounts,
  ]);

  useEffect(() => {
    if (status !== "authenticated" || !isAdmin) {
      return;
    }
    const intervalId = window.setInterval(() => {
      loadAdminAccounts({ page: adminPage });
    }, 60000);
    return () => window.clearInterval(intervalId);
  }, [status, isAdmin, loadAdminAccounts, adminPage]);

  useEffect(() => {
    if (status !== "authenticated" || !showOpsMonitoring) {
      return;
    }
    void loadOpsMonitoringSchools({ page: 1 });
  }, [status, showOpsMonitoring, loadOpsMonitoringSchools]);

  useEffect(() => {
    if (status === "unauthenticated") {
      router.replace("/login");
    }
  }, [status, router]);

  useEffect(() => {
    if (!isAdminMaster) return;
    const query = debouncedResetSchoolQuery.trim();
    if (query.length < 2) {
      setResetSchoolSuggestions([]);
      setIsResetSchoolLoading(false);
      return;
    }
    let isActive = true;
    setIsResetSchoolLoading(true);
    fetchSchoolSuggestions(query)
      .then((data) => {
        if (!isActive) return;
        setResetSchoolSuggestions(data);
      })
      .catch(() => {
        if (!isActive) return;
        setResetSchoolSuggestions([]);
      })
      .finally(() => {
        if (!isActive) return;
        setIsResetSchoolLoading(false);
      });
    return () => {
      isActive = false;
    };
  }, [debouncedResetSchoolQuery, isAdminMaster]);

  useEffect(() => {
    if (!isAdmin) return;
    const query = debouncedAdminSchoolQuery.trim();
    if (query.length < 2) {
      setAdminSchoolSuggestions([]);
      setIsAdminSchoolLoading(false);
      return;
    }
    let isActive = true;
    setIsAdminSchoolLoading(true);
    fetchSchoolSuggestions(query)
      .then((data) => {
        if (!isActive) return;
        setAdminSchoolSuggestions(data);
      })
      .catch(() => {
        if (!isActive) return;
        setAdminSchoolSuggestions([]);
      })
      .finally(() => {
        if (!isActive) return;
        setIsAdminSchoolLoading(false);
      });
    return () => {
      isActive = false;
    };
  }, [debouncedAdminSchoolQuery, isAdmin]);

  useEffect(() => {
    if (!isAdminMaster || !isPengawasCreateOpen) return;
    const query = debouncedPengawasCreateSchoolQuery.trim();
    if (query.length < 2) {
      setPengawasCreateSchoolSuggestions([]);
      setIsPengawasCreateSchoolLoading(false);
      return;
    }
    let isActive = true;
    setIsPengawasCreateSchoolLoading(true);
    fetchSchoolSuggestions(query)
      .then((data) => {
        if (!isActive) return;
        setPengawasCreateSchoolSuggestions(data);
      })
      .catch(() => {
        if (!isActive) return;
        setPengawasCreateSchoolSuggestions([]);
      })
      .finally(() => {
        if (!isActive) return;
        setIsPengawasCreateSchoolLoading(false);
      });
    return () => {
      isActive = false;
    };
  }, [debouncedPengawasCreateSchoolQuery, isAdminMaster, isPengawasCreateOpen]);

  useEffect(() => {
    if (!isAdminMaster || !isPengawasSchoolsOpen) return;
    const query = debouncedPengawasSchoolsQuery.trim();
    if (query.length < 2) {
      setPengawasSchoolsSuggestions([]);
      setIsPengawasSchoolsLoading(false);
      return;
    }
    let isActive = true;
    setIsPengawasSchoolsLoading(true);
    fetchSchoolSuggestions(query)
      .then((data) => {
        if (!isActive) return;
        setPengawasSchoolsSuggestions(data);
      })
      .catch(() => {
        if (!isActive) return;
        setPengawasSchoolsSuggestions([]);
      })
      .finally(() => {
        if (!isActive) return;
        setIsPengawasSchoolsLoading(false);
      });
    return () => {
      isActive = false;
    };
  }, [debouncedPengawasSchoolsQuery, isAdminMaster, isPengawasSchoolsOpen]);

  useEffect(() => {
    if (!isAdmin) return;
    loadAdminAccounts();
  }, [isAdmin, loadAdminAccounts]);

  const adminSchoolItems = useMemo(
    () =>
      adminSchoolSuggestions.map((school) => ({
        key: school.id,
        label: `${school.nama_sekolah}${school.npsn ? ` - ${school.npsn}` : ""}`,
        value: school.nama_sekolah,
      })),
    [adminSchoolSuggestions]
  );

  const pengawasCreateSchoolItems = useMemo(
    () =>
      pengawasCreateSchoolSuggestions.map((school) => ({
        key: school.id,
        label: `${school.nama_sekolah}${school.npsn ? ` - ${school.npsn}` : ""}`,
        value: school.nama_sekolah,
      })),
    [pengawasCreateSchoolSuggestions]
  );

  const pengawasSchoolsItems = useMemo(
    () =>
      pengawasSchoolsSuggestions.map((school) => ({
        key: school.id,
        label: `${school.nama_sekolah}${school.npsn ? ` - ${school.npsn}` : ""}`,
        value: school.nama_sekolah,
      })),
    [pengawasSchoolsSuggestions]
  );

  const schoolOptionByID = useMemo(() => {
    const map = new Map<number, SchoolOption>();
    schoolOptions.forEach((school) => {
      map.set(school.id, school);
    });
    return map;
  }, [schoolOptions]);

  const handleFieldChange = (key: keyof SchoolSettings, value: string) => {
    if (!formValues) return;
    if (!isEditing) return;
    setFormValues({
      ...formValues,
      [key]: value,
    } as SchoolSettings);
  };

  const handleNumberChange = (key: keyof SchoolSettings, value: string) => {
    if (!formValues) return;
    if (!isEditing) return;
    const parsed = Number(value);
    setFormValues({
      ...formValues,
      [key]: Number.isFinite(parsed) ? parsed : 0,
    } as SchoolSettings);
  };

  const handleSave = async () => {
    if (!formValues || !userProfile) return;
    const accountChange =
      accountValues.username.trim() !== userProfile.username ||
      accountValues.newPassword.trim() !== "";
    const didChangePassword =
      accountChange && accountValues.newPassword.trim() !== "";
    if (accountChange) {
      if (!accountValues.currentPassword.trim()) {
        setError("Password lama wajib diisi untuk perubahan akun.");
        return;
      }
      if (accountValues.newPassword.trim()) {
        if (accountValues.newPassword.trim().length < 8) {
          setError("Password baru minimal 8 karakter.");
          return;
        }
        if (
          accountValues.newPassword.trim() !==
          accountValues.confirmPassword.trim()
        ) {
          setError("Konfirmasi password baru tidak sama.");
          return;
        }
      }
    }

    setIsSaving(true);
    setMessage("");
    setError("");
    try {
      const payload: {
        school: SchoolSettings;
        user?: {
          username?: string;
          current_password?: string;
          new_password?: string;
        };
      } = {
        school: formValues,
      };

      if (accountChange) {
        payload.user = {
          username: accountValues.username.trim(),
          current_password: accountValues.currentPassword.trim(),
        };
        if (accountValues.newPassword.trim()) {
          payload.user.new_password = accountValues.newPassword.trim();
        }
      }

      const updated = await updateSchoolSettings(payload);
      setInitialValues(updated.school);
      setFormValues({ ...updated.school });
      setUserProfile(updated.user);
      setAccountValues({
        username: updated.user.username,
        currentPassword: "",
        newPassword: "",
        confirmPassword: "",
      });
      setMessage("Berhasil disimpan");
      setIsEditing(false);
      if (didChangePassword) {
        if (typeof window !== "undefined") {
          window.localStorage.setItem(
            AUTH_MESSAGE_KEY,
            "Password berhasil diubah. Silakan login ulang."
          );
        }
        logout();
        router.replace("/login");
        return;
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan data.");
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancel = () => {
    if (!initialValues || !userProfile) return;
    setFormValues({ ...initialValues });
    setAccountValues({
      username: userProfile.username,
      currentPassword: "",
      newPassword: "",
      confirmPassword: "",
    });
    setIsEditing(false);
    setMessage("");
    setError("");
  };

  const openAdminModal = (account: SchoolAdminAccount) => {
    setSelectedAdmin(account);
    setEditUsername(account.username);
    setAdminMessage("");
    setIsModalOpen(true);
  };

  const openResetModal = (account: SchoolAdminAccount) => {
    setSelectedAdmin(account);
    setAdminPassword("");
    setNewAdminPassword("");
    setConfirmAdminPassword("");
    setResetError("");
    setAdminError("");
    setAdminMessage("");
    setIsResetModalOpen(true);
  };

  const handleAdminSave = async () => {
    if (!selectedAdmin) return;
    setAdminError("");
    try {
      await updateSchoolAdmin(selectedAdmin.id, { username: editUsername });
      loadAdminAccounts();
      setIsModalOpen(false);
      setAdminMessage("Perubahan akun admin sekolah berhasil disimpan.");
    } catch (err) {
      setAdminError(err instanceof Error ? err.message : "Gagal update akun.");
    }
  };

  const handleResetPassword = async () => {
    if (!selectedAdmin) return;
    if (!adminPassword.trim()) {
      setResetError("Password admin wajib diisi.");
      return;
    }
    if (newAdminPassword.trim().length < 8) {
      setResetError("Password baru minimal 8 karakter.");
      return;
    }
    if (newAdminPassword.trim() !== confirmAdminPassword.trim()) {
      setResetError("Konfirmasi password baru tidak sama.");
      return;
    }

    setResetError("");
    setIsResetSaving(true);
    try {
      await resetSchoolAdminPassword(selectedAdmin.id, {
        adminPassword: adminPassword.trim(),
        newPassword: newAdminPassword.trim(),
      });
      setIsResetModalOpen(false);
      setAdminMessage(
        "Password admin sekolah berhasil diubah. Akun tersebut akan otomatis logout."
      );
      loadAdminAccounts();
    } catch (err) {
      setResetError(
        err instanceof Error ? err.message : "Gagal reset password."
      );
    } finally {
      setIsResetSaving(false);
    }
  };

  const copyToClipboard = async (text: string) => {
    if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return;
    }
    if (typeof document === "undefined") return;
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.left = "-9999px";
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    document.execCommand("copy");
    document.body.removeChild(textarea);
  };

  const handleCopyUsername = async (username: string) => {
    try {
      await copyToClipboard(username);
      setAdminMessage("Username berhasil disalin.");
    } catch {
      setAdminError("Gagal menyalin username.");
    }
  };

  const handleCopyCabdinUsername = async (username: string) => {
    try {
      await copyToClipboard(username);
      setCabdinMessage("Username berhasil disalin.");
    } catch {
      setCabdinError("Gagal menyalin username.");
    }
  };

  const addCreatePengawasSchool = (schoolID: number) => {
    setPengawasCreateSchoolIDs((prev) =>
      prev.includes(schoolID) ? prev : [...prev, schoolID]
    );
  };

  const removeCreatePengawasSchool = (schoolID: number) => {
    setPengawasCreateSchoolIDs((prev) => prev.filter((id) => id !== schoolID));
  };

  const handleCreatePengawas = async () => {
    const username = pengawasCreateUsername.trim();
    const password = pengawasCreatePassword.trim();
    const name = pengawasCreateName.trim();
    if (!username || !password) {
      setPengawasCreateError("Username dan password wajib diisi.");
      return;
    }
    if (password.length < 8) {
      setPengawasCreateError("Password minimal 8 karakter.");
      return;
    }

    setPengawasCreateError("");
    setIsPengawasCreating(true);
    try {
      await createPengawasAccount({
        username,
        password,
        name: name || undefined,
        school_ids: pengawasCreateSchoolIDs,
      });
      setPengawasMessage("Akun pengawas berhasil dibuat.");
      setPengawasCreateName("");
      setPengawasCreateUsername("");
      setPengawasCreatePassword("");
      setPengawasCreateSchoolQuery("");
      setPengawasCreateSchoolIDs([]);
      setIsPengawasCreateOpen(false);
      await loadPengawasAccounts();
      await loadAdminSchools();
    } catch (createErr) {
      setPengawasCreateError(
        createErr instanceof Error
          ? createErr.message
          : "Gagal membuat akun pengawas."
      );
    } finally {
      setIsPengawasCreating(false);
    }
  };

  const openPengawasEditModal = (account: PengawasAccount) => {
    setPengawasEditTarget(account);
    setPengawasEditName(account.name ?? "");
    setPengawasEditUsername(account.username ?? "");
    setPengawasEditError("");
    setIsPengawasEditOpen(true);
  };

  const handleSavePengawasAccount = async () => {
    if (!pengawasEditTarget) return;
    const payload: { username?: string; name?: string } = {};
    const username = pengawasEditUsername.trim();
    const name = pengawasEditName.trim();
    if (!username) {
      setPengawasEditError("Username wajib diisi.");
      return;
    }
    payload.username = username;
    payload.name = name || username;

    setIsPengawasEditSaving(true);
    setPengawasEditError("");
    try {
      await updatePengawasAccount(pengawasEditTarget.id, payload);
      await loadPengawasAccounts();
      setPengawasMessage("Akun pengawas berhasil diperbarui.");
      setIsPengawasEditOpen(false);
      setPengawasEditTarget(null);
    } catch (err) {
      setPengawasEditError(
        err instanceof Error ? err.message : "Gagal memperbarui akun pengawas."
      );
    } finally {
      setIsPengawasEditSaving(false);
    }
  };

  const openPengawasResetModal = (account: PengawasAccount) => {
    setPengawasResetTarget(account);
    setPengawasResetPassword("");
    setPengawasResetConfirm("");
    setPengawasResetError("");
    setIsPengawasResetOpen(true);
  };

  const handleResetPengawasPassword = async () => {
    if (!pengawasResetTarget) return;
    const password = pengawasResetPassword.trim();
    const confirm = pengawasResetConfirm.trim();
    if (password.length < 8) {
      setPengawasResetError("Password minimal 8 karakter.");
      return;
    }
    if (password !== confirm) {
      setPengawasResetError("Konfirmasi password tidak sama.");
      return;
    }

    setIsPengawasResetSaving(true);
    setPengawasResetError("");
    try {
      await resetPengawasPassword(pengawasResetTarget.id, password);
      setPengawasMessage("Password pengawas berhasil direset.");
      setIsPengawasResetOpen(false);
      setPengawasResetTarget(null);
    } catch (err) {
      setPengawasResetError(
        err instanceof Error ? err.message : "Gagal reset password pengawas."
      );
    } finally {
      setIsPengawasResetSaving(false);
    }
  };

  const openPengawasSchoolsModal = (account: PengawasAccount) => {
    setPengawasSchoolsTarget(account);
    setPengawasSchoolsQuery("");
    setPengawasSchoolsError("");
    setPengawasSchoolsSelectedIDs(
      (account.schools ?? []).map((school) => school.id)
    );
    setIsPengawasSchoolsOpen(true);
  };

  const addPengawasSchoolSelection = (schoolID: number) => {
    setPengawasSchoolsSelectedIDs((prev) =>
      prev.includes(schoolID) ? prev : [...prev, schoolID]
    );
  };

  const removePengawasSchoolSelection = (schoolID: number) => {
    setPengawasSchoolsSelectedIDs((prev) =>
      prev.filter((id) => id !== schoolID)
    );
  };

  const handleSavePengawasSchools = async () => {
    if (!pengawasSchoolsTarget) return;
    setIsPengawasSchoolsSaving(true);
    setPengawasSchoolsError("");
    try {
      await updatePengawasSchools(
        pengawasSchoolsTarget.id,
        pengawasSchoolsSelectedIDs
      );
      setPengawasMessage("Sekolah ampu pengawas berhasil diperbarui.");
      setIsPengawasSchoolsOpen(false);
      setPengawasSchoolsTarget(null);
      await loadPengawasAccounts();
      await loadAdminSchools();
    } catch (err) {
      setPengawasSchoolsError(
        err instanceof Error
          ? err.message
          : "Gagal memperbarui sekolah ampu pengawas."
      );
    } finally {
      setIsPengawasSchoolsSaving(false);
    }
  };

  const openRevealPasswordModal = (account: SchoolAdminAccount) => {
    if (!account.hasInitialPassword) {
      setAdminError("Password tidak tersedia.");
      return;
    }
    setRevealTarget(account);
    setRevealAdminPassword("");
    setRevealedPassword(null);
    setRevealError("");
    setIsRevealPasswordOpen(true);
  };

  const handleRevealPassword = async () => {
    if (!revealTarget) return;
    const trimmed = revealAdminPassword.trim();
    if (!trimmed) {
      setRevealError("Password admin wajib diisi.");
      return;
    }

    setRevealError("");
    setIsRevealing(true);
    setRevealedPassword(null);
    try {
      const payload = await revealSchoolAdminPassword(revealTarget.id, trimmed);
      setRevealedPassword(payload.password);
    } catch (err) {
      setRevealError(
        err instanceof Error ? err.message : "Gagal memuat password."
      );
    } finally {
      setIsRevealing(false);
      setRevealAdminPassword("");
    }
  };

  const handleCopyRevealedPassword = async () => {
    if (!revealedPassword) return;
    try {
      await copyToClipboard(revealedPassword);
      setAdminMessage("Password berhasil disalin.");
    } catch {
      setAdminError("Gagal menyalin password.");
    }
  };

  const toggleCabdinLevel = (
    value: string,
    setter: Dispatch<SetStateAction<string[]>>
  ) => {
    setter((prev) =>
      prev.includes(value)
        ? prev.filter((level) => level !== value)
        : [...prev, value]
    );
  };

  const openCabdinCreateModal = () => {
    setCabdinCreateName("");
    setCabdinCreateUsername("");
    setCabdinCreatePassword("");
    setCabdinCreateLevels([...jenjangOptions]);
    setCabdinCreateError("");
    setCabdinMessage("");
    setIsCabdinCreateOpen(true);
  };

  const openCabdinEditModal = (account: CabdinAdminAccount) => {
    setCabdinEditTarget(account);
    setCabdinEditLevels(
      Array.isArray(account.levels) ? [...account.levels] : []
    );
    setCabdinEditError("");
    setCabdinMessage("");
    setIsCabdinEditOpen(true);
  };

  const openCabdinAccountEditModal = (account: CabdinAdminAccount) => {
    setCabdinAccountEditTarget(account);
    setCabdinAccountEditName(account.name ?? "");
    setCabdinAccountEditUsername(account.username ?? "");
    setCabdinAccountEditError("");
    setCabdinMessage("");
    setIsCabdinAccountEditOpen(true);
  };

  const openCabdinResetPasswordModal = (account: CabdinAdminAccount) => {
    setCabdinPasswordResetTarget(account);
    setCabdinPasswordResetValue("");
    setCabdinPasswordResetConfirm("");
    setCabdinPasswordResetError("");
    setCabdinMessage("");
    setIsCabdinPasswordResetOpen(true);
  };

  const handleCreateCabdinAdmin = async () => {
    const name = cabdinCreateName.trim();
    const username = cabdinCreateUsername.trim();
    const password = cabdinCreatePassword.trim();
    if (!name || !username || !password) {
      setCabdinCreateError("Lengkapi data terlebih dahulu.");
      return;
    }
    if (password.length < 8) {
      setCabdinCreateError("Password minimal 8 karakter.");
      return;
    }
    if (cabdinCreateLevels.length === 0) {
      setCabdinCreateError("Pilih minimal satu jenjang.");
      return;
    }
    setCabdinCreateError("");
    setIsCabdinSaving(true);
    try {
      await createCabdinAdmin({
        name,
        username,
        password,
        levels: cabdinCreateLevels,
      });
      setIsCabdinCreateOpen(false);
      setCabdinMessage("Admin cabdin berhasil dibuat.");
      await loadCabdinAdmins();
    } catch (err) {
      setCabdinCreateError(
        err instanceof Error ? err.message : "Gagal membuat admin cabdin."
      );
    } finally {
      setIsCabdinSaving(false);
    }
  };

  const handleSaveCabdinLevels = async () => {
    if (!cabdinEditTarget) return;
    if (cabdinEditLevels.length === 0) {
      setCabdinEditError("Pilih minimal satu jenjang.");
      return;
    }
    setCabdinEditError("");
    setIsCabdinSaving(true);
    try {
      await updateCabdinAdminLevels(cabdinEditTarget.id, cabdinEditLevels);
      setIsCabdinEditOpen(false);
      setCabdinMessage("Jenjang admin cabdin berhasil diperbarui.");
      await loadCabdinAdmins();
    } catch (err) {
      setCabdinEditError(
        err instanceof Error
          ? err.message
          : "Gagal mengubah jenjang admin cabdin."
      );
    } finally {
      setIsCabdinSaving(false);
    }
  };

  const toggleLegacyOverride = (level: string) => {
    setLegacyLevelOverrides((prev) => {
      const current = prev[level] ?? { override: false, enabled: true };
      return {
        ...prev,
        [level]: { ...current, override: !current.override },
      };
    });
  };

  const toggleLegacyEnabled = (level: string) => {
    setLegacyLevelOverrides((prev) => {
      const current = prev[level] ?? { override: false, enabled: true };
      return {
        ...prev,
        [level]: { ...current, enabled: !current.enabled },
      };
    });
  };

  const handleSaveLegacySettings = async () => {
    setLegacySettingsError("");
    setLegacySettingsMessage("");
    const autoDisableYear = Number(legacyAutoDisableYear);
    const maxYear = Number(legacyMaxYear);
    if (!Number.isFinite(autoDisableYear) || autoDisableYear <= 0) {
      setLegacySettingsError("Tahun auto-disable tidak valid.");
      return;
    }
    if (!Number.isFinite(maxYear) || maxYear <= 0) {
      setLegacySettingsError("Tahun maksimal tidak valid.");
      return;
    }

    setIsLegacySettingsSaving(true);
    try {
      await updateLegacyImportSettings({
        globalEnabled: legacyGlobalEnabled,
        autoDisableYear,
        forceEnabled: legacyForceEnabled,
        maxYear,
      });
      setLegacySettingsMessage("Pengaturan legacy import berhasil diperbarui.");
    } catch (err) {
      setLegacySettingsError(
        err instanceof Error
          ? err.message
          : "Gagal memperbarui pengaturan legacy import."
      );
    } finally {
      setIsLegacySettingsSaving(false);
    }
  };

  const handleSaveLegacyLevels = async () => {
    setLegacySettingsError("");
    setLegacySettingsMessage("");
    const levelsPayload = jenjangOptions.map((level) => {
      const state = legacyLevelOverrides[level] ?? {
        override: false,
        enabled: true,
      };
      return {
        level,
        override: state.override,
        enabled: state.enabled,
      };
    });
    setIsLegacyLevelsSaving(true);
    try {
      await updateLegacyImportLevelsMaster(levelsPayload);
      setLegacySettingsMessage(
        "Pengaturan jenjang legacy berhasil diperbarui."
      );
      await loadLegacySettings();
    } catch (err) {
      setLegacySettingsError(
        err instanceof Error
          ? err.message
          : "Gagal memperbarui pengaturan jenjang legacy."
      );
    } finally {
      setIsLegacyLevelsSaving(false);
    }
  };

  const handleRunLegacyPtMigration = async () => {
    setLegacyPtError("");
    setLegacyPtMessage("");
    setIsLegacyPtRunning(true);
    try {
      const status = await fetchLegacyPTMigrationStatus(true);
      setLegacyPtStatus(status);
      if (status.status === "done") {
        setLegacyPtMessage("Migrasi legacy PT selesai dijalankan.");
      }
      await loadLegacyPtMigration(false);
    } catch {
      setLegacyPtError("Gagal menjalankan migrasi legacy PT.");
    } finally {
      setIsLegacyPtRunning(false);
    }
  };

  const handleSaveCabdinAccount = async () => {
    if (!cabdinAccountEditTarget) return;
    const parsed = cabdinAccountEditSchema.safeParse({
      name: cabdinAccountEditName,
      username: cabdinAccountEditUsername,
    });
    if (!parsed.success) {
      setCabdinAccountEditError(
        parsed.error.issues[0]?.message ?? "Data tidak valid."
      );
      return;
    }

    setCabdinAccountEditError("");
    setIsCabdinAccountSaving(true);
    try {
      await updateCabdinAdminAccount(cabdinAccountEditTarget.id, {
        name: parsed.data.name,
        username: parsed.data.username,
      });
      setIsCabdinAccountEditOpen(false);
      setCabdinAccountEditTarget(null);
      setCabdinMessage("Akun admin cabdin berhasil diperbarui.");
      await loadCabdinAdmins();
    } catch (err) {
      setCabdinAccountEditError(
        err instanceof Error
          ? err.message
          : "Gagal memperbarui akun admin cabdin."
      );
    } finally {
      setIsCabdinAccountSaving(false);
    }
  };

  const handleResetCabdinPassword = async () => {
    if (!cabdinPasswordResetTarget) return;
    const parsed = cabdinPasswordResetSchema.safeParse({
      newPassword: cabdinPasswordResetValue,
      confirmPassword: cabdinPasswordResetConfirm,
    });
    if (!parsed.success) {
      setCabdinPasswordResetError(
        parsed.error.issues[0]?.message ?? "Data tidak valid."
      );
      return;
    }

    setCabdinPasswordResetError("");
    setIsCabdinPasswordResetSaving(true);
    try {
      await resetCabdinAdminPassword(
        cabdinPasswordResetTarget.id,
        parsed.data.newPassword
      );
      setIsCabdinPasswordResetOpen(false);
      setCabdinPasswordResetTarget(null);
      setCabdinMessage("Password berhasil direset.");
      await loadCabdinAdmins();
    } catch (err) {
      setCabdinPasswordResetError(
        err instanceof Error
          ? err.message
          : "Gagal reset password admin cabdin."
      );
    } finally {
      setIsCabdinPasswordResetSaving(false);
    }
  };

  const handleDownloadLegacyPtReport = async () => {
    setLegacyPtError("");
    setLegacyPtMessage("");
    try {
      const { blob, filename } = await downloadLegacyPTMigrationReport();
      triggerDownload(blob, filename);
      setLegacyPtMessage("Laporan migrasi berhasil diunduh.");
    } catch (err) {
      setLegacyPtError(
        err instanceof Error ? err.message : "Gagal mengunduh laporan migrasi."
      );
    }
  };

  const toggleCabdinLegacyLevel = (level: string, current: boolean) => {
    setCabdinLegacyDraft((prev) => ({
      ...prev,
      [level]: !current,
    }));
  };

  const handleSaveCabdinLegacyLevels = async () => {
    setCabdinLegacyError("");
    setCabdinLegacyMessage("");
    const levelsPayload = cabdinLegacyLevels
      .filter((item) => !item.locked)
      .map((item) => ({
        level: item.level,
        enabled: cabdinLegacyDraft[item.level] ?? item.enabled,
      }));
    if (levelsPayload.length === 0) {
      setCabdinLegacyError("Tidak ada jenjang yang bisa diperbarui.");
      return;
    }
    setIsCabdinLegacySaving(true);
    try {
      await updateCabdinLegacyLevels(levelsPayload);
      setCabdinLegacyMessage("Pengaturan legacy import berhasil diperbarui.");
      await loadCabdinLegacySettings();
    } catch (err) {
      setCabdinLegacyError(
        err instanceof Error
          ? err.message
          : "Gagal memperbarui pengaturan legacy import."
      );
    } finally {
      setIsCabdinLegacySaving(false);
    }
  };
  /*
  const getFilenameFromContentDisposition = (
    header: string | null
  ): string | null => {
    if (!header) return null;
    const utf8Match = header.match(/filename\*\s*=\s*UTF-8''([^;]+)/i);
    if (utf8Match?.[1]) {
      try {
        return decodeURIComponent(utf8Match[1].trim());
      } catch {
        return utf8Match[1].trim();
      }
    }
    const quotedMatch = header.match(/filename\s*=\s*\"([^\"]+)\"/i);
    if (quotedMatch?.[1]) return quotedMatch[1].trim();
    const plainMatch = header.match(/filename\s*=\s*([^;]+)/i);
    if (plainMatch?.[1]) return plainMatch[1].trim();
    return null;
  };*/

  const triggerDownload = (blob: Blob, filename: string) => {
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  const openExportModal = () => {
    setExportAdminPassword("");
    setExportError("");
    setIsExportOpen(true);
  };

  const handleExportAccounts = async () => {
    const trimmed = exportAdminPassword.trim();
    if (!trimmed) {
      setExportError("Password admin wajib diisi.");
      return;
    }

    setExportError("");
    setIsExporting(true);
    try {
      const response = await fetch("/api/admin/school-admins/export", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ adminPassword: trimmed }),
      });

      if (response.status === 401) {
        setExportError("Password admin salah.");
        return;
      }
      if (response.status === 403) {
        setExportError("Tidak punya akses.");
        return;
      }
      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as {
          message?: string;
        };
        setExportError(body?.message || "Gagal export akun.");
        return;
      }

      const blob = await response.blob();
      const filename =
        getFilenameFromContentDisposition(
          response.headers.get("content-disposition")
        ) ?? "akun-admin-sekolah.csv";
      triggerDownload(blob, filename);
      setAdminMessage("File berhasil diunduh.");
      setIsExportOpen(false);
    } catch (err) {
      setExportError(err instanceof Error ? err.message : "Gagal export akun.");
    } finally {
      setIsExporting(false);
      setExportAdminPassword("");
    }
  };

  const handleBulkGenerate = async () => {
    setBulkError("");
    setBulkMessage("");
    setBulkSummary(null);
    setIsBulkGenerating(true);
    try {
      const { blob, filename } = await downloadGeneratedSchoolAdmins({
        jenjang: bulkJenjang,
        status: bulkStatus,
      });
      triggerDownload(blob, filename);
      setBulkMessage("Generate akun admin sekolah berhasil.");
      loadAdminAccounts();
    } catch (err) {
      setBulkError(err instanceof Error ? err.message : "Gagal generate akun.");
    } finally {
      setIsBulkGenerating(false);
    }
  };

  const openResetDataModal = () => {
    setResetDataError("");
    setResetMessage("");
    setResetModalOpen(true);
  };

  const openChangePasswordModal = () => {
    setOldAdminPassword("");
    setNewAdminPasswordValue("");
    setConfirmAdminPasswordValue("");
    setChangePasswordError("");
    setIsChangePasswordOpen(true);
  };

  const handleChangePassword = async () => {
    const trimmedOld = oldAdminPassword.trim();
    const trimmedNew = newAdminPasswordValue.trim();
    if (!trimmedOld) {
      setChangePasswordError("Password lama wajib diisi.");
      return;
    }
    if (trimmedNew.length < 8) {
      setChangePasswordError("Password baru minimal 8 karakter.");
      return;
    }
    if (trimmedNew === trimmedOld) {
      setChangePasswordError(
        "Password baru harus berbeda dengan password lama."
      );
      return;
    }
    if (trimmedNew !== confirmAdminPasswordValue.trim()) {
      setChangePasswordError("Konfirmasi password baru tidak sama.");
      return;
    }

    setChangePasswordError("");
    setIsChangePasswordSaving(true);
    try {
      await changeSelfPassword({
        oldPassword: trimmedOld,
        newPassword: trimmedNew,
      });
      setIsChangePasswordOpen(false);
      if (typeof window !== "undefined") {
        window.localStorage.setItem(
          AUTH_MESSAGE_KEY,
          "Password berhasil diubah. Silakan login ulang."
        );
      }
      logout();
      router.replace("/login");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Gagal mengubah password.";
      if (message === "OLD_PASSWORD_INVALID") {
        setChangePasswordError("Password lama salah. Silakan periksa kembali.");
        setNewAdminPasswordValue("");
        setConfirmAdminPasswordValue("");
        return;
      }
      if (message === "FORBIDDEN") {
        setChangePasswordError("Anda tidak memiliki akses.");
        return;
      }
      if (message === "PASSWORD_MIN_8") {
        setChangePasswordError("Password baru minimal 8 karakter.");
        return;
      }
      if (message === "TOO_MANY_REQUESTS") {
        setChangePasswordError(
          "Terlalu banyak percobaan. Coba lagi beberapa saat."
        );
        return;
      }
      setChangePasswordError("Gagal mengubah password.");
    } finally {
      setIsChangePasswordSaving(false);
    }
  };

  const handleConfirmResetData = async () => {
    if (!resetSchoolId) {
      setResetDataError("Pilih sekolah terlebih dahulu.");
      return;
    }
    if (!resetDeleteAchievement && !resetDeleteAdmissions) {
      setResetDataError("Minimal pilih satu jenis data yang dihapus.");
      return;
    }
    if (resetDeleteAdmissions && resetTahunLulus === null && !resetOverride) {
      setResetDataError(
        "Reset semua tahun berisiko. Pilih Tahun Lulus atau aktifkan Override."
      );
      return;
    }

    setResetDataError("");
    setIsResetDataSaving(true);
    try {
      const result = await resetSchoolData({
        school_id: resetSchoolId,
        tahun_lulus: resetTahunLulus,
        reset_prestasi: resetDeleteAchievement,
        reset_siswa: resetDeleteAdmissions,
        override: resetOverride,
      });
      const formatter = new Intl.NumberFormat("id-ID");
      const deleted = result.deleted;
      const siswaDeletedTotal =
        deleted.class12_students +
        deleted.student_plans +
        deleted.student_outcomes +
        deleted.outcome_pt_details +
        deleted.outcome_work_details +
        deleted.outcome_business_details +
        deleted.applications +
        deleted.acceptance_offers +
        deleted.wirausaha +
        deleted.student_cohorts +
        deleted.student_final_outcomes;
      const deletedSummary = [
        resetDeleteAchievement
          ? `Prestasi: ${formatter.format(deleted.prestasi_siswa)} record`
          : null,
        resetDeleteAdmissions
          ? `Data Siswa/Outcome: ${formatter.format(siswaDeletedTotal)} record`
          : null,
      ]
        .filter(Boolean)
        .join(", ");
      const scopeLabel =
        result.tahun_lulus == null
          ? "semua tahun"
          : `tahun lulus ${result.tahun_lulus}`;
      setResetMessage(
        deletedSummary
          ? `Reset berhasil untuk ${scopeLabel}. ${deletedSummary}.`
          : "Data berhasil dihapus."
      );
      setResetDeleteAchievement(false);
      setResetDeleteAdmissions(false);
      setResetTahunLulus(null);
      setResetOverride(false);
      setResetModalOpen(false);
      loadAdminAccounts();
    } catch (err) {
      setResetDataError(
        err instanceof Error ? err.message : "Gagal menghapus data."
      );
    } finally {
      setIsResetDataSaving(false);
    }
  };

  const handleCreateAdmin = async () => {
    if (!selectedSchoolId) {
      setCreateError("Pilih sekolah terlebih dahulu.");
      return;
    }
    if (!createUsername.trim() || !createPassword.trim()) {
      setCreateError("Username dan password wajib diisi.");
      return;
    }
    if (createPassword.trim().length < 6) {
      setCreateError("Password minimal 6 karakter.");
      return;
    }

    setCreateError("");
    try {
      await createSchoolAdmin(selectedSchoolId, createUsername, createPassword);
      loadAdminAccounts();
      setCreateModalOpen(false);
      setCreateUsername("");
      setCreatePassword("");
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : "Gagal membuat akun."
      );
    }
  };

  const handleNextAdminPage = () => {
    if (isAdminLoadingMore || !adminHasMore) {
      return;
    }
    void loadAdminAccounts({ page: adminPage + 1 });
  };

  const handlePrevAdminPage = () => {
    if (isAdminLoadingMore || adminPage <= 1) {
      return;
    }
    void loadAdminAccounts({ page: adminPage - 1 });
  };

  const handleNextOpsMonitoringPage = () => {
    if (isOpsMonitoringLoading || !opsMonitoringHasMore) {
      return;
    }
    void loadOpsMonitoringSchools({ page: opsMonitoringPage + 1 });
  };

  const handlePrevOpsMonitoringPage = () => {
    if (isOpsMonitoringLoading || opsMonitoringPage <= 1) {
      return;
    }
    void loadOpsMonitoringSchools({ page: opsMonitoringPage - 1 });
  };

  const statusBadge = (isOnline: boolean) => (
    <div className="flex items-center gap-2">
      <span
        className={`h-2 w-2 rounded-full ${
          isOnline ? "bg-emerald-500" : "bg-zinc-900"
        }`}
      />
      <span
        className={`text-sm ${isOnline ? "text-emerald-700" : "text-zinc-900"}`}
      >
        {isOnline ? "online" : "offline"}
      </span>
    </div>
  );

  if (!isMounted || status === "loading") {
    return <p className="text-sm text-zinc-500">Memuat...</p>;
  }

  if (status === "unauthenticated") {
    return null;
  }

  if (isSchoolAdmin && (!formValues || !userProfile)) {
    return (
      <p className="text-sm text-zinc-500">Data sekolah belum tersedia.</p>
    );
  }

  const inputClass =
    "w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm";
  const readOnlyClass = isEditing
    ? ""
    : "pointer-events-none bg-zinc-100 text-zinc-500";
  const isResetAllYearsRisk =
    resetDeleteAdmissions && resetTahunLulus === null && !resetOverride;
  const canResetData =
    isAdminMaster &&
    Boolean(resetSchoolId) &&
    (resetDeleteAchievement || resetDeleteAdmissions) &&
    !isResetAllYearsRisk;
  const resetSchoolItems = resetSchoolSuggestions.map((school) => ({
    key: school.id,
    label: `${school.nama_sekolah}${school.npsn ? ` - ${school.npsn}` : ""}`,
    value: school.nama_sekolah,
  }));
  const formatLastUpdate = (value?: string | null) =>
    formatIndoDateTime(value) ?? "belum update";
  const renderSortIcon = (column: string) => {
    if (adminSortBy !== column) {
      return <ArrowUpDown className="h-3.5 w-3.5 text-zinc-400" />;
    }
    return adminSortOrder === "asc" ? (
      <ArrowUp className="h-3.5 w-3.5 text-zinc-600" />
    ) : (
      <ArrowDown className="h-3.5 w-3.5 text-zinc-600" />
    );
  };
  const toggleSort = (column: string) => {
    if (adminSortBy === column) {
      setAdminSortOrder((prev) => (prev === "asc" ? "desc" : "asc"));
      return;
    }
    setAdminSortBy(column);
    setAdminSortOrder("asc");
  };
  const renderCabdinLevels = (levels: string[]) => {
    if (!levels || levels.length === 0) {
      return <span className="text-xs text-zinc-500">-</span>;
    }
    const order = new Map(jenjangOptions.map((value, index) => [value, index]));
    const sorted = [...levels].sort((a, b) => {
      const ai = order.get(a as (typeof jenjangOptions)[number]) ?? 99;
      const bi = order.get(b as (typeof jenjangOptions)[number]) ?? 99;
      return ai - bi;
    });
    return (
      <div className="flex flex-wrap justify-center gap-2">
        {sorted.map((level) => (
          <span
            key={level}
            className="rounded-full border border-zinc-200 bg-zinc-50 px-2.5 py-1 text-xs font-semibold text-zinc-700"
          >
            {level}
          </span>
        ))}
      </div>
    );
  };
  const formatCount = (value: number | null | undefined) =>
    value == null ? "-" : new Intl.NumberFormat("id-ID").format(value);
  const isResetConfirmDisabled = isResetDataSaving || !canResetData;
  const trimmedOldPassword = oldAdminPassword.trim();
  const trimmedNewPassword = newAdminPasswordValue.trim();
  const trimmedConfirmPassword = confirmAdminPasswordValue.trim();
  const isChangePasswordValid =
    trimmedOldPassword !== "" &&
    trimmedNewPassword.length >= 8 &&
    trimmedNewPassword !== trimmedOldPassword &&
    trimmedNewPassword === trimmedConfirmPassword;
  const isChangePasswordDisabled =
    isChangePasswordSaving || !isChangePasswordValid;
  const adminHasTotal = adminTotalCount != null;
  const adminTotalPages = adminHasTotal
    ? Math.max(1, Math.ceil(adminTotalCount / adminPageSize))
    : Math.max(1, adminPage);
  const adminRangeStart =
    adminAccounts.length === 0 ? 0 : (adminPage - 1) * adminPageSize + 1;
  const adminRangeEnd =
    adminAccounts.length === 0
      ? 0
      : adminHasTotal
        ? Math.min(adminPage * adminPageSize, adminTotalCount)
        : (adminPage - 1) * adminPageSize + adminAccounts.length;
  const adminHasMore = adminHasTotal
    ? adminPage < adminTotalPages
    : adminAccounts.length === adminPageSize;
  const opsMonitoringTotalPages = Math.max(
    1,
    Math.ceil(opsMonitoringTotal / opsMonitoringPageSize)
  );
  const opsMonitoringRangeStart =
    opsMonitoringRows.length === 0
      ? 0
      : (opsMonitoringPage - 1) * opsMonitoringPageSize + 1;
  const opsMonitoringRangeEnd =
    opsMonitoringRows.length === 0
      ? 0
      : Math.min(opsMonitoringPage * opsMonitoringPageSize, opsMonitoringTotal);
  const opsMonitoringHasMore = opsMonitoringPage < opsMonitoringTotalPages;
  const opsMonitoringExportHref = (() => {
    const params = new URLSearchParams();
    const query = debouncedOpsMonitoringQuery.trim();
    if (query !== "") {
      params.set("q", query);
    }
    const serialized = params.toString();
    return `/api/user/me/ops-monitoring-schools/export.xlsx${
      serialized ? `?${serialized}` : ""
    }`;
  })();
  const renderUpdateBadge = (
    isUpdated: boolean,
    lastUpdate?: string | null
  ) => (
    <div className="flex flex-col items-center gap-1">
      <span
        className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ${
          isUpdated
            ? "bg-emerald-50 text-emerald-700"
            : "bg-amber-50 text-amber-700"
        }`}
      >
        {isUpdated ? "Sudah update" : "Belum update"}
      </span>
      <span className="text-xs text-zinc-500">
        {formatLastUpdate(lastUpdate)}
      </span>
    </div>
  );
  /*
  const legacyPtStatusLabel = useMemo(() => {
    switch (legacyPtStatus?.status) {
      case "done":
        return "Selesai";
      case "running":
        return "Sedang berjalan";
      case "blocked":
        return "Belum siap";
      case "failed":
        return "Gagal";
      case "not_started":
        return "Belum dijalankan";
      default:
        return "Belum dijalankan";
    }
  }, [legacyPtStatus?.status]);*/
  const legacyPtStatusLabel = (() => {
    switch (legacyPtStatus?.status) {
      case "done":
        return "Selesai";
      case "running":
        return "Sedang berjalan";
      case "blocked":
        return "Belum siap";
      case "failed":
        return "Gagal";
      case "not_started":
        return "Belum dijalankan";
      default:
        return "Belum dijalankan";
    }
  })();
  const legacyPtTimestamp =
    legacyPtStatus?.finishedAt ?? legacyPtStatus?.startedAt ?? null;
  const canRunLegacyPt =
    legacyPtStatus?.status !== "done" && legacyPtStatus?.status !== "running";
  const hasLegacyPtReport = Boolean(legacyPtStatus?.reportPath);

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Pengaturan Pengguna
        </p>
        <h1 className="text-2xl font-semibold">
          {isAdmin
            ? "Akun Admin Sekolah"
            : isGaspolSchoolUser
              ? "Pengaturan Akun GASPOL"
              : "Data Sekolah"}
        </h1>
      </header>

      {isGaspolSchoolUser ? (
        <Card>
          <CardContent className="space-y-3">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Akun GASPOL
              </p>
              <h2 className="text-lg font-semibold">
                Akses Operasional Khusus
              </h2>
              <p className="text-sm text-zinc-600">
                Akun ini dibatasi untuk operasional GASPOL dan perubahan
                password akun sendiri.
              </p>
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-3">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Username
                </p>
                <p className="mt-1 text-sm font-medium text-zinc-900">
                  {user?.username ?? "-"}
                </p>
              </div>
              <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-3">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Instansi
                </p>
                <p className="mt-1 text-sm font-medium text-zinc-900">
                  {user?.namaInstansi?.trim() || user?.username || "-"}
                </p>
              </div>
            </div>
            <div className="flex justify-end">
              <button
                type="button"
                onClick={() => router.push("/gaspol")}
                className="rounded-lg border border-emerald-300 bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-700 transition hover:bg-emerald-100"
              >
                Buka GASPOL
              </button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {isPengawas ? (
        <Card>
          <CardContent className="space-y-3">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Pengawas
              </p>
              <h2 className="text-lg font-semibold">Sekolah yang Diampu</h2>
            </div>
            {isSupervisedSchoolsLoading ? (
              <p className="text-sm text-zinc-500">Memuat daftar sekolah...</p>
            ) : null}
            {supervisedSchoolsError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {supervisedSchoolsError}
              </p>
            ) : null}
            {!isSupervisedSchoolsLoading &&
            !supervisedSchoolsError &&
            supervisedSchools.length === 0 ? (
              <p className="text-sm text-zinc-500">
                Belum ada sekolah yang ditetapkan untuk akun ini.
              </p>
            ) : null}
            {!isSupervisedSchoolsLoading &&
            !supervisedSchoolsError &&
            supervisedSchools.length > 0 ? (
              <div className="space-y-2">
                {supervisedSchools.map((school) => (
                  <div
                    key={school.id}
                    className="rounded-lg border border-zinc-200 bg-white px-3 py-2"
                  >
                    <p className="text-sm font-semibold text-zinc-900">
                      {school.namaSekolah}
                    </p>
                    <p className="text-xs text-zinc-600">
                      {(school.npsnSekolah ?? "-") +
                        " | " +
                        (school.bentukPendidikan ?? "-")}
                    </p>
                  </div>
                ))}
              </div>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {canUseSelfPassword ? (
        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Akun
              </p>
              <h2 className="text-lg font-semibold">Ubah Password</h2>
              <p className="text-sm text-zinc-600">
                Ganti password akun Anda sendiri.
              </p>
            </div>
            <div className="flex justify-end">
              <button
                type="button"
                onClick={openChangePasswordModal}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Ubah Password
              </button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {showOpsMonitoring ? (
        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Monitoring
              </p>
              <h2 className="text-lg font-semibold">
                Monitoring Operasional Sekolah
              </h2>
              <p className="text-sm text-zinc-600">
                Pantau update data dan progres operasional sekolah dalam scope
                Anda.
              </p>
            </div>

            <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
              <div className="w-full md:max-w-sm">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Cari Nama Sekolah
                </label>
                <input
                  type="text"
                  value={opsMonitoringQuery}
                  onChange={(event) => {
                    setOpsMonitoringQuery(event.target.value);
                    setOpsMonitoringPage(1);
                  }}
                  placeholder="Ketik nama sekolah..."
                  className="mt-1 w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="flex items-center justify-between gap-3 md:justify-end">
                <p className="text-xs text-zinc-500">
                  Tampilkan {opsMonitoringPageSize} sekolah per halaman.
                </p>
                <a
                  href={opsMonitoringExportHref}
                  className="inline-flex items-center rounded-lg border border-emerald-300 bg-emerald-50 px-3 py-2 text-xs font-semibold text-emerald-700 transition hover:bg-emerald-100"
                >
                  Download Excel
                </a>
              </div>
            </div>

            {isOpsMonitoringLoading ? (
              <p className="text-sm text-zinc-500">
                Memuat monitoring operasional sekolah...
              </p>
            ) : null}
            {opsMonitoringError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {opsMonitoringError}
              </p>
            ) : null}

            {!isOpsMonitoringLoading && !opsMonitoringError ? (
              <Table
                wrapperClassName="max-h-[420px] overflow-y-auto"
                className="min-w-[1100px] md:min-w-full"
              >
                <TableHeader>
                  <TableRow>
                    <TableHead className="sticky top-0 z-10 bg-white">
                      Nama Sekolah
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white text-center">
                      Jenjang
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white text-center">
                      Status Sekolah
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white text-center">
                      Update Data Prestasi
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white text-center">
                      Update Data Siswa PT / Rencana
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white text-center">
                      Status Lulusan (Final)
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white text-center">
                      Pending Validasi
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white text-center">
                      Aksi
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {opsMonitoringRows.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={8}
                        className="text-center text-sm text-zinc-500"
                      >
                        Tidak ada sekolah dalam scope.
                      </TableCell>
                    </TableRow>
                  ) : (
                    opsMonitoringRows.map((row) => (
                      <TableRow key={row.school_id}>
                        <TableCell className="font-semibold text-zinc-900">
                          {row.school_name}
                        </TableCell>
                        <TableCell className="text-center text-sm text-zinc-700">
                          {row.jenjang ?? "-"}
                        </TableCell>
                        <TableCell className="text-center text-sm text-zinc-700">
                          {row.status_sekolah ?? "-"}
                        </TableCell>
                        <TableCell className="text-center">
                          {renderUpdateBadge(
                            row.prestasi_updated,
                            row.prestasi_last_update
                          )}
                        </TableCell>
                        <TableCell className="text-center">
                          {renderUpdateBadge(
                            row.siswa_updated,
                            row.siswa_last_update
                          )}
                        </TableCell>
                        <TableCell className="text-center text-sm font-semibold text-zinc-900">
                          {formatCount(row.status_lulusan_final_count)}
                        </TableCell>
                        <TableCell className="text-center text-sm font-semibold text-zinc-900">
                          {formatCount(row.pending_validasi_count)}
                        </TableCell>
                        <TableCell className="text-center">
                          <button
                            type="button"
                            onClick={() =>
                              router.push(
                                `/siswa/status-lulusan/pt?tab=PT&school_id=${row.school_id}`
                              )
                            }
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Lihat detail
                          </button>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            ) : null}

            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-zinc-200 pt-3 text-sm text-zinc-600">
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handlePrevOpsMonitoringPage}
                  disabled={isOpsMonitoringLoading || opsMonitoringPage <= 1}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  Sebelumnya
                </button>
                <span className="text-zinc-700">
                  Halaman {opsMonitoringPage} dari {opsMonitoringTotalPages}
                </span>
              </div>
              <span>
                {opsMonitoringRows.length === 0
                  ? "Menampilkan 0 data"
                  : `Menampilkan ${opsMonitoringRangeStart}-${opsMonitoringRangeEnd} dari ${formatCount(
                      opsMonitoringTotal
                    )} data`}
              </span>
              <button
                type="button"
                onClick={handleNextOpsMonitoringPage}
                disabled={isOpsMonitoringLoading || !opsMonitoringHasMore}
                className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
              >
                Berikutnya
              </button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {isSchoolAdmin ? (
        <Card>
          <CardContent className="space-y-6">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  NPSN
                </label>
                <input
                  value={formValues?.npsnSekolah ?? ""}
                  onChange={(event) =>
                    handleFieldChange("npsnSekolah", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama Sekolah
                </label>
                <input
                  value={formValues?.namaSekolah ?? ""}
                  onChange={(event) =>
                    handleFieldChange("namaSekolah", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Bentuk Pendidikan
                </label>
                <input
                  value={formValues?.bentukPendidikan ?? ""}
                  onChange={(event) =>
                    handleFieldChange("bentukPendidikan", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Status Sekolah
                </label>
                <input
                  value={formValues?.statusSekolah ?? ""}
                  onChange={(event) =>
                    handleFieldChange("statusSekolah", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  TMT Akreditasi
                </label>
                <input
                  value={formValues?.tmtAkreditasi ?? ""}
                  onChange={(event) =>
                    handleFieldChange("tmtAkreditasi", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Akreditasi
                </label>
                <input
                  value={formValues?.akreditasi ?? ""}
                  onChange={(event) =>
                    handleFieldChange("akreditasi", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Guru (ASN/Yayasan)
                </label>
                <input
                  value={formValues?.jumlahGuruAsn ?? 0}
                  onChange={(event) =>
                    handleNumberChange("jumlahGuruAsn", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  type="number"
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Guru Honorer
                </label>
                <input
                  value={formValues?.jumlahGuruHonorer ?? 0}
                  onChange={(event) =>
                    handleNumberChange("jumlahGuruHonorer", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  type="number"
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tendik (ASN/Yayasan)
                </label>
                <input
                  value={formValues?.jumlahTendikAsn ?? 0}
                  onChange={(event) =>
                    handleNumberChange("jumlahTendikAsn", event.target.value)
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  type="number"
                  readOnly={!isEditing}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tendik Honorer
                </label>
                <input
                  value={formValues?.jumlahTendikHonorer ?? 0}
                  onChange={(event) =>
                    handleNumberChange(
                      "jumlahTendikHonorer",
                      event.target.value
                    )
                  }
                  className={`${inputClass} ${readOnlyClass}`}
                  type="number"
                  readOnly={!isEditing}
                />
              </div>
            </div>

            <div className="space-y-3 border-t border-zinc-100 pt-4">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Akun Login
                </p>
                <p className="text-sm text-zinc-600">
                  Isi Password Lama untuk menyimpan perubahan akun.
                </p>
                <p className="text-sm text-zinc-600">
                  Kosongkan Password Baru jika tidak ingin mengganti password.
                </p>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Username
                  </label>
                  <input
                    value={accountValues.username}
                    onChange={(event) =>
                      setAccountValues((prev) => ({
                        ...prev,
                        username: event.target.value,
                      }))
                    }
                    className={`${inputClass} ${readOnlyClass}`}
                    readOnly={!isEditing}
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Password Lama
                  </label>
                  <PasswordInput
                    value={accountValues.currentPassword}
                    onChange={(event) =>
                      setAccountValues((prev) => ({
                        ...prev,
                        currentPassword: event.target.value,
                      }))
                    }
                    className={`${inputClass} ${readOnlyClass}`}
                    readOnly={!isEditing}
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Password Baru
                  </label>
                  <PasswordInput
                    value={accountValues.newPassword}
                    onChange={(event) =>
                      setAccountValues((prev) => ({
                        ...prev,
                        newPassword: event.target.value,
                      }))
                    }
                    className={`${inputClass} ${readOnlyClass}`}
                    readOnly={!isEditing}
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Konfirmasi Password Baru
                  </label>
                  <PasswordInput
                    value={accountValues.confirmPassword}
                    onChange={(event) =>
                      setAccountValues((prev) => ({
                        ...prev,
                        confirmPassword: event.target.value,
                      }))
                    }
                    className={`${inputClass} ${readOnlyClass}`}
                    readOnly={!isEditing}
                  />
                </div>
              </div>
            </div>

            {message ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {message}
              </p>
            ) : null}
            {error ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {error}
              </p>
            ) : null}

            {!isEditing ? (
              <button
                type="button"
                onClick={() => setIsEditing(true)}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                Edit
              </button>
            ) : (
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={isSaving}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isSaving ? "Menyimpan..." : "Simpan"}
                </button>
                <button
                  type="button"
                  onClick={handleCancel}
                  className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
                >
                  Batal
                </button>
              </div>
            )}
          </CardContent>
        </Card>
      ) : null}

      {isAdmin ? (
        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Admin
              </p>
              <h2 className="text-lg font-semibold">Daftar Admin Sekolah</h2>
            </div>
            <div className="flex flex-wrap justify-end gap-2">
              <button
                type="button"
                onClick={() => setCreateModalOpen(true)}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                Tambah Admin Sekolah
              </button>
            </div>

            <div className="rounded-xl border border-zinc-200 bg-white p-4">
              <div className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Generate Akun Admin Sekolah
                </p>
                <p className="text-sm text-zinc-600">
                  Pilih jenjang dan status sekolah untuk membuat akun secara
                  otomatis.
                </p>
              </div>
              <div className="mt-3 grid gap-3 md:grid-cols-3 md:items-end">
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Jenjang
                  </label>
                  <Select value={bulkJenjang} onValueChange={setBulkJenjang}>
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Pilih jenjang" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="SMA">SMA</SelectItem>
                      <SelectItem value="SMK">SMK</SelectItem>
                      <SelectItem value="SLB">SLB</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Status
                  </label>
                  <Select value={bulkStatus} onValueChange={setBulkStatus}>
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Pilih status" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="NEGERI">Negeri</SelectItem>
                      <SelectItem value="SWASTA">Swasta</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <button
                  type="button"
                  onClick={handleBulkGenerate}
                  disabled={isBulkGenerating}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isBulkGenerating ? "Memproses..." : "Generate Akun"}
                </button>
              </div>

              {bulkMessage ? (
                <p className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                  {bulkMessage}
                </p>
              ) : null}
              {bulkError ? (
                <p className="mt-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                  {bulkError}
                </p>
              ) : null}

              {bulkSummary ? (
                <div className="mt-3 grid gap-3 md:grid-cols-2">
                  <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3">
                    <p className="text-xs font-semibold tracking-[0.2em] text-emerald-700 uppercase">
                      Berhasil Digenerate ({bulkSummary.generatedCount})
                    </p>
                    <div className="mt-2 max-h-40 space-y-1 overflow-auto text-sm text-emerald-800">
                      {bulkSummary.generated.length === 0 ? (
                        <p className="text-emerald-700">Tidak ada sekolah.</p>
                      ) : (
                        bulkSummary.generated.map((item) => (
                          <p key={item.schoolId}>
                            {item.namaSekolah} ({item.username})
                          </p>
                        ))
                      )}
                    </div>
                  </div>
                  <div className="rounded-lg border border-amber-200 bg-amber-50 p-3">
                    <p className="text-xs font-semibold tracking-[0.2em] text-amber-700 uppercase">
                      Di-skip ({bulkSummary.skippedCount})
                    </p>
                    <div className="mt-2 max-h-40 space-y-1 overflow-auto text-sm text-amber-800">
                      {bulkSummary.skipped.length === 0 ? (
                        <p className="text-amber-700">Tidak ada sekolah.</p>
                      ) : (
                        bulkSummary.skipped.map((item) => (
                          <p key={item.schoolId}>{item.namaSekolah}</p>
                        ))
                      )}
                    </div>
                  </div>
                </div>
              ) : null}
            </div>

            <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
              <div className="w-full md:max-w-sm">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Cari Nama Sekolah
                </label>
                <TypeaheadInput
                  value={adminSchoolQuery}
                  onValueChange={(value) => {
                    setAdminSchoolQuery(value);
                    setAdminSelectedSchoolId(null);
                  }}
                  items={adminSchoolItems}
                  onSelect={(item) => {
                    const selected = adminSchoolSuggestions.find(
                      (school) => school.id === item.key
                    );
                    if (selected) {
                      setAdminSelectedSchoolId(selected.id);
                      setAdminSchoolQuery(selected.nama_sekolah);
                    }
                  }}
                  placeholder="Ketik nama sekolah..."
                  emptyText="Sekolah tidak ditemukan"
                  isLoading={isAdminSchoolLoading}
                  showClear={Boolean(adminSchoolQuery || adminSelectedSchoolId)}
                  onClear={() => {
                    setAdminSchoolQuery("");
                    setAdminSelectedSchoolId(null);
                  }}
                />
              </div>
              <p className="text-xs text-zinc-500">
                Tampilkan 10 sekolah terbaru per halaman.
              </p>
            </div>

            {adminError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {adminError}
              </p>
            ) : null}
            {adminMessage ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {adminMessage}
              </p>
            ) : null}

            <div className="flex justify-end">
              <button
                type="button"
                onClick={openExportModal}
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Download Akun (CSV)
              </button>
            </div>

            <Table
              wrapperClassName="max-h-[420px] overflow-y-auto"
              className="min-w-[960px] md:min-w-full"
            >
              <TableHeader>
                <TableRow>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    Username
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    Pengawas
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    <button
                      type="button"
                      onClick={() => toggleSort("nama_sekolah")}
                      className="mx-auto flex items-center gap-2"
                    >
                      Nama Sekolah
                      {renderSortIcon("nama_sekolah")}
                    </button>
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    <button
                      type="button"
                      onClick={() => toggleSort("last_update_prestasi")}
                      className="mx-auto flex items-center gap-2"
                    >
                      Update Data Prestasi
                      {renderSortIcon("last_update_prestasi")}
                    </button>
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    <button
                      type="button"
                      onClick={() => toggleSort("last_update_siswa_pt")}
                      className="mx-auto flex items-center gap-2"
                    >
                      Update Data Siswa PT
                      {renderSortIcon("last_update_siswa_pt")}
                    </button>
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    Status
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    Aksi
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {adminAccounts.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={7}
                      className="text-center text-sm text-zinc-500"
                    >
                      Belum ada admin sekolah.
                    </TableCell>
                  </TableRow>
                ) : (
                  adminAccounts.map((account) => (
                    <TableRow key={account.id}>
                      <TableCell className="text-center font-semibold text-zinc-900">
                        <div className="flex flex-col items-center gap-2">
                          <span>{account.username}</span>
                          <button
                            type="button"
                            onClick={() => handleCopyUsername(account.username)}
                            className="rounded-lg border border-slate-300 bg-white px-2 py-1 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Copy Username
                          </button>
                        </div>
                      </TableCell>
                      <TableCell className="text-center">
                        <span className="text-sm text-zinc-700">
                          {account.supervisor_name?.trim() ||
                            account.supervisor_username?.trim() ||
                            "-"}
                        </span>
                      </TableCell>
                      <TableCell className="text-center font-semibold text-zinc-900">
                        {account.namaSekolah ?? "-"}
                      </TableCell>
                      <TableCell className="text-center">
                        {formatLastUpdate(account.lastUpdatePrestasi)}
                      </TableCell>
                      <TableCell className="text-center">
                        {formatLastUpdate(account.lastUpdateSiswaPt)}
                      </TableCell>
                      <TableCell className="text-center">
                        <div className="flex justify-center">
                          {statusBadge(account.isOnline)}
                        </div>
                      </TableCell>
                      <TableCell className="text-center">
                        <div className="flex flex-wrap items-center justify-center gap-2">
                          <button
                            type="button"
                            onClick={() => openAdminModal(account)}
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Edit
                          </button>
                          {account.hasInitialPassword ? (
                            <button
                              type="button"
                              onClick={() => openRevealPasswordModal(account)}
                              className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                            >
                              Lihat Password
                            </button>
                          ) : (
                            <button
                              type="button"
                              onClick={() => openResetModal(account)}
                              className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                            >
                              Reset Password
                            </button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>

            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-zinc-200 pt-3 text-sm text-zinc-600">
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handlePrevAdminPage}
                  disabled={isAdminLoadingMore || adminPage <= 1}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  Sebelumnya
                </button>
                <span className="text-zinc-700">
                  Halaman {adminPage}
                  {adminHasTotal ? ` dari ${adminTotalPages}` : ""}
                </span>
              </div>
              <span>
                {adminHasTotal ? (
                  <>
                    Menampilkan {adminRangeStart}â€“{adminRangeEnd} dari{" "}
                    {adminTotalCount} data
                  </>
                ) : adminAccounts.length === 0 ? (
                  "Menampilkan 0 data"
                ) : (
                  <>
                    Menampilkan {adminRangeStart}â€“{adminRangeEnd}
                  </>
                )}
              </span>
              <button
                type="button"
                onClick={handleNextAdminPage}
                disabled={isAdminLoadingMore || !adminHasMore}
                className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70"
              >
                Berikutnya
              </button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {isAdmin ? (
        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Sekolah
              </p>
              <h2 className="text-lg font-semibold">
                Daftar Sekolah & Pengawas
              </h2>
              <p className="text-sm text-zinc-600">
                Lihat pemetaan pengawas untuk setiap sekolah.
              </p>
            </div>
            {schoolOptions.length === 0 ? (
              <p className="text-sm text-zinc-500">
                Daftar sekolah belum tersedia.
              </p>
            ) : (
              <Table wrapperClassName="max-h-[360px] overflow-y-auto">
                <TableHeader>
                  <TableRow>
                    <TableHead className="sticky top-0 z-10 bg-white">
                      Sekolah
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white">
                      NPSN
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 bg-white">
                      Pengawas
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {schoolOptions.map((school) => (
                    <TableRow key={school.id}>
                      <TableCell className="font-semibold text-zinc-900">
                        {school.namaSekolah}
                      </TableCell>
                      <TableCell>{school.npsnSekolah ?? "-"}</TableCell>
                      <TableCell className="text-sm text-zinc-700">
                        {school.supervisor
                          ? `${school.supervisor.name} (${school.supervisor.username})`
                          : "-"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      ) : null}

      {isAdminMaster ? (
        <Card id="pengawas-management">
          <CardContent className="space-y-4">
            <div className="flex flex-wrap items-end justify-between gap-3">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Pengawas
                </p>
                <h2 className="text-lg font-semibold">Daftar Pengawas</h2>
              </div>
              <button
                type="button"
                onClick={() => {
                  setPengawasCreateError("");
                  setPengawasCreateName("");
                  setPengawasCreateUsername("");
                  setPengawasCreatePassword("");
                  setPengawasCreateSchoolQuery("");
                  setPengawasCreateSchoolIDs([]);
                  setIsPengawasCreateOpen(true);
                }}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                Tambah Pengawas
              </button>
            </div>

            {pengawasError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {pengawasError}
              </p>
            ) : null}
            {pengawasMessage ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {pengawasMessage}
              </p>
            ) : null}

            <Table wrapperClassName="max-h-[320px] overflow-y-auto">
              <TableHeader>
                <TableRow>
                  <TableHead className="sticky top-0 z-10 bg-white">
                    Nama
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white">
                    Username/Email
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-right">
                    Jumlah Sekolah Ampu
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-right">
                    Aksi
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isPengawasLoading ? (
                  <TableRow>
                    <TableCell
                      colSpan={4}
                      className="text-center text-sm text-zinc-500"
                    >
                      Memuat daftar pengawas...
                    </TableCell>
                  </TableRow>
                ) : pengawasAccounts.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={4}
                      className="text-center text-sm text-zinc-500"
                    >
                      Belum ada akun pengawas.
                    </TableCell>
                  </TableRow>
                ) : (
                  pengawasAccounts.map((account) => (
                    <TableRow key={account.id}>
                      <TableCell className="font-semibold text-zinc-900">
                        {account.name}
                      </TableCell>
                      <TableCell>{account.username}</TableCell>
                      <TableCell className="text-right">
                        {account.schoolCount}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex flex-wrap justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => openPengawasEditModal(account)}
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Edit Akun
                          </button>
                          <button
                            type="button"
                            onClick={() => openPengawasResetModal(account)}
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Reset Password
                          </button>
                          <button
                            type="button"
                            onClick={() => openPengawasSchoolsModal(account)}
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Kelola Sekolah Ampu
                          </button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}

      {isAdminCabdin ? (
        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Legacy Import
              </p>
              <h2 className="text-lg font-semibold">
                Pengaturan Legacy Import Jenjang
              </h2>
              <p className="text-sm text-zinc-600">
                Atur aktivasi pendataan lulusan tahun sebelumnya per jenjang
                yang diampu.
              </p>
            </div>

            {cabdinLegacyError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {cabdinLegacyError}
              </p>
            ) : null}
            {cabdinLegacyMessage ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {cabdinLegacyMessage}
              </p>
            ) : null}

            {isCabdinLegacyLoading ? (
              <p className="text-sm text-zinc-500">Memuat pengaturan...</p>
            ) : cabdinLegacyLevels.length === 0 ? (
              <p className="text-sm text-zinc-500">
                Jenjang belum tersedia untuk diatur.
              </p>
            ) : (
              <div className="space-y-3">
                {cabdinLegacyLevels.map((item) => {
                  const checked =
                    cabdinLegacyDraft[item.level] ?? item.enabled ?? false;
                  return (
                    <div
                      key={item.level}
                      className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-zinc-200 px-3 py-2"
                    >
                      <div className="space-y-1">
                        <p className="text-sm font-semibold text-zinc-900">
                          {item.level}
                        </p>
                        <p className="text-xs text-zinc-500">
                          {item.locked
                            ? item.source === "MASTER"
                              ? "Dikunci oleh admin master"
                              : item.source === "GLOBAL_DISABLED"
                                ? "Nonaktif secara global"
                                : item.source === "AUTO_DISABLED"
                                  ? "Nonaktif otomatis"
                                  : "Pengaturan dikunci"
                            : item.source === "CABDIN"
                              ? "Mengikuti pengaturan cabdin"
                              : "Mengikuti default sistem"}
                        </p>
                      </div>
                      <label className="flex items-center gap-2 text-sm text-zinc-700">
                        <input
                          type="checkbox"
                          checked={checked}
                          disabled={item.locked}
                          onChange={() =>
                            toggleCabdinLegacyLevel(item.level, checked)
                          }
                        />
                        Aktif
                      </label>
                    </div>
                  );
                })}
              </div>
            )}

            <button
              type="button"
              onClick={handleSaveCabdinLegacyLevels}
              disabled={isCabdinLegacySaving || isCabdinLegacyLoading}
              className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
            >
              {isCabdinLegacySaving ? "Menyimpan..." : "Simpan Pengaturan"}
            </button>
          </CardContent>
        </Card>
      ) : null}

      {isAdminMaster && (
        <section className="portal-card p-4 sm:p-6">
          <h2 className="text-lg font-semibold">Beranda</h2>
          <p className="mt-1 text-sm text-slate-600">Kelola hero, kontak, dan panduan pada beranda website.</p>
          <Link href="/admin/settings/landing" className="mt-3 inline-flex rounded-lg bg-blue-800 px-4 py-2 text-sm font-semibold text-white">Pengaturan Beranda</Link>
        </section>
      )}
      {isSchoolAdmin || isAdmin || isPengawas ? <SchoolLocationSettings /> : null}

      {isAdminMaster ? <MasterPerguruanTinggiCard /> : null}

      {isAdminMaster ? <PtChoiceRulesCard /> : null}

      {isAdminMaster ? <SystemTimeSettingsCard /> : null}

      {isAdminMaster ? (
        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Legacy Import
              </p>
              <h2 className="text-lg font-semibold">
                Pengaturan Legacy Import Global
              </h2>
              <p className="text-sm text-zinc-600">
                Kendalikan aktivasi dan batas tahun untuk pendataan lulusan
                tahun sebelumnya.
              </p>
            </div>

            {legacySettingsError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {legacySettingsError}
              </p>
            ) : null}
            {legacySettingsMessage ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {legacySettingsMessage}
              </p>
            ) : null}

            {isLegacySettingsLoading ? (
              <p className="text-sm text-zinc-500">Memuat pengaturan...</p>
            ) : (
              <>
                <div className="grid gap-3 md:grid-cols-2">
                  <label className="flex items-center gap-2 text-sm text-zinc-700">
                    <input
                      type="checkbox"
                      checked={legacyGlobalEnabled}
                      onChange={() => setLegacyGlobalEnabled((prev) => !prev)}
                    />
                    Legacy import aktif secara global
                  </label>
                  <label className="flex items-center gap-2 text-sm text-zinc-700">
                    <input
                      type="checkbox"
                      checked={legacyForceEnabled}
                      onChange={() => setLegacyForceEnabled((prev) => !prev)}
                    />
                    Paksa aktif (override auto-disable)
                  </label>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Tahun Auto-Disable
                    </label>
                    <input
                      type="number"
                      value={legacyAutoDisableYear}
                      onChange={(event) =>
                        setLegacyAutoDisableYear(event.target.value)
                      }
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      min={2000}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Tahun Maksimal Data
                    </label>
                    <input
                      type="number"
                      value={legacyMaxYear}
                      onChange={(event) => setLegacyMaxYear(event.target.value)}
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      min={2000}
                    />
                  </div>
                </div>
                <button
                  type="button"
                  onClick={handleSaveLegacySettings}
                  disabled={isLegacySettingsSaving}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isLegacySettingsSaving ? "Menyimpan..." : "Simpan Global"}
                </button>

                <div className="space-y-3 border-t border-zinc-100 pt-4">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Override Per Jenjang
                    </p>
                    <p className="text-sm text-zinc-600">
                      Aktifkan override jika ingin mengunci status legacy import
                      per jenjang.
                    </p>
                  </div>
                  <div className="space-y-3">
                    {jenjangOptions.map((level) => {
                      const state = legacyLevelOverrides[level] ?? {
                        override: false,
                        enabled: true,
                      };
                      return (
                        <div
                          key={level}
                          className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-zinc-200 px-3 py-2"
                        >
                          <p className="text-sm font-semibold text-zinc-900">
                            {level}
                          </p>
                          <div className="flex flex-wrap items-center gap-4">
                            <label className="flex items-center gap-2 text-sm text-zinc-700">
                              <input
                                type="checkbox"
                                checked={state.override}
                                onChange={() => toggleLegacyOverride(level)}
                              />
                              Override
                            </label>
                            <label className="flex items-center gap-2 text-sm text-zinc-700">
                              <input
                                type="checkbox"
                                checked={state.enabled}
                                disabled={!state.override}
                                onChange={() => toggleLegacyEnabled(level)}
                              />
                              Aktif
                            </label>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                  <button
                    type="button"
                    onClick={handleSaveLegacyLevels}
                    disabled={isLegacyLevelsSaving}
                    className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                  >
                    {isLegacyLevelsSaving ? "Menyimpan..." : "Simpan Jenjang"}
                  </button>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      ) : null}

      {isAdminMaster ? (
        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Migrasi Data
              </p>
              <h2 className="text-lg font-semibold">Migrasi Legacy PT</h2>
              <p className="text-sm text-zinc-600">
                Migrasikan data siswa_perguruan_tinggi ke skema siswa kelas
                akhir.
              </p>
            </div>

            {legacyPtError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {legacyPtError}
              </p>
            ) : null}
            {legacyPtMessage ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {legacyPtMessage}
              </p>
            ) : null}

            {isLegacyPtLoading ? (
              <p className="text-sm text-zinc-500">Memuat status migrasi...</p>
            ) : (
              <div className="grid gap-3 md:grid-cols-4">
                <div className="rounded-md border border-zinc-200 bg-white px-3 py-2">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Status
                  </p>
                  <p className="text-sm font-semibold text-zinc-900">
                    {legacyPtStatusLabel}
                  </p>
                </div>
                <div className="rounded-md border border-zinc-200 bg-white px-3 py-2">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Migrated
                  </p>
                  <p className="text-sm font-semibold text-zinc-900">
                    {legacyPtStatus?.migratedCount ?? 0}
                  </p>
                </div>
                <div className="rounded-md border border-zinc-200 bg-white px-3 py-2">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Skipped
                  </p>
                  <p className="text-sm font-semibold text-zinc-900">
                    {legacyPtStatus?.skippedCount ?? 0}
                  </p>
                </div>
                <div className="rounded-md border border-zinc-200 bg-white px-3 py-2">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Terakhir
                  </p>
                  <p className="text-sm font-semibold text-zinc-900">
                    {legacyPtTimestamp
                      ? formatIndoDateTime(legacyPtTimestamp)
                      : "-"}
                  </p>
                </div>
              </div>
            )}

            <div className="flex flex-wrap gap-3">
              {hasLegacyPtReport ? (
                <button
                  type="button"
                  onClick={handleDownloadLegacyPtReport}
                  className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
                >
                  Download laporan migrasi terakhir
                </button>
              ) : null}
              {canRunLegacyPt ? (
                <button
                  type="button"
                  onClick={handleRunLegacyPtMigration}
                  disabled={isLegacyPtRunning}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isLegacyPtRunning ? "Menjalankan..." : "Jalankan migrasi"}
                </button>
              ) : null}
            </div>
          </CardContent>
        </Card>
      ) : null}

      {isAdminMaster ? (
        <Card><CardContent className="flex flex-wrap items-center justify-between gap-4"><div><h2 className="text-lg font-semibold">Layanan & Tautan</h2><p className="text-sm text-zinc-600">Kelola Halo Kepegawaian, SINOM, dan Pusat Tautan.</p></div><button type="button" onClick={() => router.push("/user/settings/layanan-tautan")} className="rounded-lg bg-blue-700 px-4 py-2 text-sm font-semibold text-white">Buka Pengaturan</button></CardContent></Card>
      ) : null}

      {isAdminMaster ? (
        <Card>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  GASPOL Bang
                </p>
                <h2 className="text-lg font-semibold">
                  Pengaturan GASPOL Bang
                </h2>
                <p className="text-sm text-zinc-600">
                  Kelola peserta, akun GASPOL, delegasi pengawas, validator,
                  akun juri, monitoring, dan audit log.
                </p>
              </div>
              <button
                type="button"
                onClick={() => router.push("/user/settings/gaspol")}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                Buka Pengaturan GASPOL
              </button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {isAdminMaster ? (
        <Card>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Admin Cabdin
                </p>
                <h2 className="text-lg font-semibold">Daftar Admin Cabdin</h2>
                <p className="text-sm text-zinc-600">
                  Kelola akun admin cabdin dan jenjang yang diampu.
                </p>
              </div>
              <button
                type="button"
                onClick={openCabdinCreateModal}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                Tambah Admin Cabdin
              </button>
            </div>

            {cabdinError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {cabdinError}
              </p>
            ) : null}
            {cabdinMessage ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {cabdinMessage}
              </p>
            ) : null}

            <Table wrapperClassName="max-h-[360px] overflow-y-auto">
              <TableHeader>
                <TableRow>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    Nama
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    Username/Email
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    Jenjang Diampu
                  </TableHead>
                  <TableHead className="sticky top-0 z-10 bg-white text-center">
                    Aksi
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isCabdinLoading ? (
                  <TableRow>
                    <TableCell
                      colSpan={4}
                      className="text-center text-sm text-zinc-500"
                    >
                      Memuat data...
                    </TableCell>
                  </TableRow>
                ) : cabdinAdmins.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={4}
                      className="text-center text-sm text-zinc-500"
                    >
                      Belum ada admin cabdin.
                    </TableCell>
                  </TableRow>
                ) : (
                  cabdinAdmins.map((account) => (
                    <TableRow key={account.id}>
                      <TableCell className="text-center font-semibold text-zinc-900">
                        {account.name}
                      </TableCell>
                      <TableCell className="text-center">
                        <div className="flex flex-col items-center gap-2">
                          <span className="font-semibold text-zinc-900">
                            {account.username}
                          </span>
                          <button
                            type="button"
                            onClick={() =>
                              handleCopyCabdinUsername(account.username)
                            }
                            className="rounded-lg border border-slate-300 bg-white px-2 py-1 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Copy Username
                          </button>
                        </div>
                      </TableCell>
                      <TableCell className="text-center">
                        {renderCabdinLevels(account.levels)}
                      </TableCell>
                      <TableCell className="text-center">
                        <div className="flex flex-wrap items-center justify-center gap-2">
                          <button
                            type="button"
                            onClick={() => openCabdinAccountEditModal(account)}
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Edit Akun
                          </button>
                          <button
                            type="button"
                            onClick={() =>
                              openCabdinResetPasswordModal(account)
                            }
                            className="rounded-md border border-amber-200 bg-amber-50 px-3 py-1.5 text-sm font-semibold text-amber-700 transition hover:bg-amber-100"
                          >
                            Reset Password
                          </button>
                          <button
                            type="button"
                            onClick={() => openCabdinEditModal(account)}
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
                          >
                            Ubah Jenjang
                          </button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}

      {isAdminMaster ? (
        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Reset Data Sekolah (Khusus Admin Master)
              </p>
              <p className="text-sm text-zinc-600">
                Fitur ini digunakan untuk menghapus data tertentu milik sekolah
                guna keperluan pengujian atau import ulang data.
              </p>
            </div>
            <div className="space-y-4">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Pilih Sekolah
                </label>
                <TypeaheadInput
                  value={resetSchoolQuery}
                  onValueChange={(value) => {
                    setResetSchoolQuery(value);
                    if (resetSchoolId !== null) {
                      setResetSchoolId(null);
                      setResetSelectedSchool(null);
                    }
                  }}
                  items={resetSchoolItems}
                  onSelect={(item) => {
                    const selected = resetSchoolSuggestions.find(
                      (school) => school.id === item.key
                    );
                    if (selected) {
                      setResetSchoolId(selected.id);
                      setResetSelectedSchool(selected);
                      setResetSchoolQuery(
                        `${selected.nama_sekolah}${
                          selected.npsn ? ` - ${selected.npsn}` : ""
                        }`
                      );
                    }
                  }}
                  placeholder="Ketik nama sekolah / NPSN..."
                  emptyText="Sekolah tidak ditemukan"
                  isLoading={isResetSchoolLoading}
                  showClear={Boolean(resetSchoolQuery || resetSchoolId)}
                  onClear={() => {
                    setResetSchoolQuery("");
                    setResetSchoolId(null);
                    setResetSelectedSchool(null);
                  }}
                />
                {resetSelectedSchool ? (
                  <div className="inline-flex items-center gap-2 rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">
                    Terpilih:{" "}
                    {`${resetSelectedSchool.nama_sekolah}${
                      resetSelectedSchool.npsn
                        ? ` - ${resetSelectedSchool.npsn}`
                        : ""
                    }`}
                  </div>
                ) : null}
              </div>
              <div className="space-y-2">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Data yang dihapus
                </p>
                <label className="flex items-start gap-2 text-sm text-zinc-700">
                  <input
                    type="checkbox"
                    checked={resetDeleteAchievement}
                    onChange={(event) =>
                      setResetDeleteAchievement(event.target.checked)
                    }
                    className="mt-1 h-4 w-4 rounded border-zinc-300"
                  />
                  <span>Data Prestasi Siswa</span>
                </label>
                <label className="flex items-start gap-2 text-sm text-zinc-700">
                  <input
                    type="checkbox"
                    checked={resetDeleteAdmissions}
                    onChange={(event) =>
                      setResetDeleteAdmissions(event.target.checked)
                    }
                    className="mt-1 h-4 w-4 rounded border-zinc-300"
                  />
                  <span>
                    Data Siswa (Kelas Akhir + Rencana Lulusan + Outcome/PT)
                  </span>
                </label>
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tahun Lulus (Opsional)
                </label>
                <input
                  type="number"
                  value={resetTahunLulus ?? ""}
                  onChange={(event) => {
                    const value = event.target.value.trim();
                    if (value === "") {
                      setResetTahunLulus(null);
                      return;
                    }
                    const parsed = Number(value);
                    setResetTahunLulus(
                      Number.isInteger(parsed) && parsed > 0 ? parsed : null
                    );
                  }}
                  disabled={!resetDeleteAdmissions}
                  placeholder="Kosong = semua tahun (berisiko)"
                  className={inputClass}
                />
              </div>
              {resetDeleteAdmissions && resetTahunLulus === null ? (
                <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                  âš ï¸ Tahun Lulus kosong berarti reset SEMUA tahun untuk
                  sekolah ini. Aktifkan Override untuk melanjutkan.
                </p>
              ) : null}
              <label className="flex items-start gap-2 text-sm text-zinc-700">
                <input
                  type="checkbox"
                  checked={resetOverride}
                  onChange={(event) => setResetOverride(event.target.checked)}
                  disabled={!resetDeleteAdmissions}
                  className="mt-1 h-4 w-4 rounded border-zinc-300"
                />
                <span>Override (bypass lock/tutup buku)</span>
              </label>
              {resetMessage ? (
                <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                  {resetMessage}
                </p>
              ) : null}
              <div>
                <button
                  type="button"
                  onClick={openResetDataModal}
                  disabled={!canResetData}
                  className="rounded-md bg-red-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  Hapus Data Terpilih
                </button>
              </div>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {isPengawasCreateOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Tambah Pengawas
              </h2>
              <p className="text-sm text-zinc-600">
                Buat akun pengawas dan tentukan sekolah yang diampu.
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama
                </label>
                <input
                  value={pengawasCreateName}
                  onChange={(event) =>
                    setPengawasCreateName(event.target.value)
                  }
                  className={inputClass}
                  placeholder="Nama pengawas"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Username/Email
                </label>
                <input
                  value={pengawasCreateUsername}
                  onChange={(event) =>
                    setPengawasCreateUsername(event.target.value)
                  }
                  className={inputClass}
                  placeholder="username"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password
                </label>
                <PasswordInput
                  value={pengawasCreatePassword}
                  onChange={(event) =>
                    setPengawasCreatePassword(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Sekolah Diampu
                </label>
                <TypeaheadInput
                  value={pengawasCreateSchoolQuery}
                  onValueChange={setPengawasCreateSchoolQuery}
                  items={pengawasCreateSchoolItems}
                  onSelect={(item) => {
                    const selected = pengawasCreateSchoolSuggestions.find(
                      (school) => school.id === item.key
                    );
                    if (!selected) return;
                    addCreatePengawasSchool(selected.id);
                    setPengawasCreateSchoolQuery("");
                  }}
                  placeholder="Cari sekolah..."
                  emptyText="Sekolah tidak ditemukan"
                  isLoading={isPengawasCreateSchoolLoading}
                />
                {pengawasCreateSchoolIDs.length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {pengawasCreateSchoolIDs.map((schoolID) => {
                      const school = schoolOptionByID.get(schoolID);
                      const label = school
                        ? `${school.namaSekolah}${
                            school.npsnSekolah ? ` - ${school.npsnSekolah}` : ""
                          }`
                        : `Sekolah #${schoolID}`;
                      return (
                        <button
                          key={schoolID}
                          type="button"
                          onClick={() => removeCreatePengawasSchool(schoolID)}
                          className="rounded-full border border-slate-300 bg-slate-50 px-3 py-1 text-xs font-semibold text-zinc-700 transition hover:bg-slate-100"
                        >
                          {label} ×
                        </button>
                      );
                    })}
                  </div>
                ) : (
                  <p className="text-xs text-zinc-500">
                    Belum ada sekolah terpilih.
                  </p>
                )}
              </div>
            </div>
            {pengawasCreateError ? (
              <p className="mt-3 text-sm text-red-600">{pengawasCreateError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsPengawasCreateOpen(false);
                  setPengawasCreateError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleCreatePengawas}
                disabled={isPengawasCreating}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isPengawasCreating ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isPengawasEditOpen && pengawasEditTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Edit Akun Pengawas
              </h2>
              <p className="text-sm text-zinc-600">
                Perbarui informasi akun untuk{" "}
                <span className="font-semibold text-zinc-900">
                  {pengawasEditTarget.username}
                </span>
                .
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama
                </label>
                <input
                  value={pengawasEditName}
                  onChange={(event) => setPengawasEditName(event.target.value)}
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Username/Email
                </label>
                <input
                  value={pengawasEditUsername}
                  onChange={(event) =>
                    setPengawasEditUsername(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
            </div>
            {pengawasEditError ? (
              <p className="mt-3 text-sm text-red-600">{pengawasEditError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsPengawasEditOpen(false);
                  setPengawasEditTarget(null);
                  setPengawasEditError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleSavePengawasAccount}
                disabled={isPengawasEditSaving}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isPengawasEditSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isPengawasResetOpen && pengawasResetTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Reset Password Pengawas
              </h2>
              <p className="text-sm text-zinc-600">
                Set password baru untuk{" "}
                <span className="font-semibold text-zinc-900">
                  {pengawasResetTarget.username}
                </span>
                .
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password Baru
                </label>
                <PasswordInput
                  value={pengawasResetPassword}
                  onChange={(event) =>
                    setPengawasResetPassword(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Konfirmasi Password
                </label>
                <PasswordInput
                  value={pengawasResetConfirm}
                  onChange={(event) =>
                    setPengawasResetConfirm(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
            </div>
            {pengawasResetError ? (
              <p className="mt-3 text-sm text-red-600">{pengawasResetError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsPengawasResetOpen(false);
                  setPengawasResetTarget(null);
                  setPengawasResetError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleResetPengawasPassword}
                disabled={isPengawasResetSaving}
                className="rounded-md bg-amber-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-amber-700 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isPengawasResetSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isPengawasSchoolsOpen && pengawasSchoolsTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Kelola Sekolah Ampu
              </h2>
              <p className="text-sm text-zinc-600">
                Atur sekolah yang diampu oleh{" "}
                <span className="font-semibold text-zinc-900">
                  {pengawasSchoolsTarget.name}
                </span>
                .
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Cari Sekolah
                </label>
                <TypeaheadInput
                  value={pengawasSchoolsQuery}
                  onValueChange={setPengawasSchoolsQuery}
                  items={pengawasSchoolsItems}
                  onSelect={(item) => {
                    const selected = pengawasSchoolsSuggestions.find(
                      (school) => school.id === item.key
                    );
                    if (!selected) return;
                    addPengawasSchoolSelection(selected.id);
                    setPengawasSchoolsQuery("");
                  }}
                  placeholder="Cari sekolah..."
                  emptyText="Sekolah tidak ditemukan"
                  isLoading={isPengawasSchoolsLoading}
                />
              </div>
              {pengawasSchoolsSelectedIDs.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {pengawasSchoolsSelectedIDs.map((schoolID) => {
                    const school = schoolOptionByID.get(schoolID);
                    const label = school
                      ? `${school.namaSekolah}${
                          school.npsnSekolah ? ` - ${school.npsnSekolah}` : ""
                        }`
                      : `Sekolah #${schoolID}`;
                    return (
                      <button
                        key={schoolID}
                        type="button"
                        onClick={() => removePengawasSchoolSelection(schoolID)}
                        className="rounded-full border border-slate-300 bg-slate-50 px-3 py-1 text-xs font-semibold text-zinc-700 transition hover:bg-slate-100"
                      >
                        {label} ×
                      </button>
                    );
                  })}
                </div>
              ) : (
                <p className="text-xs text-zinc-500">
                  Belum ada sekolah terpilih.
                </p>
              )}
            </div>
            {pengawasSchoolsError ? (
              <p className="mt-3 text-sm text-red-600">
                {pengawasSchoolsError}
              </p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsPengawasSchoolsOpen(false);
                  setPengawasSchoolsTarget(null);
                  setPengawasSchoolsError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleSavePengawasSchools}
                disabled={isPengawasSchoolsSaving}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isPengawasSchoolsSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isRevealPasswordOpen && revealTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Lihat Password Awal
              </h2>
              <p className="text-sm text-zinc-600">
                Verifikasi password admin untuk melihat password{" "}
                <span className="font-semibold text-zinc-900">
                  {revealTarget.username}
                </span>
                .
              </p>
            </div>

            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password Admin
                </label>
                <PasswordInput
                  value={revealAdminPassword}
                  onChange={(event) =>
                    setRevealAdminPassword(event.target.value)
                  }
                  className={inputClass}
                />
              </div>

              {revealedPassword ? (
                <div className="rounded-md border border-zinc-200 bg-zinc-50 p-3 text-sm text-zinc-800">
                  <p className="font-semibold text-zinc-700">Password:</p>
                  <div className="mt-2 flex items-center justify-between gap-2">
                    <span className="font-mono break-all">
                      {revealedPassword}
                    </span>
                    <button
                      type="button"
                      onClick={handleCopyRevealedPassword}
                      className="shrink-0 rounded-lg border border-slate-300 bg-white px-2 py-1 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                    >
                      Copy
                    </button>
                  </div>
                </div>
              ) : null}
            </div>

            {revealError ? (
              <p className="mt-3 text-sm text-red-600">{revealError}</p>
            ) : null}

            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsRevealPasswordOpen(false);
                  setRevealTarget(null);
                  setRevealAdminPassword("");
                  setRevealedPassword(null);
                  setRevealError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Tutup
              </button>
              <button
                type="button"
                onClick={handleRevealPassword}
                disabled={isRevealing}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isRevealing ? "Memuat..." : "Lihat"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isExportOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Download Akun Admin Sekolah (CSV)
              </h2>
              <p className="text-sm text-zinc-600">
                Verifikasi password admin sebelum mengunduh file.
              </p>
            </div>

            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password Admin
                </label>
                <PasswordInput
                  value={exportAdminPassword}
                  onChange={(event) =>
                    setExportAdminPassword(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
            </div>

            {exportError ? (
              <p className="mt-3 text-sm text-red-600">{exportError}</p>
            ) : null}

            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsExportOpen(false);
                  setExportAdminPassword("");
                  setExportError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleExportAccounts}
                disabled={isExporting}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isExporting ? "Mengunduh..." : "Download"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isModalOpen && selectedAdmin ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Edit Admin Sekolah
              </h2>
              <p className="text-sm text-zinc-600">
                Update username atau reset password.
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Username
                </label>
                <input
                  value={editUsername}
                  onChange={(event) => setEditUsername(event.target.value)}
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password
                </label>
                <PasswordInput
                  value="********"
                  onChange={() => {}}
                  readOnly
                  className={`${inputClass} bg-zinc-100 text-zinc-500`}
                />
                <button
                  type="button"
                  onClick={() => {
                    if (selectedAdmin) {
                      openResetModal(selectedAdmin);
                    }
                    setIsModalOpen(false);
                  }}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
                >
                  Ubah Password
                </button>
              </div>
            </div>
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setIsModalOpen(false)}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleAdminSave}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                Simpan
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isResetModalOpen && selectedAdmin ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Reset Password Admin Sekolah
              </h2>
              <p className="text-sm text-zinc-600">
                Verifikasi password admin sebelum membuat password baru.
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password Admin
                </label>
                <PasswordInput
                  value={adminPassword}
                  onChange={(event) => setAdminPassword(event.target.value)}
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password Baru Admin Sekolah
                </label>
                <PasswordInput
                  value={newAdminPassword}
                  onChange={(event) => setNewAdminPassword(event.target.value)}
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Konfirmasi Password Baru
                </label>
                <PasswordInput
                  value={confirmAdminPassword}
                  onChange={(event) =>
                    setConfirmAdminPassword(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
            </div>
            {resetError ? (
              <p className="mt-3 text-sm text-red-600">{resetError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsResetModalOpen(false);
                  setResetError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleResetPassword}
                disabled={isResetSaving}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isResetSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isChangePasswordOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Ubah Password
              </h2>
              <p className="text-sm text-zinc-600">
                Masukkan password lama dan password baru akun Anda.
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password Lama
                </label>
                <PasswordInput
                  value={oldAdminPassword}
                  onChange={(event) => setOldAdminPassword(event.target.value)}
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password Baru
                </label>
                <PasswordInput
                  value={newAdminPasswordValue}
                  onChange={(event) =>
                    setNewAdminPasswordValue(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Konfirmasi Password Baru
                </label>
                <PasswordInput
                  value={confirmAdminPasswordValue}
                  onChange={(event) =>
                    setConfirmAdminPasswordValue(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
            </div>
            {changePasswordError ? (
              <p className="mt-3 text-sm text-red-600">{changePasswordError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsChangePasswordOpen(false);
                  setChangePasswordError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleChangePassword}
                disabled={isChangePasswordDisabled}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isChangePasswordSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {resetModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Konfirmasi Penghapusan Data
              </h2>
              <p className="text-sm font-semibold text-red-600">
                Tindakan ini akan menghapus data secara permanen dan tidak dapat
                dikembalikan.
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="rounded-md border border-zinc-200 bg-zinc-50 p-3 text-sm">
                <p className="font-semibold text-zinc-700">Akan menghapus:</p>
                <div className="mt-2 space-y-1 text-zinc-600">
                  {resetDeleteAchievement ? <p>- Data Prestasi Siswa</p> : null}
                  {resetDeleteAdmissions ? (
                    <p>
                      - Data Siswa (Kelas Akhir + Rencana Lulusan + Outcome/PT)
                    </p>
                  ) : null}
                  {resetDeleteAdmissions ? (
                    <p>
                      - Scope Tahun Lulus:{" "}
                      {resetTahunLulus == null
                        ? "SEMUA tahun (berisiko)"
                        : resetTahunLulus}
                    </p>
                  ) : null}
                  {resetDeleteAdmissions ? (
                    <p>- Override: {resetOverride ? "Aktif" : "Tidak aktif"}</p>
                  ) : null}
                </div>
              </div>
            </div>
            {resetDataError ? (
              <p className="mt-3 text-sm text-red-600">{resetDataError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setResetModalOpen(false);
                  setResetDataError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleConfirmResetData}
                disabled={isResetConfirmDisabled}
                className="rounded-md bg-red-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isResetDataSaving ? "Menghapus..." : "Konfirmasi Hapus"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isCabdinCreateOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Tambah Admin Cabdin
              </h2>
              <p className="text-sm text-zinc-600">
                Buat akun admin cabdin baru dan tentukan jenjang yang diampu.
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama
                </label>
                <input
                  value={cabdinCreateName}
                  onChange={(event) => setCabdinCreateName(event.target.value)}
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Username/Email
                </label>
                <input
                  value={cabdinCreateUsername}
                  onChange={(event) =>
                    setCabdinCreateUsername(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password
                </label>
                <PasswordInput
                  value={cabdinCreatePassword}
                  onChange={(event) =>
                    setCabdinCreatePassword(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
              <div className="space-y-2">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Jenjang Diampu
                </p>
                <div className="grid gap-2 sm:grid-cols-3">
                  {jenjangOptions.map((level) => (
                    <label
                      key={level}
                      className="flex items-center gap-2 text-sm text-zinc-700"
                    >
                      <input
                        type="checkbox"
                        checked={cabdinCreateLevels.includes(level)}
                        onChange={() =>
                          toggleCabdinLevel(level, setCabdinCreateLevels)
                        }
                        className="h-4 w-4 rounded border-zinc-300"
                      />
                      <span>{level}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
            {cabdinCreateError ? (
              <p className="mt-3 text-sm text-red-600">{cabdinCreateError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsCabdinCreateOpen(false);
                  setCabdinCreateError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleCreateCabdinAdmin}
                disabled={isCabdinSaving}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isCabdinSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isCabdinAccountEditOpen && cabdinAccountEditTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Edit Akun Admin Cabdin
              </h2>
              <p className="text-sm text-zinc-600">
                Ubah nama dan email login untuk{" "}
                <span className="font-semibold text-zinc-900">
                  {cabdinAccountEditTarget.name}
                </span>
                .
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama
                </label>
                <input
                  value={cabdinAccountEditName}
                  onChange={(event) =>
                    setCabdinAccountEditName(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Username/Email
                </label>
                <input
                  value={cabdinAccountEditUsername}
                  onChange={(event) =>
                    setCabdinAccountEditUsername(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
            </div>
            {cabdinAccountEditError ? (
              <p className="mt-3 text-sm text-red-600">
                {cabdinAccountEditError}
              </p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsCabdinAccountEditOpen(false);
                  setCabdinAccountEditTarget(null);
                  setCabdinAccountEditError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleSaveCabdinAccount}
                disabled={isCabdinAccountSaving}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isCabdinAccountSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isCabdinPasswordResetOpen && cabdinPasswordResetTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Reset Password Admin Cabdin
              </h2>
              <p className="text-sm text-zinc-600">
                Set password baru untuk{" "}
                <span className="font-semibold text-zinc-900">
                  {cabdinPasswordResetTarget.username}
                </span>
                .
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password Baru
                </label>
                <PasswordInput
                  value={cabdinPasswordResetValue}
                  onChange={(event) =>
                    setCabdinPasswordResetValue(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Konfirmasi Password
                </label>
                <PasswordInput
                  value={cabdinPasswordResetConfirm}
                  onChange={(event) =>
                    setCabdinPasswordResetConfirm(event.target.value)
                  }
                  className={inputClass}
                />
              </div>
            </div>
            {cabdinPasswordResetError ? (
              <p className="mt-3 text-sm text-red-600">
                {cabdinPasswordResetError}
              </p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsCabdinPasswordResetOpen(false);
                  setCabdinPasswordResetTarget(null);
                  setCabdinPasswordResetError("");
                  setCabdinPasswordResetValue("");
                  setCabdinPasswordResetConfirm("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleResetCabdinPassword}
                disabled={isCabdinPasswordResetSaving}
                className="rounded-md bg-amber-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-amber-700 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isCabdinPasswordResetSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isCabdinEditOpen && cabdinEditTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Ubah Jenjang Admin Cabdin
              </h2>
              <p className="text-sm text-zinc-600">
                Atur jenjang untuk{" "}
                <span className="font-semibold text-zinc-900">
                  {cabdinEditTarget.name}
                </span>
                .
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
                Username:{" "}
                <span className="font-semibold text-zinc-900">
                  {cabdinEditTarget.username}
                </span>
              </div>
              <div className="space-y-2">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Jenjang Diampu
                </p>
                <div className="grid gap-2 sm:grid-cols-3">
                  {jenjangOptions.map((level) => (
                    <label
                      key={level}
                      className="flex items-center gap-2 text-sm text-zinc-700"
                    >
                      <input
                        type="checkbox"
                        checked={cabdinEditLevels.includes(level)}
                        onChange={() =>
                          toggleCabdinLevel(level, setCabdinEditLevels)
                        }
                        className="h-4 w-4 rounded border-zinc-300"
                      />
                      <span>{level}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
            {cabdinEditError ? (
              <p className="mt-3 text-sm text-red-600">{cabdinEditError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsCabdinEditOpen(false);
                  setCabdinEditTarget(null);
                  setCabdinEditError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleSaveCabdinLevels}
                disabled={isCabdinSaving}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isCabdinSaving ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {createModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-900">
                Tambah Admin Sekolah
              </h2>
              <p className="text-sm text-zinc-600">
                Buat akun SCHOOL_ADMIN untuk sekolah terpilih.
              </p>
            </div>
            <div className="mt-4 space-y-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Sekolah
                </label>
                <Select
                  value={selectedSchoolId ? String(selectedSchoolId) : ""}
                  onValueChange={(value) => setSelectedSchoolId(Number(value))}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Pilih sekolah" />
                  </SelectTrigger>
                  <SelectContent>
                    {schoolOptions.map((school) => (
                      <SelectItem key={school.id} value={String(school.id)}>
                        {(school.npsnSekolah ?? "-") +
                          " - " +
                          school.namaSekolah}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Username
                </label>
                <input
                  value={createUsername}
                  onChange={(event) => setCreateUsername(event.target.value)}
                  className={inputClass}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Password
                </label>
                <PasswordInput
                  value={createPassword}
                  onChange={(event) => setCreatePassword(event.target.value)}
                  className={inputClass}
                />
              </div>
            </div>
            {createError ? (
              <p className="mt-3 text-sm text-red-600">{createError}</p>
            ) : null}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setCreateModalOpen(false);
                  setCreateError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleCreateAdmin}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                Simpan
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}
