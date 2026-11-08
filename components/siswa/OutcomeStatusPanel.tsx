"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import SiswaModuleActions from "@/components/siswa/SiswaModuleActions";

type OutcomeStatusPanelProps = {
  title: string;
  description: string;
  status: "PT" | "KERJA" | "WIRAUSAHA" | "BELUM";
};

type OutcomeListRow = {
  id: number;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  finalStatus: string;
  finalPath: string;
  detail: string;
  ptKampus?: string;
  ptProdi?: string;
  ptJalur?: string;
  ptTahunDiterima?: number | null;
  finalizedAt?: string | null;
};

type OutcomeListMeta = {
  page: number;
  pageSize: number;
  total: number;
};

type OutcomeListResponse = {
  data: OutcomeListRow[];
  meta: OutcomeListMeta;
};

type OutcomeAnalyticsPoint = {
  tahunLulus: number;
  finalCount: number;
};

type OutcomeDetailPayload = {
  student: {
    id: number;
    nisn: string;
    namaSiswa: string;
    rombel: string;
    tahunLulus: number;
  };
  outcome: {
    final_status?: string | null;
    final_path?: string | null;
    finalized_at?: string | null;
    pt_detail?: {
      kampus?: string | null;
      prodi?: string | null;
      tahun_diterima?: number | null;
    } | null;
    work_detail?: {
      perusahaan?: string | null;
      jabatan?: string | null;
      tahun_mulai?: number | null;
    } | null;
    business_detail?: {
      bidang?: string | null;
      nama_usaha?: string | null;
      tahun_mulai?: number | null;
    } | null;
  } | null;
};

type OutcomeDraft = {
  finalStatus: "PT" | "KERJA" | "WIRAUSAHA" | "BELUM";
  finalPath: string;
  ptKampus: string;
  ptProdi: string;
  ptTahunDiterima: string;
  workPerusahaan: string;
  workJabatan: string;
  workTahunMulai: string;
  businessBidang: string;
  businessNama: string;
  businessTahunMulai: string;
  reason: string;
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

const formatDate = (value?: string | null) => {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
  return parsed.toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
};

const emptyDraft = (status: OutcomeDraft["finalStatus"]): OutcomeDraft => ({
  finalStatus: status,
  finalPath: "",
  ptKampus: "",
  ptProdi: "",
  ptTahunDiterima: "",
  workPerusahaan: "",
  workJabatan: "",
  workTahunMulai: "",
  businessBidang: "",
  businessNama: "",
  businessTahunMulai: "",
  reason: "",
});

const fetchOutcomeList = async (params: {
  status: string;
  page: number;
  pageSize: number;
  search: string;
  year: string;
}): Promise<OutcomeListResponse> => {
  const query = new URLSearchParams();
  query.set("status", params.status);
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.search.trim()) query.set("q", params.search.trim());
  if (params.year.trim()) query.set("year", params.year.trim());

  const response = await apiFetch(`/api/siswa/outcomes/list?${query.toString()}`);
  if (!response.ok) {
    throw new Error("Failed to fetch outcomes.");
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
          nisn: readString(row.nisn),
          namaSiswa: readString(row.nama_siswa ?? row.namaSiswa),
          rombel: readString(row.rombel),
          tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
          finalStatus: readString(row.final_status ?? row.finalStatus),
          finalPath: readString(row.final_path ?? row.finalPath),
          detail: readString(row.detail),
          ptKampus: readString(row.pt_kampus ?? row.ptKampus),
          ptProdi: readString(row.pt_prodi ?? row.ptProdi),
          ptJalur: readString(row.pt_jalur ?? row.ptJalur),
          ptTahunDiterima: (() => {
            const parsed = readNumber(row.pt_tahun_diterima ?? row.ptTahunDiterima);
            return parsed > 0 ? parsed : null;
          })(),
          finalizedAt: readString(row.finalized_at ?? row.finalizedAt) || null,
        };
      })
      .filter((row: OutcomeListRow | null): row is OutcomeListRow =>
        Boolean(row && row.id)
      ),
    meta: {
      page: readNumber(meta.page) || params.page,
      pageSize: readNumber(meta.pageSize) || params.pageSize,
      total: readNumber(meta.total),
    },
  };
};

const fetchOutcomeAnalytics = async (
  status: string,
  year: string
): Promise<OutcomeAnalyticsPoint[]> => {
  const query = new URLSearchParams();
  query.set("status", status);
  if (year.trim()) query.set("year", year.trim());
  const response = await apiFetch(`/api/siswa/outcomes/analytics?${query.toString()}`);
  if (!response.ok) {
    throw new Error("Failed to fetch analytics.");
  }
  const payload = await response.json().catch(() => null);
  const rows = Array.isArray(payload?.data) ? payload.data : payload;
  if (!Array.isArray(rows)) return [];
  return rows
    .map((row: unknown) => {
      if (!isRecord(row)) return null;
      const tahunLulus = readNumber(row.tahun_lulus ?? row.tahunLulus);
      const finalCount = readNumber(row.final_count ?? row.finalCount);
      if (!tahunLulus) return null;
      return { tahunLulus, finalCount };
    })
    .filter((row): row is OutcomeAnalyticsPoint => Boolean(row));
};

const fetchOutcomeDetail = async (id: number): Promise<OutcomeDetailPayload> => {
  const response = await apiFetch(`/api/siswa/outcomes/${id}`);
  if (!response.ok) {
    throw new Error("Failed to fetch detail.");
  }
  const payload = (await response.json().catch(() => null)) as {
    data?: OutcomeDetailPayload;
  } | null;
  return payload?.data ?? (payload as OutcomeDetailPayload);
};

export default function OutcomeStatusPanel({
  title,
  description,
  status,
}: OutcomeStatusPanelProps) {
  const { status: authStatus, user } = useAuth();
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const isReadOnly = !isSchoolAdmin;
  const queryClient = useQueryClient();
  const searchParams = useSearchParams();
  const initialYear = searchParams.get("year") ?? "";
  const [page, setPage] = useState(1);
  const pageSize = 10;
  const [search, setSearch] = useState("");
  const [year, setYear] = useState(initialYear);

  useEffect(() => {
    setYear(initialYear);
    setPage(1);
  }, [initialYear]);

  const {
    data: listData,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ["siswa-outcomes-list", status, page, pageSize, search, year],
    queryFn: () =>
      fetchOutcomeList({
        status,
        page,
        pageSize,
        search,
        year,
      }),
    enabled: authStatus === "authenticated" && isSchoolAdmin,
  });

  const { data: analyticsData = [] } = useQuery({
    queryKey: ["siswa-outcomes-analytics", status, user?.role ?? "PUBLIC", year],
    queryFn: () => fetchOutcomeAnalytics(status, year),
    enabled: authStatus === "authenticated",
  });

  const totalFinal = analyticsData.reduce((acc, row) => acc + row.finalCount, 0);

  const chartData = useMemo(
    () =>
      analyticsData.map((row) => ({
        label: String(row.tahunLulus),
        total: row.finalCount,
      })),
    [analyticsData]
  );

  const columns = useMemo(
    () => [
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "tahun", label: "Tahun Lulus", align: "right" as const },
      { key: "jalur", label: "Jalur/Detail" },
      { key: "finalized", label: "Finalized", align: "center" as const },
      { key: "actions", label: "Aksi", align: "center" as const },
    ],
    []
  );

  const [detailTarget, setDetailTarget] = useState<OutcomeListRow | null>(null);
  const [detailError, setDetailError] = useState("");
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [draft, setDraft] = useState<OutcomeDraft>(emptyDraft(status));
  const [initialStatus, setInitialStatus] = useState<OutcomeDraft["finalStatus"]>(
    status
  );
  const [saveMessage, setSaveMessage] = useState("");
  const [saveError, setSaveError] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  const openDetail = async (row: OutcomeListRow) => {
    setDetailTarget(row);
    setDetailError("");
    setSaveMessage("");
    setSaveError("");
    setIsDetailLoading(true);
    try {
      const payload = await fetchOutcomeDetail(row.id);
      const outcome = payload?.outcome ?? null;
      const normalizedStatus = (
        readString(outcome?.final_status ?? row.finalStatus) ||
        status
      ).toUpperCase() as OutcomeDraft["finalStatus"];
      setInitialStatus(normalizedStatus);

      setDraft({
        finalStatus: normalizedStatus,
        finalPath: readString(outcome?.final_path),
        ptKampus: readString(outcome?.pt_detail?.kampus),
        ptProdi: readString(outcome?.pt_detail?.prodi),
        ptTahunDiterima: readString(outcome?.pt_detail?.tahun_diterima),
        workPerusahaan: readString(outcome?.work_detail?.perusahaan),
        workJabatan: readString(outcome?.work_detail?.jabatan),
        workTahunMulai: readString(outcome?.work_detail?.tahun_mulai),
        businessBidang: readString(outcome?.business_detail?.bidang),
        businessNama: readString(outcome?.business_detail?.nama_usaha),
        businessTahunMulai: readString(outcome?.business_detail?.tahun_mulai),
        reason: "",
      });
    } catch (error) {
      setDetailError(
        error instanceof Error ? error.message : "Gagal memuat detail."
      );
    } finally {
      setIsDetailLoading(false);
    }
  };

  const closeDetail = () => {
    setDetailTarget(null);
    setDetailError("");
    setSaveMessage("");
    setSaveError("");
    setDraft(emptyDraft(status));
  };

  const updateDraft = (field: keyof OutcomeDraft, value: string) => {
    setDraft((prev) => ({ ...prev, [field]: value }));
  };

  const handleSave = async () => {
    if (!detailTarget) return;
    setSaveMessage("");
    setSaveError("");

    if (draft.finalStatus !== initialStatus) {
      const confirmed = window.confirm(
        "Anda mengubah kategori status akhir siswa. Lanjutkan perubahan?"
      );
      if (!confirmed) return;
    }

    if (draft.finalStatus === "PT") {
      if (!draft.finalPath.trim()) {
        setSaveError("Final path wajib diisi untuk status PT.");
        return;
      }
      if (!draft.ptKampus.trim() || !draft.ptProdi.trim()) {
        setSaveError("Kampus dan prodi wajib diisi untuk status PT.");
        return;
      }
    }
    if (draft.finalStatus === "KERJA" && !draft.workPerusahaan.trim()) {
      setSaveError("Nama perusahaan wajib diisi untuk status kerja.");
      return;
    }
    if (draft.finalStatus === "WIRAUSAHA" && !draft.businessBidang.trim()) {
      setSaveError("Bidang usaha wajib diisi untuk status wirausaha.");
      return;
    }

    const payload: Record<string, unknown> = {
      finalStatus: draft.finalStatus,
      finalPath: draft.finalStatus === "PT" ? draft.finalPath.trim() : "",
      reason: draft.reason.trim(),
    };

    if (draft.finalStatus === "PT") {
      payload.ptDetail = {
        kampus: draft.ptKampus.trim(),
        prodi: draft.ptProdi.trim(),
        tahunDiterima: draft.ptTahunDiterima.trim()
          ? Number(draft.ptTahunDiterima)
          : undefined,
      };
    }
    if (draft.finalStatus === "KERJA") {
      payload.workDetail = {
        perusahaan: draft.workPerusahaan.trim(),
        jabatan: draft.workJabatan.trim(),
        tahunMulai: draft.workTahunMulai.trim()
          ? Number(draft.workTahunMulai)
          : undefined,
      };
    }
    if (draft.finalStatus === "WIRAUSAHA") {
      payload.businessDetail = {
        bidang: draft.businessBidang.trim(),
        namaUsaha: draft.businessNama.trim(),
        tahunMulai: draft.businessTahunMulai.trim()
          ? Number(draft.businessTahunMulai)
          : undefined,
      };
    }

    setIsSaving(true);
    try {
      const response = await apiFetch(`/api/siswa/outcomes/${detailTarget.id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      const responsePayload = await response.json().catch(() => null);
      if (!response.ok) {
        const message = readString(responsePayload?.message);
        throw new Error(message || "Gagal menyimpan status akhir.");
      }
      const data = responsePayload?.data ?? responsePayload ?? {};
      const warning = readString(data.warning);
      if (warning) {
        setSaveMessage(warning);
      } else {
        setSaveMessage("Status akhir berhasil diperbarui.");
      }
      void refetch();
      void queryClient.invalidateQueries({ queryKey: ["pt-pipeline"] });
      void queryClient.invalidateQueries({ queryKey: ["wirausaha-pipeline"] });
      void queryClient.invalidateQueries({ queryKey: ["siswa-cohorts-list"] });
    } catch (error) {
      setSaveError(
        error instanceof Error ? error.message : "Gagal menyimpan status akhir."
      );
    } finally {
      setIsSaving(false);
    }
  };

  const tableRows = listData?.data ?? [];
  const tableMeta = listData?.meta ?? {
    page,
    pageSize,
    total: 0,
  };

  return (
    <section className="space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Status Lulusan
        </p>
        <h1 className="text-3xl font-semibold text-zinc-900">{title}</h1>
        <p className="text-sm text-zinc-600">{description}</p>
        {!isSchoolAdmin ? (
          <p className="text-xs font-medium text-zinc-500">
            Data yang ditampilkan bersifat agregat sesuai scope akses.
          </p>
        ) : null}
      </header>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardContent className="space-y-1">
            <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Total Final
            </p>
            <p className="text-3xl font-semibold text-zinc-900">{totalFinal}</p>
          </CardContent>
        </Card>
        <Card className="md:col-span-2">
          <CardContent className="h-40 w-full">
            {chartData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="label" />
                  <YAxis />
                  <Tooltip />
                  <Bar dataKey="total" fill="#1f2937" />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                Data agregat belum tersedia
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {isSchoolAdmin ? (
        <>
          <Card>
            <CardContent className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Cari Siswa
                </label>
                <input
                  type="text"
                  value={search}
                  onChange={(event) => {
                    setSearch(event.target.value);
                    setPage(1);
                  }}
                  placeholder="Cari NISN atau nama siswa"
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:min-w-[260px]"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tahun Lulus
                </label>
                <input
                  type="number"
                  value={year}
                  onChange={(event) => {
                    setYear(event.target.value);
                    setPage(1);
                  }}
                  placeholder="Semua"
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-[140px]"
                />
              </div>
            </CardContent>
          </Card>

          <SiswaModuleActions
            title={`Status Akhir (${title})`}
            templateUrl="/api/siswa/outcomes/template.xlsx"
            importUrl="/api/siswa/outcomes/import"
            accept=".xlsx,.xlsm"
            isSchoolAdmin
            onSuccess={() => void refetch()}
          />

          <Card>
            <CardContent className="space-y-4">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Daftar Siswa Final
                </p>
                <h2 className="text-lg font-semibold text-zinc-900">
                  Status akhir {title.toLowerCase()}
                </h2>
              </div>

              <DataTable
                columns={columns}
                rows={tableRows}
                isLoading={isLoading}
                isError={isError}
                emptyMessage="Belum ada data siswa final."
                tableWrapperClassName="max-h-[420px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
                tableHeaderClassName="sticky top-0 z-10 bg-white"
                page={tableMeta.page}
                pageSize={tableMeta.pageSize}
                total={tableMeta.total}
                onPageChange={setPage}
                renderRow={(row) => {
                  const detailFallback = [row.ptKampus, row.ptProdi]
                    .map((value) => (value ?? "").trim())
                    .filter(Boolean)
                    .join(" - ");
                  const detailText = row.detail || detailFallback;
                  const yearText = row.ptTahunDiterima
                    ? `Tahun ${row.ptTahunDiterima}`
                    : "";
                  const detailLine = [detailText, yearText].filter(Boolean).join(" - ");

                  return (
                    <TableRow key={row.id}>
                      <TableCell>{row.nisn || "-"}</TableCell>
                      <TableCell>{row.namaSiswa || "-"}</TableCell>
                      <TableCell>{row.rombel || "-"}</TableCell>
                      <TableCell className="text-right">
                        {row.tahunLulus || "-"}
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1">
                          <p className="text-xs font-semibold text-zinc-900">
                            {row.finalPath || row.finalStatus || "-"}
                          </p>
                          <p className="text-xs text-zinc-500">
                            {detailLine || "-"}
                          </p>
                        </div>
                      </TableCell>
                      <TableCell className="text-center text-xs text-zinc-600">
                        {formatDate(row.finalizedAt)}
                      </TableCell>
                      <TableCell className="text-center">
                        <button
                          type="button"
                          onClick={() => openDetail(row)}
                          className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50"
                        >
                          {isReadOnly ? "Detail" : "Kelola"}
                        </button>
                      </TableCell>
                    </TableRow>
                  );
                }}
              />
            </CardContent>
          </Card>
        </>
      ) : null}

      {detailTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-4xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Perbarui Status Akhir
              </h2>
              <p className="text-sm text-zinc-600">
                {detailTarget.namaSiswa || "-"} - {detailTarget.nisn || "-"} -{" "}
                {detailTarget.rombel || "-"}
              </p>
            </div>

            {isDetailLoading ? (
              <div className="mt-4 rounded-md border border-dashed border-zinc-200 p-4 text-sm text-zinc-500">
                Memuat detail...
              </div>
            ) : detailError ? (
              <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-600">
                {detailError}
              </div>
            ) : (
              <div className="mt-4 space-y-5">
                <div className="grid gap-3 md:grid-cols-3">
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Status Akhir
                    </label>
                    <select
                      value={draft.finalStatus}
                      onChange={(event) =>
                        updateDraft(
                          "finalStatus",
                          event.target.value as OutcomeDraft["finalStatus"]
                        )
                      }
                      disabled={isReadOnly}
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                    >
                      <option value="PT">Perguruan Tinggi</option>
                      <option value="KERJA">Dunia Kerja</option>
                      <option value="WIRAUSAHA">Wirausaha</option>
                      <option value="BELUM">Belum Melanjutkan</option>
                    </select>
                  </div>
                  {draft.finalStatus === "PT" ? (
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Jalur PT
                      </label>
                      <select
                        value={draft.finalPath}
                        onChange={(event) =>
                          updateDraft("finalPath", event.target.value)
                        }
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      >
                        <option value="">Pilih Jalur</option>
                        <option value="SNBP">SNBP</option>
                        <option value="SNBT">SNBT</option>
                        <option value="KEDINASAN">KEDINASAN</option>
                        <option value="MANDIRI">MANDIRI</option>
                        <option value="LAINNYA">LAINNYA</option>
                      </select>
                    </div>
                  ) : null}
                  <div className="space-y-1">
                    <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Alasan Perubahan (Opsional)
                    </label>
                    <input
                      type="text"
                      value={draft.reason}
                      onChange={(event) => updateDraft("reason", event.target.value)}
                      disabled={isReadOnly}
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      placeholder="Catatan audit"
                    />
                  </div>
                </div>

                {draft.finalStatus === "PT" ? (
                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Kampus
                      </label>
                      <input
                        type="text"
                        value={draft.ptKampus}
                        onChange={(event) => updateDraft("ptKampus", event.target.value)}
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Prodi
                      </label>
                      <input
                        type="text"
                        value={draft.ptProdi}
                        onChange={(event) => updateDraft("ptProdi", event.target.value)}
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Tahun Diterima
                      </label>
                      <input
                        type="number"
                        value={draft.ptTahunDiterima}
                        onChange={(event) =>
                          updateDraft("ptTahunDiterima", event.target.value)
                        }
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                  </div>
                ) : null}

                {draft.finalStatus === "KERJA" ? (
                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Perusahaan
                      </label>
                      <input
                        type="text"
                        value={draft.workPerusahaan}
                        onChange={(event) =>
                          updateDraft("workPerusahaan", event.target.value)
                        }
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Jabatan
                      </label>
                      <input
                        type="text"
                        value={draft.workJabatan}
                        onChange={(event) =>
                          updateDraft("workJabatan", event.target.value)
                        }
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Tahun Mulai
                      </label>
                      <input
                        type="number"
                        value={draft.workTahunMulai}
                        onChange={(event) =>
                          updateDraft("workTahunMulai", event.target.value)
                        }
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                  </div>
                ) : null}

                {draft.finalStatus === "WIRAUSAHA" ? (
                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Bidang Usaha
                      </label>
                      <input
                        type="text"
                        value={draft.businessBidang}
                        onChange={(event) =>
                          updateDraft("businessBidang", event.target.value)
                        }
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Nama Usaha
                      </label>
                      <input
                        type="text"
                        value={draft.businessNama}
                        onChange={(event) =>
                          updateDraft("businessNama", event.target.value)
                        }
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                        Tahun Mulai
                      </label>
                      <input
                        type="number"
                        value={draft.businessTahunMulai}
                        onChange={(event) =>
                          updateDraft("businessTahunMulai", event.target.value)
                        }
                        disabled={isReadOnly}
                        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      />
                    </div>
                  </div>
                ) : null}

                {saveMessage ? (
                  <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                    {saveMessage}
                  </p>
                ) : null}
                {saveError ? (
                  <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                    {saveError}
                  </p>
                ) : null}
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={closeDetail}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Tutup
              </button>
              {!isReadOnly ? (
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={isSaving}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isSaving ? "Menyimpan..." : "Simpan"}
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}

