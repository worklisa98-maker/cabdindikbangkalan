"use client";

import { useEffect, useRef, useState } from "react";
import { normalizeGaspolReportURL } from "@/lib/gaspol-report-url";
import { apiFetch } from "@/lib/api/client";

type Notice = {
  id: number;
  lockId: number;
  status: string;
  message: string;
  tone: string;
  eventAt: string;
  read: boolean;
};
type Report = {
  year: number;
  schoolId: number;
  submissionType: string;
  status: string;
  canEdit: boolean;
  improvementDetails: string;
  improvementResult: string;
  responseText: string;
  driveUrl: string;
  evidenceUrls: string[];
};
const nomination = "NOT_FAILED_NOMINATION";
const appeal = "BOTTOM5_APPEAL";
const fieldClass =
  "mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm disabled:bg-slate-100";
const tones: Record<string, string> = {
  success: "border-emerald-200 bg-emerald-50 text-emerald-950",
  attention: "border-amber-200 bg-amber-50 text-amber-950",
  candidate: "border-rose-200 bg-rose-50 text-rose-950",
};
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await apiFetch(path, init);
  const payload = await response.json();
  if (!response.ok)
    throw new Error(
      payload.message ?? payload.error ?? "Permintaan gagal. Silakan coba lagi."
    );
  return payload.data as T;
}
function safeLink(value: string) {
  return !!normalizeGaspolReportURL(value);
}
function ReportSummary({ report }: { report: Report }) {
  return (
    <div className="space-y-3 text-sm">
      <p className="font-semibold">Status laporan: {report.status}</p>
      <dl className="space-y-2">
        {[
          ["Permasalahan / upaya perbaikan", report.improvementDetails],
          ["Hasil perbaikan", report.improvementResult],
          ["Tanggapan / keterangan", report.responseText],
        ].map(([label, value]) => (
          <div key={label}>
            <dt className="font-medium">{label}</dt>
            <dd className="whitespace-pre-wrap text-slate-600">
              {value || "Belum diisi"}
            </dd>
          </div>
        ))}
      </dl>
      {[report.driveUrl, ...(report.evidenceUrls ?? [])]
        .filter(safeLink)
        .map((url, i) => (
          <a
            key={`${i}-${url}`}
            className="block break-all text-emerald-700 underline"
            href={url}
            target="_blank"
            rel="noopener noreferrer"
          >
            {i === 0 ? "Tautan laporan / Drive" : `Bukti pendukung ${i}`}: {url}
          </a>
        ))}
    </div>
  );
}

export function NominationReportPanel({
  year,
  schoolId,
  stage,
  editable = false,
}: {
  year: number;
  schoolId: number;
  stage: string;
  editable?: boolean;
}) {
  const [report, setReport] = useState<Report | null>(null);
  const [linkErrors, setLinkErrors] = useState<{
    drive?: string;
    evidence?: string;
  }>({});
  const [evidence, setEvidence] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [expanded, setExpanded] = useState(false);
  useEffect(() => {
    let active = true;
    setReport(null);
    setError("");
    setMessage("");
    setExpanded(false);
    request<Report>(
      `/gaspol/nomination-submissions?year=${year}&schoolId=${schoolId}&stage=${stage}`
    )
      .then((data) => {
        if (active) {
          setReport(data);
          setEvidence((data.evidenceUrls ?? []).join("\n"));
        }
      })
      .catch((err) => {
        if (active) setError(err.message);
      });
    return () => {
      active = false;
    };
  }, [year, schoolId, stage]);
  const save = async (status: "DRAFT" | "SUBMITTED") => {
    if (!report || busy) return;
    setError("");
    setMessage("");
    const rawLinks = evidence
      .split(/\r?\n/)
      .map((link) => link.trim())
      .filter(Boolean);
    const driveUrl = normalizeGaspolReportURL(report.driveUrl);
    const evidenceUrls = rawLinks.map(normalizeGaspolReportURL);
    const invalid = {
      drive:
        report.driveUrl.trim() && !driveUrl
          ? "Gunakan domain atau tautan HTTP/HTTPS yang valid."
          : status === "SUBMITTED" && !driveUrl
            ? "Tautan laporan wajib diisi saat mengirim laporan."
            : undefined,
      evidence: rawLinks.some((link) => !normalizeGaspolReportURL(link))
        ? "Periksa bukti pada baris: " +
          rawLinks
            .map((link, i) => (!normalizeGaspolReportURL(link) ? i + 1 : null))
            .filter(Boolean)
            .join(", ")
        : undefined,
    };
    setLinkErrors(invalid);
    if (invalid.drive || invalid.evidence) return;
    setBusy(true);
    try {
      await request("/gaspol/school/nomination-submission", {
        method: "PUT",
        body: JSON.stringify({ ...report, driveUrl, status, evidenceUrls }),
      });
      const saved = await request<Report>(
        `/gaspol/nomination-submissions?year=${year}&schoolId=${schoolId}&stage=${stage}`
      );
      setReport(saved);
      setExpanded(false);
      setMessage(
        status === "DRAFT"
          ? "Draft berhasil disimpan."
          : "Laporan berhasil dikirim."
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan laporan.");
    } finally {
      setBusy(false);
    }
  };
  const canEdit = editable && report?.canEdit && report.status !== "LOCKED";
  return (
    <section className="space-y-4 rounded-2xl border border-slate-200 bg-white p-4">
      <h3 className="font-semibold text-slate-950">
        {stage === appeal
          ? "Laporan Banding Kandidat Zona Tergagal"
          : "Laporan Nominasi Tidak Tergagal"}
      </h3>
      {error && (
        <p role="alert" className="text-sm text-red-700">
          {error}
        </p>
      )}
      {message && (
        <p role="status" className="text-sm text-emerald-700">
          {message}
        </p>
      )}
      {!report && !error && <p role="status">Memuat laporan…</p>}
      {report && <ReportSummary report={report} />}
      {canEdit && (
        <button
          type="button"
          className="rounded-xl bg-emerald-700 px-4 py-2 text-sm text-white"
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
        >
          {report?.status === "DRAFT" ? "Isi Laporan" : "Perbarui Laporan"}
        </button>
      )}
      {canEdit && expanded && report && (
        <form
          onSubmit={(event) => {
            event.preventDefault();
            void save("SUBMITTED");
          }}
          className="space-y-3"
        >
          {(
            [
              ["improvementDetails", "Uraian permasalahan / upaya perbaikan"],
              ["improvementResult", "Hasil perbaikan"],
              ["responseText", "Tanggapan / keterangan"],
            ] as const
          ).map(([key, label]) => (
            <label key={key} className="block text-sm font-medium">
              {label}
              <textarea
                rows={3}
                className={fieldClass}
                disabled={busy}
                value={report[key]}
                onChange={(event) =>
                  setReport({ ...report, [key]: event.target.value })
                }
              />
            </label>
          ))}
          <label className="block text-sm font-medium">
            Tautan Drive / laporan (wajib saat kirim)
            <input
              type="text"
              aria-invalid={!!linkErrors.drive}
              disabled={busy}
              className={fieldClass}
              value={report.driveUrl}
              onChange={(event) =>
                setReport({ ...report, driveUrl: event.target.value })
              }
            />
            {linkErrors.drive && (
              <span role="alert" className="text-red-700">
                {linkErrors.drive}
              </span>
            )}
          </label>
          <label className="block text-sm font-medium">
            Bukti pendukung (satu tautan per baris)
            <textarea
              rows={3}
              disabled={busy}
              className={fieldClass}
              value={evidence}
              onChange={(event) => setEvidence(event.target.value)}
            />
          </label>
          {linkErrors.evidence && (
            <p role="alert" className="text-sm text-red-700">
              {linkErrors.evidence}
            </p>
          )}
          <div className="flex flex-wrap gap-2">
            <button
              disabled={busy}
              type="button"
              className="rounded-xl border px-4 py-2"
              onClick={() => void save("DRAFT")}
            >
              Simpan Draft
            </button>
            <button
              disabled={busy}
              type="submit"
              className="rounded-xl bg-emerald-700 px-4 py-2 text-white"
            >
              Kirim Laporan
            </button>
          </div>
        </form>
      )}
      {editable && report && !canEdit && (
        <p className="text-sm text-slate-600">
          Laporan hanya dapat dibaca karena telah terkunci atau tahap pengisian
          telah ditutup.
        </p>
      )}
    </section>
  );
}

export function ReviewNominationEvidence({
  year,
  rows,
}: {
  year: number;
  rows: { id: number; schoolId: number; schoolName: string; stage: string }[];
}) {
  const [selected, setSelected] = useState("");
  const reports = rows.filter(
    (row) => row.stage === nomination || row.stage === appeal
  );
  const target =
    reports.find((row) => String(row.id) === selected) ?? reports[0];
  if (!target) return null;
  return (
    <div className="space-y-3">
      <label className="block text-sm font-medium">
        Laporan sekolah untuk review
        <select
          className={fieldClass}
          value={String(target.id)}
          onChange={(event) => setSelected(event.target.value)}
        >
          {reports.map((row) => (
            <option key={row.id} value={row.id}>
              {row.schoolName}
            </option>
          ))}
        </select>
      </label>
      <NominationReportPanel
        key={`${year}-${target.stage}-${target.schoolId}`}
        year={year}
        schoolId={target.schoolId}
        stage={target.stage}
      />
    </div>
  );
}

export function GaspolSchoolPhase({
  year,
  schoolId,
}: {
  year: number;
  schoolId: number;
}) {
  const [notices, setNotices] = useState<Notice[]>([]);
  const [popup, setPopup] = useState<Notice | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    let active = true;
    setNotices([]);
    setPopup(null);
    setError("");
    request<Notice[]>(`/gaspol/school/competition-notices?year=${year}`)
      .then((data) => {
        if (!active) return;
        setNotices(data);
        if (data[0] && !data[0].read) setPopup(data[0]);
      })
      .catch((err) => {
        if (active) setError(err.message);
      });
    return () => {
      active = false;
    };
  }, [year, schoolId]);
  useEffect(() => {
    if (popup && !dialog.current?.open) dialog.current?.showModal();
  }, [popup]);
  // Persist when the official change is first displayed, including a refresh
  // before the user closes the dialog. Acknowledgement is idempotent on the server.
  useEffect(() => {
    if (!popup || popup.read) return;
    let active = true;
    request(`/gaspol/school/competition-notices/${popup.id}/ack`, {
      method: "POST",
      body: JSON.stringify({ year, lockId: popup.lockId }),
    })
      .then(() => {
        if (active) {
          setNotices((current) =>
            current.map((n) => (n.id === popup.id ? { ...n, read: true } : n))
          );
          setPopup({ ...popup, read: true });
        }
      })
      .catch((err) => {
        if (active) setError(err.message);
      });
    return () => {
      active = false;
    };
  }, [popup, year]);
  const dismiss = async () => {
    if (!popup || busy) return;
    setBusy(true);
    setError("");
    try {
      await request(`/gaspol/school/competition-notices/${popup.id}/ack`, {
        method: "POST",
        body: JSON.stringify({ year, lockId: popup.lockId }),
      });
      setNotices((current) =>
        current.map((n) => (n.id === popup.id ? { ...n, read: true } : n))
      );
      dialog.current?.close();
      setPopup(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal menyimpan tanda baca."
      );
    } finally {
      setBusy(false);
    }
  };
  const latest = notices[0];
  const hasNomination = notices.some((n) => n.status === "NOT_FAILED");
  const hasAppeal = notices.some((n) => n.status === "BOTTOM_5");
  return (
    <div className="space-y-4">
      {latest && (
        <div
          className={`rounded-2xl border p-4 ${tones[latest.tone]}`}
          role="status"
        >
          <p className="font-semibold">Hasil resmi GASPOL {year}</p>
          <p>{latest.message}</p>
          {latest.status === "NOT_FAILED" && (
            <a
              className="mt-2 inline-block font-semibold underline"
              href="#gaspol-nomination-report"
            >
              Isi Laporan Nominasi Tidak Tergagal
            </a>
          )}
        </div>
      )}
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      {hasNomination && (
        <div id="gaspol-nomination-report">
          <NominationReportPanel
            key={`${year}-${schoolId}-${nomination}`}
            year={year}
            schoolId={schoolId}
            stage={nomination}
            editable
          />
        </div>
      )}
      {hasAppeal && (
        <NominationReportPanel
          key={`${year}-${schoolId}-${appeal}`}
          year={year}
          schoolId={schoolId}
          stage={appeal}
          editable
        />
      )}
      {notices.length > 0 && (
        <details className="rounded-2xl border p-4">
          <summary className="cursor-pointer font-semibold">
            Histori pemberitahuan resmi
          </summary>
          <ul className="mt-3 space-y-2">
            {notices.map((n) => (
              <li
                key={`${n.id}-${n.lockId}`}
                className={`rounded-xl border p-3 ${tones[n.tone]}`}
              >
                <p className="text-xs">
                  {new Date(n.eventAt).toLocaleString("id-ID", {
                    timeZone: "Asia/Jakarta",
                  })}
                </p>
                <p>{n.message}</p>
              </li>
            ))}
          </ul>
        </details>
      )}
      {popup && (
        <dialog
          ref={dialog}
          aria-labelledby="gaspol-notice-title"
          onCancel={(event) => {
            event.preventDefault();
            void dismiss();
          }}
          className="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-lg rounded-2xl p-6 shadow-xl backdrop:bg-slate-950/50"
        >
          <h2 id="gaspol-notice-title" className="text-lg font-bold">
            Pemberitahuan resmi GASPOL {year}
          </h2>
          <p className={`my-4 rounded-xl border p-4 ${tones[popup.tone]}`}>
            {popup.message}
          </p>
          {popup.status === "NOT_FAILED" && (
            <a
              href="#gaspol-nomination-report"
              onClick={() => void dismiss()}
              className="mb-4 block font-semibold text-emerald-700 underline"
            >
              Isi Laporan Nominasi Tidak Tergagal
            </a>
          )}
          {error && (
            <p role="alert" className="mb-3 text-red-700">
              {error}
            </p>
          )}
          <button
            type="button"
            autoFocus
            disabled={busy}
            onClick={() => void dismiss()}
            className="rounded-xl bg-emerald-700 px-4 py-2 text-white"
          >
            {busy ? "Menyimpan…" : "Sudah dibaca"}
          </button>
        </dialog>
      )}
    </div>
  );
}
