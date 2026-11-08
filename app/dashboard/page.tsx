"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactElement,
} from "react";
import { useRouter, useSearchParams } from "next/navigation";
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
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { jenjangOptions, type JenjangOption } from "@/lib/shared/jenjangOptions";
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

type StatusOption = (typeof statusOptions)[number]["value"];
type JenjangFilter = JenjangOption | "ALL";
type StatusFilter = StatusOption | "ALL";

const isJenjangValue = (value: string): value is JenjangOption =>
  jenjangOptions.some((option) => option === value);

const isStatusValue = (value: string): value is StatusOption =>
  statusOptions.some((option) => option.value === value);

const toJenjangFilter = (value: string): JenjangFilter =>
  isJenjangValue(value) ? value : "ALL";

const toStatusFilter = (value: string): StatusFilter =>
  isStatusValue(value) ? value : "ALL";

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
  jenjang: JenjangOption | "",
  status: StatusOption | ""
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
  const searchParams = useSearchParams();
  const { status, user } = useAuth();
  const isExportMode = searchParams.get("export") === "1";
  const isAdmin = user?.role === "ADMIN" || user?.role === "ADMIN_MASTER";
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const [selectedJenjang, setSelectedJenjang] = useState<JenjangFilter>("ALL");
  const [selectedStatus, setSelectedStatus] = useState<StatusFilter>("ALL");
  const [isPdfDownloading, setIsPdfDownloading] = useState(false);
  const [pdfError, setPdfError] = useState("");
  const [updateMessage, setUpdateMessage] = useState("");
  const [isReportReady, setIsReportReady] = useState(false);
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

  useEffect(() => {
    if (!isExportMode) {
      return;
    }
    const jenjangParam = (searchParams.get("jenjang") ?? "")
      .trim()
      .toUpperCase();
    const statusParam = (searchParams.get("status") ?? "").trim().toUpperCase();
    const nextJenjang = toJenjangFilter(jenjangParam);
    const nextStatus = toStatusFilter(statusParam);
    setSelectedJenjang(nextJenjang);
    setSelectedStatus(nextStatus);
  }, [isExportMode, searchParams]);

  const handleJenjangChange = useCallback((value: string) => {
    setSelectedJenjang(toJenjangFilter(value));
  }, []);

  const handleStatusChange = useCallback((value: string) => {
    setSelectedStatus(toStatusFilter(value));
  }, []);

  const shouldFetchPublicDashboard =
    status === "unauthenticated" ||
    (isExportMode && status === "authenticated" && !isSchoolAdmin);

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
    enabled: shouldFetchPublicDashboard,
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
    const updatedAt = lastUpdatedMeta?.dataSekolahUpdatedAt ?? "";
    if (
      lastDataSekolahUpdatedAtRef.current &&
      updatedAt &&
      lastDataSekolahUpdatedAtRef.current !== updatedAt
    ) {
      void queryClient.invalidateQueries({ queryKey: ["public-dashboard"] });
      showUpdateToast();
    }
    lastDataSekolahUpdatedAtRef.current = updatedAt;
  }, [
    shouldPollMeta,
    lastUpdatedMeta?.dataSekolahUpdatedAt,
    queryClient,
    showUpdateToast,
  ]);

  const canDownloadAkreditasiPdf =
    status === "authenticated" &&
    (user?.role === "ADMIN" ||
      user?.role === "ADMIN_MASTER" ||
      user?.role === "SCHOOL_ADMIN" ||
      user?.role === "PENGAWAS");

  const handleDownloadAkreditasiPdf = async () => {
    if (!canDownloadAkreditasiPdf) return;
    setIsPdfDownloading(true);
    setPdfError("");
    try {
      const query = new URLSearchParams();
      if (selectedJenjang !== "ALL") {
        query.set("jenjang", selectedJenjang);
      }
      if (selectedStatus !== "ALL") {
        query.set("status", selectedStatus);
      }
      const response = await apiFetch(
        `/api/reports/akreditasi.pdf${query.toString() ? `?${query.toString()}` : ""}`
      );
      if (!response.ok) {
        throw new Error("akreditasi_pdf_failed");
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.setAttribute("download", "akreditasi.pdf");
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch {
      setPdfError("Gagal mengunduh PDF akreditasi.");
    } finally {
      setIsPdfDownloading(false);
    }
  };

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
    () => publicSafe.statusByJenjang.some((row) => row.Negeri + row.Swasta > 0),
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

  const renderReportRoot = (content: ReactElement) => (
    <div
      id="report-root"
      data-report-ready={isReportReady ? "1" : "0"}
      className={isExportMode ? "w-full max-w-none" : "w-full"}
    >
      {content}
    </div>
  );

  const isReportLoading =
    !isMounted ||
    status === "loading" ||
    (status === "authenticated" && isAdmin && !isExportMode) ||
    (status === "authenticated" &&
      isSchoolAdmin &&
      (isInternalLoading || isSettingsLoading)) ||
    (!isSchoolAdmin && shouldFetchPublicDashboard && isPublicLoading);

  useEffect(() => {
    let cancelled = false;
    requestAnimationFrame(() => {
      if (cancelled) {
        return;
      }
      if (isReportLoading) {
        setIsReportReady(false);
        return;
      }
      requestAnimationFrame(() => {
        if (!cancelled) {
          setIsReportReady(true);
        }
      });
    });

    return () => {
      cancelled = true;
    };
  }, [isReportLoading]);

  useEffect(() => {
    if (status === "authenticated" && isAdmin && !isExportMode) {
      router.replace("/internal/dashboard");
    }
  }, [status, isAdmin, isExportMode, router]);

  if (
    !isMounted ||
    status === "loading" ||
    (status === "authenticated" && isAdmin && !isExportMode)
  ) {
    return renderReportRoot(
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
      return renderReportRoot(
        <EmptyState
          title="Memuat dashboard"
          description="Sedang mengambil ringkasan dashboard sekolah."
          className="min-h-24"
        />
      );
    }
    if (
      isInternalError ||
      isSettingsError ||
      !internalData ||
      !schoolSettings
    ) {
      return renderReportRoot(
        <EmptyState
          title="Gagal memuat dashboard"
          description="Unable to load data right now."
          className="min-h-24 border-red-200 bg-red-50/30"
        />
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

    return renderReportRoot(
      <>
        <UpdateToast message={updateMessage} />
        <section className="space-y-8">
          <PageHeader
            eyebrow="Cabang Dinas Pendidikan Wilayah Bangkalan"
            title="Dashboard Akreditasi Sekolah"
            description={
              school.npsnSekolah
                ? `${school.namaSekolah ?? "Sekolah"} - NPSN (Nomor Pokok Sekolah Nasional) ${school.npsnSekolah}`
                : (school.namaSekolah ?? "Sekolah")
            }
            actions={
              canDownloadAkreditasiPdf ? (
                <button
                  type="button"
                  onClick={() => void handleDownloadAkreditasiPdf()}
                  disabled={isPdfDownloading}
                  className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isPdfDownloading ? "Menyiapkan..." : "Unduh PDF"}
                </button>
              ) : null
            }
          />
          {pdfError ? <p className="text-xs text-red-600">{pdfError}</p> : null}
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
                <Table className="min-w-[720px] md:min-w-full">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Sekolah</TableHead>
                      <TableHead>Jenjang</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">
                        Guru (Pendidik)
                      </TableHead>
                      <TableHead className="text-right">
                        Tenaga Kependidikan
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow>
                      <TableCell className="font-semibold text-zinc-900">
                        {school.namaSekolah ?? "-"}
                      </TableCell>
                      <TableCell>{school.bentukPendidikan ?? "-"}</TableCell>
                      <TableCell>{school.statusSekolah ?? "-"}</TableCell>
                      <TableCell className="text-right">
                        {internalData.totals.totalGuru}
                      </TableCell>
                      <TableCell className="text-right">
                        {internalData.totals.totalTendik}
                      </TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </section>
        </section>
      </>
    );
  }

  const indexValue =
    publicData?.pctAB ?? publicData?.kpi?.percentAkreditasiAB ?? 0;

  return renderReportRoot(
    <>
      <UpdateToast message={updateMessage} />
      <section className="space-y-8">
        <PageHeader
          eyebrow="Cabang Dinas Pendidikan Wilayah Bangkalan"
          title="Dashboard Akreditasi"
          description="Ringkasan data sekolah, guru, tendik, dan akreditasi."
          actions={
            canDownloadAkreditasiPdf ? (
              <button
                type="button"
                onClick={() => void handleDownloadAkreditasiPdf()}
                disabled={isPdfDownloading}
                className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isPdfDownloading ? "Menyiapkan..." : "Unduh PDF"}
              </button>
            ) : null
          }
        />
        {pdfError ? <p className="text-xs text-red-600">{pdfError}</p> : null}

        <Card>
          <CardContent className="p-4">
            <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-end">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Jenjang
                </label>
                <Select
                  value={selectedJenjang}
                  onValueChange={handleJenjangChange}
                >
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
                <Select
                  value={selectedStatus}
                  onValueChange={handleStatusChange}
                >
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
          </CardContent>
        </Card>

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
          <EmptyState
            title="Gagal memuat dashboard"
            description="Unable to load data right now."
            className="border-red-200 bg-red-50/30"
          />
        ) : (
          <div className="space-y-6">
            {publicData.kpi.totalSchools === 0 ? (
              <EmptyState
                title="Data belum tersedia"
                description="Data belum tersedia atau belum tervalidasi."
                className="min-h-24"
              />
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
                <Table className="min-w-[720px] md:min-w-full">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Jenjang</TableHead>
                      <TableHead className="text-right">
                        Total Sekolah
                      </TableHead>
                      <TableHead className="text-right">
                        Guru (Pendidik)
                      </TableHead>
                      <TableHead className="text-right">
                        Tenaga Kependidikan
                      </TableHead>
                      <TableHead className="text-right">% A+B</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {publicSafe.jenjangSummaries.length === 0 ? (
                      <TableRow>
                        <TableCell
                          className="py-4 text-center text-zinc-500"
                          colSpan={5}
                        >
                          Belum ada data.
                        </TableCell>
                      </TableRow>
                    ) : (
                      publicSafe.jenjangSummaries.map((row) => (
                        <TableRow key={row.jenjang}>
                          <TableCell className="font-semibold text-zinc-900">
                            {row.jenjang}
                          </TableCell>
                          <TableCell className="text-right">
                            {row.totalSekolah}
                          </TableCell>
                          <TableCell className="text-right">
                            {row.totalGuru}
                          </TableCell>
                          <TableCell className="text-right">
                            {row.totalTendik}
                          </TableCell>
                          <TableCell className="text-right">
                            {formatPercent(row.percentAkreditasiAB)}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        )}
      </section>
    </>
  );
}
