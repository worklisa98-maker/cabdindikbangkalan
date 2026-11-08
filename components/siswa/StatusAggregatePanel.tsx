"use client";

import { useMemo } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import LegacyImportActions from "@/components/siswa/LegacyImportActions";

type StatusAggregatePanelProps = {
  title: string;
  description: string;
  legacyImport?: {
    title?: string;
    templateUrl: string;
    importUrl: string;
  };
  historyHref?: string;
};

type ValidationSummary = {
  pending: number;
  overdue: number;
};

type LegacyImportStatus = {
  enabled: boolean;
  legacyImportMaxYear?: number | null;
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

const normalizeSummary = (payload: unknown): ValidationSummary => {
  if (!isRecord(payload)) {
    return { pending: 0, overdue: 0 };
  }
  const data = isRecord(payload.data) ? payload.data : payload;
  return {
    pending: readNumber(data.pending ?? data.pending_count ?? data.pendingCount),
    overdue: readNumber(data.overdue ?? data.overdue_count ?? data.overdueCount),
  };
};

const fetchSummary = async (isPublic: boolean): Promise<ValidationSummary> => {
  const endpoint = isPublic
    ? "/api/public/siswa/analytics/status"
    : "/api/siswa/validation/summary";
  const response = await apiFetch(
    endpoint,
    undefined,
    isPublic ? { skipAuthRedirect: true } : undefined
  );

  if (!response.ok) {
    throw new Error("summary_failed");
  }

  const payload = await response.json().catch(() => null);
  return normalizeSummary(payload);
};

const fetchLegacyImportStatus = async (): Promise<LegacyImportStatus> => {
  const response = await apiFetch("/legacy-import/status");
  if (!response.ok) {
    throw new Error("legacy_status_failed");
  }
  const payload = (await response.json().catch(() => null)) as {
    data?: LegacyImportStatus;
  } | null;
  return payload?.data ?? { enabled: false };
};

export default function StatusAggregatePanel({
  title,
  description,
  legacyImport,
  historyHref,
}: StatusAggregatePanelProps) {
  const { status, user } = useAuth();
  const isPublic = status === "unauthenticated";
  const isSchoolAdmin = user?.role === "SCHOOL_ADMIN";
  const router = useRouter();

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["siswa-status-aggregate", status, user?.role ?? "PUBLIC", title],
    queryFn: () => fetchSummary(isPublic),
    enabled: status !== "loading",
  });

  const { data: legacyStatus } = useQuery({
    queryKey: ["legacy-import-status", status, user?.id ?? 0, title],
    queryFn: fetchLegacyImportStatus,
    enabled: Boolean(legacyImport) && status === "authenticated" && isSchoolAdmin,
  });

  const chartData = useMemo(
    () => [
      { label: "Pending", total: data?.pending ?? 0 },
      { label: "Overdue", total: data?.overdue ?? 0 },
    ],
    [data?.overdue, data?.pending]
  );
  const hasChartData = (data?.pending ?? 0) > 0 || (data?.overdue ?? 0) > 0;

  return (
    <section className="space-y-6">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Status Lulusan
          </p>
          <h1 className="text-3xl font-semibold text-zinc-900">{title}</h1>
          <p className="text-sm text-zinc-600">{description}</p>
          {isPublic ? (
            <p className="text-xs font-medium text-zinc-500">
              Mode publik hanya menampilkan data agregat tanpa detail siswa.
            </p>
          ) : null}
        </div>
        {historyHref && !isPublic ? (
          <button
            type="button"
            onClick={() => {
              router.push(historyHref);
            }}
            className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
          >
            Histori Import
          </button>
        ) : null}
      </header>

      {legacyImport && isSchoolAdmin ? (
        <LegacyImportActions
          title={legacyImport.title ?? title}
          templateUrl={legacyImport.templateUrl}
          importUrl={legacyImport.importUrl}
          accept=".xlsx,.xlsm"
          isSchoolAdmin={isSchoolAdmin}
          enabled={legacyStatus?.enabled ?? false}
          maxYear={legacyStatus?.legacyImportMaxYear ?? null}
        />
      ) : null}

      {isLoading ? (
        <div className="h-72 animate-pulse rounded-xl border border-zinc-200 bg-zinc-100" />
      ) : isError ? (
        <div className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-600">
          <button
            type="button"
            onClick={() => void refetch()}
            className="rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600"
          >
            Gagal memuat data agregat. Coba lagi
          </button>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <CardContent className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Pending Validasi
              </p>
              <p className="text-3xl font-semibold text-zinc-900">{data?.pending ?? 0}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Overdue Validasi
              </p>
              <p className="text-3xl font-semibold text-zinc-900">{data?.overdue ?? 0}</p>
            </CardContent>
          </Card>
          <Card className="md:col-span-2">
            <CardContent className="h-72 w-full">
              {hasChartData ? (
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
                  Data belum tersedia
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </section>
  );
}

