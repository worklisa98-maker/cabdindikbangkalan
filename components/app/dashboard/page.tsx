"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { useRouter } from "next/navigation";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  type PieLabelRenderProps,
} from "recharts";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { Card, CardContent } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { jenjangOptions } from "@/lib/shared/jenjangOptions";
import { fetchLastUpdatedMeta } from "@/lib/meta";
import { UpdateToast } from "@/components/update-toast";

type DashboardTotals = {
  totalSchools: number;
  totalGuru: number;
  totalTendik: number;
  avgGuruPerSchool: number;
  percentAkreditasiAB: number;
};

type DashboardKPI = {
  totalSchools: number;
  totalGuru: number;
  totalTendik: number;
  percentAkreditasiAB: number;
};

type AkreditasiCounts = {
  A: number;
  B: number;
  C: number;
  BELUM: number;
};

type TopSekolahItem = {
  namaSekolah: string;
  value: number;
};

type AkreditasiByJenjang = {
  jenjang: string;
  A: number;
  B: number;
  C: number;
  Belum: number;
};

type AkreditasiComposition = {
  label: string;
  total: number;
};

type StatusByJenjang = {
  jenjang: string;
  Negeri: number;
  Swasta: number;
};

type JenjangSummary = {
  jenjang: string;
  totalSekolah: number;
  totalGuru: number;
  totalTendik: number;
  percentAkreditasiAB: number;
};

type DashboardResponse = {
  kpi: DashboardKPI;
  akreditasi: AkreditasiCounts;
  pctAB: number;
  topSekolah: TopSekolahItem[];
  totals: DashboardTotals;
  akreditasiPerJenjang: AkreditasiByJenjang[];
  akreditasiComposition: AkreditasiComposition[];
  statusByJenjang: StatusByJenjang[];
  guruVsTendik: { label: string; value: number }[];
  jenjangSummaries: JenjangSummary[];
  lastUpdated?: string | null;
};

type InternalDashboardExtras = {
  guruAsn: number;
  guruHonorer: number;
  tendikAsn: number;
  tendikHonorer: number;
  missingAkreditasi: number;
  missingTmt: number;
  missingNpsn: number;
  missingGuru: number;
  missingTendik: number;
};

type InternalDashboardResponse = DashboardResponse & {
  extras: InternalDashboardExtras;
};

type SchoolSettingsResponse = {
  school: {
    id: number;
    npsnSekolah?: string | null;
    namaSekolah?: string | null;
    bentukPendidikan?: string | null;
    statusSekolah?: string | null;
    tmtAkreditasi?: string | null;
    akreditasi?: string | null;
    jumlahTotalGuru?: number;
    jumlahTotalTendik?: number;
  };
};

const statusOptions = [
  { value: "NEGERI", label: "Negeri" },
  { value: "SWASTA", label: "Swasta" },
] as const;

const akreditasiColors: Record<string, string> = {
  A: "#0f766e",
  B: "#2563eb",
  C: "#f59e0b",
  BELUM: "#9ca3af",
};

const formatNumber = (value: number) =>
  new Intl.NumberFormat("id-ID").format(value);

const formatPercent = (value?: number) => {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "0.0%";
  }
  return `${value.toFixed(1)}%`;
};

const formatAkreditasiLabel = (label: string) =>
  label === "BELUM" ? "Belum Terakreditasi" : label;

const getIndexCategory = (value?: number) => {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return { label: "Belum Tersedia", color: "bg-zinc-300" };
  }
  if (value >= 80) {
    return { label: "Baik", color: "bg-emerald-500" };
  }
  if (value >= 60) {
    return { label: "Cukup", color: "bg-amber-400" };
  }
  return { label: "Perlu Perhatian", color: "bg-rose-500" };
};

const formatLastUpdated = (value?: string | null) => {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date);
};

const fetchDashboard = async (
  jenjang: string,
  status: string
): Promise<DashboardResponse> => {
  const params = new URLSearchParams();
  if (jenjang) params.set("jenjang", jenjang);
  if (status) params.set("status", status);
  const endpoint = params.toString()
    ? `/api/public/dashboard?${params.toString()}`
    : "/api/public/dashboard";
  const response = await apiFetch(endpoint, undefined, {
    skipAuthRedirect: true,
  });
  if (!response.ok) {
    throw new Error("Failed to fetch dashboard data.");
  }
  const payload = (await response.json()) as { data: DashboardResponse };
  return payload.data;
};

const fetchInternalDashboard = async (): Promise<InternalDashboardResponse> => {
  const response = await apiFetch("/api/internal/dashboard");
  if (!response.ok) {
    throw new Error("Failed to fetch dashboard data.");
  }
  const payload = (await response.json()) as {
    data: InternalDashboardResponse;
  };
  return payload.data;
};

const fetchSchoolSettings = async (): Promise<SchoolSettingsResponse> => {
  const response = await apiFetch("/api/user/settings");
  if (!response.ok) {
    throw new Error("Failed to fetch school settings.");
  }
  const payload = (await response.json()) as { data: SchoolSettingsResponse };
  return payload.data;
};

export default function DashboardPage() {
  const router = useRouter();
  const { status, user } = useAuth();
  const isAdmin = user?.role === "ADMIN" || user?.role === "ADMIN_MASTER";
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const [selectedJenjang, setSelectedJenjang] = useState<string>("ALL");
  const [selectedStatus, setSelectedStatus] = useState<string>("ALL");
  const [updateMessage, setUpdateMessage] = useState("");
  const updateMessageTimerRef = useRef<number | null>(null);
  const lastDataSekolahUpdatedAtRef = useRef<string | null>(null);
  const queryClient = useQueryClient();

  const showUpdateToast = useCallback(() => {
    setUpdateMessage("Data diperbarui.");
    if (updateMessageTimerRef.current) {
      window.clearTimeout(updateMessageTimerRef.current);
    }
    updateMessageTimerRef.current = window.setTimeout(() => {
      setUpdateMessage("");
    }, 3000);
  }, []);

  useEffect(() => {
    return () => {
      if (updateMessageTimerRef.current) {
        window.clearTimeout(updateMessageTimerRef.current);
      }
    };
  }, []);

  const {
    data: publicData,
    isLoading: isPublicLoading,
    isError: isPublicError,
  } = useQuery({
    queryKey: ["public-dashboard", selectedJenjang, selectedStatus],
    queryFn: () =>
      fetchDashboard(
        selectedJenjang === "ALL" ? "" : selectedJenjang,
        selectedStatus === "ALL" ? "" : selectedStatus
      ),
    enabled: status === "unauthenticated",
  });

  const {
    data: internalData,
    isLoading: isInternalLoading,
    isError: isInternalError,
  } = useQuery({
    queryKey: ["internal-dashboard", user?.id ?? "anon"],
    queryFn: fetchInternalDashboard,
    enabled: status === "authenticated" && isSchoolAdmin,
  });

  const {
    data: schoolSettings,
    isLoading: isSettingsLoading,
    isError: isSettingsError,
  } = useQuery({
    queryKey: ["school-settings", user?.id ?? "anon"],
    queryFn: fetchSchoolSettings,
    enabled: status === "authenticated" && isSchoolAdmin,
  });

  const shouldPollMeta = status === "unauthenticated";
  const { data: lastUpdatedMeta } = useQuery({
    queryKey: ["meta-last-updated"],
    queryFn: fetchLastUpdatedMeta,
    refetchInterval: 30000,
    refetchIntervalInBackground: true,
    enabled: shouldPollMeta,
  });

  useEffect(() => {
    if (!shouldPollMeta) return;
    let cancelled = false;
    const updatedAt = lastUpdatedMeta?.dataSekolahUpdatedAt ?? "";
    if (
      lastDataSekolahUpdatedAtRef.current &&
      updatedAt &&
      lastDataSekolahUpdatedAtRef.current !== updatedAt
    ) {
      void queryClient.invalidateQueries({ queryKey: ["public-dashboard"] }).then(() => {
        if (!cancelled) showUpdateToast();
      });
    }
    lastDataSekolahUpdatedAtRef.current = updatedAt;
    return () => { cancelled = true; };
  }, [
    shouldPollMeta,
    lastUpdatedMeta?.dataSekolahUpdatedAt,
    queryClient,
    showUpdateToast,
  ]);

  const publicSafe = useMemo(
    () => ({
      akreditasi: publicData?.akreditasi ?? {
        A: 0,
        B: 0,
        C: 0,
        BELUM: 0,
      },
      akreditasiComposition: (publicData?.akreditasiComposition ?? []).map(
        (item) => ({
          ...item,
          displayLabel: formatAkreditasiLabel(item.label),
          rawLabel: item.label,
        })
      ),
      guruVsTendik: (publicData?.guruVsTendik ?? []).map((item) => ({
        ...item,
        label:
          item.label === "Guru"
            ? "Guru (Pendidik)"
            : item.label === "Tendik"
              ? "Tenaga Kependidikan"
              : item.label,
      })),
      akreditasiPerJenjang: publicData?.akreditasiPerJenjang ?? [],
      statusByJenjang: publicData?.statusByJenjang ?? [],
      topSekolah: publicData?.topSekolah ?? [],
      jenjangSummaries: publicData?.jenjangSummaries ?? [],
    }),
    [publicData]
  );
  const hasAkreditasiPerJenjangData = useMemo(
    () =>
      publicSafe.akreditasiPerJenjang.some(
        (row) => row.A + row.B + row.C + row.Belum > 0
      ),
    [publicSafe.akreditasiPerJenjang]
  );
  const hasAkreditasiCompositionData = useMemo(
    () => publicSafe.akreditasiComposition.some((item) => item.total > 0),
    [publicSafe.akreditasiComposition]
  );
  const hasStatusByJenjangData = useMemo(
    () =>
      publicSafe.statusByJenjang.some(
        (row) => row.Negeri + row.Swasta > 0
      ),
    [publicSafe.statusByJenjang]
  );
  const hasGuruVsTendikData = useMemo(
    () => publicSafe.guruVsTendik.some((item) => item.value > 0),
    [publicSafe.guruVsTendik]
  );
  const hasJenjangSummariesChartData = useMemo(
    () =>
      publicSafe.jenjangSummaries.some(
        (row) => row.totalGuru > 0 || row.totalTendik > 0
      ),
    [publicSafe.jenjangSummaries]
  );
  const renderAkreditasiLabel = useMemo(
    () =>
      function RenderAkreditasiLabel({
        cx,
        cy,
        midAngle,
        innerRadius,
        outerRadius,
        percent,
        payload,
      }: PieLabelRenderProps) {
        if (
          cx == null ||
          cy == null ||
          midAngle == null ||
          innerRadius == null ||
          outerRadius == null
        ) {
          return null;
        }
        const typedPayload = payload as
          | (AkreditasiComposition & { displayLabel?: string })
          | undefined;
        if (!typedPayload || (percent ?? 0) < 0.05) return null;
        const radius = innerRadius + (outerRadius - innerRadius) * 0.55;
        const RADIAN = Math.PI / 180;
        const x = cx + radius * Math.cos(-midAngle * RADIAN);
        const y = cy + radius * Math.sin(-midAngle * RADIAN);
        return (
          <text
            x={x}
            y={y}
            fill="#ffffff"
            textAnchor="middle"
            dominantBaseline="central"
            fontSize={12}
            fontWeight={600}
          >
            {typedPayload.displayLabel ?? typedPayload.label}:{" "}
            {typedPayload.total}
          </text>
        );
      },
    []
  );
  const isMounted = useSyncExternalStore(
    (onStoreChange) => {
      onStoreChange();
      return () => {};
    },
    () => true,
    () => false
  );

  useEffect(() => {
    if (status === "authenticated" && isAdmin) {
      router.replace("/internal/dashboard");
    }
  }, [status, isAdmin, router]);

  if (
    !isMounted ||
    status === "loading" ||
    (status === "authenticated" && isAdmin)
  ) {
    return (
      <>
        <UpdateToast message={updateMessage} />
        <div className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }).map((_, index) => (
              <div
                key={`kpi-skeleton-${index}`}
                className="h-24 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100"
              />
            ))}
          </div>
          <div className="h-56 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-64 lg:h-72" />
          <div className="grid gap-4 md:grid-cols-2">
            <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72 lg:h-80" />
            <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72 lg:h-80" />
          </div>
        </div>
      </>
    );
  }

  if (status === "authenticated" && isSchoolAdmin) {
    if (isInternalLoading || isSettingsLoading) {
      return (
        <div className="rounded-xl border border-dashed border-zinc-200 bg-white p-6 text-sm text-zinc-500">
          Loading dashboard data...
        </div>
      );
    }
    if (
      isInternalError ||
      isSettingsError ||
      !internalData ||
      !schoolSettings
    ) {
      return (
        <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
          Unable to load data right now.
        </div>
      );
    }

    const school = schoolSettings.school;
    const akreditasi = formatAkreditasiLabel(
      school.akreditasi?.trim() || "BELUM"
    );
    const tmt = school.tmtAkreditasi?.trim() || "-";
    const hasAsnHonorerData =
      internalData.extras.guruAsn +
        internalData.extras.guruHonorer +
        internalData.extras.tendikAsn +
        internalData.extras.tendikHonorer >
      0;

    return (
      <>
        <UpdateToast message={updateMessage} />
        <section className="space-y-6">
          <header className="space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Cabang Dinas Pendidikan Wilayah Bangkalan
            </p>
            <h1 className="text-3xl font-semibold">
              Dashboard Akreditasi Sekolah
            </h1>
            <p className="text-sm text-zinc-600">
              {school.namaSekolah ?? "Sekolah"}{" "}
              {school.npsnSekolah
                ? `• NPSN (Nomor Pokok Sekolah Nasional) ${school.npsnSekolah}`
                : ""}
            </p>
          </header>

          <section className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <Card>
              <CardContent className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Akreditasi
                </p>
                <p className="text-2xl font-semibold text-zinc-900">
                  {akreditasi}
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  TMT Akreditasi
                </p>
                <p className="text-2xl font-semibold text-zinc-900">{tmt}</p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Guru (Pendidik)
                </p>
                <p className="text-2xl font-semibold text-zinc-900">
                  {internalData.totals.totalGuru}
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="space-y-1">
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tenaga Kependidikan
                </p>
                <p className="text-2xl font-semibold text-zinc-900">
                  {internalData.totals.totalTendik}
                </p>
              </CardContent>
            </Card>
          </section>

          <section className="grid gap-6 lg:grid-cols-2">
            <Card>
              <CardContent className="space-y-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    ASN/Yayasan vs Honorer
                  </p>
                  <p className="text-sm font-semibold text-zinc-700">
                    Komposisi tenaga pendidik dan kependidikan
                  </p>
                </div>
                <div className="h-64 w-full sm:h-72 lg:h-80">
                  {hasAsnHonorerData ? (
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart
                        data={[
                          {
                            label: "Guru (Pendidik)",
                            ASN: internalData.extras.guruAsn,
                            Honorer: internalData.extras.guruHonorer,
                          },
                          {
                            label: "Tenaga Kependidikan",
                            ASN: internalData.extras.tendikAsn,
                            Honorer: internalData.extras.tendikHonorer,
                          },
                        ]}
                      >
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="label" />
                        <YAxis />
                        <Tooltip />
                        <Legend />
                        <Bar
                          dataKey="ASN"
                          name="ASN/Yayasan"
                          fill="#2563eb"
                        />
                        <Bar dataKey="Honorer" name="Honorer" fill="#f59e0b" />
                      </BarChart>
                    </ResponsiveContainer>
                  ) : (
                    <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                      Data belum tersedia
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="space-y-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Ringkasan Sekolah
                  </p>
                  <p className="text-sm font-semibold text-zinc-700">
                    Detail ringkas sekolah Anda
                  </p>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[720px] text-sm">
                    <thead>
                      <tr className="text-left text-zinc-500">
                        <th className="py-2">Sekolah</th>
                        <th className="py-2">Jenjang</th>
                        <th className="py-2">Status</th>
                        <th className="py-2 text-right">Guru (Pendidik)</th>
                        <th className="py-2 text-right">Tenaga Kependidikan</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr className="border-t">
                        <td className="py-2 font-semibold text-zinc-900">
                          {school.namaSekolah ?? "-"}
                        </td>
                        <td className="py-2">
                          {school.bentukPendidikan ?? "-"}
                        </td>
                        <td className="py-2">{school.statusSekolah ?? "-"}</td>
                        <td className="py-2 text-right">
                          {internalData.totals.totalGuru}
                        </td>
                        <td className="py-2 text-right">
                          {internalData.totals.totalTendik}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>
          </section>
        </section>
      </>
    );
  }

  const indexValue =
    publicData?.pctAB ?? publicData?.kpi?.percentAkreditasiAB ?? 0;

  return (
    <>
      <UpdateToast message={updateMessage} />
      <section className="space-y-6">
        <header className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Cabang Dinas Pendidikan Wilayah Bangkalan
          </p>
          <h1 className="text-3xl font-semibold">Dashboard Akreditasi</h1>
          <p className="text-sm text-zinc-600">
            Ringkasan data sekolah, guru, tendik, dan akreditasi.
          </p>
        </header>

        <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-end">
          <div className="space-y-1">
            <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Jenjang
            </label>
            <Select value={selectedJenjang} onValueChange={setSelectedJenjang}>
              <SelectTrigger className="w-full md:w-40">
                <SelectValue placeholder="Semua" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ALL">Semua</SelectItem>
                {jenjangOptions.map((jenjang) => (
                  <SelectItem key={jenjang} value={jenjang}>
                    {jenjang}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Status
            </label>
            <Select value={selectedStatus} onValueChange={setSelectedStatus}>
              <SelectTrigger className="w-full md:w-40">
                <SelectValue placeholder="Semua" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ALL">Semua</SelectItem>
                {statusOptions.map((status) => (
                  <SelectItem key={status.value} value={status.value}>
                    {status.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        {isPublicLoading ? (
          <div className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
              {Array.from({ length: 4 }).map((_, index) => (
                <div
                  key={`kpi-skeleton-${index}`}
                  className="h-24 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100"
                />
              ))}
            </div>
            <div className="h-56 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-64 lg:h-72" />
            <div className="grid gap-4 md:grid-cols-2">
              <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72 lg:h-80" />
              <div className="h-64 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100 sm:h-72 lg:h-80" />
            </div>
          </div>
        ) : isPublicError || !publicData ? (
          <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
            Unable to load data right now.
          </div>
        ) : (
          <div className="space-y-6">
            {publicData.kpi.totalSchools === 0 ? (
              <div className="rounded-xl border border-dashed border-zinc-200 bg-white p-6 text-sm text-zinc-500">
                Data belum tersedia atau belum tervalidasi.
              </div>
            ) : null}
            <section className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
              {[
                {
                  label: "Total Sekolah",
                  value: formatNumber(publicData.kpi.totalSchools),
                },
                {
                  label: "Guru (Pendidik)",
                  value: formatNumber(publicData.kpi.totalGuru),
                },
                {
                  label: "Tenaga Kependidikan",
                  value: formatNumber(publicData.kpi.totalTendik),
                },
                {
                  label: "% Akreditasi A+B",
                  value: formatPercent(publicData.kpi?.percentAkreditasiAB),
                },
              ].map((item) => (
                <Card key={item.label}>
                  <CardContent className="space-y-1">
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      {item.label}
                    </p>
                    <p className="text-3xl font-semibold text-zinc-900">
                      {item.value}
                    </p>
                  </CardContent>
                </Card>
              ))}
            </section>

            <section className="grid gap-6">
              <Card className="lg:col-span-1">
                <CardContent className="space-y-4">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Indeks Mutu Pendidikan Wilayah
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Persentase sekolah terakreditasi A dan B (BAN-S/M)
                    </p>
                  </div>
                  <div className="space-y-4">
                    <div className="text-4xl font-semibold text-zinc-900">
                      {formatPercent(indexValue)}
                    </div>
                    <div className="h-3 w-full overflow-hidden rounded-full bg-zinc-100">
                      <div
                        className={`h-full rounded-full ${
                          getIndexCategory(indexValue).color
                        }`}
                        style={{
                          width: `${Math.min(100, Math.max(0, indexValue))}%`,
                        }}
                      />
                    </div>
                    <p className="text-xs text-zinc-600">
                      Kategori: {getIndexCategory(indexValue).label}
                    </p>
                    <div className="grid grid-cols-2 gap-2 text-xs text-zinc-600">
                      <span>A: {publicSafe.akreditasi.A}</span>
                      <span>B: {publicSafe.akreditasi.B}</span>
                      <span>C: {publicSafe.akreditasi.C}</span>
                      <span>
                        Belum Terakreditasi: {publicSafe.akreditasi.BELUM}
                      </span>
                    </div>
                    <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-3 text-xs text-zinc-600">
                      <p className="font-semibold text-zinc-700">
                        Metode Perhitungan
                      </p>
                      <p className="mt-1">
                        Capaian ini menggunakan indikator persentase sekolah
                        terakreditasi A dan B sesuai standar BAN-S/M.
                      </p>
                      <p className="mt-1">
                        Distribusi kategori akreditasi disajikan sebagai data
                        pendukung.
                      </p>
                      <p className="mt-1">
                        Indikator ini merupakan ringkasan capaian wilayah, bukan
                        penilaian individu sekolah.
                      </p>
                      <p className="mt-1">
                        Total sekolah: {publicData.kpi.totalSchools}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </section>

            <p className="text-xs text-zinc-500">
              Terakhir diperbarui: {formatLastUpdated(publicData.lastUpdated)}
            </p>

            <section className="grid gap-6 lg:grid-cols-2">
              <Card>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Akreditasi per Jenjang
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Distribusi akreditasi sekolah per jenjang
                    </p>
                  </div>
                  <div className="h-64 w-full sm:h-72 lg:h-80">
                    {hasAkreditasiPerJenjangData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart data={publicSafe.akreditasiPerJenjang}>
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis dataKey="jenjang" />
                          <YAxis />
                          <Tooltip />
                          <Legend />
                          {["A", "B", "C", "Belum"].map((key) => (
                            <Bar
                              key={key}
                              dataKey={key}
                              stackId="akreditasi"
                              name={
                                key === "Belum" ? "Belum Terakreditasi" : key
                              }
                              fill={
                                akreditasiColors[key.toUpperCase()] ?? "#94a3b8"
                              }
                            />
                          ))}
                        </BarChart>
                      </ResponsiveContainer>
                    ) : (
                      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                        Data belum tersedia
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Komposisi Akreditasi
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Perbandingan keseluruhan akreditasi
                    </p>
                  </div>
                  <div className="h-64 w-full sm:h-72 lg:h-80">
                    {hasAkreditasiCompositionData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <PieChart>
                          <Pie
                            data={publicSafe.akreditasiComposition}
                            dataKey="total"
                            nameKey="displayLabel"
                            innerRadius={45}
                            outerRadius={90}
                            paddingAngle={2}
                            labelLine={false}
                            label={renderAkreditasiLabel}
                          >
                            {publicSafe.akreditasiComposition.map((entry) => (
                              <Cell
                                key={entry.label}
                                fill={
                                  akreditasiColors[
                                    (entry as { rawLabel?: string }).rawLabel ??
                                      entry.label
                                  ] ?? "#94a3b8"
                                }
                              />
                            ))}
                          </Pie>
                          <Tooltip />
                        </PieChart>
                      </ResponsiveContainer>
                    ) : (
                      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                        Data belum tersedia
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Negeri vs Swasta
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Perbandingan status sekolah per jenjang
                    </p>
                  </div>
                  <div className="h-64 w-full sm:h-72 lg:h-80">
                    {hasStatusByJenjangData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart data={publicSafe.statusByJenjang}>
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis dataKey="jenjang" />
                          <YAxis />
                          <Tooltip />
                          <Legend />
                          <Bar dataKey="Negeri" fill="#0f766e" />
                          <Bar dataKey="Swasta" fill="#94a3b8" />
                        </BarChart>
                      </ResponsiveContainer>
                    ) : (
                      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                        Data belum tersedia
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Guru (Pendidik) vs Tenaga Kependidikan
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Perbandingan total pendidik dan tenaga kependidikan
                    </p>
                  </div>
                  <div className="h-64 w-full sm:h-72 lg:h-80">
                    {hasGuruVsTendikData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart
                          data={publicSafe.guruVsTendik}
                          layout="vertical"
                        >
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis type="number" />
                          <YAxis dataKey="label" type="category" />
                          <Tooltip />
                          <Bar dataKey="value" fill="#1f2937" />
                        </BarChart>
                      </ResponsiveContainer>
                    ) : (
                      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                        Data belum tersedia
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
            </section>

            <section className="grid gap-6 lg:grid-cols-2">
              <Card>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Top 10 Sekolah Paling Berprestasi
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Rekap jumlah prestasi per sekolah
                    </p>
                  </div>
                  <div className="space-y-2">
                    {publicSafe.topSekolah.length === 0 ? (
                      <p className="text-sm text-zinc-500">
                        Belum ada data prestasi.
                      </p>
                    ) : (
                      publicSafe.topSekolah.map((row, index) => (
                        <div
                          key={`${row.namaSekolah}-${index}`}
                          className="flex items-center justify-between rounded-lg border border-zinc-100 bg-white px-3 py-2 text-sm"
                        >
                          <span className="font-semibold text-zinc-900">
                            {index + 1}. {row.namaSekolah}
                          </span>
                          <span className="text-zinc-600">{row.value}</span>
                        </div>
                      ))
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="space-y-3">
                  <div>
                    <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                      Guru (Pendidik) vs Tenaga Kependidikan per Jenjang
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Perbandingan total pendidik dan tenaga kependidikan per
                      jenjang
                    </p>
                  </div>
                  <div className="h-64 w-full sm:h-72 lg:h-80">
                    {hasJenjangSummariesChartData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart
                          data={publicSafe.jenjangSummaries.map((row) => ({
                            jenjang: row.jenjang,
                            Guru: row.totalGuru,
                            Tendik: row.totalTendik,
                          }))}
                          layout="vertical"
                        >
                          <CartesianGrid strokeDasharray="3 3" />
                          <XAxis type="number" />
                          <YAxis dataKey="jenjang" type="category" />
                          <Tooltip />
                          <Legend />
                          <Bar
                            dataKey="Guru"
                            name="Guru (Pendidik)"
                            fill="#0f766e"
                          />
                          <Bar
                            dataKey="Tendik"
                            name="Tenaga Kependidikan"
                            fill="#1f2937"
                          />
                        </BarChart>
                      </ResponsiveContainer>
                    ) : (
                      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                        Data belum tersedia
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
            </section>

            <Card>
              <CardContent className="space-y-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Ringkasan Jenjang
                  </p>
                  <p className="text-sm font-semibold text-zinc-700">
                    Ringkasan tanpa data sensitif
                  </p>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[720px] text-sm">
                    <thead>
                      <tr className="text-left text-zinc-500">
                        <th className="py-2">Jenjang</th>
                        <th className="py-2 text-right">Total Sekolah</th>
                        <th className="py-2 text-right">Guru (Pendidik)</th>
                        <th className="py-2 text-right">Tenaga Kependidikan</th>
                        <th className="py-2 text-right">% A+B</th>
                      </tr>
                    </thead>
                    <tbody>
                      {publicSafe.jenjangSummaries.length === 0 ? (
                        <tr>
                          <td
                            className="py-4 text-center text-zinc-500"
                            colSpan={5}
                          >
                            Belum ada data.
                          </td>
                        </tr>
                      ) : (
                        publicSafe.jenjangSummaries.map((row) => (
                          <tr key={row.jenjang} className="border-t">
                            <td className="py-2 font-semibold text-zinc-900">
                              {row.jenjang}
                            </td>
                            <td className="py-2 text-right">
                              {row.totalSekolah}
                            </td>
                            <td className="py-2 text-right">{row.totalGuru}</td>
                            <td className="py-2 text-right">
                              {row.totalTendik}
                            </td>
                            <td className="py-2 text-right">
                              {formatPercent(row.percentAkreditasiAB)}
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>
          </div>
        )}
      </section>
    </>
  );
}
