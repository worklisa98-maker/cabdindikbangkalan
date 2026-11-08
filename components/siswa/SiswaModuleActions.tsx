"use client";

import { useState, type ChangeEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "@/lib/api/client";
import { invalidatePTDerivedQueries } from "@/lib/invalidate-pt-queries";

type ImportSummary = {
  totalRows: number;
  accepted: number;
  rejected: number;
  rejectedReportUrl?: string;
  rejectedFilename?: string;
};

type HeaderValidationState = {
  expectedTemplates: Array<{ label: string; headers: string[] }>;
  expectedHeaders: string[];
  actualHeaders: string[];
};

type SiswaModuleActionsProps = {
  title: string;
  sectionLabel?: string;
  description?: string;
  templateUrl: string;
  templateButtonLabel?: string;
  importUrl: string;
  importButtonLabel?: string;
  importCloseButtonLabel?: string;
  submitButtonLabel?: string;
  formatHint?: string;
  accept: string;
  isSchoolAdmin: boolean;
  onSuccess?: () => void;
};

const isObjectRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === "object" && value !== null;
};

const buildErrorMessage = (payload: unknown) => {
  if (!isObjectRecord(payload)) {
    return "Upload gagal. Periksa koneksi atau format file.";
  }

  const payloadMessage = payload.message;
  if (typeof payloadMessage === "string" && payloadMessage.trim()) {
    return payloadMessage;
  }

  const payloadError = payload.error;
  if (typeof payloadError === "string" && payloadError.trim()) {
    if (payloadError === "TEMPLATE_MISMATCH") {
      return "Header template tidak sesuai. Silakan unduh template terbaru.";
    }
    return payloadError;
  }

  const details = isObjectRecord(payload.details) ? payload.details : null;
  const missing = details?.missing_columns;
  if (Array.isArray(missing) && missing.length > 0) {
    return `Kolom wajib belum lengkap: ${missing.join(", ")}`;
  }
  return "Upload gagal. Periksa koneksi atau format file.";
};

const formatHeaderList = (headers: string[]) => headers.join("; ");

export default function SiswaModuleActions({
  title,
  sectionLabel = "Aksi Modul",
  description,
  templateUrl,
  templateButtonLabel = "Template",
  importUrl,
  importButtonLabel = "Import",
  importCloseButtonLabel = "Tutup Import",
  submitButtonLabel = "Submit",
  formatHint = "Format file mengikuti template resmi.",
  accept,
  isSchoolAdmin,
  onSuccess,
}: SiswaModuleActionsProps) {
  const queryClient = useQueryClient();
  const [isOpen, setIsOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [message, setMessage] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [summary, setSummary] = useState<ImportSummary | null>(null);
  const [headerMismatch, setHeaderMismatch] =
    useState<HeaderValidationState | null>(null);
  const [isHeaderModalOpen, setIsHeaderModalOpen] = useState(false);

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
        if (payload?.error === "TEMPLATE_MISMATCH") {
          const expectedTemplatesRaw =
            payload?.details?.expected_header_templates;
          const expectedTemplates = Array.isArray(expectedTemplatesRaw)
            ? expectedTemplatesRaw
                .map((item) => ({
                  label:
                    typeof item?.label === "string" ? item.label : "Template",
                  headers: Array.isArray(item?.headers)
                    ? item.headers.filter(
                        (header: unknown) => typeof header === "string"
                      )
                    : [],
                }))
                .filter((item) => item.headers.length > 0)
            : [];
          const expected = Array.isArray(payload?.details?.expected_headers)
            ? payload.details.expected_headers
            : [];
          const found = Array.isArray(payload?.details?.found_headers)
            ? payload.details.found_headers
            : [];
          setHeaderMismatch({
            expectedTemplates,
            expectedHeaders: expected,
            actualHeaders: found,
          });
          setIsHeaderModalOpen(true);
          setMessage("Header template tidak sesuai.");
          return;
        }
        setMessage(buildErrorMessage(payload));
        return;
      }

      const data = payload?.data ?? payload ?? {};
      const nextSummary: ImportSummary = {
        totalRows: Number(data?.total_rows ?? data?.totalRows ?? 0),
        accepted: Number(data?.accepted ?? 0),
        rejected: Number(data?.rejected ?? 0),
        rejectedReportUrl: data?.rejected_excel_url ?? data?.rejectedReportUrl,
        rejectedFilename:
          data?.rejected_excel_filename ?? data?.rejectedFilename,
      };

      setSummary(nextSummary);
      setHeaderMismatch(null);
      setMessage("Upload berhasil diproses.");
      resetForm();
      onSuccess?.();
      await invalidatePTDerivedQueries(queryClient);
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

  return (
    <div className="rounded-xl border border-zinc-200 bg-white p-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            {sectionLabel}
          </p>
          <p className="text-sm text-zinc-600">
            {description ||
              (isSchoolAdmin
                ? `Kelola template dan import data ${title}.`
                : "Lihat histori import data pada modul ini.")}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {isSchoolAdmin ? (
            <>
              <a
                href={templateUrl}
                download
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                {templateButtonLabel}
              </a>
              <button
                type="button"
                onClick={() => setIsOpen((prev) => !prev)}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                {isOpen ? importCloseButtonLabel : importButtonLabel}
              </button>
            </>
          ) : null}
        </div>
      </div>

      {isSchoolAdmin && isOpen ? (
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
              {isSubmitting ? "Mengunggah..." : submitButtonLabel}
            </button>
            <span className="text-xs text-zinc-500">{formatHint}</span>
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
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
              >
                OK
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isHeaderModalOpen && headerMismatch ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-lg rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Template tidak sesuai
              </h2>
              <p className="text-sm text-zinc-600">
                Header kolom pada file tidak sesuai template. Unduh template dan
                sesuaikan lalu coba lagi.
              </p>
            </div>
            <div className="mt-4 space-y-2 rounded-md border border-zinc-200 bg-zinc-50 p-3 text-xs text-zinc-700">
              {headerMismatch.expectedTemplates.length > 0 ? (
                headerMismatch.expectedTemplates.map((item) => (
                  <div key={item.label}>
                    <span className="font-semibold">
                      Header benar ({item.label}):
                    </span>{" "}
                    {formatHeaderList(item.headers)}
                  </div>
                ))
              ) : (
                <div>
                  <span className="font-semibold">Header yang benar:</span>{" "}
                  {headerMismatch.expectedHeaders.length > 0
                    ? formatHeaderList(headerMismatch.expectedHeaders)
                    : "-"}
                </div>
              )}
              <div>
                <span className="font-semibold">Header pada file:</span>{" "}
                {headerMismatch.actualHeaders.length > 0
                  ? formatHeaderList(headerMismatch.actualHeaders)
                  : "-"}
              </div>
            </div>
            <div className="mt-5 flex justify-end">
              <button
                type="button"
                onClick={() => setIsHeaderModalOpen(false)}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95"
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
