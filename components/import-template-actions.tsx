"use client";

import { useState, type ChangeEvent } from "react";
import { apiFetch } from "@/lib/api/client";
import { PasswordInput } from "@/components/password-input";
import readXlsxFile from "read-excel-file";
import { expectedHeadersByUploadType } from "@/lib/import-templates";

type ImportTemplateActionsProps = {
  title: string;
  templateUrl: string;
  uploadType: "ACHIEVEMENT" | "ACCEPTANCE";
  accept: string;
  onSuccess?: () => void;
  note?: string;
};

type HeaderValidationState = {
  expectedHeaders: string[];
  actualHeaders: string[];
};

const isExcelFile = (file: File | null) => {
  if (!file) return false;
  const name = file.name.toLowerCase();
  return name.endsWith(".xlsx") || name.endsWith(".xlsm");
};

const trimHeaderCell = (value: unknown) => String(value ?? "").trim();

const headersMatchExact = (actual: string[], expected: string[]) => {
  if (actual.length !== expected.length) return false;
  return actual.every((value, index) => value === expected[index]);
};

const formatHeaderList = (headers: string[]) => headers.join("; ");

export default function ImportTemplateActions({
  title,
  templateUrl,
  uploadType,
  accept,
  onSuccess,
  note,
}: ImportTemplateActionsProps) {
  const expectedHeaders = expectedHeadersByUploadType[uploadType];
  const [isOpen, setIsOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [year, setYear] = useState("");
  const [message, setMessage] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isCheckingHeader, setIsCheckingHeader] = useState(false);
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false);
  const [password, setPassword] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [headerMismatch, setHeaderMismatch] =
    useState<HeaderValidationState | null>(null);
  const [isHeaderModalOpen, setIsHeaderModalOpen] = useState(false);
  const [summary, setSummary] = useState<{
    totalRows: number;
    inserted: number;
    rejected: number;
    rejectedReportUrl?: string;
  } | null>(null);

  const resetForm = () => {
    setFile(null);
    setYear("");
  };

  const openPasswordModal = () => {
    setPasswordError("");
    setIsPasswordModalOpen(true);
  };

  const handleSubmit = async () => {
    setMessage("");
    if (!file) {
      setMessage("File wajib diunggah.");
      return;
    }
    if (!isExcelFile(file)) {
      setMessage("Format file harus .xlsx atau .xlsm.");
      return;
    }
    if (isCheckingHeader) {
      setMessage("Sedang memeriksa header Excel.");
      return;
    }
    if (headerMismatch) {
      setIsHeaderModalOpen(true);
      return;
    }
    if (!year.trim()) {
      setMessage("Tahun wajib diisi.");
      return;
    }

    const parsedYear = Number(year);
    if (!Number.isFinite(parsedYear) || parsedYear <= 0) {
      setMessage("Tahun harus berupa angka.");
      return;
    }

    openPasswordModal();
  };

  const triggerRejectedDownload = async (reportUrl: string) => {
    const response = await apiFetch(reportUrl, { method: "GET" });
    if (!response.ok) {
      return;
    }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    const dateLabel = new Date().toISOString().split("T")[0];
    const prefix =
      uploadType === "ACHIEVEMENT"
        ? "rejected-prestasi"
        : "rejected-siswa-perguruan-tinggi";
    link.href = url;
    link.setAttribute("download", `${prefix}-${dateLabel}.xlsx`);
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  const validateHeaderRow = async (targetFile: File) => {
    setIsCheckingHeader(true);
    try {
      const rows = await readXlsxFile(targetFile, { sheet: 1 });
      const headerRow = Array.isArray(rows[0]) ? rows[0] : [];
      const actualHeaders = headerRow.map(trimHeaderCell);
      const isMatch = headersMatchExact(actualHeaders, expectedHeaders);
      if (!isMatch) {
        setHeaderMismatch({ expectedHeaders, actualHeaders });
        setIsHeaderModalOpen(true);
      } else {
        setHeaderMismatch(null);
      }
    } catch {
      setHeaderMismatch({ expectedHeaders, actualHeaders: [] });
      setIsHeaderModalOpen(true);
      setMessage("Gagal membaca header Excel. Pastikan file sesuai template.");
    } finally {
      setIsCheckingHeader(false);
    }
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextFile = event.target.files?.[0] ?? null;
    setFile(nextFile);
    setMessage("");
    setHeaderMismatch(null);
    if (nextFile && isExcelFile(nextFile)) {
      void validateHeaderRow(nextFile);
    }
  };

  const handleConfirmImport = async () => {
    if (isSubmitting) return;
    setPasswordError("");
    if (!file) {
      setMessage("File wajib diunggah.");
      setIsPasswordModalOpen(false);
      return;
    }
    if (isCheckingHeader) {
      setMessage("Sedang memeriksa header Excel.");
      setIsPasswordModalOpen(false);
      return;
    }
    if (headerMismatch) {
      setIsHeaderModalOpen(true);
      setIsPasswordModalOpen(false);
      return;
    }
    const parsedYear = Number(year);
    if (!Number.isFinite(parsedYear) || parsedYear <= 0) {
      setMessage("Tahun harus berupa angka.");
      setIsPasswordModalOpen(false);
      return;
    }
    if (!password.trim()) {
      setPasswordError("Password wajib diisi.");
      return;
    }

    const formData = new FormData();
    formData.append("file", file);
    formData.append("type", uploadType);
    formData.append("year", String(parsedYear));
    formData.append("password", password);

    setIsSubmitting(true);
    try {
      const response = await apiFetch("/uploads", {
        method: "POST",
        body: formData,
      });

      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        if (payload?.message === "PASSWORD_INVALID") {
          setPasswordError("Password salah.");
          return;
        }
        if (payload?.message === "PASSWORD_REQUIRED") {
          setPasswordError("Password wajib diisi.");
          return;
        }
        const details = payload?.details ? `: ${payload.details}` : "";
        const errorMessage =
          payload?.error ?? `Upload gagal${details ? details : ""}`;
        setMessage(errorMessage);
        setIsPasswordModalOpen(false);
        return;
      }

      const nextSummary = {
        totalRows: Number(payload?.totalRows ?? 0),
        inserted: Number(payload?.inserted ?? 0),
        rejected: Number(payload?.rejected ?? 0),
        rejectedReportUrl: payload?.rejectedReportUrl,
      };
      setSummary(nextSummary);
      setIsPasswordModalOpen(false);
      setPassword("");
      if (nextSummary.rejected > 0 && nextSummary.rejectedReportUrl) {
        void triggerRejectedDownload(nextSummary.rejectedReportUrl);
      }
      setMessage("Upload berhasil diproses.");
      resetForm();
      onSuccess?.();
    } catch {
      setMessage("Upload gagal. Periksa koneksi atau format file.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="space-y-3 rounded-xl border border-zinc-200 bg-white p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Template & Import
          </p>
          <p className="text-sm text-zinc-600">
            Gunakan template agar format {title} sesuai.
          </p>
          {note ? <p className="mt-1 text-sm text-zinc-500">{note}</p> : null}
        </div>
        <div className="flex flex-col gap-2 sm:flex-row">
          <a
            href={templateUrl}
            download
            className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-900 transition hover:bg-slate-50"
          >
            Download Template
          </a>
          <button
            type="button"
            onClick={() => setIsOpen((prev) => !prev)}
            className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95"
          >
            Import Excel
          </button>
        </div>
      </div>

      {isOpen ? (
        <div className="grid gap-3 rounded-md border border-dashed border-zinc-200 p-3 sm:grid-cols-[1fr_160px_auto] sm:items-end">
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
          <div className="space-y-1">
            <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Tahun
            </label>
            <input
              type="number"
              value={year}
              onChange={(event) => setYear(event.target.value)}
              className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
              min={2000}
              placeholder="2025"
            />
          </div>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={isSubmitting || isCheckingHeader}
            className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-70"
          >
            {isSubmitting
              ? "Mengunggah..."
              : isCheckingHeader
                ? "Memeriksa..."
                : "Submit"}
          </button>
        </div>
      ) : null}

      {message ? <p className="text-sm text-zinc-600">{message}</p> : null}

      {isPasswordModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-lg rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Konfirmasi Import
              </h2>
              <p className="text-sm text-zinc-600">
                Masukkan password admin sekolah untuk melanjutkan import.
              </p>
            </div>
            <div className="mt-4 space-y-2">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Password Admin Sekolah
              </label>
              <PasswordInput
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                placeholder="Masukkan password"
                required
              />
              {passwordError ? (
                <p className="text-sm text-red-600">{passwordError}</p>
              ) : null}
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  setIsPasswordModalOpen(false);
                  setPasswordError("");
                }}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleConfirmImport}
                disabled={isSubmitting}
                className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isSubmitting ? "Mengunggah..." : "Lanjut Import"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

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
              <div>Berhasil diinsert: {summary.inserted}</div>
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
              <div>
                <span className="font-semibold">Header yang benar:</span>{" "}
                {formatHeaderList(headerMismatch.expectedHeaders)}
              </div>
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

