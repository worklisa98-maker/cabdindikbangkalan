"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  LabelList,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  type LabelProps,
  type PieLabelRenderProps,
  type TooltipContentProps,
} from "recharts";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";
import {
  canUseScopedSchoolFilter,
  isAdminMonitoringRole,
  isPengawasRole,
  isSchoolAdminRole,
} from "@/lib/auth/role-scope";

import { Card, CardContent } from "@/components/ui/card";
import { PageHeader } from "@/components/page-header";
import { TypeaheadInput } from "@/components/typeahead-input";
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
import { jenjangOptions } from "@/lib/shared/jenjangOptions";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
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

type InternalDashboardResponse = {
  kpi: DashboardKPI;
  akreditasi: AkreditasiCounts;
  pctAB: number;
  topSekolah: TopSekolahItem[];
  totals: DashboardTotals;
  akreditasiPerJenjang: AkreditasiByJenjang[];
  akreditasiComposition: { label: string; total: number }[];
  statusByJenjang: StatusByJenjang[];
  guruVsTendik: { label: string; value: number }[];
  jenjangSummaries: JenjangSummary[];
  extras: InternalDashboardExtras;
  internalIndex?: {
    value: number;
    label: string;
    disclaimer: boolean;
  } | null;
  lastUpdated?: string | null;
};

type SchoolOption = {
  id: number;
  namaSekolah: string;
  npsnSekolah?: string | null;
};

type MonitoringSchoolRow = {
  school_id: number;
  nama_sekolah: string;
  bentuk_pendidikan: string;
  akreditasi: string;
  guru_asn: number;
  guru_honorer: number;
  tendik_asn: number;
  tendik_honorer: number;
};

type MonitoringSortKey =
  | "nama_sekolah"
  | "bentuk_pendidikan"
  | "akreditasi"
  | "guru_asn"
  | "guru_honorer"
  | "tendik_asn"
  | "tendik_honorer";

const statusOptions = [
  { value: "NEGERI", label: "Negeri" },
  { value: "SWASTA", label: "Swasta" },
] as const;

const monitoringAkreditasiOptions = ["ALL", "A", "B", "C", "BELUM"] as const;

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

const formatFixed = (value?: number, digits = 1) => {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "0.0";
  }
  return value.toFixed(digits);
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
  schoolId?: number | null,
  jenjang?: string,
  status?: string
): Promise<InternalDashboardResponse> => {
  const params = new URLSearchParams();
  if (schoolId) params.set("school_id", String(schoolId));
  if (jenjang) params.set("jenjang", jenjang);
  if (status) params.set("status", status);
  const endpoint = params.toString()
    ? `/api/dashboard?${params.toString()}`
    : "/api/dashboard";
  const response = await apiFetch(endpoint);
  if (!response.ok) {
    throw new Error("Failed to fetch dashboard data.");
  }
  const payload = (await response.json()) as {
    data: InternalDashboardResponse;
  };
  return payload.data;
};

const fetchScopedSchools = async (): Promise<SchoolOption[]> => {
  const response = await apiFetch("/api/user/me/schools");
  if (!response.ok) {
    throw new Error("Failed to fetch scoped schools.");
  }
  const payload = (await response.json()) as {
    data: Array<{
      id: number;
      namaSekolah: string;
      npsnSekolah?: string | null;
    }>;
  };
  return (payload.data ?? []).map((item) => ({
    id: item.id,
    namaSekolah: item.namaSekolah,
    npsnSekolah: item.npsnSekolah ?? null,
  }));
};

const fetchMonitoringSchools = async (params: {
  jenjang: string;
  status: string;
  akreditasi: string;
  schoolId?: number | null;
  query: string;
  sortBy: MonitoringSortKey;
  order: "asc" | "desc";
  guruAsnMin?: string;
  guruAsnMax?: string;
  tendikAsnMin?: string;
  tendikAsnMax?: string;
}): Promise<MonitoringSchoolRow[]> => {
  const query = new URLSearchParams();
  if (params.jenjang && params.jenjang !== "ALL") {
    query.set("jenjang", params.jenjang);
  }
  if (params.status && params.status !== "ALL") {
    query.set("status", params.status);
  }
  if (params.akreditasi && params.akreditasi !== "ALL") {
    query.set("akreditasi", params.akreditasi);
  }
  if (params.schoolId) {
    query.set("schoolId", String(params.schoolId));
  } else if (params.query.trim().length >= 2) {
    query.set("q", params.query.trim());
  }
  if (params.guruAsnMin) {
    query.set("guruAsnMin", params.guruAsnMin);
  }
  if (params.guruAsnMax) {
    query.set("guruAsnMax", params.guruAsnMax);
  }
  if (params.tendikAsnMin) {
    query.set("tendikAsnMin", params.tendikAsnMin);
  }
  if (params.tendikAsnMax) {
    query.set("tendikAsnMax", params.tendikAsnMax);
  }
  query.set("sortBy", params.sortBy);
  query.set("order", params.order);
  query.set("limit", "10");

  const response = await apiFetch(
    `/api/admin/sekolah/table?${query.toString()}`
  );
  if (!response.ok) {
    throw new Error("Failed to fetch monitoring schools.");
  }
  const payload = (await response.json()) as { data: MonitoringSchoolRow[] };
  return payload.data ?? [];
};

const buildCsv = (rows: SchoolOption[]) => {
  const header = ["id", "nama_sekolah"];
  const lines = [header.join(",")];
  rows.forEach((row) => {
    lines.push(`${row.id},"${row.namaSekolah.replace(/"/g, '""')}"`);
  });
  return lines.join("\n");
};

export default function InternalDashboardPage() {
  const { status, user } = useAuth();
  const isAdmin = isAdminMonitoringRole(user?.role);
  const isSchoolAdmin = isSchoolAdminRole(user?.role);
  const isPengawas = isPengawasRole(user?.role);
  const canSelectSchool = canUseScopedSchoolFilter(user?.role);

  const [selectedSchoolId, setSelectedSchoolId] = useState<number | null>(null);
  const [selectedJenjang, setSelectedJenjang] = useState<string>("ALL");
  const [selectedStatus, setSelectedStatus] = useState<string>("ALL");
  const [isAkreditasiPdfDownloading, setIsAkreditasiPdfDownloading] =
    useState(false);
  const [akreditasiPdfError, setAkreditasiPdfError] = useState("");
  const [schoolSearch, setSchoolSearch] = useState<string>("");
  const [isSchoolDropdownOpen, setIsSchoolDropdownOpen] = useState(false);
  const [monitoringSchoolQuery, setMonitoringSchoolQuery] = useState("");
  const [monitoringSchoolId, setMonitoringSchoolId] = useState<number | null>(
    null
  );
  const [monitoringAkreditasi, setMonitoringAkreditasi] = useState("ALL");
  const [monitoringSortBy, setMonitoringSortBy] =
    useState<MonitoringSortKey>("nama_sekolah");
  const [monitoringOrder, setMonitoringOrder] = useState<"asc" | "desc">("asc");
  const [monitoringGuruAsnMin, setMonitoringGuruAsnMin] = useState("");
  const [monitoringGuruAsnMax, setMonitoringGuruAsnMax] = useState("");
  const [monitoringTendikAsnMin, setMonitoringTendikAsnMin] = useState("");
  const [monitoringTendikAsnMax, setMonitoringTendikAsnMax] = useState("");
  const [monitoringError, setMonitoringError] = useState("");
  const [isMonitoringExporting, setIsMonitoringExporting] = useState(false);
  const debouncedMonitoringQuery = useDebouncedValue(
    monitoringSchoolQuery,
    300
  );
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

  const isMounted = useSyncExternalStore(
    (onStoreChange) => {
      onStoreChange();
      return () => {};
    },
    () => true,
    () => false
  );

  const { data, isLoading, isError } = useQuery({
    queryKey: [
      "internal-dashboard",
      selectedSchoolId ?? "ALL",
      isSchoolAdmin ? "ALL" : selectedJenjang,
      selectedStatus,
    ],
    queryFn: () =>
      fetchDashboard(
        selectedSchoolId,
        isSchoolAdmin || selectedJenjang === "ALL"
          ? undefined
          : selectedJenjang,
        selectedStatus === "ALL" ? undefined : selectedStatus
      ),
    enabled: status === "authenticated",
  });

  const { data: schools = [] } = useQuery({
    queryKey: ["dashboard-schools"],
    queryFn: fetchScopedSchools,
    enabled: status === "authenticated" && canSelectSchool,
  });

  useEffect(() => {
    if (!canSelectSchool) {
      return;
    }
    if (schools.length === 0) {
      if (selectedSchoolId !== null) {
        setSelectedSchoolId(null);
      }
      if (schoolSearch !== "") {
        setSchoolSearch("");
      }
      return;
    }
    const hasCurrentSelection =
      selectedSchoolId != null && schools.some((school) => school.id === selectedSchoolId);
    if (selectedSchoolId != null && !hasCurrentSelection) {
      setSelectedSchoolId(null);
      setSchoolSearch("");
    }
  }, [canSelectSchool, schoolSearch, schools, selectedSchoolId]);

  const {
    data: monitoringRows = [],
    isLoading: isMonitoringLoading,
    isError: isMonitoringError,
  } = useQuery({
    queryKey: [
      "monitoring-sekolah",
      selectedJenjang,
      selectedStatus,
      monitoringAkreditasi,
      monitoringSchoolId ?? "ALL",
      debouncedMonitoringQuery,
      monitoringSortBy,
      monitoringOrder,
      monitoringGuruAsnMin,
      monitoringGuruAsnMax,
      monitoringTendikAsnMin,
      monitoringTendikAsnMax,
    ],
    queryFn: () =>
      fetchMonitoringSchools({
        jenjang: selectedJenjang,
        status: selectedStatus,
        akreditasi: monitoringAkreditasi,
        schoolId: monitoringSchoolId,
        query: debouncedMonitoringQuery,
        sortBy: monitoringSortBy,
        order: monitoringOrder,
        guruAsnMin: monitoringGuruAsnMin,
        guruAsnMax: monitoringGuruAsnMax,
        tendikAsnMin: monitoringTendikAsnMin,
        tendikAsnMax: monitoringTendikAsnMax,
      }),
    enabled: status === "authenticated" && isAdmin,
  });

  const shouldPollMeta = status === "authenticated" && isAdmin;
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
      void queryClient.invalidateQueries({ queryKey: ["internal-dashboard"] });
      void queryClient.invalidateQueries({ queryKey: ["monitoring-sekolah"] });
      void queryClient.invalidateQueries({ queryKey: ["dashboard-schools"] });
      showUpdateToast();
    }
    lastDataSekolahUpdatedAtRef.current = updatedAt;
  }, [
    shouldPollMeta,
    lastUpdatedMeta?.dataSekolahUpdatedAt,
    queryClient,
    showUpdateToast,
  ]);

  const safeData = useMemo(
    () => ({
      akreditasi: data?.akreditasi ?? {
        A: 0,
        B: 0,
        C: 0,
        BELUM: 0,
      },
      akreditasiComposition: (data?.akreditasiComposition ?? [])
        .map((item) => ({
          ...item,
          displayLabel: formatAkreditasiLabel(item.label),
          rawLabel: item.label,
        }))
        .sort((a, b) => {
          const order: Record<string, number> = {
            A: 0,
            B: 1,
            C: 2,
            BELUM: 3,
          };
          const aKey = (a.rawLabel ?? a.label ?? "").toUpperCase();
          const bKey = (b.rawLabel ?? b.label ?? "").toUpperCase();
          return (order[aKey] ?? 99) - (order[bKey] ?? 99);
        }),
      guruVsTendik: (data?.guruVsTendik ?? []).map((item) => ({
        ...item,
        label:
          item.label === "Guru"
            ? "Guru (Pendidik)"
            : item.label === "Tendik"
              ? "Tenaga Kependidikan"
              : item.label,
      })),
      topSekolah: data?.topSekolah ?? [],
      jenjangSummaries: data?.jenjangSummaries ?? [],
      akreditasiPerJenjang: data?.akreditasiPerJenjang ?? [],
      statusByJenjang: data?.statusByJenjang ?? [],
    }),
    [data]
  );
  const hasAkreditasiPerJenjangData = useMemo(
    () =>
      safeData.akreditasiPerJenjang.some(
        (row) => row.A + row.B + row.C + row.Belum > 0
      ),
    [safeData.akreditasiPerJenjang]
  );
  const hasStatusByJenjangData = useMemo(
    () =>
      safeData.statusByJenjang.some(
        (row) => row.Negeri + row.Swasta > 0
      ),
    [safeData.statusByJenjang]
  );
  const hasGuruVsTendikData = useMemo(
    () => safeData.guruVsTendik.some((item) => item.value > 0),
    [safeData.guruVsTendik]
  );
  const hasJenjangSummariesChartData = useMemo(
    () =>
      safeData.jenjangSummaries.some(
        (row) => row.totalGuru > 0 || row.totalTendik > 0
      ),
    [safeData.jenjangSummaries]
  );
  const filteredSchools = useMemo(() => {
    const term = schoolSearch.trim().toLowerCase();
    if (!term) {
      return schools.slice(0, 10);
    }
    return schools
      .map((school) => ({
        school,
        name: school.namaSekolah.trim().toLowerCase(),
      }))
      .filter((entry) => entry.name.includes(term))
      .sort((a, b) => {
        const aStarts = a.name.startsWith(term);
        const bStarts = b.name.startsWith(term);
        if (aStarts !== bStarts) {
          return aStarts ? -1 : 1;
        }
        return a.name.indexOf(term) - b.name.indexOf(term);
      })
      .slice(0, 10)
      .map((entry) => entry.school);
  }, [schoolSearch, schools]);
  const pieTotal = useMemo(
    () =>
      safeData.akreditasiComposition?.reduce(
        (sum, item) => sum + item.total,
        0
      ) ?? 0,
    [safeData]
  );
  const hasAkreditasiCompositionData = pieTotal > 0;
  const hasAsnHonorerData = useMemo(() => {
    if (!data) return false;
    return (
      data.extras.guruAsn +
        data.extras.guruHonorer +
        data.extras.tendikAsn +
        data.extras.tendikHonorer >
      0
    );
  }, [data]);
  const renderPieLabel = useMemo(
    () =>
      function RenderPieLabel({
        cx,
        cy,
        midAngle,
        innerRadius,
        outerRadius,
        percent,
        value,
        name,
      }: PieLabelRenderProps) {
        const numericValue = Number(value ?? 0);
        if (
          !isAdmin ||
          numericValue <= 0 ||
          cx == null ||
          cy == null ||
          midAngle == null ||
          innerRadius == null ||
          outerRadius == null
        ) {
          return null;
        }
        const radius = innerRadius + (outerRadius - innerRadius) * 0.55;
        const RADIAN = Math.PI / 180;
        const sin = Math.sin(-midAngle * RADIAN);
        const cos = Math.cos(-midAngle * RADIAN);
        const x = cx + radius * cos;
        const y = cy + radius * sin;
        const outerLabelRadius = outerRadius + 18;
        const xOuter = cx + outerLabelRadius * cos;
        const yOuter = cy + outerLabelRadius * sin;
        const percentLabel =
          pieTotal > 0 ? Math.round((numericValue / pieTotal) * 100) : 0;
        return (
          <g>
            {(percent ?? 0) >= 0.05 ? (
              <text
                x={x}
                y={y}
                fill="#ffffff"
                textAnchor="middle"
                dominantBaseline="central"
                fontSize={11}
                fontWeight={600}
              >
                {`${Math.round((percent ?? 0) * 100)}%`}
              </text>
            ) : null}
            <text
              x={xOuter}
              y={yOuter}
              fill="#111827"
              textAnchor={cos >= 0 ? "start" : "end"}
              dominantBaseline="central"
              fontSize={11}
              fontWeight={600}
            >
              {`${name ?? ""}: ${percentLabel}% (${numericValue})`}
            </text>
          </g>
        );
      },
    [isAdmin, pieTotal]
  );
  const renderPieTooltip = useMemo(
    () =>
      function RenderPieTooltip({
        active,
        payload,
      }: TooltipContentProps<number, string>) {
        if (!active || !payload || payload.length === 0) {
          return null;
        }
        const entry = payload[0];
        const value = Number(entry?.value ?? 0);
        const label =
          (entry?.name as string | undefined) ??
          (entry?.payload as { label?: string } | undefined)?.label ??
          "";
        const percent = pieTotal > 0 ? Math.round((value / pieTotal) * 100) : 0;
        return (
          <div className="rounded-md border border-zinc-200 bg-white px-3 py-2 text-xs text-zinc-700 shadow-sm">
            <p className="font-semibold text-zinc-900">{label}</p>
            <p>Jumlah: {value}</p>
            <p>Persen: {percent}%</p>
          </div>
        );
      },
    [pieTotal]
  );
  const getBarLabelColor = (key: string) => {
    if (key === "A" || key === "B") {
      return "#ffffff";
    }
    return "#111827";
  };
  const renderStackedLabel = (key: string) =>
    function RenderStackedLabel({ x, y, width, height, value }: LabelProps) {
      const numeric = Number(value ?? 0);
      if (!isAdmin || numeric < 2) {
        return null;
      }
      const parsedX = typeof x === "number" ? x : Number(x);
      const parsedY = typeof y === "number" ? y : Number(y);
      const parsedWidth = typeof width === "number" ? width : Number(width);
      const parsedHeight = typeof height === "number" ? height : Number(height);
      if (
        !Number.isFinite(parsedX) ||
        !Number.isFinite(parsedY) ||
        !Number.isFinite(parsedWidth) ||
        !Number.isFinite(parsedHeight) ||
        parsedHeight < 16
      ) {
        return null;
      }
      return (
        <text
          x={parsedX + parsedWidth / 2}
          y={parsedY + parsedHeight / 2}
          textAnchor="middle"
          dominantBaseline="central"
          fontSize={11}
          fontWeight={600}
          fill={getBarLabelColor(key)}
        >
          {numeric}
        </text>
      );
    };

  const monitoringSuggestionItems = useMemo(() => {
    const term = monitoringSchoolQuery.trim().toLowerCase();
    const filtered = term
      ? schools.filter((school) =>
          school.namaSekolah.toLowerCase().includes(term)
        )
      : schools;
    return [
      {
        key: "ALL",
        label: "Semua sekolah",
        value: "",
      },
      ...filtered.map((school) => ({
        key: school.id,
        label: school.namaSekolah,
        value: school.namaSekolah,
        subLabel: school.npsnSekolah ? `NPSN ${school.npsnSekolah}` : undefined,
      })),
    ];
  }, [monitoringSchoolQuery, schools]);

  const monitoringSortIndicator = (key: MonitoringSortKey) => {
    if (monitoringSortBy !== key) return "";
    return monitoringOrder === "asc" ? "^" : "v";
  };

  const handleMonitoringSort = (key: MonitoringSortKey) => {
    if (monitoringSortBy === key) {
      setMonitoringOrder((prev) => (prev === "asc" ? "desc" : "asc"));
      return;
    }
    setMonitoringSortBy(key);
    setMonitoringOrder("asc");
  };

  const handleMonitoringExport = async () => {
    setIsMonitoringExporting(true);
    setMonitoringError("");
    try {
      const query = new URLSearchParams();
      if (selectedJenjang && selectedJenjang !== "ALL") {
        query.set("jenjang", selectedJenjang);
      }
      if (selectedStatus && selectedStatus !== "ALL") {
        query.set("status", selectedStatus);
      }
      if (monitoringAkreditasi && monitoringAkreditasi !== "ALL") {
        query.set("akreditasi", monitoringAkreditasi);
      }
      if (monitoringSchoolId) {
        query.set("schoolId", String(monitoringSchoolId));
      } else if (debouncedMonitoringQuery.trim().length >= 2) {
        query.set("q", debouncedMonitoringQuery.trim());
      }
      if (monitoringGuruAsnMin) {
        query.set("guruAsnMin", monitoringGuruAsnMin);
      }
      if (monitoringGuruAsnMax) {
        query.set("guruAsnMax", monitoringGuruAsnMax);
      }
      if (monitoringTendikAsnMin) {
        query.set("tendikAsnMin", monitoringTendikAsnMin);
      }
      if (monitoringTendikAsnMax) {
        query.set("tendikAsnMax", monitoringTendikAsnMax);
      }
      query.set("sortBy", monitoringSortBy);
      query.set("order", monitoringOrder);

      const response = await apiFetch(
        `/api/admin/sekolah/table/export.xlsx${query.toString() ? `?${query.toString()}` : ""}`
      );
      if (!response.ok) {
        throw new Error("Failed to export monitoring data.");
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.setAttribute("download", "monitoring-data-sekolah.xlsx");
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch {
      setMonitoringError("Gagal mengunduh Excel monitoring sekolah.");
    } finally {
      setIsMonitoringExporting(false);
    }
  };

  const handleExport = () => {
    const csv = buildCsv(schools);
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.setAttribute("download", "sekolah.csv");
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  const canDownloadAkreditasiPdf = isAdmin || isPengawas || isSchoolAdmin;
  const handleDownloadAkreditasiPdf = async () => {
    if (!canDownloadAkreditasiPdf) return;
    setIsAkreditasiPdfDownloading(true);
    setAkreditasiPdfError("");
    try {
      const query = new URLSearchParams();
      if (!isSchoolAdmin && selectedJenjang !== "ALL") {
        query.set("jenjang", selectedJenjang);
      }
      if (selectedStatus !== "ALL") {
        query.set("status", selectedStatus);
      }
      if (selectedSchoolId) {
        query.set("school_id", String(selectedSchoolId));
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
      setAkreditasiPdfError("Gagal mengunduh PDF akreditasi.");
    } finally {
      setIsAkreditasiPdfDownloading(false);
    }
  };

  if (!isMounted || status === "loading") {
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

  if (status === "unauthenticated") {
    return null;
  }

  const schoolLabel = user?.namaInstansi?.trim();

  const indexValue = data?.pctAB ?? data?.kpi?.percentAkreditasiAB ?? 0;

  return (
    <>
      <UpdateToast message={updateMessage} />
      <section className="space-y-8">
        <PageHeader
          eyebrow="Cabang Dinas Pendidikan Wilayah Bangkalan"
          title="Dashboard Internal"
          description={
            isSchoolAdmin && schoolLabel
              ? `Sekolah: ${schoolLabel}`
              : "Ringkasan internal data akreditasi."
          }
          actions={
            isAdmin || isPengawas ? (
              <span className="rounded-full bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-2 py-0.5 text-[10px] font-semibold tracking-[0.2em] text-white uppercase">
                Mode Internal
              </span>
            ) : null
          }
        />

        <Card>
          <CardContent className="p-4">
            <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-end">
          {canSelectSchool && schools.length > 0 ? (
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Sekolah
              </label>
              <div className="relative w-56">
                <input
                  type="text"
                  value={schoolSearch}
                  onChange={(event) => {
                    setSchoolSearch(event.target.value);
                    setIsSchoolDropdownOpen(true);
                  }}
                  onFocus={() => setIsSchoolDropdownOpen(true)}
                  onBlur={() => {
                    window.setTimeout(
                      () => setIsSchoolDropdownOpen(false),
                      150
                    );
                  }}
                  placeholder="Cari sekolah..."
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900 shadow-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 focus:outline-none"
                  aria-label="Cari sekolah"
                />
                {selectedSchoolId ? (
                  <button
                    type="button"
                    onClick={() => {
                      setSelectedSchoolId(null);
                      setSchoolSearch("");
                      setIsSchoolDropdownOpen(true);
                    }}
                    className="absolute top-2 right-2 text-xs text-zinc-500 hover:text-zinc-800"
                  >
                    Reset
                  </button>
                ) : null}
                {isSchoolDropdownOpen ? (
                  <div className="absolute z-20 mt-2 max-h-64 w-full overflow-auto rounded-md border border-zinc-200 bg-white shadow-lg">
                    <button
                      type="button"
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => {
                        setSelectedSchoolId(null);
                        setSchoolSearch("");
                        setIsSchoolDropdownOpen(false);
                      }}
                      className="block w-full px-3 py-2 text-left text-sm text-zinc-600 hover:bg-zinc-100"
                    >
                      Semua sekolah
                    </button>
                    {filteredSchools.length === 0 ? (
                      <div className="px-3 py-2 text-sm text-zinc-500">
                        Sekolah tidak ditemukan
                      </div>
                    ) : (
                      filteredSchools.map((school) => (
                        <button
                          key={school.id}
                          type="button"
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => {
                            setSelectedSchoolId(school.id);
                            setSchoolSearch(school.namaSekolah);
                            setIsSchoolDropdownOpen(false);
                          }}
                          className="block w-full px-3 py-2 text-left text-sm text-zinc-800 hover:bg-zinc-100"
                        >
                          {school.namaSekolah}
                        </button>
                      ))
                    )}
                  </div>
                ) : null}
              </div>
            </div>
          ) : null}
          {!isSchoolAdmin ? (
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Jenjang
              </label>
              <Select
                value={selectedJenjang}
                onValueChange={setSelectedJenjang}
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
          ) : null}
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
          {isAdmin ? (
            <div className="flex items-end">
              <button
                type="button"
                onClick={handleExport}
                className="w-full rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 md:w-auto"
              >
                Export CSV
              </button>
            </div>
          ) : null}
          {canDownloadAkreditasiPdf ? (
            <div className="flex items-end">
              <button
                type="button"
                onClick={() => void handleDownloadAkreditasiPdf()}
                disabled={isAkreditasiPdfDownloading}
                className="w-full rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60 md:w-auto"
              >
                {isAkreditasiPdfDownloading ? "Menyiapkan..." : "Unduh PDF"}
              </button>
            </div>
          ) : null}
            </div>
            {akreditasiPdfError ? (
              <p className="mt-2 text-xs text-red-600">{akreditasiPdfError}</p>
            ) : null}
          </CardContent>
        </Card>

        {isAdmin && data ? (
          <section className="space-y-4 rounded-2xl border border-zinc-200 bg-white p-4 sm:p-6">
            <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Executive Dashboard
                </p>
                <h2 className="text-lg font-semibold text-zinc-900">
                  Ringkasan Mutu Pendidikan
                </h2>
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
              {[
                {
                  label: "Total Sekolah",
                  value: formatNumber(data.kpi.totalSchools),
                },
                {
                  label: "Guru (Pendidik)",
                  value: formatNumber(data.kpi.totalGuru),
                },
                {
                  label: "Tenaga Kependidikan",
                  value: formatNumber(data.kpi.totalTendik),
                },
                {
                  label: "% Akreditasi A+B",
                  value: formatPercent(data.kpi?.percentAkreditasiAB),
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
            </div>

            <div className="grid gap-6">
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
                      <span>A: {safeData.akreditasi.A}</span>
                      <span>B: {safeData.akreditasi.B}</span>
                      <span>C: {safeData.akreditasi.C}</span>
                      <span>
                        Belum Terakreditasi: {safeData.akreditasi.BELUM}
                      </span>
                    </div>
                    {isAdmin ? (
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
                          Indikator ini merupakan ringkasan capaian wilayah,
                          bukan penilaian individu sekolah.
                        </p>
                        <p className="mt-1">
                          Total sekolah: {data.kpi.totalSchools}
                        </p>
                      </div>
                    ) : null}
                    {isAdmin && data.internalIndex ? (
                      <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-3 text-xs text-amber-900">
                        <p className="font-semibold">
                          {data.internalIndex.label}
                        </p>
                        <p className="mt-1 text-sm font-semibold">
                          {formatFixed(data.internalIndex.value)}
                        </p>
                        <p className="mt-1">
                          Indeks ini merupakan olahan internal berbasis kategori
                          akreditasi BAN-S/M, bukan nilai resmi atau penilaian
                          pemerintah.
                        </p>
                      </div>
                    ) : null}
                  </div>
                </CardContent>
              </Card>
            </div>

            <div className="grid gap-6 lg:grid-cols-2">
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
                    {safeData.topSekolah.length === 0 ? (
                      <p className="text-sm text-zinc-500">
                        Belum ada data prestasi.
                      </p>
                    ) : (
                      safeData.topSekolah.map((row, index) => (
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
                  <div className="h-56 w-full sm:h-64 lg:h-72">
                    {hasJenjangSummariesChartData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart
                          data={safeData.jenjangSummaries.map((row) => ({
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
            </div>
            <p className="text-xs text-zinc-500">
              Terakhir diperbarui: {formatLastUpdated(data.lastUpdated)}
            </p>
          </section>
        ) : null}

        {isLoading ? (
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
        ) : isError || !data ? (
          <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-500">
            Unable to load data right now.
          </div>
        ) : (
          <div className="space-y-6">
            {data.totals.totalSchools === 0 ? (
              <div className="rounded-xl border border-dashed border-zinc-200 bg-white p-6 text-sm text-zinc-500">
                Data belum tersedia atau belum tervalidasi.
              </div>
            ) : null}
            <section className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
              <Card>
                <CardContent className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Total Sekolah
                  </p>
                  <p className="text-2xl font-semibold text-zinc-900">
                    {data.totals.totalSchools}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Guru (Pendidik)
                  </p>
                  <p className="text-2xl font-semibold text-zinc-900">
                    {data.totals.totalGuru}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Tenaga Kependidikan
                  </p>
                  <p className="text-2xl font-semibold text-zinc-900">
                    {data.totals.totalTendik}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Rata-rata Guru/Sekolah
                  </p>
                  <p className="text-2xl font-semibold text-zinc-900">
                    {formatFixed(data.totals?.avgGuruPerSchool)}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    % Akreditasi A+B
                  </p>
                  <p className="text-2xl font-semibold text-zinc-900">
                    {formatPercent(data.totals?.percentAkreditasiAB)}
                  </p>
                </CardContent>
              </Card>
            </section>

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
                        <BarChart data={safeData.akreditasiPerJenjang}>
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
                            >
                              {isAdmin ? (
                                <LabelList content={renderStackedLabel(key)} />
                              ) : null}
                            </Bar>
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
                            data={safeData.akreditasiComposition}
                            dataKey="total"
                            nameKey="displayLabel"
                            innerRadius={45}
                            outerRadius={90}
                            paddingAngle={2}
                            label={isAdmin ? renderPieLabel : undefined}
                            labelLine={isAdmin}
                          >
                            {safeData.akreditasiComposition.map((entry) => (
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
                          {isAdmin ? (
                            <Tooltip content={renderPieTooltip} />
                          ) : (
                            <Tooltip />
                          )}
                          {isAdmin ? (
                            <Legend verticalAlign="bottom" itemSorter={null} />
                          ) : null}
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
                        <BarChart data={safeData.statusByJenjang}>
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
                        <BarChart data={safeData.guruVsTendik} layout="vertical">
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
                      ASN/Yayasan vs Honorer
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Komposisi tenaga pendidik
                    </p>
                  </div>
                  <div className="h-64 w-full sm:h-72 lg:h-80">
                    {hasAsnHonorerData ? (
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart
                          data={[
                            {
                              label: "Guru",
                              ASN: data.extras.guruAsn,
                              Honorer: data.extras.guruHonorer,
                            },
                            {
                              label: "Tendik",
                              ASN: data.extras.tendikAsn,
                              Honorer: data.extras.tendikHonorer,
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
                      Data Quality
                    </p>
                    <p className="text-sm font-semibold text-zinc-700">
                      Pengecekan kelengkapan data
                    </p>
                  </div>
                  <div className="grid gap-2 text-sm text-zinc-600">
                    <p>
                      Akreditasi belum terisi: {data.extras.missingAkreditasi}
                    </p>
                    <p>
                      Tanggal Terbit Akreditasi (TMT) kosong:{" "}
                      {data.extras.missingTmt}
                    </p>
                    <p>
                      NPSN (Nomor Pokok Sekolah Nasional) kosong:{" "}
                      {data.extras.missingNpsn}
                    </p>
                    <p>Data guru kosong: {data.extras.missingGuru}</p>
                    <p>Data tendik kosong: {data.extras.missingTendik}</p>
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
                      <TableHead className="text-right">Total Sekolah</TableHead>
                      <TableHead className="text-right">Guru (Pendidik)</TableHead>
                      <TableHead className="text-right">
                        Tenaga Kependidikan
                      </TableHead>
                      <TableHead className="text-right">% A+B</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {safeData.jenjangSummaries.length === 0 ? (
                      <TableRow>
                        <TableCell className="py-4 text-center text-zinc-500" colSpan={5}>
                          Belum ada data.
                        </TableCell>
                      </TableRow>
                    ) : (
                      safeData.jenjangSummaries.map((row) => (
                        <TableRow key={row.jenjang}>
                          <TableCell className="font-semibold text-zinc-900">
                            {row.jenjang}
                          </TableCell>
                          <TableCell className="text-right">
                            {row.totalSekolah}
                          </TableCell>
                          <TableCell className="text-right">{row.totalGuru}</TableCell>
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

        {isAdmin ? (
          <section className="mt-12 space-y-4 rounded-xl border border-slate-200/60 bg-white p-4 shadow-sm sm:p-6">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Monitoring Data Sekolah
                </p>
                <h2 className="text-lg font-semibold text-zinc-900">
                  Monitoring Data Sekolah
                </h2>
              </div>
              <button
                type="button"
                onClick={handleMonitoringExport}
                disabled={isMonitoringExporting}
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              >
                {isMonitoringExporting ? "Menyiapkan..." : "Download Excel"}
              </button>
            </div>

            <div className="flex flex-col gap-3 lg:flex-row lg:flex-wrap lg:items-end">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Nama Sekolah
                </label>
                <TypeaheadInput
                  value={monitoringSchoolQuery}
                  onValueChange={(value) => {
                    setMonitoringSchoolQuery(value);
                    setMonitoringSchoolId(null);
                  }}
                  items={monitoringSuggestionItems}
                  onSelect={(item) => {
                    if (item.key === "ALL") {
                      setMonitoringSchoolId(null);
                      setMonitoringSchoolQuery("");
                      return;
                    }
                    const selected = schools.find((school) => school.id === item.key);
                    if (!selected) return;
                    setMonitoringSchoolId(selected.id);
                    setMonitoringSchoolQuery(selected.namaSekolah);
                  }}
                  placeholder="Ketik nama sekolah..."
                  emptyText="Sekolah tidak ditemukan"
                  minChars={0}
                  showClear={Boolean(
                    monitoringSchoolId || monitoringSchoolQuery
                  )}
                  onClear={() => {
                    setMonitoringSchoolId(null);
                    setMonitoringSchoolQuery("");
                  }}
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Akreditasi
                </label>
                <Select
                  value={monitoringAkreditasi}
                  onValueChange={setMonitoringAkreditasi}
                >
                  <SelectTrigger className="w-full md:w-40">
                    <SelectValue placeholder="Semua" />
                  </SelectTrigger>
                  <SelectContent>
                    {monitoringAkreditasiOptions.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Guru (ASN/Yayasan) (Min)
                </label>
                <input
                  type="number"
                  min={0}
                  value={monitoringGuruAsnMin}
                  onChange={(event) =>
                    setMonitoringGuruAsnMin(event.target.value)
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-36"
                  placeholder="0"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Guru (ASN/Yayasan) (Max)
                </label>
                <input
                  type="number"
                  min={0}
                  value={monitoringGuruAsnMax}
                  onChange={(event) =>
                    setMonitoringGuruAsnMax(event.target.value)
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-36"
                  placeholder="0"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tendik (ASN/Yayasan) (Min)
                </label>
                <input
                  type="number"
                  min={0}
                  value={monitoringTendikAsnMin}
                  onChange={(event) =>
                    setMonitoringTendikAsnMin(event.target.value)
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-36"
                  placeholder="0"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tendik (ASN/Yayasan) (Max)
                </label>
                <input
                  type="number"
                  min={0}
                  value={monitoringTendikAsnMax}
                  onChange={(event) =>
                    setMonitoringTendikAsnMax(event.target.value)
                  }
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm md:w-36"
                  placeholder="0"
                />
              </div>
            </div>

            {monitoringError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {monitoringError}
              </p>
            ) : null}

            {isMonitoringError ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                Gagal memuat data monitoring sekolah.
              </p>
            ) : null}

            <div className="max-h-[420px] overflow-y-auto rounded-xl border border-slate-200">
              <table className="w-full min-w-[720px] text-sm">
                <thead className="bg-slate-50">
                  <tr className="text-left text-slate-600">
                    {[
                      { key: "nama_sekolah", label: "Nama Sekolah" },
                      { key: "bentuk_pendidikan", label: "Bentuk Sekolah" },
                      { key: "akreditasi", label: "Akreditasi Sekolah" },
                      { key: "guru_asn", label: "Guru (ASN/Yayasan)" },
                      { key: "guru_honorer", label: "Guru Honorer" },
                      { key: "tendik_asn", label: "Tendik (ASN/Yayasan)" },
                      { key: "tendik_honorer", label: "Tendik Honorer" },
                    ].map((column) => (
                      <th
                        key={column.key}
                        className="sticky top-0 z-10 cursor-pointer bg-slate-50 px-4 py-3 text-sm font-medium"
                        onClick={() =>
                          handleMonitoringSort(column.key as MonitoringSortKey)
                        }
                      >
                        <span className="flex items-center gap-2">
                          {column.label}
                          <span className="text-[10px] text-zinc-400">
                            {monitoringSortIndicator(
                              column.key as MonitoringSortKey
                            )}
                          </span>
                        </span>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {isMonitoringLoading ? (
                    <tr>
                      <td
                            className="px-4 py-3 text-center text-sm text-zinc-500"
                            colSpan={7}
                      >
                        Memuat data...
                      </td>
                    </tr>
                  ) : monitoringRows.length === 0 ? (
                    <tr>
                      <td
                            className="px-4 py-3 text-center text-sm text-zinc-500"
                            colSpan={7}
                      >
                        Belum ada data sekolah.
                      </td>
                    </tr>
                  ) : (
                    monitoringRows.map((row) => (
                      <tr key={row.school_id} className="border-b border-slate-200 transition-colors hover:bg-slate-50">
                        <td className="px-4 py-3 font-semibold text-zinc-900">
                          {row.nama_sekolah}
                        </td>
                        <td className="px-4 py-3">{row.bentuk_pendidikan}</td>
                        <td className="px-4 py-3">{row.akreditasi}</td>
                        <td className="px-4 py-3 text-right">{row.guru_asn}</td>
                        <td className="px-4 py-3 text-right">
                          {row.guru_honorer}
                        </td>
                        <td className="px-4 py-3 text-right">
                          {row.tendik_asn}
                        </td>
                        <td className="px-4 py-3 text-right">
                          {row.tendik_honorer}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </section>
        ) : null}
      </section>
    </>
  );
}
