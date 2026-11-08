"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import { DataTable } from "@/components/data-table";
import { TableCell, TableRow } from "@/components/ui/table";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { SYSTEM_MAX_PT_CHOICES } from "@/lib/pt-choice-rules";
import SiswaModuleActions from "@/components/siswa/SiswaModuleActions";
import UkkPanel from "@/components/siswa/UkkPanel";
import RencanaTabs, { type RencanaTabKey } from "@/components/siswa/RencanaTabs";

type PipelineRow = {
  id: number;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  status: string;
  final_status?: string;
  finalStatus?: string;
  finalized_at?: string | null;
  finalizedAt?: string | null;
  year_closed?: boolean;
  yearClosed?: boolean;
};

type PipelineMeta = {
  page: number;
  pageSize: number;
  total: number;
};

type PipelineListResponse = {
  data: PipelineRow[];
  meta: PipelineMeta;
};

type PtChoicePayload = {
  pilihan_ke: number;
  jalur: string;
  kampus: string;
  prodi: string;
};

type PtDetailResponse = {
  student: {
    id: number;
    nisn: string;
    namaSiswa: string;
    rombel: string;
    tahunLulus: number;
    finalized_at?: string | null;
  };
  status: string;
  choices: PtChoicePayload[];
};

type PtDraftChoice = {
  jalur: string;
  kampus: string;
  prodi: string;
};

type PtDraft = {
  choices: PtDraftChoice[];
};

type WirausahaPlanPayload = {
  rencana_ke: number;
  nama_usaha: string;
  kategori_usaha: string;
  status_kesiapan: string;
  rencana_mulai: string;
  modal_awal: string;
  sumber_modal: string;
  lokasi?: string;
  platform?: string;
  catatan?: string;
};

type WirausahaDetailResponse = {
  student: {
    id: number;
    nisn: string;
    namaSiswa: string;
    rombel: string;
    tahunLulus: number;
    finalized_at?: string | null;
  };
  status: string;
  plans: WirausahaPlanPayload[];
};

type WirausahaDraft = {
  plans: Array<
    Omit<WirausahaPlanPayload, "rencana_ke"> & { rencana_ke: number }
  >;
};

const jalurOptions = [
  { value: "SNBP", label: "SNBP" },
  { value: "SNBT", label: "SNBT" },
  { value: "MANDIRI", label: "SNBM/Mandiri" },
  { value: "KEDINASAN", label: "Kedinasan" },
  { value: "LAINNYA", label: "Lainnya" },
];

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

const readBoolean = (value: unknown) => {
  if (typeof value === "boolean") return value;
  if (typeof value === "number") return value !== 0;
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return normalized === "true" || normalized === "1" || normalized === "yes";
  }
  return false;
};

const emptyPtDraft = (): PtDraft => ({
  choices: Array.from({ length: SYSTEM_MAX_PT_CHOICES }, () => ({
    jalur: "",
    kampus: "",
    prodi: "",
  })),
});

const emptyWirausahaDraft = (): WirausahaDraft => ({
  plans: [
    {
      rencana_ke: 1,
      nama_usaha: "",
      kategori_usaha: "",
      status_kesiapan: "",
      rencana_mulai: "",
      modal_awal: "",
      sumber_modal: "",
      lokasi: "",
      platform: "",
      catatan: "",
    },
    {
      rencana_ke: 2,
      nama_usaha: "",
      kategori_usaha: "",
      status_kesiapan: "",
      rencana_mulai: "",
      modal_awal: "",
      sumber_modal: "",
      lokasi: "",
      platform: "",
      catatan: "",
    },
  ],
});

const fetchPtPipeline = async (params: {
  page: number;
  pageSize: number;
  search: string;
}): Promise<PipelineListResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.search.trim()) {
    query.set("q", params.search.trim());
  }

  const response = await apiFetch(
    `/api/siswa/applications/pipeline?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch PT pipeline.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return {
    data: Array.isArray(data?.data) ? data.data : [],
    meta: isRecord(data?.meta)
      ? (data.meta as PipelineMeta)
      : {
          page: params.page,
          pageSize: params.pageSize,
          total: 0,
        },
  };
};

const fetchWirausahaPipeline = async (params: {
  page: number;
  pageSize: number;
  search: string;
}): Promise<PipelineListResponse> => {
  const query = new URLSearchParams();
  query.set("page", String(params.page));
  query.set("pageSize", String(params.pageSize));
  if (params.search.trim()) {
    query.set("q", params.search.trim());
  }

  const response = await apiFetch(
    `/api/siswa/wirausaha/list?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch wirausaha pipeline.");
  }
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  return {
    data: Array.isArray(data?.data) ? data.data : [],
    meta: isRecord(data?.meta)
      ? (data.meta as PipelineMeta)
      : {
          page: params.page,
          pageSize: params.pageSize,
          total: 0,
        },
  };
};

export default function RencanaLulusanClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { status, user } = useAuth();
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const isReadOnly = !isSchoolAdmin;
  const schoolLevel = (user?.schoolLevel ?? "").toUpperCase();
  const isSMK = schoolLevel === "SMK";
  const queryClient = useQueryClient();

  const activeTab = useMemo<RencanaTabKey>(() => {
    const raw = (searchParams.get("tab") ?? "PT").toUpperCase();
    if (raw === "UKK" && !isSMK) {
      return "PT";
    }
    if (raw === "PT" || raw === "WIRAUSAHA" || raw === "UKK") {
      return raw as RencanaTabKey;
    }
    return "PT";
  }, [searchParams, isSMK]);

  const tabItems = useMemo(
    () =>
      [
        { key: "PT" as const, label: "Pendaftaran PT" },
        { key: "WIRAUSAHA" as const, label: "Rencana Wirausaha" },
        isSMK ? { key: "UKK" as const, label: "UKK (SMK)" } : null,
      ].filter(Boolean) as { key: RencanaTabKey; label: string }[],
    [isSMK]
  );

  const handleTabChange = useCallback((nextTab: RencanaTabKey) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("tab", nextTab);
    router.replace(params.toString() ? `?${params.toString()}` : "?", {
      scroll: false,
    });
  }, [router, searchParams]);

  useEffect(() => {
    if (!isSMK && activeTab === "UKK") {
      handleTabChange("PT");
    }
  }, [activeTab, handleTabChange, isSMK]);

  const [ptPage, setPtPage] = useState(1);
  const [ptSearch, setPtSearch] = useState("");
  const ptPageSize = 10;

  const {
    data: ptData,
    isLoading: isPtLoading,
    isError: isPtError,
    refetch: refetchPt,
  } = useQuery({
    queryKey: ["pt-pipeline", ptPage, ptPageSize, ptSearch],
    queryFn: () =>
      fetchPtPipeline({ page: ptPage, pageSize: ptPageSize, search: ptSearch }),
    enabled: status === "authenticated" && activeTab === "PT",
  });

  const rawPtRows = useMemo(() => ptData?.data ?? [], [ptData?.data]);
  const { visibleRows: ptRows, filteredCount: ptFilteredCount } = useMemo(() => {
    const visibleRows = rawPtRows.filter((row) => {
      const finalStatusValue = readString(row.final_status ?? row.finalStatus);
      const finalizedAtValue = readString(row.finalized_at ?? row.finalizedAt);
      return !finalStatusValue && !finalizedAtValue;
    });
    return {
      visibleRows,
      filteredCount: rawPtRows.length - visibleRows.length,
    };
  }, [rawPtRows]);
  const ptMeta = ptData?.meta ?? {
    page: ptPage,
    pageSize: ptPageSize,
    total: 0,
  };

  const ptColumns = useMemo(
    () => [
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "tahunLulus", label: "Tahun Lulus", align: "right" as const },
      { key: "status", label: "Status" },
      { key: "actions", label: "Aksi", align: "center" as const },
    ],
    []
  );

  const [ptTarget, setPtTarget] = useState<PipelineRow | null>(null);
  const [ptDraft, setPtDraft] = useState<PtDraft>(emptyPtDraft());
  const [ptDetailError, setPtDetailError] = useState("");
  const [ptSaveMessage, setPtSaveMessage] = useState("");
  const [ptSaveError, setPtSaveError] = useState("");
  const [isPtLoadingDetail, setIsPtLoadingDetail] = useState(false);
  const [isPtSaving, setIsPtSaving] = useState(false);
  const ptIsYearClosed = readBoolean(ptTarget?.year_closed ?? ptTarget?.yearClosed);
  const ptIsFinalized = Boolean(
    readString(ptTarget?.final_status ?? ptTarget?.finalStatus) ||
      readString(ptTarget?.finalized_at ?? ptTarget?.finalizedAt)
  );
  const ptLocked = ptIsYearClosed || ptIsFinalized;
  const canEditPt = !isReadOnly && !ptLocked;

  const reset = (next: PtDraft | WirausahaDraft) => {
    if ("choices" in next) {
      setPtDraft(next);
    } else {
      setWirausahaDraft(next);
    }
  };

  const openPtDetail = async (row: PipelineRow) => {
    setPtTarget(row);
    setPtDetailError("");
    setPtSaveMessage("");
    setPtSaveError("");
    setIsPtLoadingDetail(true);

    try {
      const params = new URLSearchParams();
      params.set("nisn", row.nisn);
      params.set("tahun_lulus", String(row.tahunLulus));
      const response = await apiFetch(
        `/api/siswa/applications/detail?${params.toString()}`
      );
      if (!response.ok) {
        throw new Error("Gagal memuat detail.");
      }
      const payload = await response.json().catch(() => null);
      const data = (payload?.data ?? payload) as PtDetailResponse;
      const choices = Array.isArray(data?.choices) ? data.choices : [];

      const nextDraft = emptyPtDraft();
      choices.forEach((choice) => {
        const index = readNumber(choice.pilihan_ke) - 1;
        if (index >= 0 && index < nextDraft.choices.length) {
          nextDraft.choices[index] = {
            jalur: readString(choice.jalur),
            kampus: readString(choice.kampus),
            prodi: readString(choice.prodi),
          };
        }
      });
      reset(nextDraft);
    } catch (error) {
      setPtDetailError(
        error instanceof Error ? error.message : "Gagal memuat detail."
      );
    } finally {
      setIsPtLoadingDetail(false);
    }
  };

  const closePtDetail = () => {
    setPtTarget(null);
    setPtDetailError("");
    setPtSaveMessage("");
    setPtSaveError("");
    setPtDraft(emptyPtDraft());
  };

  const updatePtChoice = (
    index: number,
    field: keyof PtDraftChoice,
    value: string
  ) => {
    setPtDraft((prev) => {
      const next = [...prev.choices];
      next[index] = { ...next[index], [field]: value };
      return { ...prev, choices: next };
    });
  };

  const handlePtSave = async () => {
    if (!ptTarget || !canEditPt) return;
    setIsPtSaving(true);
    setPtSaveMessage("");
    setPtSaveError("");

    const normalized: PtChoicePayload[] = [];
    const jalurSet = new Set<string>();

    for (let i = 0; i < ptDraft.choices.length; i += 1) {
      const choice = ptDraft.choices[i];
      const jalur = choice.jalur.trim();
      const kampus = choice.kampus.trim();
      const prodi = choice.prodi.trim();
      const hasAny = jalur !== "" || kampus !== "" || prodi !== "";
      if (!hasAny) {
        continue;
      }
      if (!jalur || !kampus || !prodi) {
        setPtSaveError("Jalur, kampus, dan prodi wajib diisi berpasangan.");
        setIsPtSaving(false);
        return;
      }
      jalurSet.add(jalur);
      normalized.push({
        pilihan_ke: i + 1,
        jalur,
        kampus,
        prodi,
      });
    }

    if (jalurSet.has("SNBP") && jalurSet.has("SNBT")) {
      setPtSaveError("Tidak boleh memilih SNBP dan SNBT dalam satu siswa.");
      setIsPtSaving(false);
      return;
    }
    if (normalized.length > SYSTEM_MAX_PT_CHOICES) {
      setPtSaveError(`Maksimal ${SYSTEM_MAX_PT_CHOICES} pilihan PT per siswa.`);
      setIsPtSaving(false);
      return;
    }

    try {
      const payload = {
        nisn: ptTarget.nisn,
        tahun_lulus: ptTarget.tahunLulus,
        choices: normalized,
      };
      const response = await apiFetch("/api/siswa/applications", {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        const message = await response.json().catch(() => null);
        throw new Error(message?.message ?? "Gagal menyimpan pendaftaran.");
      }
      setPtSaveMessage("Pendaftaran PT berhasil disimpan.");
      void refetchPt();
      void queryClient.invalidateQueries({
        queryKey: ["siswa-cohorts-list"],
      });
    } catch (error) {
      setPtSaveError(
        error instanceof Error ? error.message : "Gagal menyimpan pendaftaran."
      );
    } finally {
      setIsPtSaving(false);
    }
  };

  const [wirausahaPage, setWirausahaPage] = useState(1);
  const [wirausahaSearch, setWirausahaSearch] = useState("");
  const wirausahaPageSize = 10;

  const {
    data: wirausahaData,
    isLoading: isWirausahaLoading,
    isError: isWirausahaError,
    refetch: refetchWirausaha,
  } = useQuery({
    queryKey: [
      "wirausaha-pipeline",
      wirausahaPage,
      wirausahaPageSize,
      wirausahaSearch,
    ],
    queryFn: () =>
      fetchWirausahaPipeline({
        page: wirausahaPage,
        pageSize: wirausahaPageSize,
        search: wirausahaSearch,
      }),
    enabled: status === "authenticated" && activeTab === "WIRAUSAHA",
  });

  const rawWirausahaRows = useMemo(() => wirausahaData?.data ?? [], [wirausahaData?.data]);
  const {
    visibleRows: wirausahaRows,
    filteredCount: wirausahaFilteredCount,
  } = useMemo(() => {
    const visibleRows = rawWirausahaRows.filter((row) => {
      const finalStatusValue = readString(row.final_status ?? row.finalStatus);
      const finalizedAtValue = readString(row.finalized_at ?? row.finalizedAt);
      return !finalStatusValue && !finalizedAtValue;
    });
    return {
      visibleRows,
      filteredCount: rawWirausahaRows.length - visibleRows.length,
    };
  }, [rawWirausahaRows]);

  const wirausahaMeta = wirausahaData?.meta ?? {
    page: wirausahaPage,
    pageSize: wirausahaPageSize,
    total: 0,
  };

  const wirausahaColumns = useMemo(
    () => [
      { key: "nisn", label: "NISN" },
      { key: "nama", label: "Nama" },
      { key: "rombel", label: "Rombel" },
      { key: "tahunLulus", label: "Tahun Lulus", align: "right" as const },
      { key: "status", label: "Status" },
      { key: "actions", label: "Aksi", align: "center" as const },
    ],
    []
  );

  const [wirausahaTarget, setWirausahaTarget] = useState<PipelineRow | null>(
    null
  );
  const [wirausahaDraft, setWirausahaDraft] = useState<WirausahaDraft>(
    emptyWirausahaDraft()
  );
  const [wirausahaDetailError, setWirausahaDetailError] = useState("");
  const [wirausahaSaveMessage, setWirausahaSaveMessage] = useState("");
  const [wirausahaSaveError, setWirausahaSaveError] = useState("");
  const [isWirausahaLoadingDetail, setIsWirausahaLoadingDetail] = useState(false);
  const [isWirausahaSaving, setIsWirausahaSaving] = useState(false);
  const wirausahaIsYearClosed = readBoolean(
    wirausahaTarget?.year_closed ?? wirausahaTarget?.yearClosed
  );
  const wirausahaIsFinalized = Boolean(
    readString(wirausahaTarget?.final_status ?? wirausahaTarget?.finalStatus) ||
      readString(wirausahaTarget?.finalized_at ?? wirausahaTarget?.finalizedAt)
  );
  const wirausahaLocked = wirausahaIsYearClosed || wirausahaIsFinalized;
  const canEditWirausaha = !isReadOnly && !wirausahaLocked;

  const openWirausahaDetail = async (row: PipelineRow) => {
    setWirausahaTarget(row);
    setWirausahaDetailError("");
    setWirausahaSaveMessage("");
    setWirausahaSaveError("");
    setIsWirausahaLoadingDetail(true);

    try {
      const params = new URLSearchParams();
      params.set("nisn", row.nisn);
      params.set("tahun_lulus", String(row.tahunLulus));
      const response = await apiFetch(
        `/api/siswa/wirausaha/detail?${params.toString()}`
      );
      if (!response.ok) {
        throw new Error("Gagal memuat detail.");
      }
      const payload = await response.json().catch(() => null);
      const data = (payload?.data ?? payload) as WirausahaDetailResponse;
      const plans = Array.isArray(data?.plans) ? data.plans : [];

      const nextDraft = emptyWirausahaDraft();
      plans.forEach((plan) => {
        const index = readNumber(plan.rencana_ke) - 1;
        if (index >= 0 && index < nextDraft.plans.length) {
          nextDraft.plans[index] = {
            rencana_ke: index + 1,
            nama_usaha: readString(plan.nama_usaha),
            kategori_usaha: readString(plan.kategori_usaha),
            status_kesiapan: readString(plan.status_kesiapan),
            rencana_mulai: readString(plan.rencana_mulai),
            modal_awal: readString(plan.modal_awal),
            sumber_modal: readString(plan.sumber_modal),
            lokasi: readString(plan.lokasi),
            platform: readString(plan.platform),
            catatan: readString(plan.catatan),
          };
        }
      });
      reset(nextDraft);
    } catch (error) {
      setWirausahaDetailError(
        error instanceof Error ? error.message : "Gagal memuat detail."
      );
    } finally {
      setIsWirausahaLoadingDetail(false);
    }
  };

  const closeWirausahaDetail = () => {
    setWirausahaTarget(null);
    setWirausahaDetailError("");
    setWirausahaSaveMessage("");
    setWirausahaSaveError("");
    setWirausahaDraft(emptyWirausahaDraft());
  };

  const updateWirausahaPlan = (
    index: number,
    field: keyof Omit<WirausahaPlanPayload, "rencana_ke">,
    value: string
  ) => {
    setWirausahaDraft((prev) => {
      const next = [...prev.plans];
      next[index] = { ...next[index], [field]: value };
      return { ...prev, plans: next };
    });
  };

  const handleWirausahaSave = async () => {
    if (!wirausahaTarget || !canEditWirausaha) return;
    setIsWirausahaSaving(true);
    setWirausahaSaveMessage("");
    setWirausahaSaveError("");

    const normalized: WirausahaPlanPayload[] = [];
    for (let i = 0; i < wirausahaDraft.plans.length; i += 1) {
      const plan = wirausahaDraft.plans[i];
      const hasAny = Object.entries(plan).some(([key, value]) => {
        if (key === "rencana_ke") return false;
        return String(value ?? "").trim() !== "";
      });
      if (!hasAny) {
        continue;
      }

      const requiredFields = [
        plan.nama_usaha,
        plan.kategori_usaha,
        plan.status_kesiapan,
        plan.rencana_mulai,
        plan.modal_awal,
        plan.sumber_modal,
      ];
      if (requiredFields.some((item) => !String(item ?? "").trim())) {
        setWirausahaSaveError(
          "Nama usaha, kategori, status kesiapan, rencana mulai, modal awal, dan sumber modal wajib diisi."
        );
        setIsWirausahaSaving(false);
        return;
      }

      normalized.push({
        rencana_ke: i + 1,
        nama_usaha: plan.nama_usaha.trim(),
        kategori_usaha: plan.kategori_usaha.trim(),
        status_kesiapan: plan.status_kesiapan.trim(),
        rencana_mulai: plan.rencana_mulai.trim(),
        modal_awal: plan.modal_awal.trim(),
        sumber_modal: plan.sumber_modal.trim(),
        lokasi: plan.lokasi?.trim(),
        platform: plan.platform?.trim(),
        catatan: plan.catatan?.trim(),
      });
    }

    if (normalized.length > 2) {
      setWirausahaSaveError("Maksimal 2 rencana per siswa.");
      setIsWirausahaSaving(false);
      return;
    }

    try {
      const payload = {
        nisn: wirausahaTarget.nisn,
        tahun_lulus: wirausahaTarget.tahunLulus,
        plans: normalized,
      };
      const response = await apiFetch("/api/siswa/wirausaha", {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        const message = await response.json().catch(() => null);
        throw new Error(message?.message ?? "Gagal menyimpan rencana.");
      }
      setWirausahaSaveMessage("Rencana wirausaha berhasil disimpan.");
      void refetchWirausaha();
      void queryClient.invalidateQueries({
        queryKey: ["siswa-cohorts-list"],
      });
    } catch (error) {
      setWirausahaSaveError(
        error instanceof Error ? error.message : "Gagal menyimpan rencana."
      );
    } finally {
      setIsWirausahaSaving(false);
    }
  };

  return (
    <AdminRouteGuard>
      <section className="space-y-6">
        <header className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Siswa
          </p>
          <h1 className="text-3xl font-semibold text-zinc-900">
            Rencana Lulusan
          </h1>
          <p className="text-sm text-zinc-600">
            Pantau dan lengkapi rencana lulusan siswa kelas akhir.
          </p>
        </header>

        <RencanaTabs value={activeTab} onChange={handleTabChange} tabs={tabItems} />

        {activeTab === "PT" ? (
          <>
            <Card>
              <CardContent className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Cari Siswa
                  </label>
                  <input
                    type="text"
                    value={ptSearch}
                    onChange={(event) => {
                      setPtSearch(event.target.value);
                      setPtPage(1);
                    }}
                    placeholder="Cari NISN atau nama siswa"
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:min-w-[280px]"
                  />
                </div>
              </CardContent>
            </Card>

            <SiswaModuleActions
              title="Pendaftaran PT"
              templateUrl="/api/siswa/applications/template.xlsx"
              importUrl="/api/siswa/applications/import"
              accept=".xlsx,.xlsm"
              isSchoolAdmin={isSchoolAdmin}
              onSuccess={() => void refetchPt()}
            />

            <Card>
              <CardContent className="space-y-4">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Daftar Pendaftaran PT
                  </p>
                  <h2 className="text-lg font-semibold text-zinc-900">
                    Status pendaftaran perguruan tinggi
                  </h2>
                </div>

                <DataTable
                  columns={ptColumns}
                  rows={ptRows}
                  isLoading={isPtLoading}
                  isError={isPtError}
                  emptyMessage="Belum ada data siswa kelas akhir."
                  tableWrapperClassName="max-h-[420px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
                  tableHeaderClassName="sticky top-0 z-10 bg-white"
                  page={ptMeta.page}
                  pageSize={ptMeta.pageSize}
                  total={ptMeta.total}
                  onPageChange={setPtPage}
                  renderRow={(row) => {
                    const rowYearClosed = readBoolean(
                      row.year_closed ?? row.yearClosed
                    );
                    const rowFinalized = Boolean(
                      readString(row.final_status ?? row.finalStatus) ||
                        readString(row.finalized_at ?? row.finalizedAt)
                    );
                    const rowLocked = rowYearClosed || rowFinalized;
                    const canOpen = isReadOnly || !rowLocked;
                    const label = isReadOnly
                      ? "Detail"
                      : rowLocked
                        ? "Terkunci"
                        : "Kelola";

                    return (
                      <TableRow key={row.id}>
                        <TableCell>{row.nisn || "-"}</TableCell>
                        <TableCell>{row.namaSiswa || "-"}</TableCell>
                        <TableCell>{row.rombel || "-"}</TableCell>
                        <TableCell className="text-right">
                          {row.tahunLulus || "-"}
                        </TableCell>
                        <TableCell>{row.status || "-"}</TableCell>
                        <TableCell className="text-center">
                          <button
                            type="button"
                            onClick={() => {
                              if (canOpen) openPtDetail(row);
                            }}
                            disabled={!canOpen}
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {label}
                          </button>
                        </TableCell>
                      </TableRow>
                    );
                  }}
                />
                {ptFilteredCount > 0 ? (
                  <p className="text-xs text-zinc-500">
                    Data final disembunyikan dari pipeline.
                  </p>
                ) : null}
              </CardContent>
            </Card>
          </>
        ) : null}

        {activeTab === "WIRAUSAHA" ? (
          <>
            <Card>
              <CardContent className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Cari Siswa
                  </label>
                  <input
                    type="text"
                    value={wirausahaSearch}
                    onChange={(event) => {
                      setWirausahaSearch(event.target.value);
                      setWirausahaPage(1);
                    }}
                    placeholder="Cari NISN atau nama siswa"
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:min-w-[280px]"
                  />
                </div>
              </CardContent>
            </Card>

            <SiswaModuleActions
              title="Rencana Wirausaha"
              templateUrl="/api/siswa/wirausaha/template.xlsx"
              importUrl="/api/siswa/wirausaha/import"
              accept=".xlsx,.xlsm"
              isSchoolAdmin={isSchoolAdmin}
              onSuccess={() => void refetchWirausaha()}
            />

            <Card>
              <CardContent className="space-y-4">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Rencana Wirausaha
                  </p>
                  <h2 className="text-lg font-semibold text-zinc-900">
                    Monitoring rencana wirausaha siswa
                  </h2>
                </div>

                <DataTable
                  columns={wirausahaColumns}
                  rows={wirausahaRows}
                  isLoading={isWirausahaLoading}
                  isError={isWirausahaError}
                  emptyMessage="Belum ada data rencana wirausaha."
                  tableWrapperClassName="max-h-[420px] overflow-y-auto rounded-xl border border-zinc-200 bg-white"
                  tableHeaderClassName="sticky top-0 z-10 bg-white"
                  page={wirausahaMeta.page}
                  pageSize={wirausahaMeta.pageSize}
                  total={wirausahaMeta.total}
                  onPageChange={setWirausahaPage}
                  renderRow={(row) => {
                    const rowYearClosed = readBoolean(
                      row.year_closed ?? row.yearClosed
                    );
                    const rowFinalized = Boolean(
                      readString(row.final_status ?? row.finalStatus) ||
                        readString(row.finalized_at ?? row.finalizedAt)
                    );
                    const rowLocked = rowYearClosed || rowFinalized;
                    const canOpen = isReadOnly || !rowLocked;
                    const label = isReadOnly
                      ? "Detail"
                      : rowLocked
                        ? "Terkunci"
                        : "Kelola";

                    return (
                      <TableRow key={row.id}>
                        <TableCell>{row.nisn || "-"}</TableCell>
                        <TableCell>{row.namaSiswa || "-"}</TableCell>
                        <TableCell>{row.rombel || "-"}</TableCell>
                        <TableCell className="text-right">
                          {row.tahunLulus || "-"}
                        </TableCell>
                        <TableCell>{row.status || "-"}</TableCell>
                        <TableCell className="text-center">
                          <button
                            type="button"
                            onClick={() => {
                              if (canOpen) openWirausahaDetail(row);
                            }}
                            disabled={!canOpen}
                            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {label}
                          </button>
                        </TableCell>
                      </TableRow>
                    );
                  }}
                />
                {wirausahaFilteredCount > 0 ? (
                  <p className="text-xs text-zinc-500">
                    Data final disembunyikan dari pipeline.
                  </p>
                ) : null}
              </CardContent>
            </Card>
          </>
        ) : null}

        {activeTab === "UKK" && isSMK ? <UkkPanel /> : null}
      </section>

      {ptTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-4xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Pendaftaran Perguruan Tinggi
              </h2>
              <p className="text-sm text-zinc-600">
                {ptTarget.namaSiswa || "-"} - {ptTarget.nisn || "-"} -{" "}
                {ptTarget.rombel || "-"}
              </p>
            </div>

            {isPtLoadingDetail ? (
              <div className="mt-4 rounded-md border border-dashed border-zinc-200 p-4 text-sm text-zinc-500">
                Memuat detail...
              </div>
            ) : ptDetailError ? (
              <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-600">
                {ptDetailError}
              </div>
            ) : (
              <div className="mt-4 space-y-6">
                {ptLocked ? (
                  <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
                    {ptIsFinalized
                      ? "Data sudah final dan terkunci."
                      : "Tahun sudah tutup buku. Hanya finalisasi yang diperbolehkan."}
                  </div>
                ) : !isReadOnly ? null : (
                  <div className="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-700">
                    Mode baca saja. Hanya admin sekolah yang dapat memperbarui
                    pendaftaran PT.
                  </div>
                )}

                <div className="space-y-3 rounded-lg border border-zinc-200 p-4">
                  <h3 className="text-sm font-semibold text-zinc-900">
                    Pilihan Kampus & Prodi
                  </h3>
                  <div className="grid gap-3 md:grid-cols-3">
                    {ptDraft.choices.map((choice, index) => (
                      <div key={`pt-choice-${index}`} className="space-y-2">
                        <div className="space-y-1">
                          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                            Pilihan {index + 1} - Jalur
                          </label>
                          <select
                            value={choice.jalur}
                            onChange={(event) =>
                              updatePtChoice(index, "jalur", event.target.value)
                            }
                            disabled={!canEditPt}
                            className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                          >
                            <option value="">Pilih jalur</option>
                            {jalurOptions.map((option) => (
                              <option key={option.value} value={option.value}>
                                {option.label}
                              </option>
                            ))}
                          </select>
                        </div>
                        <div className="space-y-1">
                          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                            Pilihan {index + 1} - Kampus
                          </label>
                          <input
                            type="text"
                            value={choice.kampus}
                            onChange={(event) =>
                              updatePtChoice(index, "kampus", event.target.value)
                            }
                            disabled={!canEditPt}
                            className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                          />
                        </div>
                        <div className="space-y-1">
                          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                            Pilihan {index + 1} - Prodi
                          </label>
                          <input
                            type="text"
                            value={choice.prodi}
                            onChange={(event) =>
                              updatePtChoice(index, "prodi", event.target.value)
                            }
                            disabled={!canEditPt}
                            className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                {ptSaveMessage ? (
                  <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                    {ptSaveMessage}
                  </p>
                ) : null}
                {ptSaveError ? (
                  <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                    {ptSaveError}
                  </p>
                ) : null}
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={closePtDetail}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Tutup
              </button>
              {canEditPt ? (
                <button
                  type="button"
                  onClick={handlePtSave}
                  disabled={isPtSaving}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isPtSaving ? "Menyimpan..." : "Simpan"}
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}

      {wirausahaTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-5xl rounded-xl bg-white p-5 shadow-lg">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-zinc-900">
                Rencana Wirausaha Siswa
              </h2>
              <p className="text-sm text-zinc-600">
                {wirausahaTarget.namaSiswa || "-"} - {wirausahaTarget.nisn || "-"} -{" "}
                {wirausahaTarget.rombel || "-"}
              </p>
            </div>

            {isWirausahaLoadingDetail ? (
              <div className="mt-4 rounded-md border border-dashed border-zinc-200 p-4 text-sm text-zinc-500">
                Memuat detail...
              </div>
            ) : wirausahaDetailError ? (
              <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-600">
                {wirausahaDetailError}
              </div>
            ) : (
              <div className="mt-4 space-y-4">
                {wirausahaLocked ? (
                  <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
                    {wirausahaIsFinalized
                      ? "Data sudah final dan terkunci."
                      : "Tahun sudah tutup buku. Hanya finalisasi yang diperbolehkan."}
                  </div>
                ) : !isReadOnly ? null : (
                  <div className="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-700">
                    Mode baca saja. Hanya admin sekolah yang dapat memperbarui
                    rencana wirausaha.
                  </div>
                )}

                <div className="grid gap-4 md:grid-cols-2">
                  {wirausahaDraft.plans.map((plan, index) => (
                    <div
                      key={`wirausaha-plan-${index}`}
                      className="space-y-3 rounded-lg border border-zinc-200 p-4"
                    >
                      <h3 className="text-sm font-semibold text-zinc-900">
                        Rencana {index + 1}
                      </h3>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Nama Usaha
                        </label>
                        <input
                          type="text"
                          value={plan.nama_usaha}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "nama_usaha", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Kategori Usaha
                        </label>
                        <input
                          type="text"
                          value={plan.kategori_usaha}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "kategori_usaha", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Status Kesiapan
                        </label>
                        <input
                          type="text"
                          value={plan.status_kesiapan}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "status_kesiapan", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Rencana Mulai
                        </label>
                        <input
                          type="text"
                          value={plan.rencana_mulai}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "rencana_mulai", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Modal Awal
                        </label>
                        <input
                          type="text"
                          value={plan.modal_awal}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "modal_awal", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Sumber Modal
                        </label>
                        <input
                          type="text"
                          value={plan.sumber_modal}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "sumber_modal", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Lokasi (Opsional)
                        </label>
                        <input
                          type="text"
                          value={plan.lokasi ?? ""}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "lokasi", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Platform (Opsional)
                        </label>
                        <input
                          type="text"
                          value={plan.platform ?? ""}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "platform", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                          Catatan (Opsional)
                        </label>
                        <textarea
                          value={plan.catatan ?? ""}
                          onChange={(event) =>
                            updateWirausahaPlan(index, "catatan", event.target.value)
                          }
                          disabled={!canEditWirausaha}
                          className="min-h-[80px] w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      </div>
                    </div>
                  ))}
                </div>

                {wirausahaSaveMessage ? (
                  <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                    {wirausahaSaveMessage}
                  </p>
                ) : null}
                {wirausahaSaveError ? (
                  <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                    {wirausahaSaveError}
                  </p>
                ) : null}
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={closeWirausahaDetail}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
              >
                Tutup
              </button>
              {canEditWirausaha ? (
                <button
                  type="button"
                  onClick={handleWirausahaSave}
                  disabled={isWirausahaSaving}
                  className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {isWirausahaSaving ? "Menyimpan..." : "Simpan"}
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </AdminRouteGuard>
  );
}

