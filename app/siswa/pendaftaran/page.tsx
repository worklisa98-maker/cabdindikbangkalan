"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import SiswaModuleActions from "@/components/siswa/SiswaModuleActions";
import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";

type ApplicationRow = {
  id: number;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  jalur: string;
  namaPerguruanTinggi: string;
  status: string;
  final_status?: string;
  finalStatus?: string;
  finalized_at?: string | null;
  finalizedAt?: string | null;
};

type ListMeta = {
  page: number;
  pageSize: number;
  total: number;
};

type ApplicationListResponse = {
  data: ApplicationRow[];
  meta: ListMeta;
};

const fetchApplications = async (params: {
  page: number;
  pageSize: number;
}): Promise<ApplicationListResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  // TODO: enforce filter finalized di backend
  query.set("hideFinal", "1");

  const response = await apiFetch(
    `/api/siswa/applications/list?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch applications.");
  }
  const payload = await response.json().catch(() => null);
  const data = payload?.data ?? {};
  return {
    data: Array.isArray(data.data) ? data.data : [],
    meta: data.meta ?? {
      page: params.page,
      pageSize: params.pageSize,
      total: 0,
    },
  };
};

export default function SiswaPendaftaranPage() {
  const { status, user } = useAuth();
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const router = useRouter();
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const {
    data: applicationData,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ["siswa-applications-list", page, pageSize],
    queryFn: () => fetchApplications({ page, pageSize }),
    enabled: status === "authenticated",
  });

  const rawRows = useMemo(() => applicationData?.data ?? [], [applicationData?.data]);
  const { visibleRows: tableRows, filteredCount } = useMemo(() => {
    const visibleRows = rawRows.filter((row) => {
      const finalStatusValue = String(
        row.final_status ?? row.finalStatus ?? ""
      ).trim();
      const finalizedAtValue = String(
        row.finalized_at ?? row.finalizedAt ?? ""
      ).trim();
      return !finalStatusValue && !finalizedAtValue;
    });
    return {
      visibleRows,
      filteredCount: rawRows.length - visibleRows.length,
    };
  }, [rawRows]);
  const tableMeta = applicationData?.meta ?? {
    page,
    pageSize,
    total: 0,
  };
  const hasFilteredFinal = filteredCount > 0;

  const columns = useMemo(
    () => [
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "jalur", label: "Jalur" },
      { key: "pt", label: "Nama Perguruan Tinggi" },
      { key: "status", label: "Status" },
    ],
    []
  );

  return (
    <AdminRouteGuard>
      <section className="space-y-6">
        <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Siswa
            </p>
            <h1 className="text-3xl font-semibold text-zinc-900">
              Pendaftaran Jalur Penerimaan
            </h1>
            <p className="text-sm text-zinc-600">
              Kelola pendaftaran jalur penerimaan siswa dan pantau statusnya.
            </p>
          </div>
          <button
            type="button"
            onClick={() => router.push("/siswa/pendaftaran/histori")}
            className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
          >
            Histori Import
          </button>
        </header>

        <SiswaModuleActions
          title="Pendaftaran Jalur"
          templateUrl="/api/siswa/applications/template.xlsx"
          importUrl="/api/siswa/applications/import"
          accept=".xlsx,.xlsm"
          isSchoolAdmin={isSchoolAdmin}
          onSuccess={() => void refetch()}
        />

        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Daftar Pendaftaran Jalur
              </p>
              <h2 className="text-lg font-semibold text-zinc-900">
                Tabel data pendaftaran jalur penerimaan
              </h2>
            </div>

            <DataTable
              columns={columns}
              rows={tableRows}
              isLoading={isLoading}
              isError={isError}
              emptyMessage="Belum ada data pendaftaran."
              tableWrapperClassName="max-h-[420px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
              tableHeaderClassName="sticky top-0 z-10 bg-white"
              page={tableMeta.page}
              pageSize={tableMeta.pageSize}
              total={tableMeta.total}
              onPageChange={setPage}
              renderRow={(row) => (
                <TableRow key={row.id}>
                  <TableCell>{row.nisn || "-"}</TableCell>
                  <TableCell>{row.namaSiswa || "-"}</TableCell>
                  <TableCell>{row.rombel || "-"}</TableCell>
                  <TableCell>{row.jalur || "-"}</TableCell>
                  <TableCell>{row.namaPerguruanTinggi || "-"}</TableCell>
                  <TableCell>{row.status || "-"}</TableCell>
                </TableRow>
              )}
            />
            {hasFilteredFinal ? (
              <p className="text-xs text-zinc-500">
                Data final disembunyikan dari pipeline.
              </p>
            ) : null}
          </CardContent>
        </Card>
      </section>
    </AdminRouteGuard>
  );
}

