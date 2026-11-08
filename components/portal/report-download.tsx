"use client";
import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";
import { normalizeAppRole } from "@/lib/auth/role-scope";
import { getErrorMessageFromResponse } from "@/lib/http";
type BundleJobPayload = {
  job_id?: string;
  status?: string;
  progress?: number;
  error_message?: string;
  download_url?: string;
};

type BundleJobState = {
  jobId: string;
  status: string;
  progress: number;
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const normalizeBundleJob = (payload: unknown): BundleJobPayload => {
  if (!isRecord(payload)) {
    return {};
  }
  const data = isRecord(payload.data) ? payload.data : payload;
  return {
    job_id: typeof data.job_id === "string" ? data.job_id : undefined,
    status: typeof data.status === "string" ? data.status : undefined,
    progress:
      typeof data.progress === "number" && Number.isFinite(data.progress)
        ? data.progress
        : undefined,
    error_message:
      typeof data.error_message === "string" ? data.error_message : undefined,
    download_url:
      typeof data.download_url === "string" ? data.download_url : undefined,
  };
};

export function ReportDownload() {
  const { user } = useAuth();
  const searchParams = useSearchParams();
  const normalizedRole = normalizeAppRole(user?.role);
  const [bundleJob, setBundleJob] = useState<BundleJobState | null>(null);
  const [bundleError, setBundleError] = useState("");
  const canDownloadBundleReport = Boolean(
    user &&
    (normalizedRole === "ADMIN" ||
      user.role === "ADMIN_MASTER" ||
      user.role === "SCHOOL_ADMIN" ||
      user.role === "PENGAWAS")
  );
  const isBundleProcessing = Boolean(
    bundleJob &&
    (bundleJob.status === "queued" || bundleJob.status === "running")
  );
  const handleDownloadBundleReport = async (forceRegenerate = false) => {
    if (isBundleProcessing) {
      return;
    }
    setBundleError("");

    const query = new URLSearchParams();
    const allowedKeys = [
      "tab",
      "status",
      "year",
      "jenjang",
      "status_sekolah",
      "school_id",
      "jenis_lomba",
      "jalur",
      "q",
      "start_year",
      "end_year",
    ];
    for (const key of allowedKeys) {
      const value = searchParams?.get(key);
      if (value) {
        query.set(key, value);
      }
    }
    if (forceRegenerate) {
      query.set("force", "1");
    }

    try {
      const createEndpoint = `/api/reports/bundle/jobs${
        query.toString() ? `?${query.toString()}` : ""
      }`;
      const response = await apiFetch(createEndpoint, { method: "POST" });
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(
            response,
            "Gagal membuat job laporan lengkap."
          )
        );
      }
      const createdPayload = normalizeBundleJob(
        await response.json().catch(() => null)
      );
      const jobId = createdPayload.job_id ?? "";
      const status = createdPayload.status ?? "queued";
      const progress = Math.max(0, Math.min(100, createdPayload.progress ?? 0));

      if (!jobId) {
        throw new Error("Job ID tidak ditemukan.");
      }
      setBundleJob({ jobId, status, progress });

      if (status === "failed") {
        throw new Error(
          createdPayload.error_message || "Proses generate PDF gagal."
        );
      }
      if (status === "done") {
        window.location.assign(
          createdPayload.download_url || `/api/reports/jobs/${jobId}/download`
        );
        setBundleJob(null);
        return;
      }

      while (true) {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        const statusResponse = await apiFetch(`/api/reports/jobs/${jobId}`);
        if (!statusResponse.ok) {
          throw new Error(
            await getErrorMessageFromResponse(
              statusResponse,
              "Gagal membaca status job laporan."
            )
          );
        }

        const statusPayload = normalizeBundleJob(
          await statusResponse.json().catch(() => null)
        );
        const polledStatus = statusPayload.status ?? "running";
        const polledProgress = Math.max(
          0,
          Math.min(100, statusPayload.progress ?? 0)
        );
        setBundleJob({ jobId, status: polledStatus, progress: polledProgress });

        if (polledStatus === "failed") {
          throw new Error(
            statusPayload.error_message || "Proses generate PDF gagal."
          );
        }
        if (polledStatus === "done") {
          window.location.assign(
            statusPayload.download_url || `/api/reports/jobs/${jobId}/download`
          );
          setBundleJob(null);
          return;
        }
      }
    } catch (error) {
      setBundleError(
        error instanceof Error
          ? error.message
          : "Gagal mengunduh laporan lengkap."
      );
      setBundleJob(null);
    }
  };

  if (!canDownloadBundleReport) return null;
  return (
    <div className="space-y-2 border-t border-slate-100 pt-2">
      <button
        className="block p-2 text-sm"
        disabled={isBundleProcessing}
        onClick={() => void handleDownloadBundleReport(false)}
      >
        Unduh laporan PDF
      </button>
      <button
        className="block p-2 text-sm"
        disabled={isBundleProcessing}
        onClick={() => void handleDownloadBundleReport(true)}
      >
        Buat ulang PDF
      </button>
      {isBundleProcessing && (
        <p role="status" className="text-xs">
          Membuat PDF� ({bundleJob?.progress ?? 0}%)
        </p>
      )}
      {bundleError && (
        <p role="alert" className="text-xs text-red-700">
          {bundleError}
        </p>
      )}
    </div>
  );
}
