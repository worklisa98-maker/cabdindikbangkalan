"use client";

import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import Link from "next/link";

import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";

type InfoItem = {
  id: number;
  title: string;
  category: string;
  sort_order: number;
  is_banner: boolean;
  is_active: boolean;
  file_name: string;
};

type DelegateItem = { id: number; username: string; name: string; delegated: boolean };

const categories = ["Informasi Umum", "Kepegawaian", "Persuratan dan Layanan", "Sekolah dan Kelembagaan"];

const fetchAdminItems = async () => {
  const response = await apiFetch("/api/admin/informasi-pelayanan-cabdin");
  if (!response.ok) throw new Error("FORBIDDEN");
  const payload = await response.json();
  return {
    items: (payload.data?.items ?? []) as InfoItem[],
    canManageDelegates: Boolean(payload.data?.can_manage_delegates),
  };
};

export default function ServiceInfoSettingsPage() {
  const [editing, setEditing] = useState<InfoItem | null>(null);
  const [title, setTitle] = useState("");
  const [category, setCategory] = useState(categories[0]);
  const [sortOrder, setSortOrder] = useState("0");
  const [isBanner, setIsBanner] = useState(false);
  const [isActive, setIsActive] = useState(true);
  const [file, setFile] = useState<File | null>(null);
  const [selectedDelegatesDraft, setSelectedDelegates] = useState<Set<number> | null>(null);
  const { data, isLoading, isError, refetch } = useQuery({ queryKey: ["service-info-admin"], queryFn: fetchAdminItems, retry: false });

  const delegatesQuery = useQuery({
    queryKey: ["service-info-delegates"],
    queryFn: async () => {
      const response = await apiFetch("/api/admin/informasi-pelayanan-cabdin/delegates");
      if (!response.ok) return [] as DelegateItem[];
      const payload = await response.json();
      return (payload.data?.items ?? []) as DelegateItem[];
    },
    enabled: Boolean(data?.canManageDelegates),
  });

  const selectedDelegates = selectedDelegatesDraft ?? new Set((delegatesQuery.data ?? []).filter((item) => item.delegated).map((item) => item.id));

  const resetForm = () => {
    setEditing(null);
    setTitle("");
    setCategory(categories[0]);
    setSortOrder("0");
    setIsBanner(false);
    setIsActive(true);
    setFile(null);
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      const form = new FormData();
      if (editing) form.set("id", String(editing.id));
      form.set("title", title);
      form.set("category", category);
      form.set("sort_order", sortOrder);
      form.set("is_banner", String(isBanner));
      form.set("is_active", String(isActive));
      if (file) form.set("file", file);
      const response = await apiFetch("/api/admin/informasi-pelayanan-cabdin", { method: "POST", body: form });
      if (!response.ok) throw new Error("Gagal menyimpan item.");
    },
    onSuccess: () => {
      resetForm();
      void refetch();
    },
  });

  const delegateMutation = useMutation({
    mutationFn: async () => {
      const response = await apiFetch("/api/admin/informasi-pelayanan-cabdin/delegates", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ user_ids: Array.from(selectedDelegates) }),
      });
      if (!response.ok) throw new Error("Gagal menyimpan delegasi.");
    },
    onSuccess: () => void delegatesQuery.refetch(),
  });

  const startEdit = (item: InfoItem) => {
    setEditing(item);
    setTitle(item.title);
    setCategory(item.category);
    setSortOrder(String(item.sort_order));
    setIsBanner(item.is_banner);
    setIsActive(item.is_active);
    setFile(null);
  };

  return (
    <AdminRouteGuard>
      <section className="space-y-6">
        <header className="space-y-2">
          <Link href="/informasi-pelayanan-cabdin" className="text-sm font-semibold text-blue-700 underline">Buka public page</Link>
          <h1 className="text-3xl font-semibold text-zinc-900">Kelola Informasi Pelayanan Cabdin</h1>
          <p className="text-sm text-zinc-600">File disimpan ke persistent storage `/data/uploads/informasi-pelayanan-cabdin`.</p>
        </header>

        {isLoading ? <p className="text-sm text-zinc-500">Memuat data...</p> : null}
        {isError ? <Card><CardContent><p className="text-sm text-red-600">Tidak diizinkan mengelola halaman ini.</p></CardContent></Card> : null}

        {!isError ? (
          <div className="grid gap-6 xl:grid-cols-[1fr_1.4fr]">
            <Card>
              <CardContent className="space-y-3">
                <h2 className="text-lg font-semibold">{editing ? "Edit Item" : "Tambah Item"}</h2>
                <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Judul" className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm" />
                <select value={category} onChange={(event) => setCategory(event.target.value)} className="h-10 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm">
                  {categories.map((item) => <option key={item}>{item}</option>)}
                </select>
                <input type="number" value={sortOrder} onChange={(event) => setSortOrder(event.target.value)} placeholder="Urutan" className="h-10 w-full rounded-md border border-zinc-200 px-3 text-sm" />
                <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={isBanner} onChange={(event) => setIsBanner(event.target.checked)} /> Jadikan banner utama</label>
                <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={isActive} onChange={(event) => setIsActive(event.target.checked)} /> Aktif</label>
                <div className="space-y-2 rounded-lg border border-dashed border-zinc-300 bg-zinc-50 p-4">
                  <label className="block text-sm font-semibold text-zinc-800">
                    Upload gambar/file SOP
                  </label>
                  <p className="text-xs text-zinc-500">
                    Unggah gambar SOP/Informasi dalam format PNG, JPG, JPEG, WEBP, atau PDF. File akan disimpan ke persistent storage /data.
                  </p>
                  <input
                    type="file"
                    accept="image/png,image/jpeg,image/webp,application/pdf"
                    onChange={(event) => setFile(event.target.files?.[0] ?? null)}
                    className="block w-full text-sm text-zinc-700 file:mr-3 file:rounded-md file:border-0 file:bg-zinc-900 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-white hover:file:bg-zinc-700"
                  />
                  {file ? (
                    <p className="text-xs font-medium text-green-700">
                      File dipilih: {file.name}
                    </p>
                  ) : editing?.file_name ? (
                    <p className="text-xs text-zinc-500">
                      File saat ini: {editing.file_name}. Kosongkan upload jika tidak ingin mengganti file.
                    </p>
                  ) : (
                    <p className="text-xs text-zinc-500">
                      Belum ada file dipilih.
                    </p>
                  )}
                </div>
                {saveMutation.error ? <p className="text-sm text-red-600">{saveMutation.error.message}</p> : null}
                <div className="flex gap-2">
                  <button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending} className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60">{saveMutation.isPending ? "Menyimpan..." : "Simpan"}</button>
                  <button onClick={resetForm} className="rounded-md border border-zinc-200 px-4 py-2 text-sm font-semibold">Reset</button>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="space-y-3">
                <h2 className="text-lg font-semibold">Daftar Item</h2>
                <div className="space-y-2">
                  {(data?.items ?? []).map((item) => (
                    <div key={item.id} className="flex items-center justify-between gap-3 rounded-md border border-zinc-200 p-3 text-sm">
                      <div>
                        <p className="font-semibold text-zinc-900">{item.title}</p>
                        <p className="text-zinc-500">{item.category} / urutan {item.sort_order} / {item.is_active ? "aktif" : "nonaktif"} {item.is_banner ? "/ banner" : ""}</p>
                        <p className="text-zinc-500">{item.file_name || "Belum ada file"}</p>
                      </div>
                      <button onClick={() => startEdit(item)} className="rounded-md border border-zinc-200 px-3 py-2 text-sm font-semibold">Edit</button>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </div>
        ) : null}

        {data?.canManageDelegates ? (
          <Card>
            <CardContent className="space-y-3">
              <h2 className="text-lg font-semibold">Delegasi Pengelola Cabdin</h2>
              {(delegatesQuery.data ?? []).map((item) => (
                <label key={item.id} className="flex items-center justify-between rounded-md border border-zinc-200 p-3 text-sm">
                  <span><span className="font-semibold">{item.name}</span> <span className="text-zinc-500">{item.username}</span></span>
                  <input type="checkbox" checked={selectedDelegates.has(item.id)} onChange={(event) => {
                    setSelectedDelegates((current) => {
                      const next = new Set(current ?? selectedDelegates);
                      if (event.target.checked) next.add(item.id);
                      else next.delete(item.id);
                      return next;
                    });
                  }} />
                </label>
              ))}
              {delegateMutation.error ? <p className="text-sm text-red-600">{delegateMutation.error.message}</p> : null}
              <button onClick={() => delegateMutation.mutate()} disabled={delegateMutation.isPending} className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60">Simpan Delegasi</button>
            </CardContent>
          </Card>
        ) : null}
      </section>
    </AdminRouteGuard>
  );
}
