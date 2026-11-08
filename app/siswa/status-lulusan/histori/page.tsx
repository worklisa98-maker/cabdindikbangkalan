"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import { Card, CardContent } from "@/components/ui/card";
import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";

type ImportHistoryItem = {
  id: number;
  schoolId?: number | null;
  schoolName?: string;
  schoolLevel?: string;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  finalStatus: string;
  finalPath: string;
  changedAt: string;
  changedBy: string;
};

type ImportHistoryResponse = {
  data: ImportHistoryItem[];
  meta: {
    page: number;
    pageSize: number;
    total: number;
  };
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

const formatDateTime = (value?: string | null) => {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
  return parsed.toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
};

const fetchImportHistory = async (params: {
  page: number;
  pageSize: number;
  year: string;
  search: string;
}): Promise<ImportHistoryResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.year.trim()) query.set("year", params.year.trim());
  if (params.search.trim()) query.set("q", params.search.trim());

  const response = await apiFetch(
    `/api/siswa/outcomes/import-history?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch import history.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const rows = Array.isArray(data?.data) ? data.data : [];
  const meta = isRecord(data?.meta) ? data.meta : {};

  return {
    data: rows
      .map((row: unknown) => {
        if (!isRecord(row)) return null;
        return {
          id: readNumber(row.id),
          schoolId: readNumber(row.school_id ?? row.schoolId) || null,
          schoolName: readString(row.school_name ?? row.schoolName),
          schoolLevel: readString(row.school_level ?? row.schoolLevel),
          nisn: readString(row.nisn),
          namaSiswa: readString(row.nama_siswa ?? row.namaSiswa),
          rombel: readString(row.rombel),
          tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
          finalStatus: readString(row.final_status ?? row.finalStatus),
          finalPath: readString(row.final_path ?? row.finalPath),
          changedAt: readString(row.changed_at ?? row.changedAt),
          changedBy: readString(row.changed_by ?? row.changedBy),
        } as ImportHistoryItem;
      })
      .filter((row: ImportHistoryItem | null): row is ImportHistoryItem =>
        Boolean(row && row.id)
      ),
    meta: {
      page: readNumber(meta.page) || params.page,
      pageSize: readNumber(meta.pageSize) || params.pageSize,
      total: readNumber(meta.total),
    },
  };
};

export default function StatusLulusanHistoriPage() {
  const { status, user } = useAuth();
  const isAdmin = user?.role === "ADMIN" || user?.role === "ADMIN_MASTER";
  const [filters, setFilters] = useState({
    year: "",
    search: "",
  });
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const updateFilters = (next: Partial<typeof filters>) => {
    setFilters((prev) => ({ ...prev, ...next }));
    setPage(1);
  };

  const debouncedSearch = useDebouncedValue(filters.search, 300);

  const {
    data: historyData,
    isLoading,
    isError,
  } = useQuery({
    queryKey: [
      "status-lulusan-import-history",
      filters.year,
      debouncedSearch,
      page,
    ],
    queryFn: () =>
      fetchImportHistory({
        page,
        pageSize,
        year: filters.year,
        search: debouncedSearch,
      }),
    enabled: status === "authenticated",
  });

  const tableRows = historyData?.data ?? [];
  const tableMeta = historyData?.meta ?? {
    page,
    pageSize,
    total: 0,
  };

  const columns = useMemo(() => {
    const base = [
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "tahun", label: "Tahun Lulus", align: "right" as const },
      { key: "status", label: "Status" },
      { key: "jalur", label: "Jalur" },
      { key: "changedBy", label: "Diubah Oleh" },
      { key: "changedAt", label: "Waktu" },
    ];
    if (isAdmin) {
      return [
        { key: "sekolah", label: "Sekolah" },
        { key: "jenjang", label: "Jenjang" },
        ...base,
      ];
    }
    return base;
  }, [isAdmin]);

  return (
    <AdminRouteGuard>
      <section className="space-y-6">
        <header className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Status Lulusan
          </p>
          <h1 className="text-3xl font-semibold text-zinc-900">Histori Import</h1>
          <p className="text-sm text-zinc-600">
            Riwayat perubahan status lulusan yang berasal dari proses import.
          </p>
        </header>

        <Card>
          <CardContent className="grid gap-3 md:grid-cols-2">
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Tahun Lulus
              </label>
              <input
                type="number"
                value={filters.year}
                onChange={(event) => updateFilters({ year: event.target.value })}
                placeholder="Semua"
                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Cari Siswa
              </label>
              <input
                type="text"
                value={filters.search}
                onChange={(event) => updateFilters({ search: event.target.value })}
                placeholder="Cari NISN atau nama siswa"
                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="space-y-4">
            <DataTable
              columns={columns}
              rows={tableRows}
              isLoading={isLoading}
              isError={isError}
              emptyMessage="Tidak ada data"
              tableWrapperClassName="max-h-[520px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
              tableHeaderClassName="sticky top-0 z-10 bg-white"
              page={tableMeta.page}
              pageSize={tableMeta.pageSize}
              total={tableMeta.total}
              onPageChange={setPage}
              renderRow={(row) => (
                <TableRow key={row.id}>
                  {isAdmin ? (
                    <>
                      <TableCell>{row.schoolName || "-"}</TableCell>
                      <TableCell>{row.schoolLevel || "-"}</TableCell>
                    </>
                  ) : null}
                  <TableCell>{row.nisn || "-"}</TableCell>
                  <TableCell>{row.namaSiswa || "-"}</TableCell>
                  <TableCell>{row.rombel || "-"}</TableCell>
                  <TableCell className="text-right">
                    {row.tahunLulus || "-"}
                  </TableCell>
                  <TableCell>{row.finalStatus || "-"}</TableCell>
                  <TableCell>{row.finalPath || "-"}</TableCell>
                  <TableCell>{row.changedBy || "-"}</TableCell>
                  <TableCell>{formatDateTime(row.changedAt)}</TableCell>
                </TableRow>
              )}
            />
          </CardContent>
        </Card>
      </section>
    </AdminRouteGuard>
  );
}
