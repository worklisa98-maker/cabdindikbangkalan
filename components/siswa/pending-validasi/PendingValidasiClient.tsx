"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
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
import { getErrorMessageFromResponse } from "@/lib/http";
import { invalidatePTDerivedQueries } from "@/lib/invalidate-pt-queries";

type ConflictRow = {
  id: number;
  nisn: string;
  namaSiswa: string;
  rombel: string;
  tahunLulus: number;
  candidates: string[];
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const readString = (value: unknown) =>
  typeof value === "string"
    ? value.trim()
    : typeof value === "number" && Number.isFinite(value)
      ? String(value)
      : "";

const readNumber = (value: unknown) => {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const normalizeRows = (payload: unknown): ConflictRow[] => {
  const data = isRecord(payload)
    ? isRecord(payload.data)
      ? payload.data
      : payload
    : {};
  const rawRows = data.multi_jalur ?? data.multiJalur;
  const rows: unknown[] = Array.isArray(rawRows) ? rawRows : [];
  return rows
    .map((row: unknown) => {
      if (!isRecord(row)) return null;
      const rawCandidates = row.candidate_outcomes ?? row.candidateOutcomes;
      const candidates = Array.isArray(rawCandidates)
        ? rawCandidates.map((item: unknown) => readString(item)).filter(Boolean)
        : [];
      return {
        id: readNumber(row.id),
        nisn: readString(row.nisn),
        namaSiswa: readString(row.nama_siswa ?? row.namaSiswa),
        rombel: readString(row.rombel),
        tahunLulus: readNumber(row.tahun_lulus ?? row.tahunLulus),
        candidates,
      };
    })
    .filter((row: ConflictRow | null): row is ConflictRow =>
      Boolean(row && row.id > 0 && row.nisn && row.candidates.length > 1)
    );
};

const fetchConflicts = async (year: string) => {
  const query = new URLSearchParams();
  if (year.trim()) query.set("year", year.trim());
  const response = await apiFetch(
    `/api/siswa/validation/pending?${query.toString()}`
  );
  if (!response.ok) throw new Error("Gagal memuat konflik status lulusan.");
  return normalizeRows(await response.json().catch(() => null));
};

const parsePTCandidate = (candidate: string) => {
  const parts = candidate.split(" - ").map((item) => item.trim());
  if (parts[0]?.toUpperCase() !== "PT") return null;
  return {
    jalur: parts[1] ?? "",
    kampus: parts[2] ?? "",
    prodi: parts.slice(3).join(" - "),
  };
};

export default function PendingValidasiClient() {
  const queryClient = useQueryClient();
  const { user, status } = useAuth();
  const currentYear = new Date().getFullYear();
  const [year, setYear] = useState(String(currentYear));
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [loadingRow, setLoadingRow] = useState<number | null>(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const {
    data = [],
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ["siswa-status-conflict-pending", year],
    queryFn: () => fetchConflicts(year),
    enabled: status === "authenticated",
  });

  const schoolId = useMemo(() => {
    const value = user?.schoolId;
    return typeof value === "number" && value > 0 ? value : null;
  }, [user?.schoolId]);

  const handlePick = async (row: ConflictRow) => {
    const candidate = selected[`${row.id}`] || row.candidates[0] || "";
    if (!candidate) return;
    setLoadingRow(row.id);
    setMessage("");
    setError("");
    try {
      const pt = parsePTCandidate(candidate);
      const response = pt
        ? await apiFetch(`/api/siswa/outcomes/${row.id}`, {
            method: "PUT",
            body: JSON.stringify({
              finalStatus: "PT",
              finalPath: pt.jalur,
              ptDetail: {
                jalur: pt.jalur,
                kampus: pt.kampus,
                prodi: pt.prodi,
              },
              reason: "PENDING_VALIDATION_CONFLICT_RESOLUTION",
            }),
          })
        : await apiFetch("/api/siswa/validation/finalize-bulk", {
            method: "POST",
            body: JSON.stringify({
              school_id: schoolId,
              year: row.tahunLulus,
              student_ids: [row.id],
              final_status: candidate.toUpperCase().startsWith("KERJA")
                ? "KERJA"
                : "WIRAUSAHA",
            }),
          });
      if (!response.ok) {
        throw new Error(await getErrorMessageFromResponse(response));
      }
      setMessage("Status final tersimpan.");
      await refetch();
      await invalidatePTDerivedQueries(queryClient);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal menyimpan status final."
      );
    } finally {
      setLoadingRow(null);
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
            Pending Validasi
          </h1>
          <p className="text-sm text-zinc-600">
            Hanya konflik outcome tahun berjalan yang perlu dipilih sebagai satu
            status final.
          </p>
        </header>

        <Card>
          <CardContent className="space-y-4">
            <label className="block max-w-xs space-y-1 text-sm font-medium text-zinc-700">
              <span>Tahun Lulus</span>
              <input
                type="number"
                value={year}
                onChange={(event) => setYear(event.target.value)}
                className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm"
              />
            </label>
            {message ? (
              <p className="text-sm text-emerald-700">{message}</p>
            ) : null}
            {error ? <p className="text-sm text-red-600">{error}</p> : null}
            <Table className="min-w-[980px]">
              <TableHeader>
                <TableRow>
                  <TableHead>NISN</TableHead>
                  <TableHead>Nama Siswa</TableHead>
                  <TableHead>Rombel</TableHead>
                  <TableHead>Tahun Lulus</TableHead>
                  <TableHead>Daftar Kandidat Outcome</TableHead>
                  <TableHead>Aksi Pilih Status Final</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={6}>Memuat konflik...</TableCell>
                  </TableRow>
                ) : isError ? (
                  <TableRow>
                    <TableCell colSpan={6} className="text-red-600">
                      Gagal memuat konflik.
                    </TableCell>
                  </TableRow>
                ) : data.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="text-zinc-500">
                      Tidak ada konflik outcome pada tahun ini.
                    </TableCell>
                  </TableRow>
                ) : (
                  data.map((row) => {
                    const key = `${row.id}`;
                    const value = selected[key] || row.candidates[0] || "";
                    return (
                      <TableRow key={`${row.nisn}-${row.tahunLulus}`}>
                        <TableCell>{row.nisn}</TableCell>
                        <TableCell>{row.namaSiswa || "-"}</TableCell>
                        <TableCell>{row.rombel || "-"}</TableCell>
                        <TableCell>{row.tahunLulus}</TableCell>
                        <TableCell>
                          <div className="space-y-1 text-sm">
                            {row.candidates.map((candidate) => (
                              <p key={candidate}>{candidate}</p>
                            ))}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-col gap-2 sm:flex-row">
                            <select
                              value={value}
                              onChange={(event) =>
                                setSelected((current) => ({
                                  ...current,
                                  [key]: event.target.value,
                                }))
                              }
                              className="h-9 rounded-md border border-zinc-200 bg-white px-2 text-sm"
                            >
                              {row.candidates.map((candidate) => (
                                <option key={candidate} value={candidate}>
                                  {candidate}
                                </option>
                              ))}
                            </select>
                            <button
                              type="button"
                              onClick={() => void handlePick(row)}
                              disabled={loadingRow === row.id}
                              className="h-9 rounded-md bg-zinc-900 px-3 text-sm font-semibold text-white disabled:opacity-60"
                            >
                              {loadingRow === row.id
                                ? "Menyimpan..."
                                : "Simpan"}
                            </button>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </section>
    </AdminRouteGuard>
  );
}
