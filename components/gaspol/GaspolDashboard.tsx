"use client";
import { GaspolDeterminations } from "./GaspolDeterminations";

import {
  GaspolPhaseTools,
  GaspolDelegatedTools,
} from "@/components/gaspol/GaspolPhaseTools";
import { GaspolVotingPanel } from "@/components/gaspol/GaspolVoting";
import {
  GaspolRubricAssessment,
  GaspolInnovationForm,
  GaspolAssessmentDetails,
} from "@/components/gaspol/GaspolRubrics";
import {
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  useTransition,
} from "react";
import { createPortal } from "react-dom";
import {
  AlertTriangle,
  CheckCircle2,
  ClipboardList,
  Download,
  FileSpreadsheet,
  History,
  Info,
  Leaf,
  RefreshCcw,
  Settings,
  ShieldCheck,
  Trophy,
  Users,
} from "lucide-react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import LogoGaspolBang from "@/app/assets/Gaspol-bang.png";
import {
  GaspolSchoolPhase,
  NominationReportPanel,
  ReviewNominationEvidence,
} from "@/components/gaspol/GaspolSchoolPhase";
import {
  assignedStageTabs,
  filterStageTasks,
} from "@/lib/gaspol-stage-navigation";
import { EmptyState } from "@/components/empty-state";
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
import { cn } from "@/lib/utils";

type GaspolView =
  | "PUBLIC"
  | "SCHOOL"
  | "SUPERVISOR"
  | "VALIDATOR"
  | "JURY"
  | "ADMIN_MASTER"
  | "DISABLED";

type ApiEnvelope<T> = {
  data?: T;
  error?: string;
  message?: string;
};

type GaspolSummaryCard = {
  label: string;
  value: number;
};

type GaspolScoreBreakdown = {
  scoreMonthly: number;
  scoreField: number;
  scoreFieldStage1?: number;
  scoreFieldStage2?: number;
  scorePresentation: number;
  scoreNomination?: number;
  scoreAppeal?: number;
  scoreTotal: number;
  maximumScore?: number;
  currentMaxScore?: number;
  contributions?: GaspolScoreContribution[];
};

type GaspolScoreContribution = {
  key: string;
  label: string;
  score?: number | null;
  weightedScore?: number | null;
  weight: number;
  status: string;
};

type GaspolMonthlyComponent = {
  month: number;
  label: string;
  weight: number;
  score: number;
  effectiveScore: number;
  status: string;
  statusLabel: string;
  statusDetail?: string;
  isFinal: boolean;
};

type GaspolRankingStatusSummary = {
  isLive: boolean;
  hasWaiting: boolean;
  hasNotYetDue: boolean;
  waitingComponents: number;
  notYetDueComponents: number;
  zeroFinalComponents: number;
  validatedComponents: number;
  topTenLockReady: boolean;
  rankingMessage: string;
  topTenLockMessage: string;
};

type GaspolRankingRow = {
  monthlyFinal: boolean;
  rank: number;
  schoolId: number;
  schoolName: string;
  npsn: string;
  schoolLevel: string;
  schoolStatus: string;
  stage: string;
  monthlyComponents: GaspolMonthlyComponent[];
  scores: GaspolScoreBreakdown;
  cohort?: string;
  path?: "MAIN" | "NOT_FAILED" | string;
  badge?: string;
  visualTone?: "PINK" | "RED" | "POSITIVE" | string;
};

type GaspolSubmission = {
  id: number;
  year?: number;
  month: number;
  reportTitle: string;
  reportBody: string;
  reportLink: string;
  status: string;
  displayStatus?: string;
  assessmentStatus?: string;
  updatedAt?: string;
};

type GaspolAssessment = {
  id: number;
  month: number;
  schoolId: number;
  schoolName: string;
  assessorName: string;
  stage: string;
  scoreMonthly: number;
  scoreField: number;
  scorePresentation: number;
  notes: string;
  isValidated: boolean;
  validationDecision?: string;
  supervisorScore?: number | null;
  validatorScore?: number | null;
  finalMonthlyScore?: number | null;
  validatorNotes?: string;
};

type GaspolStageAssessment = {
  id: number;
  year: number;
  stage: string;
  schoolId: number;
  schoolName: string;
  assessorName: string;
  rawScore: number;
  workflowStatus: string;
  notes: string;
  reviewerNotes: string;
  submittedAt: string;
};

type GaspolStageTask = {
  assignmentId: number;
  stage: string;
  assignmentRole: string;
  schoolId: number;
  schoolName: string;
  npsn: string;
  assessmentId?: number | null;
  rawScore?: number | null;
  workflowStatus?: string;
  notes: string;
  returnNotes: string;
};

type GaspolAssignment = {
  id: number;
  month: number;
  schoolId: number;
  schoolName: string;
  supervisorNameSnapshot: string;
  method: string;
};

type GaspolSchoolMonthStatus = {
  year: number;
  month: number;
  reportTitle: string;
  reportLink: string;
  displayStatus: string;
  assessmentStatus: string;
  finalScore?: number | null;
};

type GaspolMonthlyWorkflowRow = {
  month: number;
  schoolId: number;
  schoolName: string;
  reportLink: string;
  submissionStatus: string;
  processStatus: string;
  supervisorName: string;
  validatorName: string;
  assessmentId?: number | null;
  stageAssessmentId?: number | null;
  workflowStatus?: string;
  supervisorNotes?: string;
  returnNotes?: string;
  supervisorScore?: number | null;
  validatorScore?: number | null;
  finalMonthlyScore?: number | null;
  validationDecision?: string;
};

type GaspolPublicDashboard = {
  year: number;
  month: number;
  stage: string;
  competitionState: string;
  cards: GaspolSummaryCard[];
  leaderboard: GaspolRankingRow[];
  rankingStatus: GaspolRankingStatusSummary;
  awards?: GaspolPublicAwards;
};

type GaspolPublicAward = {
  awardType: "BEST" | "INNOVATIVE" | "FAVORITE" | "FAILED";
  schoolId?: number | null;
  schoolName: string;
  score?: number | null;
  isFinal: boolean;
};

type GaspolPublicAwards = {
  best: GaspolPublicAward;
  innovative: GaspolPublicAward;
  favorite: GaspolPublicAward;
  failed: GaspolPublicAward[];
};

type GaspolSchoolDashboardData = {
  schoolId: number;
  monthlyFinal: boolean;
  year: number;
  month: number;
  stage: string;
  schoolName: string;
  isParticipant: boolean;
  canEdit: boolean;
  actionsDisabled?: boolean;
  disabledReason: string;
  assignedSupervisor: string;
  assignedValidator: string;
  effectiveTime?: string;
  lockedMonths?: number[];
  scoredMonths?: number[];
  latestSubmission?: GaspolSubmission | null;
  scores: GaspolScoreBreakdown;
  monthlyComponents: GaspolMonthlyComponent[];
  monthlyStatuses: GaspolSchoolMonthStatus[];
  history: GaspolSubmission[];
  cohorts: string[];
  competitionNotice?: string;
};

type GaspolSupervisorDashboardData = {
  year: number;
  month: number;
  assignmentCount: number;
  validatedCount: number;
  pendingCount: number;
  missingSubmissionCount: number;
  assignments: GaspolAssignment[];
  latestAssessments: GaspolAssessment[];
  missingSubmissions: GaspolMonthlyWorkflowRow[];
  pendingReviews: GaspolMonthlyWorkflowRow[];
  completedReviews: GaspolMonthlyWorkflowRow[];
  isKoorGaspol: boolean;
  koorPendingAssessments: GaspolStageAssessment[];
  stageTasks: GaspolStageTask[];
};

type GaspolValidatorDashboardData = {
  year: number;
  month: number;
  pendingCount: number;
  validatedCount: number;
  participantCount: number;
  blockedCount: number;
  pendingAssessments: GaspolAssessment[];
  validatedAssessments: GaspolAssessment[];
  notReadyRows: GaspolMonthlyWorkflowRow[];
  pendingRows: GaspolMonthlyWorkflowRow[];
  validatedRows: GaspolMonthlyWorkflowRow[];
  leaderboard: GaspolRankingRow[];
  validationQueue: GaspolStageAssessment[];
};

type GaspolAccessContext = {
  year: number;
  view: string;
};

type GaspolJuryDashboardData = {
  year: number;
  month: number;
  stage: string;
  candidateCount: number;
  candidates: GaspolRankingRow[];
  latestAssessments: GaspolAssessment[];
  stageTasks: GaspolStageTask[];
};

type GaspolMonitoringSchoolRow = {
  schoolId: number;
  schoolName: string;
  npsn: string;
  assignedSupervisor: string;
  assignedValidator: string;
  isParticipant: boolean;
  lastSubmission?: GaspolSubmission | null;
  scores: GaspolScoreBreakdown;
};

type GaspolMonitoringSupervisorRow = {
  displayName: string;
  assignmentCount: number;
  completedCount: number;
};

type GaspolAdminDashboardData = {
  topTenReady: boolean;
  year: number;
  month: number;
  stage: string;
  participantCount: number;
  supervisorCount: number;
  validatorCount: number;
  juryCount: number;
  pendingValidation: number;
  unassignedCabdinAssessments: number;
  rankingStatus: GaspolRankingStatusSummary;
  topTen: GaspolRankingRow[];
  topFive: GaspolRankingRow[];
};

type GaspolStageOnePreviewRow = {
  rank?: number | null;
  schoolId: number;
  schoolName: string;
  monthlyScore?: number | null;
  monthlyContribution?: number | null;
  fieldStage1Score?: number | null;
  fieldStage1Contribution?: number | null;
  total?: number | null;
  koorStatus: string;
  cabdinStatus: string;
  finalStatus: string;
  workflowStatus: string;
  assessments: {
    id: number;
    assessorName: string;
    rawScore: number;
    workflowStatus: string;
  }[];
};

type GaspolStageOnePreview = {
  year: number;
  stage: string;
  rows: GaspolStageOnePreviewRow[];
  completeness: {
    participants: number;
    monthlyFinal: number;
    fieldStage1Filled: number;
    koorValidated: number;
    cabdinValidated: number;
    pendingOrReturned: number;
  };
  canLockTopTen: boolean;
  disabledReason: string;
  hasBoundaryTie: boolean;
  lock: {
    isLocked: boolean;
    lockId?: number;
    revision?: number;
    lockedAt?: string;
    lockedBy: string;
  };
  officialTopTen: Array<{
    rank: number;
    schoolId: number;
    schoolName: string;
    total: number;
  }>;
};

type ResetPasswordResult = {
  username: string;
  password: string;
  role: string;
  displayName: string;
};

const buttonClass =
  "inline-flex items-center justify-center rounded-lg px-3 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-50";

const inputClass =
  "w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 outline-none transition focus:border-emerald-400";

const gaspolMonthlyTotalWeight = 0.2;
const gaspolScrollableTableClass = "max-h-[36rem] overflow-y-auto";

const monthName = (value: number) =>
  [
    "",
    "Januari",
    "Februari",
    "Maret",
    "April",
    "Mei",
    "Juni",
    "Juli",
    "Agustus",
    "September",
    "Oktober",
    "November",
    "Desember",
  ][value] ?? "";

const schoolMonths = [4, 5, 6, 7, 8];
const reviewMonths = [4, 5, 6, 7, 8, 9];
const gaspolDeadlineNotice =
  "deadline tanggal 15 bulan berikutnya pukul 23:59:59 WIB";
const submittedLockedNotice =
  "Laporan sudah dikirim ke penilai sehingga tidak dapat diedit. Apabila hendak turun status, silakan menghubungi operator Cabdin. *Berlaku hanya sebelum deadline tanggal 15 bulan berikutnya pukul 23:59:59 WIB.";

const formatPeriod = (month: number, year: number) =>
  `${monthName(month)} ${year}`;

const renderReportLinkAction = (reportLink?: string) => {
  const normalizedLink = reportLink?.trim() ?? "";
  if (!normalizedLink) {
    return <span className="text-slate-400">-</span>;
  }
  return (
    <a
      href={normalizedLink}
      target="_blank"
      rel="noreferrer noopener"
      className="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-3 py-1.5 text-xs font-semibold whitespace-nowrap text-sky-700 transition hover:bg-sky-100"
    >
      Buka
    </a>
  );
};

const normalizeGaspolView = (value: string): GaspolView => {
  switch (value) {
    case "SCHOOL_ENTRY":
      return "SCHOOL";
    case "SUPERVISOR":
    case "VALIDATOR":
    case "JURY":
    case "ADMIN_MASTER":
      return value;
    default:
      return "DISABLED";
  }
};

const gaspolViewLabel = (view: GaspolView) => {
  switch (view) {
    case "SCHOOL":
      return "Sekolah";
    case "SUPERVISOR":
      return "Pengawas";
    case "VALIDATOR":
      return "Validator Cabdin";
    case "JURY":
      return "Juri";
    case "ADMIN_MASTER":
      return "Admin Master";
    case "DISABLED":
      return "Akses Belum Aktif";
    default:
      return "Publik";
  }
};

const competitionStateLabel = (state?: string) =>
  ({
    MONTHLY_ACTIVE: "Penilaian Bulanan",
    AUGUST_DIRECT_ASSESSMENT: "Penilaian Juli–Agustus Hybrid",
    SEPTEMBER_FIELD1_ASSESSMENT: "September dan Lapangan Tahap 1",
    WAITING_STAGE1_VALIDATION: "Validasi Lapangan Tahap 1",
    STAGE1_FINAL: "Lapangan Tahap 1 Final",
    TOP10_LOCKED: "10 Besar GASPOL Bang",
    FIELD2_ACTIVE: "Lapangan Tahap 2",
    FIELD2_FINAL: "Lapangan Tahap 2 Final",
    TOP5_WAITING_BA: "Menunggu BA Penetapan 5 Besar",
    TOP5_LOCKED: "5 Besar GASPOL Bang — Finalis Presentasi",
    PRESENTATION_ACTIVE: "Presentasi Finalis",
    NOT_FAILED_NOMINATION_ACTIVE: "Nominasi Tidak Tergagal",
    BOTTOM5_LOCKED: "Banding Kandidat Zona Tergagal",
    APPEAL_FINAL: "Hasil Banding Final",
    GASPOL_FINALIZED: "GASPOL Selesai",
  })[state ?? ""] ?? "Tahapan GASPOL";

const stageTaskLabel = (stage: string) =>
  ({
    MONTHLY: "Bulanan",
    FIELD_STAGE_1: "Lapangan Tahap 1",
    FIELD_STAGE_2: "Lapangan Tahap 2",
    PRESENTATION: "Presentasi",
    NOT_FAILED_NOMINATION: "Nominasi Tidak Tergagal",
    BOTTOM5_APPEAL: "Banding Kandidat Zona Tergagal",
    INNOVATION: "Penilaian Ter-Inovatif",
  })[stage] ?? stage;

const validationStageTabs = [
  "MONTHLY",
  "FIELD_STAGE_1",
  "NOT_FAILED_NOMINATION",
  "BOTTOM5_APPEAL",
  "INNOVATION",
] as const;

type ValidationStage = (typeof validationStageTabs)[number];

const resolveDeterministicGaspolView = (
  role: string | undefined
): GaspolView | null => {
  switch ((role ?? "").trim().toUpperCase()) {
    case "USER_SEKOLAH_GASPOL":
      return "SCHOOL";
    case "PENGAWAS":
      return "SUPERVISOR";
    case "ADMIN_MASTER":
      return "ADMIN_MASTER";
    default:
      return null;
  }
};

const currentYear = new Date().getFullYear();

const currentMonthInRange = (min: number, max: number, fallback: number) => {
  const month = new Date().getMonth() + 1;
  if (month >= min && month <= max) {
    return month;
  }
  return fallback;
};

const getSubmissionValidationErrors = (form: {
  reportTitle: string;
  reportBody: string;
  reportLink: string;
}) => {
  const errors: string[] = [];
  if (form.reportTitle.trim() === "") {
    errors.push("Judul laporan wajib diisi.");
  }
  if (form.reportBody.trim() === "") {
    errors.push("Ringkasan laporan wajib diisi.");
  }
  if (form.reportLink.trim() === "") {
    errors.push("Link laporan wajib diisi.");
  } else {
    try {
      const parsed = new URL(form.reportLink.trim());
      if (!parsed.protocol.startsWith("http")) {
        errors.push("Link laporan harus berupa URL lengkap yang valid.");
      }
    } catch {
      errors.push("Link laporan harus berupa URL lengkap yang valid.");
    }
  }
  return errors;
};

const getSchoolStatusLabel = (
  month: number,
  status: string | undefined,
  lockedMonths: number[],
  scoredMonths: number[]
) => {
  if (scoredMonths.includes(month)) {
    return "Sudah Dinilai";
  }
  if ((status ?? "").toUpperCase() === "SUBMITTED") {
    return "Terkirim ke Penilai";
  }
  if (lockedMonths.includes(month)) {
    return "Terkunci";
  }
  return "Belum Dikirim";
};

const getSchoolStatusVariant = (label: string) => {
  switch (label) {
    case "Sudah Dinilai":
      return "success" as const;
    case "Terkirim ke Penilai":
      return "neutral" as const;
    case "Terkunci":
      return "danger" as const;
    default:
      return "warning" as const;
  }
};

const getMonthlyComponentVariant = (status: string) => {
  switch ((status ?? "").trim().toUpperCase()) {
    case "VALIDATED":
      return "success" as const;
    case "WAITING":
      return "neutral" as const;
    case "ZERO_FINAL":
      return "danger" as const;
    case "NOT_YET_DUE":
      return "warning" as const;
    default:
      return "neutral" as const;
  }
};

const getGaspolScrollableTableProps = (rowCount: number) =>
  rowCount > 10
    ? {
        stickyHeader: true,
        wrapperClassName: gaspolScrollableTableClass,
      }
    : {};

const getRankingStatusMeta = (row: GaspolRankingRow) => {
  const components = row.monthlyComponents ?? [];
  if (components.length === 0) {
    return {
      label: "Belum lengkap",
      variant: "warning" as const,
      description: "Komponen April-September belum tersedia lengkap.",
    };
  }

  const hasWaiting = components.some((item) => item.status === "WAITING");
  const hasZeroFinal = components.some((item) => item.status === "ZERO_FINAL");
  const hasNotYetDue = components.some((item) => item.status === "NOT_YET_DUE");
  const allFinal = components.every((item) => item.isFinal);

  if (hasWaiting) {
    return {
      label: "Waiting",
      variant: "neutral" as const,
      description:
        "Ranking masih sementara karena ada komponen yang menunggu penilaian atau validasi.",
    };
  }
  if (hasNotYetDue) {
    return {
      label: "Belum lengkap",
      variant: "warning" as const,
      description:
        "Ranking belum final karena masih ada komponen yang belum jatuh tempo.",
    };
  }
  if (hasZeroFinal) {
    return {
      label: "Ada 0 Final",
      variant: "danger" as const,
      description:
        "Ada komponen yang final 0 karena sekolah tidak mengirim sampai deadline.",
    };
  }
  if (allFinal) {
    return {
      label: "Final",
      variant: "success" as const,
      description: "Semua komponen April-September sudah final.",
    };
  }
  return {
    label: "Sementara",
    variant: "neutral" as const,
    description: "Ranking masih sementara dan bisa berubah.",
  };
};

function RankingStatusNotice({
  summary,
  title = "Status Ranking Live",
}: {
  summary?: GaspolRankingStatusSummary | null;
  title?: string;
}) {
  if (!summary?.rankingMessage && !summary?.topTenLockMessage) {
    return null;
  }
  return (
    <div className="rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-950">
      <div className="flex items-start gap-3">
        {summary.topTenLockReady ? (
          <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-700" />
        ) : (
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
        )}
        <div className="space-y-1">
          <p className="font-semibold text-slate-900">{title}</p>
          <p>{summary.rankingMessage}</p>
          <p className="text-slate-700">{summary.topTenLockMessage}</p>
        </div>
      </div>
    </div>
  );
}

function MonthlyComponentsCell({
  components,
}: {
  components: GaspolMonthlyComponent[];
}) {
  if (!components.length) {
    return <span className="text-slate-400">Belum ada komponen</span>;
  }
  return (
    <div className="grid min-w-[280px] gap-2 md:grid-cols-2 xl:grid-cols-3">
      {components.map((component) => (
        <div
          key={component.month}
          className="rounded-xl border border-slate-200 bg-slate-50 p-2"
        >
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs font-semibold text-slate-900">
              {component.label}
            </p>
            <span className="text-[11px] text-slate-500">
              {(component.weight * 100).toFixed(0)}%
            </span>
          </div>
          <p className="mt-1 text-sm font-semibold text-slate-900">
            {component.isFinal ? component.score.toFixed(2) : "—"}
          </p>
          <p className="text-[11px] text-slate-500">
            Kontribusi {component.effectiveScore.toFixed(2)}
          </p>
          <div className="mt-2">
            <StatusBadge variant={getMonthlyComponentVariant(component.status)}>
              {component.statusLabel}
            </StatusBadge>
          </div>
        </div>
      ))}
    </div>
  );
}

const contributionByKey = (row: GaspolRankingRow, key: string) =>
  row.scores.contributions?.find((item) => item.key === key);

const contributionDisplay = (
  contribution: GaspolScoreContribution | undefined
) =>
  contribution?.status === "FINAL" && typeof contribution.score === "number"
    ? contribution.score.toFixed(2)
    : "Belum dinilai";

function ScoreContributionCard({
  contribution,
}: {
  contribution: GaspolScoreContribution;
}) {
  const isFinal =
    contribution.status === "FINAL" &&
    typeof contribution.score === "number" &&
    typeof contribution.weightedScore === "number";
  return (
    <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
      <h4 className="font-semibold text-slate-950">{contribution.label}</h4>
      {isFinal ? (
        <dl className="mt-3 space-y-2 text-sm">
          <div className="flex justify-between gap-4">
            <dt className="text-slate-500">Nilai</dt>
            <dd className="font-semibold text-slate-900">
              {contribution.score?.toFixed(2)}
            </dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt className="text-slate-500">Nilai terhadap bobot</dt>
            <dd className="font-semibold text-slate-900">
              {contribution.weightedScore?.toFixed(2)}
            </dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt className="text-slate-500">Bobot</dt>
            <dd className="font-semibold text-slate-900">
              {(contribution.weight * 100).toFixed(0)}%
            </dd>
          </div>
        </dl>
      ) : (
        <p className="mt-3 text-sm font-medium text-amber-700">
          Belum memasuki tahapan / belum dinilai
        </p>
      )}
    </div>
  );
}

type AwardInfoKey = "BEST" | "INNOVATIVE" | "FAVORITE" | "FAILED";

function AwardInfo({
  awardKey,
  label,
  text,
  activeAwardInfo,
  setActiveAwardInfo,
}: {
  awardKey: AwardInfoKey;
  label: string;
  text: string;
  activeAwardInfo: AwardInfoKey | null;
  setActiveAwardInfo: (value: AwardInfoKey | null) => void;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const [panelPosition, setPanelPosition] = useState({ left: 0, top: 0 });
  const isOpen = activeAwardInfo === awardKey;

  useEffect(() => {
    if (!isOpen) return;
    const updatePanelPosition = () => {
      const rect = containerRef.current?.getBoundingClientRect();
      if (!rect) return;
      const panelWidth = Math.min(320, window.innerWidth - 24);
      const left = Math.max(
        12,
        Math.min(
          rect.left + rect.width / 2 - panelWidth / 2,
          window.innerWidth - panelWidth - 12
        )
      );
      const panelHeight = panelRef.current?.offsetHeight ?? 220;
      const fitsBelow =
        rect.bottom + 8 + panelHeight <= window.innerHeight - 12;
      const top = fitsBelow
        ? rect.bottom + 8
        : Math.max(12, rect.top - panelHeight - 8);
      setPanelPosition({ left, top });
    };
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (
        !containerRef.current?.contains(target) &&
        !panelRef.current?.contains(target)
      ) {
        setActiveAwardInfo(null);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setActiveAwardInfo(null);
      }
    };
    updatePanelPosition();
    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    window.addEventListener("resize", updatePanelPosition);
    window.addEventListener("scroll", updatePanelPosition, true);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("resize", updatePanelPosition);
      window.removeEventListener("scroll", updatePanelPosition, true);
    };
  }, [isOpen, setActiveAwardInfo]);

  return (
    <div ref={containerRef} className="relative inline-block">
      <button
        type="button"
        aria-label={`Informasi ${label}`}
        aria-expanded={isOpen}
        onClick={() => setActiveAwardInfo(isOpen ? null : awardKey)}
        className="inline-flex h-7 w-7 items-center justify-center rounded-full border border-white/25 bg-white/10 text-white transition hover:bg-white/20"
      >
        <Info className="h-4 w-4" />
      </button>
      {isOpen && typeof document !== "undefined"
        ? createPortal(
            <div
              ref={panelRef}
              role="dialog"
              aria-label={`Ketentuan ${label}`}
              style={{ left: panelPosition.left, top: panelPosition.top }}
              className="fixed z-[9999] max-h-[calc(100vh-1.5rem)] w-[calc(100vw-1.5rem)] max-w-80 overflow-y-auto rounded-2xl border border-slate-200 bg-white p-4 text-left text-sm leading-6 text-slate-700 shadow-2xl"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="font-semibold text-slate-950">
                    Ketentuan {label}
                  </p>
                  <p className="mt-1">{text}</p>
                </div>
                <button
                  type="button"
                  aria-label="Tutup informasi"
                  onClick={() => setActiveAwardInfo(null)}
                  className="rounded-lg px-2 py-1 text-slate-500 hover:bg-slate-100"
                >
                  ×
                </button>
              </div>
            </div>,
            document.body
          )
        : null}
    </div>
  );
}

function AwardCategoryCard({
  label,
  info,
  awards,
  featured = false,
  awardKey,
  activeAwardInfo,
  setActiveAwardInfo,
}: {
  label: string;
  info: string;
  awards: GaspolPublicAward[];
  featured?: boolean;
  awardKey: AwardInfoKey;
  activeAwardInfo: AwardInfoKey | null;
  setActiveAwardInfo: (value: AwardInfoKey | null) => void;
}) {
  return (
    <div
      className={cn(
        "rounded-3xl border border-white/10 bg-white/5 p-5 text-center backdrop-blur",
        featured && "ring-1 ring-amber-300/60"
      )}
    >
      <div className="flex items-center justify-center gap-2">
        <p className="text-xs font-semibold tracking-[0.18em] text-slate-300 uppercase">
          {label}
        </p>
        <AwardInfo
          awardKey={awardKey}
          label={label}
          text={info}
          activeAwardInfo={activeAwardInfo}
          setActiveAwardInfo={setActiveAwardInfo}
        />
      </div>
      <div
        className={cn(
          "mt-4 gap-3",
          awards.length > 1 ? "grid md:grid-cols-3" : "space-y-3"
        )}
      >
        {awards.map((award, index) => (
          <div
            key={`${award.awardType}-${award.schoolId ?? "pending"}-${index}`}
            className="rounded-2xl bg-white/10 px-4 py-3"
          >
            <p className="font-semibold text-white">
              {award.schoolName || "Belum ditetapkan"}
            </p>
            {award.awardType === "BEST" && typeof award.score === "number" ? (
              <p className="mt-1 text-sm text-slate-300">
                {award.isFinal ? "Nilai akhir" : "Nilai sementara"}{" "}
                {award.score.toFixed(2)}
              </p>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

function MonthlyDetailModal({
  row,
  onClose,
}: {
  row: GaspolRankingRow | null;
  onClose: () => void;
}) {
  if (!row) {
    return null;
  }

  const statusMeta = getRankingStatusMeta(row);
  const path =
    row.path === "NOT_FAILED" || row.cohort === "NOT_FAILED"
      ? "NOT_FAILED"
      : "MAIN";
  const contributions = row.scores.contributions ?? [
    {
      key: "MONTHLY",
      label: "Nilai Bulanan",
      score: row.scores.scoreMonthly,
      weightedScore: row.scores.scoreMonthly * gaspolMonthlyTotalWeight,
      weight: 0.2,
      status: "FINAL",
    },
    {
      key: "FIELD_STAGE_1",
      label: "Lapangan Tahap 1",
      score: row.scores.scoreFieldStage1 ?? row.scores.scoreField,
      weightedScore:
        (row.scores.scoreFieldStage1 ?? row.scores.scoreField) * 0.4,
      weight: 0.4,
      status: row.scores.scoreField > 0 ? "FINAL" : "NOT_ASSESSED",
    },
    ...(path === "NOT_FAILED"
      ? [
          {
            key: "NOT_FAILED_NOMINATION",
            label: "Nominasi Tidak Tergagal",
            score: row.scores.scoreNomination,
            weightedScore: (row.scores.scoreNomination ?? 0) * 0.3,
            weight: 0.3,
            status:
              (row.scores.scoreNomination ?? 0) > 0 ? "FINAL" : "NOT_ASSESSED",
          },
          {
            key: "BOTTOM5_APPEAL",
            label: "Banding Kandidat Zona Tergagal",
            score: row.scores.scoreAppeal,
            weightedScore: (row.scores.scoreAppeal ?? 0) * 0.1,
            weight: 0.1,
            status:
              (row.scores.scoreAppeal ?? 0) > 0 ? "FINAL" : "NOT_ASSESSED",
          },
        ]
      : [
          {
            key: "FIELD_STAGE_2",
            label: "Lapangan Tahap 2",
            score: row.scores.scoreFieldStage2,
            weightedScore: (row.scores.scoreFieldStage2 ?? 0) * 0.2,
            weight: 0.2,
            status:
              (row.scores.scoreFieldStage2 ?? 0) > 0 ? "FINAL" : "NOT_ASSESSED",
          },
          {
            key: "PRESENTATION",
            label: "Presentasi",
            score: row.scores.scorePresentation,
            weightedScore: row.scores.scorePresentation * 0.2,
            weight: 0.2,
            status: row.scores.scorePresentation > 0 ? "FINAL" : "NOT_ASSESSED",
          },
        ]),
  ];
  const monthlyContribution = contributions.find(
    (item) => item.key === "MONTHLY"
  )!;
  const field1Contribution = contributions.find(
    (item) => item.key === "FIELD_STAGE_1"
  )!;
  const field2Contribution = contributions.find(
    (item) => item.key === "FIELD_STAGE_2"
  );
  const presentationContribution = contributions.find(
    (item) => item.key === "PRESENTATION"
  );
  const nominationContribution = contributions.find(
    (item) => item.key === "NOT_FAILED_NOMINATION"
  );
  const appealContribution = contributions.find(
    (item) => item.key === "BOTTOM5_APPEAL"
  );
  const currentMaxScore =
    row.scores.currentMaxScore ?? row.scores.maximumScore ?? 20;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 px-4 py-5">
      <button
        type="button"
        aria-label="Tutup rincian nilai"
        className="absolute inset-0"
        onClick={onClose}
      />

      <section
        role="dialog"
        aria-modal="true"
        className="relative flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-slate-200 px-5 py-4">
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
              Rincian Nilai
            </p>
            <h2 className="truncate text-lg font-semibold text-slate-950">
              Rincian Nilai - {row.schoolName}
            </h2>
            <p className="mt-1 text-sm text-slate-500">NPSN {row.npsn}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
          >
            Tutup
          </button>
        </div>

        <div className="flex-1 space-y-5 overflow-y-auto p-5">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {row.monthlyComponents.map((component) => (
              <div
                key={component.month}
                className="rounded-2xl border border-slate-200 bg-slate-50 p-4"
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h3 className="text-base font-semibold text-slate-950">
                      {component.label}
                    </h3>
                    <p className="text-sm text-slate-500">
                      Bobot {(component.weight * 100).toFixed(0)}%
                    </p>
                  </div>
                  <StatusBadge
                    variant={getMonthlyComponentVariant(component.status)}
                  >
                    {component.statusLabel}
                  </StatusBadge>
                </div>

                <dl className="mt-4 grid gap-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                      Bobot
                    </dt>
                    <dd className="mt-1 text-sm font-semibold text-slate-900">
                      {(component.weight * 100).toFixed(0)}%
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                      Nilai
                    </dt>
                    <dd className="mt-1 text-sm font-semibold text-slate-900">
                      {component.score.toFixed(2)}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                      Kontribusi
                    </dt>
                    <dd className="mt-1 text-sm font-semibold text-slate-900">
                      {component.effectiveScore.toFixed(2)}
                    </dd>
                  </div>
                </dl>

                <p className="mt-4 text-sm text-slate-600">
                  Status:{" "}
                  <span className="font-medium text-slate-900">
                    {component.statusLabel}
                  </span>
                </p>
                <p className="mt-1 text-sm text-slate-500">
                  {component.statusDetail || "Belum ada keterangan tambahan."}
                </p>
              </div>
            ))}
          </div>

          <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-sm font-semibold text-slate-950">
                Rekapitulasi Nilai Total Sekolah
              </p>
              <StatusBadge variant={statusMeta.variant}>
                {statusMeta.label}
              </StatusBadge>
            </div>
            <div className="mt-4 grid gap-4 lg:grid-cols-2">
              <ScoreContributionCard contribution={monthlyContribution} />
              {path === "NOT_FAILED" ? (
                <>
                  <ScoreContributionCard contribution={field1Contribution} />
                  {nominationContribution ? (
                    <ScoreContributionCard
                      contribution={nominationContribution}
                    />
                  ) : null}
                  {appealContribution ? (
                    <ScoreContributionCard contribution={appealContribution} />
                  ) : null}
                </>
              ) : (
                <>
                  <div className="rounded-2xl border border-slate-200 bg-white p-4 lg:col-span-2">
                    <h4 className="font-semibold text-slate-950">Lapangan</h4>
                    <div className="mt-3 grid gap-3 md:grid-cols-2">
                      <ScoreContributionCard
                        contribution={field1Contribution}
                      />
                      {field2Contribution ? (
                        <ScoreContributionCard
                          contribution={field2Contribution}
                        />
                      ) : null}
                    </div>
                  </div>
                  {presentationContribution ? (
                    <ScoreContributionCard
                      contribution={presentationContribution}
                    />
                  ) : null}
                </>
              )}
            </div>
            <div className="mt-4 rounded-2xl bg-slate-950 px-5 py-4 text-white">
              <p className="text-sm text-slate-300">
                {currentMaxScore >= 100
                  ? "Nilai Total Akhir"
                  : "Nilai Total Sementara"}
              </p>
              <p className="mt-1 text-2xl font-semibold">
                {row.scores.scoreTotal.toFixed(2)} / {currentMaxScore}
              </p>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

function CompactRankingTable({ rows }: { rows: GaspolRankingRow[] }) {
  const [selectedRow, setSelectedRow] = useState<GaspolRankingRow | null>(null);
  const tableProps = getGaspolScrollableTableProps(rows.length);

  return (
    <>
      <Table
        className="min-w-[1120px]"
        stickyHeader={tableProps.stickyHeader}
        wrapperClassName={tableProps.wrapperClassName}
      >
        <TableHeader>
          <TableRow>
            <TableHead className="w-20">Peringkat</TableHead>
            <TableHead className="min-w-[220px]">Sekolah</TableHead>
            <TableHead className="w-32">NPSN</TableHead>
            <TableHead className="w-28 text-right">Bulanan</TableHead>
            <TableHead className="w-28 text-right">Lapangan T1</TableHead>
            <TableHead className="w-28 text-right">Lapangan T2</TableHead>
            <TableHead className="w-32 text-right">Presentasi</TableHead>
            <TableHead className="w-28 text-right">Total</TableHead>
            <TableHead className="w-36 text-center">Status</TableHead>
            <TableHead className="w-44 text-center">Aksi</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => {
            const statusMeta = getRankingStatusMeta(row);
            return (
              <TableRow
                key={`${row.schoolId}-${row.rank}`}
                className={getPublicRankingTone(row.rank, row.visualTone)}
              >
                <TableCell className="font-semibold text-slate-950">
                  {row.rank > 0 ? row.rank : "Finalis"}
                </TableCell>
                <TableCell>
                  <div className="min-w-0">
                    <p className="font-semibold text-slate-950">
                      {row.schoolName}
                    </p>
                  </div>
                </TableCell>
                <TableCell className="font-mono text-xs sm:text-sm">
                  {row.npsn}
                </TableCell>
                <TableCell className="text-right font-medium text-slate-900 tabular-nums">
                  {row.scores.scoreMonthly.toFixed(2)}
                </TableCell>
                <TableCell className="text-right font-medium text-slate-900 tabular-nums">
                  {contributionDisplay(contributionByKey(row, "FIELD_STAGE_1"))}
                </TableCell>
                <TableCell className="text-right font-medium text-slate-900 tabular-nums">
                  {row.path === "NOT_FAILED" || row.cohort === "NOT_FAILED"
                    ? "Tidak mengikuti"
                    : contributionDisplay(
                        contributionByKey(row, "FIELD_STAGE_2")
                      )}
                </TableCell>
                <TableCell className="text-right font-medium text-slate-900 tabular-nums">
                  {row.path === "NOT_FAILED" || row.cohort === "NOT_FAILED"
                    ? "Tidak mengikuti"
                    : contributionDisplay(
                        contributionByKey(row, "PRESENTATION")
                      )}
                </TableCell>
                <TableCell className="text-right font-semibold text-slate-950 tabular-nums">
                  {row.scores.scoreTotal.toFixed(2)} /{" "}
                  {row.scores.maximumScore ?? 100}
                </TableCell>
                <TableCell className="text-center">
                  <StatusBadge
                    variant={
                      row.visualTone === "RED"
                        ? "danger"
                        : row.visualTone === "POSITIVE"
                          ? "success"
                          : statusMeta.variant
                    }
                  >
                    {row.badge || statusMeta.label}
                  </StatusBadge>
                </TableCell>
                <TableCell className="text-center">
                  <button
                    type="button"
                    className={cn(
                      buttonClass,
                      "border border-sky-200 bg-sky-50 text-sky-700 hover:bg-sky-100"
                    )}
                    onClick={() => setSelectedRow(row)}
                  >
                    Rincian Nilai
                  </button>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>

      <MonthlyDetailModal
        row={selectedRow}
        onClose={() => setSelectedRow(null)}
      />
    </>
  );
}

const getAssessmentScore = (item: GaspolAssessment) => {
  if (item.stage === "MONTHLY") {
    return item.finalMonthlyScore ?? item.scoreMonthly;
  }
  if (item.stage === "FIELD") {
    return item.scoreField;
  }
  return item.scorePresentation;
};

const formatScore = (value: number | null | undefined) =>
  typeof value === "number" ? value.toFixed(2) : "-";

const getValidationDecisionLabel = (value: string | undefined) => {
  switch ((value ?? "").trim().toUpperCase()) {
    case "AGREE":
      return "Setuju";
    case "OBJECT":
      return "Keberatan";
    default:
      return "-";
  }
};

const getPublicRankingTone = (rank: number, visualTone?: string) => {
  if (visualTone === "RED") return "bg-red-100";
  if (visualTone === "PINK") return "bg-pink-100";
  if (visualTone === "POSITIVE") return "bg-emerald-100";
  if (rank <= 5) {
    return "bg-emerald-50";
  }
  if (rank <= 10) {
    return "bg-sky-50";
  }
  return "";
};

const getDisabledReasonLabel = (reason: string | undefined) => {
  switch ((reason ?? "").trim()) {
    case "SCHOOL_NOT_PARTICIPANT":
      return "Sekolah ini belum ditetapkan sebagai peserta GASPOL pada tahun berjalan.";
    case "READ_ONLY":
      return "Akun ini hanya memiliki akses lihat pada laporan GASPOL.";
    default:
      return "";
  }
};

async function readApi<T>(path: string, init?: RequestInit) {
  const response = await apiFetch(path, init);
  let payload: ApiEnvelope<T> | T | null = null;
  try {
    payload = (await response.json()) as ApiEnvelope<T> | T;
  } catch {
    payload = null;
  }
  const data =
    payload && typeof payload === "object" && "data" in payload
      ? ((payload as ApiEnvelope<T>).data ?? null)
      : (payload as T | null);
  return { response, data };
}

function MetricCard({
  title,
  value,
  icon: Icon,
  tone = "emerald",
}: {
  title: string;
  value: string | number;
  icon: React.ComponentType<{ className?: string }>;
  tone?: "emerald" | "sky" | "slate";
}) {
  return (
    <Card
      className={cn(
        "border-0 text-white shadow-lg",
        tone === "emerald" &&
          "bg-gradient-to-br from-emerald-600 to-emerald-400",
        tone === "sky" && "bg-gradient-to-br from-sky-600 to-cyan-400",
        tone === "slate" && "bg-gradient-to-br from-slate-800 to-slate-600"
      )}
    >
      <CardContent className="flex items-start justify-between gap-4 p-5">
        <div>
          <p className="text-xs font-semibold tracking-[0.18em] text-white/75 uppercase">
            {title}
          </p>
          <p className="mt-3 text-3xl font-semibold">{value}</p>
        </div>
        <div className="rounded-2xl bg-white/15 p-3">
          <Icon className="h-5 w-5" />
        </div>
      </CardContent>
    </Card>
  );
}

function RankingTable({
  rows,
  title,
  description,
  summary,
}: {
  rows: GaspolRankingRow[];
  title: string;
  description: string;
  summary?: GaspolRankingStatusSummary | null;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <RankingStatusNotice summary={summary} />
        {rows.length === 0 ? (
          <EmptyState
            title="Belum ada ranking"
            description="Ranking live akan muncul meskipun nilai masih 0 sementara."
            icon={<Trophy className="h-5 w-5" />}
          />
        ) : (
          <CompactRankingTable rows={rows} />
        )}
      </CardContent>
    </Card>
  );
}

export function GaspolDashboard() {
  const { user, status } = useAuth();
  const router = useRouter();
  const [view, setView] = useState<GaspolView>("PUBLIC");
  const [activeAwardInfo, setActiveAwardInfo] = useState<AwardInfoKey | null>(
    null
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [isPending, startTransition] = useTransition();

  const [publicDashboard, setPublicDashboard] =
    useState<GaspolPublicDashboard | null>(null);
  const [schoolDashboard, setSchoolDashboard] =
    useState<GaspolSchoolDashboardData | null>(null);
  const [supervisorDashboard, setSupervisorDashboard] =
    useState<GaspolSupervisorDashboardData | null>(null);
  const [validatorDashboard, setValidatorDashboard] =
    useState<GaspolValidatorDashboardData | null>(null);
  const [juryDashboard, setJuryDashboard] =
    useState<GaspolJuryDashboardData | null>(null);
  const [adminDashboard, setAdminDashboard] =
    useState<GaspolAdminDashboardData | null>(null);
  const [stageOnePreview, setStageOnePreview] =
    useState<GaspolStageOnePreview | null>(null);
  const [showTopTenConfirmation, setShowTopTenConfirmation] = useState(false);
  const [masterReturnTarget, setMasterReturnTarget] = useState<{
    id: number;
    schoolName: string;
    assessorName: string;
  } | null>(null);
  const [masterReturnNotes, setMasterReturnNotes] = useState("");
  const [monitoringSchools, setMonitoringSchools] = useState<
    GaspolMonitoringSchoolRow[]
  >([]);
  const [monitoringSupervisors, setMonitoringSupervisors] = useState<
    GaspolMonitoringSupervisorRow[]
  >([]);
  const [resetResult, setResetResult] = useState<ResetPasswordResult | null>(
    null
  );

  const [schoolForm, setSchoolForm] = useState({
    year: currentYear,
    month: currentMonthInRange(4, 11, 4),
    reportTitle: "",
    reportBody: "",
    reportLink: "",
  });
  const [supervisorTab, setSupervisorTab] = useState<
    "MISSING" | "PENDING" | "DONE"
  >("PENDING");
  const [supervisorFilterMonth, setSupervisorFilterMonth] = useState(
    currentMonthInRange(4, 9, 4)
  );
  const [koorStageFilter, setKoorStageFilter] =
    useState<ValidationStage>("MONTHLY");
  const [cabdinStageFilter, setCabdinStageFilter] =
    useState<ValidationStage>("MONTHLY");
  const [validatorTab, setValidatorTab] = useState<
    "BLOCKED" | "PENDING" | "DONE"
  >("BLOCKED");
  const [validatorFilterMonth, setValidatorFilterMonth] = useState(
    currentMonthInRange(4, 9, 9)
  );
  const [activeSupervisorSchoolId, setActiveSupervisorSchoolId] = useState<
    number | null
  >(null);
  const [supervisorConfirmation, setSupervisorConfirmation] =
    useState<GaspolMonthlyWorkflowRow | null>(null);
  const [supervisorScores, setSupervisorScores] = useState<
    Record<number, { score: string; notes: string }>
  >({});
  const [validatorDrafts, setValidatorDrafts] = useState<
    Record<
      number,
      { decision: "AGREE" | "OBJECT"; score: string; notes: string }
    >
  >({});
  const [selectedValidatorAssessmentIds, setSelectedValidatorAssessmentIds] =
    useState<Set<number>>(() => new Set());
  const validatorSelectAllRef = useRef<HTMLInputElement | null>(null);
  const [juryStage] = useState<"FIELD" | "PRESENTATION">(
    currentMonthInRange(11, 11, 10) === 11 ? "PRESENTATION" : "FIELD"
  );
  const [assessmentStage, setAssessmentStage] = useState("");
  const [taskSearch, setTaskSearch] = useState("");
  const [taskStatusFilter, setTaskStatusFilter] = useState("ALL");
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);
  const [monthlySearch, setMonthlySearch] = useState("");
  const [reviewSchoolSearch, setReviewSchoolSearch] = useState("");
  const [stageTaskDrafts, setStageTaskDrafts] = useState<
    Record<string, { score: string; notes: string }>
  >({});
  const [resetForm, setResetForm] = useState({ userId: "", newPassword: "" });
  const [cancelSubmitForm, setCancelSubmitForm] = useState({
    year: currentYear,
    month: currentMonthInRange(4, 11, 4),
    schoolId: "",
    reason: "",
  });

  const clearFlash = () => {
    setError("");
    setMessage("");
  };

  const lockedMonths = schoolDashboard?.lockedMonths ?? [];
  const scoredMonths = schoolDashboard?.scoredMonths ?? [];
  const selectedMonthLocked = lockedMonths.includes(schoolForm.month);
  const selectedSubmission =
    schoolDashboard?.history.find((item) => item.month === schoolForm.month) ??
    null;
  const selectedMonthStatus =
    schoolDashboard?.monthlyStatuses.find(
      (item) => item.month === schoolForm.month
    ) ?? null;
  const isSelectedSubmissionSubmitted =
    (selectedSubmission?.status ?? "").toUpperCase() === "SUBMITTED";
  const schoolMonthlyStatuses = schoolDashboard?.monthlyStatuses ?? [];
  const selectedSchoolStatus = getSchoolStatusLabel(
    schoolForm.month,
    selectedSubmission?.status,
    lockedMonths,
    scoredMonths
  );
  const schoolSubmissionErrors = getSubmissionValidationErrors(schoolForm);
  const canSchoolEdit =
    Boolean(schoolDashboard?.canEdit) &&
    !Boolean(schoolDashboard?.actionsDisabled) &&
    !selectedMonthLocked &&
    selectedSchoolStatus !== "Sudah Dinilai" &&
    !isSelectedSubmissionSubmitted;
  const canSchoolSend = canSchoolEdit && schoolSubmissionErrors.length === 0;
  const supervisorCompletedRows = supervisorDashboard?.completedReviews ?? [];
  const supervisorPendingRows = supervisorDashboard?.pendingReviews ?? [];
  const validatorPendingRows = validatorDashboard?.pendingRows ?? [];
  const selectableValidatorAssessmentIds = validatorPendingRows
    .map((item) => item.assessmentId)
    .filter((id): id is number => typeof id === "number" && id > 0);
  const selectableValidatorAssessmentIdKey =
    selectableValidatorAssessmentIds.join(",");
  const selectedValidatorCount = selectedValidatorAssessmentIds.size;
  const allValidatorPendingSelected =
    selectableValidatorAssessmentIds.length > 0 &&
    selectableValidatorAssessmentIds.every((id) =>
      selectedValidatorAssessmentIds.has(id)
    );
  const someValidatorPendingSelected =
    selectedValidatorCount > 0 && !allValidatorPendingSelected;
  const activeSupervisorAssignment =
    supervisorPendingRows.find(
      (assignment) => assignment.schoolId === activeSupervisorSchoolId
    ) ??
    supervisorPendingRows[0] ??
    null;

  useEffect(() => {
    const pendingAssignments = supervisorDashboard?.pendingReviews ?? [];
    if (pendingAssignments.length === 0) {
      if (activeSupervisorSchoolId !== null) {
        setActiveSupervisorSchoolId(null);
      }
      return;
    }
    if (
      activeSupervisorSchoolId == null ||
      !pendingAssignments.some(
        (assignment) => assignment.schoolId === activeSupervisorSchoolId
      )
    ) {
      setActiveSupervisorSchoolId(pendingAssignments[0].schoolId);
    }
  }, [activeSupervisorSchoolId, supervisorDashboard]);

  useEffect(() => {
    const allowed = new Set(
      selectableValidatorAssessmentIdKey.split(",").filter(Boolean).map(Number)
    );
    setSelectedValidatorAssessmentIds((current) => {
      let changed = false;
      const next = new Set<number>();
      current.forEach((id) => {
        if (allowed.has(id)) {
          next.add(id);
        } else {
          changed = true;
        }
      });
      return changed ? next : current;
    });
  }, [selectableValidatorAssessmentIdKey]);

  useEffect(() => {
    if (validatorSelectAllRef.current) {
      validatorSelectAllRef.current.indeterminate =
        someValidatorPendingSelected;
    }
  }, [someValidatorPendingSelected]);

  useEffect(() => {
    if (
      cancelSubmitForm.schoolId ||
      view !== "ADMIN_MASTER" ||
      monitoringSchools.length === 0
    ) {
      return;
    }
    setCancelSubmitForm((current) => ({
      ...current,
      schoolId: String(monitoringSchools[0].schoolId),
    }));
  }, [cancelSubmitForm.schoolId, monitoringSchools, view]);

  useEffect(() => {
    const existingSubmission =
      schoolDashboard?.history.find(
        (item) => item.month === schoolForm.month
      ) ?? null;
    setSchoolForm((current) => {
      const next = {
        ...current,
        year: schoolDashboard?.year ?? current.year,
        reportTitle: existingSubmission?.reportTitle ?? "",
        reportBody: existingSubmission?.reportBody ?? "",
        reportLink: existingSubmission?.reportLink ?? "",
      };
      if (
        next.year === current.year &&
        next.reportTitle === current.reportTitle &&
        next.reportBody === current.reportBody &&
        next.reportLink === current.reportLink
      ) {
        return current;
      }
      return next;
    });
  }, [schoolDashboard, schoolForm.month]);

  const loadAdminData = async () => {
    const adminResponse = await readApi<GaspolAdminDashboardData>(
      "/gaspol/admin/dashboard?year=" + currentYear
    );
    if (!adminResponse.response.ok || !adminResponse.data) {
      throw new Error("Gagal memuat dashboard admin GASPOL.");
    }
    const schoolResponse = await readApi<GaspolMonitoringSchoolRow[]>(
      "/gaspol/admin/monitoring/schools?year=" + currentYear
    );
    const supervisorResponse = await readApi<GaspolMonitoringSupervisorRow[]>(
      "/gaspol/admin/monitoring/supervisors?year=" +
        currentYear +
        "&month=" +
        currentMonthInRange(4, 9, 4)
    );
    const previewResponse = await readApi<GaspolStageOnePreview>(
      "/gaspol/internal/ranking-preview?year=" +
        currentYear +
        "&stage=FIELD_STAGE_1"
    );
    if (!previewResponse.response.ok || !previewResponse.data) {
      throw new Error("Gagal memuat preview ranking Tahap 1.");
    }
    setAdminDashboard(adminResponse.data);
    setStageOnePreview(previewResponse.data);
    setMonitoringSchools(schoolResponse.data ?? []);
    setMonitoringSupervisors(supervisorResponse.data ?? []);
    setView("ADMIN_MASTER");
  };

  const loadTargetedDashboard = async <T,>(
    nextView: GaspolView,
    path: string,
    setter: (payload: T) => void,
    errorMessage: string
  ) => {
    const result = await readApi<T>(path);
    if (result.response.status === 403) {
      setView("DISABLED");
      return;
    }
    if (!result.response.ok || !result.data) {
      throw new Error(errorMessage);
    }
    setter(result.data);
    setView(nextView);
  };

  const loadLiveData = async () => {
    if (user?.role === "GASPOL_VOTER") {
      setLoading(false);
      return;
    }
    clearFlash();
    setLoading(true);
    try {
      const publicResponse = await readApi<GaspolPublicDashboard>(
        "/public/gaspol/dashboard?year=" + currentYear
      );
      setPublicDashboard(publicResponse.data);

      if (status !== "authenticated" || !user) {
        setView("PUBLIC");
        return;
      }

      const deterministicView = resolveDeterministicGaspolView(user.role);
      let targetView = deterministicView;
      if (!targetView) {
        const accessContextResponse = await readApi<GaspolAccessContext>(
          "/gaspol/access-context?year=" + currentYear
        );
        if (
          accessContextResponse.response.status === 403 ||
          !accessContextResponse.response.ok ||
          !accessContextResponse.data
        ) {
          setView("DISABLED");
          return;
        }
        targetView = normalizeGaspolView(accessContextResponse.data.view);
      }

      if (targetView === "ADMIN_MASTER") {
        await loadAdminData();
        return;
      }
      if (targetView === "VALIDATOR") {
        await loadTargetedDashboard(
          "VALIDATOR",
          "/gaspol/validator/dashboard?year=" +
            currentYear +
            "&month=" +
            validatorFilterMonth,
          setValidatorDashboard,
          "Gagal memuat dashboard validator GASPOL."
        );
        return;
      }
      if (targetView === "SUPERVISOR") {
        await loadTargetedDashboard(
          "SUPERVISOR",
          "/gaspol/supervisor/dashboard?year=" +
            currentYear +
            "&month=" +
            supervisorFilterMonth,
          setSupervisorDashboard,
          "Gagal memuat dashboard pengawas GASPOL."
        );
        return;
      }
      if (targetView === "JURY") {
        await loadTargetedDashboard(
          "JURY",
          "/gaspol/jury/dashboard?year=" + currentYear + "&stage=" + juryStage,
          setJuryDashboard,
          "Gagal memuat dashboard juri GASPOL."
        );
        return;
      }
      if (targetView === "SCHOOL") {
        await loadTargetedDashboard(
          "SCHOOL",
          "/gaspol/school/dashboard?year=" + currentYear,
          setSchoolDashboard,
          "Gagal memuat dashboard sekolah GASPOL."
        );
        return;
      }
      setView("DISABLED");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal memuat modul GASPOL."
      );
    } finally {
      setLoading(false);
    }
  };

  const loadLiveDataEvent = useEffectEvent(() => {
    void loadLiveData();
  });

  useEffect(() => {
    loadLiveDataEvent();
  }, [juryStage, status, supervisorFilterMonth, user, validatorFilterMonth]);

  const refresh = () => {
    startTransition(() => {
      void loadLiveData();
    });
  };

  const submitSchool = async (nextStatus: "DRAFT" | "SUBMITTED") => {
    clearFlash();
    if (selectedMonthLocked) {
      setError(
        `Isian bulan ${monthName(schoolForm.month)} sudah terkunci oleh rule deadline bulanan GASPOL.`
      );
      return;
    }
    if (nextStatus === "SUBMITTED" && schoolSubmissionErrors.length > 0) {
      setError("Lengkapi seluruh field wajib sebelum mengirimkan ke penilai.");
      return;
    }
    const response = await apiFetch("/gaspol/school/submissions", {
      method: "POST",
      body: JSON.stringify({
        ...schoolForm,
        status: nextStatus,
      }),
    });
    const payload = (await response.json().catch(() => null)) as {
      message?: string;
    } | null;
    if (!response.ok) {
      setError(
        payload?.message === "MONTHLY_ENTRY_LOCKED"
          ? `Isian bulan ${monthName(
              schoolForm.month
            )} sudah terkunci karena melewati ${gaspolDeadlineNotice}.`
          : payload?.message === "SUBMISSION_LOCKED"
            ? "Laporan yang sudah dikirim ke penilai tidak dapat diubah kembali."
            : payload?.message === "INCOMPLETE_SUBMISSION"
              ? "Lengkapi judul laporan, ringkasan laporan, dan link laporan sebelum mengirimkan ke penilai."
              : payload?.message === "INVALID_REPORT_LINK"
                ? "Link laporan harus berupa URL lengkap yang valid."
                : nextStatus === "SUBMITTED"
                  ? "Pengiriman laporan ke penilai gagal."
                  : "Simpan laporan sekolah gagal."
      );
      return;
    }
    setMessage(
      nextStatus === "SUBMITTED"
        ? "Laporan berhasil dikirimkan ke penilai."
        : "Laporan berhasil disimpan."
    );
    await loadLiveData();
  };

  const cancelSubmitSchoolSubmission = async () => {
    clearFlash();
    const schoolId = Number(cancelSubmitForm.schoolId);
    if (!schoolId) {
      setError("Pilih sekolah terlebih dahulu.");
      return;
    }
    const confirmed = window.confirm(
      "Status laporan akan dikembalikan ke Draft agar sekolah dapat memperbaiki laporan selama belum melewati deadline. Laporan harus dikirim ulang sebelum deadline agar dapat dinilai. Lanjutkan?"
    );
    if (!confirmed) {
      return;
    }

    const response = await apiFetch("/gaspol/admin/submissions/cancel-submit", {
      method: "PATCH",
      body: JSON.stringify({
        year: cancelSubmitForm.year,
        month: cancelSubmitForm.month,
        schoolId,
        reason: cancelSubmitForm.reason,
      }),
    });
    const payload = (await response.json().catch(() => null)) as {
      message?: string;
    } | null;
    if (!response.ok) {
      setError(
        payload?.message?.trim() || "Gagal membatalkan status kirim laporan."
      );
      return;
    }
    setMessage(
      "Status kirim berhasil dibatalkan. Sekolah dapat memperbaiki laporan dan mengirim ulang sebelum deadline."
    );
    setCancelSubmitForm((current) => ({ ...current, reason: "" }));
    await loadLiveData();
  };

  const submitSupervisorAssessment = async (
    assignment: GaspolMonthlyWorkflowRow
  ) => {
    clearFlash();
    const draft = supervisorScores[assignment.schoolId];
    const parsedScore = Number(draft?.score ?? "");
    if (
      draft?.score?.trim() === "" ||
      Number.isNaN(parsedScore) ||
      parsedScore < 0 ||
      parsedScore > 100
    ) {
      setError("Nilai pengawas harus diisi dengan angka 0 sampai 100.");
      return;
    }
    const { response } = await readApi<GaspolAssessment>(
      "/gaspol/supervisor/assessments/monthly",
      {
        method: "POST",
        body: JSON.stringify({
          year: supervisorDashboard?.year ?? currentYear,
          month: assignment.month,
          schoolId: assignment.schoolId,
          score: parsedScore,
          notes: draft?.notes ?? "",
        }),
      }
    );
    if (!response.ok) {
      const payload = (await response.json().catch(() => null)) as {
        message?: string;
      } | null;
      setError(payload?.message || "Kirim penilaian pengawas gagal.");
      return;
    }
    setSupervisorConfirmation(null);
    setMessage(
      assignment.workflowStatus === "RETURNED_BY_KOOR" ||
        assignment.workflowStatus === "RETURNED_BY_CABDIN"
        ? "Perbaikan penilaian berhasil dikirim ulang ke Koor GASPOL."
        : "Penilaian bulanan berhasil dikirim."
    );
    await loadLiveData();
  };

  const openSupervisorAssessmentEditor = (
    assignment: GaspolMonthlyWorkflowRow
  ) => {
    const existingAssessment =
      supervisorDashboard?.latestAssessments.find(
        (item) => item.schoolId === assignment.schoolId
      ) ?? null;
    setSupervisorScores((current) => ({
      ...current,
      [assignment.schoolId]: current[assignment.schoolId] ?? {
        score:
          typeof assignment.supervisorScore === "number"
            ? String(assignment.supervisorScore)
            : existingAssessment != null
              ? String(getAssessmentScore(existingAssessment))
              : "",
        notes: assignment.supervisorNotes ?? existingAssessment?.notes ?? "",
      },
    }));
    setActiveSupervisorSchoolId(assignment.schoolId);
  };

  const reviewStageAssessment = async (
    assessmentId: number,
    level: "KOOR" | "CABDIN",
    decision: "APPROVE" | "RETURN",
    notes = ""
  ) => {
    clearFlash();
    const { response } = await readApi<{ reviewed: boolean }>(
      `/gaspol/assessments/${assessmentId}/review/${level}`,
      {
        method: "POST",
        body: JSON.stringify({ decision, notes }),
      }
    );
    if (!response.ok) {
      setError(
        level === "KOOR"
          ? "Review Koor GASPOL gagal. Pastikan status nilai masih menunggu Koor."
          : "Review Validator Cabdin gagal. Nilai harus sudah disetujui Koor GASPOL."
      );
      return;
    }
    setMessage(
      `${level === "KOOR" ? "Koor GASPOL" : "Validator Cabdin"} berhasil ${
        decision === "APPROVE" ? "menyetujui" : "mengembalikan"
      } nilai.`
    );
    await loadLiveData();
  };

  const validateAssessment = async (row: GaspolMonthlyWorkflowRow) => {
    clearFlash();
    if (!row.assessmentId) {
      setError("Data penilaian pengawas tidak ditemukan.");
      return;
    }
    const draft = validatorDrafts[row.assessmentId] ?? {
      decision: "AGREE" as const,
      score: "",
      notes: "",
    };
    const { response } = await readApi<{ reviewed: boolean }>(
      `/gaspol/assessments/${row.assessmentId}/review/CABDIN`,
      {
        method: "POST",
        body: JSON.stringify({
          decision: draft.decision === "AGREE" ? "APPROVE" : "RETURN",
          notes: draft.notes,
        }),
      }
    );
    if (!response.ok) {
      setError(
        "Validasi nilai gagal. Nilai harus lebih dulu disetujui Koor GASPOL."
      );
      return;
    }
    setMessage(
      draft.decision === "OBJECT"
        ? "Nilai dikembalikan oleh Validator Cabdin."
        : "Validator Cabdin menyetujui nilai sebagai final."
    );
    await loadLiveData();
  };

  const toggleValidatorAssessmentSelection = (
    assessmentId: number,
    checked: boolean
  ) => {
    setSelectedValidatorAssessmentIds((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(assessmentId);
      } else {
        next.delete(assessmentId);
      }
      return next;
    });
  };

  const toggleAllValidatorPendingSelection = (checked: boolean) => {
    setSelectedValidatorAssessmentIds((current) => {
      const next = new Set(current);
      selectableValidatorAssessmentIds.forEach((id) => {
        if (checked) {
          next.add(id);
        } else {
          next.delete(id);
        }
      });
      return next;
    });
  };

  const approveSelectedValidatorAssessments = async () => {
    clearFlash();
    const selectedIds = Array.from(selectedValidatorAssessmentIds).filter(
      (id) => selectableValidatorAssessmentIds.includes(id)
    );
    if (selectedIds.length === 0) {
      return;
    }
    const confirmed = window.confirm(
      `Anda akan menyetujui nilai pengawas untuk ${selectedIds.length} sekolah terpilih. Nilai final bulanan akan mengikuti nilai dari pengawas. Lanjutkan?`
    );
    if (!confirmed) {
      return;
    }

    const results = await Promise.all(
      selectedIds.map((assessmentId) =>
        readApi<{ reviewed: boolean }>(
          `/gaspol/assessments/${assessmentId}/review/CABDIN`,
          {
            method: "POST",
            body: JSON.stringify({ decision: "APPROVE", notes: "" }),
          }
        )
      )
    );
    if (results.some((result) => !result.response.ok)) {
      setError("Validasi nilai terpilih gagal.");
      return;
    }
    setMessage(
      `Nilai pengawas untuk ${selectedIds.length} sekolah terpilih berhasil disetujui.`
    );
    setSelectedValidatorAssessmentIds(new Set());
    await loadLiveData();
  };

  const submitStageTask = async (task: GaspolStageTask) => {
    clearFlash();
    const key = `${task.stage}-${task.schoolId}`;
    const draft = stageTaskDrafts[key];
    const rawScore = draft?.score ?? task.rawScore;
    if (rawScore == null || String(rawScore).trim() === "") {
      setError("Isi nilai sebelum mengirim penilaian.");
      return;
    }
    const score = Number(rawScore);
    if (!Number.isFinite(score) || score < 0 || score > 100) {
      setError("Nilai harus berada pada rentang 0–100.");
      return;
    }
    const { response } = await readApi<GaspolStageAssessment>(
      "/gaspol/assessments",
      {
        method: "POST",
        body: JSON.stringify({
          year: currentYear,
          stage: task.stage,
          month: 0,
          schoolId: task.schoolId,
          score,
          rubric: {},
          notes: draft?.notes ?? task.notes ?? "",
        }),
      }
    );
    if (!response.ok) {
      setError("Simpan penilaian assignment GASPOL gagal.");
      return;
    }
    setMessage(`Nilai ${stageTaskLabel(task.stage)} berhasil dikirim.`);
    await loadLiveData();
  };

  const lockTopTen = async () => {
    clearFlash();
    const { response, data } = await readApi<{ id: number }>(
      "/gaspol/admin/result-locks",
      {
        method: "POST",
        body: JSON.stringify({ year: currentYear, lockType: "TOP_10" }),
      }
    );
    if (!response.ok || !data) {
      setError(
        response.status === 400
          ? "Top 10 belum dapat ditetapkan. Periksa kelengkapan atau tie pada batas peringkat 10/11."
          : "Penetapan Top 10 gagal."
      );
      return;
    }
    setShowTopTenConfirmation(false);
    setMessage("Top 10 telah ditetapkan sebagai snapshot resmi.");
    await loadLiveData();
  };

  const returnStageOneAssessment = async () => {
    if (!masterReturnTarget || !masterReturnNotes.trim()) return;
    clearFlash();
    const response = await apiFetch(
      `/gaspol/stage-assessments/${masterReturnTarget.id}/master-return`,
      {
        method: "POST",
        body: JSON.stringify({ notes: masterReturnNotes.trim() }),
      }
    );
    const payload = (await response.json().catch(() => null)) as {
      message?: string;
      error?: string;
    } | null;
    if (!response.ok) {
      setError(
        payload?.message === "TOP_10_ALREADY_LOCKED" ||
          payload?.error === "TOP_10_ALREADY_LOCKED"
          ? "Top 10 sudah ditetapkan sehingga nilai tidak dapat dikembalikan."
          : "Nilai gagal dikembalikan. Pastikan statusnya masih Final dan Top 10 belum ditetapkan."
      );
      return;
    }
    setMasterReturnTarget(null);
    setMasterReturnNotes("");
    setMessage(
      "Nilai Lapangan Tahap 1 dikembalikan kepada pengawas untuk direview."
    );
    await loadLiveData();
  };

  const resetPassword = async () => {
    clearFlash();
    const { response, data } = await readApi<ResetPasswordResult>(
      "/gaspol/admin/gaspol-users/reset-password",
      {
        method: "POST",
        body: JSON.stringify({
          userId: Number(resetForm.userId),
          newPassword: resetForm.newPassword,
        }),
      }
    );
    if (!response.ok || !data) {
      setError("Reset password gagal.");
      return;
    }
    setResetResult(data);
    setMessage("Password berhasil direset.");
  };

  const downloadExport = async () => {
    clearFlash();
    const response = await apiFetch(
      "/gaspol/admin/export.xlsx?year=" +
        currentYear +
        "&month=" +
        currentMonthInRange(4, 9, 4)
    );
    if (!response.ok) {
      setError("Export Excel gagal.");
      return;
    }
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "gaspol-monitoring.xlsx";
    link.click();
    window.URL.revokeObjectURL(url);
  };

  const renderAlert = () => {
    if (!error && !message) {
      return null;
    }
    return (
      <div
        className={cn(
          "flex items-start gap-3 rounded-xl border px-4 py-3 text-sm",
          error
            ? "border-red-200 bg-red-50 text-red-800"
            : "border-emerald-200 bg-emerald-50 text-emerald-800"
        )}
      >
        {error ? (
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
        ) : (
          <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />
        )}
        <p>{error || message}</p>
      </div>
    );
  };

  const renderPublic = () => {
    const pendingAward = (
      awardType: GaspolPublicAward["awardType"]
    ): GaspolPublicAward => ({
      awardType,
      schoolName: "Belum ditetapkan",
      isFinal: false,
    });
    const awards = publicDashboard?.awards ?? {
      best: pendingAward("BEST"),
      innovative: pendingAward("INNOVATIVE"),
      favorite: pendingAward("FAVORITE"),
      failed: [
        pendingAward("FAILED"),
        pendingAward("FAILED"),
        pendingAward("FAILED"),
      ],
    };
    return (
      <div className="space-y-6">
        <div className="rounded-2xl border border-emerald-200 bg-emerald-50 px-5 py-4">
          <p className="text-xs font-semibold tracking-[0.18em] text-emerald-700 uppercase">
            Fase Kompetisi
          </p>
          <p className="mt-1 font-semibold text-emerald-950">
            {competitionStateLabel(publicDashboard?.competitionState)}
          </p>
          <p className="mt-1 text-sm text-emerald-800">
            Publik hanya melihat hasil resmi yang sudah final atau snapshot
            penetapan yang telah dikunci.
          </p>
        </div>
        <Card className="overflow-visible border-0 bg-gradient-to-br from-slate-950 via-slate-900 to-slate-800 text-white shadow-xl">
          <CardHeader>
            <CardTitle>Kategori Penghargaan</CardTitle>
            <CardDescription className="text-slate-300">
              Nilai yang masih menunggu Koor GASPOL atau Cabdin tidak
              ditampilkan.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="mx-auto max-w-5xl space-y-4">
              <div className="mx-auto max-w-xl">
                <AwardCategoryCard
                  featured
                  awardKey="BEST"
                  activeAwardInfo={activeAwardInfo}
                  setActiveAwardInfo={setActiveAwardInfo}
                  label={awards.best.isFinal ? "TERBAIK" : "TERBAIK SEMENTARA"}
                  info={
                    awards.best.isFinal
                      ? "Terbaik merupakan hasil akhir yang telah dikunci setelah seluruh penilaian Presentasi selesai dan tervalidasi."
                      : "Terbaik Sementara menunjukkan sekolah dengan nilai kumulatif tertinggi berdasarkan tahapan yang telah selesai dan tervalidasi. Hasil ini belum merupakan penetapan akhir dan masih dapat berubah hingga seluruh penilaian Presentasi selesai."
                  }
                  awards={[awards.best]}
                />
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <AwardCategoryCard
                  awardKey="INNOVATIVE"
                  activeAwardInfo={activeAwardInfo}
                  setActiveAwardInfo={setActiveAwardInfo}
                  label="TER-INOVATIF"
                  info="Dipilih dari 5 Besar GASPOL Bang berdasarkan penilaian pengawas yang ditunjuk menggunakan rubrik inovasi, kemudian ditetapkan setelah persetujuan Kasi SMK."
                  awards={[awards.innovative]}
                />
                <AwardCategoryCard
                  awardKey="FAVORITE"
                  activeAwardInfo={activeAwardInfo}
                  setActiveAwardInfo={setActiveAwardInfo}
                  label="TER-FAVORIT"
                  info="Dipilih dari 5 Besar berdasarkan 60% vote internal dan 40% performa like konten. Vote internal dihitung terhadap jumlah voter hadir, sedangkan skor like dihitung berdasarkan threshold yang ditetapkan panitia."
                  awards={[awards.favorite]}
                />
              </div>
              <AwardCategoryCard
                awardKey="FAILED"
                activeAwardInfo={activeAwardInfo}
                setActiveAwardInfo={setActiveAwardInfo}
                label="TERGAGAL"
                info="Tiga sekolah ditetapkan setelah proses Nominasi Tidak Tergagal dan Banding Kandidat Zona Tergagal. Nilai akhir menggunakan Nilai Bulanan 20%, Lapangan Tahap 1 40%, Nominasi Tidak Tergagal 30%, dan Banding 10%."
                awards={
                  awards.failed.length > 0
                    ? awards.failed
                    : [
                        pendingAward("FAILED"),
                        pendingAward("FAILED"),
                        pendingAward("FAILED"),
                      ]
                }
              />
            </div>
            <div className="grid gap-4 md:grid-cols-3">
              {(publicDashboard?.cards ?? []).map((card) => (
                <MetricCard
                  key={card.label}
                  title={card.label}
                  value={card.value}
                  icon={
                    card.label === "participants"
                      ? Users
                      : card.label === "pendingValidation"
                        ? ShieldCheck
                        : Trophy
                  }
                  tone={card.label === "pendingValidation" ? "sky" : "emerald"}
                />
              ))}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Klasemen Kumulatif Sementara</CardTitle>
            <CardDescription>
              Periode{" "}
              {formatPeriod(
                publicDashboard?.month ?? 0,
                publicDashboard?.year ?? currentYear
              )}
              . Peringkat 1-5 ditandai hijau muda dan peringkat 6-10 ditandai
              biru muda.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <RankingStatusNotice summary={publicDashboard?.rankingStatus} />
            <CompactRankingTable rows={publicDashboard?.leaderboard ?? []} />

            <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">
              <p className="font-semibold text-slate-900">
                Legenda warna dan tahapan
              </p>
              <div className="mt-3 flex flex-wrap gap-3">
                <span className="rounded-full bg-emerald-100 px-3 py-1 text-emerald-800">
                  Hijau muda = lolos tahap final bulan 11
                </span>
                <span className="rounded-full bg-sky-100 px-3 py-1 text-sky-800">
                  Biru muda = lolos tahap kunjungan bulan 10
                </span>
              </div>
              <p className="mt-4">
                Klasemen ini adalah klasemen kumulatif sementara. Nilai bulanan
                reguler dibobot 20%, Lapangan Tahap 1 dibobot 40%, Lapangan
                Tahap 2 dibobot 20%, dan presentasi final dibobot 20%.
              </p>
              <p className="mt-2">
                Tahapan seleksi berlangsung dari penilaian bulanan, lanjut ke
                Lapangan Tahap 1 untuk menetapkan Top 10, Lapangan Tahap 2 untuk
                menetapkan Top 5 melalui BA, lalu presentasi final.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  };

  const renderSchool = () => (
    <div className="space-y-6">
      {schoolDashboard?.isParticipant && (
        <GaspolSchoolPhase
          key={`${schoolDashboard.year}-${schoolDashboard.schoolId}`}
          year={schoolDashboard.year}
          schoolId={schoolDashboard.schoolId}
        />
      )}
      {schoolDashboard?.cohorts?.includes("TOP_5") && (
        <>
          <GaspolInnovationForm
            year={schoolDashboard.year}
            schoolId={schoolDashboard.schoolId}
            school
          />
          <GaspolVotingPanel year={schoolDashboard.year} school />
        </>
      )}
      <div className="grid gap-4 md:grid-cols-3">
        <MetricCard
          title="Sekolah"
          value={schoolDashboard?.schoolName ?? "-"}
          icon={Leaf}
        />
        <MetricCard
          title="Bulanan"
          value={schoolDashboard?.scores.scoreMonthly.toFixed(2) ?? "0.00"}
          icon={Trophy}
          tone="sky"
        />
        <MetricCard
          title="Total"
          value={
            schoolDashboard
              ? `${schoolDashboard.scores.scoreTotal.toFixed(2)} / ${schoolDashboard.scores.maximumScore ?? 20}`
              : "Belum dinilai"
          }
          icon={ShieldCheck}
          tone="slate"
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Status Laporan Sekolah</CardTitle>
          <CardDescription>
            Pengisian laporan bulanan dilakukan per periode April sampai
            November.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap gap-2">
            <StatusBadge
              variant={schoolDashboard?.isParticipant ? "success" : "neutral"}
            >
              {schoolDashboard?.isParticipant
                ? "Peserta aktif"
                : "Bukan peserta"}
            </StatusBadge>
            <StatusBadge variant={getSchoolStatusVariant(selectedSchoolStatus)}>
              {selectedSchoolStatus}
            </StatusBadge>
            <StatusBadge variant={canSchoolEdit ? "success" : "warning"}>
              {canSchoolEdit ? "Pengisian Aktif" : "Tidak Dapat Diubah"}
            </StatusBadge>
          </div>
          <p className="text-sm text-slate-600">
            Status Penilaian:{" "}
            <span className="font-medium text-slate-900">
              {selectedMonthStatus?.assessmentStatus ??
                "Belum dikirim ke penilai"}
            </span>
          </p>
          <p className="text-sm text-slate-600">
            Pengawas:{" "}
            <span className="font-medium text-slate-900">
              {schoolDashboard?.assignedSupervisor || "-"}
            </span>
          </p>
          <p className="text-sm text-slate-600">
            Validator Cabdin:{" "}
            <span className="font-medium text-slate-900">
              {schoolDashboard?.assignedValidator || "-"}
            </span>
          </p>
          {schoolDashboard?.disabledReason ? (
            <p className="text-sm text-amber-700">
              {getDisabledReasonLabel(schoolDashboard.disabledReason) ||
                schoolDashboard.disabledReason}
            </p>
          ) : null}
          {schoolDashboard?.effectiveTime ? (
            <p className="text-sm text-slate-600">
              Waktu efektif saat evaluasi lock:{" "}
              <span className="font-medium text-slate-900">
                {new Date(schoolDashboard.effectiveTime).toLocaleString(
                  "id-ID",
                  {
                    timeZone: "Asia/Jakarta",
                    day: "2-digit",
                    month: "short",
                    year: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                    second: "2-digit",
                    hour12: false,
                  }
                )}
              </span>
            </p>
          ) : null}
          {selectedMonthLocked ? (
            <p className="text-sm text-red-700">
              Periode {formatPeriod(schoolForm.month, schoolForm.year)} sudah
              terkunci karena melewati batas edit {gaspolDeadlineNotice}.
            </p>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Komponen Ranking April-September</CardTitle>
          <p className="text-sm font-semibold text-slate-700">
            {schoolDashboard?.monthlyFinal
              ? "Nilai Bulanan Final"
              : "Nilai Bulanan belum final"}
          </p>
          <CardDescription>
            Enam komponen ini membentuk `monthly_score` berbobot tetap. Komponen
            waiting belum final. April–Juni tanpa laporan setelah deadline
            bernilai 0 final; Juli–September menunggu finalisasi penilaian.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <MonthlyComponentsCell
            components={schoolDashboard?.monthlyComponents ?? []}
          />
          <p className="text-sm text-slate-600">
            Oktober dan November tidak masuk ke `monthly_score`; keduanya hanya
            dipakai untuk tahap lanjutan.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Laporan Bulanan Sekolah</CardTitle>
          <CardDescription>
            Gunakan tombol Simpan untuk menyimpan perubahan dan Kirimkan ke
            Penilai saat laporan sudah lengkap.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2">
            <select
              className={inputClass}
              value={schoolForm.month}
              disabled={!schoolDashboard?.canEdit}
              onChange={(event) =>
                setSchoolForm((current) => ({
                  ...current,
                  month: Number(event.target.value) || 4,
                }))
              }
            >
              {schoolMonths.map((month) => (
                <option key={month} value={month}>
                  {formatPeriod(month, schoolDashboard?.year ?? currentYear)}
                </option>
              ))}
            </select>
            <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
              <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                Status Periode
              </p>
              <div className="mt-2 flex items-center gap-2">
                <StatusBadge
                  variant={getSchoolStatusVariant(selectedSchoolStatus)}
                >
                  {selectedSchoolStatus}
                </StatusBadge>
                <span className="text-sm text-slate-600">
                  {formatPeriod(schoolForm.month, schoolForm.year)}
                </span>
              </div>
            </div>
            <input
              className={inputClass}
              value={schoolForm.reportTitle}
              disabled={!canSchoolEdit}
              onChange={(event) =>
                setSchoolForm((current) => ({
                  ...current,
                  reportTitle: event.target.value,
                }))
              }
              placeholder="Judul laporan"
            />
            <input
              className={inputClass}
              value={schoolForm.reportLink}
              disabled={!canSchoolEdit}
              onChange={(event) =>
                setSchoolForm((current) => ({
                  ...current,
                  reportLink: event.target.value,
                }))
              }
              placeholder="Link laporan"
            />
          </div>
          <textarea
            className={cn(inputClass, "min-h-28")}
            value={schoolForm.reportBody}
            disabled={!canSchoolEdit}
            onChange={(event) =>
              setSchoolForm((current) => ({
                ...current,
                reportBody: event.target.value,
              }))
            }
            placeholder="Ringkasan laporan"
          />
          {schoolSubmissionErrors.length > 0 ? (
            <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
              <p className="font-semibold">
                Lengkapi field wajib sebelum laporan dikirimkan ke penilai.
              </p>
              <ul className="mt-2 list-disc pl-5">
                {schoolSubmissionErrors.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            </div>
          ) : null}
          {isSelectedSubmissionSubmitted ? (
            <p className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
              {submittedLockedNotice}
            </p>
          ) : null}
          <div
            className="flex flex-wrap gap-3"
            title={
              isSelectedSubmissionSubmitted ? submittedLockedNotice : undefined
            }
          >
            <button
              type="button"
              className={cn(
                buttonClass,
                "border border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
              )}
              disabled={!canSchoolEdit || isPending}
              onClick={() => startTransition(() => void submitSchool("DRAFT"))}
            >
              Simpan
            </button>
            <button
              type="button"
              className={cn(
                buttonClass,
                "bg-emerald-600 text-white hover:bg-emerald-700"
              )}
              disabled={!canSchoolSend || isPending}
              onClick={() =>
                startTransition(() => void submitSchool("SUBMITTED"))
              }
            >
              Kirimkan ke Penilai
            </button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Riwayat Laporan Bulanan</CardTitle>
          <CardDescription>
            Tabel ini menampilkan status laporan dan status penilaian per
            periode.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {schoolMonthlyStatuses.length ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Periode</TableHead>
                  <TableHead>Judul</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Status Penilaian</TableHead>
                  <TableHead>Link</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {schoolMonthlyStatuses.map((item) => (
                  <TableRow key={item.month}>
                    <TableCell>
                      {formatPeriod(item.month, item.year ?? currentYear)}
                    </TableCell>
                    <TableCell>{item.reportTitle || "-"}</TableCell>
                    <TableCell>
                      <StatusBadge
                        variant={getSchoolStatusVariant(item.displayStatus)}
                      >
                        {item.displayStatus}
                      </StatusBadge>
                    </TableCell>
                    <TableCell>{item.assessmentStatus}</TableCell>
                    <TableCell>
                      {item.reportLink ? (
                        <a
                          className="text-emerald-700 underline"
                          href={item.reportLink}
                          target="_blank"
                          rel="noreferrer"
                        >
                          Buka
                        </a>
                      ) : (
                        "-"
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <EmptyState
              title="Belum ada laporan"
              description="Riwayat akan muncul setelah laporan pertama berhasil disimpan."
              icon={<FileSpreadsheet className="h-5 w-5" />}
            />
          )}
        </CardContent>
      </Card>
    </div>
  );

  const availableTasks =
    view === "SUPERVISOR"
      ? (supervisorDashboard?.stageTasks ?? [])
      : (juryDashboard?.stageTasks ?? []);
  const hasMonthlyTasks =
    view === "SUPERVISOR" && (supervisorDashboard?.assignmentCount ?? 0) > 0;
  const assessmentTabs = assignedStageTabs(availableTasks, hasMonthlyTasks);
  const activeAssessmentStage = assessmentTabs.includes(assessmentStage)
    ? assessmentStage
    : (assessmentTabs[0] ?? "");
  const renderStageTasks = (tasks: GaspolStageTask[]) => {
    const needsWork = (task: GaspolStageTask) =>
      !task.workflowStatus ||
      ["RETURNED_BY_KOOR", "RETURNED_BY_CABDIN", "RETURNED_BY_MASTER"].includes(
        task.workflowStatus
      );
    const filteredTasks = filterStageTasks(
      tasks,
      activeAssessmentStage,
      taskSearch,
      taskStatusFilter
    );
    const selectedTask =
      filteredTasks.find((task) => task.assignmentId === selectedTaskId) ??
      filteredTasks[0];
    const orderedTasks = selectedTask ? [selectedTask] : [];
    return (
      <Card>
        <CardHeader>
          <CardTitle>Penugasan Tahap Kompetisi</CardTitle>
          <CardDescription>
            Hanya sekolah yang ditugaskan aktif kepada akun ini yang
            ditampilkan.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div aria-label="Tahap penilaian" className="flex flex-wrap gap-2">
            {assessmentTabs.map((stage) => (
              <button
                key={stage}
                type="button"
                aria-pressed={activeAssessmentStage === stage}
                onClick={() => {
                  setAssessmentStage(stage);
                  setSelectedTaskId(null);
                  setTaskSearch("");
                  setTaskStatusFilter("ALL");
                }}
                className={cn(
                  buttonClass,
                  activeAssessmentStage === stage
                    ? "bg-emerald-700 text-white"
                    : "border border-slate-200"
                )}
              >
                {stageTaskLabel(stage)}
              </button>
            ))}
          </div>
          {activeAssessmentStage !== "MONTHLY" && assessmentTabs.length > 0 && (
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="text-sm">
                Cari sekolah
                <input
                  className={inputClass}
                  value={taskSearch}
                  onChange={(event) => setTaskSearch(event.target.value)}
                  placeholder="Nama sekolah atau NPSN"
                />
              </label>
              <label className="text-sm">
                Filter status
                <select
                  className={inputClass}
                  value={taskStatusFilter}
                  onChange={(event) => setTaskStatusFilter(event.target.value)}
                >
                  <option value="ALL">Semua status</option>
                  <option value="TODO">Belum dinilai / dikembalikan</option>
                  <option value="SUBMITTED">Sudah dikirim</option>
                  <option value="FINAL">Final</option>
                </select>
              </label>
            </div>
          )}
          {activeAssessmentStage !== "MONTHLY" && (
            <div className="max-h-64 space-y-2 overflow-y-auto">
              {filteredTasks.map((task) => (
                <button
                  key={task.assignmentId}
                  type="button"
                  aria-pressed={
                    selectedTask?.assignmentId === task.assignmentId
                  }
                  onClick={() => setSelectedTaskId(task.assignmentId)}
                  className={cn(
                    "flex w-full flex-wrap justify-between gap-2 rounded-xl border p-3 text-left text-sm",
                    selectedTask?.assignmentId === task.assignmentId &&
                      "border-emerald-500 bg-emerald-50"
                  )}
                >
                  <span>
                    {task.schoolName} · {task.npsn}
                  </span>
                  <span>{task.workflowStatus || "Belum dinilai"}</span>
                </button>
              ))}
            </div>
          )}
          {activeAssessmentStage === "MONTHLY" ? (
            <p className="text-sm text-slate-600">
              Pilih sekolah pada panel Bulanan di bawah.
            </p>
          ) : orderedTasks.length === 0 ? (
            <EmptyState
              title="Belum ada assignment tahap"
              description="Admin Master belum menugaskan sekolah untuk tahapan kompetisi."
              icon={<ClipboardList className="h-5 w-5" />}
            />
          ) : (
            orderedTasks.map((task, index) => {
              const key = `${task.stage}-${task.schoolId}`;
              const editable = needsWork(task);
              const firstCompleted =
                !editable &&
                (index === 0 || needsWork(orderedTasks[index - 1]));
              const draft = stageTaskDrafts[key] ?? {
                score:
                  typeof task.rawScore === "number"
                    ? String(task.rawScore)
                    : "",
                notes: task.notes ?? "",
              };
              return (
                <div key={task.assignmentId} className="space-y-2">
                  {index === 0 ? (
                    <h3 className="text-sm font-bold text-slate-900">
                      {editable ? "Belum Dinilai" : "Sudah Dinilai"}
                    </h3>
                  ) : firstCompleted ? (
                    <h3 className="pt-3 text-sm font-bold text-slate-900">
                      Sudah Dinilai
                    </h3>
                  ) : null}
                  <div className="rounded-2xl border border-slate-200 p-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <p className="text-xs font-semibold tracking-[0.16em] text-emerald-700 uppercase">
                          {stageTaskLabel(task.stage)}
                        </p>
                        <p className="mt-1 font-semibold text-slate-950">
                          {task.schoolName}
                        </p>
                        <p className="text-sm text-slate-500">
                          {task.npsn || "NPSN -"}
                        </p>
                      </div>
                      {task.workflowStatus ? (
                        <StatusBadge variant={editable ? "warning" : "neutral"}>
                          {task.workflowStatus === "RETURNED_BY_MASTER"
                            ? "Dikembalikan Master - perlu review"
                            : task.workflowStatus === "RETURNED_BY_KOOR"
                              ? "Dikembalikan Koor - perlu perbaikan"
                              : task.workflowStatus === "RETURNED_BY_CABDIN"
                                ? "Dikembalikan Cabdin - perlu perbaikan"
                                : task.workflowStatus.replaceAll("_", " ")}
                        </StatusBadge>
                      ) : null}
                    </div>
                    {task.returnNotes ? (
                      <div className="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
                        <p className="font-semibold">Alasan pengembalian</p>
                        <p className="mt-1 whitespace-pre-wrap">
                          {task.returnNotes}
                        </p>
                      </div>
                    ) : null}
                    {(task.stage === "NOT_FAILED_NOMINATION" ||
                      task.stage === "BOTTOM5_APPEAL") && (
                      <div className="mt-4">
                        <NominationReportPanel
                          key={`${task.stage}-${task.schoolId}`}
                          year={
                            view === "SUPERVISOR"
                              ? (supervisorDashboard?.year ?? currentYear)
                              : (juryDashboard?.year ?? currentYear)
                          }
                          schoolId={task.schoolId}
                          stage={task.stage}
                        />
                      </div>
                    )}
                    {task.stage === "PRESENTATION" && (
                      <p className="mt-3 text-sm text-slate-600">
                        Penilaian Presentasi. Ter-Favorit dan Ter-Inovatif
                        dikelola sesuai penugasan masing-masing.
                      </p>
                    )}
                    {task.stage === "NOT_FAILED_NOMINATION" ||
                    task.stage === "INNOVATION" ? (
                      <>
                        {task.stage === "INNOVATION" && (
                          <GaspolInnovationForm
                            year={currentYear}
                            schoolId={task.schoolId}
                          />
                        )}
                        {task.assessmentId && (
                          <GaspolAssessmentDetails id={task.assessmentId} />
                        )}
                        <GaspolRubricAssessment
                          key={key}
                          year={currentYear}
                          schoolId={task.schoolId}
                          stage={task.stage}
                          editable={editable}
                          onSaved={loadLiveData}
                        />
                      </>
                    ) : (
                      <div className="mt-4 grid gap-3 md:grid-cols-[180px_1fr_auto] md:items-end">
                        <div>
                          <label className="text-xs font-semibold text-slate-500 uppercase">
                            Nilai 0–100
                          </label>
                          <input
                            className={inputClass}
                            type="number"
                            min={0}
                            max={100}
                            disabled={!editable}
                            value={draft.score}
                            onChange={(event) =>
                              setStageTaskDrafts((current) => ({
                                ...current,
                                [key]: { ...draft, score: event.target.value },
                              }))
                            }
                          />
                        </div>
                        <div>
                          <label className="text-xs font-semibold text-slate-500 uppercase">
                            Catatan
                          </label>
                          <input
                            className={inputClass}
                            disabled={!editable}
                            value={draft.notes}
                            onChange={(event) =>
                              setStageTaskDrafts((current) => ({
                                ...current,
                                [key]: { ...draft, notes: event.target.value },
                              }))
                            }
                          />
                        </div>
                        <button
                          type="button"
                          disabled={!editable || isPending}
                          className={cn(
                            buttonClass,
                            "bg-emerald-600 text-white hover:bg-emerald-700"
                          )}
                          onClick={() =>
                            startTransition(() => void submitStageTask(task))
                          }
                        >
                          {task.workflowStatus === "RETURNED_BY_MASTER"
                            ? "Review Penilaian / Kirim Ulang"
                            : editable && task.workflowStatus
                              ? "Kirim Ulang"
                              : editable
                                ? "Kirim Penilaian"
                                : "Sudah Dikirim"}
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              );
            })
          )}
        </CardContent>
      </Card>
    );
  };

  const renderSupervisor = () => {
    const supervisorMissingRows = supervisorDashboard?.missingSubmissions ?? [];
    const supervisorMissingTableProps = getGaspolScrollableTableProps(
      supervisorMissingRows.length
    );
    const supervisorPendingTableProps = getGaspolScrollableTableProps(
      supervisorPendingRows.length
    );
    const supervisorCompletedTableProps = getGaspolScrollableTableProps(
      supervisorCompletedRows.length
    );
    const koorQueue = (
      supervisorDashboard?.koorPendingAssessments ?? []
    ).filter(
      (item) =>
        item.stage === koorStageFilter &&
        item.schoolName.toLowerCase().includes(reviewSchoolSearch.toLowerCase())
    );

    return (
      <div className="space-y-6">
        <div className="grid gap-4 md:grid-cols-3">
          <MetricCard
            title="Total Penugasan"
            value={supervisorDashboard?.assignmentCount ?? 0}
            icon={Users}
          />
          <MetricCard
            title="Belum Mengirim"
            value={supervisorDashboard?.missingSubmissionCount ?? 0}
            icon={AlertTriangle}
            tone="sky"
          />
          <MetricCard
            title="Sudah Dinilai"
            value={supervisorCompletedRows.length}
            icon={ShieldCheck}
            tone="slate"
          />
        </div>

        {renderStageTasks(supervisorDashboard?.stageTasks ?? [])}

        {supervisorDashboard?.isKoorGaspol ? (
          <Card className="border-sky-200 bg-sky-50/40">
            <CardHeader>
              <CardTitle>Review Koor GASPOL</CardTitle>
              <CardDescription>
                Nilai dari penilai harus disetujui di sini sebelum masuk ke
                antrean Validator Cabdin.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex flex-wrap gap-2">
                {validationStageTabs.map((stage) => (
                  <button
                    key={stage}
                    type="button"
                    onClick={() => setKoorStageFilter(stage)}
                    className={cn(
                      buttonClass,
                      koorStageFilter === stage
                        ? "bg-slate-900 text-white"
                        : "border border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
                    )}
                  >
                    {stageTaskLabel(stage)}
                  </button>
                ))}
              </div>
              <label className="block text-sm">
                Cari sekolah untuk review
                <input
                  className={inputClass}
                  value={reviewSchoolSearch}
                  onChange={(event) =>
                    setReviewSchoolSearch(event.target.value)
                  }
                  placeholder="Nama sekolah"
                />
              </label>
              <ReviewNominationEvidence
                year={supervisorDashboard?.year ?? currentYear}
                rows={koorQueue}
              />
              {koorQueue.length === 0 ? (
                <EmptyState
                  title={`Tidak ada antrean ${stageTaskLabel(koorStageFilter)}`}
                  description="Belum ada nilai pada stage ini yang menunggu persetujuan Koor GASPOL."
                  icon={<ShieldCheck className="h-5 w-5" />}
                />
              ) : (
                <Table className="min-w-[1050px]">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Sekolah</TableHead>
                      <TableHead>Tahap</TableHead>
                      <TableHead>Penilai</TableHead>
                      <TableHead>Nilai</TableHead>
                      <TableHead>Tanggal Kirim</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Catatan Penilai</TableHead>
                      <TableHead>Aksi</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {koorQueue.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell className="font-semibold text-slate-900">
                          {item.schoolName}
                        </TableCell>
                        <TableCell>{item.stage}</TableCell>
                        <TableCell>{item.assessorName || "-"}</TableCell>
                        <TableCell>
                          {item.rawScore.toFixed(2)}
                          <GaspolAssessmentDetails id={item.id} />
                        </TableCell>
                        <TableCell>
                          {item.submittedAt
                            ? new Date(item.submittedAt).toLocaleString("id-ID")
                            : "-"}
                        </TableCell>
                        <TableCell>
                          {item.workflowStatus.replaceAll("_", " ")}
                        </TableCell>
                        <TableCell>{item.notes || "-"}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-2">
                            <button
                              type="button"
                              className={cn(
                                buttonClass,
                                "bg-emerald-600 text-white hover:bg-emerald-700"
                              )}
                              onClick={() =>
                                void reviewStageAssessment(
                                  item.id,
                                  "KOOR",
                                  "APPROVE"
                                )
                              }
                            >
                              Setujui
                            </button>
                            <button
                              type="button"
                              className={cn(
                                buttonClass,
                                "border border-amber-200 bg-amber-50 text-amber-800 hover:bg-amber-100"
                              )}
                              onClick={() =>
                                void reviewStageAssessment(
                                  item.id,
                                  "KOOR",
                                  "RETURN",
                                  "Dikembalikan oleh Koor GASPOL"
                                )
                              }
                            >
                              Kembalikan
                            </button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        ) : null}

        {activeAssessmentStage === "MONTHLY" && (
          <Card>
            <CardHeader>
              <CardTitle>Penilaian Bulanan Pengawas</CardTitle>
              <CardDescription>
                Penilaian dilakukan dari tabel agar daftar yang belum selesai
                mudah dipantau.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <label className="block text-sm">
                Cari sekolah
                <input
                  className={inputClass}
                  value={monthlySearch}
                  onChange={(event) => setMonthlySearch(event.target.value)}
                  placeholder="Nama sekolah"
                />
              </label>
              <div className="grid gap-3 md:grid-cols-[220px_1fr]">
                <div>
                  <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                    Filter Bulan
                  </label>
                  <select
                    value={supervisorFilterMonth}
                    onChange={(event) =>
                      setSupervisorFilterMonth(Number(event.target.value))
                    }
                    className={inputClass}
                  >
                    {reviewMonths.map((month) => (
                      <option key={month} value={month}>
                        {monthName(month)}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
                  Menampilkan penilaian pengawas untuk{" "}
                  <span className="font-semibold text-slate-900">
                    {formatPeriod(
                      supervisorFilterMonth,
                      supervisorDashboard?.year ?? currentYear
                    )}
                  </span>
                  .
                </div>
              </div>

              <div className="flex flex-wrap gap-2">
                {(
                  [
                    ["MISSING", "Belum Mengirim Laporan"],
                    ["PENDING", "Belum Dinilai"],
                    ["DONE", "Sudah Dinilai"],
                  ] as const
                ).map(([value, label]) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => setSupervisorTab(value)}
                    className={cn(
                      buttonClass,
                      supervisorTab === value
                        ? "bg-slate-900 text-white"
                        : "border border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
                    )}
                  >
                    {label}
                  </button>
                ))}
              </div>

              {supervisorTab === "MISSING" ? (
                supervisorMissingRows.length === 0 ? (
                  <EmptyState
                    title="Semua sekolah target sudah mengirim"
                    description="Tidak ada sekolah target yang masih belum mengirim laporan."
                    icon={<CheckCircle2 className="h-5 w-5" />}
                  />
                ) : (
                  <Table
                    className="min-w-[560px]"
                    stickyHeader={supervisorMissingTableProps.stickyHeader}
                    wrapperClassName={
                      supervisorMissingTableProps.wrapperClassName
                    }
                  >
                    <TableHeader>
                      <TableRow>
                        <TableHead>Sekolah</TableHead>
                        <TableHead>Periode</TableHead>
                        <TableHead>Status Proses</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {supervisorMissingRows
                        .filter((row) =>
                          row.schoolName
                            .toLowerCase()
                            .includes(monthlySearch.toLowerCase())
                        )
                        .map((item) => (
                          <TableRow key={`${item.schoolId}-${item.month}`}>
                            <TableCell className="font-semibold text-slate-900">
                              {item.schoolName}
                            </TableCell>
                            <TableCell>
                              {formatPeriod(
                                item.month,
                                supervisorDashboard?.year ?? currentYear
                              )}
                            </TableCell>
                            <TableCell>{item.processStatus}</TableCell>
                          </TableRow>
                        ))}
                    </TableBody>
                  </Table>
                )
              ) : supervisorTab === "PENDING" ? (
                supervisorPendingRows.length === 0 ? (
                  <EmptyState
                    title="Semua laporan sudah dinilai"
                    description="Tidak ada laporan bulanan yang menunggu penilaian pengawas."
                    icon={<CheckCircle2 className="h-5 w-5" />}
                  />
                ) : (
                  <div className="space-y-4">
                    <Table
                      className="min-w-[760px]"
                      stickyHeader={supervisorPendingTableProps.stickyHeader}
                      wrapperClassName={
                        supervisorPendingTableProps.wrapperClassName
                      }
                    >
                      <TableHeader>
                        <TableRow>
                          <TableHead>Sekolah</TableHead>
                          <TableHead>Periode</TableHead>
                          <TableHead>Link</TableHead>
                          <TableHead>Status Proses</TableHead>
                          <TableHead>Aksi</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {supervisorPendingRows
                          .filter((row) =>
                            row.schoolName
                              .toLowerCase()
                              .includes(monthlySearch.toLowerCase())
                          )
                          .map((assignment) => {
                            const isReturned =
                              assignment.workflowStatus ===
                                "RETURNED_BY_KOOR" ||
                              assignment.workflowStatus ===
                                "RETURNED_BY_CABDIN";
                            return (
                              <TableRow
                                key={`${assignment.schoolId}-${assignment.month}`}
                              >
                                <TableCell className="font-semibold text-slate-900">
                                  {assignment.schoolName}
                                </TableCell>
                                <TableCell>
                                  {formatPeriod(
                                    assignment.month,
                                    supervisorDashboard?.year ?? currentYear
                                  )}
                                </TableCell>
                                <TableCell>
                                  {renderReportLinkAction(
                                    assignment.reportLink
                                  )}
                                </TableCell>
                                <TableCell>
                                  <div className="space-y-1">
                                    <p>{assignment.processStatus}</p>
                                    {isReturned ? (
                                      <StatusBadge variant="warning">
                                        Perlu Perbaikan
                                      </StatusBadge>
                                    ) : null}
                                  </div>
                                </TableCell>
                                <TableCell>
                                  <button
                                    type="button"
                                    className={cn(
                                      buttonClass,
                                      "bg-sky-600 text-white hover:bg-sky-700"
                                    )}
                                    onClick={() =>
                                      openSupervisorAssessmentEditor(assignment)
                                    }
                                  >
                                    {isReturned
                                      ? "Perbaiki Penilaian"
                                      : "Nilai"}
                                  </button>
                                </TableCell>
                              </TableRow>
                            );
                          })}
                      </TableBody>
                    </Table>

                    {activeSupervisorAssignment ? (
                      <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
                        <div className="flex flex-wrap items-start justify-between gap-3">
                          <div>
                            <p className="text-lg font-semibold text-slate-900">
                              {activeSupervisorAssignment.schoolName}
                            </p>
                            <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-slate-600">
                              <span>
                                {formatPeriod(
                                  activeSupervisorAssignment.month,
                                  supervisorDashboard?.year ?? currentYear
                                )}
                              </span>
                              {renderReportLinkAction(
                                activeSupervisorAssignment.reportLink
                              )}
                            </div>
                          </div>
                          <StatusBadge variant="warning">
                            {activeSupervisorAssignment.workflowStatus ===
                              "RETURNED_BY_KOOR" ||
                            activeSupervisorAssignment.workflowStatus ===
                              "RETURNED_BY_CABDIN"
                              ? "Perlu Perbaikan"
                              : "Belum Dinilai"}
                          </StatusBadge>
                        </div>
                        {activeSupervisorAssignment.returnNotes ? (
                          <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
                            <p className="font-semibold">
                              Catatan pengembalian
                            </p>
                            <p className="mt-1 whitespace-pre-wrap">
                              {activeSupervisorAssignment.returnNotes}
                            </p>
                          </div>
                        ) : null}
                        <div className="mt-4 grid gap-3 md:grid-cols-[180px_1fr]">
                          <input
                            className={inputClass}
                            type="number"
                            min={0}
                            max={100}
                            value={
                              supervisorScores[
                                activeSupervisorAssignment.schoolId
                              ]?.score ?? ""
                            }
                            onChange={(event) =>
                              setSupervisorScores((current) => ({
                                ...current,
                                [activeSupervisorAssignment.schoolId]: {
                                  ...current[
                                    activeSupervisorAssignment.schoolId
                                  ],
                                  score: event.target.value,
                                  notes:
                                    current[activeSupervisorAssignment.schoolId]
                                      ?.notes ?? "",
                                },
                              }))
                            }
                            placeholder="Nilai 0-100"
                          />
                          <input
                            className={inputClass}
                            value={
                              supervisorScores[
                                activeSupervisorAssignment.schoolId
                              ]?.notes ?? ""
                            }
                            onChange={(event) =>
                              setSupervisorScores((current) => ({
                                ...current,
                                [activeSupervisorAssignment.schoolId]: {
                                  ...current[
                                    activeSupervisorAssignment.schoolId
                                  ],
                                  score:
                                    current[activeSupervisorAssignment.schoolId]
                                      ?.score ?? "",
                                  notes: event.target.value,
                                },
                              }))
                            }
                            placeholder="Catatan penilaian"
                          />
                        </div>
                        <div className="mt-4 flex justify-end">
                          <button
                            type="button"
                            className={cn(
                              buttonClass,
                              "bg-emerald-600 text-white hover:bg-emerald-700"
                            )}
                            disabled={isPending}
                            onClick={() =>
                              setSupervisorConfirmation(
                                activeSupervisorAssignment
                              )
                            }
                          >
                            {activeSupervisorAssignment.workflowStatus ===
                              "RETURNED_BY_KOOR" ||
                            activeSupervisorAssignment.workflowStatus ===
                              "RETURNED_BY_CABDIN"
                              ? "Kirim Ulang"
                              : "Kirim Penilaian"}
                          </button>
                        </div>
                      </div>
                    ) : null}
                  </div>
                )
              ) : supervisorCompletedRows.length === 0 ? (
                <EmptyState
                  title="Belum ada penilaian tersimpan"
                  description="Riwayat penilaian pengawas akan muncul di sini setelah disimpan."
                  icon={<Users className="h-5 w-5" />}
                />
              ) : (
                <Table
                  className="min-w-[720px]"
                  stickyHeader={supervisorCompletedTableProps.stickyHeader}
                  wrapperClassName={
                    supervisorCompletedTableProps.wrapperClassName
                  }
                >
                  <TableHeader>
                    <TableRow>
                      <TableHead>Sekolah</TableHead>
                      <TableHead>Periode</TableHead>
                      <TableHead>Link</TableHead>
                      <TableHead>Nilai</TableHead>
                      <TableHead>Status</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {supervisorCompletedRows
                      .filter((row) =>
                        row.schoolName
                          .toLowerCase()
                          .includes(monthlySearch.toLowerCase())
                      )
                      .map((item) => (
                        <TableRow key={`${item.schoolId}-${item.month}`}>
                          <TableCell className="font-semibold text-slate-900">
                            {item.schoolName}
                          </TableCell>
                          <TableCell>
                            {formatPeriod(
                              item.month,
                              supervisorDashboard?.year ?? currentYear
                            )}
                          </TableCell>
                          <TableCell>
                            {renderReportLinkAction(item.reportLink)}
                          </TableCell>
                          <TableCell>
                            {formatScore(item.supervisorScore)}
                          </TableCell>
                          <TableCell>{item.processStatus}</TableCell>
                        </TableRow>
                      ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        )}
      </div>
    );
  };

  const renderValidator = () => {
    const validatorBlockedRows = validatorDashboard?.notReadyRows ?? [];
    const validatorPendingRows = validatorDashboard?.pendingRows ?? [];
    const validatorDoneRows = validatorDashboard?.validatedRows ?? [];
    const validatorBlockedTableProps = getGaspolScrollableTableProps(
      validatorBlockedRows.length
    );
    const validatorPendingTableProps = getGaspolScrollableTableProps(
      validatorPendingRows.length
    );
    const validatorDoneTableProps = getGaspolScrollableTableProps(
      validatorDoneRows.length
    );
    const cabdinQueue = (validatorDashboard?.validationQueue ?? []).filter(
      (item) =>
        item.stage === cabdinStageFilter &&
        item.schoolName.toLowerCase().includes(reviewSchoolSearch.toLowerCase())
    );

    return (
      <div className="space-y-6">
        <div className="grid gap-4 md:grid-cols-3">
          <MetricCard
            title="Belum Siap"
            value={validatorDashboard?.blockedCount ?? 0}
            icon={AlertTriangle}
          />
          <MetricCard
            title="Belum Divalidasi"
            value={validatorDashboard?.pendingCount ?? 0}
            icon={CheckCircle2}
            tone="sky"
          />
          <MetricCard
            title="Sudah Divalidasi"
            value={validatorDashboard?.validatedCount ?? 0}
            icon={Users}
            tone="slate"
          />
        </div>

        <Card className="border-emerald-200 bg-emerald-50/30">
          <CardHeader>
            <CardTitle>Antrean Stage — Validator Cabdin</CardTitle>
            <CardDescription>
              Hanya nilai yang sudah disetujui Koor dan ditugaskan kepada akun
              Validator Cabdin ini yang ditampilkan.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap gap-2">
              {validationStageTabs.map((stage) => (
                <button
                  key={stage}
                  type="button"
                  onClick={() => setCabdinStageFilter(stage)}
                  className={cn(
                    buttonClass,
                    cabdinStageFilter === stage
                      ? "bg-slate-900 text-white"
                      : "border border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
                  )}
                >
                  {stageTaskLabel(stage)}
                </button>
              ))}
            </div>
            <label className="block text-sm">
              Cari sekolah untuk review
              <input
                className={inputClass}
                value={reviewSchoolSearch}
                onChange={(event) => setReviewSchoolSearch(event.target.value)}
                placeholder="Nama sekolah"
              />
            </label>
            <ReviewNominationEvidence
              year={validatorDashboard?.year ?? currentYear}
              rows={cabdinQueue}
            />
            {cabdinQueue.length === 0 ? (
              <EmptyState
                title={`Tidak ada antrean ${stageTaskLabel(cabdinStageFilter)}`}
                description="Belum ada nilai pada stage ini yang menunggu validasi Cabdin."
                icon={<ShieldCheck className="h-5 w-5" />}
              />
            ) : (
              <Table className="min-w-[1120px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>Sekolah</TableHead>
                    <TableHead>Stage</TableHead>
                    <TableHead>Penilai</TableHead>
                    <TableHead>Nilai</TableHead>
                    <TableHead>Tanggal Kirim</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Catatan</TableHead>
                    <TableHead>Aksi</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {cabdinQueue.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell className="font-semibold text-slate-900">
                        {item.schoolName}
                      </TableCell>
                      <TableCell>{stageTaskLabel(item.stage)}</TableCell>
                      <TableCell>{item.assessorName || "-"}</TableCell>
                      <TableCell>
                        {item.rawScore.toFixed(2)}
                        <GaspolAssessmentDetails id={item.id} />
                      </TableCell>
                      <TableCell>
                        {item.submittedAt
                          ? new Date(item.submittedAt).toLocaleString("id-ID")
                          : "-"}
                      </TableCell>
                      <TableCell>
                        {item.workflowStatus.replaceAll("_", " ")}
                      </TableCell>
                      <TableCell>
                        {item.notes || item.reviewerNotes || "-"}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-2">
                          <button
                            type="button"
                            className={cn(
                              buttonClass,
                              "bg-emerald-600 text-white hover:bg-emerald-700"
                            )}
                            disabled={isPending}
                            onClick={() =>
                              startTransition(
                                () =>
                                  void reviewStageAssessment(
                                    item.id,
                                    "CABDIN",
                                    "APPROVE"
                                  )
                              )
                            }
                          >
                            Setujui
                          </button>
                          <button
                            type="button"
                            className={cn(
                              buttonClass,
                              "border border-amber-200 bg-amber-50 text-amber-800 hover:bg-amber-100"
                            )}
                            disabled={isPending}
                            onClick={() =>
                              startTransition(
                                () =>
                                  void reviewStageAssessment(
                                    item.id,
                                    "CABDIN",
                                    "RETURN",
                                    "Dikembalikan oleh Validator Cabdin"
                                  )
                              )
                            }
                          >
                            Kembalikan
                          </button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Validasi Tingkat Kedua — Validator Cabdin</CardTitle>
            <CardDescription>
              Validasi dilakukan langsung dari tabel agar laporan yang belum
              selesai mudah dipilah.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 md:grid-cols-[220px_1fr]">
              <div>
                <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                  Filter Bulan
                </label>
                <select
                  value={validatorFilterMonth}
                  onChange={(event) =>
                    setValidatorFilterMonth(Number(event.target.value))
                  }
                  className={inputClass}
                >
                  {reviewMonths.map((month) => (
                    <option key={month} value={month}>
                      {monthName(month)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
                Menampilkan validasi untuk{" "}
                <span className="font-semibold text-slate-900">
                  {formatPeriod(
                    validatorFilterMonth,
                    validatorDashboard?.year ?? currentYear
                  )}
                </span>
                .
              </div>
            </div>

            <div className="flex flex-wrap gap-2">
              {(
                [
                  ["BLOCKED", "Belum Mengerjakan / Dinilai Pengawas"],
                  ["PENDING", "Belum Divalidasi"],
                  ["DONE", "Sudah Divalidasi"],
                ] as const
              ).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setValidatorTab(value)}
                  className={cn(
                    buttonClass,
                    validatorTab === value
                      ? "bg-slate-900 text-white"
                      : "border border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
                  )}
                >
                  {label}
                </button>
              ))}
            </div>

            {validatorTab === "BLOCKED" ? (
              validatorBlockedRows.length === 0 ? (
                <EmptyState
                  title="Semua sekolah dalam scope sudah siap divalidasi"
                  description="Tidak ada bottleneck pada tahap sekolah atau pengawas."
                  icon={<ShieldCheck className="h-5 w-5" />}
                />
              ) : (
                <Table
                  className="min-w-[820px]"
                  stickyHeader={validatorBlockedTableProps.stickyHeader}
                  wrapperClassName={validatorBlockedTableProps.wrapperClassName}
                >
                  <TableHeader>
                    <TableRow>
                      <TableHead>Sekolah</TableHead>
                      <TableHead>Periode</TableHead>
                      <TableHead>Pengawas</TableHead>
                      <TableHead>Link</TableHead>
                      <TableHead>Status Proses</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {validatorBlockedRows.map((item) => (
                      <TableRow key={`${item.schoolId}-${item.month}`}>
                        <TableCell className="font-semibold text-slate-900">
                          {item.schoolName}
                        </TableCell>
                        <TableCell>
                          {formatPeriod(
                            item.month,
                            validatorDashboard?.year ?? currentYear
                          )}
                        </TableCell>
                        <TableCell>{item.supervisorName || "-"}</TableCell>
                        <TableCell>
                          {renderReportLinkAction(item.reportLink)}
                        </TableCell>
                        <TableCell>{item.processStatus}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )
            ) : validatorTab === "PENDING" ? (
              validatorPendingRows.length === 0 ? (
                <EmptyState
                  title="Tidak ada laporan menunggu validasi"
                  description="Semua nilai sudah tervalidasi atau belum ada submission baru."
                  icon={<ShieldCheck className="h-5 w-5" />}
                />
              ) : (
                <div className="space-y-3">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <button
                      type="button"
                      className={cn(
                        buttonClass,
                        "bg-emerald-600 text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:bg-slate-300 disabled:text-slate-600"
                      )}
                      disabled={selectedValidatorCount === 0 || isPending}
                      onClick={() =>
                        startTransition(
                          () => void approveSelectedValidatorAssessments()
                        )
                      }
                    >
                      Setujui Nilai Terpilih ({selectedValidatorCount})
                    </button>
                  </div>

                  <Table
                    className="min-w-[1120px]"
                    stickyHeader={validatorPendingTableProps.stickyHeader}
                    wrapperClassName={
                      validatorPendingTableProps.wrapperClassName
                    }
                  >
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-12">
                          <input
                            ref={validatorSelectAllRef}
                            type="checkbox"
                            aria-label="Pilih semua di halaman ini"
                            checked={allValidatorPendingSelected}
                            disabled={
                              selectableValidatorAssessmentIds.length === 0 ||
                              isPending
                            }
                            onChange={(event) =>
                              toggleAllValidatorPendingSelection(
                                event.target.checked
                              )
                            }
                            className="h-4 w-4 rounded border-slate-300 text-emerald-600"
                          />
                        </TableHead>
                        <TableHead>Sekolah</TableHead>
                        <TableHead>Periode</TableHead>
                        <TableHead>Pengawas</TableHead>
                        <TableHead>Link</TableHead>
                        <TableHead>Nilai Pengawas</TableHead>
                        <TableHead>Keputusan</TableHead>
                        <TableHead>Catatan Validator Cabdin</TableHead>
                        <TableHead>Aksi</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {validatorPendingRows.map((item) => {
                        const assessmentId = item.assessmentId ?? 0;
                        const canSelectAssessment = assessmentId > 0;
                        const draft = validatorDrafts[assessmentId] ?? {
                          decision: "AGREE" as const,
                          score: "",
                          notes: "",
                        };
                        return (
                          <TableRow key={`${item.schoolId}-${item.month}`}>
                            <TableCell>
                              <input
                                type="checkbox"
                                aria-label={`Pilih ${item.schoolName}`}
                                checked={
                                  canSelectAssessment &&
                                  selectedValidatorAssessmentIds.has(
                                    assessmentId
                                  )
                                }
                                disabled={!canSelectAssessment || isPending}
                                onChange={(event) =>
                                  toggleValidatorAssessmentSelection(
                                    assessmentId,
                                    event.target.checked
                                  )
                                }
                                className="h-4 w-4 rounded border-slate-300 text-emerald-600"
                              />
                            </TableCell>
                            <TableCell className="font-semibold text-slate-900">
                              {item.schoolName}
                            </TableCell>
                            <TableCell>
                              {formatPeriod(
                                item.month,
                                validatorDashboard?.year ?? currentYear
                              )}
                            </TableCell>
                            <TableCell>{item.supervisorName || "-"}</TableCell>
                            <TableCell>
                              {renderReportLinkAction(item.reportLink)}
                            </TableCell>
                            <TableCell>
                              {formatScore(item.supervisorScore)}
                            </TableCell>
                            <TableCell>
                              <select
                                className={inputClass}
                                value={draft.decision}
                                onChange={(event) =>
                                  setValidatorDrafts((current) => ({
                                    ...current,
                                    [assessmentId]: {
                                      ...draft,
                                      decision: event.target.value as
                                        | "AGREE"
                                        | "OBJECT",
                                    },
                                  }))
                                }
                              >
                                <option value="AGREE">
                                  Setujui menjadi final
                                </option>
                                <option value="OBJECT">
                                  Kembalikan ke penilai
                                </option>
                              </select>
                            </TableCell>
                            <TableCell>
                              <input
                                className={inputClass}
                                value={draft.notes}
                                onChange={(event) =>
                                  setValidatorDrafts((current) => ({
                                    ...current,
                                    [assessmentId]: {
                                      ...draft,
                                      notes: event.target.value,
                                    },
                                  }))
                                }
                                placeholder={
                                  draft.decision === "OBJECT"
                                    ? "Alasan pengembalian"
                                    : "Catatan opsional"
                                }
                              />
                            </TableCell>
                            <TableCell>
                              <button
                                type="button"
                                className={cn(
                                  buttonClass,
                                  "bg-emerald-600 text-white hover:bg-emerald-700"
                                )}
                                disabled={isPending}
                                onClick={() =>
                                  startTransition(
                                    () => void validateAssessment(item)
                                  )
                                }
                              >
                                Validasi
                              </button>
                            </TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                </div>
              )
            ) : validatorDoneRows.length === 0 ? (
              <EmptyState
                title="Belum ada riwayat validasi"
                description="Riwayat yang sudah tervalidasi akan muncul pada tab ini."
                icon={<ShieldCheck className="h-5 w-5" />}
              />
            ) : (
              <Table
                className="min-w-[940px]"
                stickyHeader={validatorDoneTableProps.stickyHeader}
                wrapperClassName={validatorDoneTableProps.wrapperClassName}
              >
                <TableHeader>
                  <TableRow>
                    <TableHead>Sekolah</TableHead>
                    <TableHead>Periode</TableHead>
                    <TableHead>Pengawas</TableHead>
                    <TableHead>Link</TableHead>
                    <TableHead>Hasil Validasi</TableHead>
                    <TableHead>Nilai Pengawas</TableHead>
                    <TableHead>Nilai Final Bulanan</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {validatorDoneRows.map((item) => (
                    <TableRow key={`${item.schoolId}-${item.month}`}>
                      <TableCell className="font-semibold text-slate-900">
                        {item.schoolName}
                      </TableCell>
                      <TableCell>
                        {formatPeriod(
                          item.month,
                          validatorDashboard?.year ?? currentYear
                        )}
                      </TableCell>
                      <TableCell>{item.supervisorName || "-"}</TableCell>
                      <TableCell>
                        {renderReportLinkAction(item.reportLink)}
                      </TableCell>
                      <TableCell>
                        {getValidationDecisionLabel(item.validationDecision)}
                      </TableCell>
                      <TableCell>{formatScore(item.supervisorScore)}</TableCell>
                      <TableCell>
                        {formatScore(item.finalMonthlyScore)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <RankingTable
          rows={validatorDashboard?.leaderboard ?? []}
          title="Ranking Scope Validator Cabdin"
          description="Ranking hanya sekolah yang menjadi tanggung jawab Validator Cabdin."
        />
      </div>
    );
  };

  const renderJury = () => (
    <div className="space-y-6">
      {renderStageTasks(juryDashboard?.stageTasks ?? [])}
    </div>
  );

  const renderAdmin = () => {
    const monitoringSchoolsTableProps = getGaspolScrollableTableProps(
      monitoringSchools.length
    );
    const monitoringSupervisorsTableProps = getGaspolScrollableTableProps(
      monitoringSupervisors.length
    );
    const cancelSubmitSchools = monitoringSchools.filter(
      (school) => school.schoolId > 0
    );

    return (
      <div className="space-y-6">
        <GaspolPhaseTools year={currentYear} master />
        <div className="grid gap-4 md:grid-cols-4">
          <MetricCard
            title="Peserta"
            value={adminDashboard?.participantCount ?? 0}
            icon={Users}
          />
          <MetricCard
            title="Pengawas"
            value={adminDashboard?.supervisorCount ?? 0}
            icon={ShieldCheck}
            tone="sky"
          />
          <MetricCard
            title="Validator Cabdin"
            value={adminDashboard?.validatorCount ?? 0}
            icon={ShieldCheck}
            tone="slate"
          />
          <MetricCard
            title="Pending"
            value={adminDashboard?.pendingValidation ?? 0}
            icon={AlertTriangle}
            tone="sky"
          />
        </div>

        {(adminDashboard?.unassignedCabdinAssessments ?? 0) > 0 ? (
          <div className="rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900">
            <p className="font-semibold">Menunggu penugasan Validator Cabdin</p>
            <p className="mt-1">
              {adminDashboard?.unassignedCabdinAssessments} assessment sudah
              disetujui Koor tetapi sekolahnya belum memiliki Validator Cabdin.
              Lengkapi assignment di Admin Master agar nilai tidak tertahan.
            </p>
          </div>
        ) : null}

        <Card className="border-emerald-200">
          <CardHeader>
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <CardTitle>
                  {stageOnePreview?.lock.isLocked
                    ? "Ranking Tahap 1 — Snapshot Resmi"
                    : "Preview Ranking Tahap 1 — Belum Ditetapkan"}
                </CardTitle>
                <CardDescription>
                  Ranking internal seluruh peserta. Nilai yang belum final tidak
                  diisi sebagai nol dan preview ini tidak dipublikasikan.
                </CardDescription>
              </div>
              {stageOnePreview?.lock.isLocked ? (
                <StatusBadge variant="success">
                  Top 10 telah ditetapkan
                </StatusBadge>
              ) : null}
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
              {[
                ["Peserta", stageOnePreview?.completeness.participants ?? 0],
                [
                  "Nilai Bulanan Final",
                  `${stageOnePreview?.completeness.monthlyFinal ?? 0}/${stageOnePreview?.completeness.participants ?? 0}`,
                ],
                [
                  "Lapangan Tahap 1 Terisi",
                  `${stageOnePreview?.completeness.fieldStage1Filled ?? 0}/${stageOnePreview?.completeness.participants ?? 0}`,
                ],
                [
                  "Validasi Koor",
                  `${stageOnePreview?.completeness.koorValidated ?? 0}/${stageOnePreview?.completeness.participants ?? 0}`,
                ],
                [
                  "Validasi Cabdin",
                  `${stageOnePreview?.completeness.cabdinValidated ?? 0}/${stageOnePreview?.completeness.participants ?? 0}`,
                ],
              ].map(([label, value]) => (
                <div
                  key={String(label)}
                  className="rounded-xl border border-slate-200 bg-slate-50 p-3"
                >
                  <p className="text-xs font-semibold tracking-wide text-slate-500 uppercase">
                    {label}
                  </p>
                  <p className="mt-1 text-xl font-bold text-slate-950">
                    {value}
                  </p>
                </div>
              ))}
            </div>

            {stageOnePreview?.lock.isLocked ? (
              <div className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900">
                <p className="font-semibold">Top 10 telah ditetapkan</p>
                <p className="mt-1">
                  Snapshot revisi {stageOnePreview.lock.revision ?? "-"} oleh{" "}
                  {stageOnePreview.lock.lockedBy || "Admin Master"}
                  {stageOnePreview.lock.lockedAt
                    ? ` pada ${new Date(stageOnePreview.lock.lockedAt).toLocaleString("id-ID")}`
                    : ""}
                  .
                </p>
                {stageOnePreview.officialTopTen.length ? (
                  <p className="mt-2">
                    Top 10 resmi:{" "}
                    {stageOnePreview.officialTopTen
                      .map((row) => row.schoolName)
                      .join(", ")}
                    .
                  </p>
                ) : null}
              </div>
            ) : (
              <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
                <p>
                  {stageOnePreview?.canLockTopTen
                    ? "Seluruh nilai dan validasi lengkap. Ranking siap ditetapkan."
                    : stageOnePreview?.disabledReason ||
                      "Preview ranking belum siap ditetapkan."}
                </p>
                <button
                  type="button"
                  className={cn(
                    buttonClass,
                    "bg-emerald-600 text-white hover:bg-emerald-700"
                  )}
                  disabled={!stageOnePreview?.canLockTopTen || isPending}
                  onClick={() => setShowTopTenConfirmation(true)}
                >
                  Tetapkan Top 10
                </button>
              </div>
            )}

            <Table
              className="min-w-[1320px]"
              stickyHeader
              wrapperClassName={gaspolScrollableTableClass}
            >
              <TableHeader>
                <TableRow>
                  <TableHead>Peringkat sementara</TableHead>
                  <TableHead>Sekolah</TableHead>
                  <TableHead className="text-right">Nilai Bulanan</TableHead>
                  <TableHead className="text-right">
                    Kontribusi Bulanan /20
                  </TableHead>
                  <TableHead className="text-right">Lapangan Tahap 1</TableHead>
                  <TableHead className="text-right">
                    Kontribusi T1 /40
                  </TableHead>
                  <TableHead className="text-right">Total /60</TableHead>
                  <TableHead>Status Koor</TableHead>
                  <TableHead>Status Cabdin</TableHead>
                  <TableHead>Status akhir nilai</TableHead>
                  <TableHead>Aksi QC Master</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(stageOnePreview?.rows ?? []).map((row) => (
                  <TableRow key={row.schoolId}>
                    <TableCell className="font-semibold">
                      {row.rank ?? "–"}
                    </TableCell>
                    <TableCell className="font-semibold text-slate-900">
                      {row.schoolName}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {typeof row.monthlyScore === "number"
                        ? row.monthlyScore.toFixed(2)
                        : "Belum dinilai"}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {typeof row.monthlyContribution === "number"
                        ? row.monthlyContribution.toFixed(2)
                        : "–"}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {typeof row.fieldStage1Score === "number"
                        ? row.fieldStage1Score.toFixed(2)
                        : "Belum dinilai"}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {typeof row.fieldStage1Contribution === "number"
                        ? row.fieldStage1Contribution.toFixed(2)
                        : "–"}
                    </TableCell>
                    <TableCell className="text-right font-semibold tabular-nums">
                      {typeof row.total === "number"
                        ? row.total.toFixed(2)
                        : "–"}
                    </TableCell>
                    <TableCell>{row.koorStatus}</TableCell>
                    <TableCell>{row.cabdinStatus}</TableCell>
                    <TableCell>
                      <StatusBadge
                        variant={
                          row.finalStatus === "Final"
                            ? "success"
                            : row.finalStatus === "Dikembalikan"
                              ? "danger"
                              : "warning"
                        }
                      >
                        {row.finalStatus}
                      </StatusBadge>
                    </TableCell>
                    <TableCell>
                      {!stageOnePreview?.lock.isLocked ? (
                        <div className="space-y-2">
                          {row.assessments
                            .filter(
                              (assessment) =>
                                assessment.workflowStatus === "FINAL"
                            )
                            .map((assessment) => (
                              <button
                                key={assessment.id}
                                type="button"
                                className={cn(
                                  buttonClass,
                                  "w-full bg-amber-600 text-white hover:bg-amber-700"
                                )}
                                onClick={() => {
                                  setMasterReturnNotes("");
                                  setMasterReturnTarget({
                                    id: assessment.id,
                                    schoolName: row.schoolName,
                                    assessorName: assessment.assessorName,
                                  });
                                }}
                              >
                                Kembalikan untuk Review ·{" "}
                                {assessment.assessorName}
                              </button>
                            ))}
                          {row.assessments.every(
                            (assessment) =>
                              assessment.workflowStatus !== "FINAL"
                          )
                            ? "–"
                            : null}
                        </div>
                      ) : (
                        "Terkunci"
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <div className="grid gap-6 xl:grid-cols-2">
          <RankingTable
            rows={adminDashboard?.topTen ?? []}
            title="Top 10 Sekolah"
            description="Seleksi dari nilai bulanan menuju lapangan."
            summary={adminDashboard?.rankingStatus}
          />
          <RankingTable
            rows={adminDashboard?.topFive ?? []}
            title="Top 5 Sekolah"
            description="Finalis presentasi yang ditetapkan melalui BA."
          />
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Reset Password GASPOL</CardTitle>
            <CardDescription>
              Admin master hanya dapat reset user sekolah GASPOL dan akun juri.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 md:grid-cols-2">
              <input
                className={inputClass}
                type="number"
                value={resetForm.userId}
                onChange={(event) =>
                  setResetForm((current) => ({
                    ...current,
                    userId: event.target.value,
                  }))
                }
                placeholder="User ID"
              />
              <input
                className={inputClass}
                value={resetForm.newPassword}
                onChange={(event) =>
                  setResetForm((current) => ({
                    ...current,
                    newPassword: event.target.value,
                  }))
                }
                placeholder="Password baru (opsional)"
              />
            </div>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className={cn(
                  buttonClass,
                  "bg-emerald-600 text-white hover:bg-emerald-700"
                )}
                onClick={() => startTransition(() => void resetPassword())}
              >
                Reset Password
              </button>
              <button
                type="button"
                className={cn(
                  buttonClass,
                  "bg-sky-600 text-white hover:bg-sky-700"
                )}
                onClick={() => startTransition(() => void downloadExport())}
              >
                <Download className="mr-2 h-4 w-4" />
                Export Excel
              </button>
            </div>
            {resetResult ? (
              <div className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
                <p>
                  {resetResult.displayName} ({resetResult.username}) - password
                  baru:{" "}
                  <span className="font-semibold">{resetResult.password}</span>
                </p>
              </div>
            ) : null}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Batalkan Kirim ke Penilai</CardTitle>
            <CardDescription>
              Mengembalikan laporan yang sudah dikirim ke penilai menjadi Draft
              agar sekolah dapat memperbaiki laporan/link. Fitur ini hanya
              berlaku sebelum deadline dan sebelum laporan dinilai pengawas.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 md:grid-cols-2">
              <input
                className={inputClass}
                type="number"
                value={cancelSubmitForm.year}
                onChange={(event) =>
                  setCancelSubmitForm((current) => ({
                    ...current,
                    year: Number(event.target.value) || currentYear,
                  }))
                }
                placeholder="Tahun"
              />
              <select
                className={inputClass}
                value={cancelSubmitForm.month}
                onChange={(event) =>
                  setCancelSubmitForm((current) => ({
                    ...current,
                    month: Number(event.target.value) || 4,
                  }))
                }
              >
                {schoolMonths.map((month) => (
                  <option key={month} value={month}>
                    {monthName(month)}
                  </option>
                ))}
              </select>
              <select
                className={inputClass}
                value={cancelSubmitForm.schoolId}
                onChange={(event) =>
                  setCancelSubmitForm((current) => ({
                    ...current,
                    schoolId: event.target.value,
                  }))
                }
              >
                {cancelSubmitSchools.length === 0 ? (
                  <option value="">Belum ada data sekolah GASPOL</option>
                ) : null}
                {cancelSubmitSchools.map((school) => (
                  <option key={school.schoolId} value={school.schoolId}>
                    {school.schoolName} ({school.npsn || "-"})
                  </option>
                ))}
              </select>
              <textarea
                className={cn(inputClass, "min-h-24")}
                value={cancelSubmitForm.reason}
                onChange={(event) =>
                  setCancelSubmitForm((current) => ({
                    ...current,
                    reason: event.target.value,
                  }))
                }
                placeholder="Alasan/catatan singkat"
              />
            </div>
            <button
              type="button"
              className={cn(
                buttonClass,
                "bg-amber-600 text-white hover:bg-amber-700"
              )}
              disabled={isPending || cancelSubmitSchools.length === 0}
              onClick={() =>
                startTransition(() => void cancelSubmitSchoolSubmission())
              }
            >
              Batalkan Kirim ke Penilai
            </button>
          </CardContent>
        </Card>

        <div className="grid gap-6 xl:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle>Monitoring Sekolah</CardTitle>
              <CardDescription>
                NPSN, sekolah, petugas, dan ringkasan skor.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {monitoringSchools.length === 0 ? (
                <EmptyState
                  title="Monitoring sekolah kosong"
                  description="Return 200 dengan list kosong tetap valid."
                  icon={<FileSpreadsheet className="h-5 w-5" />}
                />
              ) : (
                <Table
                  className="min-w-[720px]"
                  stickyHeader={monitoringSchoolsTableProps.stickyHeader}
                  wrapperClassName={
                    monitoringSchoolsTableProps.wrapperClassName
                  }
                >
                  <TableHeader>
                    <TableRow>
                      <TableHead>NPSN</TableHead>
                      <TableHead>Sekolah</TableHead>
                      <TableHead>Pengawas</TableHead>
                      <TableHead>Validator Cabdin</TableHead>
                      <TableHead>Total</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {monitoringSchools.map((row) => (
                      <TableRow key={row.npsn}>
                        <TableCell>{row.npsn}</TableCell>
                        <TableCell>{row.schoolName}</TableCell>
                        <TableCell>{row.assignedSupervisor || "-"}</TableCell>
                        <TableCell>{row.assignedValidator || "-"}</TableCell>
                        <TableCell>
                          {row.scores.scoreTotal.toFixed(2)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Monitoring Pengawas</CardTitle>
              <CardDescription>
                Progress tugas pengawas per bulan aktif.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {monitoringSupervisors.length === 0 ? (
                <EmptyState
                  title="Monitoring pengawas kosong"
                  description="Belum ada penugasan pengawas."
                  icon={<Users className="h-5 w-5" />}
                />
              ) : (
                <Table
                  className="min-w-[560px]"
                  stickyHeader={monitoringSupervisorsTableProps.stickyHeader}
                  wrapperClassName={
                    monitoringSupervisorsTableProps.wrapperClassName
                  }
                >
                  <TableHeader>
                    <TableRow>
                      <TableHead>Nama Pengawas</TableHead>
                      <TableHead>Assignment</TableHead>
                      <TableHead>Selesai</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {monitoringSupervisors.map((row) => (
                      <TableRow key={row.displayName}>
                        <TableCell>{row.displayName}</TableCell>
                        <TableCell>{row.assignmentCount}</TableCell>
                        <TableCell>{row.completedCount}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    );
  };

  if (user?.role === "GASPOL_VOTER")
    return <GaspolVotingPanel year={currentYear} />;
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-4">
        <Image
          src={LogoGaspolBang}
          alt="Logo Gaspol Bang"
          className="h-40 w-auto"
          priority
        />
        <div className="min-w-0">
          <p className="text-2xl font-semibold tracking-tight text-slate-950 sm:text-3xl">
            GASPOL BANG
          </p>
          <p className="mt-1 text-sm leading-5 text-slate-600 sm:text-base">
            Gerakan Antri Sampah Plastik dan Olah Lingkungan di Sekolah
            Bangkalan
          </p>
          <p className="text-xs text-slate-500 sm:text-sm">
            Cabang Dinas Wilayah Kabupaten Bangkalan
          </p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <span className="rounded-full bg-emerald-50 px-3 py-2 text-sm font-medium text-emerald-700 ring-1 ring-emerald-200">
          {gaspolViewLabel(view)}
        </span>
        {user?.role === "ADMIN" ||
        user?.role === "ADMIN_CABDIN" ||
        user?.role === "ADMIN_MASTER" ? (
          <button
            type="button"
            onClick={() => router.push("/gaspol/histori")}
            className={cn(
              buttonClass,
              "border border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
            )}
          >
            <History className="mr-2 h-4 w-4" />
            Histori
          </button>
        ) : null}
        {user?.role === "ADMIN_MASTER" ? (
          <button
            type="button"
            onClick={() => router.push("/user/settings/gaspol")}
            className={cn(
              buttonClass,
              "border border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
            )}
          >
            <Settings className="mr-2 h-4 w-4" />
            Settings
          </button>
        ) : null}
        <button
          type="button"
          onClick={refresh}
          className={cn(
            buttonClass,
            "border border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
          )}
        >
          <RefreshCcw
            className={cn("mr-2 h-4 w-4", isPending && "animate-spin")}
          />
          Refresh
        </button>
      </div>
      <p className="mx-auto text-center text-sm font-medium tracking-[0.28em] text-slate-500 uppercase">
        Scoring, ranking, monitoring, reset password, dan export Excel memakai
        data live backend.
      </p>
      {renderAlert()}

      {loading ? (
        <Card>
          <CardContent className="p-8 text-sm text-slate-500">
            Memuat dashboard GASPOL...
          </CardContent>
        </Card>
      ) : null}

      {!loading && view === "PUBLIC" ? renderPublic() : null}
      {!loading && view === "SCHOOL" ? renderSchool() : null}
      {!loading && view === "SUPERVISOR" ? renderSupervisor() : null}
      {!loading && view === "VALIDATOR" ? renderValidator() : null}
      {!loading && view === "JURY" ? renderJury() : null}
      {!loading && user && user.role !== "ADMIN_MASTER" && (
        <>
          <GaspolDelegatedTools year={currentYear} />
          <GaspolDeterminations year={currentYear} />
        </>
      )}
      {!loading && view === "ADMIN_MASTER" ? renderAdmin() : null}
      {!loading && view === "DISABLED" ? (
        <EmptyState
          title="Akses belum tersedia"
          description="Role ini belum punya scope GASPOL aktif pada tahun berjalan."
          icon={<AlertTriangle className="h-5 w-5" />}
        />
      ) : null}
      {masterReturnTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 p-4">
          <button
            type="button"
            aria-label="Tutup konfirmasi pengembalian nilai"
            className="absolute inset-0"
            disabled={isPending}
            onClick={() => setMasterReturnTarget(null)}
          />
          <section
            role="alertdialog"
            aria-modal="true"
            className="relative w-full max-w-lg rounded-3xl bg-white p-6 shadow-2xl"
          >
            <h2 className="text-xl font-bold text-slate-950">
              Kembalikan untuk Review
            </h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              Nilai Lapangan Tahap 1 {masterReturnTarget.schoolName} oleh{" "}
              {masterReturnTarget.assessorName} akan dikembalikan kepada
              pengawas. Nilai tidak dapat diedit oleh Admin Master.
            </p>
            <label className="mt-4 block text-sm font-semibold text-slate-700">
              Alasan wajib
              <textarea
                className={cn(inputClass, "mt-2 min-h-28")}
                value={masterReturnNotes}
                onChange={(event) => setMasterReturnNotes(event.target.value)}
                placeholder="Jelaskan bagian yang perlu diverifikasi ulang."
              />
            </label>
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                className={cn(
                  buttonClass,
                  "border border-slate-200 bg-white text-slate-700"
                )}
                disabled={isPending}
                onClick={() => setMasterReturnTarget(null)}
              >
                Batal
              </button>
              <button
                type="button"
                className={cn(
                  buttonClass,
                  "bg-amber-600 text-white hover:bg-amber-700"
                )}
                disabled={isPending || !masterReturnNotes.trim()}
                onClick={() =>
                  startTransition(() => void returnStageOneAssessment())
                }
              >
                {isPending ? "Mengembalikan..." : "Kembalikan untuk Review"}
              </button>
            </div>
          </section>
        </div>
      ) : null}
      {showTopTenConfirmation && stageOnePreview ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 p-4">
          <button
            type="button"
            aria-label="Tutup konfirmasi Top 10"
            className="absolute inset-0"
            disabled={isPending}
            onClick={() => setShowTopTenConfirmation(false)}
          />
          <section
            role="alertdialog"
            aria-modal="true"
            className="relative max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-3xl bg-white p-6 shadow-2xl"
          >
            <h2 className="text-xl font-bold text-slate-950">
              Tetapkan Top 10 GASPOL {stageOnePreview.year}?
            </h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              {stageOnePreview.completeness.participants} peserta telah memiliki
              nilai lengkap dan final. Hasil berikut akan dikunci sebagai
              snapshot resmi dan menjadi sumber hasil publik.
            </p>
            <ol className="mt-4 grid gap-2 sm:grid-cols-2">
              {stageOnePreview.rows.slice(0, 10).map((row) => (
                <li
                  key={row.schoolId}
                  className="rounded-xl border border-slate-200 px-3 py-2 text-sm"
                >
                  <span className="font-semibold">
                    {row.rank}. {row.schoolName}
                  </span>
                  <span className="ml-2 text-slate-500">
                    {typeof row.total === "number"
                      ? `${row.total.toFixed(2)} / 60`
                      : "–"}
                  </span>
                </li>
              ))}
            </ol>
            <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
              Setelah dikonfirmasi, rank 1–10 menjadi cohort TOP_10 dan peserta
              berikutnya menjadi NOT_FAILED sesuai snapshot penetapan.
            </div>
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                disabled={isPending}
                onClick={() => setShowTopTenConfirmation(false)}
                className={cn(buttonClass, "border border-slate-200 bg-white")}
              >
                Batal
              </button>
              <button
                type="button"
                disabled={isPending || !stageOnePreview.canLockTopTen}
                onClick={() => startTransition(() => void lockTopTen())}
                className={cn(buttonClass, "bg-emerald-600 text-white")}
              >
                {isPending ? "Menetapkan..." : "Tetapkan Top 10"}
              </button>
            </div>
          </section>
        </div>
      ) : null}
      {supervisorConfirmation ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 p-4">
          <button
            type="button"
            aria-label="Tutup konfirmasi"
            className="absolute inset-0"
            disabled={isPending}
            onClick={() => setSupervisorConfirmation(null)}
          />
          <section
            role="alertdialog"
            aria-modal="true"
            className="relative w-full max-w-md rounded-3xl bg-white p-6 shadow-2xl"
          >
            <h2 className="text-lg font-bold text-slate-950">
              {supervisorConfirmation.workflowStatus === "RETURNED_BY_KOOR" ||
              supervisorConfirmation.workflowStatus === "RETURNED_BY_CABDIN"
                ? "Kirim ulang penilaian?"
                : "Kirim penilaian?"}
            </h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              {supervisorConfirmation.workflowStatus === "RETURNED_BY_KOOR" ||
              supervisorConfirmation.workflowStatus === "RETURNED_BY_CABDIN"
                ? "Nilai yang diperbaiki akan kembali ke antrean validasi Koor GASPOL."
                : "Setelah dikirim, nilai hanya dapat diubah jika dikembalikan validator. Pastikan nilai dan catatan sudah benar."}
            </p>
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                disabled={isPending}
                onClick={() => setSupervisorConfirmation(null)}
                className={cn(buttonClass, "border border-slate-200 bg-white")}
              >
                Batal
              </button>
              <button
                type="button"
                disabled={isPending}
                onClick={() =>
                  startTransition(
                    () =>
                      void submitSupervisorAssessment(supervisorConfirmation)
                  )
                }
                className={cn(buttonClass, "bg-emerald-600 text-white")}
              >
                {isPending
                  ? "Mengirim..."
                  : supervisorConfirmation.workflowStatus ===
                        "RETURNED_BY_KOOR" ||
                      supervisorConfirmation.workflowStatus ===
                        "RETURNED_BY_CABDIN"
                    ? "Ya, Kirim Ulang"
                    : "Ya, Kirim Penilaian"}
              </button>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
