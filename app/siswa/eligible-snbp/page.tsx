"use client";

import { FormEvent, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import { EmptyState } from "@/components/empty-state";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { canUseScopedSchoolFilter } from "@/lib/auth/role-scope";
import { jenjangOptions } from "@/lib/shared/jenjangOptions";

type ScopedSchool = {
  id: number;
  namaSekolah: string;
};

type EligibleItem = {
  schoolId: number;
  sekolah: string;
  jenjang: string;
  statusSekolah: string;
  tahunLulus: number;
  eligibleCount: number;
  totalSiswaAkhir: number;
  penerimaanSnbp: number;
  note: string;
  updatedAt: string | null;
  statusLabel: string;
  needsWarning: boolean;
};

const readNumber = (value: unknown) => {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const readString = (value: unknown) =>
  typeof value === "string" ? value.trim() : "";

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const formatNumber = (value: number) => new Intl.NumberFormat("id-ID").format(value);

const fetchScopedSchools = async (): Promise<ScopedSchool[]> => {
  const response = await apiFetch("/api/user/me/schools");
  if (!response.ok) throw new Error("Failed to fetch schools.");
  const payload = await response.json().catch(() => null);
  const rows = Array.isArray(payload?.data) ? payload.data : [];
  return rows.map((row: unknown) => {
    const item = isRecord(row) ? row : {};
    return {
      id: readNumber(item.id),
      namaSekolah: readString(item.namaSekolah ?? item.nama_sekolah),
    };
  });
};

const fetchEligible = async (params: {
  year: string;
  jenjang: string;
  statusSekolah: string;
  schoolId: number | null;
}): Promise<EligibleItem[]> => {
  const query = new URLSearchParams();
  if (params.year.trim()) query.set("year", params.year.trim());
  if (params.jenjang !== "ALL") query.set("jenjang", params.jenjang);
  if (params.statusSekolah !== "ALL") query.set("status_sekolah", params.statusSekolah);
  if (params.schoolId && params.schoolId > 0) query.set("school_id", String(params.schoolId));
  const response = await apiFetch(`/api/siswa/eligible-snbp?${query.toString()}`, {
    cache: "no-store",
  });
  if (!response.ok) throw new Error("Failed to fetch eligible SNBP.");
  const payload = await response.json().catch(() => null);
  const data = isRecord(payload?.data) ? payload.data : payload;
  const rows = Array.isArray(data?.items) ? data.items : [];
  return rows.map((row: unknown) => {
    const item = isRecord(row) ? row : {};
    return {
      schoolId: readNumber(item.school_id ?? item.schoolId),
      sekolah: readString(item.sekolah),
      jenjang: readString(item.jenjang),
      statusSekolah: readString(item.status_sekolah ?? item.statusSekolah),
      tahunLulus: readNumber(item.tahun_lulus ?? item.tahunLulus),
      eligibleCount: readNumber(item.eligible_count ?? item.eligibleCount),
      totalSiswaAkhir: readNumber(item.total_siswa_akhir ?? item.totalSiswaAkhir),
      penerimaanSnbp: readNumber(item.penerimaan_snbp ?? item.penerimaanSnbp),
      note: readString(item.note),
      updatedAt: readString(item.updated_at ?? item.updatedAt) || null,
      statusLabel: readString(item.status_label ?? item.statusLabel),
      needsWarning: Boolean(item.needs_warning ?? item.needsWarning),
    };
  });
};

export default function EligibleSNBPPage() {
  const { user, status } = useAuth();
  const role = (user?.role ?? "").toUpperCase();
  const isSchoolAdmin = role === "SCHOOL_ADMIN";
  const canEdit = role === "SCHOOL_ADMIN" || role === "ADMIN_MASTER";
  const showFilters = !isSchoolAdmin && canUseScopedSchoolFilter(user?.role);
  const currentYear = new Date().getFullYear();
  const [year, setYear] = useState(String(currentYear));
  const [jenjang, setJenjang] = useState("ALL");
  const [statusSekolah, setStatusSekolah] = useState("ALL");
  const [schoolId, setSchoolId] = useState<number | null>(null);
  const [eligibleCount, setEligibleCount] = useState("");
  const [note, setNote] = useState("");
  const [message, setMessage] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  const { data: schools = [] } = useQuery({
    queryKey: ["eligible-snbp-schools", user?.id ?? "anon"],
    queryFn: fetchScopedSchools,
    enabled: status === "authenticated" && showFilters,
  });

  const {
    data: items = [],
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ["eligible-snbp", year, jenjang, statusSekolah, schoolId ?? "ALL", isSchoolAdmin],
    queryFn: () =>
      fetchEligible({
        year,
        jenjang: showFilters ? jenjang : "ALL",
        statusSekolah: showFilters ? statusSekolah : "ALL",
        schoolId: showFilters ? schoolId : null,
      }),
    enabled: status === "authenticated",
  });

  const warningItems = useMemo(() => items.filter((item) => item.needsWarning), [items]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setMessage("");
    const parsedYear = Number(year);
    const parsedCount = Number(eligibleCount);
    if (!Number.isFinite(parsedYear) || parsedYear <= 0 || !Number.isFinite(parsedCount) || parsedCount < 0) {
      setMessage("Tahun lulus dan jumlah eligible wajib valid.");
      return;
    }
    if (!isSchoolAdmin && (!schoolId || schoolId <= 0)) {
      setMessage("Pilih sekolah terlebih dahulu.");
      return;
    }
    setIsSaving(true);
    try {
      const response = await apiFetch("/api/siswa/eligible-snbp", {
        method: "PUT",
        body: JSON.stringify({
          school_id: isSchoolAdmin ? undefined : schoolId,
          tahun_lulus: parsedYear,
          eligible_count: parsedCount,
          note,
        }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) throw new Error(payload?.message ?? "Gagal menyimpan data eligible.");
      setEligibleCount("");
      setNote("");
      setMessage("Data eligible SNBP tersimpan.");
      void refetch();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Gagal menyimpan data eligible.");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <AdminRouteGuard>
      <section className="space-y-6">
        <header className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">Siswa</p>
          <h1 className="text-3xl font-semibold text-zinc-900">Data Eligible SNBP</h1>
          <p className="text-sm text-zinc-600">
            Rekap jumlah siswa eligible SNBP per sekolah dan tahun lulus.
          </p>
        </header>

        {warningItems.length > 0 ? (
          <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
            Data Eligible SNBP tahun {year} belum diisi. Mohon lengkapi agar analisis SNBP dan pembanding Cabdin valid.
          </div>
        ) : null}

        <Card>
          <CardContent className="space-y-4 p-4">
            <div className="grid gap-3 md:grid-cols-4">
              {showFilters ? (
                <select value={schoolId ?? ""} onChange={(event) => setSchoolId(Number(event.target.value) || null)} className="h-10 rounded-md border border-zinc-200 bg-white px-3 text-sm">
                  <option value="">Semua sekolah dalam scope</option>
                  {schools.map((school) => (
                    <option key={school.id} value={school.id}>{school.namaSekolah}</option>
                  ))}
                </select>
              ) : null}
              {showFilters ? (
                <select value={jenjang} onChange={(event) => setJenjang(event.target.value)} className="h-10 rounded-md border border-zinc-200 bg-white px-3 text-sm">
                  <option value="ALL">Semua jenjang</option>
                  {jenjangOptions.map((option) => <option key={option} value={option}>{option}</option>)}
                </select>
              ) : null}
              {showFilters ? (
                <select value={statusSekolah} onChange={(event) => setStatusSekolah(event.target.value)} className="h-10 rounded-md border border-zinc-200 bg-white px-3 text-sm">
                  <option value="ALL">Semua status</option>
                  <option value="NEGERI">Negeri</option>
                  <option value="SWASTA">Swasta</option>
                </select>
              ) : null}
              <input type="number" value={year} onChange={(event) => setYear(event.target.value)} className="h-10 rounded-md border border-zinc-200 px-3 text-sm" placeholder="Tahun lulus" />
            </div>
          </CardContent>
        </Card>

        {canEdit ? (
          <Card>
            <CardContent className="p-4">
              <form onSubmit={handleSubmit} className="grid gap-3 md:grid-cols-4">
                <input type="number" value={eligibleCount} onChange={(event) => setEligibleCount(event.target.value)} min={0} className="h-10 rounded-md border border-zinc-200 px-3 text-sm" placeholder="Jumlah eligible" />
                <input value={note} onChange={(event) => setNote(event.target.value)} className="h-10 rounded-md border border-zinc-200 px-3 text-sm md:col-span-2" placeholder="Catatan" />
                <button type="submit" disabled={isSaving} className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60">
                  {isSaving ? "Menyimpan..." : "Simpan"}
                </button>
                {message ? <p className="text-sm text-zinc-600 md:col-span-4">{message}</p> : null}
              </form>
            </CardContent>
          </Card>
        ) : null}

        <Card>
          <CardContent className="p-4">
            <Table wrapperClassName="max-h-[560px] overflow-auto" className="min-w-[980px]">
              <TableHeader className="sticky top-0 z-10 bg-slate-50">
                <TableRow>
                  <TableHead>Sekolah</TableHead>
                  <TableHead>Jenjang</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Tahun</TableHead>
                  <TableHead className="text-right">Eligible</TableHead>
                  <TableHead className="text-right">Siswa Akhir</TableHead>
                  <TableHead className="text-right">Diterima SNBP</TableHead>
                  <TableHead>Status Input</TableHead>
                  <TableHead>Catatan</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow><TableCell colSpan={9}>Memuat data eligible...</TableCell></TableRow>
                ) : isError ? (
                  <TableRow><TableCell colSpan={9}><EmptyState title="Gagal memuat data" description="Silakan coba lagi." /></TableCell></TableRow>
                ) : items.length === 0 ? (
                  <TableRow><TableCell colSpan={9}><EmptyState title="Belum ada data" description="Tidak ada sekolah pada filter aktif." /></TableCell></TableRow>
                ) : (
                  items.map((item) => (
                    <TableRow key={`${item.schoolId}-${item.tahunLulus}`}>
                      <TableCell className="font-medium">{item.sekolah || "-"}</TableCell>
                      <TableCell>{item.jenjang || "-"}</TableCell>
                      <TableCell>{item.statusSekolah || "-"}</TableCell>
                      <TableCell className="text-right">{item.tahunLulus}</TableCell>
                      <TableCell className="text-right">{formatNumber(item.eligibleCount)}</TableCell>
                      <TableCell className="text-right">{formatNumber(item.totalSiswaAkhir)}</TableCell>
                      <TableCell className="text-right">{formatNumber(item.penerimaanSnbp)}</TableCell>
                      <TableCell>
                        <span className={`rounded-full px-2 py-1 text-xs font-semibold ${item.needsWarning ? "bg-amber-100 text-amber-800" : "bg-zinc-100 text-zinc-700"}`}>
                          {item.needsWarning ? "Eligible SNBP belum diisi" : item.statusLabel}
                        </span>
                      </TableCell>
                      <TableCell>{item.note || "-"}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </section>
    </AdminRouteGuard>
  );
}
