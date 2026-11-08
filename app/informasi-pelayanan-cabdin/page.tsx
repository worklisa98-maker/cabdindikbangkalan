"use client";

/* eslint-disable @next/next/no-img-element */

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { FileText, Search, Settings } from "lucide-react";
import { useRouter } from "next/navigation";

import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";

type InfoItem = {
  id: number;
  title: string;
  category: string;
  sort_order: number;
  is_banner: boolean;
  file_url: string;
  download_url: string;
  file_name: string;
  content_type: string;
};

const categories = ["Informasi Umum", "Kepegawaian", "Persuratan dan Layanan", "Sekolah dan Kelembagaan"];

const fetchItems = async () => {
  const response = await apiFetch("/api/public/informasi-pelayanan-cabdin");
  if (!response.ok) throw new Error("Gagal memuat informasi pelayanan.");
  const payload = await response.json();
  return (payload.data?.items ?? []) as InfoItem[];
};

export default function InformasiPelayananCabdinPage() {
  const router = useRouter();
  const { user } = useAuth();
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("ALL");
  const [preview, setPreview] = useState<InfoItem | null>(null);
  const { data = [], isLoading, isError } = useQuery({ queryKey: ["service-info-public"], queryFn: fetchItems });
  const [canManage, setCanManage] = useState(false);

  useQuery({
    queryKey: ["service-info-admin-access", user?.id ?? "anon"],
    queryFn: async () => {
      const response = await apiFetch("/api/admin/informasi-pelayanan-cabdin");
      setCanManage(response.ok);
      return null;
    },
    enabled: Boolean(user),
    retry: false,
  });

  const banner = data.find((item) => item.is_banner) ?? data.find((item) => item.category === "Informasi Umum");
  const serviceItems = useMemo(() => data.filter((item) => item.id !== banner?.id), [banner?.id, data]);
  const categoryCount = useMemo(() => new Set(serviceItems.map((item) => item.category).filter(Boolean)).size, [serviceItems]);
  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return data.filter((item) => {
      if (item.id === banner?.id) return false;
      if (category !== "ALL" && item.category !== category) return false;
      return !term || item.title.toLowerCase().includes(term) || item.category.toLowerCase().includes(term);
    });
  }, [banner?.id, category, data, search]);

  return (
    <section className="min-h-full bg-slate-50 px-4 py-4 md:px-8 md:py-6">
      <div className="mx-auto max-w-7xl space-y-8">
        <header className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-slate-950 via-blue-900 to-indigo-800 p-6 text-white shadow-lg shadow-blue-950/15 md:p-8">
          <div className="absolute inset-0 opacity-20 [background-image:linear-gradient(135deg,rgba(255,255,255,.16)_1px,transparent_1px)] [background-size:24px_24px]" />
          <div className="relative flex flex-col gap-6 md:flex-row md:items-start md:justify-between">
            <div className="max-w-3xl space-y-4">
              <span className="inline-flex w-fit rounded-full border border-sky-200/30 bg-white/10 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-sky-100">
                Layanan Publik
              </span>
              <div className="space-y-2">
                <h1 className="text-3xl font-semibold tracking-tight md:text-4xl">Informasi Pelayanan Cabdin</h1>
                <p className="max-w-2xl text-sm leading-6 text-blue-50 md:text-base">
                  Pusat informasi Standart Layanan, Standart Operasional Prosedur, dan alur pelayanan Cabang Dinas Pendidikan Wilayah Kabupaten Bangkalan.
                </p>
              </div>
              <div className="flex flex-wrap gap-3 pt-1">
                <div className="rounded-2xl border border-white/15 bg-white/10 px-4 py-3 backdrop-blur">
                  <p className="text-2xl font-semibold">{serviceItems.length}</p>
                  <p className="text-xs font-medium text-blue-100">SOP dan informasi</p>
                </div>
                <div className="rounded-2xl border border-white/15 bg-white/10 px-4 py-3 backdrop-blur">
                  <p className="text-2xl font-semibold">{categoryCount}</p>
                  <p className="text-xs font-medium text-blue-100">Kategori layanan</p>
                </div>
              </div>
            </div>
            {canManage ? (
              <button
                onClick={() => router.push("/user/settings/informasi-pelayanan-cabdin")}
                className="inline-flex w-fit items-center gap-2 rounded-full border border-white/20 bg-white px-4 py-2 text-sm font-semibold text-slate-900 shadow-sm transition hover:bg-blue-50"
              >
                <Settings className="h-4 w-4" /> Kelola
              </button>
            ) : null}
          </div>
        </header>

        {banner ? (
          <div className="rounded-3xl border border-slate-200 bg-white p-3 shadow-sm md:p-5">
            <div className="mb-3 flex flex-col gap-1 px-1 md:mb-4">
              <p className="text-sm font-semibold text-slate-900">Informasi Umum</p>
              <p className="text-xs text-slate-500">Alur dan informasi dasar pelayanan</p>
            </div>
            <button
              onClick={() => setPreview(banner)}
              className="flex w-full items-center justify-center overflow-hidden rounded-2xl bg-slate-50 transition hover:bg-blue-50/70 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
            >
              {banner.content_type?.startsWith("image/") ? (
                <img src={banner.file_url} alt={banner.title} className="h-auto w-full max-w-full object-contain" />
              ) : (
                <span className="px-4 py-10 text-sm font-medium text-slate-500">{banner.file_name || "Banner informasi"}</span>
              )}
            </button>
          </div>
        ) : null}

        <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
          <p className="mb-3 text-sm font-medium text-slate-600">Temukan Standar Pelayanan, SOP atau informasi layanan yang dibutuhkan.</p>
          <div className="flex flex-col gap-3 md:flex-row md:items-center">
            <label className="relative flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Cari SOP, layanan, atau kata kunci..."
                className="h-11 w-full rounded-xl border border-slate-200 bg-white pl-10 pr-3 text-sm text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-blue-500 focus:ring-4 focus:ring-blue-100"
              />
            </label>
            <select
              value={category}
              onChange={(event) => setCategory(event.target.value)}
              className="h-11 rounded-xl border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 outline-none transition focus:border-blue-500 focus:ring-4 focus:ring-blue-100 md:w-72"
            >
              <option value="ALL">Semua kategori</option>
              {categories.map((item) => (
                <option key={item}>{item}</option>
              ))}
            </select>
          </div>
        </div>

        <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <div className="space-y-1">
            <h2 className="text-2xl font-semibold tracking-tight text-slate-950">Daftar Standar Pelayanan, SOP, dan Informasi Layanan</h2>
            <p className="max-w-2xl text-sm leading-6 text-slate-600">
              Pilih dokumen layanan untuk melihat detail alur, persyaratan, dan informasi terkait.
            </p>
          </div>
          {!isLoading && !isError ? <p className="text-sm font-medium text-slate-500">Menampilkan {filtered.length} informasi</p> : null}
        </div>

        {isLoading ? (
          <div className="rounded-2xl border border-slate-200 bg-white p-6 text-sm font-medium text-slate-500 shadow-sm">Memuat informasi...</div>
        ) : null}
        {isError ? (
          <div className="rounded-2xl border border-red-100 bg-red-50 p-6 text-sm font-medium text-red-700 shadow-sm">Gagal memuat informasi pelayanan.</div>
        ) : null}

        {!isLoading && !isError && filtered.length === 0 ? (
          <div className="rounded-3xl border border-slate-200 bg-white p-8 text-center shadow-sm">
            <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-blue-50 text-blue-700">
              <FileText className="h-6 w-6" />
            </div>
            <h3 className="text-lg font-semibold text-slate-950">Informasi tidak ditemukan</h3>
            <p className="mt-1 text-sm text-slate-500">Coba gunakan kata kunci lain atau pilih kategori berbeda.</p>
          </div>
        ) : null}

        {!isLoading && !isError && filtered.length > 0 ? (
          <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
            {filtered.map((item, index) => (
              <Card key={item.id} className="overflow-hidden rounded-3xl border-slate-200 bg-white shadow-sm transition hover:-translate-y-0.5 hover:shadow-md">
                <CardContent className="flex h-full flex-col p-0">
                  <div className="flex aspect-video items-center justify-center bg-gradient-to-br from-blue-50 to-slate-50 p-3 text-sm font-medium text-slate-500">
                    {item.content_type?.startsWith("image/") ? (
                      <img src={item.file_url} alt={item.title} className="h-full w-full object-contain" />
                    ) : (
                      item.file_name || "File SOP"
                    )}
                  </div>
                  <div className="flex flex-1 flex-col gap-4 p-5">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="rounded-full bg-blue-50 px-3 py-1 text-xs font-semibold text-blue-800">{item.category}</span>
                      <span className="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700">
                        SOP {String(index + 1).padStart(2, "0")}
                      </span>
                    </div>
                    <div className="space-y-2">
                      <h3 className="line-clamp-2 text-base font-semibold leading-6 text-slate-950">{item.title}</h3>
                      {item.file_name ? <p className="line-clamp-1 text-xs text-slate-500">{item.file_name}</p> : null}
                    </div>
                    <button
                      onClick={() => setPreview(item)}
                      className="mt-auto inline-flex h-10 items-center justify-center rounded-xl bg-blue-900 px-4 text-sm font-semibold text-white transition hover:bg-blue-800 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                    >
                      Lihat Detail
                    </button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        ) : null}
      </div>

      {preview ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-4xl rounded-xl bg-white p-5 shadow-xl">
            <h2 className="text-lg font-semibold">{preview.title}</h2>
            <div className="mt-4 flex max-h-[65vh] items-center justify-center overflow-auto rounded-md bg-zinc-100 p-3">
              {preview.content_type?.startsWith("image/") ? <img src={preview.file_url} alt={preview.title} className="max-h-[60vh] object-contain" /> : <iframe src={preview.file_url} className="h-[60vh] w-full rounded-md bg-white" title={preview.title} />}
            </div>
            <div className="mt-4 flex justify-end gap-2">
              <button onClick={() => setPreview(null)} className="rounded-md border border-zinc-200 px-4 py-2 text-sm font-semibold">Tutup</button>
              <a href={preview.file_url} target="_blank" className="rounded-md border border-zinc-200 px-4 py-2 text-sm font-semibold">Buka File</a>
              <a href={preview.download_url} className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-semibold text-white">Unduh</a>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}
