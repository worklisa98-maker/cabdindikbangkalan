"use client";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useAuth } from "@/lib/auth/client";
import { fetchServiceLinks } from "@/lib/service-links";

export default function HaloPage() {
 const { user, status } = useAuth();
 const scope = status === "authenticated" && user ? "user" : "public";
 const { data, isFetching, isError } = useQuery({ queryKey:["service-links",scope,scope==="user"?user?.id:"anon"], queryFn:()=>fetchServiceLinks(scope), enabled:status!=="loading", staleTime:0, refetchOnMount:"always" });
 const halo = data?.halo;
 return <main className="mx-auto max-w-xl p-6 md:p-12"><Link href="/" className="text-blue-700">← Beranda</Link><div className="mt-8 rounded-2xl border bg-white p-6 shadow-lg">{isFetching || status==="loading" ? <p>Memuat layanan...</p> : isError ? <p role="alert">Gagal memuat layanan.</p> : !halo?.is_active ? <p>Layanan Halo Kepegawaian sedang tidak tersedia.</p> : <><h1 className="text-2xl font-bold">{halo.dialog_title}</h1><p className="mt-4 whitespace-pre-wrap text-slate-700">{halo.dialog_body}</p><a href={halo.url} target="_blank" rel="noopener noreferrer" className="mt-6 inline-block rounded-lg bg-blue-700 px-5 py-3 font-semibold text-white">{halo.dialog_button || "Lanjutkan"}</a></>}</div></main>;
}
