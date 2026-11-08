"use client";

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { TypeaheadInput } from "@/components/typeahead-input";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { getErrorMessageFromResponse } from "@/lib/http";
import { invalidatePTDerivedQueries } from "@/lib/invalidate-pt-queries";

type SchoolItem = {
  id: number;
  namaSekolah: string;
  npsnSekolah?: string | null;
};
type OfficialCoverage = {
  graduationYear: number;
  path: string;
  officialCount: number;
  detailCount: number;
  missingCount: number;
  coveragePercent: number;
  exceedsOfficial: boolean;
  sourceNote: string;
};
export type AdminPTEditableOffer = {
  id: number;
  schoolId: number;
  schoolName: string;
  graduationYear: number;
  path: string;
  studentName: string;
  nisn: string;
  campusName: string;
  studyProgram: string;
  sourceRole: string;
  conflictStatus: string;
  announcementStatus: string;
};

const inputClass =
  "w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm";

function AdminPTModal({
  open,
  title,
  onClose,
  children,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const closeRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    closeRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose, open]);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="admin-pt-modal-title"
        className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-xl bg-white shadow-xl"
      >
        <div className="sticky top-0 flex items-center justify-between border-b border-zinc-200 bg-white p-4">
          <h2 id="admin-pt-modal-title" className="text-lg font-semibold">
            {title}
          </h2>
          <button
            ref={closeRef}
            type="button"
            aria-label="Tutup dialog"
            onClick={onClose}
            className="rounded-md border border-zinc-300 px-3 py-1 text-sm font-semibold"
          >
            Tutup
          </button>
        </div>
        <div className="p-4 sm:p-6">{children}</div>
      </div>
    </div>
  );
}

export default function AdminPTEntryPanel({
  schools,
  onSuccess,
  editingOffer,
  onEditingChange,
}: {
  schools: SchoolItem[];
  onSuccess: () => void;
  editingOffer: AdminPTEditableOffer | null;
  onEditingChange: (offer: AdminPTEditableOffer | null) => void;
}) {
  const queryClient = useQueryClient();
  const currentYear = new Date().getFullYear();
  const [form, setForm] = useState({
    schoolId: 0,
    schoolName: "",
    graduationYear: String(currentYear),
    path: "SNBP",
    studentName: "",
    nisn: "",
    campusName: "",
    studyProgram: "",
    announcementStatus: "DITERIMA",
  });
  const [campusQuery, setCampusQuery] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [rejectedDownload, setRejectedDownload] = useState<{
    url: string;
    filename: string;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const [manualOpen, setManualOpen] = useState(false);
  const [officialOpen, setOfficialOpen] = useState(false);
  const [coverageYear, setCoverageYear] = useState(currentYear);
  const [officialForm, setOfficialForm] = useState({
    path: "SNBP",
    officialCount: "",
    sourceNote: "",
  });

  const schoolSuggestions = useMemo(
    () =>
      schools
        .filter((item) =>
          item.namaSekolah.toLowerCase().includes(form.schoolName.toLowerCase())
        )
        .slice(0, 30)
        .map((item) => ({
          key: item.id,
          value: item.namaSekolah,
          label: item.namaSekolah,
          subLabel: item.npsnSekolah ?? undefined,
        })),
    [form.schoolName, schools]
  );

  const { data: campusSuggestions = [] } = useQuery({
    queryKey: ["admin-pt-campus-suggest", campusQuery],
    enabled: campusQuery.trim().length >= 2,
    queryFn: async () => {
      const response = await apiFetch(
        `/api/universities/suggest?q=${encodeURIComponent(campusQuery)}`
      );
      if (!response.ok) return [];
      const payload = await response.json();
      const rows = Array.isArray(payload?.data) ? payload.data : [];
      return rows.map((item: { nama: string }) => ({
        key: item.nama,
        value: item.nama,
        label: item.nama,
      }));
    },
  });

  const { data: coverage = [], refetch: refetchCoverage } = useQuery({
    queryKey: ["pt-official-coverage", coverageYear],
    queryFn: async (): Promise<OfficialCoverage[]> => {
      const response = await apiFetch(
        `/api/siswa/acceptance-offers/official-counts?year=${coverageYear}`
      );
      if (!response.ok) return [];
      const payload = await response.json();
      return Array.isArray(payload?.data) ? payload.data : [];
    },
  });
  useEffect(() => {
    if (!editingOffer) return;
    setCampusQuery(editingOffer.campusName);
    setForm({
      schoolId: editingOffer.schoolId,
      schoolName: editingOffer.schoolName,
      graduationYear: String(editingOffer.graduationYear),
      path: editingOffer.path,
      studentName: editingOffer.studentName,
      nisn: editingOffer.nisn,
      campusName: editingOffer.campusName,
      studyProgram: editingOffer.studyProgram,
      announcementStatus: editingOffer.announcementStatus,
    });
    setManualOpen(true);
  }, [editingOffer]);

  const manualDirty = Boolean(
    form.studentName || form.nisn || form.campusName || form.studyProgram
  );
  const closeManual = () => {
    if (busy) return;
    if (
      manualDirty &&
      !window.confirm("Perubahan belum disimpan. Tutup formulir?")
    )
      return;
    setManualOpen(false);
    onEditingChange(null);
    setError("");
  };
  const closeOfficial = () => {
    if (busy) return;
    if (
      (officialForm.officialCount || officialForm.sourceNote) &&
      !window.confirm("Perubahan belum disimpan. Tutup formulir?")
    )
      return;
    setOfficialOpen(false);
    setError("");
  };
  const submitManual = async () => {
    setBusy(true);
    setError("");
    setMessage("");
    setRejectedDownload(null);
    try {
      const response = await apiFetch(
        editingOffer
          ? `/api/siswa/acceptance-offers/admin/${editingOffer.id}`
          : "/api/siswa/acceptance-offers/admin",
        {
          method: editingOffer ? "PUT" : "POST",
          body: JSON.stringify({
            schoolId: form.schoolId,
            graduationYear: Number(form.graduationYear),
            path: form.path,
            studentName: form.studentName,
            nisn: form.nisn,
            campusName: form.campusName,
            studyProgram: form.studyProgram,
            announcementStatus: form.announcementStatus,
          }),
        }
      );
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(response, "Gagal menyimpan data.")
        );
      }
      setMessage(
        editingOffer
          ? "Data Pengumuman PT berhasil diperbarui."
          : "Data Pengumuman PT berhasil disimpan."
      );
      onEditingChange(null);
      setForm((prev) => ({
        ...prev,
        studentName: "",
        nisn: "",
        campusName: "",
        studyProgram: "",
        announcementStatus: "DITERIMA",
      }));
      setCampusQuery("");
      setManualOpen(false);
      onSuccess();
      await invalidatePTDerivedQueries(queryClient);
      void refetchCoverage();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan data.");
    } finally {
      setBusy(false);
    }
  };

  const importFile = async (file: File) => {
    setBusy(true);
    setError("");
    setMessage("");
    setRejectedDownload(null);
    try {
      const body = new FormData();
      body.set("file", file);
      const response = await apiFetch(
        "/api/siswa/acceptance-offers/admin/import",
        { method: "POST", body }
      );
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(response, "Import gagal.")
        );
      }
      const payload = await response.json();
      const result = payload?.data ?? {};
      const rejectedUrl =
        typeof result.rejectedExcelUrl === "string"
          ? result.rejectedExcelUrl
          : "";
      if (rejectedUrl) {
        setRejectedDownload({
          url: rejectedUrl,
          filename:
            typeof result.rejectedExcelFilename === "string"
              ? result.rejectedExcelFilename
              : "rejected-admin-pengumuman-pt.xlsx",
        });
      }
      setMessage(
        `Import selesai: ${result.accepted ?? 0} berhasil, ${result.rejected ?? 0} ditolak.`
      );
      onSuccess();
      await invalidatePTDerivedQueries(queryClient);
      void refetchCoverage();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Import gagal.");
    } finally {
      setBusy(false);
    }
  };

  const saveOfficial = async () => {
    setBusy(true);
    setError("");
    try {
      const response = await apiFetch(
        "/api/siswa/acceptance-offers/official-counts",
        {
          method: "PUT",
          body: JSON.stringify({
            graduationYear: coverageYear,
            path: officialForm.path,
            officialCount: Number(officialForm.officialCount),
            sourceNote: officialForm.sourceNote,
            sourceUpdatedAt: new Date().toISOString().slice(0, 10),
          }),
        }
      );
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(
            response,
            "Gagal menyimpan jumlah resmi."
          )
        );
      }
      setMessage("Jumlah resmi penerimaan berhasil diperbarui.");
      await refetchCoverage();
      await invalidatePTDerivedQueries(queryClient);
      onSuccess();
      setOfficialForm((prev) => ({
        ...prev,
        officialCount: "",
        sourceNote: "",
      }));
      setOfficialOpen(false);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal menyimpan jumlah resmi."
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <CardContent className="space-y-6 p-5">
        <div>
          <h2 className="text-lg font-semibold text-zinc-900">Aksi Modul</h2>
          <p className="text-sm text-zinc-600">
            Formulir tambah data dan jumlah resmi dibuka melalui pop-up.
          </p>
        </div>
        {message ? (
          <p className="rounded-md bg-emerald-50 p-3 text-sm text-emerald-700">
            {message}
          </p>
        ) : null}
        {error ? (
          <p className="rounded-md bg-red-50 p-3 text-sm text-red-700">
            {error}
          </p>
        ) : null}
        {rejectedDownload ? (
          <a
            href={rejectedDownload.url}
            download={rejectedDownload.filename}
            className="inline-flex rounded-md border border-amber-300 bg-amber-50 px-4 py-2 text-sm font-semibold text-amber-900"
          >
            Unduh file baris yang ditolak
          </a>
        ) : null}
        <div className="flex flex-wrap gap-3">
          <a
            href="/api/siswa/acceptance-offers/admin/template.xlsx"
            className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-semibold"
          >
            Unduh Template Admin
          </a>
          <label className="cursor-pointer rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white">
            Import Excel Admin
            <input
              type="file"
              accept=".xlsx,.xlsm"
              className="hidden"
              disabled={busy}
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) void importFile(file);
                event.currentTarget.value = "";
              }}
            />
          </label>
          <button
            type="button"
            onClick={() => setManualOpen(true)}
            className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-semibold"
          >
            Tambah Data Satu per Satu
          </button>
          <button
            type="button"
            onClick={() => setOfficialOpen(true)}
            className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-semibold"
          >
            Isi Jumlah Resmi
          </button>
        </div>
        <div className="border-t border-zinc-200 pt-5">
          <h3 className="font-semibold text-zinc-900">
            Jumlah Resmi & Cakupan Detail Jalur
          </h3>
          <div className="mt-4 grid gap-3 md:grid-cols-2">
            {coverage.map((item) => (
              <div
                key={`${item.graduationYear}-${item.path}`}
                className={`rounded-lg border p-4 ${item.exceedsOfficial ? "border-amber-300 bg-amber-50" : "border-zinc-200"}`}
              >
                <p className="font-semibold">
                  {item.path} {item.graduationYear}
                </p>
                <p className="mt-1 text-sm">
                  Resmi: {item.officialCount} · Detail terinput:{" "}
                  {item.detailCount} · Belum terinput: {item.missingCount}
                </p>
                <p className="text-sm">
                  Cakupan: {item.coveragePercent.toFixed(1)}%
                </p>
                {item.exceedsOfficial ? (
                  <p className="mt-1 text-sm font-medium text-amber-800">
                    Detail terinput melebihi angka resmi dan perlu diperiksa.
                  </p>
                ) : null}
              </div>
            ))}
          </div>
        </div>
        <AdminPTModal
          open={manualOpen}
          title={
            editingOffer
              ? "Edit Data Pengumuman PT"
              : "Tambah Data Satu per Satu"
          }
          onClose={closeManual}
        >
          <div className="space-y-4">
            {error ? (
              <p className="rounded-md bg-red-50 p-3 text-sm text-red-700">
                {error}
              </p>
            ) : null}
            <div className="grid gap-4 md:grid-cols-2">
              <label className="space-y-1 text-sm font-medium">
                Sekolah
                <TypeaheadInput
                  value={form.schoolName}
                  onValueChange={(value) =>
                    setForm((prev) => ({
                      ...prev,
                      schoolName: value,
                      schoolId: 0,
                    }))
                  }
                  items={schoolSuggestions}
                  onSelect={(item) =>
                    setForm((prev) => ({
                      ...prev,
                      schoolId: Number(item.key),
                      schoolName: item.value,
                    }))
                  }
                  placeholder="Cari nama sekolah"
                  minChars={1}
                />
              </label>
              <label className="space-y-1 text-sm font-medium">
                Tahun lulus
                <input
                  className={inputClass}
                  type="number"
                  value={form.graduationYear}
                  onChange={(e) =>
                    setForm((p) => ({ ...p, graduationYear: e.target.value }))
                  }
                />
              </label>
              <label className="space-y-1 text-sm font-medium">
                Jalur
                <select
                  className={inputClass}
                  value={form.path}
                  onChange={(e) =>
                    setForm((p) => ({ ...p, path: e.target.value }))
                  }
                >
                  {["SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"].map(
                    (item) => (
                      <option key={item}>{item}</option>
                    )
                  )}
                </select>
              </label>
              <label className="space-y-1 text-sm font-medium">
                Status pengumuman
                <select
                  className={inputClass}
                  value={form.announcementStatus}
                  onChange={(e) =>
                    setForm((p) => ({
                      ...p,
                      announcementStatus: e.target.value,
                    }))
                  }
                >
                  <option value="DITERIMA">DITERIMA</option>
                  <option value="DITOLAK">DITOLAK</option>
                </select>
              </label>
              <label className="space-y-1 text-sm font-medium">
                Nama siswa
                <input
                  className={inputClass}
                  value={form.studentName}
                  onChange={(e) =>
                    setForm((p) => ({ ...p, studentName: e.target.value }))
                  }
                />
              </label>
              <label className="space-y-1 text-sm font-medium">
                NISN (opsional)
                <input
                  className={inputClass}
                  inputMode="numeric"
                  value={form.nisn}
                  onChange={(e) =>
                    setForm((p) => ({ ...p, nisn: e.target.value }))
                  }
                />
              </label>
              <label className="space-y-1 text-sm font-medium">
                Perguruan tinggi
                <TypeaheadInput
                  value={campusQuery || form.campusName}
                  onValueChange={(value) => {
                    setCampusQuery(value);
                    setForm((prev) => ({ ...prev, campusName: "" }));
                  }}
                  items={campusSuggestions}
                  onSelect={(item) => {
                    setCampusQuery(item.value);
                    setForm((prev) => ({ ...prev, campusName: item.value }));
                  }}
                  placeholder="Cari perguruan tinggi"
                />
              </label>
              <label className="space-y-1 text-sm font-medium">
                Program studi/jurusan
                <input
                  className={inputClass}
                  value={form.studyProgram}
                  onChange={(e) =>
                    setForm((p) => ({ ...p, studyProgram: e.target.value }))
                  }
                />
              </label>
            </div>
            <div className="flex justify-end gap-3">
              <button
                type="button"
                disabled={busy}
                onClick={closeManual}
                className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-semibold"
              >
                Batal
              </button>
              <button
                type="button"
                disabled={
                  busy ||
                  !form.schoolId ||
                  !form.campusName ||
                  !form.studentName ||
                  !form.studyProgram
                }
                onClick={() => void submitManual()}
                className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50"
              >
                {busy
                  ? "Memproses..."
                  : editingOffer
                    ? "Simpan Perubahan"
                    : "Tambah Data"}
              </button>
            </div>
          </div>
        </AdminPTModal>
        <AdminPTModal
          open={officialOpen}
          title="Isi Jumlah Resmi"
          onClose={closeOfficial}
        >
          <div className="space-y-4">
            {error ? (
              <p className="rounded-md bg-red-50 p-3 text-sm text-red-700">
                {error}
              </p>
            ) : null}
            <div className="grid gap-4 md:grid-cols-2">
              <label className="space-y-1 text-sm font-medium">
                Tahun
                <input
                  className={inputClass}
                  type="number"
                  value={coverageYear}
                  onChange={(e) => setCoverageYear(Number(e.target.value))}
                />
              </label>
              <label className="space-y-1 text-sm font-medium">
                Jalur
                <select
                  className={inputClass}
                  value={officialForm.path}
                  onChange={(e) =>
                    setOfficialForm((p) => ({ ...p, path: e.target.value }))
                  }
                >
                  {["SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"].map(
                    (item) => (
                      <option key={item}>{item}</option>
                    )
                  )}
                </select>
              </label>
              <label className="space-y-1 text-sm font-medium">
                Jumlah resmi
                <input
                  className={inputClass}
                  type="number"
                  min={0}
                  value={officialForm.officialCount}
                  onChange={(e) =>
                    setOfficialForm((p) => ({
                      ...p,
                      officialCount: e.target.value,
                    }))
                  }
                />
              </label>
              <label className="space-y-1 text-sm font-medium">
                Sumber/keterangan
                <input
                  className={inputClass}
                  value={officialForm.sourceNote}
                  onChange={(e) =>
                    setOfficialForm((p) => ({
                      ...p,
                      sourceNote: e.target.value,
                    }))
                  }
                />
              </label>
            </div>
            <div className="flex justify-end gap-3">
              <button
                type="button"
                disabled={busy}
                onClick={closeOfficial}
                className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-semibold"
              >
                Batal
              </button>
              <button
                type="button"
                disabled={busy || officialForm.officialCount === ""}
                onClick={() => void saveOfficial()}
                className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50"
              >
                {busy ? "Menyimpan..." : "Simpan Jumlah Resmi"}
              </button>
            </div>
          </div>
        </AdminPTModal>
      </CardContent>
    </Card>
  );
}
