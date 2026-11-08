"use client";

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api/client";
import { Button } from "./primitives";

type Report = {
  total_rows: number;
  valid_coordinates: number;
  matched: number;
  eligible: number;
  imported: number;
  skipped_existing: number;
  skipped_manual: number;
  skipped_invalid: number;
  not_found: number;
  errors: string[];
  rows: { npsn: string; name: string; latitude: number; longitude: number }[];
};

const labels = {
  total_rows: "Total baris Excel",
  valid_coordinates: "Koordinat valid",
  matched: "Cocok dengan sekolah (NPSN)",
  not_found: "NPSN tidak ditemukan",
  skipped_existing: "Sudah mempunyai lokasi",
  skipped_manual: "Sudah diedit/manual atau dilindungi audit",
  eligible: "Siap diimpor saat pemeriksaan",
  skipped_invalid: "Koordinat/NPSN invalid atau duplikat",
} as const;

export function SchoolLocationImport({
  onImported,
}: {
  onImported: () => void;
}) {
  const client = useQueryClient();
  const [report, setReport] = useState<Report | null>(null);
  const [busy, setBusy] = useState(false);
  const [applied, setApplied] = useState(false);
  const [error, setError] = useState("");

  async function run(apply: boolean) {
    if (
      apply &&
      (!report ||
        applied ||
        !report.eligible ||
        !window.confirm(
          `Import ${report.eligible} lokasi sekolah?\n\nData lokasi sekolah yang sudah memiliki koordinat atau pernah diedit secara manual tidak akan diubah. Kelayakan diperiksa kembali saat import.`
        ))
    )
      return;
    setBusy(true);
    setError("");
    setReport(null);
    try {
      const response = await apiFetch(
        `/admin/master/school-locations/import/${apply ? "apply" : "preview"}`,
        { method: "POST" }
      );
      const payload = await response.json();
      if (!response.ok)
        throw new Error(
          payload.message ||
            "Import gagal diproses. Silakan ulangi Preview Import."
        );
      setReport(payload.data);
      setApplied(apply);
      if (apply) {
        if (payload.data.imported > 0) onImported();
        await Promise.all([
          client.invalidateQueries({ queryKey: ["school-locations"] }),
          client.invalidateQueries({ queryKey: ["school-location"] }),
        ]);
      }
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Gagal memproses data awal lokasi."
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-6 space-y-4 rounded-xl border border-slate-200 bg-slate-50 p-4">
      <div>
        <h3 className="font-semibold text-slate-900">
          Data Awal Lokasi Sekolah
        </h3>
        <p className="mt-1 text-sm text-slate-600">
          Fitur ini digunakan untuk mengimpor data awal koordinat sekolah dari
          file referensi. Data yang sudah pernah diubah secara manual tidak akan
          ditimpa.
        </p>
        <p className="mt-2 text-xs text-slate-500">
          Sumber: data-sekolah-map-bangkalan.xlsx pada server. Preview tidak
          mengubah database.
        </p>
      </div>
      <div className="flex flex-wrap gap-3">
        <Button disabled={busy} onClick={() => run(false)}>
          {busy ? "Memproses..." : "Preview Import"}
        </Button>
        <Button
          disabled={busy || !report || !report.eligible || applied}
          onClick={() => run(true)}
        >
          Import Data Awal
        </Button>
      </div>
      <div aria-live="polite">
        {error && (
          <p role="alert" className="text-sm text-red-700">
            {error}
          </p>
        )}
        {!report && !busy && !error && (
          <p className="text-sm text-slate-600">
            Belum ada preview. Klik Preview Import untuk memeriksa data.
          </p>
        )}
        {report && (
          <>
            {applied && (
              <p className="mb-3 font-medium text-green-800">
                Berhasil mengimpor {report.imported} lokasi sekolah.{" "}
                {report.skipped_existing} lokasi existing dan{" "}
                {report.skipped_manual} lokasi manual/audit dilewati.
              </p>
            )}
            {!applied && report.eligible === 0 && (
              <p className="mb-3 text-sm">
                Tidak ada data lokasi baru yang dapat diimpor.
              </p>
            )}
            <dl className="grid gap-2 text-sm sm:grid-cols-2">
              {Object.entries(labels).map(([key, label]) => (
                <div
                  key={key}
                  className="flex justify-between gap-4 rounded bg-white p-2"
                >
                  <dt>{label}</dt>
                  <dd className="font-semibold">
                    {report[key as keyof typeof labels]}
                  </dd>
                </div>
              ))}
            </dl>
            {report.errors.map((message, index) => (
              <p key={index} role="alert">
                {message}
              </p>
            ))}
            {!applied && report.rows.length > 0 && (
              <div className="mt-4 overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <caption className="mb-2 text-left text-xs text-slate-500">
                    Preview maksimal 50 lokasi yang siap diimpor
                  </caption>
                  <thead>
                    <tr>
                      {["NPSN", "Nama Sekolah", "Latitude", "Longitude"].map(
                        (label) => (
                          <th key={label} className="p-2">
                            {label}
                          </th>
                        )
                      )}
                    </tr>
                  </thead>
                  <tbody>
                    {report.rows.map((row) => (
                      <tr key={row.npsn} className="border-t border-slate-200">
                        <td className="p-2">{row.npsn}</td>
                        <td className="p-2">{row.name}</td>
                        <td className="p-2">{row.latitude}</td>
                        <td className="p-2">{row.longitude}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
