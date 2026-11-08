"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import { Card, CardContent } from "@/components/ui/card";
import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { isValidPtChoiceSlot } from "@/lib/pt-choice-rules";
import {
  getErrorMessageFromResponse,
  getFilenameFromContentDisposition,
} from "@/lib/http";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { TypeaheadInput } from "@/components/typeahead-input";
import StatusLulusanModuleActions from "@/components/siswa/status-lulusan/StatusLulusanModuleActions";
import type { OutcomeStatusValue } from "@/components/siswa/OutcomeStatusTabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { jenjangOptions } from "@/lib/shared/jenjangOptions";

type OutcomeDashboardKpis = {
  total: number;
  currentYear: number;
  prevYear: number;
  currentYearTotal: number;
  prevYearTotal: number;
  yoyPercent: number | null;
};

type OutcomeDashboardPoint = {
  year: number;
  total: number;
};

type OutcomeDashboardItem = {
  label: string;
  total: number;
  sharePercent: number;
};

type PTRankingItem = {
  ptnName: string;
  count: number;
};

type OutcomeDashboardPayload = {
  summary?: {
    pt: number;
    kerja: number;
    wirausaha: number;
    belum: number;
  };
  total?: number;
  kpis: OutcomeDashboardKpis;
  trendByYear: OutcomeDashboardPoint[];
  breakdownJalur: OutcomeDashboardItem[];
  topKampus: OutcomeDashboardItem[];
  topProdi: OutcomeDashboardItem[];
};

type OutcomeListRow = {
  id: number;
  schoolId?: number | null;
  schoolName?: string;
  schoolLevel?: string;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  finalStatus: string;
  finalPath: string;
  detail: string;
  ptKampus?: string;
  ptProdi?: string;
  ptJalur?: string;
  ptTahunDiterima?: number | null;
  source: string;
  finalizedAt?: string | null;
  isFinal: boolean;
  progressState: string;
  ptOfferCount: number;
};

type OutcomeListMeta = {
  page: number;
  pageSize: number;
  total: number;
};

type OutcomeListResponse = {
  data: OutcomeListRow[];
  meta: OutcomeListMeta;
};

type PendingPTChoice = {
  slot: number;
  kampus: string;
  prodi: string;
  jalur: string;
};

type PendingPTOffer = {
  id: number;
  jalur: string;
  kampus: string;
  prodi: string;
  tahunDiterima: number | null;
};

type PendingPTItem = {
  id: number;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  jalurRegistered: string[];
  ptChoices: PendingPTChoice[];
  ptOffers: PendingPTOffer[];
  ptOfferCount: number;
};

type PendingPTResponse = {
  finalkanPilihanSiswa: PendingPTItem[];
  multiJalur: PendingPTItem[];
};

type PTFinalizeOption = {
  key: string;
  jalur: string;
  kampus: string;
  prodi: string;
  tahunDiterima: number | null;
  disabled: boolean;
  reason: string;
};

type SchoolSuggestion = {
  id: number;
  nama_sekolah: string;
  npsn?: string | null;
};

type YearLockPayload = {
  isClosed: boolean;
  closedAt?: string | null;
  closedBy?: number | null;
};

const emptyDashboard: OutcomeDashboardPayload = {
  summary: {
    pt: 0,
    kerja: 0,
    wirausaha: 0,
    belum: 0,
  },
  total: 0,
  kpis: {
    total: 0,
    currentYear: 0,
    prevYear: 0,
    currentYearTotal: 0,
    prevYearTotal: 0,
    yoyPercent: null,
  },
  trendByYear: [],
  breakdownJalur: [],
  topKampus: [],
  topProdi: [],
};

const statusTabs = [
  { value: "PT", label: "Perguruan Tinggi" },
  { value: "KERJA", label: "Dunia Kerja" },
  { value: "WIRAUSAHA", label: "Wirausaha" },
  { value: "BELUM", label: "Belum Melanjutkan" },
] as const;

const jalurOptions = [
  { value: "ALL", label: "Semua Jalur" },
  { value: "SNBP", label: "SNBP" },
  { value: "SNBT", label: "SNBT" },
  { value: "SNBM (Mandiri)", label: "SNBM (Mandiri)" },
  { value: "Kedinasan", label: "Kedinasan" },
  { value: "Lainnya", label: "Lainnya" },
];

const statusSekolahOptions = [
  { value: "ALL", label: "Semua Status" },
  { value: "NEGERI", label: "Negeri" },
  { value: "SWASTA", label: "Swasta" },
];

const PT_PENDING_KEY = {
  single: "PT_ACCEPTED_PENDING",
  multi: "PT_ACCEPTED_MULTI_JALUR",
} as const;

const JALUR_SORT_ORDER = ["SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"];

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

const readBoolean = (value: unknown) => {
  if (typeof value === "boolean") return value;
  if (typeof value === "number") return value !== 0;
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return normalized === "true" || normalized === "1" || normalized === "yes";
  }
  return false;
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

const normalizeJalurLabel = (value: string) => {
  const raw = value.trim();
  if (!raw) return "";
  const lower = raw.toLowerCase();
  if (lower.includes("snbp")) return "SNBP";
  if (lower.includes("snbt")) return "SNBT";
  if (lower.includes("mandiri") || lower.includes("snbm")) {
    return "SNBM (Mandiri)";
  }
  if (lower.includes("kedinas")) return "Kedinasan";
  return "Lainnya";
};

const normalizeJalurKey = (value: string) => {
  const upper = value.trim().toUpperCase();
  if (!upper) return "";
  if (upper.includes("SNBP")) return "SNBP";
  if (upper.includes("SNBT")) return "SNBT";
  if (upper.includes("MANDIRI") || upper.includes("SNBM")) return "MANDIRI";
  if (upper.includes("KEDINAS")) return "KEDINASAN";
  if (upper.includes("LAIN")) return "LAINNYA";
  return upper;
};

const jalurSortWeight = (value: string) => {
  const index = JALUR_SORT_ORDER.indexOf(value);
  return index >= 0 ? index : 999;
};

const formatStatusLabel = (value: string) => {
  const upper = value.trim().toUpperCase();
  switch (upper) {
    case "PT":
      return "Perguruan Tinggi";
    case "KERJA":
      return "Dunia Kerja";
    case "WIRAUSAHA":
      return "Wirausaha";
    case "BELUM":
      return "Belum Melanjutkan";
    default:
      return value || "-";
  }
};
const fetchOutcomeDashboard = async (params: {
  status: string;
  year: string;
  jenjang: string;
  statusSekolah: string;
  schoolId?: number | null;
}): Promise<OutcomeDashboardPayload> => {
  const query = new URLSearchParams();
  if (params.status && params.status !== "ALL")
    query.set("status", params.status);
  if (params.year) query.set("year", params.year);
  if (params.jenjang && params.jenjang !== "ALL")
    query.set("jenjang", params.jenjang);
  if (params.statusSekolah && params.statusSekolah !== "ALL") {
    query.set("status_sekolah", params.statusSekolah);
  }
  if (params.schoolId) query.set("school_id", String(params.schoolId));

  const response = await apiFetch(
    `/api/siswa/outcomes/dashboard?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch outcome dashboard.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;

  const summary = isRecord(data?.summary) ? data.summary : {};
  const kpis = isRecord(data?.kpis) ? data.kpis : {};
  const summaryTotal =
    readNumber(summary.pt) +
    readNumber(summary.kerja) +
    readNumber(summary.wirausaha) +
    readNumber(summary.belum);
  return {
    summary: {
      pt: readNumber(summary.pt),
      kerja: readNumber(summary.kerja),
      wirausaha: readNumber(summary.wirausaha),
      belum: readNumber(summary.belum),
    },
    total: readNumber(data?.total) || summaryTotal,
    kpis: {
      total: readNumber(kpis.total) || readNumber(data?.total) || summaryTotal,
      currentYear: readNumber(kpis.currentYear),
      prevYear: readNumber(kpis.prevYear),
      currentYearTotal: readNumber(kpis.currentYearTotal),
      prevYearTotal: readNumber(kpis.prevYearTotal),
      yoyPercent: kpis.yoyPercent == null ? null : readNumber(kpis.yoyPercent),
    },
    trendByYear: Array.isArray(data?.trendByYear)
      ? data.trendByYear
          .map((row: unknown) => {
            if (!isRecord(row)) return null;
            const year = readNumber(row.year);
            const total = readNumber(row.total);
            if (!year) return null;
            return { year, total };
          })
          .filter(
            (row: OutcomeDashboardPoint | null): row is OutcomeDashboardPoint =>
              Boolean(row)
          )
      : [],
    breakdownJalur: Array.isArray(data?.breakdownJalur)
      ? data.breakdownJalur
          .map((row: unknown) => {
            if (!isRecord(row)) return null;
            const label = readString(row.label);
            const total = readNumber(row.total);
            if (!label) return null;
            return {
              label,
              total,
              sharePercent: readNumber(row.sharePercent),
            };
          })
          .filter(
            (row: OutcomeDashboardItem | null): row is OutcomeDashboardItem =>
              Boolean(row)
          )
      : [],
    topKampus: Array.isArray(data?.topKampus)
      ? data.topKampus
          .map((row: unknown) => {
            if (!isRecord(row)) return null;
            const label = readString(row.label);
            const total = readNumber(row.total);
            if (!label) return null;
            return {
              label,
              total,
              sharePercent: readNumber(row.sharePercent),
            };
          })
          .filter(
            (row: OutcomeDashboardItem | null): row is OutcomeDashboardItem =>
              Boolean(row)
          )
      : [],
    topProdi: Array.isArray(data?.topProdi)
      ? data.topProdi
          .map((row: unknown) => {
            if (!isRecord(row)) return null;
            const label = readString(row.label);
            const total = readNumber(row.total);
            if (!label) return null;
            return {
              label,
              total,
              sharePercent: readNumber(row.sharePercent),
            };
          })
          .filter(
            (row: OutcomeDashboardItem | null): row is OutcomeDashboardItem =>
              Boolean(row)
          )
      : [],
  };
};

const fetchOutcomeList = async (params: {
  status: string;
  page: number;
  pageSize: number;
  search: string;
  year: string;
  schoolId?: number | null;
  jalur: string;
  jenjang: string;
  statusSekolah: string;
}): Promise<OutcomeListResponse> => {
  const query = new URLSearchParams();
  query.set("status", params.status);
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.search.trim()) query.set("q", params.search.trim());
  if (params.year.trim()) query.set("year", params.year.trim());
  if (params.schoolId) query.set("school_id", String(params.schoolId));
  if (params.jalur && params.jalur !== "ALL") query.set("jalur", params.jalur);
  if (params.jenjang && params.jenjang !== "ALL")
    query.set("jenjang", params.jenjang);
  if (params.statusSekolah && params.statusSekolah !== "ALL") {
    query.set("status_sekolah", params.statusSekolah);
  }

  const response = await apiFetch(
    `/api/siswa/outcomes/list?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch outcomes list.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const rows = Array.isArray(data?.items)
    ? data.items
    : Array.isArray(data?.data)
      ? data.data
      : [];
  const meta = isRecord(data?.meta)
    ? data.meta
    : {
        page: data?.page,
        pageSize: data?.page_size ?? data?.pageSize,
        total: data?.total,
      };

  return {
    data: rows
      .map((row: unknown) => {
        if (!isRecord(row)) return null;
        return {
          id: readNumber(row.id),
          schoolId: readNumber(row.school_id ?? row.schoolId) || null,
          schoolName: readString(row.school_name ?? row.schoolName),
          schoolLevel: readString(row.school_level ?? row.schoolLevel),
          nisn: readString(row.nisn),
          namaSiswa: readString(row.nama_siswa ?? row.namaSiswa),
          rombel: readString(row.rombel),
          tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
          finalStatus: readString(row.final_status ?? row.finalStatus),
          finalPath: readString(row.final_path ?? row.finalPath),
          detail: readString(row.detail),
          ptKampus: readString(row.pt_kampus ?? row.ptKampus),
          ptProdi: readString(row.pt_prodi ?? row.ptProdi),
          ptJalur: readString(row.pt_jalur ?? row.ptJalur),
          ptTahunDiterima: (() => {
            const parsed = readNumber(
              row.pt_tahun_diterima ?? row.ptTahunDiterima
            );
            return parsed > 0 ? parsed : null;
          })(),
          source: readString(row.source) || "FINAL_SISWA_AKHIR",
          finalizedAt: readString(row.finalized_at ?? row.finalizedAt) || null,
          isFinal: readBoolean(row.is_final ?? row.isFinal),
          progressState: readString(row.progress_state ?? row.progressState),
          ptOfferCount: readNumber(row.pt_offer_count ?? row.ptOfferCount),
        } as OutcomeListRow;
      })
      .filter((row: OutcomeListRow | null): row is OutcomeListRow =>
        Boolean(row && row.id)
      ),
    meta: {
      page: readNumber(meta.page) || params.page,
      pageSize: readNumber(meta.pageSize) || params.pageSize,
      total: readNumber(meta.total),
    },
  };
};

const normalizePendingPTChoices = (rows: unknown): PendingPTChoice[] => {
  if (!Array.isArray(rows)) return [];
  return rows
    .map((row: unknown) => {
      if (!isRecord(row)) return null;
      const slot = readNumber(row.slot);
      if (!isValidPtChoiceSlot(slot)) return null;
      return {
        slot,
        kampus: readString(row.kampus),
        prodi: readString(row.prodi),
        jalur: readString(row.jalur),
      };
    })
    .filter((row): row is PendingPTChoice => Boolean(row))
    .sort((a, b) => a.slot - b.slot);
};

const normalizePendingPTOffers = (rows: unknown): PendingPTOffer[] => {
  if (!Array.isArray(rows)) return [];
  return rows
    .map((row: unknown) => {
      if (!isRecord(row)) return null;
      const id = readNumber(row.id);
      if (id <= 0) return null;
      const rawYear = row.tahun_diterima ?? row.tahunDiterima;
      const parsedYear = rawYear == null ? 0 : readNumber(rawYear);
      return {
        id,
        jalur: readString(row.jalur),
        kampus: readString(row.kampus),
        prodi: readString(row.prodi),
        tahunDiterima: parsedYear > 0 ? parsedYear : null,
      };
    })
    .filter((row): row is PendingPTOffer => Boolean(row))
    .sort((a, b) => a.id - b.id);
};

const normalizePendingJalurRegistered = (rows: unknown): string[] => {
  if (!Array.isArray(rows)) return [];
  const seen = new Set<string>();
  for (const item of rows) {
    const normalized = normalizeJalurKey(readString(item));
    if (!normalized) continue;
    seen.add(normalized);
  }
  return Array.from(seen).sort((a, b) => {
    const left = jalurSortWeight(a);
    const right = jalurSortWeight(b);
    if (left !== right) return left - right;
    return a.localeCompare(b);
  });
};

const normalizePendingPTItems = (rows: unknown): PendingPTItem[] => {
  if (!Array.isArray(rows)) return [];
  return rows
    .map((row: unknown) => {
      if (!isRecord(row)) return null;
      const ptOffers = normalizePendingPTOffers(row.pt_offers ?? row.ptOffers);
      return {
        id: readNumber(row.id),
        nisn: readString(row.nisn),
        namaSiswa: readString(row.nama_siswa ?? row.namaSiswa),
        rombel: readString(row.rombel),
        tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
        jalurRegistered: normalizePendingJalurRegistered(
          row.jalur_registered ?? row.jalurRegistered
        ),
        ptChoices: normalizePendingPTChoices(row.pt_choices ?? row.ptChoices),
        ptOffers,
        ptOfferCount: Math.max(
          readNumber(row.pt_offer_count ?? row.ptOfferCount),
          ptOffers.length
        ),
      };
    })
    .filter((row): row is PendingPTItem =>
      Boolean(row && row.id > 0 && row.nisn)
    );
};

const normalizePendingPTResponse = (payload: unknown): PendingPTResponse => {
  const data = isRecord(payload)
    ? isRecord(payload.data)
      ? payload.data
      : payload
    : {};
  return {
    finalkanPilihanSiswa: normalizePendingPTItems(
      data.finalkan_pilihan_siswa ?? data.finalkanPilihanSiswa
    ),
    multiJalur: normalizePendingPTItems(
      data.multi_jalur ?? data.multiJalur ?? data.diterima_multi_jalur
    ),
  };
};

const getPendingPTActionOptions = (row: PendingPTItem): PTFinalizeOption[] => {
  const options: PTFinalizeOption[] = [];
  const seen = new Set<string>();
  const seenByJalur = new Set<string>();

  for (const offer of row.ptOffers) {
    const jalur = normalizeJalurKey(offer.jalur);
    if (!jalur) continue;
    const kampus = offer.kampus.trim();
    const prodi = offer.prodi.trim();
    const hasDetail = kampus !== "" && prodi !== "";
    const key = `${jalur}|${kampus}|${prodi}`;
    if (seen.has(key)) continue;
    seen.add(key);
    seenByJalur.add(jalur);
    options.push({
      key,
      jalur,
      kampus,
      prodi,
      tahunDiterima: offer.tahunDiterima,
      disabled: !hasDetail,
      reason: hasDetail ? "" : "Detail PT belum lengkap.",
    });
  }

  for (const choice of row.ptChoices) {
    const jalur = normalizeJalurKey(choice.jalur);
    if (!jalur) continue;
    const kampus = choice.kampus.trim();
    const prodi = choice.prodi.trim();
    const hasDetail = kampus !== "" && prodi !== "";
    const key = `${jalur}|${kampus}|${prodi}`;
    if (seen.has(key)) continue;
    seen.add(key);
    seenByJalur.add(jalur);
    options.push({
      key,
      jalur,
      kampus,
      prodi,
      tahunDiterima: null,
      disabled: !hasDetail,
      reason: hasDetail ? "" : "Detail PT belum lengkap.",
    });
  }

  for (const jalur of row.jalurRegistered) {
    const normalized = normalizeJalurKey(jalur);
    if (!normalized || seenByJalur.has(normalized)) continue;
    const key = `${normalized}||`;
    if (seen.has(key)) continue;
    seen.add(key);
    options.push({
      key,
      jalur: normalized,
      kampus: "",
      prodi: "",
      tahunDiterima: null,
      disabled: true,
      reason: "Detail PT belum lengkap.",
    });
  }

  return options.sort((a, b) => {
    const left = jalurSortWeight(a.jalur);
    const right = jalurSortWeight(b.jalur);
    if (left !== right) return left - right;
    if (a.kampus !== b.kampus) return a.kampus.localeCompare(b.kampus);
    return a.prodi.localeCompare(b.prodi);
  });
};

const buildPendingPTKey = (nisn: string, tahunLulus: number) =>
  `${nisn.trim()}|${tahunLulus}`;

const buildSinglePTFinalizeOption = (
  row: OutcomeListRow
): PTFinalizeOption | null => {
  const jalur = normalizeJalurKey(row.ptJalur || row.finalPath);
  const kampus = (row.ptKampus ?? "").trim();
  const prodi = (row.ptProdi ?? "").trim();
  if (!jalur || !kampus || !prodi) {
    return null;
  }
  return {
    key: `${jalur}|${kampus}|${prodi}`,
    jalur,
    kampus,
    prodi,
    tahunDiterima: row.ptTahunDiterima ?? null,
    disabled: false,
    reason: "",
  };
};

const fetchPendingPTActions = async (
  year: string
): Promise<PendingPTResponse> => {
  const query = new URLSearchParams();
  if (year.trim()) query.set("year", year.trim());
  const response = await apiFetch(
    `/api/siswa/validation/pending?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch pending PT actions.");
  }
  const payload = await response.json().catch(() => null);
  return normalizePendingPTResponse(payload);
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

const fetchPTRanking = async (params: {
  year: string;
  jenjang: string;
  statusSekolah: string;
  schoolId?: number | null;
}): Promise<PTRankingItem[]> => {
  const query = new URLSearchParams();
  if (params.year.trim()) query.set("year", params.year.trim());
  if (params.jenjang && params.jenjang !== "ALL")
    query.set("jenjang", params.jenjang);
  if (params.statusSekolah && params.statusSekolah !== "ALL") {
    query.set("status_sekolah", params.statusSekolah);
  }
  if (params.schoolId) query.set("school_id", String(params.schoolId));

  const response = await apiFetch(`/api/siswa/pt/ranking?${query.toString()}`);
  if (!response.ok) {
    throw new Error("Failed to fetch PT ranking.");
  }
  const payload = await response.json().catch(() => null);
  const data = Array.isArray(payload?.data) ? payload.data : [];
  return data
    .map((row: unknown) => {
      if (!isRecord(row)) return null;
      const ptnName = readString(row.ptn_name ?? row.ptnName);
      const count = readNumber(row.count);
      if (!ptnName) return null;
      return { ptnName, count } as PTRankingItem;
    })
    .filter((row: PTRankingItem | null): row is PTRankingItem => Boolean(row));
};

const fetchOutcomeYears = async (params: {
  status: string;
  jalur: string;
  jenjang: string;
  statusSekolah: string;
  schoolId?: number | null;
}): Promise<number[]> => {
  const query = new URLSearchParams();
  query.set("status", params.status);
  if (params.jalur && params.jalur !== "ALL") query.set("jalur", params.jalur);
  if (params.jenjang && params.jenjang !== "ALL")
    query.set("jenjang", params.jenjang);
  if (params.statusSekolah && params.statusSekolah !== "ALL") {
    query.set("status_sekolah", params.statusSekolah);
  }
  if (params.schoolId) query.set("school_id", String(params.schoolId));

  const response = await apiFetch(
    `/api/siswa/outcomes/years?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch outcome years.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const rows = Array.isArray(data?.years)
    ? data.years
    : Array.isArray(data)
      ? data
      : [];

  const seen = new Set<number>();
  return rows
    .map((row: unknown) => readNumber(row))
    .filter((year: number) => year > 0)
    .filter((year: number) => {
      if (seen.has(year)) return false;
      seen.add(year);
      return true;
    })
    .sort((left: number, right: number) => right - left);
};

const fetchYearLock = async (params: {
  year: number;
  schoolId?: number | null;
}): Promise<YearLockPayload> => {
  const query = new URLSearchParams();
  query.set("tahun_lulus", String(params.year));
  if (params.schoolId) query.set("school_id", String(params.schoolId));

  const response = await apiFetch(`/api/siswa/tutup-buku?${query.toString()}`);
  if (!response.ok) {
    throw new Error("Failed to fetch year lock.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return {
    isClosed: readBoolean(data?.is_closed ?? data?.isClosed),
    closedAt: readString(data?.closed_at ?? data?.closedAt) || null,
    closedBy: readNumber(data?.closed_by ?? data?.closedBy) || null,
  };
};

const TopListTable = ({
  title,
  items,
  action,
}: {
  title: string;
  items: OutcomeDashboardItem[];
  action?: ReactNode;
}) => {
  return (
    <div className="rounded-xl border border-zinc-200 bg-white p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          {title}
        </p>
        {action}
      </div>
      {items.length === 0 ? (
        <div className="mt-3 rounded-lg border border-dashed border-zinc-200 p-4 text-sm text-zinc-500">
          Tidak ada data
        </div>
      ) : (
        <div className="mt-3 space-y-3">
          {items.map((item, index) => (
            <div
              key={`${item.label}-${index}`}
              className="rounded-lg border border-zinc-100 bg-zinc-50 p-3"
            >
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <span className="flex h-7 w-7 items-center justify-center rounded-full bg-zinc-900 text-xs font-semibold text-white">
                    {index + 1}
                  </span>
                  <div>
                    <p className="text-sm font-semibold text-zinc-900">
                      {item.label}
                    </p>
                    <p className="text-xs text-zinc-500">
                      {formatNumber(item.total)} siswa
                    </p>
                  </div>
                </div>
                <div className="text-right text-xs font-semibold text-zinc-700">
                  {item.sharePercent.toFixed(1)}%
                </div>
              </div>
              <div className="mt-2 h-2 w-full rounded-full bg-zinc-200">
                <div
                  className="h-2 rounded-full bg-zinc-900"
                  style={{ width: `${Math.min(100, item.sharePercent)}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
type StatusLulusanDashboardProps = {
  status: OutcomeStatusValue;
};

export default function StatusLulusanDashboard({
  status,
}: StatusLulusanDashboardProps) {
  const searchParams = useSearchParams();
  const { status: authStatus, user } = useAuth();
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const isAdminCabdin = user?.role === "ADMIN";
  const isAdminMaster = user?.role === "ADMIN_MASTER";
  const isPengawas = user?.role === "PENGAWAS";
  const isAdmin = isAdminCabdin || isAdminMaster;
  const canUseSchoolScope = isAdmin || isPengawas;
  const queryClient = useQueryClient();

  const activeStatus = useMemo<OutcomeStatusValue>(() => {
    const rawStatus = status.toUpperCase();
    const validValues = statusTabs.map((item) => item.value);
    return validValues.includes(rawStatus as OutcomeStatusValue)
      ? (rawStatus as OutcomeStatusValue)
      : "PT";
  }, [status]);
  const isPTStatus = activeStatus === "PT";
  const showPTInsightSections = isPTStatus;
  const showPTScopeColumns =
    isPTStatus && (isSchoolAdmin || isAdmin || isPengawas);

  const [filters, setFilters] = useState({
    year: "",
    jenjang: "ALL",
    statusSekolah: "ALL",
    jalur: "ALL",
    schoolId: null as number | null,
    schoolQuery: "",
  });
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const pageSize = 10;
  const [closeMessage, setCloseMessage] = useState("");
  const [closeError, setCloseError] = useState("");
  const [isClosing, setIsClosing] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState("");
  const [isPdfDownloading, setIsPdfDownloading] = useState(false);
  const [pdfError, setPdfError] = useState("");
  const [isPTModalOpen, setIsPTModalOpen] = useState(false);
  const [ptSearch, setPtSearch] = useState("");
  const [ptFinalizeTarget, setPtFinalizeTarget] =
    useState<OutcomeListRow | null>(null);
  const [ptFinalizeKey, setPtFinalizeKey] = useState("");
  const [ptFinalizeError, setPtFinalizeError] = useState("");
  const [ptActionLoadingRowId, setPtActionLoadingRowId] = useState<
    number | null
  >(null);
  const [cancelTarget, setCancelTarget] = useState<OutcomeListRow | null>(null);
  const [cancelError, setCancelError] = useState("");
  const [isCanceling, setIsCanceling] = useState(false);
  const [successToast, setSuccessToast] = useState("");
  const [errorToast, setErrorToast] = useState("");

  useEffect(() => {
    if (activeStatus !== "PT" && filters.jalur !== "ALL") {
      setFilters((prev) => ({ ...prev, jalur: "ALL" }));
    }
  }, [activeStatus, filters.jalur]);

  useEffect(() => {
    if (!successToast) {
      return;
    }
    const timeout = window.setTimeout(() => setSuccessToast(""), 3000);
    return () => window.clearTimeout(timeout);
  }, [successToast]);

  useEffect(() => {
    if (!errorToast) {
      return;
    }
    const timeout = window.setTimeout(() => setErrorToast(""), 4000);
    return () => window.clearTimeout(timeout);
  }, [errorToast]);

  useEffect(() => {
    if (activeStatus !== "PT") {
      setPtFinalizeTarget(null);
      setPtFinalizeError("");
      setPtFinalizeKey("");
    }
  }, [activeStatus]);

  const updateFilters = (next: Partial<typeof filters>) => {
    setFilters((prev) => ({ ...prev, ...next }));
    setPage(1);
  };

  const debouncedSearch = useDebouncedValue(search, 300);
  const debouncedSchoolQuery = useDebouncedValue(filters.schoolQuery, 300);
  const requestedSchoolID = useMemo(() => {
    const raw = searchParams.get("school_id");
    if (!raw) return null;
    const parsed = Number(raw);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
  }, [searchParams]);

  const showJenjangFilter = isAdmin;
  const jenjangChoices = useMemo(() => {
    if (isAdminCabdin) {
      const allowed = user?.allowedLevels ?? [];
      return jenjangOptions.filter((option) =>
        allowed.map((item) => item.toUpperCase()).includes(option)
      );
    }
    return jenjangOptions;
  }, [isAdminCabdin, user?.allowedLevels]);

  const { data: schoolSuggestions = [], isLoading: isSchoolLoading } = useQuery(
    {
      queryKey: ["status-lulusan-school-suggest", debouncedSchoolQuery],
      queryFn: () => fetchSchoolSuggestions(debouncedSchoolQuery),
      enabled:
        authStatus === "authenticated" &&
        isAdmin &&
        debouncedSchoolQuery.trim().length >= 2,
    }
  );

  useEffect(() => {
    if (!canUseSchoolScope || requestedSchoolID == null) {
      return;
    }
    let shouldResetPage = false;
    setFilters((prev) => {
      if (prev.schoolId === requestedSchoolID) {
        return prev;
      }
      shouldResetPage = true;
      return {
        ...prev,
        schoolId: requestedSchoolID,
      };
    });
    if (shouldResetPage) {
      setPage(1);
    }
  }, [canUseSchoolScope, requestedSchoolID]);

  const {
    data: dashboardData = emptyDashboard,
    isError: isDashboardError,
    refetch: refetchDashboard,
  } = useQuery({
    queryKey: [
      "status-lulusan-dashboard",
      authStatus,
      user?.role ?? "PUBLIC",
      activeStatus,
      filters.year,
      filters.jenjang,
      filters.statusSekolah,
      filters.schoolId,
    ],
    queryFn: () =>
      fetchOutcomeDashboard({
        status: activeStatus,
        year: filters.year,
        jenjang: filters.jenjang,
        statusSekolah: filters.statusSekolah,
        schoolId: canUseSchoolScope ? filters.schoolId : null,
      }),
    enabled: authStatus === "authenticated",
  });

  const {
    data: ptRanking = [],
    isLoading: isPTRankingLoading,
    isError: isPTRankingError,
    refetch: refetchPTRanking,
  } = useQuery({
    queryKey: [
      "status-lulusan-pt-ranking",
      authStatus,
      user?.role ?? "PUBLIC",
      user?.schoolId ?? null,
      filters.year,
      filters.jenjang,
      filters.statusSekolah,
      filters.schoolId,
    ],
    queryFn: () =>
      fetchPTRanking({
        year: filters.year,
        jenjang: filters.jenjang,
        statusSekolah: filters.statusSekolah,
        schoolId: canUseSchoolScope ? filters.schoolId : null,
      }),
    enabled: authStatus === "authenticated" && isPTStatus,
  });

  const { data: outcomeYears = [], isLoading: isOutcomeYearsLoading } =
    useQuery({
      queryKey: [
        "status-lulusan-years",
        authStatus,
        user?.role ?? "PUBLIC",
        activeStatus,
        filters.jalur,
        filters.jenjang,
        filters.statusSekolah,
        filters.schoolId,
        user?.schoolId ?? null,
      ],
      queryFn: () =>
        fetchOutcomeYears({
          status: activeStatus,
          jalur: filters.jalur,
          jenjang: filters.jenjang,
          statusSekolah: filters.statusSekolah,
          schoolId: canUseSchoolScope ? filters.schoolId : null,
        }),
      enabled: authStatus === "authenticated" && isPTStatus,
    });

  const selectedYear = useMemo(() => {
    const parsed = Number(filters.year);
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed;
    }
    if (dashboardData.kpis.currentYear > 0) {
      return dashboardData.kpis.currentYear;
    }
    return new Date().getFullYear();
  }, [filters.year, dashboardData.kpis.currentYear]);

  const schoolPTYearOptions = useMemo(() => {
    const years = [...outcomeYears];
    const selectedFilterYear = readNumber(filters.year);
    if (selectedFilterYear > 0 && !years.includes(selectedFilterYear)) {
      years.unshift(selectedFilterYear);
    }
    return years.sort((left, right) => right - left);
  }, [filters.year, outcomeYears]);
  const isAllYearsSelected = isPTStatus && !filters.year.trim();

  const {
    data: yearLock,
    isError: isYearLockError,
    isFetching: isYearLockFetching,
    refetch: refetchYearLock,
  } = useQuery({
    queryKey: [
      "graduation-year-lock",
      authStatus,
      selectedYear,
      canUseSchoolScope ? filters.schoolId : (user?.schoolId ?? null),
    ],
    queryFn: () =>
      fetchYearLock({
        year: selectedYear,
        schoolId: canUseSchoolScope ? filters.schoolId : null,
      }),
    enabled: authStatus === "authenticated" && selectedYear > 0,
  });

  const yearClosed = yearLock?.isClosed ?? false;

  useEffect(() => {
    setCloseMessage("");
    setCloseError("");
  }, [selectedYear, filters.schoolId]);

  const {
    data: listData,
    isLoading: isListLoading,
    isError: isListError,
    refetch: refetchList,
  } = useQuery({
    queryKey: [
      "status-lulusan-list",
      activeStatus,
      filters.year,
      filters.jalur,
      filters.jenjang,
      filters.statusSekolah,
      filters.schoolId,
      debouncedSearch,
      page,
    ],
    queryFn: () =>
      fetchOutcomeList({
        status: activeStatus,
        page,
        pageSize,
        search: debouncedSearch,
        year: filters.year,
        schoolId: canUseSchoolScope ? filters.schoolId : null,
        jalur: filters.jalur,
        jenjang: filters.jenjang,
        statusSekolah: filters.statusSekolah,
      }),
    enabled: authStatus === "authenticated",
  });

  const {
    data: pendingPTActions = {
      finalkanPilihanSiswa: [],
      multiJalur: [],
    },
    refetch: refetchPendingPTActions,
  } = useQuery({
    queryKey: ["status-lulusan-pt-pending-actions", filters.year],
    queryFn: () => fetchPendingPTActions(filters.year),
    enabled: authStatus === "authenticated" && isSchoolAdmin && isPTStatus,
  });

  const handleCloseYear = async () => {
    if (!isAdmin || !selectedYear) {
      return;
    }
    const scopeLabel = filters.schoolId
      ? `sekolah ${filters.schoolQuery || filters.schoolId}`
      : "semua sekolah";
    const confirmed = window.confirm(
      `Tutup buku tahun ${selectedYear} untuk ${scopeLabel}? Data final akan terkunci permanen.`
    );
    if (!confirmed) return;

    setIsClosing(true);
    setCloseMessage("");
    setCloseError("");

    try {
      const payload: Record<string, unknown> = { tahun_lulus: selectedYear };
      if (filters.schoolId) {
        payload.school_id = filters.schoolId;
      }
      const response = await apiFetch("/api/siswa/tutup-buku", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      const data = await response.json().catch(() => null);
      if (!response.ok) {
        const message =
          readString(data?.message ?? data?.details) || "Gagal menutup buku.";
        throw new Error(message);
      }
      setCloseMessage(`Tahun ${selectedYear} berhasil ditutup.`);
      void refetchYearLock();
      void refetchDashboard();
      void refetchList();
    } catch (error) {
      setCloseError(
        error instanceof Error ? error.message : "Gagal menutup buku."
      );
    } finally {
      setIsClosing(false);
    }
  };

  const chartData = useMemo(
    () =>
      dashboardData.trendByYear.map((row) => ({
        label: String(row.year),
        total: row.total,
      })),
    [dashboardData.trendByYear]
  );

  const columns = useMemo(() => {
    if (isPTStatus) {
      const ptBase = [
        { key: "nisn", label: "NISN" },
        { key: "nama", label: "Nama Siswa" },
        { key: "rombel", label: "Rombel" },
        { key: "tahun", label: "Tahun Lulus", align: "right" as const },
        { key: "jalur", label: "Jalur" },
        { key: "pt", label: "Perguruan Tinggi" },
        { key: "prodi", label: "Program/Jurusan" },
        { key: "tahunDiterima", label: "Tahun Diterima", align: "right" as const },
      ];
      return showPTScopeColumns
        ? [
            { key: "sekolah", label: "Sekolah" },
            { key: "jenjangSekolah", label: "Jenjang Sekolah" },
            ...ptBase,
          ]
        : ptBase;
    }

    const base = [
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "tahun", label: "Tahun Lulus", align: "right" as const },
      { key: "status", label: "Status" },
      { key: "detail", label: "Jalur/Detail" },
      { key: "finalized", label: "Progres", align: "center" as const },
    ];
    const withScopeColumns = showPTScopeColumns
      ? [
          { key: "sekolah", label: "Sekolah" },
          { key: "jenjang", label: "Jenjang" },
          ...base,
        ]
      : base;
    if (isSchoolAdmin) {
      return [
        ...withScopeColumns,
        { key: "actions", label: "Aksi", align: "center" as const },
      ];
    }
    if (showPTScopeColumns) {
      return withScopeColumns;
    }
    return base;
  }, [isPTStatus, isSchoolAdmin, showPTScopeColumns]);

  const tableRows = listData?.data ?? [];
  const tableMeta = listData?.meta ?? {
    page,
    pageSize,
    total: 0,
  };
  const pendingPTActionMap = useMemo(() => {
    const result = new Map<string, PendingPTItem>();
    for (const row of pendingPTActions.finalkanPilihanSiswa) {
      result.set(buildPendingPTKey(row.nisn, row.tahunLulus), row);
    }
    for (const row of pendingPTActions.multiJalur) {
      result.set(buildPendingPTKey(row.nisn, row.tahunLulus), row);
    }
    return result;
  }, [pendingPTActions.finalkanPilihanSiswa, pendingPTActions.multiJalur]);
  const pendingPTOptions = useMemo(() => {
    if (!ptFinalizeTarget) return [];
    const pendingRow = pendingPTActionMap.get(
      buildPendingPTKey(ptFinalizeTarget.nisn, ptFinalizeTarget.tahunLulus)
    );
    return pendingRow
      ? getPendingPTActionOptions(pendingRow).filter(
          (option) => !option.disabled
        )
      : [];
  }, [pendingPTActionMap, ptFinalizeTarget]);
  const selectedPendingPTOption = useMemo(
    () =>
      pendingPTOptions.find((option) => option.key === ptFinalizeKey) ??
      pendingPTOptions[0] ??
      null,
    [pendingPTOptions, ptFinalizeKey]
  );

  useEffect(() => {
    if (!ptFinalizeTarget) {
      if (ptFinalizeKey) {
        setPtFinalizeKey("");
      }
      return;
    }
    if (pendingPTOptions.length === 0) {
      return;
    }
    if (!pendingPTOptions.some((option) => option.key === ptFinalizeKey)) {
      setPtFinalizeKey(pendingPTOptions[0]?.key ?? "");
    }
  }, [pendingPTOptions, ptFinalizeKey, ptFinalizeTarget]);

  const showJalurFilter = activeStatus === "PT";
  const handleImportSuccess = () => {
    void refetchDashboard();
    void refetchList();
    if (activeStatus === "PT") {
      void refetchPTRanking();
    }
    void queryClient.invalidateQueries({
      queryKey: ["pt-pipeline"],
    });
    void queryClient.invalidateQueries({
      queryKey: ["wirausaha-pipeline"],
    });
    void queryClient.invalidateQueries({
      queryKey: ["siswa-cohorts-list"],
    });
  };

  const refreshAfterPTQuickAction = () => {
    handleImportSuccess();
    void refetchPendingPTActions();
    void queryClient.invalidateQueries({
      queryKey: ["siswa-status-pending-school"],
    });
    void queryClient.invalidateQueries({
      queryKey: ["siswa-status-pending-summary"],
    });
    void queryClient.invalidateQueries({
      queryKey: ["siswa-status-pending-recap"],
    });
  };

  const finalizePTFromStatusRow = async (
    row: OutcomeListRow,
    option: PTFinalizeOption,
    reason: string
  ) => {
    setPtActionLoadingRowId(row.id);
    try {
      const response = await apiFetch(`/api/siswa/outcomes/${row.id}`, {
        method: "PUT",
        body: JSON.stringify({
          finalStatus: "PT",
          finalPath: option.jalur,
          ptDetail: {
            kampus: option.kampus,
            prodi: option.prodi,
            tahunDiterima: option.tahunDiterima,
          },
          reason,
        }),
      });
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(response, "Gagal memfinalkan PT.")
        );
      }
      setSuccessToast(
        `Finalisasi PT untuk ${row.namaSiswa || row.nisn} berhasil.`
      );
      setPtFinalizeError("");
      setPtFinalizeTarget(null);
      setPtFinalizeKey("");
      refreshAfterPTQuickAction();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Gagal memfinalkan PT.";
      if (ptFinalizeTarget?.id === row.id) {
        setPtFinalizeError(message);
      } else {
        setErrorToast(message);
      }
    } finally {
      setPtActionLoadingRowId((current) =>
        current === row.id ? null : current
      );
    }
  };

  const handleDirectPTFinalize = async (row: OutcomeListRow) => {
    const option = buildSinglePTFinalizeOption(row);
    if (!option) {
      setErrorToast("Detail PT belum lengkap sehingga belum bisa difinalkan.");
      return;
    }
    const confirmed = window.confirm(
      `Finalkan ${row.namaSiswa || row.nisn} ke ${option.kampus} - ${option.prodi} (${normalizeJalurLabel(option.jalur)})?`
    );
    if (!confirmed) return;
    await finalizePTFromStatusRow(row, option, "STATUS_LULUSAN_SINGLE_ACTION");
  };

  const openMultiPTFinalizeModal = (row: OutcomeListRow) => {
    const pendingRow = pendingPTActionMap.get(
      buildPendingPTKey(row.nisn, row.tahunLulus)
    );
    const options = pendingRow
      ? getPendingPTActionOptions(pendingRow).filter(
          (option) => !option.disabled
        )
      : [];
    if (options.length === 0) {
      setErrorToast("Opsi PT multi jalur belum siap untuk difinalkan.");
      return;
    }
    setPtFinalizeError("");
    setPtFinalizeTarget(row);
    setPtFinalizeKey(options[0]?.key ?? "");
  };

  const closeMultiPTFinalizeModal = () => {
    if (ptActionLoadingRowId != null) return;
    setPtFinalizeError("");
    setPtFinalizeTarget(null);
    setPtFinalizeKey("");
  };

  const handleMultiPTFinalize = async () => {
    if (!ptFinalizeTarget || !selectedPendingPTOption) {
      setPtFinalizeError("Pilih opsi PT yang valid.");
      return;
    }
    await finalizePTFromStatusRow(
      ptFinalizeTarget,
      selectedPendingPTOption,
      "STATUS_LULUSAN_MULTI_ACTION"
    );
  };

  const handleConfirmCancel = async () => {
    if (!cancelTarget) {
      return;
    }

    const schoolId = cancelTarget.schoolId ?? user?.schoolId ?? null;
    if (!schoolId) {
      setCancelError("School scope tidak ditemukan.");
      return;
    }

    setIsCanceling(true);
    setCancelError("");
    try {
      const response = await apiFetch("/api/siswa/finalize/cancel", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          school_id: schoolId,
          nisn: cancelTarget.nisn,
          tahun_lulus: cancelTarget.tahunLulus,
        }),
      });
      if (!response.ok) {
        throw new Error(await getErrorMessageFromResponse(response));
      }
      setCancelTarget(null);
      setSuccessToast("Finalisasi berhasil dibatalkan.");
      handleImportSuccess();
    } catch (error) {
      setCancelError(
        error instanceof Error ? error.message : "Gagal membatalkan finalisasi."
      );
    } finally {
      setIsCanceling(false);
    }
  };

  const effectivePTRanking = useMemo(() => {
    if (ptRanking.length > 0) return ptRanking;
    return dashboardData.topKampus.map((item) => ({
      ptnName: item.label,
      count: item.total,
    }));
  }, [dashboardData.topKampus, ptRanking]);

  const filteredPTRanking = useMemo(() => {
    const term = ptSearch.trim().toLowerCase();
    if (!term) return effectivePTRanking;
    return effectivePTRanking.filter((item) =>
      item.ptnName.toLowerCase().includes(term)
    );
  }, [effectivePTRanking, ptSearch]);

  const topKampusItems = useMemo(
    () => dashboardData.topKampus.slice(0, 10),
    [dashboardData.topKampus]
  );

  const resolveFilename = (response: Response, fallback: string) =>
    getFilenameFromContentDisposition(
      response.headers.get("content-disposition"),
      fallback
    );

  const triggerDownload = (blob: Blob, filename: string) => {
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.setAttribute("download", filename);
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  const handleExport = async () => {
    setIsExporting(true);
    setExportError("");
    try {
      const query = new URLSearchParams();
      query.set("status", activeStatus);
      if (filters.year.trim()) query.set("year", filters.year.trim());
      if (filters.jenjang !== "ALL") query.set("jenjang", filters.jenjang);
      if (isAdmin && filters.statusSekolah !== "ALL") {
        query.set("status_sekolah", filters.statusSekolah);
      }
      if (canUseSchoolScope && filters.schoolId) {
        query.set("school_id", String(filters.schoolId));
      }
      if (activeStatus === "PT" && filters.jalur !== "ALL") {
        query.set("jalur", filters.jalur);
      }
      if (search.trim()) {
        query.set("q", search.trim());
      }

      const response = await apiFetch(
        `/api/siswa/outcomes/export.xlsx?${query.toString()}`
      );
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(
            response,
            "Gagal mengunduh Excel status lulusan."
          )
        );
      }
      const blob = await response.blob();
      triggerDownload(blob, resolveFilename(response, "siswa-outcomes.xlsx"));
    } catch (error) {
      setExportError(
        error instanceof Error
          ? error.message
          : "Gagal mengunduh Excel status lulusan."
      );
    } finally {
      setIsExporting(false);
    }
  };

  const handleDownloadPdf = async () => {
    setIsPdfDownloading(true);
    setPdfError("");
    try {
      const query = new URLSearchParams();
      query.set("tab", activeStatus.toLowerCase());
      if (filters.year.trim()) query.set("year", filters.year.trim());
      if (filters.jenjang !== "ALL") query.set("jenjang", filters.jenjang);
      if (isAdmin && filters.statusSekolah !== "ALL") {
        query.set("status_sekolah", filters.statusSekolah);
      }
      if (canUseSchoolScope && filters.schoolId) {
        query.set("school_id", String(filters.schoolId));
      }
      if (activeStatus === "PT" && filters.jalur !== "ALL") {
        query.set("jalur", filters.jalur);
      }
      if (search.trim()) query.set("q", search.trim());

      const response = await apiFetch(
        `/api/reports/siswa.pdf?${query.toString()}`
      );
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(
            response,
            "Gagal mengunduh PDF tab ini."
          )
        );
      }
      const blob = await response.blob();
      triggerDownload(
        blob,
        resolveFilename(response, `siswa-${activeStatus.toLowerCase()}.pdf`)
      );
    } catch (error) {
      setPdfError(
        error instanceof Error ? error.message : "Gagal mengunduh PDF tab ini."
      );
    } finally {
      setIsPdfDownloading(false);
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
            Status Lulusan (Final Outcome)
          </h1>
          <p className="text-sm text-zinc-600">
            Ringkasan hasil akhir lulusan berdasarkan status final dan detail
            tindak lanjut.
          </p>
          {isAdmin ? (
            <p className="text-xs font-medium text-zinc-500">
              Data menyesuaikan scope jenjang yang diampu.
            </p>
          ) : null}
          <div className="flex flex-wrap items-center gap-2">
            {yearClosed ? (
              <span className="rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">
                Tahun Ditutup
              </span>
            ) : null}
            {isYearLockFetching ? (
              <span className="text-xs text-zinc-500">
                Memeriksa status tahun...
              </span>
            ) : null}
            {isYearLockError ? (
              <span className="text-xs text-red-500">
                Gagal memuat status tutup buku.
              </span>
            ) : null}
            {isAdmin ? (
              <button
                type="button"
                onClick={handleCloseYear}
                disabled={yearClosed || isClosing || selectedYear <= 0}
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isClosing
                  ? "Menutup..."
                  : `Tutup Buku Tahun ${selectedYear || "-"}`}
              </button>
            ) : null}
          </div>
          {closeMessage ? (
            <div className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
              {closeMessage}
            </div>
          ) : null}
          {closeError ? (
            <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
              {closeError}
            </div>
          ) : null}
        </header>

        <StatusLulusanModuleActions
          activeStatus={activeStatus}
          isSchoolAdmin={isSchoolAdmin}
          onSuccess={handleImportSuccess}
        />

        {isAdmin ? (
          <Card>
            <CardContent className="space-y-4">
              <div className="grid gap-3 md:grid-cols-3">
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Tahun Lulus
                  </label>
                  <input
                    type="number"
                    value={filters.year}
                    onChange={(event) =>
                      updateFilters({ year: event.target.value })
                    }
                    placeholder="Semua"
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                </div>
                {showJenjangFilter ? (
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Jenjang
                    </label>
                    <Select
                      value={filters.jenjang}
                      onValueChange={(value) =>
                        updateFilters({ jenjang: value })
                      }
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue placeholder="Semua" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="ALL">Semua</SelectItem>
                        {jenjangChoices.map((option) => (
                          <SelectItem key={option} value={option}>
                            {option}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                ) : null}
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Sekolah
                  </label>
                  <TypeaheadInput
                    value={filters.schoolQuery}
                    onValueChange={(value) =>
                      updateFilters({ schoolQuery: value, schoolId: null })
                    }
                    items={schoolSuggestions.map((school) => ({
                      key: school.id,
                      label: school.nama_sekolah,
                      value: school.nama_sekolah,
                      subLabel: school.npsn ?? undefined,
                    }))}
                    onSelect={(item) => {
                      const selected = schoolSuggestions.find(
                        (school) => school.id === item.key
                      );
                      if (selected) {
                        updateFilters({
                          schoolId: selected.id,
                          schoolQuery: selected.nama_sekolah,
                        });
                      }
                    }}
                    placeholder="Ketik nama sekolah..."
                    emptyText="Sekolah tidak ditemukan"
                    isLoading={isSchoolLoading}
                    showClear={Boolean(filters.schoolId || filters.schoolQuery)}
                    onClear={() =>
                      updateFilters({ schoolId: null, schoolQuery: "" })
                    }
                  />
                </div>
              </div>

              <div className="grid gap-3 md:grid-cols-4">
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Status Sekolah
                  </label>
                  <Select
                    value={filters.statusSekolah}
                    onValueChange={(value) =>
                      updateFilters({ statusSekolah: value })
                    }
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Semua status" />
                    </SelectTrigger>
                    <SelectContent>
                      {statusSekolahOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Jalur PT
                  </label>
                  <Select
                    value={filters.jalur}
                    onValueChange={(value) => updateFilters({ jalur: value })}
                    disabled={!showJalurFilter}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Semua jalur" />
                    </SelectTrigger>
                    <SelectContent>
                      {jalurOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {!showJalurFilter ? (
                    <p className="text-xs text-zinc-500">
                      Pilih status PT untuk mengaktifkan filter jalur.
                    </p>
                  ) : null}
                </div>
                <div className="space-y-1 md:col-span-2">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Cari Siswa
                  </label>
                  <input
                    type="text"
                    value={search}
                    onChange={(event) => {
                      setSearch(event.target.value);
                      setPage(1);
                    }}
                    placeholder="Cari NISN atau nama siswa"
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={() => void handleExport()}
                  disabled={isExporting}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isExporting ? "Menyiapkan..." : "Download Excel"}
                </button>
                <button
                  type="button"
                  onClick={() => void handleDownloadPdf()}
                  disabled={isPdfDownloading}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isPdfDownloading ? "Menyiapkan..." : "Unduh PDF (Tab ini)"}
                </button>
                {exportError ? (
                  <p className="text-xs text-red-600">{exportError}</p>
                ) : null}
                {pdfError ? (
                  <p className="text-xs text-red-600">{pdfError}</p>
                ) : null}
              </div>
            </CardContent>
          </Card>
        ) : null}

        {isSchoolAdmin && !isAdmin ? (
          <Card>
            <CardContent className="grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  {isPTStatus ? "Tahun Lulusan" : "Tahun Lulus"}
                </label>
                {isPTStatus ? (
                  <Select
                    value={filters.year.trim() || "ALL"}
                    onValueChange={(value) =>
                      updateFilters({ year: value === "ALL" ? "" : value })
                    }
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Semua Tahun" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="ALL">Semua Tahun</SelectItem>
                      {schoolPTYearOptions.map((year) => (
                        <SelectItem key={year} value={String(year)}>
                          {year}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <input
                    type="number"
                    value={filters.year}
                    onChange={(event) =>
                      updateFilters({ year: event.target.value })
                    }
                    placeholder="Semua"
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                )}
                {isPTStatus && isOutcomeYearsLoading ? (
                  <p className="text-xs text-zinc-500">Memuat opsi tahun...</p>
                ) : null}
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Cari Siswa
                </label>
                <input
                  type="text"
                  value={search}
                  onChange={(event) => {
                    setSearch(event.target.value);
                    setPage(1);
                  }}
                  placeholder="Cari NISN atau nama siswa"
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="flex flex-wrap items-center gap-2 md:col-span-2">
                <button
                  type="button"
                  onClick={() => void handleExport()}
                  disabled={isExporting}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isExporting ? "Menyiapkan..." : "Download Excel"}
                </button>
                <button
                  type="button"
                  onClick={() => void handleDownloadPdf()}
                  disabled={isPdfDownloading}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isPdfDownloading ? "Menyiapkan..." : "Unduh PDF (Tab ini)"}
                </button>
                {exportError ? (
                  <p className="text-xs text-red-600">{exportError}</p>
                ) : null}
                {pdfError ? (
                  <p className="text-xs text-red-600">{pdfError}</p>
                ) : null}
              </div>
            </CardContent>
          </Card>
        ) : null}
        <div
          className={`grid gap-4 ${isAdmin ? "md:grid-cols-3" : "md:grid-cols-2"}`}
        >
          <Card className={isAdmin ? "md:col-span-2" : ""}>
            <CardContent className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                {isAllYearsSelected
                  ? "Total Final (Semua Tahun)"
                  : dashboardData.kpis.currentYear
                    ? `Total Final (${dashboardData.kpis.currentYear})`
                    : "Total Final"}
              </p>
              <p
                className={`${isAdmin ? "text-4xl" : "text-3xl"} font-semibold text-zinc-900`}
              >
                {formatNumber(
                  isAllYearsSelected
                    ? dashboardData.kpis.total
                    : dashboardData.kpis.currentYearTotal
                )}
              </p>
              <p className="text-xs text-zinc-500">
                {isAllYearsSelected
                  ? "Akumulasi seluruh data lintas tahun yang tersedia."
                  : `Dibanding tahun ${dashboardData.kpis.prevYear || "-"}: ${formatNumber(
                      dashboardData.kpis.prevYearTotal
                    )}`}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                YoY
              </p>
              <p
                className={`${isAdmin ? "text-4xl" : "text-3xl"} font-semibold text-zinc-900`}
              >
                {dashboardData.kpis.yoyPercent == null
                  ? "-"
                  : `${dashboardData.kpis.yoyPercent.toFixed(1)}%`}
              </p>
              <p className="text-xs text-zinc-500">
                Perbandingan {dashboardData.kpis.prevYear || "-"} ke{" "}
                {dashboardData.kpis.currentYear || "-"}
              </p>
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardContent className="space-y-3">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Trend 5 Tahun
              </p>
              <p className="text-sm text-zinc-600">
                Tren tahunan berdasarkan tahun lulus tanpa zero-fill.
              </p>
            </div>
            <div className="h-56 w-full">
              {isDashboardError ? (
                <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-red-200 text-sm text-red-500">
                  Gagal memuat data.
                </div>
              ) : chartData.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={chartData}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="label" />
                    <YAxis />
                    <Tooltip />
                    <Bar dataKey="total" fill="#1f2937" />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                  Tidak ada data
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        <div className="grid gap-4">
          <Card>
            <CardContent className="space-y-3">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Breakdown Jalur PT
              </p>
              {dashboardData.breakdownJalur.length === 0 ? (
                <div className="rounded-lg border border-dashed border-zinc-200 p-4 text-sm text-zinc-500">
                  Tidak ada data
                </div>
              ) : (
                <div className="space-y-3">
                  {dashboardData.breakdownJalur.map((item) => (
                    <div key={item.label} className="space-y-1">
                      <div className="flex items-center justify-between text-sm">
                        <span className="font-semibold text-zinc-900">
                          {item.label}
                        </span>
                        <span className="text-zinc-600">
                          {formatNumber(item.total)} (
                          {item.sharePercent.toFixed(1)}%)
                        </span>
                      </div>
                      <div className="h-2 w-full rounded-full bg-zinc-200">
                        <div
                          className="h-2 rounded-full bg-zinc-900"
                          style={{
                            width: `${Math.min(100, item.sharePercent)}%`,
                          }}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {showPTInsightSections ? (
          <div className="grid gap-4 md:grid-cols-2">
            <TopListTable
              title="Top 10 PTN"
              items={topKampusItems}
              action={
                effectivePTRanking.length > topKampusItems.length ? (
                  <button
                    type="button"
                    onClick={() => setIsPTModalOpen(true)}
                    className="rounded-md border border-zinc-200 bg-white px-2 py-1 text-[11px] font-semibold text-zinc-700 transition hover:bg-zinc-50"
                  >
                    Lihat semua
                  </button>
                ) : null
              }
            />
            <TopListTable title="Top 10 Prodi" items={dashboardData.topProdi} />
          </div>
        ) : null}

        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Tabel Status PT
              </p>
              <h2 className="text-lg font-semibold text-zinc-900">
                Daftar Status Lulusan PT
              </h2>
              <p className="mt-1 text-xs text-zinc-500">
                Data single dari Pengumuman PT otomatis masuk Status Lulusan. Data ganda masuk Pending Validasi.
              </p>
            </div>

            <DataTable
              columns={columns}
              rows={tableRows}
              isLoading={isListLoading}
              isError={isListError}
              emptyMessage="Belum ada data status PT."
              tableWrapperClassName="max-h-[480px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
              tableHeaderClassName="sticky top-0 z-10 bg-white"
              page={tableMeta.page}
              pageSize={tableMeta.pageSize}
              total={tableMeta.total}
              onPageChange={setPage}
              renderRow={(row) => {
                const jalur = normalizeJalurLabel(row.ptJalur || row.finalPath);
                if (isPTStatus) {
                  return (
                    <TableRow key={row.id}>
                      {showPTScopeColumns ? (
                        <>
                          <TableCell>{row.schoolName || "-"}</TableCell>
                          <TableCell>{row.schoolLevel || "-"}</TableCell>
                        </>
                      ) : null}
                      <TableCell>{row.nisn || "-"}</TableCell>
                      <TableCell>{row.namaSiswa || "-"}</TableCell>
                      <TableCell>{row.rombel || "-"}</TableCell>
                      <TableCell className="text-right">
                        {row.tahunLulus || "-"}
                      </TableCell>
                      <TableCell>
                        <span className="inline-flex rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700">
                          {jalur || row.finalPath || "-"}
                        </span>
                      </TableCell>
                      <TableCell>{row.ptKampus || "-"}</TableCell>
                      <TableCell>{row.ptProdi || "-"}</TableCell>
                      <TableCell className="text-right">
                        {row.ptTahunDiterima ?? "-"}
                      </TableCell>
                    </TableRow>
                  );
                }

                const detailFallback = [row.ptKampus, row.ptProdi]
                  .map((value) => (value ?? "").trim())
                  .filter(Boolean)
                  .join(" - ");
                const detailText = row.detail || detailFallback;
                const yearText = row.ptTahunDiterima
                  ? `Tahun ${row.ptTahunDiterima}`
                  : "";
                const isPendingSinglePT =
                  !row.isFinal && row.progressState === PT_PENDING_KEY.single;
                const isPendingMultiPT =
                  !row.isFinal && row.progressState === PT_PENDING_KEY.multi;
                const pendingPTRow = pendingPTActionMap.get(
                  buildPendingPTKey(row.nisn, row.tahunLulus)
                );
                const canDirectPTFinalize =
                  isPendingSinglePT && buildSinglePTFinalizeOption(row) != null;
                const availableMultiPTOptions = isPendingMultiPT
                  ? pendingPTRow
                    ? getPendingPTActionOptions(pendingPTRow).filter(
                        (option) => !option.disabled
                      )
                    : []
                  : [];
                const isRowBusy = ptActionLoadingRowId === row.id;
                const isAutoFinalPT =
                  row.isFinal &&
                  row.progressState === "FINAL" &&
                  !row.finalizedAt &&
                  row.source === PT_PENDING_KEY.single;
                const detailLine = row.isFinal
                  ? isAutoFinalPT
                    ? "Final otomatis"
                    : yearText
                  : isPendingMultiPT
                    ? `${row.ptOfferCount} offer diterima, pilih & finalkan dari tabel ini`
                    : "Sudah diterima PT, belum final";

                return (
                  <TableRow key={row.id}>
                    {showPTScopeColumns ? (
                      <>
                        <TableCell>{row.schoolName || "-"}</TableCell>
                        <TableCell>{row.schoolLevel || "-"}</TableCell>
                      </>
                    ) : null}
                    <TableCell>{row.nisn || "-"}</TableCell>
                    <TableCell>{row.namaSiswa || "-"}</TableCell>
                    <TableCell>{row.rombel || "-"}</TableCell>
                    <TableCell className="text-right">
                      {row.tahunLulus || "-"}
                    </TableCell>
                    <TableCell>
                      <div className="space-y-1">
                        <p className="text-xs font-semibold text-zinc-900">
                          {formatStatusLabel(row.finalStatus)}
                        </p>
                        <p className="text-xs text-zinc-500">
                          {jalur || row.finalPath || "-"}
                        </p>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="space-y-1">
                        <p className="text-xs font-semibold text-zinc-900">
                          {detailText || "-"}
                        </p>
                        <p className="text-xs text-zinc-500">
                          {detailLine || "-"}
                        </p>
                      </div>
                    </TableCell>
                    <TableCell className="text-center text-xs text-zinc-600">
                      {row.isFinal ? (
                        row.finalizedAt ? (
                          formatDate(row.finalizedAt)
                        ) : (
                          <span className="rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 font-semibold text-emerald-700">
                            Final otomatis
                          </span>
                        )
                      ) : (
                        <span className="rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 font-semibold text-amber-700">
                          Belum final
                        </span>
                      )}
                    </TableCell>
                    {false && isSchoolAdmin ? (
                      <TableCell className="text-center">
                        {row.isFinal && row.source === "FINAL_SISWA_AKHIR" ? (
                          <button
                            type="button"
                            onClick={() => {
                              setCancelError("");
                              setCancelTarget(row);
                            }}
                            className="rounded-md border border-red-200 bg-red-50 px-3 py-1.5 text-xs font-semibold text-red-700 transition hover:bg-red-100"
                          >
                            Batalkan Finalisasi
                          </button>
                        ) : isPendingSinglePT ? (
                          <button
                            type="button"
                            onClick={() => void handleDirectPTFinalize(row)}
                            disabled={isRowBusy || !canDirectPTFinalize}
                            className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-semibold text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {isRowBusy ? "Memfinalkan..." : "Finalkan"}
                          </button>
                        ) : isPendingMultiPT ? (
                          <button
                            type="button"
                            onClick={() => openMultiPTFinalizeModal(row)}
                            disabled={
                              isRowBusy || availableMultiPTOptions.length === 0
                            }
                            className="rounded-md border border-sky-200 bg-sky-50 px-3 py-1.5 text-xs font-semibold text-sky-700 transition hover:bg-sky-100 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {isRowBusy ? "Memfinalkan..." : "Pilih & Finalkan"}
                          </button>
                        ) : (
                          <span className="text-xs font-medium text-zinc-400">
                            -
                          </span>
                        )}
                      </TableCell>
                    ) : null}
                  </TableRow>
                );
              }}
            />
          </CardContent>
        </Card>

        {ptFinalizeTarget ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
            <div className="w-full max-w-lg rounded-xl bg-white p-5 shadow-lg">
              <div className="space-y-2">
                <p className="text-xs font-semibold tracking-[0.2em] text-sky-600 uppercase">
                  Finalisasi PT
                </p>
                <h3 className="text-lg font-semibold text-zinc-900">
                  {ptFinalizeTarget.namaSiswa || ptFinalizeTarget.nisn}
                </h3>
                <p className="text-sm text-zinc-600">
                  Pilih offer PT yang akan dijadikan hasil akhir final.
                </p>
              </div>

              <div className="mt-4 space-y-3">
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Opsi PT
                  </label>
                  <Select
                    value={selectedPendingPTOption?.key ?? ""}
                    onValueChange={(value) => {
                      setPtFinalizeError("");
                      setPtFinalizeKey(value);
                    }}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Pilih offer PT" />
                    </SelectTrigger>
                    <SelectContent>
                      {pendingPTOptions.map((option) => (
                        <SelectItem key={option.key} value={option.key}>
                          {`${normalizeJalurLabel(option.jalur)} - ${option.kampus} - ${option.prodi}`}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                {selectedPendingPTOption ? (
                  <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
                    <p className="font-semibold text-zinc-900">
                      {selectedPendingPTOption.kampus} -{" "}
                      {selectedPendingPTOption.prodi}
                    </p>
                    <p className="text-xs text-zinc-500">
                      Jalur {normalizeJalurLabel(selectedPendingPTOption.jalur)}
                      {selectedPendingPTOption.tahunDiterima
                        ? ` • Tahun ${selectedPendingPTOption.tahunDiterima}`
                        : ""}
                    </p>
                  </div>
                ) : null}

                {ptFinalizeError ? (
                  <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                    {ptFinalizeError}
                  </div>
                ) : null}
              </div>

              <div className="mt-5 flex flex-wrap justify-end gap-2">
                <button
                  type="button"
                  onClick={closeMultiPTFinalizeModal}
                  disabled={ptActionLoadingRowId === ptFinalizeTarget.id}
                  className="rounded-md border border-zinc-200 px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  Tutup
                </button>
                <button
                  type="button"
                  onClick={() => void handleMultiPTFinalize()}
                  disabled={
                    !selectedPendingPTOption ||
                    ptActionLoadingRowId === ptFinalizeTarget.id
                  }
                  className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {ptActionLoadingRowId === ptFinalizeTarget.id
                    ? "Memfinalkan..."
                    : "Pilih & Finalkan"}
                </button>
              </div>
            </div>
          </div>
        ) : null}

        {cancelTarget ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
            <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
              <div className="space-y-2">
                <p className="text-xs font-semibold tracking-[0.2em] text-red-600 uppercase">
                  Batalkan Finalisasi
                </p>
                <h3 className="text-lg font-semibold text-zinc-900">
                  {cancelTarget.namaSiswa || cancelTarget.nisn}
                </h3>
                <p className="text-sm text-zinc-600">
                  Yang dihapus hanya data finalisasi aktif. Data rencana lulusan
                  tetap aman dan bisa diinput ulang setelah finalisasi
                  dibatalkan. Lanjutkan?
                </p>
                {cancelError ? (
                  <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                    {cancelError}
                  </div>
                ) : null}
              </div>
              <div className="mt-5 flex flex-wrap justify-end gap-2">
                <button
                  type="button"
                  onClick={() => {
                    if (isCanceling) return;
                    setCancelError("");
                    setCancelTarget(null);
                  }}
                  className="rounded-md border border-zinc-200 px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-50"
                >
                  Tutup
                </button>
                <button
                  type="button"
                  onClick={() => void handleConfirmCancel()}
                  disabled={isCanceling}
                  className="rounded-md bg-red-600 px-3 py-2 text-sm font-semibold text-white transition hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isCanceling ? "Membatalkan..." : "Batalkan Finalisasi"}
                </button>
              </div>
            </div>
          </div>
        ) : null}

        {isPTModalOpen ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
            <div className="w-full max-w-3xl rounded-xl bg-white p-4 shadow-lg sm:p-5">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Ranking PTN
                  </p>
                  <h3 className="text-lg font-semibold text-zinc-900">
                    Daftar Lengkap PTN
                  </h3>
                </div>
                <button
                  type="button"
                  onClick={() => setIsPTModalOpen(false)}
                  className="rounded-md border border-zinc-200 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-50"
                >
                  Tutup
                </button>
              </div>

              <div className="mt-4 space-y-3">
                <input
                  type="text"
                  value={ptSearch}
                  onChange={(event) => setPtSearch(event.target.value)}
                  placeholder="Cari PTN..."
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
                {isPTRankingError ? (
                  <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600">
                    <div className="flex items-center justify-between gap-3">
                      <span>Gagal memuat ranking PTN.</span>
                      <button
                        type="button"
                        onClick={() => void refetchPTRanking()}
                        className="rounded-md border border-red-200 bg-white px-2 py-1 text-xs font-semibold text-red-700 hover:bg-red-50"
                      >
                        Coba lagi
                      </button>
                    </div>
                  </div>
                ) : null}
                <div className="max-h-[420px] overflow-y-auto rounded-lg border border-zinc-200">
                  {isPTRankingLoading ? (
                    <p className="p-4 text-sm text-zinc-500">
                      Memuat ranking...
                    </p>
                  ) : filteredPTRanking.length === 0 ? (
                    <p className="p-4 text-sm text-zinc-500">
                      Data PTN tidak ditemukan.
                    </p>
                  ) : (
                    <div className="divide-y divide-zinc-100">
                      {filteredPTRanking.map((item, index) => (
                        <div
                          key={`${item.ptnName}-${index}`}
                          className="flex items-center justify-between gap-3 px-4 py-3 text-sm"
                        >
                          <span className="font-semibold text-zinc-900">
                            {index + 1}. {item.ptnName}
                          </span>
                          <span className="text-zinc-600">
                            {formatNumber(item.count)} siswa
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        ) : null}
        {successToast ? (
          <div className="fixed right-4 bottom-4 z-50 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-semibold text-emerald-700 shadow-lg">
            {successToast}
          </div>
        ) : null}
        {errorToast ? (
          <div className="fixed right-4 bottom-20 z-50 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm font-semibold text-red-700 shadow-lg">
            {errorToast}
          </div>
        ) : null}
      </section>
    </AdminRouteGuard>
  );
}
