"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowLeft, Download, RefreshCcw, Search, X } from "lucide-react";
import { useRouter } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
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

type Tab = "events" | "submissions" | "assessments" | "validations" | "jury";
type Option = { id: number; name: string };
type Row = {
  id: number;
  year: number;
  month: number;
  stage: string;
  schoolId: number;
  schoolName: string;
  npsn: string;
  schoolLevel: string;
  reportTitle: string;
  reportLink: string;
  status: string;
  submittedByName: string;
  supervisorName: string;
  validatorName: string;
  actorName: string;
  eventAt?: string;
  supervisorScore?: number | null;
  validatorScore?: number | null;
  finalScore?: number | null;
  score?: number | null;
  notes: string;
  validatorNotes: string;
  validationDecision: string;
  action: string;
  actorRole: string;
  beforeStatus: string;
  afterStatus: string;
  aggregateScore?: number | null;
  reference: string;
};
type Summary = {
  submitted: number;
  assessed: number;
  validated: number;
  pendingValidation: number;
  juryAssessed: number;
};
type Payload = {
  items: Row[];
  summary: Summary;
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
  supervisors: Option[];
  validators: Option[];
};
type Audit = {
  id: number;
  actorName: string;
  actorRole: string;
  action: string;
  payload: string;
  createdAt?: string;
};
type Detail = { row: Row; timeline: Audit[] };

const tabs: { id: Tab; label: string }[] = [
  { id: "events", label: "Aktivitas Tahapan" },
  { id: "submissions", label: "Pengumpulan Sekolah" },
  { id: "assessments", label: "Penilaian Pengawas" },
  { id: "validations", label: "Validasi" },
  { id: "jury", label: "Penilaian Juri" },
];
const stages = [
  ["MONTHLY", "Bulanan"],
  ["FIELD_STAGE_1", "Lapangan Tahap 1"],
  ["FIELD_STAGE_2", "Lapangan Tahap 2"],
  ["PRESENTATION", "Presentasi"],
  ["NOT_FAILED_NOMINATION", "Nominasi Tidak Tergagal"],
  ["BOTTOM5_APPEAL", "Banding Kandidat Zona Tergagal"],
  ["INNOVATION", "Ter-Inovatif"],
  ["TOP_10_LOCK", "Penetapan Top 10"],
  ["TOP_5_LOCK", "Penetapan Top 5"],
  ["BOTTOM_5_LOCK", "Penetapan Bottom 5"],
  ["FINAL_RESULT", "Hasil Final"],
  ["AWARDING", "Awarding"],
  ["FAVORITE_VOTE", "Vote Ter-Favorit"],
  ["LIKE_SYNC", "Sinkronisasi/Override Like"],
] as const;
const months = [4, 5, 6, 7, 8, 9, 10, 11];
const monthLabel = (month: number) =>
  new Intl.DateTimeFormat("id-ID", { month: "long" }).format(
    new Date(2026, month - 1, 1)
  );
const dateWIB = (value?: string) =>
  value
    ? new Intl.DateTimeFormat("id-ID", {
        dateStyle: "medium",
        timeStyle: "medium",
        timeZone: "Asia/Jakarta",
      }).format(new Date(value)) + " WIB"
    : "-";
const score = (value?: number | null) =>
  typeof value === "number" ? value.toFixed(2) : "-";
const decision = (value: string) =>
  value === "OBJECT" ? "Dikoreksi" : value === "AGREE" ? "Setuju" : "-";
const stageLabel = (stage: string, month: number) =>
  stages.find(([value]) => value === stage)?.[1] ??
  (stage === "FIELD" ? "Lapangan Tahap 1" : stage || monthLabel(month));
const inputClass =
  "h-10 w-full rounded-xl border border-slate-200 bg-white px-3 text-sm text-slate-800 outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100";
const buttonClass =
  "inline-flex h-10 items-center justify-center rounded-xl px-4 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-60";

export function GaspolHistoryPage() {
  const router = useRouter();
  const { user, status: authStatus } = useAuth();
  const [tab, setTab] = useState<Tab>("events");
  const [year, setYear] = useState(new Date().getFullYear());
  const [month, setMonth] = useState(0);
  const [stage, setStage] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [filterStatus, setFilterStatus] = useState("");
  const [supervisorId, setSupervisorId] = useState(0);
  const [validatorId, setValidatorId] = useState(0);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [payload, setPayload] = useState<Payload | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [detail, setDetail] = useState<Detail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const allowed =
    user?.role === "ADMIN" ||
    user?.role === "ADMIN_CABDIN" ||
    user?.role === "ADMIN_MASTER";
  useEffect(() => {
    if (authStatus === "authenticated" && !allowed) router.replace("/gaspol");
  }, [allowed, authStatus, router]);

  const params = useMemo(() => {
    const q = new URLSearchParams({
      tab,
      year: String(year),
      page: String(page),
      pageSize: "20",
    });
    if (month) q.set("month", String(month));
    if (stage) q.set("stage", stage);
    if (dateFrom) q.set("dateFrom", dateFrom);
    if (dateTo) q.set("dateTo", dateTo);
    if (filterStatus) q.set("status", filterStatus);
    if (supervisorId) q.set("supervisorId", String(supervisorId));
    if (validatorId) q.set("validatorId", String(validatorId));
    if (search.trim()) q.set("search", search.trim());
    return q;
  }, [
    tab,
    year,
    month,
    stage,
    dateFrom,
    dateTo,
    filterStatus,
    supervisorId,
    validatorId,
    search,
    page,
  ]);
  const load = useCallback(async () => {
    if (!allowed) return;
    setLoading(true);
    setError("");
    try {
      const response = await apiFetch(`/gaspol/history?${params}`);
      const body = (await response.json().catch(() => null)) as {
        data?: Payload;
        message?: string;
      } | null;
      if (!response.ok || !body?.data)
        throw new Error(body?.message || "Histori GASPOL gagal dimuat.");
      setPayload(body.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Histori GASPOL gagal dimuat.");
    } finally {
      setLoading(false);
    }
  }, [allowed, params]);
  useEffect(() => {
    void load();
  }, [load]);
  const changeFilter = (fn: () => void) => {
    setPage(1);
    fn();
  };
  const reset = () => {
    setPage(1);
    setMonth(0);
    setStage("");
    setDateFrom("");
    setDateTo("");
    setFilterStatus("");
    setSupervisorId(0);
    setValidatorId(0);
    setSearch("");
  };
  const openDetail = async (row: Row) => {
    setDetailLoading(true);
    setError("");
    try {
      const response = await apiFetch(`/gaspol/history/${row.id}?tab=${tab}`);
      const body = (await response.json().catch(() => null)) as {
        data?: Detail;
        message?: string;
      } | null;
      if (!response.ok || !body?.data)
        throw new Error(body?.message || "Detail gagal dimuat.");
      setDetail(body.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Detail gagal dimuat.");
    } finally {
      setDetailLoading(false);
    }
  };
  const download = async () => {
    setExporting(true);
    setError("");
    try {
      const q = new URLSearchParams(params);
      q.delete("page");
      q.delete("pageSize");
      const response = await apiFetch(`/gaspol/history/export.xlsx?${q}`);
      if (!response.ok) {
        const body = (await response.json().catch(() => null)) as {
          message?: string;
        } | null;
        throw new Error(body?.message || "Unduh Excel gagal.");
      }
      const blob = await response.blob();
      const disposition = response.headers.get("content-disposition") || "";
      const name =
        disposition.match(/filename="?([^";]+)"?/)?.[1] ||
        "Histori-GASPOL.xlsx";
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = name;
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unduh Excel gagal.");
    } finally {
      setExporting(false);
    }
  };
  const statuses =
    tab === "events"
      ? [
          { v: "ASSESSMENT_SUBMITTED", l: "Assessor mengirim" },
          { v: "KOOR_APPROVE", l: "Koor menyetujui" },
          { v: "KOOR_RETURN", l: "Koor mengembalikan" },
          { v: "CABDIN_APPROVE", l: "Cabdin menyetujui" },
          { v: "CABDIN_RETURN", l: "Cabdin mengembalikan" },
          { v: "DIRECT_STAGE_FINALIZED", l: "Aggregate direct-final" },
          { v: "TOP_10_LOCKED", l: "Top 10 dikunci" },
        ]
      : tab === "submissions"
        ? [
            { v: "SUBMITTED", l: "Terkirim" },
            { v: "DRAFT", l: "Draft" },
          ]
        : tab === "validations"
          ? [
              { v: "AGREE", l: "Setuju" },
              { v: "OBJECT", l: "Dikoreksi" },
            ]
          : [
              { v: "PENDING", l: "Menunggu" },
              { v: "VALIDATED", l: "Tervalidasi" },
            ];
  if (authStatus === "loading" || !allowed)
    return (
      <div className="p-6">
        <Skeleton className="h-40 w-full" />
      </div>
    );
  const summary = payload?.summary ?? {
    submitted: 0,
    assessed: 0,
    validated: 0,
    pendingValidation: 0,
    juryAssessed: 0,
  };
  const cards =
    tab === "events"
      ? [
          ["Aktivitas", payload?.total ?? 0],
          ["Submit assessor", summary.assessed],
          ["Keputusan validator", summary.validated],
          ["Pending/returned", summary.pendingValidation],
        ]
      : tab === "jury"
        ? [["Penilaian juri", summary.juryAssessed]]
        : [
            ["Laporan dikumpulkan", summary.submitted],
            ["Sudah dinilai", summary.assessed],
            ["Sudah divalidasi", summary.validated],
            ["Menunggu validasi", summary.pendingValidation],
          ];
  return (
    <div className="min-h-full space-y-5 px-4 py-5 md:px-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <button
            onClick={() => router.push("/gaspol")}
            className="mb-2 inline-flex items-center text-sm font-semibold text-emerald-700"
          >
            <ArrowLeft className="mr-2 h-4 w-4" />
            Kembali ke GASPOL
          </button>
          <h1 className="text-2xl font-bold text-slate-950">
            Histori GASPOL Bang
          </h1>
          <p className="text-sm text-slate-500">
            Bukti aktivitas read-only, ditampilkan dalam zona waktu WIB.
          </p>
        </div>
        <button
          onClick={() => void load()}
          className={`${buttonClass} border border-slate-200 bg-white text-slate-700`}
        >
          <RefreshCcw
            className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`}
          />
          Refresh
        </button>
      </header>
      <div className="flex gap-2 overflow-x-auto pb-1">
        {tabs.map((x) => (
          <button
            key={x.id}
            onClick={() => changeFilter(() => setTab(x.id))}
            className={`${buttonClass} whitespace-nowrap ${tab === x.id ? "bg-emerald-600 text-white" : "border border-slate-200 bg-white text-slate-700"}`}
          >
            {x.label}
          </button>
        ))}
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Filter global</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5">
          <select
            className={inputClass}
            value={year}
            onChange={(e) =>
              changeFilter(() => setYear(Number(e.target.value)))
            }
          >
            {Array.from(
              {
                length: Math.max(new Date().getFullYear() - 2024 + 2, 2),
              },
              (_, index) => 2024 + index
            ).map((x) => (
              <option key={x}>{x}</option>
            ))}
          </select>
          <select
            className={inputClass}
            value={month}
            onChange={(e) =>
              changeFilter(() => setMonth(Number(e.target.value)))
            }
          >
            <option value={0}>Semua bulan/tahap</option>
            {months.map((x) => (
              <option key={x} value={x}>
                {monthLabel(x)}
              </option>
            ))}
          </select>
          {tab === "events" ? (
            <select
              className={inputClass}
              value={stage}
              onChange={(e) => changeFilter(() => setStage(e.target.value))}
            >
              <option value="">Semua tahapan</option>
              {stages.map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          ) : null}
          <label className="text-xs font-semibold text-slate-600">
            Dari Tanggal
            <input
              aria-label="Dari tanggal"
              className={`mt-1 ${inputClass}`}
              type="date"
              value={dateFrom}
              onChange={(e) => changeFilter(() => setDateFrom(e.target.value))}
            />
          </label>
          <label className="text-xs font-semibold text-slate-600">
            Sampai Tanggal
            <input
              aria-label="Sampai tanggal"
              className={`mt-1 ${inputClass}`}
              type="date"
              value={dateTo}
              onChange={(e) => changeFilter(() => setDateTo(e.target.value))}
            />
          </label>
          <select
            className={inputClass}
            value={filterStatus}
            onChange={(e) =>
              changeFilter(() => setFilterStatus(e.target.value))
            }
          >
            <option value="">Semua status</option>
            {statuses.map((x) => (
              <option key={x.v} value={x.v}>
                {x.l}
              </option>
            ))}
          </select>
          {tab !== "jury" && tab !== "events" && (
            <select
              className={inputClass}
              value={supervisorId}
              onChange={(e) =>
                changeFilter(() => setSupervisorId(Number(e.target.value)))
              }
            >
              <option value={0}>Semua pengawas</option>
              {payload?.supervisors.map((x) => (
                <option key={x.id} value={x.id}>
                  {x.name}
                </option>
              ))}
            </select>
          )}
          {tab !== "jury" && tab !== "events" && (
            <select
              className={inputClass}
              value={validatorId}
              onChange={(e) =>
                changeFilter(() => setValidatorId(Number(e.target.value)))
              }
            >
              <option value={0}>Semua validator</option>
              {payload?.validators.map((x) => (
                <option key={x.id} value={x.id}>
                  {x.name}
                </option>
              ))}
            </select>
          )}
          <label className="relative sm:col-span-2">
            <Search className="absolute top-3 left-3 h-4 w-4 text-slate-400" />
            <input
              className={`${inputClass} pl-9`}
              value={search}
              onChange={(e) => changeFilter(() => setSearch(e.target.value))}
              placeholder="Cari sekolah atau NPSN"
            />
          </label>
          <button
            onClick={reset}
            className={`${buttonClass} border border-slate-200 bg-white text-slate-700`}
          >
            <X className="mr-2 h-4 w-4" />
            Reset
          </button>
          <button
            disabled={exporting}
            onClick={() => void download()}
            className={`${buttonClass} bg-emerald-600 text-white`}
          >
            <Download className="mr-2 h-4 w-4" />
            {exporting ? "Menyiapkan..." : "Unduh Excel"}
          </button>
        </CardContent>
      </Card>
      {error && (
        <div className="flex items-center justify-between rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
          <span>{error}</span>
          <button className="font-semibold" onClick={() => void load()}>
            Coba lagi
          </button>
        </div>
      )}
      <div
        className={`grid gap-3 ${tab === "jury" ? "sm:grid-cols-1" : "sm:grid-cols-2 xl:grid-cols-4"}`}
      >
        {cards.map(([label, value]) => (
          <Card key={String(label)}>
            <CardContent className="p-4">
              <p className="text-sm text-slate-500">{label}</p>
              <p className="mt-1 text-2xl font-bold text-slate-950">{value}</p>
            </CardContent>
          </Card>
        ))}
      </div>
      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="space-y-3 p-5">
              {[1, 2, 3, 4].map((x) => (
                <Skeleton key={x} className="h-12 w-full" />
              ))}
            </div>
          ) : !payload?.items.length ? (
            <div className="p-10 text-center">
              <p className="font-semibold text-slate-800">Belum ada histori</p>
              <p className="text-sm text-slate-500">
                Tidak ada data yang sesuai dengan tab dan filter aktif.
              </p>
            </div>
          ) : (
            <Table className="min-w-[1050px]">
              <TableHeader>
                <TableRow>
                  <TableHead>Bulan/Tahap</TableHead>
                  <TableHead>Sekolah</TableHead>
                  <TableHead>NPSN</TableHead>
                  {tab === "submissions" && (
                    <>
                      <TableHead>Judul</TableHead>
                      <TableHead>Laporan</TableHead>
                      <TableHead>Dikumpulkan oleh</TableHead>
                    </>
                  )}
                  {tab !== "submissions" && (
                    <TableHead>
                      {tab === "events" ? "Actor" : "Pengawas/Juri"}
                    </TableHead>
                  )}
                  {tab === "events" && (
                    <>
                      <TableHead>Role actor</TableHead>
                      <TableHead>Action</TableHead>
                    </>
                  )}
                  {tab === "validations" && <TableHead>Validator</TableHead>}
                  <TableHead>Tanggal WIB</TableHead>
                  {tab === "assessments" && (
                    <>
                      <TableHead>Nilai</TableHead>
                      <TableHead>Catatan</TableHead>
                    </>
                  )}
                  {tab === "validations" && (
                    <>
                      <TableHead>Nilai pengawas</TableHead>
                      <TableHead>Keputusan</TableHead>
                      <TableHead>Koreksi</TableHead>
                      <TableHead>Final</TableHead>
                      <TableHead>Catatan</TableHead>
                    </>
                  )}
                  {tab === "jury" && (
                    <>
                      <TableHead>Nilai</TableHead>
                      <TableHead>Catatan</TableHead>
                    </>
                  )}
                  {tab === "events" && (
                    <>
                      <TableHead>Nilai raw</TableHead>
                      <TableHead>Aggregate/final</TableHead>
                      <TableHead>Sebelum → Sesudah</TableHead>
                      <TableHead>Catatan</TableHead>
                      <TableHead>Referensi</TableHead>
                    </>
                  )}
                  <TableHead>Status</TableHead>
                  <TableHead>Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {payload.items.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell>{stageLabel(row.stage, row.month)}</TableCell>
                    <TableCell className="font-medium">
                      {row.schoolName}
                    </TableCell>
                    <TableCell>{row.npsn}</TableCell>
                    {tab === "submissions" && (
                      <>
                        <TableCell>{row.reportTitle || "-"}</TableCell>
                        <TableCell>
                          {row.reportLink ? (
                            <a
                              className="font-semibold text-emerald-700"
                              target="_blank"
                              rel="noreferrer"
                              href={row.reportLink}
                            >
                              Buka
                            </a>
                          ) : (
                            "-"
                          )}
                        </TableCell>
                        <TableCell>{row.submittedByName || "-"}</TableCell>
                      </>
                    )}
                    {tab !== "submissions" && (
                      <TableCell>
                        {row.actorName || row.supervisorName || "-"}
                      </TableCell>
                    )}
                    {tab === "events" && (
                      <>
                        <TableCell>{row.actorRole || "-"}</TableCell>
                        <TableCell className="font-medium">
                          {(row.action || "-").replaceAll("_", " ")}
                        </TableCell>
                      </>
                    )}
                    {tab === "validations" && (
                      <TableCell>{row.validatorName || "-"}</TableCell>
                    )}
                    <TableCell>{dateWIB(row.eventAt)}</TableCell>
                    {tab === "assessments" && (
                      <>
                        <TableCell>{score(row.supervisorScore)}</TableCell>
                        <TableCell className="max-w-64 whitespace-normal">
                          {row.notes || "-"}
                        </TableCell>
                      </>
                    )}
                    {tab === "validations" && (
                      <>
                        <TableCell>{score(row.supervisorScore)}</TableCell>
                        <TableCell>
                          {decision(row.validationDecision)}
                        </TableCell>
                        <TableCell>{score(row.validatorScore)}</TableCell>
                        <TableCell>{score(row.finalScore)}</TableCell>
                        <TableCell className="max-w-64 whitespace-normal">
                          {row.validatorNotes || "-"}
                        </TableCell>
                      </>
                    )}
                    {tab === "jury" && (
                      <>
                        <TableCell>{score(row.score)}</TableCell>
                        <TableCell className="max-w-64 whitespace-normal">
                          {row.notes || "-"}
                        </TableCell>
                      </>
                    )}
                    {tab === "events" && (
                      <>
                        <TableCell>{score(row.score)}</TableCell>
                        <TableCell>
                          {score(row.aggregateScore ?? row.finalScore)}
                        </TableCell>
                        <TableCell>
                          {row.beforeStatus || "-"} →{" "}
                          {row.afterStatus || row.status || "-"}
                        </TableCell>
                        <TableCell className="max-w-64 whitespace-normal">
                          {row.notes || "-"}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {row.reference || "-"}
                        </TableCell>
                      </>
                    )}
                    <TableCell>
                      <StatusBadge
                        variant={
                          row.status === "VALIDATED" ||
                          row.status === "SUBMITTED"
                            ? "success"
                            : row.status === "DRAFT"
                              ? "neutral"
                              : "warning"
                        }
                      >
                        {row.status}
                      </StatusBadge>
                    </TableCell>
                    <TableCell>
                      <button
                        disabled={detailLoading}
                        onClick={() => void openDetail(row)}
                        className="font-semibold text-emerald-700"
                      >
                        Detail
                      </button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-slate-600">
        <span>
          {payload?.total ?? 0} data • Halaman {payload?.page ?? 1} dari{" "}
          {Math.max(payload?.totalPages ?? 0, 1)}
        </span>
        <div className="flex gap-2">
          <button
            className={`${buttonClass} border border-slate-200 bg-white`}
            disabled={page <= 1}
            onClick={() => setPage((x) => x - 1)}
          >
            Sebelumnya
          </button>
          <button
            className={`${buttonClass} border border-slate-200 bg-white`}
            disabled={!payload || page >= payload.totalPages}
            onClick={() => setPage((x) => x + 1)}
          >
            Berikutnya
          </button>
        </div>
      </div>
      {detail && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 p-4">
          <button
            aria-label="Tutup"
            className="absolute inset-0"
            onClick={() => setDetail(null)}
          />
          <section
            role="dialog"
            aria-modal="true"
            className="relative max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-3xl bg-white p-5 shadow-2xl"
          >
            <div className="flex justify-between gap-4">
              <div>
                <p className="text-xs font-semibold tracking-wider text-emerald-700 uppercase">
                  Detail aktivitas
                </p>
                <h2 className="text-xl font-bold">{detail.row.schoolName}</h2>
                <p className="text-sm text-slate-500">
                  {detail.row.month ? `${monthLabel(detail.row.month)} ` : ""}
                  {detail.row.year} •{" "}
                  {stageLabel(detail.row.stage, detail.row.month)}
                  {detail.row.npsn ? ` • NPSN ${detail.row.npsn}` : ""}
                </p>
                {detail.row.reference ? (
                  <p className="mt-1 font-mono text-xs text-slate-500">
                    {detail.row.reference.replace(
                      "stage_assessment#",
                      "Assessment #"
                    )}
                  </p>
                ) : null}
              </div>
              <button onClick={() => setDetail(null)}>
                <X />
              </button>
            </div>
            <div className="mt-6 space-y-4">
              {detail.row.reference.startsWith("stage_assessment#") ? (
                <div className="grid gap-2 rounded-2xl border border-emerald-100 bg-emerald-50/60 p-4 text-sm sm:grid-cols-2">
                  <p>
                    <span className="text-slate-500">Nilai:</span>{" "}
                    <span className="font-semibold">
                      {score(detail.row.score ?? detail.row.supervisorScore)}
                    </span>
                  </p>
                  <p>
                    <span className="text-slate-500">Status saat ini:</span>{" "}
                    <span className="font-semibold">{detail.row.status}</span>
                  </p>
                </div>
              ) : null}
              <div className="border-l-2 border-emerald-300 pl-4">
                <p className="font-semibold">
                  {detail.row.reference.startsWith("stage_assessment#")
                    ? `Timeline Assessment #${detail.row.reference.split("#")[1]}`
                    : "Data resmi pada tab ini"}
                </p>
                <p className="text-sm text-slate-500">
                  {dateWIB(detail.row.eventAt)}
                </p>
              </div>
              {detail.timeline.length ? (
                detail.timeline.map((item) => (
                  <div
                    key={item.id}
                    className="border-l-2 border-slate-200 pl-4"
                  >
                    <p className="font-semibold">
                      {item.action.replaceAll("_", " ")}
                    </p>
                    <p className="text-sm text-slate-600">
                      {item.actorName || "Pelaku tidak tersedia"} •{" "}
                      {item.actorRole || "Role tidak tersedia"}
                    </p>
                    <p className="text-xs text-slate-500">
                      {dateWIB(item.createdAt)}
                    </p>
                    {item.payload && item.payload !== "{}" ? (
                      <pre className="mt-2 overflow-x-auto rounded-lg bg-slate-50 p-2 text-xs whitespace-pre-wrap text-slate-600">
                        {(() => {
                          try {
                            return JSON.stringify(
                              JSON.parse(item.payload),
                              null,
                              2
                            );
                          } catch {
                            return item.payload;
                          }
                        })()}
                      </pre>
                    ) : null}
                  </div>
                ))
              ) : (
                <p className="rounded-xl bg-slate-50 p-4 text-sm text-slate-500">
                  Audit lama untuk aktivitas ini tidak tersedia. Data resmi di
                  atas berasal dari record utama.
                </p>
              )}
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
