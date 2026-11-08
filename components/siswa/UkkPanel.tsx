"use client";

import { useEffect, useMemo, useState, type ChangeEvent } from "react";
import { useQuery } from "@tanstack/react-query";

import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

type UkkAssessor = {
  nama: string;
  instansi: string;
};

type UkkRow = {
  nisn: string;
  nama: string;
  rombel: string;
  tahunLulus: number;
  skemaUkk: string;
  nilaiAkhir: number | null;
  predikat: string;
  asesor: UkkAssessor[];
  isComplete: boolean;
  finalStatus: string;
  finalizedAt: string | null;
};

type UkkListResponse = {
  schoolId?: number | null;
  totalStudents: number;
  completeCount: number;
  missingCount: number;
  yearClosed: boolean;
  emptyStateMessage?: string;
  rows: UkkRow[];
};

type UkkDraft = {
  skemaUkk: string;
  nilaiAkhir: string;
  predikat: string;
  asesor1Nama: string;
  asesor1Instansi: string;
  asesor2Nama: string;
  asesor2Instansi: string;
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

const readBoolean = (value: unknown) => {
  if (typeof value === "boolean") return value;
  if (typeof value === "number") return value !== 0;
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return normalized === "true" || normalized === "1" || normalized === "yes";
  }
  return false;
};

const emptyUkkDraft = (): UkkDraft => ({
  skemaUkk: "",
  nilaiAkhir: "",
  predikat: "",
  asesor1Nama: "",
  asesor1Instansi: "",
  asesor2Nama: "",
  asesor2Instansi: "",
});

const buildErrorMessage = (payload: unknown) => {
  if (!isRecord(payload)) {
    return "Upload gagal. Periksa koneksi atau format file.";
  }
  if (typeof payload.message === "string" && payload.message.trim()) {
    return payload.message;
  }
  if (typeof payload.error === "string" && payload.error.trim()) {
    if (payload.error === "TEMPLATE_MISMATCH") {
      return "Header template tidak sesuai. Silakan unduh template terbaru.";
    }
    return payload.error;
  }
  const details = isRecord(payload.details) ? payload.details : null;
  const missing = details?.missing_columns;
  if (Array.isArray(missing) && missing.length > 0) {
    return `Kolom wajib belum lengkap: ${missing.join(", ")}`;
  }
  return "Upload gagal. Periksa koneksi atau format file.";
};

const normalizeUkkRows = (rows: unknown[]): UkkRow[] => {
  return rows
    .map((row) => {
      if (!isRecord(row)) return null;
      const asesorRaw = Array.isArray(row.asesor) ? row.asesor : [];
      const asesor = asesorRaw
        .map((item) => {
          if (!isRecord(item)) return null;
          return {
            nama: readString(item.nama),
            instansi: readString(item.instansi),
          };
        })
        .filter((item): item is UkkAssessor => Boolean(item));

      const skemaUkk = readString(row.skema_ukk ?? row.skemaUkk);
      const nilaiRaw = row.nilai_akhir ?? row.nilaiAkhir;
      let nilaiAkhir: number | null = null;
      if (typeof nilaiRaw === "number" && Number.isFinite(nilaiRaw)) {
        nilaiAkhir = nilaiRaw;
      } else if (typeof nilaiRaw === "string") {
        const trimmed = nilaiRaw.trim();
        if (trimmed) {
          const parsed = Number(trimmed);
          if (Number.isFinite(parsed)) {
            nilaiAkhir = parsed;
          }
        }
      }
      const predikat = readString(row.predikat);
      const finalStatus = readString(row.final_status ?? row.finalStatus);
      const finalizedAt =
        readString(row.finalized_at ?? row.finalizedAt) || null;
      const isComplete =
        skemaUkk.trim() !== "" &&
        nilaiAkhir !== null &&
        predikat.trim() !== "" &&
        asesor.some(
          (item) => item.nama.trim() !== "" && item.instansi.trim() !== ""
        );

      return {
        nisn: readString(row.nisn),
        nama: readString(row.nama),
        rombel: readString(row.rombel),
        tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
        skemaUkk,
        nilaiAkhir,
        predikat,
        asesor,
        isComplete,
        finalStatus,
        finalizedAt,
      };
    })
    .filter((row): row is UkkRow => row !== null);
};

type UkkPanelProps = {
  schoolId?: number | null;
  hasSMKScope?: boolean;
  emptyStateMessage?: string;
};

const fetchUkkList = async (params: {
  year: string;
  schoolId?: number | null;
}): Promise<UkkListResponse> => {
  const query = new URLSearchParams();
  query.set("tahun_lulus", params.year);
  if (params.schoolId && params.schoolId > 0) {
    query.set("school_id", String(params.schoolId));
  }
  const response = await apiFetch(`/api/siswa/ukk/list?${query.toString()}`);
  if (!response.ok) {
    throw new Error("Failed to fetch UKK list.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return {
    schoolId: readNumber(data?.school_id ?? data?.schoolId) || null,
    totalStudents: readNumber(data?.total_students ?? data?.totalStudents),
    completeCount: readNumber(data?.complete_count ?? data?.completeCount),
    missingCount: readNumber(data?.missing_count ?? data?.missingCount),
    yearClosed: readBoolean(data?.year_closed ?? data?.yearClosed),
    emptyStateMessage:
      readString(data?.empty_state_message ?? data?.emptyStateMessage) || "",
    rows: normalizeUkkRows(Array.isArray(data?.rows) ? data.rows : []),
  };
};

export default function UkkPanel({
  schoolId = null,
  hasSMKScope = true,
  emptyStateMessage = "",
}: UkkPanelProps) {
  const { status, user } = useAuth();
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const currentYear = new Date().getFullYear();
  const yearOptions = useMemo(() => {
    const years: number[] = [];
    for (let year = currentYear - 2; year <= currentYear; year += 1) {
      years.push(year);
    }
    return years;
  }, [currentYear]);
  const [year, setYear] = useState(String(currentYear));
  const [filter, setFilter] = useState<"all" | "missing" | "complete">("all");
  const [page, setPage] = useState(1);
  const pageSize = 20;
  const [isImportOpen, setIsImportOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [message, setMessage] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [editTarget, setEditTarget] = useState<UkkRow | null>(null);
  const [editDraft, setEditDraft] = useState<UkkDraft>(emptyUkkDraft());
  const [editMessage, setEditMessage] = useState("");
  const [editError, setEditError] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    setPage(1);
  }, [filter, year, schoolId]);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["ukk-list", year, schoolId ?? "default"],
    queryFn: () =>
      fetchUkkList({
        year,
        schoolId,
      }),
    enabled: status === "authenticated" && hasSMKScope,
  });

  const rows = useMemo(() => data?.rows ?? [], [data?.rows]);
  const yearClosed = data?.yearClosed ?? false;
  const backendEmptyStateMessage = data?.emptyStateMessage?.trim() ?? "";
  const effectiveEmptyMessage =
    (!hasSMKScope
      ? emptyStateMessage
      : backendEmptyStateMessage || emptyStateMessage) || "Belum ada data UKK.";
  const resetDraft = (next: UkkDraft) => setEditDraft(next);
  const openEdit = (row: UkkRow) => {
    const asesor1 = row.asesor[0];
    const asesor2 = row.asesor[1];
    setEditTarget(row);
    setEditMessage("");
    setEditError("");
    resetDraft({
      skemaUkk: row.skemaUkk ?? "",
      nilaiAkhir: row.nilaiAkhir != null ? String(row.nilaiAkhir) : "",
      predikat: row.predikat ?? "",
      asesor1Nama: asesor1?.nama ?? "",
      asesor1Instansi: asesor1?.instansi ?? "",
      asesor2Nama: asesor2?.nama ?? "",
      asesor2Instansi: asesor2?.instansi ?? "",
    });
  };
  const closeEdit = () => {
    setEditTarget(null);
    setEditMessage("");
    setEditError("");
    setEditDraft(emptyUkkDraft());
  };

  const handleSave = async () => {
    if (!editTarget || yearClosed) return;
    setIsSaving(true);
    setEditMessage("");
    setEditError("");

    const trimmedNilai = editDraft.nilaiAkhir.trim();
    let nilaiAkhir: number | null = null;
    if (trimmedNilai) {
      const parsed = Number(trimmedNilai.replace(",", "."));
      if (!Number.isFinite(parsed)) {
        setEditError("Nilai akhir harus berupa angka.");
        setIsSaving(false);
        return;
      }
      nilaiAkhir = parsed;
    }

    const payload = {
      nisn: editTarget.nisn,
      tahun_lulus: editTarget.tahunLulus,
      skema_ukk: editDraft.skemaUkk.trim(),
      nilai_akhir: nilaiAkhir,
      predikat: editDraft.predikat.trim(),
      asesor: [
        {
          nama: editDraft.asesor1Nama.trim(),
          instansi: editDraft.asesor1Instansi.trim(),
        },
        {
          nama: editDraft.asesor2Nama.trim(),
          instansi: editDraft.asesor2Instansi.trim(),
        },
      ],
    };

    try {
      const response = await apiFetch("/api/siswa/ukk", {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        throw new Error("Gagal menyimpan UKK.");
      }
      setEditMessage("Data UKK berhasil disimpan.");
      void refetch();
    } catch (error) {
      setEditError(
        error instanceof Error ? error.message : "Gagal menyimpan UKK."
      );
    } finally {
      setIsSaving(false);
    }
  };

  // TODO: enforce filter finalized di backend
  const { visibleRows, filteredCount } = useMemo(() => {
    const visibleRows = rows.filter((row) => {
      const finalStatusValue = readString(row.finalStatus ?? "");
      const finalizedAtValue = readString(row.finalizedAt ?? "");
      return !finalStatusValue && !finalizedAtValue;
    });
    return {
      visibleRows,
      filteredCount: rows.length - visibleRows.length,
    };
  }, [rows]);

  const filteredRows = useMemo(() => {
    if (filter === "missing") {
      return visibleRows.filter((row) => !row.isComplete);
    }
    if (filter === "complete") {
      return visibleRows.filter((row) => row.isComplete);
    }
    return visibleRows;
  }, [visibleRows, filter]);

  const pagedRows = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredRows.slice(start, start + pageSize);
  }, [filteredRows, page, pageSize]);

  const handleDownloadMissing = async () => {
    if (!isSchoolAdmin) {
      return;
    }
    const query = new URLSearchParams();
    query.set("tahun_lulus", year);
    query.set("missing_only", "true");
    const response = await apiFetch(`/api/templates/ukk?${query.toString()}`);
    if (!response.ok) {
      return;
    }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.setAttribute("download", `ukk_missing_${year}.xlsx`);
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  const triggerRejectedDownload = async (
    reportUrl: string,
    filename?: string
  ) => {
    const response = await apiFetch(reportUrl, { method: "GET" });
    if (!response.ok) {
      return;
    }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    const dateLabel = new Date().toISOString().split("T")[0];
    const safeName =
      filename && filename.trim()
        ? filename.trim()
        : `rejected-ukk-${dateLabel}.xlsx`;
    link.href = url;
    link.setAttribute("download", safeName);
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    setFile(event.target.files?.[0] ?? null);
    setMessage("");
  };

  const handleImport = async () => {
    if (!isSchoolAdmin) {
      return;
    }
    if (!file) {
      setMessage("File wajib diunggah.");
      return;
    }
    const formData = new FormData();
    formData.append("file", file);

    setIsSubmitting(true);
    setMessage("");
    try {
      const response = await apiFetch("/api/siswa/ukk/import", {
        method: "POST",
        body: formData,
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        setMessage(buildErrorMessage(payload));
        return;
      }
      const data = payload?.data ?? payload ?? {};
      const rejected = Number(data?.rejected ?? 0);
      setMessage("Upload berhasil diproses.");
      setFile(null);
      void refetch();
      if (rejected > 0 && data?.rejected_excel_url) {
        void triggerRejectedDownload(
          data.rejected_excel_url,
          data.rejected_excel_filename
        );
      }
    } catch {
      setMessage("Upload gagal. Periksa koneksi atau format file.");
    } finally {
      setIsSubmitting(false);
    }
  };

  const columns = useMemo(() => {
    const items = [
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "skema", label: "Skema" },
      { key: "nilai", label: "Nilai", align: "right" as const },
      { key: "predikat", label: "Predikat" },
      { key: "asesor", label: "Asesor", align: "center" as const },
      { key: "status", label: "Status", align: "center" as const },
    ];
    if (isSchoolAdmin) {
      items.push({ key: "actions", label: "Aksi", align: "center" as const });
    }
    return items;
  }, [isSchoolAdmin]);

  const totalStudents = visibleRows.length;
  const completeCount = visibleRows.filter((row) => row.isComplete).length;
  const missingCount = totalStudents - completeCount;
  const hasFilteredFinal = filteredCount > 0;
  const canEditUkk = isSchoolAdmin && hasSMKScope && !yearClosed;

  return (
    <>
      <Card>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                UKK (SMK)
              </p>
              <h2 className="text-lg font-semibold text-zinc-900">
                Rekap kelengkapan UKK siswa
              </h2>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={handleDownloadMissing}
                disabled={!isSchoolAdmin || !hasSMKScope}
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Download Template (Missing)
              </button>
              <button
                type="button"
                onClick={() => setIsImportOpen((prev) => !prev)}
                disabled={!isSchoolAdmin || !hasSMKScope || yearClosed}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isImportOpen ? "Tutup Import" : "Import UKK"}
              </button>
            </div>
          </div>

          <div className="flex flex-wrap items-end gap-3">
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Tahun Lulus
              </label>
              <Select value={year} onValueChange={setYear}>
                <SelectTrigger className="h-10 w-[160px]">
                  <SelectValue placeholder="Pilih tahun" />
                </SelectTrigger>
                <SelectContent>
                  {yearOptions.map((option) => (
                    <SelectItem key={option} value={String(option)}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">
                Lengkap {completeCount}/{totalStudents}
              </span>
              <span className="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700">
                Belum lengkap {missingCount}
              </span>
              {yearClosed ? (
                <span className="rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">
                  Tahun Ditutup
                </span>
              ) : null}
            </div>
          </div>

          {!hasSMKScope || backendEmptyStateMessage ? (
            <div className="rounded-md border border-slate-200 bg-slate-50 p-3 text-sm text-slate-700">
              {effectiveEmptyMessage}
            </div>
          ) : null}
          {hasSMKScope && !backendEmptyStateMessage && missingCount > 0 ? (
            <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
              UKK belum lengkap: {missingCount} siswa. Gunakan Download Template
              (Missing) untuk mengisi yang belum lengkap.
            </div>
          ) : null}
          {hasSMKScope && yearClosed ? (
            <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
              Tahun sudah tutup buku. Perubahan UKK tidak diperbolehkan.
            </div>
          ) : null}

          {isImportOpen && isSchoolAdmin && hasSMKScope ? (
            <div className="rounded-md border border-dashed border-zinc-200 p-3">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  File Excel
                </label>
                <input
                  type="file"
                  accept=".xlsx,.xlsm"
                  onChange={handleFileChange}
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={handleImport}
                  disabled={isSubmitting}
                  className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isSubmitting ? "Mengunggah..." : "Submit"}
                </button>
                <span className="text-xs text-zinc-500">
                  Format file mengikuti template resmi.
                </span>
              </div>
              {message ? (
                <p className="mt-3 rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-600">
                  {message}
                </p>
              ) : null}
            </div>
          ) : null}

          <div className="flex flex-wrap items-center gap-2">
            {[
              { key: "all", label: "Semua" },
              { key: "missing", label: "Belum lengkap" },
              { key: "complete", label: "Lengkap" },
            ].map((item) => {
              const isActive = filter === item.key;
              return (
                <button
                  key={item.key}
                  type="button"
                  onClick={() =>
                    setFilter(item.key as "all" | "missing" | "complete")
                  }
                  className={`rounded-full border px-3 py-1 text-xs font-semibold transition ${
                    isActive
                      ? "border-zinc-900 bg-zinc-900 text-white"
                      : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-900 hover:text-zinc-900"
                  }`}
                >
                  {item.label}
                </button>
              );
            })}
          </div>

          <DataTable
            columns={columns}
            rows={pagedRows}
            isLoading={hasSMKScope ? isLoading : false}
            isError={isError}
            emptyMessage={effectiveEmptyMessage}
            page={page}
            pageSize={pageSize}
            total={filteredRows.length}
            onPageChange={setPage}
            renderRow={(row) => {
              const rowFinal = Boolean(
                readString(row.finalStatus) || readString(row.finalizedAt)
              );
              const rowLocked = yearClosed || rowFinal;
              const canEdit = isSchoolAdmin && !rowLocked;
              const label = rowLocked ? "Terkunci" : "Edit";

              return (
                <TableRow key={`${row.nisn}-${row.tahunLulus}`}>
                  <TableCell>{row.nisn || "-"}</TableCell>
                  <TableCell>{row.nama || "-"}</TableCell>
                  <TableCell>{row.rombel || "-"}</TableCell>
                  <TableCell>{row.skemaUkk || "-"}</TableCell>
                  <TableCell className="text-right">
                    {row.nilaiAkhir ?? "-"}
                  </TableCell>
                  <TableCell>{row.predikat || "-"}</TableCell>
                  <TableCell className="text-center">
                    {row.asesor.length}
                  </TableCell>
                  <TableCell className="text-center">
                    <span
                      className={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-semibold ${
                        row.isComplete
                          ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                          : "border-amber-200 bg-amber-50 text-amber-700"
                      }`}
                    >
                      {row.isComplete ? "Lengkap" : "Belum"}
                    </span>
                  </TableCell>
                  {isSchoolAdmin ? (
                    <TableCell className="text-center">
                      <button
                        type="button"
                        onClick={() => {
                          if (canEdit) openEdit(row);
                        }}
                        disabled={!canEdit}
                        className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {label}
                      </button>
                    </TableCell>
                  ) : null}
                </TableRow>
              );
            }}
          />
          {hasFilteredFinal ? (
            <p className="text-xs text-zinc-500">
              Data final disembunyikan dari pipeline.
            </p>
          ) : null}
        </CardContent>
      </Card>

      {editTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-3xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Edit Data UKK
              </h2>
              <p className="text-sm text-zinc-600">
                {editTarget.nama || "-"} - {editTarget.nisn || "-"} -{" "}
                {editTarget.rombel || "-"}
              </p>
            </div>

            <div className="mt-4 space-y-4">
              {yearClosed ? (
                <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
                  Tahun sudah tutup buku. Perubahan UKK tidak diperbolehkan.
                </div>
              ) : null}
              <div className="grid gap-3 md:grid-cols-2">
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Skema UKK
                  </label>
                  <input
                    type="text"
                    value={editDraft.skemaUkk}
                    onChange={(event) =>
                      setEditDraft((prev) => ({
                        ...prev,
                        skemaUkk: event.target.value,
                      }))
                    }
                    disabled={!canEditUkk}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Nilai Akhir
                  </label>
                  <input
                    type="text"
                    value={editDraft.nilaiAkhir}
                    onChange={(event) =>
                      setEditDraft((prev) => ({
                        ...prev,
                        nilaiAkhir: event.target.value,
                      }))
                    }
                    disabled={!canEditUkk}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                </div>
              </div>

              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Predikat
                </label>
                <input
                  type="text"
                  value={editDraft.predikat}
                  onChange={(event) =>
                    setEditDraft((prev) => ({
                      ...prev,
                      predikat: event.target.value,
                    }))
                  }
                  disabled={!canEditUkk}
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                <div className="space-y-2 rounded-lg border border-zinc-200 p-3">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Asesor 1
                  </p>
                  <input
                    type="text"
                    placeholder="Nama Asesor"
                    value={editDraft.asesor1Nama}
                    onChange={(event) =>
                      setEditDraft((prev) => ({
                        ...prev,
                        asesor1Nama: event.target.value,
                      }))
                    }
                    disabled={!canEditUkk}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                  <input
                    type="text"
                    placeholder="Instansi"
                    value={editDraft.asesor1Instansi}
                    onChange={(event) =>
                      setEditDraft((prev) => ({
                        ...prev,
                        asesor1Instansi: event.target.value,
                      }))
                    }
                    disabled={!canEditUkk}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                </div>
                <div className="space-y-2 rounded-lg border border-zinc-200 p-3">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Asesor 2
                  </p>
                  <input
                    type="text"
                    placeholder="Nama Asesor"
                    value={editDraft.asesor2Nama}
                    onChange={(event) =>
                      setEditDraft((prev) => ({
                        ...prev,
                        asesor2Nama: event.target.value,
                      }))
                    }
                    disabled={!canEditUkk}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                  <input
                    type="text"
                    placeholder="Instansi"
                    value={editDraft.asesor2Instansi}
                    onChange={(event) =>
                      setEditDraft((prev) => ({
                        ...prev,
                        asesor2Instansi: event.target.value,
                      }))
                    }
                    disabled={!canEditUkk}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  />
                </div>
              </div>

              {editMessage ? (
                <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                  {editMessage}
                </p>
              ) : null}
              {editError ? (
                <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                  {editError}
                </p>
              ) : null}
            </div>

            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={closeEdit}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Tutup
              </button>
              {canEditUkk ? (
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={isSaving}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isSaving ? "Menyimpan..." : "Simpan"}
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
