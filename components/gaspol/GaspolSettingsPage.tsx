"use client";

import {
  useCallback,
  useEffect,
  useState,
  useTransition,
  type ComponentType,
} from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ClipboardList,
  Download,
  Gavel,
  RefreshCcw,
  School,
  ScrollText,
  Settings2,
  ShieldCheck,
  Users,
} from "lucide-react";

import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/status-badge";
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
import { formatIndoDateTime } from "@/lib/date";
import {
  getErrorMessageFromResponse,
  getFilenameFromContentDisposition,
} from "@/lib/http";
import { cn } from "@/lib/utils";

type GaspolAdminDashboard = {
  isYearActive: boolean;
  participantCount: number;
  supervisorCount: number;
  validatorCount: number;
  juryCount: number;
  pendingValidation: number;
};

type GaspolSchoolParticipant = {
  id: number;
  schoolId?: number | null;
  npsn: string;
  schoolName: string;
  schoolLevel: string;
  schoolStatus: string;
  isActive: boolean;
  validatorUserId?: number | null;
  validatorName: string;
  schoolEntryUserId?: number | null;
  schoolEntryName: string;
};

type GaspolSchoolUserRow = {
  schoolId: number;
  npsn: string;
  schoolName: string;
  schoolLevel: string;
  schoolStatus: string;
  isParticipant: boolean;
  userId?: number | null;
  username: string;
  accountStatus: string;
  roleIsActive: boolean;
  displayName: string;
};

type GaspolRoleCandidate = {
  roleAssignmentId?: number | null;
  userId: number;
  username: string;
  displayName: string;
  baseRole: string;
  isAssigned: boolean;
  isActive: boolean;
  assignedSchoolCount: number;
  assignmentCount: number;
};

type GaspolStageAssignment = {
  id: number;
  year: number;
  stage: string;
  schoolId?: number | null;
  schoolName: string;
  npsn: string;
  assessorUserId: number;
  assessorName: string;
  username: string;
  baseRole: string;
  assignmentRole: string;
  isActive: boolean;
  updatedAt?: string | null;
};

type GaspolAssignmentSchoolCandidate = {
  schoolId: number;
  npsn: string;
  schoolName: string;
  cohorts: string[];
};

type GaspolSchoolMonitoringStatusRow = {
  schoolId: number;
  npsn: string;
  schoolName: string;
  schoolLevel: string;
  schoolStatus: string;
  month: number;
  mengisi: "Sudah" | "Belum" | "Tidak";
};

type GaspolSupervisorMonitoringStatusRow = {
  assignmentId: number;
  schoolId: number;
  npsn: string;
  schoolName: string;
  month: number;
  supervisorUserId?: number | null;
  supervisorName: string;
  method: string;
  penilaian: "Sudah" | "Belum";
  hasSubmittedScore: boolean;
};

type GaspolAssignmentPreviewRow = {
  supervisorUserId: number;
  supervisorName: string;
  previousMonthCount: number;
  currentMonthCount: number;
  historicalCount: number;
  runningTotal: number;
  schoolNames: string[];
  newCount: number;
  manualCount: number;
  lockedCount: number;
};

type GaspolAssignmentPreview = {
  previewId: string;
  year: number;
  month: number;
  rows: GaspolAssignmentPreviewRow[];
  unassignedSchoolIds: number[];
};

type GaspolAuditLogRow = {
  id: number;
  actorUserId?: number | null;
  actorName: string;
  actorRole: string;
  action: string;
  entityType: string;
  entityId?: number | null;
  schoolId?: number | null;
  schoolName: string;
  payload: string;
  createdAt?: string | null;
};

type GaspolAdminSettingsOverview = {
  year: number;
  summary: GaspolAdminDashboard;
  participants: GaspolSchoolParticipant[];
  schoolUsers: GaspolSchoolUserRow[];
  supervisorCandidates: GaspolRoleCandidate[];
  validatorCandidates: GaspolRoleCandidate[];
  koorCandidates: GaspolRoleCandidate[];
  koorAssignments: GaspolStageAssignment[];
  stageAssignments: GaspolStageAssignment[];
  stageSchoolCandidates: GaspolAssignmentSchoolCandidate[];
  internalCandidates: GaspolRoleCandidate[];
  juryAccounts: GaspolRoleCandidate[];
  schoolMonitoring: GaspolSchoolMonitoringStatusRow[];
  supervisorMonitoring: GaspolSupervisorMonitoringStatusRow[];
  auditLogs: GaspolAuditLogRow[];
};

type GeneratedCredential = {
  userId: number;
  username: string;
  password: string;
  role: string;
  schoolId?: number | null;
  schoolName: string;
  displayName: string;
};

type GenerateUsersResult = {
  created: GeneratedCredential[];
  skipped: GeneratedCredential[];
};

type AssignAllValidatorsResult = {
  year: number;
  validatorUserId: number;
  updatedCount: number;
};

type BulkStageAssignmentResult = {
  created: number[];
  skippedExisting: number[];
  removed: number[];
  errors: Array<{ schoolId: number; error: string }>;
};

const buttonClass =
  "inline-flex items-center justify-center rounded-lg px-3 py-2 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-60";
const primaryButtonClass =
  "bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] text-white shadow-sm hover:opacity-95";
const secondaryButtonClass =
  "border border-slate-300 bg-white text-slate-700 hover:bg-slate-50";
const warningButtonClass =
  "border border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100";
const inputClass =
  "w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 outline-none transition focus:border-sky-400";
const currentYear = new Date().getFullYear();
const currentMonth = new Date().getMonth() + 1;

const monthOptions = [
  { value: 4, label: "April" },
  { value: 5, label: "Mei" },
  { value: 6, label: "Juni" },
  { value: 7, label: "Juli" },
  { value: 8, label: "Agustus" },
  { value: 9, label: "September" },
  { value: 10, label: "Oktober" },
  { value: 11, label: "November" },
] as const;

const stageAssignmentConfigs: StageAssignmentConfig[] = [
  {
    stage: "FIELD_STAGE_1",
    assignmentRole: "FIELD_STAGE_1_ASSESSOR",
    title: "Lapangan Tahap 1",
    description:
      "Pengawas hanya dapat menilai sekolah peserta yang ditugaskan kepadanya.",
  },
  {
    stage: "FIELD_STAGE_2",
    assignmentRole: "FIELD_STAGE_2_ASSESSOR",
    title: "Lapangan Tahap 2 / Juri LH",
    description:
      "Akun Juri LH ditugaskan per sekolah dari cohort Top 10 yang sudah dikunci.",
  },
  {
    stage: "PRESENTATION",
    assignmentRole: "PRESENTATION_ASSESSOR",
    title: "Presentasi",
    description:
      "Penilai internal ditugaskan per finalis Top 5 tanpa mengubah base role akun.",
  },
  {
    stage: "NOT_FAILED_NOMINATION",
    assignmentRole: "NOMINATION_ASSESSOR",
    title: "Nominasi Tidak Tergagal",
    description:
      "Pengawas menilai sekolah cohort rank 11–21; peserta tidak dipilih manual.",
  },
  {
    stage: "BOTTOM5_APPEAL",
    assignmentRole: "APPEAL_PANEL",
    title: "Banding Kandidat Zona Tergagal",
    description:
      "Panel internal ditugaskan ke sekolah Bottom 5 yang sudah dikunci.",
  },
  {
    stage: "INNOVATION",
    assignmentRole: "INNOVATION_ASSESSOR",
    title: "Ter-Inovatif",
    description:
      "Pengawas yang ditunjuk menilai finalis Top 5 sebelum approval Kasi SMK.",
  },
];

const defaultOverview = (year: number): GaspolAdminSettingsOverview => ({
  year,
  summary: {
    isYearActive: false,
    participantCount: 0,
    supervisorCount: 0,
    validatorCount: 0,
    juryCount: 0,
    pendingValidation: 0,
  },
  participants: [],
  schoolUsers: [],
  supervisorCandidates: [],
  validatorCandidates: [],
  koorCandidates: [],
  koorAssignments: [],
  stageAssignments: [],
  stageSchoolCandidates: [],
  internalCandidates: [],
  juryAccounts: [],
  schoolMonitoring: [],
  supervisorMonitoring: [],
  auditLogs: [],
});

const monthName = (value: number) =>
  monthOptions.find((item) => item.value === value)?.label ?? `Bulan ${value}`;

const pickDefaultMonth = (min: number, max: number, fallback: number) => {
  if (currentMonth >= min && currentMonth <= max) {
    return currentMonth;
  }
  return fallback;
};

const readKeyword = (value: string) => value.trim().toLowerCase();

const includesKeyword = (
  keyword: string,
  values: Array<string | number | null | undefined>
) => {
  if (!keyword) return true;
  return values.some((value) =>
    String(value ?? "")
      .toLowerCase()
      .includes(keyword)
  );
};

const trimPayload = (value: string, maxLength = 120) => {
  if (!value.trim()) return "-";
  if (value.length <= maxLength) return value;
  return `${value.slice(0, maxLength)}...`;
};

async function fetchOverview(year: number) {
  const response = await apiFetch(
    `/gaspol/admin/settings/overview?year=${year}`
  );
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal memuat pengaturan GASPOL."
      )
    );
  }
  const payload = (await response.json()) as {
    data?: GaspolAdminSettingsOverview;
  };
  return payload.data ?? defaultOverview(year);
}

async function activateYear(year: number) {
  const response = await apiFetch("/gaspol/admin/year/activate", {
    method: "POST",
    body: JSON.stringify({ year }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal mengaktifkan tahun GASPOL."
      )
    );
  }
}

async function saveParticipants(year: number, schoolIds: number[]) {
  const response = await apiFetch("/gaspol/admin/participants", {
    method: "PUT",
    body: JSON.stringify({ year, schoolIds }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal menyimpan peserta GASPOL."
      )
    );
  }
}

async function bulkGenerateSchoolUsers(year: number, schoolIds: number[]) {
  const response = await apiFetch("/gaspol/admin/school-users/generate", {
    method: "POST",
    body: JSON.stringify({ year, schoolIds }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal generate user sekolah GASPOL."
      )
    );
  }
  const payload = (await response.json()) as { data?: GenerateUsersResult };
  return payload.data ?? { created: [], skipped: [] };
}

async function resetGaspolPassword(userId: number) {
  const response = await apiFetch("/gaspol/admin/gaspol-users/reset-password", {
    method: "POST",
    body: JSON.stringify({ userId }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal reset password akun GASPOL."
      )
    );
  }
  const payload = (await response.json()) as { data?: GeneratedCredential };
  if (!payload.data) {
    throw new Error("Respons reset password GASPOL tidak lengkap.");
  }
  return payload.data;
}

async function updateRoleStatus(
  year: number,
  userId: number,
  role: "SUPERVISOR" | "VALIDATOR" | "JURY",
  isActive: boolean
) {
  const response = await apiFetch("/gaspol/admin/roles/status", {
    method: "PATCH",
    body: JSON.stringify({ year, userId, role, isActive }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal memperbarui status role GASPOL."
      )
    );
  }
}

async function assignValidator(
  year: number,
  schoolId: number,
  validatorUserId: number
) {
  const response = await apiFetch("/gaspol/admin/validators/assign", {
    method: "POST",
    body: JSON.stringify({ year, schoolId, validatorUserId }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal assign validator GASPOL."
      )
    );
  }
}

async function assignValidatorToAllParticipants(
  year: number,
  validatorUserId: number
) {
  const response = await apiFetch("/gaspol/admin/validators/assign-all", {
    method: "POST",
    body: JSON.stringify({ year, validatorUserId }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal menerapkan validator ke semua sekolah peserta GASPOL."
      )
    );
  }
  const payload = (await response.json()) as {
    data?: AssignAllValidatorsResult;
  };
  return payload.data ?? { year, validatorUserId, updatedCount: 0 };
}

async function updateKoorGaspolAssignment(
  year: number,
  assessorUserId: number,
  isActive: boolean
) {
  const response = await apiFetch("/gaspol/admin/stage-assignments", {
    method: "POST",
    body: JSON.stringify({
      year,
      stage: "MONTHLY",
      assessorUserId,
      assignmentRole: "KOOR_GASPOL",
      isActive,
    }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal memperbarui assignment Koor GASPOL."
      )
    );
  }
}

async function updateStageAssignments(
  year: number,
  stage: string,
  assignmentRole: string,
  schoolIds: number[],
  assessorUserId: number,
  isActive: boolean
) {
  const response = await apiFetch("/gaspol/admin/stage-assignments/bulk", {
    method: "POST",
    body: JSON.stringify({
      year,
      stage,
      assignmentRole,
      schoolIds,
      assessorUserId,
      isActive,
    }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal memperbarui penugasan penilai GASPOL."
      )
    );
  }
  const payload = (await response.json()) as {
    data?: BulkStageAssignmentResult;
  };
  return (
    payload.data ?? {
      created: [],
      skippedExisting: [],
      removed: [],
      errors: [],
    }
  );
}

async function manualAssignment(
  year: number,
  month: number,
  schoolId: number,
  supervisorUserId: number
) {
  const response = await apiFetch("/gaspol/admin/assignments/manual", {
    method: "POST",
    body: JSON.stringify({ year, month, schoolId, supervisorUserId }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal assign manual delegasi pengawas."
      )
    );
  }
}

async function previewRandomAssignments(year: number, month: number) {
  const response = await apiFetch("/gaspol/admin/assignments/random/preview", {
    method: "POST",
    body: JSON.stringify({ year, month }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal generate assignment pengawas."
      )
    );
  }
  const payload = (await response.json()) as { data?: GaspolAssignmentPreview };
  if (!payload.data) {
    throw new Error("Preview random tidak tersedia.");
  }
  return payload.data;
}

async function confirmRandomAssignments(previewId: string) {
  const response = await apiFetch("/gaspol/admin/assignments/random/confirm", {
    method: "POST",
    body: JSON.stringify({ previewId }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal menyimpan assignment hasil preview."
      )
    );
  }
}

async function generateJuryAccount(
  year: number,
  username: string,
  displayName: string
) {
  const response = await apiFetch("/gaspol/admin/jury-account", {
    method: "POST",
    body: JSON.stringify({ year, username, displayName }),
  });
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal membuat akun juri GASPOL."
      )
    );
  }
  const payload = (await response.json()) as { data?: GeneratedCredential };
  if (!payload.data) {
    throw new Error("Respons akun juri GASPOL tidak lengkap.");
  }
  return payload.data;
}

async function downloadExport(year: number, month: number) {
  const response = await apiFetch(
    `/gaspol/admin/export.xlsx?year=${year}&month=${month}`
  );
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal mengunduh export GASPOL."
      )
    );
  }
  const blob = await response.blob();
  const filename = getFilenameFromContentDisposition(
    response.headers.get("content-disposition"),
    "gaspol-monitoring.xlsx"
  );
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  window.URL.revokeObjectURL(url);
}

async function downloadSchoolUsersExport(year: number) {
  const response = await apiFetch(
    `/gaspol/admin/school-users/export.xlsx?year=${year}`
  );
  if (!response.ok) {
    throw new Error(
      await getErrorMessageFromResponse(
        response,
        "Gagal mengunduh daftar user sekolah GASPOL."
      )
    );
  }
  const blob = await response.blob();
  const filename = getFilenameFromContentDisposition(
    response.headers.get("content-disposition"),
    `gaspol-user-sekolah-${year}.xlsx`
  );
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  window.URL.revokeObjectURL(url);
}

function SummaryCard({
  title,
  value,
  icon: Icon,
}: {
  title: string;
  value: string | number;
  icon: ComponentType<{ className?: string }>;
}) {
  return (
    <Card className="border-slate-200/80">
      <CardContent className="flex items-start justify-between gap-4 p-5">
        <div>
          <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
            {title}
          </p>
          <p className="mt-3 text-3xl font-semibold text-slate-950">{value}</p>
        </div>
        <div className="rounded-2xl bg-sky-50 p-3 text-sky-700">
          <Icon className="h-5 w-5" />
        </div>
      </CardContent>
    </Card>
  );
}

type StageAssignmentConfig = {
  stage: string;
  assignmentRole: string;
  title: string;
  description: string;
};

function SchoolChecklist({
  schools,
  selected,
  onChange,
  assignedSchoolIds = new Set<number>(),
  mode = "assign",
  emptyMessage,
}: {
  schools: GaspolAssignmentSchoolCandidate[];
  selected: Set<number>;
  onChange: (value: Set<number>) => void;
  assignedSchoolIds?: Set<number>;
  mode?: "assign" | "unassign";
  emptyMessage: string;
}) {
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const pageSize = 50;
  const keyword = query.trim().toLowerCase();
  const visibleSchools = schools.filter((school) => {
    if (mode === "unassign" && !assignedSchoolIds.has(school.schoolId)) {
      return false;
    }
    return `${school.npsn} ${school.schoolName}`
      .toLowerCase()
      .includes(keyword);
  });
  const pageCount = Math.max(1, Math.ceil(visibleSchools.length / pageSize));
  const safePage = Math.min(page, pageCount);
  const pageSchools = visibleSchools.slice(
    (safePage - 1) * pageSize,
    safePage * pageSize
  );
  const selectableFiltered = visibleSchools.filter(
    (school) => mode === "unassign" || !assignedSchoolIds.has(school.schoolId)
  );

  const toggle = (schoolId: number) => {
    const next = new Set(selected);
    if (next.has(schoolId)) next.delete(schoolId);
    else next.add(schoolId);
    onChange(next);
  };

  return (
    <div className="space-y-3">
      <input
        className={inputClass}
        value={query}
        onChange={(event) => {
          setQuery(event.target.value);
          setPage(1);
        }}
        placeholder="Cari nama sekolah atau NPSN..."
      />
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          className={cn(buttonClass, secondaryButtonClass)}
          disabled={selectableFiltered.length === 0}
          onClick={() =>
            onChange(
              new Set([
                ...selected,
                ...selectableFiltered.map((school) => school.schoolId),
              ])
            )
          }
        >
          Pilih Semua Hasil Filter
        </button>
        <button
          type="button"
          className={cn(buttonClass, secondaryButtonClass)}
          disabled={selected.size === 0}
          onClick={() => onChange(new Set())}
        >
          Batalkan Semua
        </button>
        <span className="text-sm font-semibold text-slate-700">
          Dipilih: {selected.size} sekolah
        </span>
      </div>
      {visibleSchools.length === 0 ? (
        <p className="rounded-xl bg-slate-50 px-4 py-5 text-sm text-slate-500">
          {emptyMessage}
        </p>
      ) : (
        <div className="max-h-80 divide-y divide-slate-100 overflow-y-auto rounded-xl border border-slate-200">
          {pageSchools.map((school) => {
            const assigned = assignedSchoolIds.has(school.schoolId);
            const disabled = mode === "assign" && assigned;
            return (
              <label
                key={school.schoolId}
                className={cn(
                  "flex items-center gap-3 px-4 py-3 text-sm",
                  disabled
                    ? "bg-slate-50 text-slate-400"
                    : "cursor-pointer hover:bg-sky-50"
                )}
              >
                <input
                  type="checkbox"
                  checked={selected.has(school.schoolId)}
                  disabled={disabled}
                  onChange={() => toggle(school.schoolId)}
                  className="h-4 w-4 rounded border-slate-300"
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-semibold">
                    {school.schoolName}
                  </span>
                  <span className="text-xs">{school.npsn || "NPSN -"}</span>
                </span>
                {assigned ? (
                  <StatusBadge variant="success">Sudah ditugaskan</StatusBadge>
                ) : null}
              </label>
            );
          })}
        </div>
      )}
      {pageCount > 1 ? (
        <div className="flex items-center justify-between text-sm text-slate-600">
          <button
            type="button"
            className={cn(buttonClass, secondaryButtonClass)}
            disabled={safePage === 1}
            onClick={() => setPage((value) => Math.max(1, value - 1))}
          >
            Sebelumnya
          </button>
          <span>
            Halaman {safePage} dari {pageCount}
          </span>
          <button
            type="button"
            className={cn(buttonClass, secondaryButtonClass)}
            disabled={safePage === pageCount}
            onClick={() => setPage((value) => Math.min(pageCount, value + 1))}
          >
            Berikutnya
          </button>
        </div>
      ) : null}
    </div>
  );
}

function StageAssignmentSection({
  config,
  schools,
  assessors,
  assignments,
  busyKey,
  onUpdate,
}: {
  config: StageAssignmentConfig;
  schools: GaspolAssignmentSchoolCandidate[];
  assessors: GaspolRoleCandidate[];
  assignments: GaspolStageAssignment[];
  busyKey: string;
  onUpdate: (
    config: StageAssignmentConfig,
    schoolIds: number[],
    assessorUserId: number,
    isActive: boolean
  ) => Promise<BulkStageAssignmentResult | void>;
}) {
  const [assessorUserId, setAssessorUserId] = useState(0);
  const [selectedSchoolIds, setSelectedSchoolIds] = useState<Set<number>>(
    new Set()
  );
  const [mode, setMode] = useState<"assign" | "unassign">("assign");
  const [assessorQuery, setAssessorQuery] = useState("");
  const normalizedAssessorQuery = assessorQuery.trim().toLowerCase();
  const filteredAssessors = assessors.filter((item) =>
    `${item.displayName} ${item.username} ${item.baseRole}`
      .toLowerCase()
      .includes(normalizedAssessorQuery)
  );
  const activeAssignments = assignments.filter(
    (item) =>
      item.isActive &&
      item.stage === config.stage &&
      item.assignmentRole === config.assignmentRole
  );
  const assignedSchoolIds = new Set(
    activeAssignments
      .filter((item) => item.assessorUserId === assessorUserId)
      .flatMap((item) => (item.schoolId ? [item.schoolId] : []))
  );
  const operationKey = `bulk-${config.stage}`;
  const emptyMessage =
    schools.length === 0
      ? config.stage === "FIELD_STAGE_2"
        ? "Top 10 belum ditetapkan."
        : config.stage === "PRESENTATION" || config.stage === "INNOVATION"
          ? "Top 5 belum ditetapkan."
          : config.stage === "NOT_FAILED_NOMINATION"
            ? "Cohort Nominasi Tidak Tergagal belum tersedia."
            : config.stage === "BOTTOM5_APPEAL"
              ? "Bottom 5 belum tersedia karena tahap Nominasi belum selesai."
              : "Belum ada peserta GASPOL yang eligible."
      : mode === "unassign"
        ? "Penilai ini belum memiliki sekolah yang ditugaskan pada tahap ini."
        : "Tidak ada sekolah yang cocok dengan pencarian.";

  const submit = async () => {
    if (
      mode === "unassign" &&
      !window.confirm(`Hapus penugasan dari ${selectedSchoolIds.size} sekolah?`)
    ) {
      return;
    }
    await onUpdate(
      config,
      [...selectedSchoolIds],
      assessorUserId,
      mode === "assign"
    );
    setSelectedSchoolIds(new Set());
  };

  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-5">
      <div>
        <h3 className="font-semibold text-slate-950">{config.title}</h3>
        <p className="mt-1 text-sm leading-6 text-slate-600">
          {config.description}
        </p>
      </div>
      <div className="mt-4 space-y-4">
        <div className="grid gap-3 lg:grid-cols-[1fr_auto] lg:items-end">
          <div className="space-y-2">
            <input
              className={inputClass}
              value={assessorQuery}
              onChange={(event) => setAssessorQuery(event.target.value)}
              placeholder="Cari nama atau username penilai"
            />
            <select
              className={inputClass}
              value={assessorUserId}
              onChange={(event) => {
                setAssessorUserId(Number(event.target.value));
                setSelectedSchoolIds(new Set());
              }}
            >
              <option value={0}>Pilih penilai eligible</option>
              {filteredAssessors.map((item) => (
                <option key={item.userId} value={item.userId}>
                  {item.displayName} ({item.username})
                </option>
              ))}
            </select>
          </div>
          <div className="flex rounded-lg border border-slate-200 p-1">
            {(["assign", "unassign"] as const).map((value) => (
              <button
                key={value}
                type="button"
                onClick={() => {
                  setMode(value);
                  setSelectedSchoolIds(new Set());
                }}
                className={cn(
                  "rounded-md px-3 py-2 text-sm font-semibold",
                  mode === value ? "bg-slate-900 text-white" : "text-slate-600"
                )}
              >
                {value === "assign" ? "Tambah" : "Hapus"}
              </button>
            ))}
          </div>
        </div>
        <SchoolChecklist
          schools={schools}
          selected={selectedSchoolIds}
          onChange={setSelectedSchoolIds}
          assignedSchoolIds={assignedSchoolIds}
          mode={mode}
          emptyMessage={emptyMessage}
        />
      </div>
      <button
        type="button"
        className={cn(buttonClass, primaryButtonClass, "mt-3")}
        disabled={
          !assessorUserId ||
          selectedSchoolIds.size === 0 ||
          busyKey === operationKey
        }
        onClick={() => void submit()}
      >
        {busyKey === operationKey
          ? "Menyimpan..."
          : mode === "assign"
            ? `Simpan Penugasan ke ${selectedSchoolIds.size} Sekolah`
            : `Hapus Penugasan dari ${selectedSchoolIds.size} Sekolah`}
      </button>

      <div className="mt-5 space-y-2">
        <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
          Assignment Aktif ({activeAssignments.length})
        </p>
        {activeAssignments.length === 0 ? (
          <p className="rounded-xl bg-slate-50 px-4 py-3 text-sm text-slate-500">
            Belum ada penilai yang ditugaskan.
          </p>
        ) : (
          activeAssignments.map((item) => (
            <div
              key={item.id}
              className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-slate-200 px-4 py-3"
            >
              <div>
                <p className="font-semibold text-slate-900">
                  {item.schoolName || `Sekolah #${item.schoolId}`}
                </p>
                <p className="text-sm text-slate-500">
                  {item.assessorName} ({item.username})
                </p>
              </div>
              <StatusBadge variant="success">Aktif</StatusBadge>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function MonthlyFieldShortcut({
  schools,
  assessors,
  busyKey,
  onSubmit,
}: {
  schools: GaspolAssignmentSchoolCandidate[];
  assessors: GaspolRoleCandidate[];
  busyKey: string;
  onSubmit: (
    assessorUserId: number,
    schoolIds: number[],
    targets: Set<"JULY" | "AUGUST" | "FIELD_STAGE_1">
  ) => Promise<void>;
}) {
  const [assessorUserId, setAssessorUserId] = useState(0);
  const [schoolIds, setSchoolIds] = useState<Set<number>>(new Set());
  const [targets, setTargets] = useState<
    Set<"JULY" | "AUGUST" | "FIELD_STAGE_1">
  >(new Set(["JULY", "AUGUST", "FIELD_STAGE_1"]));
  const options = [
    { value: "JULY" as const, label: "Juli" },
    { value: "AUGUST" as const, label: "Agustus" },
    { value: "FIELD_STAGE_1" as const, label: "Lapangan Tahap 1" },
  ];
  const toggleTarget = (target: "JULY" | "AUGUST" | "FIELD_STAGE_1") => {
    const next = new Set(targets);
    if (next.has(target)) next.delete(target);
    else next.add(target);
    setTargets(next);
  };

  return (
    <div className="rounded-2xl border border-sky-200 bg-sky-50/50 p-5">
      <h3 className="font-semibold text-slate-950">
        Shortcut Juli + Agustus + Lapangan Tahap 1
      </h3>
      <p className="mt-1 text-sm leading-6 text-slate-600">
        Terapkan pengawas dan sekolah yang sama ke beberapa mekanisme existing
        dalam satu workflow.
      </p>
      <div className="mt-4 space-y-4">
        <select
          className={inputClass}
          value={assessorUserId}
          onChange={(event) => {
            setAssessorUserId(Number(event.target.value));
            setSchoolIds(new Set());
          }}
        >
          <option value={0}>Pilih pengawas aktif</option>
          {assessors.map((item) => (
            <option key={item.userId} value={item.userId}>
              {item.displayName} ({item.username})
            </option>
          ))}
        </select>
        <SchoolChecklist
          schools={schools}
          selected={schoolIds}
          onChange={setSchoolIds}
          emptyMessage="Belum ada peserta GASPOL yang eligible."
        />
        <div>
          <p className="mb-2 text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
            Terapkan ke
          </p>
          <div className="flex flex-wrap gap-4">
            {options.map((option) => (
              <label
                key={option.value}
                className="flex cursor-pointer items-center gap-2 text-sm font-semibold text-slate-700"
              >
                <input
                  type="checkbox"
                  checked={targets.has(option.value)}
                  onChange={() => toggleTarget(option.value)}
                  className="h-4 w-4 rounded border-slate-300"
                />
                {option.label}
              </label>
            ))}
          </div>
        </div>
        <button
          type="button"
          className={cn(buttonClass, primaryButtonClass)}
          disabled={
            !assessorUserId ||
            schoolIds.size === 0 ||
            targets.size === 0 ||
            busyKey === "bulk-monthly-field1"
          }
          onClick={() =>
            void onSubmit(assessorUserId, [...schoolIds], targets).then(() =>
              setSchoolIds(new Set())
            )
          }
        >
          {busyKey === "bulk-monthly-field1"
            ? "Menyimpan..."
            : `Simpan Penugasan ke ${schoolIds.size} Sekolah`}
        </button>
      </div>
    </div>
  );
}

function SectionNav() {
  const items = [
    { id: "peserta", label: "Peserta" },
    { id: "user-sekolah", label: "User Sekolah" },
    { id: "delegasi-pengawas", label: "Pengawas" },
    { id: "penugasan-penilai", label: "Penugasan Penilai" },
    { id: "koor-gaspol", label: "Koor GASPOL" },
    { id: "validator-cabdin", label: "Validator Cabdin" },
    { id: "akun-juri", label: "Akun Juri" },
    { id: "monitoring-sekolah", label: "Monitoring Sekolah" },
    { id: "monitoring-pengawas", label: "Monitoring Pengawas" },
    { id: "audit-log", label: "Audit Log" },
  ];

  return (
    <div className="sticky top-0 z-20 rounded-2xl border border-slate-200 bg-white/95 p-3 backdrop-blur">
      <div className="flex flex-wrap gap-2">
        {items.map((item) => (
          <a
            key={item.id}
            href={`#${item.id}`}
            className="rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-700 transition hover:border-slate-900 hover:text-slate-900"
          >
            {item.label}
          </a>
        ))}
      </div>
    </div>
  );
}

export function GaspolSettingsPage() {
  const router = useRouter();
  const { user, status } = useAuth();
  const [overview, setOverview] = useState<GaspolAdminSettingsOverview>(
    defaultOverview(currentYear)
  );
  const [year, setYear] = useState(currentYear);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [busyKey, setBusyKey] = useState("");
  const [isPending, startTransition] = useTransition();
  const [generatedCredentials, setGeneratedCredentials] = useState<
    GeneratedCredential[]
  >([]);
  const [resetCredential, setResetCredential] =
    useState<GeneratedCredential | null>(null);
  const [assignmentPreview, setAssignmentPreview] =
    useState<GaspolAssignmentPreview | null>(null);
  const [assignmentPreviewMode, setAssignmentPreviewMode] = useState<
    "generate" | "regenerate"
  >("generate");

  const [participantQuery, setParticipantQuery] = useState("");
  const [participantJenjang, setParticipantJenjang] = useState("ALL");
  const [participantStatus, setParticipantStatus] = useState("ALL");
  const [schoolUserQuery, setSchoolUserQuery] = useState("");
  const [supervisorQuery, setSupervisorQuery] = useState("");
  const [assignmentSearchSchool, setAssignmentSearchSchool] = useState("");
  const [assignmentSearchSupervisor, setAssignmentSearchSupervisor] =
    useState("");
  const [assignmentMonth, setAssignmentMonth] = useState(
    pickDefaultMonth(4, 9, 4)
  );
  const [selectedSupervisorSchoolId, setSelectedSupervisorSchoolId] =
    useState(0);
  const [selectedSupervisorUserId, setSelectedSupervisorUserId] = useState(0);
  const [validatorQuery, setValidatorQuery] = useState("");
  const [selectedKoorUserId, setSelectedKoorUserId] = useState(0);
  const [selectedValidatorSchoolId, setSelectedValidatorSchoolId] = useState(0);
  const [selectedValidatorUserId, setSelectedValidatorUserId] = useState(0);
  const [selectedBulkValidatorUserId, setSelectedBulkValidatorUserId] =
    useState(0);
  const [juryQuery, setJuryQuery] = useState("");
  const [juryUsername, setJuryUsername] = useState("gaspol-jury");
  const [juryDisplayName, setJuryDisplayName] = useState("Gaspol Jury");
  const [schoolMonitoringQuery, setSchoolMonitoringQuery] = useState("");
  const [schoolMonitoringStatus, setSchoolMonitoringStatus] = useState("ALL");
  const [schoolMonitoringMonth, setSchoolMonitoringMonth] = useState(
    pickDefaultMonth(4, 11, 4)
  );
  const [supervisorMonitoringQuery, setSupervisorMonitoringQuery] =
    useState("");
  const [supervisorMonitoringSchoolQuery, setSupervisorMonitoringSchoolQuery] =
    useState("");
  const [supervisorMonitoringStatus, setSupervisorMonitoringStatus] =
    useState("ALL");
  const [supervisorMonitoringMonth, setSupervisorMonitoringMonth] = useState(
    pickDefaultMonth(4, 9, 4)
  );
  const [auditQuery, setAuditQuery] = useState("");

  const isAdminMaster = user?.role === "ADMIN_MASTER";
  const canAccess = status === "authenticated" && isAdminMaster;

  const loadOverview = useCallback(async (selectedYear: number) => {
    setLoading(true);
    setError("");
    try {
      const data = await fetchOverview(selectedYear);
      setOverview(data);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal memuat pengaturan GASPOL."
      );
      setOverview(defaultOverview(selectedYear));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (status === "unauthenticated") {
      router.replace("/login");
    }
  }, [router, status]);

  useEffect(() => {
    if (!canAccess) return;
    void loadOverview(year);
  }, [canAccess, loadOverview, year]);

  const clearFlash = () => {
    setError("");
    setMessage("");
  };

  const handleRefresh = () => {
    clearFlash();
    startTransition(() => {
      void loadOverview(year);
    });
  };

  const handleToggleParticipant = async (
    schoolId: number,
    nextActive: boolean
  ) => {
    const currentActiveIds = overview.participants
      .filter((item) => item.isActive && item.schoolId)
      .map((item) => item.schoolId as number);
    const nextIds = nextActive
      ? Array.from(new Set([...currentActiveIds, schoolId]))
      : currentActiveIds.filter((item) => item !== schoolId);

    clearFlash();
    setBusyKey(`participant-${schoolId}`);
    try {
      await saveParticipants(year, nextIds);
      setMessage("Daftar peserta GASPOL berhasil diperbarui.");
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal memperbarui peserta GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleActivateYear = async () => {
    clearFlash();
    setBusyKey("activate-year");
    try {
      await activateYear(year);
      setMessage(`Tahun GASPOL ${year} berhasil diaktifkan.`);
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal mengaktifkan tahun GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleBulkGenerateUsers = async () => {
    const schoolIds = overview.schoolUsers
      .filter((item) => item.isParticipant)
      .map((item) => item.schoolId);
    if (schoolIds.length === 0) {
      setError("Belum ada peserta GASPOL aktif yang bisa dibuatkan akun.");
      return;
    }
    clearFlash();
    setBusyKey("bulk-generate-users");
    try {
      const result = await bulkGenerateSchoolUsers(year, schoolIds);
      setGeneratedCredentials(result.created);
      setResetCredential(null);
      setMessage(
        `Generate user sekolah selesai. Dibuat ${result.created.length} akun, dilewati ${result.skipped.length} akun.`
      );
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal generate user sekolah GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleResetPassword = async (userId: number) => {
    clearFlash();
    setBusyKey(`reset-${userId}`);
    try {
      const result = await resetGaspolPassword(userId);
      setResetCredential(result);
      setGeneratedCredentials([]);
      setMessage(`Password akun ${result.username} berhasil direset.`);
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal reset password GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleRoleToggle = async (
    role: "SUPERVISOR" | "VALIDATOR" | "JURY",
    userId: number,
    nextActive: boolean
  ) => {
    clearFlash();
    setBusyKey(`${role}-${userId}`);
    try {
      await updateRoleStatus(year, userId, role, nextActive);
      setMessage(`Status ${role} GASPOL berhasil diperbarui.`);
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal memperbarui status role GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleManualAssignment = async () => {
    if (!selectedSupervisorSchoolId || !selectedSupervisorUserId) {
      setError(
        "Pilih sekolah dan pengawas sebelum menyimpan assignment manual."
      );
      return;
    }
    clearFlash();
    setBusyKey("manual-assignment");
    try {
      await manualAssignment(
        year,
        assignmentMonth,
        selectedSupervisorSchoolId,
        selectedSupervisorUserId
      );
      setMessage(
        `Delegasi pengawas bulan ${monthName(assignmentMonth)} berhasil diperbarui.`
      );
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal assign manual pengawas GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleRandomAssignments = async (mode: "generate" | "regenerate") => {
    clearFlash();
    setBusyKey(mode === "generate" ? "generate-random" : "regenerate-random");
    try {
      const preview = await previewRandomAssignments(year, assignmentMonth);
      setAssignmentPreviewMode(mode);
      setAssignmentPreview(preview);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal generate assignment pengawas."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleConfirmRandomAssignments = async () => {
    if (!assignmentPreview) return;
    clearFlash();
    setBusyKey("confirm-random");
    try {
      await confirmRandomAssignments(assignmentPreview.previewId);
      setMessage(
        `${
          assignmentPreviewMode === "generate"
            ? "Generate random"
            : "Regenerate random"
        } bulan ${monthName(assignmentPreview.month)} selesai. Assignment bernilai/terkunci dipertahankan.`
      );
      setAssignmentPreview(null);
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal menyimpan assignment hasil preview."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleAssignValidator = async () => {
    if (!selectedValidatorSchoolId || !selectedValidatorUserId) {
      setError("Pilih sekolah dan validator sebelum menyimpan assignment.");
      return;
    }
    clearFlash();
    setBusyKey("assign-validator");
    try {
      await assignValidator(
        year,
        selectedValidatorSchoolId,
        selectedValidatorUserId
      );
      setMessage("Validator GASPOL berhasil ditetapkan.");
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal assign validator GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleKoorAssignment = async (
    assessorUserId: number,
    isActive: boolean
  ) => {
    if (!assessorUserId) {
      setError("Pilih akun PENGAWAS yang akan menjadi Koor GASPOL.");
      return;
    }
    clearFlash();
    setBusyKey("koor-gaspol");
    try {
      await updateKoorGaspolAssignment(year, assessorUserId, isActive);
      setMessage(
        isActive
          ? `Koor GASPOL tahun ${year} berhasil ditetapkan.`
          : `Assignment Koor GASPOL tahun ${year} berhasil dinonaktifkan.`
      );
      if (!isActive) setSelectedKoorUserId(0);
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal memperbarui assignment Koor GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleStageAssignment = async (
    config: StageAssignmentConfig,
    schoolIds: number[],
    assessorUserId: number,
    isActive: boolean
  ) => {
    if (schoolIds.length === 0 || !assessorUserId) {
      setError("Pilih sekolah dan penilai sebelum menyimpan penugasan.");
      return;
    }
    clearFlash();
    setBusyKey(`bulk-${config.stage}`);
    try {
      const result = await updateStageAssignments(
        year,
        config.stage,
        config.assignmentRole,
        schoolIds,
        assessorUserId,
        isActive
      );
      setMessage(
        isActive
          ? `${config.title} — Berhasil: ${result.created.length}, dilewati karena sudah ada: ${result.skippedExisting.length}, gagal: ${result.errors.length}.`
          : `${config.title} — Dihapus: ${result.removed.length}, dilewati: ${result.skippedExisting.length}, gagal: ${result.errors.length}.`
      );
      await loadOverview(year);
      return result;
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal memperbarui assignment penilai GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleMonthlyFieldShortcut = async (
    assessorUserId: number,
    schoolIds: number[],
    targets: Set<"JULY" | "AUGUST" | "FIELD_STAGE_1">
  ) => {
    clearFlash();
    setBusyKey("bulk-monthly-field1");
    let created = 0;
    let skipped = 0;
    let failed = 0;
    try {
      for (const schoolId of schoolIds) {
        for (const target of ["JULY", "AUGUST"] as const) {
          if (!targets.has(target)) continue;
          const month = target === "JULY" ? 7 : 8;
          const exists = overview.supervisorMonitoring.some(
            (item) =>
              item.month === month &&
              item.schoolId === schoolId &&
              item.supervisorUserId === assessorUserId
          );
          if (exists) {
            skipped += 1;
            continue;
          }
          try {
            await manualAssignment(year, month, schoolId, assessorUserId);
            created += 1;
          } catch {
            failed += 1;
          }
        }
      }
      if (targets.has("FIELD_STAGE_1")) {
        const result = await updateStageAssignments(
          year,
          "FIELD_STAGE_1",
          "FIELD_STAGE_1_ASSESSOR",
          schoolIds,
          assessorUserId,
          true
        );
        created += result.created.length;
        skipped += result.skippedExisting.length;
        failed += result.errors.length;
      }
      setMessage(
        `Shortcut Juli/Agustus/Tahap 1 — Berhasil: ${created}, dilewati karena sudah ada: ${skipped}, gagal: ${failed}.`
      );
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal menyimpan shortcut penugasan."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleAssignValidatorAll = async () => {
    if (!selectedBulkValidatorUserId) {
      setError("Pilih validator sebelum menerapkan ke semua sekolah peserta.");
      return;
    }
    clearFlash();
    setBusyKey("assign-validator-all");
    try {
      const result = await assignValidatorToAllParticipants(
        year,
        selectedBulkValidatorUserId
      );
      setMessage(
        `Validator aktif berhasil diganti untuk ${result.updatedCount} sekolah peserta GASPOL tahun ${year}.`
      );
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal menerapkan validator ke semua sekolah peserta GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleGenerateJury = async () => {
    clearFlash();
    setBusyKey("generate-jury");
    try {
      const result = await generateJuryAccount(
        year,
        juryUsername.trim(),
        juryDisplayName.trim()
      );
      setResetCredential(result);
      setGeneratedCredentials([]);
      setMessage(`Akun juri ${result.username} berhasil disiapkan.`);
      await loadOverview(year);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal membuat akun juri GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleDownloadExport = async (month: number) => {
    clearFlash();
    setBusyKey(`export-${month}`);
    try {
      await downloadExport(year, month);
      setMessage(`Export GASPOL bulan ${monthName(month)} berhasil diunduh.`);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal mengunduh export GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const handleDownloadSchoolUsersExport = async () => {
    clearFlash();
    setBusyKey("export-school-users");
    try {
      await downloadSchoolUsersExport(year);
      setMessage(`Daftar user sekolah GASPOL tahun ${year} berhasil diunduh.`);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal mengunduh daftar user sekolah GASPOL."
      );
    } finally {
      setBusyKey("");
    }
  };

  const participantKeyword = readKeyword(participantQuery);
  const participantRows = overview.participants.filter((item) => {
    const matchesQuery = includesKeyword(participantKeyword, [
      item.npsn,
      item.schoolName,
    ]);
    const matchesJenjang =
      participantJenjang === "ALL" || item.schoolLevel === participantJenjang;
    const matchesStatus =
      participantStatus === "ALL" || item.schoolStatus === participantStatus;
    return matchesQuery && matchesJenjang && matchesStatus;
  });

  const schoolUserKeyword = readKeyword(schoolUserQuery);
  const schoolUserRows = overview.schoolUsers.filter((item) => {
    if (!item.isParticipant) return false;
    return includesKeyword(schoolUserKeyword, [
      item.npsn,
      item.schoolName,
      item.username,
      item.displayName,
    ]);
  });

  const supervisorKeyword = readKeyword(supervisorQuery);
  const supervisorCandidates = overview.supervisorCandidates.filter((item) =>
    includesKeyword(supervisorKeyword, [
      item.displayName,
      item.username,
      item.baseRole,
    ])
  );
  const assignmentSchoolKeyword = readKeyword(assignmentSearchSchool);
  const assignmentSupervisorKeyword = readKeyword(assignmentSearchSupervisor);
  const assignmentRows = overview.supervisorMonitoring.filter((item) => {
    if (item.month !== assignmentMonth) return false;
    if (!includesKeyword(assignmentSchoolKeyword, [item.npsn, item.schoolName]))
      return false;
    return includesKeyword(assignmentSupervisorKeyword, [item.supervisorName]);
  });

  const validatorKeyword = readKeyword(validatorQuery);
  const validatorCandidates = overview.validatorCandidates.filter((item) =>
    includesKeyword(validatorKeyword, [
      item.displayName,
      item.username,
      item.baseRole,
    ])
  );
  const activeKoorAssignment = overview.koorAssignments.find(
    (item) => item.isActive
  );
  const koorOptions = overview.koorCandidates.filter(
    (item) => item.baseRole === "PENGAWAS"
  );
  const validatorAssignments = overview.participants.filter(
    (item) =>
      item.isActive &&
      includesKeyword(validatorKeyword, [
        item.schoolName,
        item.validatorName,
        item.npsn,
      ])
  );

  const juryKeyword = readKeyword(juryQuery);
  const juryRows = overview.juryAccounts.filter((item) =>
    includesKeyword(juryKeyword, [
      item.displayName,
      item.username,
      item.baseRole,
    ])
  );

  const schoolMonitoringMonthRows = overview.schoolMonitoring.filter(
    (item) => item.month === schoolMonitoringMonth
  );
  const schoolMonitoringKeyword = readKeyword(schoolMonitoringQuery);
  const filteredSchoolMonitoringRows = schoolMonitoringMonthRows.filter(
    (item) => {
      const matchesQuery = includesKeyword(schoolMonitoringKeyword, [
        item.npsn,
        item.schoolName,
      ]);
      const matchesStatus =
        schoolMonitoringStatus === "ALL" ||
        item.mengisi === schoolMonitoringStatus;
      return matchesQuery && matchesStatus;
    }
  );

  const supervisorMonitoringMonthRows = overview.supervisorMonitoring.filter(
    (item) => item.month === supervisorMonitoringMonth
  );
  const supervisorMonitoringKeyword = readKeyword(supervisorMonitoringQuery);
  const supervisorMonitoringSchoolKeyword = readKeyword(
    supervisorMonitoringSchoolQuery
  );
  const filteredSupervisorMonitoringRows = supervisorMonitoringMonthRows.filter(
    (item) => {
      const matchesSupervisor = includesKeyword(supervisorMonitoringKeyword, [
        item.supervisorName,
      ]);
      const matchesSchool = includesKeyword(supervisorMonitoringSchoolKeyword, [
        item.npsn,
        item.schoolName,
      ]);
      const matchesStatus =
        supervisorMonitoringStatus === "ALL" ||
        item.penilaian === supervisorMonitoringStatus;
      return matchesSupervisor && matchesSchool && matchesStatus;
    }
  );

  const auditKeyword = readKeyword(auditQuery);
  const auditRows = overview.auditLogs.filter((item) =>
    includesKeyword(auditKeyword, [
      item.actorName,
      item.actorRole,
      item.action,
      item.entityType,
      item.schoolName,
      item.payload,
    ])
  );

  const activeParticipants = overview.participants.filter(
    (
      item
    ): item is GaspolSchoolParticipant & {
      schoolId: number;
    } => item.isActive && typeof item.schoolId === "number"
  );
  const activeSupervisorOptions = overview.supervisorCandidates.filter(
    (item) => item.isActive
  );
  const activeValidatorOptions = overview.validatorCandidates.filter(
    (item) => item.isActive
  );
  const stageSchoolsFor = (stage: string) => {
    const requiredCohort =
      stage === "FIELD_STAGE_2"
        ? "TOP_10"
        : stage === "PRESENTATION" || stage === "INNOVATION"
          ? "TOP_5"
          : stage === "NOT_FAILED_NOMINATION"
            ? "NOT_FAILED"
            : stage === "BOTTOM5_APPEAL"
              ? "BOTTOM_5"
              : "";
    return overview.stageSchoolCandidates.filter(
      (item) => !requiredCohort || item.cohorts.includes(requiredCohort)
    );
  };
  const stageAssessorsFor = (stage: string) => {
    if (
      stage === "FIELD_STAGE_1" ||
      stage === "NOT_FAILED_NOMINATION" ||
      stage === "INNOVATION"
    ) {
      return overview.supervisorCandidates.filter(
        (item) => item.isActive && item.baseRole === "PENGAWAS"
      );
    }
    if (stage === "FIELD_STAGE_2") {
      return overview.juryAccounts.filter((item) => item.isActive);
    }
    return overview.internalCandidates.filter((item) => item.isActive);
  };
  const totalSudahSekolah = schoolMonitoringMonthRows.filter(
    (item) => item.mengisi === "Sudah"
  ).length;
  const totalBelumSekolah = schoolMonitoringMonthRows.filter(
    (item) => item.mengisi === "Belum"
  ).length;
  const totalTidakSekolah = schoolMonitoringMonthRows.filter(
    (item) => item.mengisi === "Tidak"
  ).length;
  const totalSudahPengawas = supervisorMonitoringMonthRows.filter(
    (item) => item.penilaian === "Sudah"
  ).length;
  const totalBelumPengawas = supervisorMonitoringMonthRows.filter(
    (item) => item.penilaian === "Belum"
  ).length;

  if (status === "loading") {
    return (
      <div className="rounded-xl border border-dashed border-slate-200 bg-white p-6 text-sm text-slate-500">
        Memuat akses pengaturan GASPOL...
      </div>
    );
  }

  if (status === "authenticated" && !isAdminMaster) {
    return (
      <div className="space-y-4">
        <div className="rounded-xl border border-red-200 bg-white p-6 text-sm text-red-600">
          Halaman ini hanya dapat diakses oleh admin-master.
        </div>
        <Link
          href="/user/settings"
          className={cn(buttonClass, secondaryButtonClass, "w-fit")}
        >
          Kembali ke Settings
        </Link>
      </div>
    );
  }

  if (!canAccess) {
    return (
      <div className="rounded-xl border border-dashed border-slate-200 bg-white p-6 text-sm text-slate-500">
        Mengalihkan ke login...
      </div>
    );
  }

  return (
    <section className="space-y-6">
      <PageHeader
        eyebrow="ADMIN MASTER"
        title="Pengaturan GASPOL Bang"
        description="Pusat pengelolaan peserta, akun GASPOL, delegasi pengawas, Koor GASPOL, Validator Cabdin, juri, monitoring, dan audit log."
        actions={
          <>
            <Link
              href="/user/settings"
              className={cn(buttonClass, secondaryButtonClass)}
            >
              Kembali ke Settings
            </Link>
            <button
              type="button"
              onClick={handleRefresh}
              className={cn(buttonClass, secondaryButtonClass)}
            >
              <RefreshCcw
                className={cn("mr-2 h-4 w-4", isPending && "animate-spin")}
              />
              Refresh
            </button>
          </>
        }
      />

      <Card className="border-slate-200/80">
        <CardContent className="grid gap-4 p-5 lg:grid-cols-[1fr_auto] lg:items-end">
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <SummaryCard
              title="Peserta"
              value={overview.summary.participantCount}
              icon={School}
            />
            <SummaryCard
              title="Pengawas"
              value={overview.summary.supervisorCount}
              icon={Users}
            />
            <SummaryCard
              title="Validator Cabdin"
              value={overview.summary.validatorCount}
              icon={ShieldCheck}
            />
            <SummaryCard
              title="Juri"
              value={overview.summary.juryCount}
              icon={Gavel}
            />
          </div>
          <div className="flex flex-wrap items-end gap-3">
            <div className="w-full sm:w-[140px]">
              <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                Tahun
              </label>
              <input
                type="number"
                min={2024}
                value={year}
                onChange={(event) =>
                  setYear(Number(event.target.value || currentYear))
                }
                className={inputClass}
              />
            </div>
            <div className="space-y-1">
              <StatusBadge
                variant={overview.summary.isYearActive ? "success" : "warning"}
              >
                {overview.summary.isYearActive
                  ? "Tahun aktif"
                  : "Tahun belum aktif"}
              </StatusBadge>
              {!overview.summary.isYearActive ? (
                <button
                  type="button"
                  onClick={handleActivateYear}
                  disabled={busyKey === "activate-year"}
                  className={cn(buttonClass, primaryButtonClass, "w-full")}
                >
                  {busyKey === "activate-year"
                    ? "Mengaktifkan..."
                    : "Aktifkan Tahun"}
                </button>
              ) : null}
            </div>
          </div>
        </CardContent>
      </Card>

      {error ? (
        <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      ) : null}
      {message ? (
        <div className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
          {message}
        </div>
      ) : null}
      {resetCredential ? (
        <div className="rounded-xl border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-800">
          <span className="font-semibold">{resetCredential.username}</span> -
          password baru:{" "}
          <span className="font-semibold">{resetCredential.password}</span>
        </div>
      ) : null}
      {generatedCredentials.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Credential Baru GASPOL</CardTitle>
            <CardDescription>
              Catat username dan password yang baru dibuat pada proses terakhir.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Username</TableHead>
                  <TableHead>Password</TableHead>
                  <TableHead>Sekolah</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {generatedCredentials.map((item) => (
                  <TableRow key={item.userId}>
                    <TableCell className="font-semibold text-slate-900">
                      {item.username}
                    </TableCell>
                    <TableCell>{item.password}</TableCell>
                    <TableCell>{item.schoolName || item.displayName}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}

      <SectionNav />

      {loading ? (
        <Card>
          <CardContent className="p-8 text-sm text-slate-500">
            Memuat pengaturan GASPOL...
          </CardContent>
        </Card>
      ) : null}

      {!loading ? (
        <>
          <Card id="peserta" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Peserta GASPOL</CardTitle>
              <CardDescription>
                Aktifkan sekolah peserta GASPOL dari master sekolah existing.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 lg:grid-cols-[1.4fr_180px_180px]">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Sekolah
                  </label>
                  <input
                    value={participantQuery}
                    onChange={(event) =>
                      setParticipantQuery(event.target.value)
                    }
                    placeholder="Cari NPSN atau nama sekolah"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Jenjang
                  </label>
                  <select
                    value={participantJenjang}
                    onChange={(event) =>
                      setParticipantJenjang(event.target.value)
                    }
                    className={inputClass}
                  >
                    <option value="ALL">Semua</option>
                    <option value="SMA">SMA</option>
                    <option value="SMK">SMK</option>
                    <option value="SLB">SLB</option>
                  </select>
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Status Sekolah
                  </label>
                  <select
                    value={participantStatus}
                    onChange={(event) =>
                      setParticipantStatus(event.target.value)
                    }
                    className={inputClass}
                  >
                    <option value="ALL">Semua</option>
                    <option value="NEGERI">Negeri</option>
                    <option value="SWASTA">Swasta</option>
                  </select>
                </div>
              </div>

              {participantRows.length === 0 ? (
                <EmptyState
                  title="Belum ada data peserta"
                  description="Data sekolah belum tersedia atau filter tidak menemukan hasil."
                  icon={<School className="h-5 w-5" />}
                />
              ) : (
                <Table wrapperClassName="max-h-[480px] overflow-y-auto">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        NPSN
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Sekolah
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Jenjang
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Status Sekolah
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Peserta GASPOL
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Aksi
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {participantRows.map((item) => (
                      <TableRow key={item.schoolId ?? `participant-${item.id}`}>
                        <TableCell>{item.npsn || "-"}</TableCell>
                        <TableCell className="font-semibold text-slate-900">
                          {item.schoolName}
                        </TableCell>
                        <TableCell>{item.schoolLevel || "-"}</TableCell>
                        <TableCell>{item.schoolStatus || "-"}</TableCell>
                        <TableCell>
                          <StatusBadge
                            variant={item.isActive ? "success" : "warning"}
                          >
                            {item.isActive ? "Aktif" : "Nonaktif"}
                          </StatusBadge>
                        </TableCell>
                        <TableCell>
                          {item.schoolId ? (
                            <button
                              type="button"
                              disabled={
                                busyKey === `participant-${item.schoolId}`
                              }
                              onClick={() =>
                                void handleToggleParticipant(
                                  item.schoolId as number,
                                  !item.isActive
                                )
                              }
                              className={cn(
                                buttonClass,
                                item.isActive
                                  ? warningButtonClass
                                  : primaryButtonClass
                              )}
                            >
                              {busyKey === `participant-${item.schoolId}`
                                ? "Menyimpan..."
                                : item.isActive
                                  ? "Nonaktifkan"
                                  : "Aktifkan"}
                            </button>
                          ) : (
                            <span className="text-xs text-slate-500">
                              Data tidak lengkap
                            </span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card id="user-sekolah" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>User Sekolah GASPOL</CardTitle>
              <CardDescription>
                Generate akun dari master sekolah existing tanpa membuat
                duplikasi akun.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
                <div className="w-full md:max-w-md">
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Sekolah / Username
                  </label>
                  <input
                    value={schoolUserQuery}
                    onChange={(event) => setSchoolUserQuery(event.target.value)}
                    placeholder="Cari sekolah atau username"
                    className={inputClass}
                  />
                </div>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <button
                    type="button"
                    onClick={() => void handleDownloadSchoolUsersExport()}
                    disabled={busyKey === "export-school-users"}
                    className={cn(buttonClass, secondaryButtonClass)}
                  >
                    <Download className="mr-2 h-4 w-4" />
                    {busyKey === "export-school-users"
                      ? "Mengunduh..."
                      : "Download Excel"}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleBulkGenerateUsers()}
                    disabled={busyKey === "bulk-generate-users"}
                    className={cn(buttonClass, primaryButtonClass)}
                  >
                    {busyKey === "bulk-generate-users"
                      ? "Memproses..."
                      : "Bulk Generate User GASPOL"}
                  </button>
                </div>
                <p className="text-xs text-slate-500">
                  Password pada export adalah password awal saat akun dibuat.
                  Jika password sudah di-reset, gunakan fitur reset password.
                </p>
              </div>

              {schoolUserRows.length === 0 ? (
                <EmptyState
                  title="Belum ada user sekolah GASPOL"
                  description="Aktifkan peserta terlebih dahulu agar user sekolah dapat dibuat."
                  icon={<Users className="h-5 w-5" />}
                />
              ) : (
                <Table wrapperClassName="max-h-[480px] overflow-y-auto">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        NPSN
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Sekolah
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Username
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Status Akun
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Reset Password
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Aksi
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {schoolUserRows.map((item) => (
                      <TableRow key={item.schoolId}>
                        <TableCell>{item.npsn || "-"}</TableCell>
                        <TableCell className="font-semibold text-slate-900">
                          {item.schoolName}
                        </TableCell>
                        <TableCell>{item.username || "-"}</TableCell>
                        <TableCell>
                          {item.userId ? (
                            <div className="flex flex-wrap gap-2">
                              <StatusBadge variant="success">
                                {item.accountStatus}
                              </StatusBadge>
                              <StatusBadge
                                variant={
                                  item.roleIsActive ? "success" : "warning"
                                }
                              >
                                {item.roleIsActive ? "Aktif" : "Role nonaktif"}
                              </StatusBadge>
                            </div>
                          ) : (
                            <StatusBadge variant="warning">
                              Belum tersedia
                            </StatusBadge>
                          )}
                        </TableCell>
                        <TableCell>
                          {item.userId ? (
                            <button
                              type="button"
                              onClick={() =>
                                void handleResetPassword(item.userId as number)
                              }
                              disabled={busyKey === `reset-${item.userId}`}
                              className={cn(buttonClass, warningButtonClass)}
                            >
                              {busyKey === `reset-${item.userId}`
                                ? "Mereset..."
                                : "Reset Password"}
                            </button>
                          ) : (
                            <span className="text-xs text-slate-500">
                              Generate dulu
                            </span>
                          )}
                        </TableCell>
                        <TableCell>
                          <StatusBadge
                            variant={item.isParticipant ? "success" : "neutral"}
                          >
                            {item.isParticipant
                              ? "Peserta aktif"
                              : "Non-peserta"}
                          </StatusBadge>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card id="delegasi-pengawas" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Delegasi Pengawas</CardTitle>
              <CardDescription>
                Aktifkan akun existing sebagai pengawas GASPOL, assign manual,
                atau generate random dari backend.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-3 lg:grid-cols-[1fr_180px_180px]">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Akun Pengawas
                  </label>
                  <input
                    value={supervisorQuery}
                    onChange={(event) => setSupervisorQuery(event.target.value)}
                    placeholder="Cari nama, username, atau role"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Bulan Assignment
                  </label>
                  <select
                    value={assignmentMonth}
                    onChange={(event) =>
                      setAssignmentMonth(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    {monthOptions
                      .filter((item) => item.value <= 9)
                      .map((item) => (
                        <option key={item.value} value={item.value}>
                          {item.label}
                        </option>
                      ))}
                  </select>
                </div>
                <div className="flex items-end gap-2">
                  <button
                    type="button"
                    onClick={() => void handleRandomAssignments("generate")}
                    disabled={busyKey === "generate-random"}
                    className={cn(buttonClass, primaryButtonClass, "flex-1")}
                  >
                    {busyKey === "generate-random"
                      ? "Memproses..."
                      : "Generate Random"}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleRandomAssignments("regenerate")}
                    disabled={busyKey === "regenerate-random"}
                    className={cn(buttonClass, secondaryButtonClass, "flex-1")}
                  >
                    {busyKey === "regenerate-random"
                      ? "Memproses..."
                      : "Regenerate Random"}
                  </button>
                </div>
              </div>

              {supervisorCandidates.length === 0 ? (
                <EmptyState
                  title="Belum ada kandidat pengawas"
                  description="Tidak ada akun existing yang siap dipakai sebagai pengawas GASPOL."
                  icon={<ShieldCheck className="h-5 w-5" />}
                />
              ) : (
                <Table wrapperClassName="max-h-[560px] overflow-y-auto">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Pengawas
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Username
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Base Role
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Assignment
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Status
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {supervisorCandidates.map((item) => (
                      <TableRow key={item.userId}>
                        <TableCell className="font-semibold text-slate-900">
                          {item.displayName}
                        </TableCell>
                        <TableCell>{item.username}</TableCell>
                        <TableCell>{item.baseRole}</TableCell>
                        <TableCell>{item.assignmentCount}</TableCell>
                        <TableCell>
                          <button
                            type="button"
                            onClick={() =>
                              void handleRoleToggle(
                                "SUPERVISOR",
                                item.userId,
                                !item.isActive
                              )
                            }
                            disabled={busyKey === `SUPERVISOR-${item.userId}`}
                            className={cn(
                              buttonClass,
                              item.isActive
                                ? warningButtonClass
                                : primaryButtonClass
                            )}
                          >
                            {busyKey === `SUPERVISOR-${item.userId}`
                              ? "Menyimpan..."
                              : item.isActive
                                ? "Nonaktifkan"
                                : "Aktifkan"}
                          </button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}

              <div className="grid gap-3 xl:grid-cols-[1fr_1fr_auto]">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Sekolah
                  </label>
                  <select
                    value={selectedSupervisorSchoolId}
                    onChange={(event) =>
                      setSelectedSupervisorSchoolId(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    <option value={0}>Pilih sekolah peserta</option>
                    {activeParticipants.map((item) => (
                      <option key={item.schoolId} value={item.schoolId}>
                        {`${item.npsn || "-"} - ${item.schoolName}`}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Pengawas
                  </label>
                  <select
                    value={selectedSupervisorUserId}
                    onChange={(event) =>
                      setSelectedSupervisorUserId(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    <option value={0}>Pilih pengawas aktif</option>
                    {activeSupervisorOptions.map((item) => (
                      <option key={item.userId} value={item.userId}>
                        {`${item.displayName} (${item.username})`}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="flex items-end">
                  <button
                    type="button"
                    onClick={() => void handleManualAssignment()}
                    disabled={busyKey === "manual-assignment"}
                    className={cn(buttonClass, primaryButtonClass)}
                  >
                    {busyKey === "manual-assignment"
                      ? "Menyimpan..."
                      : "Assign Manual"}
                  </button>
                </div>
              </div>

              <div className="grid gap-3 lg:grid-cols-2">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Sekolah Assignment
                  </label>
                  <input
                    value={assignmentSearchSchool}
                    onChange={(event) =>
                      setAssignmentSearchSchool(event.target.value)
                    }
                    placeholder="Cari sekolah"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Pengawas Assignment
                  </label>
                  <input
                    value={assignmentSearchSupervisor}
                    onChange={(event) =>
                      setAssignmentSearchSupervisor(event.target.value)
                    }
                    placeholder="Cari pengawas"
                    className={inputClass}
                  />
                </div>
              </div>

              {assignmentRows.length === 0 ? (
                <EmptyState
                  title="Belum ada assignment pengawas"
                  description="Generate random atau assign manual untuk bulan yang dipilih."
                  icon={<ClipboardList className="h-5 w-5" />}
                />
              ) : (
                <Table wrapperClassName="max-h-[420px] overflow-y-auto">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Pengawas
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Sekolah
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Bulan
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Status
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {assignmentRows.map((item) => (
                      <TableRow key={item.assignmentId}>
                        <TableCell>{item.supervisorName}</TableCell>
                        <TableCell className="font-semibold text-slate-900">
                          {item.schoolName}
                        </TableCell>
                        <TableCell>{monthName(item.month)}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-2">
                            <StatusBadge
                              variant={
                                item.penilaian === "Sudah"
                                  ? "success"
                                  : "warning"
                              }
                            >
                              {item.penilaian}
                            </StatusBadge>
                            <StatusBadge variant="neutral">
                              {item.method}
                            </StatusBadge>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card id="penugasan-penilai" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Penugasan Penilai</CardTitle>
              <CardDescription>
                Penugasan per sekolah dan tahun menggunakan stage assignment
                existing. Satu sekolah dapat memiliki beberapa penilai aktif.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <MonthlyFieldShortcut
                schools={stageSchoolsFor("FIELD_STAGE_1")}
                assessors={stageAssessorsFor("FIELD_STAGE_1")}
                busyKey={busyKey}
                onSubmit={handleMonthlyFieldShortcut}
              />
              {stageAssignmentConfigs.map((config) => (
                <StageAssignmentSection
                  key={config.stage}
                  config={config}
                  schools={stageSchoolsFor(config.stage)}
                  assessors={stageAssessorsFor(config.stage)}
                  assignments={overview.stageAssignments}
                  busyKey={busyKey}
                  onUpdate={handleStageAssignment}
                />
              ))}
            </CardContent>
          </Card>

          <Card id="koor-gaspol" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Koor GASPOL</CardTitle>
              <CardDescription>
                Tetapkan satu akun PENGAWAS existing sebagai Koor GASPOL untuk
                tahun {year}. Assignment ini tidak mengubah role permanen akun.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="rounded-2xl border border-sky-200 bg-sky-50 p-4 text-sm text-sky-950">
                <p className="font-semibold">Alur validasi dua tingkat</p>
                <p className="mt-1">
                  Penilai → Koor GASPOL (Pengawas) → Validator Cabdin → Final
                </p>
                <p className="mt-2 text-sky-800">
                  Validator Cabdin hanya dapat memfinalisasi nilai baru setelah
                  disetujui Koor GASPOL. Nilai lama yang sudah final tetap
                  final.
                </p>
              </div>

              <div className="grid gap-3 xl:grid-cols-[1fr_auto] xl:items-end">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Akun PENGAWAS
                  </label>
                  <select
                    value={selectedKoorUserId}
                    onChange={(event) =>
                      setSelectedKoorUserId(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    <option value={0}>Pilih Koor GASPOL</option>
                    {koorOptions.map((item) => (
                      <option key={item.userId} value={item.userId}>
                        {item.displayName} ({item.username})
                      </option>
                    ))}
                  </select>
                </div>
                <button
                  type="button"
                  onClick={() =>
                    void handleKoorAssignment(selectedKoorUserId, true)
                  }
                  disabled={!selectedKoorUserId || busyKey === "koor-gaspol"}
                  className={cn(buttonClass, primaryButtonClass)}
                >
                  {busyKey === "koor-gaspol"
                    ? "Menyimpan..."
                    : activeKoorAssignment
                      ? "Ganti Koor GASPOL"
                      : "Assign Koor GASPOL"}
                </button>
              </div>

              <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                      Koor aktif tahun {year}
                    </p>
                    <p className="mt-2 font-semibold text-slate-950">
                      {activeKoorAssignment?.assessorName || "Belum ditetapkan"}
                    </p>
                    {activeKoorAssignment ? (
                      <p className="mt-1 text-sm text-slate-600">
                        {activeKoorAssignment.username} ·{" "}
                        {activeKoorAssignment.baseRole}
                      </p>
                    ) : null}
                  </div>
                  <div className="flex items-center gap-2">
                    <StatusBadge
                      variant={activeKoorAssignment ? "success" : "warning"}
                    >
                      {activeKoorAssignment ? "Aktif" : "Belum assigned"}
                    </StatusBadge>
                    {activeKoorAssignment ? (
                      <button
                        type="button"
                        onClick={() =>
                          void handleKoorAssignment(
                            activeKoorAssignment.assessorUserId,
                            false
                          )
                        }
                        disabled={busyKey === "koor-gaspol"}
                        className={cn(buttonClass, warningButtonClass)}
                      >
                        Unassign
                      </button>
                    ) : null}
                  </div>
                </div>
              </div>

              {overview.koorAssignments.length > 0 ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Riwayat Koor</TableHead>
                      <TableHead>Role Dasar</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Perubahan Terakhir</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {overview.koorAssignments.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell className="font-semibold text-slate-900">
                          {item.assessorName}
                        </TableCell>
                        <TableCell>{item.baseRole}</TableCell>
                        <TableCell>
                          <StatusBadge
                            variant={item.isActive ? "success" : "neutral"}
                          >
                            {item.isActive ? "Aktif" : "Tidak aktif"}
                          </StatusBadge>
                        </TableCell>
                        <TableCell>
                          {formatIndoDateTime(item.updatedAt)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ) : null}
            </CardContent>
          </Card>

          <Card id="validator-cabdin" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Validator Cabdin</CardTitle>
              <CardDescription>
                Tahap kedua setelah approval Koor GASPOL. Gunakan akun Admin
                Cabdin existing dan assign ke sekolah peserta; field
                validator_user_id tetap dipertahankan untuk kompatibilitas.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-3 xl:grid-cols-[1fr_1fr_1fr_auto]">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Validator
                  </label>
                  <input
                    value={validatorQuery}
                    onChange={(event) => setValidatorQuery(event.target.value)}
                    placeholder="Cari nama, username, atau role"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Sekolah Peserta
                  </label>
                  <select
                    value={selectedValidatorSchoolId}
                    onChange={(event) =>
                      setSelectedValidatorSchoolId(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    <option value={0}>Pilih sekolah peserta</option>
                    {activeParticipants.map((item) => (
                      <option key={item.schoolId} value={item.schoolId}>
                        {`${item.npsn || "-"} - ${item.schoolName}`}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Validator Aktif
                  </label>
                  <select
                    value={selectedValidatorUserId}
                    onChange={(event) =>
                      setSelectedValidatorUserId(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    <option value={0}>Pilih validator aktif</option>
                    {activeValidatorOptions.map((item) => (
                      <option key={item.userId} value={item.userId}>
                        {`${item.displayName} (${item.username})`}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="flex items-end">
                  <button
                    type="button"
                    onClick={() => void handleAssignValidator()}
                    disabled={busyKey === "assign-validator"}
                    className={cn(buttonClass, primaryButtonClass)}
                  >
                    {busyKey === "assign-validator"
                      ? "Menyimpan..."
                      : "Assign Validator"}
                  </button>
                </div>
              </div>

              <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-4">
                <div className="grid gap-3 xl:grid-cols-[1fr_auto] xl:items-end">
                  <div>
                    <label className="text-xs font-semibold tracking-[0.18em] text-amber-800 uppercase">
                      Validator untuk Semua Sekolah Peserta
                    </label>
                    <select
                      value={selectedBulkValidatorUserId}
                      onChange={(event) =>
                        setSelectedBulkValidatorUserId(
                          Number(event.target.value)
                        )
                      }
                      className={inputClass}
                    >
                      <option value={0}>
                        Pilih validator untuk semua peserta
                      </option>
                      {activeValidatorOptions.map((item) => (
                        <option key={item.userId} value={item.userId}>
                          {`${item.displayName} (${item.username})`}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="flex items-end">
                    <button
                      type="button"
                      onClick={() => void handleAssignValidatorAll()}
                      disabled={busyKey === "assign-validator-all"}
                      className={cn(buttonClass, warningButtonClass)}
                    >
                      {busyKey === "assign-validator-all"
                        ? "Menerapkan..."
                        : "Terapkan ke semua sekolah peserta"}
                    </button>
                  </div>
                </div>
                <p className="mt-3 text-xs text-amber-900">
                  Perubahan ini akan mengganti validator aktif untuk semua
                  sekolah peserta pada tahun ini. Histori validasi yang sudah
                  dilakukan sebelumnya tidak diubah.
                </p>
              </div>

              {validatorCandidates.length === 0 ? (
                <EmptyState
                  title="Belum ada kandidat validator"
                  description="Tidak ada akun existing yang siap dipakai sebagai validator GASPOL."
                  icon={<ShieldCheck className="h-5 w-5" />}
                />
              ) : (
                <Table wrapperClassName="max-h-[420px] overflow-y-auto">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Validator
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Username
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Base Role
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Sekolah
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Status
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {validatorCandidates.map((item) => (
                      <TableRow key={item.userId}>
                        <TableCell className="font-semibold text-slate-900">
                          {item.displayName}
                        </TableCell>
                        <TableCell>{item.username}</TableCell>
                        <TableCell>{item.baseRole}</TableCell>
                        <TableCell>{item.assignedSchoolCount}</TableCell>
                        <TableCell className="space-x-2">
                          <button
                            type="button"
                            onClick={() => {
                              setSelectedValidatorUserId(item.userId);
                              setSelectedBulkValidatorUserId(item.userId);
                            }}
                            className={cn(buttonClass, secondaryButtonClass)}
                          >
                            Pilih
                          </button>
                          <button
                            type="button"
                            onClick={() =>
                              void handleRoleToggle(
                                "VALIDATOR",
                                item.userId,
                                !item.isActive
                              )
                            }
                            disabled={busyKey === `VALIDATOR-${item.userId}`}
                            className={cn(
                              buttonClass,
                              item.isActive
                                ? warningButtonClass
                                : primaryButtonClass
                            )}
                          >
                            {busyKey === `VALIDATOR-${item.userId}`
                              ? "Menyimpan..."
                              : item.isActive
                                ? "Nonaktifkan"
                                : "Aktifkan"}
                          </button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}

              {validatorAssignments.length === 0 ? (
                <EmptyState
                  title="Belum ada assignment validator"
                  description="Belum ada sekolah peserta aktif yang membutuhkan validator."
                  icon={<ClipboardList className="h-5 w-5" />}
                />
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Sekolah</TableHead>
                      <TableHead>Validator</TableHead>
                      <TableHead>Status</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {validatorAssignments.map((item) => (
                      <TableRow key={item.schoolId}>
                        <TableCell className="font-semibold text-slate-900">
                          {item.schoolName}
                        </TableCell>
                        <TableCell>{item.validatorName || "-"}</TableCell>
                        <TableCell>
                          <StatusBadge
                            variant={
                              item.validatorUserId ? "success" : "warning"
                            }
                          >
                            {item.validatorUserId
                              ? "Sudah assigned"
                              : "Belum assigned"}
                          </StatusBadge>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card id="akun-juri" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Akun Juri</CardTitle>
              <CardDescription>
                Satu akun juri dipakai untuk lapangan dan presentasi.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-3 xl:grid-cols-[1fr_1fr_auto]">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Username Juri
                  </label>
                  <input
                    value={juryUsername}
                    onChange={(event) => setJuryUsername(event.target.value)}
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Nama Tampilan
                  </label>
                  <input
                    value={juryDisplayName}
                    onChange={(event) => setJuryDisplayName(event.target.value)}
                    className={inputClass}
                  />
                </div>
                <div className="flex items-end">
                  <button
                    type="button"
                    onClick={() => void handleGenerateJury()}
                    disabled={busyKey === "generate-jury"}
                    className={cn(buttonClass, primaryButtonClass)}
                  >
                    {busyKey === "generate-jury"
                      ? "Memproses..."
                      : "Generate Akun Juri"}
                  </button>
                </div>
              </div>

              <div>
                <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                  Cari Akun Juri
                </label>
                <input
                  value={juryQuery}
                  onChange={(event) => setJuryQuery(event.target.value)}
                  placeholder="Cari username atau nama juri"
                  className={inputClass}
                />
              </div>

              {juryRows.length === 0 ? (
                <EmptyState
                  title="Belum ada akun juri"
                  description="Generate akun juri pertama untuk memulai penilaian lapangan dan presentasi."
                  icon={<Gavel className="h-5 w-5" />}
                />
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Username</TableHead>
                      <TableHead>Status Akun</TableHead>
                      <TableHead>Reset Password</TableHead>
                      <TableHead>Status Aktif</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {juryRows.map((item) => (
                      <TableRow key={item.userId}>
                        <TableCell className="font-semibold text-slate-900">
                          {item.username}
                        </TableCell>
                        <TableCell>
                          <StatusBadge variant="success">Tersedia</StatusBadge>
                        </TableCell>
                        <TableCell>
                          <button
                            type="button"
                            onClick={() =>
                              void handleResetPassword(item.userId)
                            }
                            disabled={busyKey === `reset-${item.userId}`}
                            className={cn(buttonClass, warningButtonClass)}
                          >
                            {busyKey === `reset-${item.userId}`
                              ? "Mereset..."
                              : "Reset Password"}
                          </button>
                        </TableCell>
                        <TableCell>
                          <button
                            type="button"
                            onClick={() =>
                              void handleRoleToggle(
                                "JURY",
                                item.userId,
                                !item.isActive
                              )
                            }
                            disabled={busyKey === `JURY-${item.userId}`}
                            className={cn(
                              buttonClass,
                              item.isActive
                                ? warningButtonClass
                                : primaryButtonClass
                            )}
                          >
                            {busyKey === `JURY-${item.userId}`
                              ? "Menyimpan..."
                              : item.isActive
                                ? "Nonaktifkan"
                                : "Aktifkan"}
                          </button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card id="monitoring-sekolah" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Monitoring Sekolah</CardTitle>
              <CardDescription>
                Ringkasan pengisian GASPOL per sekolah peserta dan per bulan.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 md:grid-cols-4">
                <SummaryCard
                  title="Total Peserta"
                  value={schoolMonitoringMonthRows.length}
                  icon={School}
                />
                <SummaryCard
                  title="Sudah Mengisi"
                  value={totalSudahSekolah}
                  icon={ClipboardList}
                />
                <SummaryCard
                  title="Belum Mengisi"
                  value={totalBelumSekolah}
                  icon={Settings2}
                />
                <SummaryCard
                  title="Tidak Mengisi"
                  value={totalTidakSekolah}
                  icon={ScrollText}
                />
              </div>

              <div className="grid gap-3 xl:grid-cols-[1.2fr_200px_200px_auto]">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Sekolah
                  </label>
                  <input
                    value={schoolMonitoringQuery}
                    onChange={(event) =>
                      setSchoolMonitoringQuery(event.target.value)
                    }
                    placeholder="Cari NPSN atau nama sekolah"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Bulan
                  </label>
                  <select
                    value={schoolMonitoringMonth}
                    onChange={(event) =>
                      setSchoolMonitoringMonth(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    {monthOptions.map((item) => (
                      <option key={item.value} value={item.value}>
                        {item.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Status Mengisi
                  </label>
                  <select
                    value={schoolMonitoringStatus}
                    onChange={(event) =>
                      setSchoolMonitoringStatus(event.target.value)
                    }
                    className={inputClass}
                  >
                    <option value="ALL">Semua</option>
                    <option value="Sudah">Sudah</option>
                    <option value="Belum">Belum</option>
                    <option value="Tidak">Tidak</option>
                  </select>
                </div>
                <div className="flex items-end">
                  <button
                    type="button"
                    onClick={() =>
                      void handleDownloadExport(schoolMonitoringMonth)
                    }
                    disabled={busyKey === `export-${schoolMonitoringMonth}`}
                    className={cn(buttonClass, secondaryButtonClass)}
                  >
                    <Download className="mr-2 h-4 w-4" />
                    Download Excel
                  </button>
                </div>
              </div>

              {filteredSchoolMonitoringRows.length === 0 ? (
                <EmptyState
                  title="Monitoring sekolah kosong"
                  description="Belum ada data monitoring sekolah untuk filter yang dipilih."
                  icon={<School className="h-5 w-5" />}
                />
              ) : (
                <Table wrapperClassName="max-h-[420px] overflow-y-auto">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        NPSN
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Sekolah
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Bulan
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Mengisi
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredSchoolMonitoringRows.map((item) => (
                      <TableRow key={`${item.schoolId}-${item.month}`}>
                        <TableCell>{item.npsn || "-"}</TableCell>
                        <TableCell className="font-semibold text-slate-900">
                          {item.schoolName}
                        </TableCell>
                        <TableCell>{monthName(item.month)}</TableCell>
                        <TableCell>
                          <StatusBadge
                            variant={
                              item.mengisi === "Sudah"
                                ? "success"
                                : item.mengisi === "Belum"
                                  ? "warning"
                                  : "danger"
                            }
                          >
                            {item.mengisi}
                          </StatusBadge>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card id="monitoring-pengawas" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Monitoring Pengawas</CardTitle>
              <CardDescription>
                Progress penilaian pengawas berdasarkan assignment yang
                terbentuk.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 md:grid-cols-3">
                <SummaryCard
                  title="Total Assignment"
                  value={supervisorMonitoringMonthRows.length}
                  icon={ClipboardList}
                />
                <SummaryCard
                  title="Sudah Menilai"
                  value={totalSudahPengawas}
                  icon={ShieldCheck}
                />
                <SummaryCard
                  title="Belum Menilai"
                  value={totalBelumPengawas}
                  icon={Users}
                />
              </div>

              <div className="grid gap-3 xl:grid-cols-[1fr_1fr_200px_200px_auto]">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Pengawas
                  </label>
                  <input
                    value={supervisorMonitoringQuery}
                    onChange={(event) =>
                      setSupervisorMonitoringQuery(event.target.value)
                    }
                    placeholder="Cari pengawas"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Cari Sekolah
                  </label>
                  <input
                    value={supervisorMonitoringSchoolQuery}
                    onChange={(event) =>
                      setSupervisorMonitoringSchoolQuery(event.target.value)
                    }
                    placeholder="Cari sekolah"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Bulan
                  </label>
                  <select
                    value={supervisorMonitoringMonth}
                    onChange={(event) =>
                      setSupervisorMonitoringMonth(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    {monthOptions
                      .filter((item) => item.value <= 9)
                      .map((item) => (
                        <option key={item.value} value={item.value}>
                          {item.label}
                        </option>
                      ))}
                  </select>
                </div>
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Status Penilaian
                  </label>
                  <select
                    value={supervisorMonitoringStatus}
                    onChange={(event) =>
                      setSupervisorMonitoringStatus(event.target.value)
                    }
                    className={inputClass}
                  >
                    <option value="ALL">Semua</option>
                    <option value="Sudah">Sudah</option>
                    <option value="Belum">Belum</option>
                  </select>
                </div>
                <div className="flex items-end">
                  <button
                    type="button"
                    onClick={() =>
                      void handleDownloadExport(supervisorMonitoringMonth)
                    }
                    disabled={busyKey === `export-${supervisorMonitoringMonth}`}
                    className={cn(buttonClass, secondaryButtonClass)}
                  >
                    <Download className="mr-2 h-4 w-4" />
                    Download Excel
                  </button>
                </div>
              </div>

              {filteredSupervisorMonitoringRows.length === 0 ? (
                <EmptyState
                  title="Monitoring pengawas kosong"
                  description="Belum ada assignment atau filter tidak menemukan hasil."
                  icon={<Users className="h-5 w-5" />}
                />
              ) : (
                <Table wrapperClassName="max-h-[420px] overflow-y-auto">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Pengawas
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Nama Sekolah
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Bulan
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Penilaian
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredSupervisorMonitoringRows.map((item) => (
                      <TableRow key={item.assignmentId}>
                        <TableCell>{item.supervisorName}</TableCell>
                        <TableCell className="font-semibold text-slate-900">
                          {item.schoolName}
                        </TableCell>
                        <TableCell>{monthName(item.month)}</TableCell>
                        <TableCell>
                          <StatusBadge
                            variant={
                              item.penilaian === "Sudah" ? "success" : "warning"
                            }
                          >
                            {item.penilaian}
                          </StatusBadge>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card id="audit-log" className="scroll-mt-24">
            <CardHeader>
              <CardTitle>Audit Log</CardTitle>
              <CardDescription>
                Jejak aktivitas GASPOL terbaru untuk admin-master.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="w-full md:max-w-md">
                <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                  Cari Audit Log
                </label>
                <input
                  value={auditQuery}
                  onChange={(event) => setAuditQuery(event.target.value)}
                  placeholder="Cari aksi, aktor, sekolah, atau payload"
                  className={inputClass}
                />
              </div>

              {auditRows.length === 0 ? (
                <EmptyState
                  title="Audit log kosong"
                  description="Belum ada aktivitas GASPOL yang tercatat untuk tahun ini."
                  icon={<ScrollText className="h-5 w-5" />}
                />
              ) : (
                <Table wrapperClassName="max-h-[460px] overflow-y-auto">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Waktu
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Aksi
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Aktor
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Sekolah
                      </TableHead>
                      <TableHead className="sticky top-0 z-10 bg-white">
                        Payload
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {auditRows.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>
                          {formatIndoDateTime(item.createdAt) ?? "-"}
                        </TableCell>
                        <TableCell>
                          <div className="space-y-1">
                            <p className="font-semibold text-slate-900">
                              {item.action}
                            </p>
                            <p className="text-xs text-slate-500">
                              {item.entityType || "-"}
                            </p>
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="space-y-1">
                            <p>{item.actorName || "-"}</p>
                            <p className="text-xs text-slate-500">
                              {item.actorRole || "-"}
                            </p>
                          </div>
                        </TableCell>
                        <TableCell>{item.schoolName || "-"}</TableCell>
                        <TableCell className="max-w-[360px]">
                          <code className="text-xs break-words whitespace-pre-wrap text-slate-600">
                            {trimPayload(item.payload)}
                          </code>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </>
      ) : null}

      {assignmentPreview ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/55 p-4">
          <div className="max-h-[92vh] w-full max-w-6xl overflow-hidden rounded-2xl bg-white shadow-2xl">
            <div className="border-b border-slate-200 p-5">
              <h2 className="text-lg font-bold text-slate-950">
                Preview Random Pengawas — {monthName(assignmentPreview.month)}{" "}
                {assignmentPreview.year}
              </h2>
              <p className="mt-1 text-sm text-slate-600">
                Assignment belum berubah. Periksa pemerataan beban berikut, lalu
                konfirmasi untuk menyimpan hasil yang sama dengan preview.
              </p>
            </div>
            <div className="max-h-[68vh] overflow-auto p-5">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Pengawas</TableHead>
                    <TableHead>Bulan lalu</TableHead>
                    <TableHead>Bulan aktif</TableHead>
                    <TableHead>Historis</TableHead>
                    <TableHead>Total berjalan</TableHead>
                    <TableHead>Sekolah</TableHead>
                    <TableHead>Penanda</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {assignmentPreview.rows.map((row) => (
                    <TableRow key={row.supervisorUserId}>
                      <TableCell className="font-semibold">
                        {row.supervisorName}
                      </TableCell>
                      <TableCell>{row.previousMonthCount}</TableCell>
                      <TableCell>{row.currentMonthCount}</TableCell>
                      <TableCell>{row.historicalCount}</TableCell>
                      <TableCell>{row.runningTotal}</TableCell>
                      <TableCell>
                        {row.schoolNames.length > 0
                          ? row.schoolNames.join(", ")
                          : "-"}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {row.newCount > 0 ? (
                            <StatusBadge variant="success">
                              Baru {row.newCount}
                            </StatusBadge>
                          ) : null}
                          {row.manualCount > 0 ? (
                            <StatusBadge variant="neutral">
                              Manual {row.manualCount}
                            </StatusBadge>
                          ) : null}
                          {row.lockedCount > 0 ? (
                            <StatusBadge variant="warning">
                              Terkunci {row.lockedCount}
                            </StatusBadge>
                          ) : null}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              {assignmentPreview.unassignedSchoolIds.length > 0 ? (
                <p className="mt-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-800">
                  {assignmentPreview.unassignedSchoolIds.length} sekolah belum
                  memiliki pengawas karena kandidat aktif tidak tersedia.
                </p>
              ) : null}
            </div>
            <div className="flex justify-end gap-3 border-t border-slate-200 p-5">
              <button
                type="button"
                onClick={() => setAssignmentPreview(null)}
                disabled={busyKey === "confirm-random"}
                className={cn(buttonClass, secondaryButtonClass)}
              >
                Batal / Kembali
              </button>
              <button
                type="button"
                onClick={() => void handleConfirmRandomAssignments()}
                disabled={busyKey === "confirm-random"}
                className={cn(buttonClass, primaryButtonClass)}
              >
                {busyKey === "confirm-random"
                  ? "Menyimpan..."
                  : "Konfirmasi & Simpan"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}
