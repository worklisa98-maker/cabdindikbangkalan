"use client";

import { useState, type ChangeEvent } from "react";
import { apiFetch } from "@/lib/api/client";

type ImportSummary = {
  totalRows: number;
  accepted: number;
  rejected: number;
  rejectedReportUrl?: string;
  rejectedFilename?: string;
};

type LegacyImportActionsProps = {
  title: string;
  templateUrl: string;
  importUrl: string;
  accept: string;
  isSchoolAdmin: boolean;
  enabled: boolean;
  maxYear?: number | null;
  onSuccess?: () => void;
};

const buildErrorMessage = (payload: unknown) => {
  if (!payload || typeof payload !== "object") {
    return "Upload gagal. Periksa koneksi atau format file.";
  }
  if ("message" in payload && typeof payload.message === "string" && payload.message.trim()) {
    return payload.message;
  }
  if ("error" in payload && typeof payload.error === "string" && payload.error.trim()) {
    if (payload.error === "TEMPLATE_MISMATCH") {
      return "Header template tidak sesuai. Silakan unduh template terbaru.";
    }
    return payload.error;
  }
  const details = "details" in payload ? payload.details : null;
  const missing = details && typeof details === "object" && "missing_columns" in details
    ? details.missing_columns : null;
  if (Array.isArray(missing) && missing.length > 0) {
    return `Kolom wajib belum lengkap: ${missing.join(", ")}`;
  }
  return "Upload gagal. Periksa koneksi atau format file.";
};

const buildTooltipText = (maxYear?: number | null) =>
  `Fitur pendataan lulusan tahun sebelumnya belum aktif.\n` +
  `Template dan import digunakan khusus untuk melengkapi data lulusan tahun-tahun sebelumnya.\n` +
  `Untuk Aktivasi fitur dapat menghubungi Cabdin.\n` +
  `Data yang dapat diisi hingga tahun ${maxYear ?? "-"}.`;

export default function LegacyImportActions({
  title,
  templateUrl,
  importUrl,
  accept,
  isSchoolAdmin,
  enabled,
  maxYear,
  onSuccess,
}: LegacyImportActionsProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [message, setMessage] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [summary, setSummary] = useState<ImportSummary | null>(null);

  const resetForm = () => {
    setFile(null);
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
        : `rejected-${dateLabel}.xlsx`;
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

  const handleSubmit = async () => {
    setMessage("");
    if (!file) {
      setMessage("File wajib diunggah.");
      return;
    }

    const formData = new FormData();
    formData.append("file", file);

    setIsSubmitting(true);
    try {
      const response = await apiFetch(importUrl, {
        method: "POST",
        body: formData,
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        setMessage(buildErrorMessage(payload));
        return;
      }

      const data = payload?.data ?? payload ?? {};
      const nextSummary: ImportSummary = {
        totalRows: Number(data?.total_rows ?? data?.totalRows ?? 0),
        accepted: Number(data?.accepted ?? 0),
        rejected: Number(data?.rejected ?? 0),
        rejectedReportUrl:
          data?.rejected_excel_url ?? data?.rejectedReportUrl,
        rejectedFilename:
          data?.rejected_excel_filename ?? data?.rejectedFilename,
      };

      setSummary(nextSummary);
      setMessage("Upload berhasil diproses.");
      resetForm();
      onSuccess?.();
      if (nextSummary.rejected > 0 && nextSummary.rejectedReportUrl) {
        void triggerRejectedDownload(
          nextSummary.rejectedReportUrl,
          nextSummary.rejectedFilename
        );
      }
    } catch {
      setMessage("Upload gagal. Periksa koneksi atau format file.");
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!isSchoolAdmin) {
    return null;
  }

  const tooltipText = buildTooltipText(maxYear);
  const isDisabled = !enabled;

  return (
    <div className="rounded-xl border border-zinc-200 bg-white p-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Pendataan Lulusan Tahun Sebelumnya
          </p>
          <p className="text-sm text-zinc-600">
            Gunakan template khusus untuk import {title} pada tahun sebelumnya.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <a
            href={templateUrl}
            download
            aria-disabled={isDisabled}
            className={`rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 ${
              isDisabled
                ? "pointer-events-none opacity-60"
                : "hover:bg-zinc-50"
            }`}
          >
            Download Template
          </a>
          <button
            type="button"
            onClick={() => setIsOpen((prev) => !prev)}
            disabled={isDisabled}
            className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-3 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isOpen ? "Tutup Import" : "Import Excel"}
          </button>
          {isDisabled ? (
            <div className="relative flex items-center">
              <div className="group relative">
                <div className="flex h-7 w-7 items-center justify-center rounded-full border border-zinc-300 bg-white text-xs font-bold text-zinc-600">
                  !
                </div>
                <div className="pointer-events-none absolute right-0 top-9 z-10 w-72 rounded-md border border-zinc-200 bg-white p-3 text-xs text-zinc-700 shadow-lg opacity-0 transition group-hover:opacity-100">
                  <span className="whitespace-pre-line">{tooltipText}</span>
                </div>
              </div>
            </div>
          ) : null}
        </div>
      </div>

      {isOpen && !isDisabled ? (
        <div className="mt-4 space-y-3 rounded-md border border-dashed border-zinc-200 p-3">
          <div className="space-y-1">
            <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              File Excel
            </label>
            <input
              type="file"
              accept={accept}
              onChange={handleFileChange}
              className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
            />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={handleSubmit}
              disabled={isSubmitting}
              className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-70"
            >
              {isSubmitting ? "Mengunggah..." : "Submit"}
            </button>
            <span className="text-xs text-zinc-500">
              Format file mengikuti template resmi.
            </span>
          </div>
        </div>
      ) : null}

      {message ? <p className="mt-3 text-sm text-zinc-600">{message}</p> : null}

      {summary ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-lg rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Ringkasan Import
              </h2>
              <p className="text-sm text-zinc-600">
                Import selesai diproses. Berikut ringkasannya.
              </p>
            </div>
            <div className="mt-4 space-y-2 rounded-md border border-zinc-200 p-3 text-sm text-zinc-700">
              <div>Total baris dibaca: {summary.totalRows}</div>
              <div>Berhasil diinsert: {summary.accepted}</div>
              <div>Ditolak/duplikat: {summary.rejected}</div>
            </div>
            <div className="mt-4 flex justify-end">
              <button
                type="button"
                onClick={() => setSummary(null)}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95"
              >
                OK
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

