"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Settings } from "lucide-react";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import {
  canUseScopedSchoolFilter,
  isSchoolAdminRole,
} from "@/lib/auth/role-scope";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";

type CohortRow = {
  id: number;
  schoolId: number;
  schoolName: string;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  status: string;
};

type RequestRow = {
  id: number;
  studentId: number;
  schoolId: number;
  schoolName: string;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  requestType: "NONAKTIF" | "KOREKSI";
  reason: string;
  schoolNote: string;
  supportLink: string;
  status: string;
  submittedBy: string;
  submittedAt: string;
  processedBy: string;
  processedAt?: string | null;
  processorNote: string;
};
type ActiveTab = "ACTIVE" | "PENDING" | "NONAKTIF" | "KOREKSI";
type YearSummaryRow = {
  id: number;
  schoolId: number;
  schoolName: string;
  tahunLulus: number;
  totalSiswaAkhir: number;
  source: string;
  sourceLabel: string;
  autoSummaryNotes: string;
};

const nonactiveReasons = [
  "Mengundurkan diri",
  "Pindah sekolah",
  "Cuti sekolah",
  "Putus sekolah",
  "Meninggal dunia",
  "Mutasi keluar wilayah Cabdin",
  "Tidak melanjutkan sekolah",
  "Lainnya",
];

const correctionReasons = [
  "Data ganda",
  "Salah input",
  "Salah import",
  "Bukan siswa kelas akhir",
  "Salah NISN",
  "Salah rombel",
  "Salah tahun lulus",
  "Salah sekolah",
  "Lainnya",
];

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const readNumber = (value: unknown) => {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const readString = (value: unknown) =>
  typeof value === "string"
    ? value.trim()
    : typeof value === "number" && Number.isFinite(value)
      ? String(value)
      : "";

const normalizeRequestRow = (item: unknown): RequestRow => {
  const row = isRecord(item) ? item : {};
  return {
    id: readNumber(row.id),
    studentId: readNumber(row.student_id ?? row.studentId),
    schoolId: readNumber(row.school_id ?? row.schoolId),
    schoolName: readString(row.school_name ?? row.schoolName),
    nisn: readString(row.nisn),
    namaSiswa: readString(row.nama_siswa ?? row.namaSiswa),
    rombel: readString(row.rombel),
    tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
    requestType:
      readString(row.request_type ?? row.requestType) === "KOREKSI"
        ? "KOREKSI"
        : "NONAKTIF",
    reason: readString(row.reason),
    schoolNote: readString(row.school_note ?? row.schoolNote),
    supportLink: readString(row.support_link ?? row.supportLink),
    status: readString(row.status),
    submittedBy: readString(row.submitted_by ?? row.submittedBy),
    submittedAt: readString(row.submitted_at ?? row.submittedAt),
    processedBy: readString(row.processed_by ?? row.processedBy),
    processedAt: readString(row.processed_at ?? row.processedAt) || null,
    processorNote: readString(row.processor_note ?? row.processorNote),
  };
};

const normalizeYearSummaryRow = (item: unknown): YearSummaryRow => {
  const row = isRecord(item) ? item : {};
  return {
    id: readNumber(row.id),
    schoolId: readNumber(row.school_id ?? row.schoolId),
    schoolName: readString(row.school_name ?? row.schoolName),
    tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
    totalSiswaAkhir: readNumber(row.total_siswa_akhir ?? row.totalSiswaAkhir),
    source: readString(row.source),
    sourceLabel: readString(row.source_label ?? row.sourceLabel),
    autoSummaryNotes: readString(
      row.auto_summary_notes ?? row.autoSummaryNotes
    ),
  };
};

const fetchYearSummaries = async (year?: number) => {
  const query = new URLSearchParams();
  if (year && year > 0) query.set("year", String(year));
  const response = await apiFetch(
    `/api/siswa/year-summaries?${query.toString()}`
  );
  if (!response.ok) throw new Error("Failed to fetch year summaries.");
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return Array.isArray(data?.items)
    ? data.items.map(normalizeYearSummaryRow)
    : [];
};

const fetchCohorts = async (params: {
  page: number;
  pageSize: number;
  search: string;
}) => {
  const query = new URLSearchParams({
    page: String(params.page),
    pageSize: String(params.pageSize),
  });
  if (params.search.trim()) query.set("q", params.search.trim());
  const response = await apiFetch(
    `/api/siswa/cohorts/list?${query.toString()}`
  );
  if (!response.ok) throw new Error("Failed to fetch cohorts.");
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return {
    data: Array.isArray(data?.data)
      ? data.data.map((item: unknown) => {
          const row = isRecord(item) ? item : {};
          return {
            id: readNumber(row.id),
            schoolId: readNumber(row.school_id ?? row.schoolId),
            schoolName: readString(row.school_name ?? row.schoolName),
            nisn: readString(row.nisn),
            namaSiswa: readString(row.namaSiswa ?? row.nama_siswa),
            rombel: readString(row.rombel ?? row.rombel_siswa),
            tahunLulus: readNumber(row.tahunLulus ?? row.tahun_lulus),
            status: readString(row.status),
          } satisfies CohortRow;
        })
      : [],
    meta: isRecord(data?.meta)
      ? {
          page: readNumber(data.meta.page) || params.page,
          pageSize:
            readNumber(data.meta.pageSize ?? data.meta.page_size) ||
            params.pageSize,
          total: readNumber(data.meta.total),
        }
      : { page: params.page, pageSize: params.pageSize, total: 0 },
  };
};

const fetchRequests = async (params: {
  tab: ActiveTab;
  page: number;
  pageSize: number;
  search: string;
}) => {
  const query = new URLSearchParams({
    page: String(params.page),
    pageSize: String(params.pageSize),
  });
  if (params.search.trim()) query.set("q", params.search.trim());
  if (params.tab === "PENDING") query.set("status", "PENDING");
  if (params.tab === "NONAKTIF") query.set("type", "NONAKTIF");
  if (params.tab === "KOREKSI") query.set("type", "KOREKSI");
  const response = await apiFetch(
    `/api/siswa/final-requests?${query.toString()}`
  );
  if (!response.ok) throw new Error("Failed to fetch requests.");
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return {
    data: Array.isArray(data?.data) ? data.data.map(normalizeRequestRow) : [],
    meta: isRecord(data?.meta)
      ? {
          page: readNumber(data.meta.page) || params.page,
          pageSize:
            readNumber(data.meta.pageSize ?? data.meta.page_size) ||
            params.pageSize,
          total: readNumber(data.meta.total),
        }
      : { page: params.page, pageSize: params.pageSize, total: 0 },
  };
};

export default function SiswaKelasAkhirPage() {
  const { status, user } = useAuth();
  const router = useRouter();
  const isSchoolAdmin = isSchoolAdminRole(user?.role);
  const isMaster = user?.role === "ADMIN_MASTER";
  const showSchoolColumn = canUseScopedSchoolFilter(user?.role);
  const [tab] = useState<ActiveTab>("ACTIVE");
  const [page] = useState(1);
  const [search] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [requestTarget, setRequestTarget] = useState<
    CohortRow | RequestRow | null
  >(null);
  const [requestType] = useState<"NONAKTIF" | "KOREKSI">(
    "NONAKTIF"
  );
  const [reason, setReason] = useState("");
  const [schoolNote, setSchoolNote] = useState("");
  const [supportLink, setSupportLink] = useState("");
  const [processTarget, setProcessTarget] = useState<RequestRow | null>(null);
  const [processAction] = useState<"APPROVE" | "REJECT">(
    "APPROVE"
  );
  const [processorNote, setProcessorNote] = useState("");
  const currentYear = new Date().getFullYear();
  const [summaryYear, setSummaryYear] = useState(currentYear);
  const [summaryTotal, setSummaryTotal] = useState("");
  const [summaryMessage, setSummaryMessage] = useState("");
  const pageSize = 10;


  const summaryQuery = useQuery({
    queryKey: ["siswa-year-summaries", summaryYear],
    queryFn: () => fetchYearSummaries(summaryYear),
    enabled: status === "authenticated",
  });

  const summaryMutation = useMutation({
    mutationFn: async () => {
      const total = Number(summaryTotal);
      if (!Number.isFinite(total) || total < 0) {
        throw new Error("Jumlah siswa akhir wajib berupa angka 0 atau lebih.");
      }
      const response = await apiFetch("/api/siswa/year-summaries", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          tahun_lulus: summaryYear,
          total_siswa_akhir: total,
        }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(
          readString(payload?.message ?? payload?.details) ||
            "Gagal menyimpan jumlah siswa akhir."
        );
      }
    },
    onSuccess: () => {
      setSummaryMessage("Jumlah siswa akhir tersimpan.");
      void summaryQuery.refetch();
    },
  });

  const activeQuery = useQuery({
    queryKey: ["siswa-cohorts-list", page, pageSize, debouncedSearch],
    queryFn: () => fetchCohorts({ page, pageSize, search: debouncedSearch }),
    enabled: status === "authenticated" && tab === "ACTIVE",
  });

  const requestQuery = useQuery({
    queryKey: ["siswa-final-requests", tab, page, pageSize, debouncedSearch],
    queryFn: () =>
      fetchRequests({ tab, page, pageSize, search: debouncedSearch }),
    enabled: status === "authenticated" && tab !== "ACTIVE",
  });

  const refresh = () => {
    void summaryQuery.refetch();
    void activeQuery.refetch();
    void requestQuery.refetch();
  };

  const submitMutation = useMutation({
    mutationFn: async () => {
      if (!requestTarget) return;
      const response = await apiFetch("/api/siswa/final-requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          student_id:
            "studentId" in requestTarget
              ? requestTarget.studentId
              : requestTarget.id,
          request_type: requestType,
          reason,
          school_note: schoolNote,
          support_link: supportLink,
        }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(
          readString(payload?.message ?? payload?.details) ||
            "Gagal menyimpan pengajuan."
        );
      }
    },
    onSuccess: () => {
      setRequestTarget(null);
      setReason("");
      setSchoolNote("");
      setSupportLink("");
      refresh();
    },
  });

  const processMutation = useMutation({
    mutationFn: async () => {
      if (!processTarget) return;
      const response = await apiFetch(
        `/api/siswa/final-requests/${processTarget.id}/process`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            action: processAction,
            processor_note: processorNote,
          }),
        }
      );
      if (!response.ok) throw new Error("Gagal memproses pengajuan.");
    },
    onSuccess: () => {
      setProcessTarget(null);
      setProcessorNote("");
      refresh();
    },
  });

  return (
    <AdminRouteGuard>
      <section className="space-y-6">
        <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Siswa
            </p>
            <h1 className="text-3xl font-semibold text-zinc-900">
              Data Siswa Kelas Akhir
            </h1>
            <p className="text-sm text-zinc-600">
              Tahap awal cukup isi jumlah siswa akhir per tahun. Detail siswa
              diisi saat pengumuman/status akhir.
            </p>
          </div>
          <div className="flex gap-2">
            {isMaster ? (
              <button
                type="button"
                onClick={() => router.push("/user/settings/siswa-akhir")}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 hover:bg-slate-50"
              >
                <Settings className="h-4 w-4" /> Settings
              </button>
            ) : null}
            <button
              type="button"
              onClick={() => router.push("/siswa/kelas-akhir/histori")}
              className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 hover:bg-slate-50"
            >
              Histori Import
            </button>
          </div>
        </header>

        <Card>
          <CardContent className="space-y-4">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
              <div className="space-y-1">
                <h2 className="text-lg font-semibold text-zinc-900">
                  Rekap Siswa Akhir
                </h2>
                <p className="text-sm text-zinc-600">
                  Data awal diambil otomatis dari input lama. Silakan koreksi
                  jika terdapat perbedaan dengan data riil sekolah.
                </p>
              </div>
              <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
                <label className="space-y-1 text-sm font-medium text-zinc-700">
                  <span>Tahun Lulus</span>
                  <input
                    type="number"
                    value={summaryYear}
                    onChange={(event) => {
                      setSummaryYear(Number(event.target.value) || currentYear);
                      setSummaryMessage("");
                    }}
                    className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm sm:w-36"
                  />
                </label>
                {isSchoolAdmin ? (
                  <>
                    <label className="space-y-1 text-sm font-medium text-zinc-700">
                      <span>Jumlah Siswa Akhir</span>
                      <input
                        type="number"
                        min={0}
                        value={summaryTotal}
                        onChange={(event) => {
                          setSummaryTotal(event.target.value);
                          setSummaryMessage("");
                        }}
                        className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm sm:w-44"
                      />
                    </label>
                    <button
                      type="button"
                      onClick={() => summaryMutation.mutate()}
                      disabled={summaryMutation.isPending}
                      className="h-10 rounded-md bg-zinc-900 px-4 text-sm font-semibold text-white disabled:opacity-60"
                    >
                      {summaryMutation.isPending ? "Menyimpan..." : "Simpan"}
                    </button>
                  </>
                ) : null}
              </div>
            </div>
            {summaryMutation.error ? (
              <p className="text-sm text-red-600">
                {summaryMutation.error.message}
              </p>
            ) : null}
            {summaryMessage ? (
              <p className="text-sm text-emerald-700">{summaryMessage}</p>
            ) : null}
            <DataTable<YearSummaryRow>
              columns={[
                ...(showSchoolColumn
                  ? [{ key: "school", label: "Sekolah" }]
                  : []),
                { key: "tahun", label: "Tahun Lulus", align: "right" as const },
                {
                  key: "total",
                  label: "Jumlah Siswa Akhir",
                  align: "right" as const,
                },
                { key: "source", label: "Sumber Data" },
              ]}
              rows={summaryQuery.data ?? []}
              isLoading={summaryQuery.isLoading}
              isError={summaryQuery.isError}
              emptyMessage="Belum ada rekap siswa akhir."
              page={1}
              pageSize={summaryQuery.data?.length || 1}
              total={summaryQuery.data?.length ?? 0}
              onPageChange={() => undefined}
              renderRow={(row) => (
                <TableRow key={`${row.schoolId}-${row.tahunLulus}`}>
                  {showSchoolColumn ? (
                    <TableCell>{row.schoolName || "-"}</TableCell>
                  ) : null}
                  <TableCell className="text-right">
                    {row.tahunLulus || "-"}
                  </TableCell>
                  <TableCell className="text-right">
                    {row.totalSiswaAkhir.toLocaleString("id-ID")}
                  </TableCell>
                  <TableCell>
                    {row.sourceLabel || row.source || "Manual"}
                  </TableCell>
                </TableRow>
              )}
            />
          </CardContent>
        </Card>

        <div className="rounded-md border border-zinc-200 bg-zinc-50 p-3 text-sm text-zinc-600">
          Alur detail siswa lama, tab pengajuan, nonaktif, koreksi, dan import
          rinci tidak ditampilkan pada alur utama operator sekolah.
        </div>

        {requestTarget ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
            <div className="w-full max-w-xl rounded-xl bg-white p-5 shadow-xl">
              <h2 className="text-lg font-semibold">
                {requestType === "NONAKTIF"
                  ? "Ajukan Nonaktif"
                  : "Ajukan Koreksi Data"}
              </h2>
              <p className="mt-1 text-sm text-zinc-600">
                {requestTarget.namaSiswa} / {requestTarget.nisn} / tahun{" "}
                {requestTarget.tahunLulus}
              </p>
              <div className="mt-4 space-y-3">
                <select
                  value={reason}
                  onChange={(event) => setReason(event.target.value)}
                  className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm"
                >
                  <option value="">
                    {requestType === "NONAKTIF"
                      ? "Pilih alasan nonaktif"
                      : "Pilih jenis koreksi"}
                  </option>
                  {(requestType === "NONAKTIF"
                    ? nonactiveReasons
                    : correctionReasons
                  ).map((item) => (
                    <option key={item}>{item}</option>
                  ))}
                </select>
                <textarea
                  value={schoolNote}
                  onChange={(event) => setSchoolNote(event.target.value)}
                  placeholder={
                    requestType === "NONAKTIF"
                      ? "Catatan sekolah"
                      : "Catatan koreksi"
                  }
                  className="min-h-24 w-full rounded-md border border-zinc-200 px-3 py-2 text-sm"
                />
                <input
                  value={supportLink}
                  onChange={(event) => setSupportLink(event.target.value)}
                  placeholder={
                    requestType === "NONAKTIF"
                      ? "Link surat pernyataan Google Drive"
                      : "Link pendukung opsional"
                  }
                  className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm"
                />
                {submitMutation.error ? (
                  <p className="text-sm text-red-600">
                    {submitMutation.error.message}
                  </p>
                ) : null}
              </div>
              <div className="mt-5 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setRequestTarget(null)}
                  className="rounded-md border border-zinc-200 px-4 py-2 text-sm font-semibold"
                >
                  Batal
                </button>
                <button
                  type="button"
                  onClick={() => submitMutation.mutate()}
                  disabled={submitMutation.isPending}
                  className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                >
                  {submitMutation.isPending
                    ? "Menyimpan..."
                    : "Submit Pengajuan"}
                </button>
              </div>
            </div>
          </div>
        ) : null}

        {processTarget ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
            <div className="w-full max-w-lg rounded-xl bg-white p-5 shadow-xl">
              <h2 className="text-lg font-semibold">
                {processAction === "APPROVE"
                  ? "Setujui Pengajuan"
                  : "Tolak Pengajuan"}
              </h2>
              <p className="mt-1 text-sm text-zinc-600">
                {processTarget.namaSiswa} / {processTarget.reason}
              </p>
              <textarea
                value={processorNote}
                onChange={(event) => setProcessorNote(event.target.value)}
                placeholder="Catatan Cabdin/Master"
                className="mt-4 min-h-24 w-full rounded-md border border-zinc-200 px-3 py-2 text-sm"
              />
              {processMutation.error ? (
                <p className="mt-2 text-sm text-red-600">
                  {processMutation.error.message}
                </p>
              ) : null}
              <div className="mt-5 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setProcessTarget(null)}
                  className="rounded-md border border-zinc-200 px-4 py-2 text-sm font-semibold"
                >
                  Batal
                </button>
                <button
                  type="button"
                  onClick={() => processMutation.mutate()}
                  disabled={processMutation.isPending}
                  className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                >
                  {processMutation.isPending ? "Memproses..." : "Simpan"}
                </button>
              </div>
            </div>
          </div>
        ) : null}
      </section>
    </AdminRouteGuard>
  );
}
