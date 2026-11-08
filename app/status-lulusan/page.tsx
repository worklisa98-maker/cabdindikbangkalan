"use client";

import { Suspense, useEffect, useMemo } from "react";
import {
  usePathname,
  useRouter,
  useSearchParams,
  type ReadonlyURLSearchParams,
} from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "@/lib/api/client";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import StatusLulusanAnalyticsPTPanel from "@/components/siswa/status-lulusan/StatusLulusanAnalyticsPTPanel";

type PublicTabValue = "PT" | "KERJA" | "WIRAUSAHA" | "BELUM";

type AggregatePoint = {
  tahunLulus: number;
  finalCount: number;
};

const PUBLIC_TABS: Array<{ value: PublicTabValue; label: string; endpoint: string }> = [
  { value: "PT", label: "Perguruan Tinggi", endpoint: "/api/public/siswa/analytics/diterima-perguruan-tinggi" },
  { value: "KERJA", label: "Dunia Kerja", endpoint: "/api/public/siswa/analytics/diterima-dunia-kerja" },
  { value: "WIRAUSAHA", label: "Berwirausaha", endpoint: "/api/public/siswa/analytics/berwirausaha" },
  { value: "BELUM", label: "Belum Melanjutkan", endpoint: "/api/public/siswa/analytics/belum-melanjutkan" },
];

const toNumber = (value: unknown) => {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const normalizePublicTab = (value: string | null): PublicTabValue | null => {
  const normalized = (value ?? "").toUpperCase();
  return PUBLIC_TABS.some((tab) => tab.value === normalized)
    ? (normalized as PublicTabValue)
    : null;
};

const buildTabHref = (
  pathname: string,
  searchParams: ReadonlyURLSearchParams,
  tab: PublicTabValue
) => {
  const params = new URLSearchParams(searchParams.toString());
  params.set("tab", tab);
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
};

const formatNumber = (value: number) => new Intl.NumberFormat("id-ID").format(value);

const fetchAggregate = async (endpoint: string): Promise<AggregatePoint[]> => {
  const response = await apiFetch(endpoint, undefined, { skipAuthRedirect: true });
  if (!response.ok) throw new Error("Gagal memuat agregat.");
  const payload = await response.json().catch(() => null);
  const data = payload?.data ?? payload ?? [];
  const rows = Array.isArray(data)
    ? data
    : Array.isArray(data?.items)
      ? data.items
      : Array.isArray(data?.data)
        ? data.data
        : [];
  return rows.map((row: unknown) => {
    const item = isRecord(row) ? row : {};
    return {
      tahunLulus: toNumber(item.tahun_lulus ?? item.tahunLulus ?? item.year),
      finalCount: toNumber(item.final_count ?? item.finalCount ?? item.total),
    };
  }).filter((row: AggregatePoint) => row.tahunLulus > 0);
};

export default function StatusLulusanPublicPage() {
  return (
    <Suspense fallback={<StatusLulusanPublicFallback />}>
      <StatusLulusanPublicPageContent />
    </Suspense>
  );
}

function StatusLulusanPublicFallback() {
  return (
    <section className="space-y-2">
      <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">Siswa</p>
      <h1 className="text-3xl font-semibold text-zinc-900">Status Lulusan</h1>
      <p className="text-sm text-zinc-500">Memuat...</p>
    </section>
  );
}

function StatusLulusanPublicPageContent() {
  const router = useRouter();
  const pathname = usePathname() ?? "/status-lulusan";
  const searchParams = useSearchParams();
  const resolvedTab = normalizePublicTab(searchParams.get("tab"));
  const activeTab: PublicTabValue = resolvedTab ?? "PT";
  const activeConfig = PUBLIC_TABS.find((tab) => tab.value === activeTab) ?? PUBLIC_TABS[0];

  useEffect(() => {
    if (resolvedTab) return;
    router.replace(buildTabHref(pathname, searchParams, "PT"));
  }, [pathname, resolvedTab, router, searchParams]);

  const { data = [], isLoading, isError, refetch } = useQuery({
    queryKey: ["public-status-lulusan-aggregate", activeConfig.endpoint],
    queryFn: () => fetchAggregate(activeConfig.endpoint),
    retry: 0,
  });

  const tabs = useMemo(
    () => PUBLIC_TABS.map((tab) => ({ ...tab, href: buildTabHref(pathname, searchParams, tab.value) })),
    [pathname, searchParams]
  );

  const total = data.reduce((sum, item) => sum + item.finalCount, 0);

  return (
    <section className="space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">Siswa</p>
        <h1 className="text-3xl font-semibold text-zinc-900">Status Lulusan</h1>
        <p className="text-sm text-zinc-600">
          Ringkasan publik agregat tanpa NISN, nama siswa, rombel, atau detail individual.
        </p>
      </header>

      <div className="sticky top-0 z-20 border-b border-zinc-200 bg-white/95 backdrop-blur">
        <div className="flex flex-wrap items-center gap-2 rounded-xl border border-zinc-200 bg-white px-3 py-2 shadow-sm">
          {tabs.map((tab) => (
            <button
              key={tab.value}
              type="button"
              onClick={() => router.push(tab.href)}
              className={`rounded-full border px-4 py-2 text-xs font-semibold transition ${
                tab.value === activeTab
                  ? "border-zinc-900 bg-zinc-900 text-white"
                  : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-900 hover:text-zinc-900"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {activeTab === "PT" ? <StatusLulusanAnalyticsPTPanel isPublic /> : null}

      <Card>
        <CardContent className="space-y-4 p-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-3">
              <p className="text-xs text-zinc-500">Kategori</p>
              <p className="text-lg font-semibold text-zinc-900">{activeConfig.label}</p>
            </div>
            <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-3">
              <p className="text-xs text-zinc-500">Total Agregat</p>
              <p className="text-2xl font-semibold text-zinc-900">{isLoading ? "..." : formatNumber(total)}</p>
            </div>
            <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-3">
              <p className="text-xs text-zinc-500">Jumlah Tahun</p>
              <p className="text-2xl font-semibold text-zinc-900">{isLoading ? "..." : formatNumber(data.length)}</p>
            </div>
          </div>

          {isError ? (
            <div className="rounded-md border border-dashed border-red-200 p-3 text-sm text-red-600">
              <div className="flex items-center justify-between gap-3">
                <span>Gagal memuat ringkasan agregat.</span>
                <button type="button" onClick={() => void refetch()} className="rounded-md border border-red-200 px-3 py-1 text-xs font-semibold text-red-700 transition hover:bg-red-50">
                  Coba lagi
                </button>
              </div>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Tahun Lulus</TableHead>
                  <TableHead className="text-right">Total</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow><TableCell colSpan={2}>Memuat tren tahun...</TableCell></TableRow>
                ) : data.length === 0 ? (
                  <TableRow><TableCell colSpan={2}>Belum ada data agregat.</TableCell></TableRow>
                ) : (
                  data.map((row) => (
                    <TableRow key={row.tahunLulus}>
                      <TableCell>{row.tahunLulus}</TableCell>
                      <TableCell className="text-right">{formatNumber(row.finalCount)}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </section>
  );
}
