"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { formatIndoDateTime } from "@/lib/date";

import { Card, CardContent } from "@/components/ui/card";
import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { TypeaheadInput } from "@/components/typeahead-input";
import { PasswordInput } from "@/components/password-input";

type HistoryItem = {
  id: number;
  schoolId: number;
  schoolName: string;
  userId: number;
  uploaderName: string;
  type: string;
  originalFilename: string;
  status: string;
  createdAt: string;
  deletedAt?: string | null;
  deletedByName?: string | null;
  accepted: number;
  rejected: number;
};

type HistoryResponse = {
  data: HistoryItem[];
  meta: {
    page: number;
    pageSize: number;
    total: number;
  };
};

type SchoolSuggestion = {
  id: number;
  nama_sekolah: string;
  npsn?: string | null;
};

type FilterState = {
  from: string;
  to: string;
  schoolId: number | null;
};

type HistoryColumn = {
  key: string;
  label: string;
  align?: "left" | "right" | "center";
};

const formatNumber = (value: number) =>
  new Intl.NumberFormat("id-ID").format(value);

const moduleValue = "PRESTASI";

const fetchHistory = async (params: {
  page: number;
  pageSize: number;
  from: string;
  to: string;
  schoolId?: number | null;
}): Promise<HistoryResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  query.set("module", moduleValue);
  if (params.from) {
    query.set("from", params.from);
  }
  if (params.to) {
    query.set("to", params.to);
  }
  if (params.schoolId) {
    query.set("school_id", String(params.schoolId));
  }

  const response = await apiFetch(`/api/histori?${query.toString()}`);
  if (!response.ok) {
    throw new Error("Failed to fetch history list.");
  }
  const payload = (await response.json()) as { data: HistoryResponse };
  return payload.data;
};

const fetchSchoolSuggestions = async (
  query: string
): Promise<SchoolSuggestion[]> => {
  const params = new URLSearchParams();
  params.set("q", query);
  const response = await apiFetch(`/api/admin/schools/suggest?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch school suggestions.");
  }
  const payload = (await response.json()) as { data: SchoolSuggestion[] };
  return payload.data ?? [];
};

export default function PrestasiHistoriPage() {
  const { status, user } = useAuth();
  const isAdminMaster = user?.role === "ADMIN_MASTER";
  const isAdminCabdin = user?.role === "ADMIN";
  const isAdmin = isAdminCabdin || isAdminMaster;
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const canDelete = isSchoolAdmin || isAdminMaster;
  const [isMounted, setIsMounted] = useState(false);

  useEffect(() => {
    setIsMounted(true);
  }, []);

  const isAuthReady = isMounted && status !== "loading";
  const isAllowed =
    isAuthReady && status === "authenticated" && (isAdmin || isSchoolAdmin);

  const [filters, setFilters] = useState<FilterState>({
    from: "",
    to: "",
    schoolId: null,
  });
  const [schoolQuery, setSchoolQuery] = useState("");
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const updateFilters = (next: Partial<FilterState>) => {
    setFilters((prev) => ({ ...prev, ...next }));
    setPage(1);
  };

  const debouncedSchoolQuery = useDebouncedValue(schoolQuery, 300);
  const { data: schoolSuggestions = [], isLoading: isSchoolLoading } = useQuery({
    queryKey: ["prestasi-histori-school-suggest", debouncedSchoolQuery],
    queryFn: () => fetchSchoolSuggestions(debouncedSchoolQuery),
    enabled: isAllowed && isAdmin && debouncedSchoolQuery.trim().length >= 2,
  });

  const {
    data: historyData,
    isLoading: isHistoryLoading,
    isError: isHistoryError,
    refetch: refetchHistory,
  } = useQuery({
    queryKey: [
      "prestasi-histori-list",
      filters.from,
      filters.to,
      filters.schoolId,
      page,
    ],
    queryFn: () =>
      fetchHistory({
        page,
        pageSize,
        from: filters.from,
        to: filters.to,
        schoolId: isAdmin ? filters.schoolId : null,
      }),
    enabled: isAllowed,
  });

  const tableRows = historyData?.data ?? [];
  const tableMeta = historyData?.meta ?? {
    page,
    pageSize,
    total: 0,
  };

  const columns = useMemo(() => {
    const base: HistoryColumn[] = [
      { key: "createdAt", label: "Tanggal/Waktu" },
    ];
    if (isAdmin) {
      base.push({ key: "schoolName", label: "Sekolah" });
    }
    base.push(
      { key: "uploaderName", label: "Pengunggah" },
      { key: "originalFilename", label: "Nama File" },
      { key: "accepted", label: "Accepted", align: "right" as const },
      { key: "rejected", label: "Rejected", align: "right" as const },
      { key: "actions", label: "Aksi", align: "center" as const }
    );
    return base;
  }, [isAdmin]);

  const [detailTarget, setDetailTarget] = useState<HistoryItem | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<HistoryItem | null>(null);
  const [deletePassword, setDeletePassword] = useState("");
  const [deleteError, setDeleteError] = useState("");
  const [isDeleting, setIsDeleting] = useState(false);

  const closeDeleteModal = () => {
    setDeleteTarget(null);
    setDeletePassword("");
    setDeleteError("");
    setIsDeleting(false);
  };

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    if (!deletePassword.trim()) {
      setDeleteError("Sandi wajib diisi.");
      return;
    }

    setIsDeleting(true);
    setDeleteError("");
    try {
      const response = await apiFetch(`/api/histori/${deleteTarget.id}`, {
        method: "DELETE",
        body: JSON.stringify({ password: deletePassword }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        const message = payload?.message ?? "";
        if (message === "PASSWORD_SALAH" || message === "PASSWORD_INVALID") {
          setDeleteError("Sandi salah");
          return;
        }
        if (message === "PASSWORD_REQUIRED") {
          setDeleteError("Sandi wajib diisi.");
          return;
        }
        if (response.status === 403) {
          setDeleteError("Tidak diizinkan menghapus batch ini.");
          return;
        }
        setDeleteError("Gagal menghapus batch import.");
        return;
      }

      closeDeleteModal();
      void refetchHistory();
    } catch {
      setDeleteError("Gagal menghapus batch import.");
    } finally {
      setIsDeleting(false);
    }
  };

  if (!isMounted || status === "loading") {
    return (
      <div className="rounded-xl border border-dashed border-zinc-200 bg-white p-6 text-sm text-zinc-500">
        Memuat...
      </div>
    );
  }

  if (!isAllowed) {
    return (
      <div className="rounded-xl border border-dashed border-zinc-200 bg-white p-6 text-sm text-zinc-500">
        Halaman ini hanya tersedia untuk admin sekolah, admin cabdin, atau
        admin master.
      </div>
    );
  }

  return (
    <section className="space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Cabang Dinas Pendidikan Wilayah Bangkalan
        </p>
        <h1 className="text-3xl font-semibold">
          Histori Import Data Prestasi
        </h1>
        <p className="text-sm text-zinc-600">
          Kelola riwayat import Excel prestasi dan hapus batch import jika ada
          data yang salah.
        </p>
      </header>

      <Card>
        <CardContent className="space-y-3">
          <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-end">
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Dari
              </label>
              <input
                type="date"
                value={filters.from}
                onChange={(event) => updateFilters({ from: event.target.value })}
                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-44"
              />
            </div>

            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Sampai
              </label>
              <input
                type="date"
                value={filters.to}
                onChange={(event) => updateFilters({ to: event.target.value })}
                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-44"
              />
            </div>

            {isAdmin ? (
              <div className="space-y-1 md:min-w-[240px]">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Sekolah
                </label>
                <TypeaheadInput
                  value={schoolQuery}
                  onValueChange={(value) => {
                    setSchoolQuery(value);
                    updateFilters({ schoolId: null });
                  }}
                  items={schoolSuggestions.map((school) => ({
                    key: school.id,
                    label: school.nama_sekolah,
                    value: school.nama_sekolah,
                    subLabel: school.npsn ?? undefined,
                  }))}
                  onSelect={(item) => {
                    const selected = schoolSuggestions.find(
                      (school) => school.id === item.key
                    );
                    if (selected) {
                      setSchoolQuery(selected.nama_sekolah);
                      updateFilters({ schoolId: selected.id });
                    }
                  }}
                  placeholder="Ketik nama sekolah..."
                  emptyText="Sekolah tidak ditemukan"
                  isLoading={isSchoolLoading}
                  showClear={Boolean(filters.schoolId || schoolQuery)}
                  onClear={() => {
                    setSchoolQuery("");
                    updateFilters({ schoolId: null });
                  }}
                />
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <DataTable
            columns={columns}
            rows={tableRows}
            renderRow={(row) => (
              <TableRow key={row.id}>
                <TableCell>
                  {formatIndoDateTime(row.createdAt) ?? "-"}
                </TableCell>
                {isAdmin ? (
                  <TableCell>{row.schoolName || "-"}</TableCell>
                ) : null}
                <TableCell>{row.uploaderName || "-"}</TableCell>
                <TableCell>{row.originalFilename || "-"}</TableCell>
                <TableCell className="text-right">
                  {formatNumber(row.accepted)}
                </TableCell>
                <TableCell className="text-right">
                  {formatNumber(row.rejected)}
                </TableCell>
                <TableCell className="text-center">
                  <div className="flex flex-col items-center justify-center gap-2 sm:flex-row">
                    <button
                      type="button"
                      onClick={() => setDetailTarget(row)}
                      className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                    >
                      Detail
                    </button>
                    {canDelete ? (
                      row.deletedAt ? (
                        <span className="rounded-md border border-zinc-200 bg-zinc-50 px-3 py-1.5 text-xs font-semibold text-zinc-500">
                          Terhapus
                        </span>
                      ) : (
                        <button
                          type="button"
                          onClick={() => {
                            setDeleteTarget(row);
                            setDeletePassword("");
                            setDeleteError("");
                          }}
                          className="rounded-md border border-rose-200 bg-rose-50 px-3 py-1.5 text-xs font-semibold text-rose-700 transition hover:bg-rose-100"
                        >
                          Hapus
                        </button>
                      )
                    ) : null}
                  </div>
                </TableCell>
              </TableRow>
            )}
            isLoading={isHistoryLoading}
            isError={isHistoryError}
            emptyMessage="Belum ada histori import data."
            page={tableMeta.page}
            pageSize={tableMeta.pageSize}
            total={tableMeta.total}
            onPageChange={setPage}
          />
        </CardContent>
      </Card>

      {detailTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-lg rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Detail Batch Import
              </h2>
              <p className="text-sm text-zinc-600">
                Ringkasan detail import yang dipilih.
              </p>
            </div>
            <div className="mt-4 space-y-2 rounded-md border border-zinc-200 bg-zinc-50 p-3 text-sm text-zinc-700">
              <div>
                <span className="font-semibold">Tanggal/Waktu:</span>{" "}
                {formatIndoDateTime(detailTarget.createdAt) ?? "-"}
              </div>
              {isAdmin ? (
                <div>
                  <span className="font-semibold">Sekolah:</span>{" "}
                  {detailTarget.schoolName || "-"}
                </div>
              ) : null}
              <div>
                <span className="font-semibold">Pengunggah:</span>{" "}
                {detailTarget.uploaderName || "-"}
              </div>
              <div>
                <span className="font-semibold">Nama File:</span>{" "}
                {detailTarget.originalFilename || "-"}
              </div>
              {detailTarget.deletedAt ? (
                <>
                  <div>
                    <span className="font-semibold">Dihapus:</span>{" "}
                    {formatIndoDateTime(detailTarget.deletedAt) ?? "-"}
                  </div>
                  <div>
                    <span className="font-semibold">Dihapus Oleh:</span>{" "}
                    {detailTarget.deletedByName || "-"}
                  </div>
                </>
              ) : null}
              <div>
                <span className="font-semibold">Accepted:</span>{" "}
                {formatNumber(detailTarget.accepted)}
              </div>
              <div>
                <span className="font-semibold">Rejected:</span>{" "}
                {formatNumber(detailTarget.rejected)}
              </div>
            </div>
            <div className="mt-5 flex justify-end">
              <button
                type="button"
                onClick={() => setDetailTarget(null)}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95"
              >
                Tutup
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {deleteTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-lg rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Hapus Import Ini?
              </h2>
              <p className="text-sm text-zinc-600">
                Aksi ini menghapus PERMANEN semua data hasil import pada batch
                ini. Data dari import lain tidak terpengaruh.
              </p>
            </div>
            <div className="mt-4 space-y-2 rounded-md border border-zinc-200 bg-zinc-50 p-3 text-sm text-zinc-700">
              {isAdmin ? (
                <div>
                  <span className="font-semibold">Sekolah:</span>{" "}
                  {deleteTarget.schoolName || "-"}
                </div>
              ) : null}
              <div>
                <span className="font-semibold">Nama File:</span>{" "}
                {deleteTarget.originalFilename || "-"}
              </div>
              <div>
                <span className="font-semibold">Accepted:</span>{" "}
                {formatNumber(deleteTarget.accepted)}
              </div>
            </div>
            <div className="mt-4 space-y-2">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Masukkan sandi akun untuk konfirmasi
              </label>
              <PasswordInput
                value={deletePassword}
                onChange={(event) => setDeletePassword(event.target.value)}
                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                placeholder="Sandi akun"
                required
              />
              {deleteError ? (
                <p className="text-sm text-red-600">{deleteError}</p>
              ) : null}
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={closeDeleteModal}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleDeleteConfirm}
                disabled={!deletePassword.trim() || isDeleting}
                className="rounded-md bg-rose-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-rose-500 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isDeleting ? "Menghapus..." : "Konfirmasi Hapus"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}

