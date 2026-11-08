"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useAuth } from "@/lib/auth/client";
import { fetchServiceLinks } from "@/lib/service-links";

export default function PusatTautanPage() {
  const { user, status } = useAuth();
  const client = useQueryClient();
  const [search, setSearch] = useState("");
  const scope = status === "authenticated" && user ? "user" : "public";
  const { data, isLoading, isError } = useQuery({
    queryKey: ["service-links", scope, scope === "user" ? user?.id : "anon"],
    queryFn: () => fetchServiceLinks(scope),
    enabled: status !== "loading",
    staleTime: 0,
  });
  // Clearing these entries also covers logout from another tab.
  useEffect(() => { if (scope === "public") client.removeQueries({ queryKey: ["service-links", "user"], type: "inactive" }); }, [scope, client]);
  const settings = data?.pusat_tautan;
  const links = useMemo(() => (settings?.links ?? []).filter((item) =>
    `${item.title} ${item.description}`.toLowerCase().includes(search.toLowerCase())
  ), [settings, search]);
  const categories = (settings?.categories ?? []).filter((category) => links.some((link) => link.category_id === category.id));
  return <main className="mx-auto max-w-6xl space-y-8 p-6 md:p-10">
    <div><Link href="/" className="text-sm text-blue-700">← Beranda</Link><h1 className="mt-3 text-3xl font-bold">Pusat Tautan</h1><p className="mt-2 text-slate-600">Akses layanan dan referensi sesuai hak akses Anda.</p></div>
    <input aria-label="Cari tautan" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Cari tautan..." className="w-full rounded-xl border border-slate-300 p-3" />
    {isLoading || status === "loading" ? <p>Memuat tautan...</p> : isError ? <p role="alert">Gagal memuat tautan.</p> : links.length === 0 ? <p>Belum ada tautan yang tersedia.</p> : <><p className="text-sm text-slate-600">{links.length} tautan tersedia</p>{categories.sort((a,b)=>a.sort_order-b.sort_order).map((category) => <section key={category.id} className="space-y-4"><h2 className="text-xl font-semibold">{category.title} <span className="text-sm font-normal text-slate-500">({links.filter(link=>link.category_id===category.id).length})</span></h2><div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">{links.filter((link)=>link.category_id===category.id).sort((a,b)=>a.sort_order-b.sort_order).map((link)=><article key={link.id} className="rounded-xl border bg-white p-5 shadow-sm"><div className="text-2xl" aria-hidden="true">{link.icon || "🔗"}</div><h3 className="mt-2 font-semibold">{link.title}</h3><p className="mt-2 text-sm text-slate-600">{link.description}</p><a href={link.url} target="_blank" rel="noopener noreferrer" className="mt-5 inline-block rounded-lg bg-blue-700 px-4 py-2 text-sm font-semibold text-white">Kunjungi Halaman</a></article>)}</div></section>)}</>}
  </main>;
}
