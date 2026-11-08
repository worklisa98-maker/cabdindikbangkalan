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

type CohortRow = {
  id: number;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  status: string;
};

type ListMeta = {
  page: number;
  pageSize: number;
  total: number;
};

type CohortListResponse = {
  data: CohortRow[];
  meta: ListMeta;
};

const fetchCohorts = async (params: {
  page: number;
  pageSize: number;
}): Promise<CohortListResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));

  const response = await apiFetch(`/api/siswa/cohorts/list?${query.toString()}`);
  if (!response.ok) {
    throw new Error("Failed to fetch cohorts.");
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

export default function SiswaKelasXiiPage() {
  const { status, user } = useAuth();
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const router = useRouter();
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const {
    data: cohortData,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ["siswa-cohorts-list", page, pageSize],
    queryFn: () => fetchCohorts({ page, pageSize }),
    enabled: status === "authenticated",
  });

  const tableRows = cohortData?.data ?? [];
  const tableMeta = cohortData?.meta ?? {
    page,
    pageSize,
    total: 0,
  };

  const columns = useMemo(
    () => [
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "tahunLulus", label: "Tahun Lulus", align: "right" as const },
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
              Data Siswa Kelas Akhir
            </h1>
            <p className="text-sm text-zinc-600">
              Kelola data cohort siswa kelas akhir dan pantau status pendaftarannya.
            </p>
          </div>
          <button
            type="button"
            onClick={() => router.push("/siswa/kelas-xii/histori")}
            className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
          >
            Histori Import
          </button>
        </header>

        <SiswaModuleActions
          title="Siswa Kelas Akhir"
          templateUrl="/api/templates/siswa-cohorts"
          importUrl="/api/siswa/cohorts/import"
          accept=".xlsx,.xlsm"
          isSchoolAdmin={isSchoolAdmin}
          onSuccess={() => void refetch()}
        />

        <Card>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Daftar Siswa Kelas Akhir
              </p>
              <h2 className="text-lg font-semibold text-zinc-900">
                Tabel data cohort siswa kelas akhir
              </h2>
            </div>

            <DataTable
              columns={columns}
              rows={tableRows}
              isLoading={isLoading}
              isError={isError}
              emptyMessage="Belum ada data siswa kelas akhir."
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
                  <TableCell className="text-right">
                    {row.tahunLulus || "-"}
                  </TableCell>
                  <TableCell>{row.status || "Belum Didaftarkan"}</TableCell>
                </TableRow>
              )}
            />
          </CardContent>
        </Card>
      </section>
    </AdminRouteGuard>
  );
}

