"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";

type DelegateItem = {
  id: number;
  username: string;
  name: string;
  delegated: boolean;
};

const fetchDelegates = async () => {
  const response = await apiFetch("/api/siswa/final-requests/delegates");
  if (!response.ok) throw new Error("Gagal memuat delegasi.");
  const payload = await response.json();
  return (payload.data?.items ?? []) as DelegateItem[];
};

export default function SiswaAkhirSettingsPage() {
  const { user } = useAuth();
  const isMaster = user?.role === "ADMIN_MASTER";
  const [selectedDraft, setSelected] = useState<Set<number> | null>(null);
  const { data = [], isLoading, isError } = useQuery({
    queryKey: ["siswa-akhir-delegates"],
    queryFn: fetchDelegates,
    enabled: isMaster,
  });

  const selected = selectedDraft ?? new Set((data).filter((item) => item.delegated).map((item) => item.id));

  const mutation = useMutation({
    mutationFn: async () => {
      const response = await apiFetch("/api/siswa/final-requests/delegates", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ user_ids: Array.from(selected) }),
      });
      if (!response.ok) throw new Error("Gagal menyimpan delegasi.");
    },
  });

  return (
    <AdminRouteGuard>
      <section className="space-y-6">
        <header className="space-y-2">
          <Link href="/siswa/kelas-akhir" className="text-sm font-semibold text-blue-700 underline">Kembali ke Siswa Akhir</Link>
          <h1 className="text-3xl font-semibold text-zinc-900">Settings Siswa Akhir</h1>
          <p className="text-sm text-zinc-600">Atur akun ADMIN_CABDIN yang boleh approve/reject pengajuan siswa akhir.</p>
        </header>
        {!isMaster ? (
          <Card><CardContent><p className="text-sm text-red-600">Hanya ADMIN_MASTER yang dapat mengatur delegasi.</p></CardContent></Card>
        ) : (
          <Card>
            <CardContent className="space-y-4">
              {isLoading ? <p className="text-sm text-zinc-500">Memuat akun Cabdin...</p> : null}
              {isError ? <p className="text-sm text-red-600">Gagal memuat delegasi.</p> : null}
              <div className="space-y-2">
                {data.map((item) => (
                  <label key={item.id} className="flex items-center justify-between rounded-md border border-zinc-200 p-3 text-sm">
                    <span>
                      <span className="font-semibold text-zinc-900">{item.name}</span>
                      <span className="ml-2 text-zinc-500">{item.username}</span>
                    </span>
                    <input
                      type="checkbox"
                      checked={selected.has(item.id)}
                      onChange={(event) => {
                        setSelected((current) => {
                          const next = new Set(current ?? selected);
                          if (event.target.checked) next.add(item.id);
                          else next.delete(item.id);
                          return next;
                        });
                      }}
                    />
                  </label>
                ))}
              </div>
              {mutation.error ? <p className="text-sm text-red-600">{mutation.error.message}</p> : null}
              {mutation.isSuccess ? <p className="text-sm text-emerald-700">Delegasi berhasil disimpan.</p> : null}
              <button onClick={() => mutation.mutate()} disabled={mutation.isPending} className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60">
                {mutation.isPending ? "Menyimpan..." : "Simpan Delegasi"}
              </button>
            </CardContent>
          </Card>
        )}
      </section>
    </AdminRouteGuard>
  );
}
